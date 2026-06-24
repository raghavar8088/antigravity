package scalpers

// buildFamily3OrderFlow returns S50-S59 (Family 3 — Order Flow & Market
// Microstructure). All 10 strategies are registered at rollout phase 99,
// which exceeds any real STRATEGY_ROLLOUT_PHASE value (1-5), so they ALWAYS
// evaluate but NEVER auto-promote to live trading — see rollout_phase.go.
// Promotion only happens via the explicit ShadowPromoter.Promote() mechanism.
func buildFamily3OrderFlow() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&TradeIntensityBurst{}, 99),
			Name:         "Trade_Intensity_Burst",
			Description:  "S50: trades a 3-bar-confirmed directional burst in trade-arrival intensity (Easley & O'Hara 1992); falls back to a 1m volume-spike proxy when MicrostructurePopulated is false",
			Regimes:      []Regime{RegimeTrending, RegimeVolatile},
			Timeframes:   []string{"1m", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&LargeTradeImpactFollow{}, 99),
			Name:         "Large_Trade_Impact_Follow",
			Description:  "S51: follows large single-trade price impact (Kyle 1985) in the direction of the 5m EMA9/EMA21 trend with a tight stop; proxy uses a 1m volume-spike + strong-close candle when the live trade-size feed is unavailable",
			Regimes:      []Regime{RegimeTrending, RegimeVolatile},
			Timeframes:   []string{"1m", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&BidAskSpreadRegime{}, 99),
			Name:         "Bid_Ask_Spread_Regime",
			Description:  "S52: trades order-book imbalance as a spread-widening proxy (Glosten & Milgrom 1985) when the live effective-spread feed is unavailable, falling back to OrderBook.Imbalance directional reasoning",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"1m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&VPINToxicitySignal{}, 99),
			Name:         "VPIN_Toxicity_Signal",
			Description:  "S53: simplified CVD-bucket VPIN proxy (Easley, Lopez de Prado & O'Hara 2012) — trades the direction of toxic (>0.7) one-sided CVD bucket delta once 3-bar confirmed",
			Regimes:      []Regime{RegimeTrending, RegimeVolatile},
			Timeframes:   []string{"1m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&OrderBookDepthImbalancePersistence{}, 99),
			Name:         "Order_Book_Depth_Imbalance_Persistence",
			Description:  "S54: depth-tier-weighted order book imbalance (Cartea, Jaimungal & Penalva 2015) via WeightedOBImbalance(), distinct from S17's raw-Imbalance approach; persistence approximated via CVDHistory direction-stability since no multi-snapshot history is stored",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"1m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&AbsorptionPatternSignal{}, 99),
			Name:         "Absorption_Pattern_Signal",
			Description:  "S55: Market Profile absorption pattern (Steidlmayer 1984) — heavy one-sided CVD flow with disproportionately small realized price move signals passive absorption and a likely reversal",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"5m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&SweepAndReloadPattern{}, 99),
			Name:         "Sweep_And_Reload_Pattern",
			Description:  "S56: single-candle-wick stop-hunt pattern (institutional desk training materials) with next-candle reload confirmation and CVD-opposes-wick absorption check; distinct single-candle logic from S3's swing-level sweep",
			Regimes:      []Regime{RegimeVolatile},
			Timeframes:   []string{"5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&MarketDepthGradient{}, 99),
			Name:         "Market_Depth_Gradient",
			Description:  "S57: order book depth-tier gradient/steepness (Cont, Kukanov & Stoikov 2014) — trades acceleration through the thinner/steeper side when CVD flow is pushing into it; requires populated order book",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"1m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&AggressorRatioMomentum{}, 99),
			Name:         "Aggressor_Ratio_Momentum",
			Description:  "S58: sustained aggressor-side dominance momentum (Hasbrouck & Saar 2002) confirmed by price holding above/below 5m EMA20; proxy derives an aggressor-ratio-like measure from CVDHistory step direction when the live feed is unavailable",
			Regimes:      []Regime{RegimeTrending, RegimeVolatile},
			Timeframes:   []string{"5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&HiddenLiquidityDetection{}, 99),
			Name:         "Hidden_Liquidity_Detection",
			Description:  "S59: trades proven support/resistance levels touched and bounced 3+ times within the last ~1hr of 5m bars (Bessembinder, Panayides & Venkataraman 2009 hidden-liquidity theory)",
			Regimes:      []Regime{RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"5m"},
			MaxPositions: 1,
		},
	}
}
