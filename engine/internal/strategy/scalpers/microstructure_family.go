package scalpers

// buildMicrostructureFamily returns the S14-S17 liquidation/OI-funding/basis/
// orderbook microstructure strategy family, merged into BuildCuratedScalpers()
// alongside BuildAllScalpers(), buildExpansionPack(), and buildVolatilityFamily().
// Follows the same naming/wiring convention as buildVolatilityFamily().
//
// Each entry is wrapped with withRolloutPhase() so STRATEGY_ROLLOUT_PHASE
// (see rollout_phase.go) gates activation independently per strategy without
// needing a redeploy: bump the env var to enable later phases.
func buildMicrostructureFamily() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&LiquidationCascadeFade{}, 1),
			Name:         "Liquidation_Cascade_Fade",
			Description:  "S14: fades BTC perpetual liquidation cascades once liquidation velocity decelerates from a >3x trailing-1h-avg spike",
			Regimes:      []Regime{RegimeVolatile},
			Timeframes:   []string{"1m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&OIFundingCrowdingComposite{}, 2),
			Name:         "OI_Funding_Crowding_Composite",
			Description:  "S15: higher-conviction contrarian fade requiring BOTH OI extremity AND funding extremity simultaneously (differentiated from S9/S8 single-leg signals)",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&PerpSpotBasisMomentum{}, 3),
			Name:         "Perp_Spot_Basis_Momentum",
			Description:  "S16: trades WITH momentum when Binance perp premium over Coinbase spot widens beyond what funding explains, confirmed by rising OI",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&OrderbookImbalancePersistence{}, 4),
			Name:         "Orderbook_Imbalance_Persistence",
			Description:  "S17: resting bid/ask wall imbalance confirmed via multi-level depth + trade-flow agreement (persistence proxy); never fires without a populated order book",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"1m", "15m"},
			MaxPositions: 1,
		},
	}
}
