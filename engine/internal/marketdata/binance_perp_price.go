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

// Binance USD-M futures mark price REST endpoint (public, no auth). Verified
// via web search (2026): GET https://fapi.binance.com/fapi/v1/premiumIndex
// Response: {"symbol":"BTCUSDT","markPrice":"...","indexPrice":"...",
//
//	"lastFundingRate":"...","nextFundingTime":...,"time":...}
//
// Used by S16 (Perp_Spot_Basis_Momentum) to compute the BTC perp premium vs
// Coinbase spot: basis_pct = (perpPrice - spotPrice) / spotPrice. This is the
// standard crypto perp basis trade construct — see Binance/Deribit/FTX-era
// basis-trading literature; widening unexplained basis (beyond what funding
// alone predicts) indicates aggressive one-sided leveraged flow.
const (
	binancePerpPriceEndpoint = "https://fapi.binance.com/fapi/v1/premiumIndex"
	perpPricePollInterval    = 30 * time.Second
	perpPriceHTTPTimeout     = 5 * time.Second
)

// BinancePerpPriceHolder polls Binance BTCUSDT perpetual mark price every 30s.
// Degrades gracefully: keeps last known good value and reports IsHealthy()=false
// on fetch failure rather than zeroing out.
type BinancePerpPriceHolder struct {
	mu sync.RWMutex

	client *http.Client

	current   float64
	lastFetch time.Time
	healthy   bool
	lastErr   error
}

// NewBinancePerpPriceHolder constructs a holder ready to be started with StartPolling.
func NewBinancePerpPriceHolder() *BinancePerpPriceHolder {
	return &BinancePerpPriceHolder{client: &http.Client{Timeout: perpPriceHTTPTimeout}}
}

// StartPolling launches a background goroutine polling every 30s until ctx is cancelled.
func (p *BinancePerpPriceHolder) StartPolling(ctx context.Context) {
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

type binancePremiumIndexResponse struct {
	Symbol    string `json:"symbol"`
	MarkPrice string `json:"markPrice"`
}

func (p *BinancePerpPriceHolder) fetchOnce(ctx context.Context) {
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

func (p *BinancePerpPriceHolder) doFetch(ctx context.Context) (float64, error) {
	url := fmt.Sprintf("%s?symbol=BTCUSDT", binancePerpPriceEndpoint)
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
	var parsed binancePremiumIndexResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode JSON: %w", err)
	}
	var mark float64
	if _, err := fmt.Sscanf(parsed.MarkPrice, "%f", &mark); err != nil || mark <= 0 {
		return 0, fmt.Errorf("invalid markPrice %q", parsed.MarkPrice)
	}
	return mark, nil
}

// Current returns the most recently fetched perp mark price (0 if never populated).
func (p *BinancePerpPriceHolder) Current() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current
}

// IsHealthy returns true if the most recent fetch attempt succeeded.
func (p *BinancePerpPriceHolder) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// IsPopulated returns true once at least one successful fetch has occurred.
func (p *BinancePerpPriceHolder) IsPopulated() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastFetch.Unix() > 0 && p.current > 0
}
