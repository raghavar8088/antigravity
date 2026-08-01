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

// Delta BTCUSD perpetual mark price (public REST, no auth):
// GET https://api.india.delta.exchange/v2/tickers/BTCUSD
//
// Used by S16 (Perp_Spot_Basis_Momentum) to compute the perp premium over spot:
// basis_pct = (perpPrice - spotPrice) / spotPrice. Widening basis beyond what
// funding alone explains indicates aggressive one-sided leveraged flow.
//
// This read Binance's BTCUSDT mark and compared it against a Coinbase spot
// price, so the "basis" it computed was the gap between two foreign venues plus
// whatever spread existed between them — a number belonging to neither book this
// engine trades. Basis is a statement about ONE contract against ITS index, and
// Delta publishes both in a single response, so the two sides of the subtraction
// finally come from the same place.
const (
	deltaPerpTickerEndpoint = "https://api.india.delta.exchange/v2/tickers/BTCUSD"
	perpPricePollInterval   = 30 * time.Second
	perpPriceHTTPTimeout    = 5 * time.Second
)

// DeltaPerpPriceHolder polls the Delta BTCUSD perpetual mark price every 30s.
// Degrades gracefully: keeps last known good value and reports IsHealthy()=false
// on fetch failure rather than zeroing out.
type DeltaPerpPriceHolder struct {
	mu sync.RWMutex

	client *http.Client

	current   float64
	lastFetch time.Time
	healthy   bool
	lastErr   error
}

// NewDeltaPerpPriceHolder constructs a holder ready to be started with StartPolling.
func NewDeltaPerpPriceHolder() *DeltaPerpPriceHolder {
	return &DeltaPerpPriceHolder{client: &http.Client{Timeout: perpPriceHTTPTimeout}}
}

// StartPolling launches a background goroutine polling every 30s until ctx is cancelled.
func (p *DeltaPerpPriceHolder) StartPolling(ctx context.Context) {
	go func() {
		p.fetchOnce(ctx)
		ticker := time.NewTicker(perpPricePollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.fetchOnce(ctx)
			}
		}
	}()
}

// deltaPerpTickerResponse is Delta's ticker envelope. Numerics are quoted.
type deltaPerpTickerResponse struct {
	Success bool `json:"success"`
	Result  struct {
		Symbol    string `json:"symbol"`
		MarkPrice string `json:"mark_price"`
		SpotPrice string `json:"spot_price"`
	} `json:"result"`
}

func (p *DeltaPerpPriceHolder) fetchOnce(ctx context.Context) {
	val, err := p.doFetch(ctx)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.healthy = false
		p.lastErr = err
		log.Printf("[PERP_PRICE] fetch failed, retaining last known value=%.2f: %v", p.current, err)
		return
	}
	p.current = val
	p.lastFetch = time.Now().UTC()
	p.healthy = true
	p.lastErr = nil
}

func (p *DeltaPerpPriceHolder) doFetch(ctx context.Context) (float64, error) {
	url := deltaPerpTickerEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	var parsed deltaPerpTickerResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode JSON: %w", err)
	}
	if !parsed.Success {
		return 0, fmt.Errorf("delta ticker returned success=false")
	}
	var mark float64
	if _, err := fmt.Sscanf(parsed.Result.MarkPrice, "%f", &mark); err != nil || mark <= 0 {
		return 0, fmt.Errorf("invalid mark_price %q", parsed.Result.MarkPrice)
	}
	return mark, nil
}

// Current returns the most recently fetched perp mark price (0 if never populated).
func (p *DeltaPerpPriceHolder) Current() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current
}

// IsHealthy returns true if the most recent fetch attempt succeeded.
func (p *DeltaPerpPriceHolder) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// IsPopulated returns true once at least one successful fetch has occurred.
func (p *DeltaPerpPriceHolder) IsPopulated() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastFetch.Unix() > 0 && p.current > 0
}
