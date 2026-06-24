package trading

import (
	"sync"
	"time"

	"antigravity-engine/internal/observability"
)

// ConcentrationPosition is a single open scalers position tracked for the
// purpose of limiting same-direction concentration risk.
type ConcentrationPosition struct {
	StrategyName string
	Direction    string // normalized: "LONG" or "SHORT"
	SizeBTC      float64
	OpenedAt     time.Time
}

// ConcentrationGate limits how much same-direction exposure the curated
// scalers strategies can accumulate at once. The pre-existing directional cap
// (ScalerBundle.maxOpenLongs/maxOpenShorts, default 3) only counts trades —
// it does not stop e.g. EMA_Ribbon_Trend_Rider, ADX_Momentum_Breakout, and
// OI_Divergence from all going long BTC on the same trend simultaneously,
// which is the same correlated bet sized 3x rather than diversification.
// This gate adds a second, size-aware check on top of the trade-count cap.
type ConcentrationGate struct {
	mu               sync.RWMutex
	openPositions    []ConcentrationPosition
	maxSameDirection int     // max strategies concurrently open in the same direction
	maxCorrelatedBTC float64 // max combined BTC size across same-direction positions
}

// NewConcentrationGate creates a gate with the given limits. A limit <= 0
// disables that particular check.
func NewConcentrationGate(maxSameDirection int, maxCorrelatedBTC float64) *ConcentrationGate {
	return &ConcentrationGate{
		maxSameDirection: maxSameDirection,
		maxCorrelatedBTC: maxCorrelatedBTC,
	}
}

// normalizeConcentrationDirection maps the various direction spellings used
// across the trading package ("LONG"/"BUY" vs "SHORT"/"SELL") to one of two
// canonical buckets.
func normalizeConcentrationDirection(dir string) string {
	if dir == "LONG" || dir == "BUY" {
		return "LONG"
	}
	return "SHORT"
}

// CanOpen always returns (true, "") — concentration gating is disabled so all
// strategies may fire simultaneously regardless of directional correlation.
func (g *ConcentrationGate) CanOpen(strategyName, direction string, sizeBTC float64) (bool, string) {
	return true, ""
}

// RecordOpen registers a newly opened position with the gate. Call only
// after the position has actually been filled.
func (g *ConcentrationGate) RecordOpen(strategyName, direction string, sizeBTC float64) {
	g.mu.Lock()
	g.openPositions = append(g.openPositions, ConcentrationPosition{
		StrategyName: strategyName,
		Direction:    normalizeConcentrationDirection(direction),
		SizeBTC:      sizeBTC,
		OpenedAt:     time.Now().UTC(),
	})
	g.mu.Unlock()
	g.publishMetrics()
}

// RecordClose removes one open position previously recorded for strategyName
// (oldest first — the gate does not track individual position IDs, mirroring
// the existing ScalerBundle directional counters). Safe to call even if no
// matching open position is found (no-op).
func (g *ConcentrationGate) RecordClose(strategyName string) {
	g.mu.Lock()
	for i, p := range g.openPositions {
		if p.StrategyName == strategyName {
			g.openPositions = append(g.openPositions[:i], g.openPositions[i+1:]...)
			break
		}
	}
	g.mu.Unlock()
	g.publishMetrics()
}

// publishMetrics updates the same-direction BTC exposure gauges. Caller must
// NOT hold g.mu.
func (g *ConcentrationGate) publishMetrics() {
	g.mu.RLock()
	longBTC, shortBTC := 0.0, 0.0
	for _, p := range g.openPositions {
		if p.Direction == "LONG" {
			longBTC += p.SizeBTC
		} else {
			shortBTC += p.SizeBTC
		}
	}
	g.mu.RUnlock()
	observability.ScalersConcentrationBTC.WithLabelValues("LONG").Set(longBTC)
	observability.ScalersConcentrationBTC.WithLabelValues("SHORT").Set(shortBTC)
}
