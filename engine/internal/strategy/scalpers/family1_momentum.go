package scalpers

// buildFamily1Momentum returns S30-S39 (Family 1 — Momentum & Trend
// Following). All 10 strategies are registered at rollout phase 99, which
// exceeds any real STRATEGY_ROLLOUT_PHASE value (1-5), so they ALWAYS
// evaluate but NEVER auto-promote to live trading — see rollout_phase.go.
// Promotion only happens via the explicit ShadowPromoter.Promote() mechanism.
func buildFamily1Momentum() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&DualMomentumTrend{}, 99),
			Name:         "Dual_Momentum_Trend",
			Description:  "S30: absolute momentum (aligned positive/negative return across 5m/15m/1h) confirmed by relative momentum via volume participation (Antonacci dual momentum)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&TimeSeriesMomentumRebalance{}, 99),
			Name:         "Time_Series_Momentum_Rebalance",
			Description:  "S31: rolling-return majority vote across 1h/4h/24h-proxy windows, confidence weighted by inverse realized volatility (Moskowitz/Ooi/Pedersen TSMOM)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h", "4h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&KalmanFilterTrend{}, 99),
			Name:         "Kalman_Filter_Trend",
			Description:  "S32: Kalman-filtered price trend on 15m, noise-gated by 1-ATR divergence check between raw price and filtered estimate",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&AdaptiveMovingAverageTrend{}, 99),
			Name:         "Adaptive_Moving_Average_Trend",
			Description:  "S33: Kaufman KAMA on 15m, trades only when efficiency ratio > 0.6 and KAMA slope is clearly directional",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&TripleBarrierMomentum{}, 99),
			Name:         "Triple_Barrier_Momentum",
			Description:  "S34: EMA9/EMA21 + CVD momentum signal filtered through a simplified Lopez de Prado triple-barrier hit-probability proxy (threshold 0.55)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&FractalBreakout{}, 99),
			Name:         "Fractal_Breakout",
			Description:  "S35: Bill Williams 5-bar fractal high/low breakout on 15m, confirmed by volume > 1.5x 20-period average",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&SuperTrendMomentum{}, 99),
			Name:         "SuperTrend_Momentum",
			Description:  "S36: SuperTrend(10,3) on 15m confirmed by 1h EMA9/EMA21 trend alignment",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&IchimokuCloudBreakout{}, 99),
			Name:         "Ichimoku_Cloud_Breakout",
			Description:  "S37: Ichimoku cloud breakout on 1h confirmed by Tenkan/Kijun alignment",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&DonchianChannelBreakout{}, 99),
			Name:         "Donchian_Channel_Breakout",
			Description:  "S38: Donchian 20-period channel breakout on 1h confirmed by CVD direction (Turtle Trading rules)",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&ParabolicSARTrend{}, 99),
			Name:         "Parabolic_SAR_Trend",
			Description:  "S39: Wilder Parabolic SAR on 15m confirmed by 1h EMA9/EMA21 trend alignment",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
	}
}
