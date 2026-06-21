package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Deribit DVOL (BTC) public REST volatility index endpoint.
// Verified via web search (2026): GET https://www.deribit.com/api/v2/public/get_volatility_index_data
// Params: currency=BTC, start_timestamp (ms), end_timestamp (ms), resolution (named
// "resolution" in some docs/SDKs and "dvol_resolution" in others — Deribit's JSON-RPC
// HTTP shim accepts "resolution" in seconds, e.g. "3600"; confirmed against public
// community wrappers, e.g. cryptarbitrage-code/volatility-dashboard and
// schepal/deribit_data_collector reference implementations).
// Response: result.data = [[timestamp, open, high, low, close], ...]
// DVOL is Deribit's 30-day forward-looking implied volatility index (BTC's "VIX"),
// publicly documented by Deribit since 2021. IV-RV spread (DVOL minus realized vol)
// is the volatility risk premium (VRP) — when IV >> RV, options are rich and vol-selling
// strategies (e.g. S10/S12 here) have positive expected edge; this is well-established
// in both traditional vol-trading literature and crypto-specific research
// (see Deribit Insights "DVOL - Deribit Implied Volatility Index").
const (
	deribitDVOLEndpoint  = "https://www.deribit.com/api/v2/public/get_volatility_index_data"
	dvolPollInterval     = 5 * time.Minute
	dvolHistoryCap       = 12 // 12 readings @ 5min = 1hr rolling window
	dvolHTTPTimeout      = 10 * time.Second
)

// DeribitDVOLHolder polls Deribit's public BTC DVOL index every 5 minutes and
// exposes the current value plus a rolling 1hr history. Safe for concurrent
// use; degrades gracefully on fetch failure by keeping the last known good
// value instead of zeroing out (avoids permanently disabling DVOL-dependent
// strategies just because one poll cycle failed).
type DeribitDVOLHolder struct {
	mu sync.RWMutex

	client *http.Client

	current   float64
	history   []float64 // rolling, newest last, capped at dvolHistoryCap
	lastFetch time.Time
	healthy   bool
	lastErr   error
}

// NewDeribitDVOLHolder constructs a holder ready to be started with StartPolling.
func NewDeribitDVOLHolder() *DeribitDVOLHolder {
	return &DeribitDVOLHolder{
		client: &http.Client{Timeout: dvolHTTPTimeout},
	}
}

// StartPolling launches a background goroutine that fetches DVOL every 5
// minutes until ctx is cancelled. Performs an initial synchronous-ish fetch
// (non-blocking — runs in the same goroutine before entering the ticker loop)
// so the holder is populated as soon as possible after startup.
func (d *DeribitDVOLHolder) StartPolling(ctx context.Context) {
	go func() {
		d.fetchOnce(ctx)
		ticker := time.NewTicker(dvolPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.fetchOnce(ctx)
			}
		}
	}()
}

func (d *DeribitDVOLHolder) fetchOnce(ctx context.Context) {
	val, err := d.doFetch(ctx)
	d.mu.Lock()
	defer d.mu.Unlock()
	if err != nil {
		// Graceful degradation: keep last known good value, mark unhealthy, log.
		d.healthy = false
		d.lastErr = err
		log.Printf("[DVOL] fetch failed, retaining last known value=%.2f: %v", d.current, err)
		return
	}
	d.current = val
	d.history = append(d.history, val)
	if len(d.history) > dvolHistoryCap {
		d.history = d.history[len(d.history)-dvolHistoryCap:]
	}
	d.lastFetch = time.Now().UTC()
	d.healthy = true
	d.lastErr = nil
}

type deribitVolIndexResponse struct {
	Result struct {
		Data [][]float64 `json:"data"`
	} `json:"result"`
}

// doFetch fetches the latest 1-hour resolution DVOL candle and returns its close.
func (d *DeribitDVOLHolder) doFetch(ctx context.Context) (float64, error) {
	now := time.Now().UTC()
	start := now.Add(-2 * time.Hour).UnixMilli()
	end := now.UnixMilli()
	url := fmt.Sprintf("%s?currency=BTC&start_timestamp=%d&end_timestamp=%d&resolution=3600",
		deribitDVOLEndpoint, start, end)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}

	var parsed deribitVolIndexResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode JSON: %w", err)
	}
	rows := parsed.Result.Data
	if len(rows) == 0 {
		return 0, fmt.Errorf("empty DVOL data")
	}
	// Each row: [timestamp, open, high, low, close]. Use close of most recent row.
	last := rows[len(rows)-1]
	if len(last) < 5 {
		return 0, fmt.Errorf("malformed DVOL row: %v", last)
	}
	return last[4], nil
}

// Current returns the most recently fetched DVOL value (0 if never populated).
func (d *DeribitDVOLHolder) Current() float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.current
}

// History returns a copy of the rolling DVOL history (oldest first, up to 12 readings).
func (d *DeribitDVOLHolder) History() []float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]float64, len(d.history))
	copy(out, d.history)
	return out
}

// IsHealthy returns true if the most recent fetch attempt succeeded.
func (d *DeribitDVOLHolder) IsHealthy() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.healthy
}

// IsPopulated returns true once at least one successful fetch has occurred,
// regardless of current health (so a since-failed feed still reports its last
// good value as "populated").
func (d *DeribitDVOLHolder) IsPopulated() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastFetch.Unix() > 0 && d.current > 0
}

// LastError returns the error from the most recent failed fetch, if any.
func (d *DeribitDVOLHolder) LastError() error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastErr
}
