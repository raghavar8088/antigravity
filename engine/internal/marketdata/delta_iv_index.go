package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Delta BTC implied-volatility index.
//
// This polled Deribit's DVOL — a 30-day forward-looking IV index, BTC's "VIX".
// Deribit is not a venue this engine trades, and its options are a different
// book from Delta's, so the volatility being measured was not the volatility
// being priced by the desks that consume it. The options buying and selling
// desks quote against Delta's chain; their IV signal has to come from there too.
//
// Delta publishes no volatility index, so one is computed from the chain it does
// publish: /v2/tickers returns every listed option with a mark_iv, and the ATM
// contracts nearest 30 days out are what such an index is built from anyway.
// The value here is the mean of the nearest-ATM call and put IV in the expiry
// closest to 30 days — a same-construction, same-venue replacement.
//
// TWO SCHEMA FACTS, verified against the live endpoint rather than assumed:
//
//  1. mark_iv is a DECIMAL (0.2484 = 24.84%). DVOL is quoted as a PERCENTAGE
//     (24.84), and every consumer here treats it that way — RealizedVol is
//     computed as "stdev * sqrt(24*365) * 100 // comparable to DVOL". Passing
//     the decimal through unscaled makes implied vol read a hundred times
//     smaller than realised, so every IV-vs-RV comparison inverts: options look
//     permanently cheap and vol-selling strategies see edge that is not there.
//
//  2. underlying_asset_symbol is FLAT on /v2/tickers and NESTED under
//     underlying_asset on /v2/products. Reading the wrong one filters 458 BTC
//     contracts down to zero and the index quietly reports nothing at all.
//     Both spellings are accepted here for that reason.
const (
	// Public, no auth. Both option types in one call.
	deltaOptionTickersEndpoint = "https://api.india.delta.exchange/v2/tickers?contract_types=call_options,put_options"
	// The tenor a 30-day IV index targets; the closest listed expiry is used.
	dvolTargetDays   = 30.0
	dvolUnderlying   = "BTC"
	dvolPollInterval = 5 * time.Minute
	dvolHistoryCap   = 12 // 12 readings @ 5min = 1hr rolling window
	dvolHTTPTimeout  = 10 * time.Second
)

// DeribitDVOLHolder computes Delta's BTC ATM implied-volatility index every 5
// minutes and exposes the current value plus a rolling 1hr history.
//
// The type name is kept so every consumer and every LoopDeps field stays
// untouched; only the source changed. Current() still means the same thing:
// 30-day BTC implied volatility, as a percentage. Safe for concurrent
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

// deltaOptionTicker is one listed option on /v2/tickers. Numerics are quoted
// strings; mark_iv lives under "quotes".
type deltaOptionTicker struct {
	Symbol      string `json:"symbol"`
	ContractTyp string `json:"contract_type"`
	StrikePrice string `json:"strike_price"`
	SpotPrice   string `json:"spot_price"`
	// Flat on /v2/tickers.
	UnderlyingFlat string `json:"underlying_asset_symbol"`
	// Nested on /v2/products. Accepted so a schema difference in either
	// direction cannot silently reduce the chain to nothing.
	UnderlyingNested struct {
		Symbol string `json:"symbol"`
	} `json:"underlying_asset"`
	Quotes struct {
		MarkIV string `json:"mark_iv"`
	} `json:"quotes"`
}

func (t deltaOptionTicker) underlying() string {
	if t.UnderlyingFlat != "" {
		return t.UnderlyingFlat
	}
	return t.UnderlyingNested.Symbol
}

type deltaOptionTickersResponse struct {
	Success bool                `json:"success"`
	Result  []deltaOptionTicker `json:"result"`
}

// deltaOptionExpiry parses the expiry out of a Delta option symbol.
// Format: C-BTC-85000-280826 -> 28 Aug 2026, settling 12:00 UTC.
func deltaOptionExpiry(symbol string) (time.Time, bool) {
	parts := strings.Split(symbol, "-")
	if len(parts) < 4 {
		return time.Time{}, false
	}
	d := parts[len(parts)-1]
	if len(d) != 6 {
		return time.Time{}, false
	}
	day, err1 := strconv.Atoi(d[0:2])
	mon, err2 := strconv.Atoi(d[2:4])
	yr, err3 := strconv.Atoi(d[4:6])
	if err1 != nil || err2 != nil || err3 != nil || mon < 1 || mon > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	return time.Date(2000+yr, time.Month(mon), day, 12, 0, 0, 0, time.UTC), true
}

// ivCandidate is one usable option quote reduced to what the index needs.
type ivCandidate struct {
	iv     float64
	strike float64
	spot   float64
	days   float64
	isCall bool
}

// dvolFromChain computes the ATM implied-volatility index from a set of option
// tickers, as a PERCENTAGE.
//
// Split out from the HTTP call so the selection logic — which expiry, which
// strike, which unit — is testable against captured payloads.
func dvolFromChain(tickers []deltaOptionTicker, now time.Time) (float64, error) {
	var cands []ivCandidate
	for _, t := range tickers {
		if t.underlying() != dvolUnderlying {
			continue
		}
		iv, err := strconv.ParseFloat(t.Quotes.MarkIV, 64)
		if err != nil || iv <= 0 {
			continue
		}
		strike, err := strconv.ParseFloat(t.StrikePrice, 64)
		if err != nil || strike <= 0 {
			continue
		}
		spot, err := strconv.ParseFloat(t.SpotPrice, 64)
		if err != nil || spot <= 0 {
			continue
		}
		exp, ok := deltaOptionExpiry(t.Symbol)
		if !ok {
			continue
		}
		days := exp.Sub(now).Hours() / 24
		if days <= 0 {
			continue // expired: IV is meaningless there
		}
		cands = append(cands, ivCandidate{
			iv: iv, strike: strike, spot: spot, days: days,
			isCall: strings.HasPrefix(t.Symbol, "C-"),
		})
	}
	if len(cands) == 0 {
		return 0, fmt.Errorf("no usable BTC option tickers (checked %d)", len(tickers))
	}

	// The expiry closest to the 30-day tenor.
	bestDays := cands[0].days
	for _, c := range cands {
		if math.Abs(c.days-dvolTargetDays) < math.Abs(bestDays-dvolTargetDays) {
			bestDays = c.days
		}
	}

	// Within it, the call and put nearest the money. Averaging the two keeps this
	// a volatility reading rather than a directional skew reading.
	var bestCall, bestPut *ivCandidate
	for i := range cands {
		c := &cands[i]
		if math.Abs(c.days-bestDays) > 0.5 {
			continue
		}
		if c.isCall {
			if bestCall == nil || math.Abs(c.strike-c.spot) < math.Abs(bestCall.strike-bestCall.spot) {
				bestCall = c
			}
		} else if bestPut == nil || math.Abs(c.strike-c.spot) < math.Abs(bestPut.strike-bestPut.spot) {
			bestPut = c
		}
	}

	sum, n := 0.0, 0
	if bestCall != nil {
		sum += bestCall.iv
		n++
	}
	if bestPut != nil {
		sum += bestPut.iv
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("no ATM contract near the %.0f-day tenor", dvolTargetDays)
	}

	// Decimal -> percentage. The index is quoted as 24.84, not 0.2484, and every
	// consumer compares it against a realised vol already expressed that way.
	return sum / float64(n) * 100, nil
}

// doFetch pulls Delta's option chain and reduces it to one IV reading.
func (d *DeribitDVOLHolder) doFetch(ctx context.Context) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deltaOptionTickersEndpoint, nil)
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<22))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	var parsed deltaOptionTickersResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode JSON: %w", err)
	}
	if !parsed.Success {
		return 0, fmt.Errorf("delta tickers returned success=false")
	}
	return dvolFromChain(parsed.Result, time.Now().UTC())
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
