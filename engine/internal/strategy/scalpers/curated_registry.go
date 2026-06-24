package scalpers

import "sync"

// performanceRegistry holds live performance stats for each scalper strategy.
// Updated by UpdatePerformance() after each trade closes.
var (
	perfMu              sync.RWMutex
	performanceRegistry = map[string]Performance{}
)

// UpdatePerformance upserts live performance data for a strategy.
// Called by the trading loop after every TP/SL/TIME exit.
func UpdatePerformance(p Performance) {
	perfMu.Lock()
	performanceRegistry[p.StrategyName] = p
	perfMu.Unlock()
}

// GetPerformance returns the last recorded performance for a strategy.
func GetPerformance(strategyName string) (Performance, bool) {
	perfMu.RLock()
	defer perfMu.RUnlock()
	p, ok := performanceRegistry[strategyName]
	return p, ok
}

// AllPerformance returns a snapshot of all performance records.
func AllPerformance() []Performance {
	perfMu.RLock()
	defer perfMu.RUnlock()
	out := make([]Performance, 0, len(performanceRegistry))
	for _, p := range performanceRegistry {
		out = append(out, p)
	}
	return out
}

// BuildCuratedScalpers returns the active set of scalpers, filtered by
// FilterWinnersOnly. New strategies (no performance record yet) are always
// included so they can accumulate their first 30 trades.
func BuildCuratedScalpers() []RegistryEntry {
	all := BuildAllScalpers()
	expansion := buildExpansionPack()
	all = append(all, expansion...)
	volatility := buildVolatilityFamily()
	all = append(all, volatility...)
	microstructure := buildMicrostructureFamily()
	all = append(all, microstructure...)
	macro := buildMacroFamily()
	all = append(all, macro...)
	statistical := buildStatisticalFamily()
	all = append(all, statistical...)
	event := buildEventFamily()
	all = append(all, event...)

	// S30-S79: 50 shadow-only strategy families (research-backed, phase-99
	// gated — see rollout_phase.go). These ALWAYS evaluate every cycle so
	// they accumulate a real walk-forward track record, but never trade live
	// until an operator explicitly promotes them via STRATEGY_LIVE_OVERRIDE
	// or ShadowPromoter.Promote().
	family1 := buildFamily1Momentum()
	all = append(all, family1...)
	family2 := buildFamily2MeanReversion()
	all = append(all, family2...)
	family3 := buildFamily3OrderFlow()
	all = append(all, family3...)
	family4 := buildFamily4MLProxy()
	all = append(all, family4...)
	family5 := buildFamily5DerivativesMacro()
	all = append(all, family5...)

	// Strategies registered without withRolloutPhase() (the original S1-S10
	// roster) never trade live unless SHADOW_STRATEGIES/STRATEGY_LIVE_OVERRIDE
	// applies. Wrap them so those two operator knobs work uniformly across
	// every strategy in the curated set, not just the phase-gated ones.
	for i, e := range all {
		if _, alreadyGated := e.Strategy.(*phaseGatedStrategy); alreadyGated {
			continue
		}
		all[i].Strategy = withShadowOverride(e.Strategy)
	}

	return FilterWinnersOnly(all)
}

// FilterWinnersOnly removes strategies that have >= 30 trades AND are marked
// inactive (Active=false) in their performance record.
// Strategies with < 30 trades are always kept (probationary period).
func FilterWinnersOnly(entries []RegistryEntry) []RegistryEntry {
	perfMu.RLock()
	defer perfMu.RUnlock()

	out := make([]RegistryEntry, 0, len(entries))
	for _, e := range entries {
		p, exists := performanceRegistry[e.Name]
		if !exists {
			// No history yet — include (probationary)
			out = append(out, e)
			continue
		}
		if p.TotalTrades < 30 {
			// Insufficient data — keep running
			out = append(out, e)
			continue
		}
		if p.Active {
			out = append(out, e)
		}
	}
	return out
}
