package scalpers

// buildMacroFamily returns the S18-S21 cross-asset/macro strategy family,
// merged into BuildCuratedScalpers() alongside BuildAllScalpers(),
// buildExpansionPack(), buildVolatilityFamily(), and buildMicrostructureFamily().
// Follows the same naming/wiring convention as the prior family builders.
//
// Each entry is wrapped with withRolloutPhase() so STRATEGY_ROLLOUT_PHASE
// (see rollout_phase.go) gates activation independently per strategy without
// needing a redeploy: bump the env var to enable later phases.
func buildMacroFamily() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&BTCEquitiesCorrelationBreak{}, 1),
			Name:         "BTC_Equities_Correlation_Break",
			Description:  "S18: trades BTC with the Nasdaq proxy direction during high-correlation regimes (lag-confirmed), reduces conviction on correlation breakdown",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&DXYInverseMomentum{}, 2),
			Name:         "DXY_Inverse_Momentum",
			Description:  "S19: trades BTC inversely to DXY 20-period rolling high/low breakouts, confirmed by BTC's own EMA9/EMA21 trend not contradicting",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&RiskOnOffRegimeProxy{}, 3),
			Name:         "Risk_On_Off_Regime_Proxy",
			Description:  "S20: composite DXY+Nasdaq+funding risk-on/risk-off regime bias, with EMA9/EMA21+RSI price-action entry trigger",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&MacroDecouplingMomentum{}, 4),
			Name:         "Macro_Decoupling_Momentum",
			Description:  "S21: trades WITH BTC's own strong 1h momentum when equities-proxy/DXY are flat (idiosyncratic crypto-specific catalyst, not macro-driven)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
	}
}
