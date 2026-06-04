package sor

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// slippageKey identifies a slippage observation bucket.
type slippageKey struct {
	Venue  VenueID
	Symbol string
}

// slippageStat holds the rolling realized-slippage statistics for a bucket.
type slippageStat struct {
	EwmaBps    float64 // EWMA of realized slippage in bps
	VarBps     float64 // EWMA of squared deviation (for stddev)
	Count      int64
	LastBps    float64
	UpdatedAt  time.Time
}

// SlippageEstimate is the predicted slippage for a proposed order.
type SlippageEstimate struct {
	VenueID        VenueID
	Symbol         string
	BaseBps        float64 // historical baseline (EWMA)
	ImpactBps      float64 // market-impact component from order size vs depth
	ExpectedBps    float64 // base + impact
	StdDevBps      float64 // historical volatility of slippage
	ConfidenceBps  float64 // 1-sigma upper bound (expected + stddev)
}

// SlippageEngine predicts execution slippage and learns from realized fills.
// It minimises implementation shortfall by feeding ExpectedBps into routing.
//
// Model:
//
//	expected = baseline(venue,symbol)        // EWMA of realized history
//	         + impact(qty, depth, spread)     // square-root market-impact law
//
// The square-root impact law is the institutional standard:
//
//	impact_bps ≈ k * spread_bps * sqrt(qty / depth)
type SlippageEngine struct {
	mu    sync.RWMutex
	stats map[slippageKey]*slippageStat
	store ledger.Store

	// ImpactCoefficient k in the square-root impact law.
	ImpactCoefficient float64
	// Alpha is the EWMA smoothing factor for learning from realized slippage.
	Alpha float64
	// DefaultBaselineBps is used when no history exists for a bucket.
	DefaultBaselineBps float64
}

// NewSlippageEngine returns a slippage engine with institutional defaults.
func NewSlippageEngine(store ledger.Store) *SlippageEngine {
	return &SlippageEngine{
		stats:              make(map[slippageKey]*slippageStat),
		store:              store,
		ImpactCoefficient:  0.5,
		Alpha:              0.2,
		DefaultBaselineBps: 2.0,
	}
}

// Expected predicts slippage for a proposed order.
// depth is the executable book depth on the relevant side (base-asset units).
func (e *SlippageEngine) Expected(venue VenueID, symbol, side string, qty, depth, spreadBps float64) SlippageEstimate {
	e.mu.RLock()
	stat := e.stats[slippageKey{venue, symbol}]
	e.mu.RUnlock()

	est := SlippageEstimate{VenueID: venue, Symbol: symbol}

	// Baseline from history (or default).
	if stat != nil && stat.Count > 0 {
		est.BaseBps = stat.EwmaBps
		est.StdDevBps = math.Sqrt(math.Max(0, stat.VarBps))
	} else {
		est.BaseBps = e.DefaultBaselineBps
		est.StdDevBps = e.DefaultBaselineBps
	}

	// Market-impact: square-root law.
	if depth > 0 && qty > 0 {
		participation := qty / depth
		spread := spreadBps
		if spread <= 0 {
			spread = 1.0 // floor to avoid zero impact on zero-spread synthetic books
		}
		est.ImpactBps = e.ImpactCoefficient * spread * math.Sqrt(participation)
	}

	est.ExpectedBps = est.BaseBps + est.ImpactBps
	est.ConfidenceBps = est.ExpectedBps + est.StdDevBps
	return est
}

// RecordRealized updates the historical model with an observed slippage and
// emits a SlippageRecorded event. realizedBps is signed positive = adverse.
func (e *SlippageEngine) RecordRealized(ctx context.Context, venue VenueID, symbol, side string, qty, expectedBps, realizedBps float64) {
	key := slippageKey{venue, symbol}

	e.mu.Lock()
	stat, ok := e.stats[key]
	if !ok {
		stat = &slippageStat{EwmaBps: realizedBps, VarBps: 0, Count: 0}
		e.stats[key] = stat
	}
	// EWMA update of mean and variance (West's online EWMA variance).
	prevMean := stat.EwmaBps
	stat.EwmaBps = e.Alpha*realizedBps + (1-e.Alpha)*prevMean
	dev := realizedBps - prevMean
	stat.VarBps = (1-e.Alpha)*(stat.VarBps + e.Alpha*dev*dev)
	stat.LastBps = realizedBps
	stat.Count++
	stat.UpdatedAt = time.Now().UTC()
	e.mu.Unlock()

	shortfall := realizedBps - expectedBps
	if e.store != nil {
		emitVenue(ctx, e.store, venue, EventSlippageRecorded, SlippageRecordedPayload{
			VenueID:                    venue,
			Symbol:                     symbol,
			Side:                       side,
			Quantity:                   qty,
			ExpectedBps:                expectedBps,
			RealizedBps:                realizedBps,
			ImplementationShortfallBps: shortfall,
			RecordedAt:                 time.Now().UTC(),
		})
	}
}

// VenueScore returns a 0–1 slippage quality score (higher = lower slippage).
func (e *SlippageEngine) VenueScore(venue VenueID, symbol string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	stat := e.stats[slippageKey{venue, symbol}]
	if stat == nil || stat.Count == 0 {
		return 0.8 // neutral-optimistic prior
	}
	// Map EWMA slippage to a score: 0 bps → 1.0, 20 bps → ~0.
	return clamp(1.0-stat.EwmaBps/20.0, 0, 1)
}

// Snapshot returns a copy of the historical slippage stats for diagnostics.
func (e *SlippageEngine) Snapshot() map[string]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]float64, len(e.stats))
	for k, v := range e.stats {
		out[fmt.Sprintf("%s:%s", k.Venue, k.Symbol)] = v.EwmaBps
	}
	return out
}
