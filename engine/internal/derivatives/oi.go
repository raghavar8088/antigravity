package derivatives

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Open interest comes from Delta, the venue this engine executes on.
//
// It was Binance's BTCUSDT OI. Open interest is a count of the contracts
// outstanding in ONE book — it is the size of the position the participants of
// that specific market are carrying. Binance's OI says nothing about how
// crowded Delta's book is, and OI-divergence signals built on it were reading
// another exchange's crowd.
//
// Delta reports OI in BTC on its ticker ("oi": "1349.4730"), alongside
// oi_value_usd. Binance's openInterest was also in BTC, so unlike funding,
// depth and volume there is no unit conversion here — but that is worth stating
// rather than leaving a reader to assume it, since every other Delta field in
// this codebase needed one.
const (
	oiEndpoint     = "https://api.india.delta.exchange/v2/tickers/BTCUSD"
	oiPollInterval = time.Hour
)

// OIFetcher polls Delta's BTCUSD open interest and classifies the market state
// by comparing the current OI to the value from 1 hour ago.
type OIFetcher struct {
	client *http.Client
	cache  *OIData
	mu     sync.RWMutex

	lastOI       float64
	lastOIAt     time.Time
	lastPriceDir string
	prevPrice    float64
}

// NewOIFetcher creates an OIFetcher with a 15-second HTTP timeout.
func NewOIFetcher() *OIFetcher {
	return &OIFetcher{
		client: &http.Client{Timeout: httpTimeout},
	}
}

// Fetch fetches the current open interest and updates the cached OIData.
// currentPrice is used to determine price direction vs the previous fetch.
func (f *OIFetcher) Fetch(ctx context.Context) (*OIData, error) {
	var lastErr error
	var rawOI float64

	for attempt := 1; attempt <= maxRetries; attempt++ {
		var err error
		rawOI, err = f.doFetch(ctx)
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = err
		backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
		slog.Warn("[derivatives] OI fetch failed, retrying",
			"attempt", attempt, "err", err, "backoff_ms", backoff.Milliseconds())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	if lastErr != nil {
		f.mu.RLock()
		stale := f.cache
		f.mu.RUnlock()
		if stale != nil {
			return stale, nil
		}
		return nil, fmt.Errorf("OI fetch failed: %w", lastErr)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	changePct := 0.0
	oiAgo := f.lastOI
	if oiAgo > 0 && time.Since(f.lastOIAt) >= 55*time.Minute {
		changePct = (rawOI - oiAgo) / oiAgo * 100
	}

	trend := "FLAT"
	switch {
	case changePct > 2.0:
		trend = "RISING"
	case changePct < -2.0:
		trend = "FALLING"
	}

	// Store snapshot for next 1-hour comparison.
	if f.lastOI == 0 || time.Since(f.lastOIAt) >= oiPollInterval {
		f.lastOI = rawOI
		f.lastOIAt = time.Now()
	}

	state, score := classifyOI(trend, f.lastPriceDir)
	_ = score

	data := &OIData{
		Symbol:         "BTCUSDT",
		OI_USD:         rawOI,
		OI_1h_ago:      oiAgo,
		OI_change_pct:  changePct,
		Trend:          trend,
		PriceDirection: f.lastPriceDir,
		MarketState:    state,
		FetchedAt:      time.Now().UTC(),
	}
	f.cache = data

	slog.Info("[derivatives] OI data updated",
		"oi_usd", rawOI,
		"change_pct", changePct,
		"trend", trend,
		"state", state,
	)
	return data, nil
}

// UpdatePriceDirection must be called by the market data layer so the OI
// fetcher knows whether price moved up or down since the last update.
func (f *OIFetcher) UpdatePriceDirection(currentPrice float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prevPrice == 0 {
		f.prevPrice = currentPrice
		f.lastPriceDir = "FLAT"
		return
	}
	switch {
	case currentPrice > f.prevPrice*1.001:
		f.lastPriceDir = "UP"
	case currentPrice < f.prevPrice*0.999:
		f.lastPriceDir = "DOWN"
	default:
		f.lastPriceDir = "FLAT"
	}
	f.prevPrice = currentPrice
}

// GetLatest returns the cached OI data. Never nil after first successful fetch.
func (f *OIFetcher) GetLatest() *OIData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.cache == nil {
		return &OIData{Symbol: "BTCUSDT", Trend: "FLAT", MarketState: "NEUTRAL"}
	}
	c := *f.cache
	return &c
}

// StartPolling starts a background goroutine polling OI every interval.
func (f *OIFetcher) StartPolling(ctx context.Context, interval time.Duration) {
	go func() {
		if _, err := f.Fetch(ctx); err != nil {
			slog.Warn("[derivatives] initial OI fetch failed", "err", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := f.Fetch(ctx); err != nil {
					slog.Warn("[derivatives] OI poll failed", "err", err)
				}
			}
		}
	}()
}

// ── internals ─────────────────────────────────────────────────────────────────

// deltaOIResponse is Delta's ticker envelope. "oi" is open interest in BTC,
// quoted as a string like every other numeric field Delta returns.
type deltaOIResponse struct {
	Success bool `json:"success"`
	Result  struct {
		OpenInterest string `json:"oi"`
	} `json:"result"`
}

func (f *OIFetcher) doFetch(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oiEndpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	resp, err := f.client.Do(req)
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

	var row deltaOIResponse
	if err := json.Unmarshal(body, &row); err != nil {
		return 0, fmt.Errorf("decode JSON: %w", err)
	}
	if !row.Success || row.Result.OpenInterest == "" {
		// Absent OI must be an error, not zero. Zero open interest would mean
		// nobody holds a position at all, which is a real and dramatic claim.
		return 0, fmt.Errorf("delta ticker carried no open interest")
	}

	var oi float64
	if _, err := fmt.Sscanf(row.Result.OpenInterest, "%f", &oi); err != nil {
		return 0, fmt.Errorf("parse OI %q: %w", row.Result.OpenInterest, err)
	}
	return oi, nil
}

// classifyOI returns the market state and OI score.
func classifyOI(trend, priceDir string) (state string, score float64) {
	switch {
	case trend == "RISING" && priceDir == "UP":
		return "GENUINE_BREAKOUT", 2.0
	case trend == "RISING" && priceDir == "DOWN":
		return "AGGRESSIVE_SHORTS", -2.0
	case trend == "FALLING" && priceDir == "UP":
		return "SHORT_SQUEEZE", 1.0
	case trend == "FALLING" && priceDir == "DOWN":
		return "CAPITULATION", 0.0
	default:
		return "NEUTRAL", 0.0
	}
}

// OIScore returns the numeric score for an OIData value.
func OIScore(d *OIData) float64 {
	_, score := classifyOI(d.Trend, d.PriceDirection)
	return score
}
