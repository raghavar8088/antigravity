package scalpers

// buildFamily2MeanReversion returns S40-S49 (Family 2 — Mean Reversion &
// Statistical Arbitrage). All 10 strategies are registered at rollout phase
// 99, which exceeds any real STRATEGY_ROLLOUT_PHASE value (1-5), so they
// ALWAYS evaluate but NEVER auto-promote to live trading — see
// rollout_phase.go. Promotion only happens via the explicit
// ShadowPromoter.Promote() mechanism.
func buildFamily2MeanReversion() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     withRolloutPhase(&OrnsteinUhlenbeckReversion{}, 99),
			Name:         "Ornstein_Uhlenbeck_Reversion",
			Description:  "S40: fades price deviation from rolling VWAP when an OU fit confirms theta>0 mean reversion (Vasicek 1977; Avellaneda & Lee 2010)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&CointegrationSpreadBTCETH{}, 99),
			Name:         "Cointegration_Spread_BTC_ETH",
			Description:  "S41: z-score fade of the BTC/ETH price ratio vs its approximated rolling mean/stdev (Engle & Granger 1987 cointegration); requires ETHPrice feed",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&RSIDivergenceMultiframe{}, 99),
			Name:         "RSI_Divergence_Multiframe",
			Description:  "S42: 3-bar confirmed RSI-vs-price divergence required simultaneously on both 5m and 15m (Wilder 1978)",
			Regimes:      []Regime{RegimeRanging, RegimeTrending},
			Timeframes:   []string{"5m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&StochasticOscillatorReversion{}, 99),
			Name:         "Stochastic_Oscillator_Reversion",
			Description:  "S43: Slow Stochastic(14,3) %K/%D oversold/overbought cross on 15m, confirmed by ADX<20 (Lane 1984)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&WilliamsPercentRReversion{}, 99),
			Name:         "Williams_Percent_R_Reversion",
			Description:  "S44: Williams %R held in oversold/overbought for 2+ bars on 5m then reverting through -50 midline (Williams 1979)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&CommodityChannelIndexReversion{}, 99),
			Name:         "Commodity_Channel_Index_Reversion",
			Description:  "S45: CCI(20) sustained beyond +-100 for >=3 bars on 15m then starting to revert toward zero (Lambert 1980)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&PriceChannelMeanReversion{}, 99),
			Name:         "Price_Channel_Mean_Reversion",
			Description:  "S46: ATR-modified Keltner Channel (EMA20+-2ATR14) on 15m, fades a close-outside-then-revert-inside 2-bar pattern (Keltner 1960 / Raschke)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&LinearRegressionChannelFade{}, 99),
			Name:         "Linear_Regression_Channel_Fade",
			Description:  "S47: linear regression channel (period 30) ValueAtEnd+-2*StdErr on 15m, fades a close-outside-then-revert-inside pattern (Murphy 1999)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&PivotPointReversion{}, 99),
			Name:         "Pivot_Point_Reversion",
			Description:  "S48: classic floor pivots (PivotPoints) using the latest 1h candle as a prior-period OHLC proxy, fades R1/R2/S1/S2 touches confirmed by a 5m reversal candle (classic floor-trader formula)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"1h", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     withRolloutPhase(&VolatilityMeanReversion{}, 99),
			Name:         "Volatility_Mean_Reversion",
			Description:  "S49: fades price when DVOL (or RealizedVol(1h) fallback) deviates >2 std devs above its own rolling mean, tight-range Keltner-style entry on 15m (Schwert 1989)",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
	}
}
