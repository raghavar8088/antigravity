package scalpers

// buildEventFamily returns the S26-S29 event-driven/session-based strategy
// family (Family E), merged into BuildCuratedScalpers() alongside
// BuildAllScalpers(), buildExpansionPack(), buildVolatilityFamily(),
// buildMicrostructureFamily(), buildMacroFamily(), and buildStatisticalFamily().
// Follows the same naming/wiring convention as the prior family builders.
//
// No new external data feed is required: S26 derives a synthetic weekend-gap
// signal from existing buffered 1h candle timestamps, S27 derives session
// windows from existing 1h candle timestamps, S28 uses a hardcoded macro
// calendar (package-level var, needs periodic manual update), and S29 uses
// the existing CVD/candle data already in MarketContext plus a fixed daily
// funding-settlement schedule.
//
// Each entry is wrapped with withRolloutPhase() so STRATEGY_ROLLOUT_PHASE
// (see rollout_phase.go) gates activation independently per strategy without
// needing a redeploy: bump the env var to enable later phases.
func buildEventFamily() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&CMEGapFill{}, 1),
			Name:         "CME_Gap_Fill",
			Description:  "S26: synthetic weekend price-discontinuity proxy (Fri ~21:00 UTC close vs Sun ~22:00 UTC reopen, derived from buffered 1h candles); trades toward the historical fill direction (~77% historical fill rate pre-24/7-CME) only with non-contradicting CVD flow; probabilistic, not guaranteed",
			Regimes:      []Regime{RegimeRanging, RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&SessionHandoffMomentum{}, 2),
			Name:         "Session_Handoff_Momentum",
			Description:  "S27: trades WITH a confirmed Asian-session (00:00-08:00 UTC) directional move once London-session candles (08:00 UTC onward) confirm continuation over 3 bars; momentum-continuation counterpart to S5's VWAP fade, not a duplicate",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&MacroCalendarVolPositioning{}, 3),
			Name:         "Macro_Calendar_Vol_Positioning",
			Description:  "S28: reduces entries (heavily discounted confidence, BB-extreme-only) in the 2-4hr pre-FOMC/CPI window (hardcoded 2026 calendar, needs annual update); trades the first clean volume-confirmed directional break 15min-2hr post-release, skipping initial whipsaw",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&FundingResetMeanReversion{}, 4),
			Name:         "Funding_Reset_Mean_Reversion",
			Description:  "S29: fades aggressive, stalling, CVD-unconfirmed price moves within +/-30min of the 8h funding settlement times (00:00/08:00/16:00 UTC), regardless of funding rate magnitude — distinct from S8 which fades on funding RATE extremes over a broader window; documented fallback since Deribit max-pain/expiry OI data is infeasible via simple REST in scope",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"1m", "5m"},
			MaxPositions: 1,
		},
	}
}
