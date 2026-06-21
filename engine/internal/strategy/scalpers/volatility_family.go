package scalpers

// buildVolatilityFamily returns the S10-S13 DVOL/volatility-based strategy
// family, merged into BuildCuratedScalpers() alongside BuildAllScalpers() and
// buildExpansionPack(). This is the first of several planned family builders
// (sibling functions should follow this naming convention, e.g.
// buildMicrostructureFamily(), buildMacroFamily(), buildStatisticalFamily(),
// buildEventDrivenFamily()) — each appended in BuildCuratedScalpers().
//
// Each entry is wrapped with withRolloutPhase() so STRATEGY_ROLLOUT_PHASE
// (see rollout_phase.go) gates activation independently per strategy without
// needing a redeploy: bump the env var to enable later phases.
func buildVolatilityFamily() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&IVRVSpreadReversion{}, 1),
			Name:         "IV_RV_Spread_Reversion",
			Description:  "S10: fades extreme DVOL-vs-RealizedVol spread, biased by funding rate crowding direction (volatility risk premium mean reversion)",
			Regimes:      []Regime{RegimeRanging, RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&VolRegimeBreakout{}, 2),
			Name:         "Vol_Regime_Breakout",
			Description:  "S11: trades continuation breakouts when DVOL breaks its own rolling high alongside accelerating realized vol",
			Regimes:      []Regime{RegimeVolatile, RegimeTrending},
			Timeframes:   []string{"1h", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&VolCrushFade{}, 3),
			Name:         "Vol_Crush_Fade",
			Description:  "S12: boosted-confidence Bollinger mean-reversion during a confirmed post-spike DVOL crush window",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"1h", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&FearGaugeDivergence{}, 4),
			Name:         "Fear_Gauge_Divergence",
			Description:  "S13: DVOL-vs-price divergence fear gauge (3-bar confirmed) — fallback for infeasible DVOL term-structure/skew signal",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
	}
}
