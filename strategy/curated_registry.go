package strategy

// ── curated_registry.go ──────────────────────────────────────────────────────
// Replaces the P0 stub that returned nil everywhere.
// BuildCuratedScalpers() is called by main.go.
// FilterWinnersOnly() promotes/demotes based on real performance data.

// BuildCuratedScalpers returns the curated strategy list.
// This is the function called by main.go:
//   allStrategies := strategy.BuildCuratedScalpers()
func BuildCuratedScalpers() []RegistryEntry {
	all := BuildAllScalpers()
	return FilterWinnersOnly(all)
}

// FilterWinnersOnly filters strategies based on live performance.
//
// Rules:
//   - Fewer than MinTradesForFilter closed trades → always keep (insufficient data)
//   - Win rate < WinRateThreshold → demote (remove from active rotation)
//   - Sharpe ratio < SharpeThreshold AND more than MinTradesForFilter trades → demote
//   - Max drawdown > MaxDrawdownThreshold → demote regardless of win rate
//
// Performance data is injected by the trading loop via UpdatePerformance().
// On first boot (no performance data) all strategies pass through unchanged.
func FilterWinnersOnly(entries []RegistryEntry) []RegistryEntry {
	const (
		MinTradesForFilter    = 30    // insufficient history guard
		WinRateThreshold      = 0.45  // minimum 45% win rate to stay active
		SharpeThreshold       = 0.50  // minimum Sharpe ratio
		MaxDrawdownThreshold  = 0.25  // maximum 25% drawdown before demotion
	)

	// No performance data yet — pass all strategies through unchanged
	if len(performanceRegistry) == 0 {
		return entries
	}

	var curated []RegistryEntry
	for _, e := range entries {
		perf, exists := performanceRegistry[e.Name]
		if !exists {
			// New strategy, no history yet — include it
			curated = append(curated, e)
			continue
		}
		if perf.TotalTrades < MinTradesForFilter {
			// Not enough trades to judge — keep it
			curated = append(curated, e)
			continue
		}
		// Demotion checks
		if perf.WinRate < WinRateThreshold {
			continue // demoted: win rate too low
		}
		if perf.SharpeRatio < SharpeThreshold {
			continue // demoted: Sharpe too low
		}
		if perf.MaxDrawdown > MaxDrawdownThreshold {
			continue // demoted: drawdown too high
		}
		curated = append(curated, e)
	}

	// Safety: if ALL strategies get demoted, return the full set
	// (prevents the P0 bug recurring — an empty registry is always wrong)
	if len(curated) == 0 {
		return entries
	}

	return curated
}

// performanceRegistry holds live performance data injected by the trading loop.
// Key: strategy name. Value: Performance struct.
var performanceRegistry = map[string]Performance{}

// UpdatePerformance is called by the trading loop after each trade closes.
// It updates the live performance data used by FilterWinnersOnly.
func UpdatePerformance(p Performance) {
	performanceRegistry[p.StrategyName] = p
}

// GetPerformance returns the current performance snapshot for a strategy.
// Returns zero value and false if no data exists yet.
func GetPerformance(strategyName string) (Performance, bool) {
	p, ok := performanceRegistry[strategyName]
	return p, ok
}

// AllPerformance returns a copy of the full performance registry.
// Used by the frontend API to populate the strategy performance dashboard.
func AllPerformance() map[string]Performance {
	out := make(map[string]Performance, len(performanceRegistry))
	for k, v := range performanceRegistry {
		out[k] = v
	}
	return out
}
