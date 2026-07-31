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

// Funding comes from Delta, the venue this engine executes on.
//
// It used to come from Binance. A perpetual's funding rate is a property of ONE
// contract on ONE exchange — it is the payment that ties that specific contract
// to spot — so reading Binance's while trading Delta's describes a cost this
// account never pays and a positioning imbalance in a book it never touches.
//
// THE UNIT DIFFERS, AND SILENTLY. Binance publishes funding as a DECIMAL
// (0.000096 = 0.0096% per 8h). Delta publishes the same economic rate as a
// PERCENT (0.01 = 0.01% per 8h). Everything downstream here expects the decimal
// — classifyFunding multiplies by 100 to get percent, and the scalper
// thresholds are documented in raw decimals (S8 at +/-0.0003). Passing Delta's
// number through unconverted inflates it a HUNDREDFOLD: a perfectly ordinary
// 0.01% funding rate reads as 1%, every strategy sees EXTREME_POSITIVE /
// OVERLEVERAGED_LONGS permanently, and nothing errors.
const (
	// One ticker call carries funding, mark, spot and open interest.
	fundingEndpoint = "https://api.india.delta.exchange/v2/tickers/BTCUSD"
	fundingCacheTTL = 15 * time.Minute
	httpTimeout     = 15 * time.Second
	maxRetries      = 3
)

// FundingFetcher polls Delta's ticker for the BTCUSD perpetual funding rate and
// caches the result for 15 minutes to respect rate limits.
type FundingFetcher struct {
	client   *http.Client
	cache    *FundingData
	cachedAt time.Time
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// NewFundingFetcher creates a FundingFetcher with a 15-second HTTP timeout.
func NewFundingFetcher() *FundingFetcher {
	return &FundingFetcher{
		client:   &http.Client{Timeout: httpTimeout},
		cacheTTL: fundingCacheTTL,
	}
}

// Fetch fetches the latest funding rate from Delta. Results are cached.
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

// deltaTickerEnvelope is Delta's /v2/tickers/<symbol> response. Numeric fields
// arrive as quoted strings.
type deltaTickerEnvelope struct {
	Success bool `json:"success"`
	Result  struct {
		Symbol      string `json:"symbol"`
		FundingRate string `json:"funding_rate"`
	} `json:"result"`
}

// deltaFundingPercentToDecimal converts Delta's percent-quoted funding rate into
// the decimal every consumer in this repo expects.
//
// Kept as a named function rather than an inline /100 so the conversion is
// visible at the call site and testable on its own. It is the single most
// dangerous line in this file: omitting it does not fail, it just multiplies
// every funding reading by a hundred.
func deltaFundingPercentToDecimal(pct float64) float64 { return pct / 100 }

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

	var env deltaTickerEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if !env.Success || env.Result.FundingRate == "" {
		return nil, fmt.Errorf("delta ticker returned no funding rate")
	}

	var pct float64
	_, parseErr := fmt.Sscanf(env.Result.FundingRate, "%f", &pct)
	if parseErr != nil {
		return nil, fmt.Errorf("parse rate %q: %w", env.Result.FundingRate, parseErr)
	}
	// Percent -> decimal. See deltaFundingPercentToDecimal.
	rate := deltaFundingPercentToDecimal(pct)

	label, signal, score := classifyFunding(rate)
	_ = score // used by score.go

	symbol := env.Result.Symbol
	if symbol == "" {
		symbol = "BTCUSD"
	}

	return &FundingData{
		Symbol:         symbol,
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
