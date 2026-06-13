package dominance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const coinGeckoGlobalURL = "https://api.coingecko.com/api/v3/global"

// DominanceFetcher fetches BTC market dominance from CoinGecko (public API, no key).
type DominanceFetcher struct {
	client  *http.Client
	history []float64 // rolling BTC.D history for EMA + trend
	latest  *DominanceData
	mu      sync.RWMutex
}

// NewDominanceFetcher creates a fetcher using the provided HTTP client.
func NewDominanceFetcher(client *http.Client) *DominanceFetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &DominanceFetcher{client: client}
}

// fetch retrieves the current BTC dominance.
func (f *DominanceFetcher) fetch(ctx context.Context) (*DominanceData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, coinGeckoGlobalURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coingecko global: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coingecko global: HTTP %d", resp.StatusCode)
	}
	var body struct {
		Data struct {
			MarketCapPct map[string]float64 `json:"market_cap_percentage"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("coingecko global decode: %w", err)
	}
	btcD := body.Data.MarketCapPct["btc"]

	f.mu.Lock()
	f.history = append(f.history, btcD)
	if len(f.history) > 24 { // keep 24h rolling window
		f.history = f.history[len(f.history)-24:]
	}
	history := make([]float64, len(f.history))
	copy(history, f.history)
	var prevLatest *DominanceData
	if f.latest != nil {
		prevLatest = f.latest
	}
	f.mu.Unlock()

	data := &DominanceData{
		BTC_D_Pct: btcD,
		FetchedAt: time.Now().UTC(),
	}
	data.EMA14 = ema(history, 14)

	if prevLatest != nil {
		data.Delta1h = btcD - prevLatest.BTC_D_Pct
		if len(history) >= 24 {
			data.Delta24h = btcD - history[0]
		}
	}

	switch {
	case data.Delta1h > 0.1:
		data.Trend = "RISING"
	case data.Delta1h < -0.1:
		data.Trend = "FALLING"
	default:
		data.Trend = "FLAT"
	}

	f.mu.Lock()
	f.latest = data
	f.mu.Unlock()
	return data, nil
}

// GetLatest returns the most recently fetched dominance data, or nil.
func (f *DominanceFetcher) GetLatest() *DominanceData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.latest
}

// StartPolling fetches dominance at the given interval until ctx is cancelled.
func (f *DominanceFetcher) StartPolling(ctx context.Context, interval time.Duration) {
	go func() {
		// Fetch immediately on startup.
		if _, err := f.fetch(ctx); err != nil {
			slog.Warn("[dominance] initial fetch failed", "error", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := f.fetch(ctx); err != nil {
					slog.Warn("[dominance] fetch failed — using cached", "error", err)
				}
			}
		}
	}()
}

// ema computes the exponential moving average of vals with period n.
func ema(vals []float64, n int) float64 {
	if len(vals) == 0 {
		return 0
	}
	k := 2.0 / float64(n+1)
	e := vals[0]
	for _, v := range vals[1:] {
		e = v*k + e*(1-k)
	}
	return e
}
