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

const (
	fundingEndpoint = "https://fapi.binance.com/fapi/v1/fundingRate?symbol=BTCUSDT&limit=1"
	fundingCacheTTL = 15 * time.Minute
	httpTimeout     = 15 * time.Second
	maxRetries      = 3
)

// FundingFetcher polls the Binance futures funding rate endpoint and caches
// the result for 15 minutes to respect rate limits.
type FundingFetcher struct {
	client  *http.Client
	cache   *FundingData
	cachedAt time.Time
	cacheTTL time.Duration
	mu      sync.RWMutex
}

// NewFundingFetcher creates a FundingFetcher with a 15-second HTTP timeout.
func NewFundingFetcher() *FundingFetcher {
	return &FundingFetcher{
		client:   &http.Client{Timeout: httpTimeout},
		cacheTTL: fundingCacheTTL,
	}
}

// Fetch fetches the latest funding rate from Binance. Results are cached.
func (f *FundingFetcher) Fetch(ctx context.Context) (*FundingData, error) {
	f.mu.RLock()
	if f.cache != nil && time.Since(f.cachedAt) < f.cacheTTL {
		cached := *f.cache
		f.mu.RUnlock()
		return &cached, nil
	}
	f.mu.RUnlock()

	var data *FundingData
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		data, lastErr = f.doFetch(ctx)
		if lastErr == nil {
			break
		}
		backoff := time.Duration(attempt*attempt) * 500 * time.Millisecond
		slog.Warn("[derivatives] funding fetch failed, retrying",
			"attempt", attempt,
			"err", lastErr,
			"backoff_ms", backoff.Milliseconds(),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	if lastErr != nil {
		// Return stale cache rather than nil so callers always have a value.
		f.mu.RLock()
		stale := f.cache
		f.mu.RUnlock()
		if stale != nil {
			slog.Warn("[derivatives] using stale funding data", "age", time.Since(f.cachedAt))
			return stale, nil
		}
		return nil, fmt.Errorf("funding fetch failed after %d attempts: %w", maxRetries, lastErr)
	}

	f.mu.Lock()
	f.cache = data
	f.cachedAt = time.Now()
	f.mu.Unlock()

	slog.Info("[derivatives] funding data updated",
		"rate", data.Rate,
		"label", data.Label,
		"signal", data.Signal,
	)
	return data, nil
}

// GetLatest returns the cached funding data. Never returns nil after first
// successful fetch; returns a zero-value FundingData if not yet fetched.
func (f *FundingFetcher) GetLatest() *FundingData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.cache == nil {
		return &FundingData{Symbol: "BTCUSDT", Label: "NEUTRAL", Signal: "NEUTRAL"}
	}
	c := *f.cache
	return &c
}

// StartPolling starts a background goroutine that fetches funding data every
// interval. The goroutine respects ctx cancellation.
func (f *FundingFetcher) StartPolling(ctx context.Context, interval time.Duration) {
	go func() {
		// Fetch immediately on start.
		if _, err := f.Fetch(ctx); err != nil {
			slog.Warn("[derivatives] initial funding fetch failed", "err", err)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := f.Fetch(ctx); err != nil {
					slog.Warn("[derivatives] funding poll failed", "err", err)
				}
			}
		}
	}()
}

// ── internals ─────────────────────────────────────────────────────────────────

type binanceFundingRow struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
}

func (f *FundingFetcher) doFetch(ctx context.Context) (*FundingData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fundingEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var rows []binanceFundingRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty funding rate response")
	}

	var rate float64
	_, parseErr := fmt.Sscanf(rows[0].FundingRate, "%f", &rate)
	if parseErr != nil {
		return nil, fmt.Errorf("parse rate %q: %w", rows[0].FundingRate, parseErr)
	}

	label, signal, score := classifyFunding(rate)
	_ = score // used by score.go

	return &FundingData{
		Symbol:         "BTCUSDT",
		Rate:           rate,
		AnnualisedRate: rate * 3 * 365,
		Label:          label,
		Signal:         signal,
		FetchedAt:      time.Now().UTC(),
	}, nil
}

// classifyFunding returns (label, signal, score) for a funding rate value.
func classifyFunding(rate float64) (label, signal string, score float64) {
	ratePct := rate * 100
	switch {
	case ratePct < -0.050:
		return "EXTREME_NEGATIVE", "SQUEEZE_SETUP", 3.0
	case ratePct < -0.010:
		return "NEGATIVE", "NEUTRAL", 1.0
	case ratePct <= 0.050:
		return "NEUTRAL", "NEUTRAL", 0.0
	case ratePct <= 0.100:
		return "POSITIVE", "NEUTRAL", -1.0
	default:
		return "EXTREME_POSITIVE", "OVERLEVERAGED_LONGS", -2.5
	}
}

// FundingScore returns the numeric score for a FundingData value.
func FundingScore(d *FundingData) float64 {
	_, _, score := classifyFunding(d.Rate)
	return score
}
