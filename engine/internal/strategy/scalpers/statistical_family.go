package scalpers

// buildStatisticalFamily returns the S22-S25 statistical/quantitative
// pattern strategy family (Family D), merged into BuildCuratedScalpers()
// alongside BuildAllScalpers(), buildExpansionPack(), buildVolatilityFamily(),
// buildMicrostructureFamily(), and buildMacroFamily(). Follows the same
// naming/wiring convention as the prior family builders.
//
// Unlike Family A/B/C, this family requires no new external data feed (S25
// is the partial exception — it reuses the pre-existing dominance.DominanceFetcher
// rather than adding a new feed): all four strategies operate on indicator
// math over candle data already present in MarketContext.
//
// Each entry is wrapped with withRolloutPhase() so STRATEGY_ROLLOUT_PHASE
// (see rollout_phase.go) gates activation independently per strategy without
// needing a redeploy: bump the env var to enable later phases.
func buildStatisticalFamily() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&HurstRegimeSwitcher{}, 1),
			Name:         "Hurst_Regime_Switcher",
			Description:  "S22: 4h Hurst exponent (R/S analysis) self-determines trend-following vs mean-reversion sub-mode; the one documented all-regimes exception since the strategy IS its own regime gate",
			Regimes:      []Regime{RegimeTrending, RegimeRanging, RegimeVolatile, RegimeUnknown},
			Timeframes:   []string{"4h", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&MTFZScoreConfluence{}, 2),
			Name:         "MTF_ZScore_Confluence",
			Description:  "S23: fades statistical extremes only when 5m+15m+1h z-scores (|z|>2.0) align in the same direction and persist for 3 confirmation bars",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"5m", "15m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&VolumeWeightedMomentumFactor{}, 3),
			Name:         "Volume_Weighted_Momentum_Factor",
			Description:  "S24: trades only outlier volume-weighted momentum factor readings (>1.5x trailing-20-bar mean) as a simplified top-decile proxy",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&DominanceRelativeStrength{}, 4),
			Name:         "Dominance_Relative_Strength",
			Description:  "S25: trades BTC's own 1h EMA trend confirmed by BTC.D dominance direction (CoinGecko, reused from existing dominance.DominanceFetcher); degrades to BTC-trend-only at reduced confidence if dominance feed unpopulated",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
	}
}
