package scalpers

// BuildAllScalpers returns every strategy in the scalpers pack as a RegistryEntry.
// Used for backtesting, monitoring dashboards, and the curated selector.
func BuildAllScalpers() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:     &EMARibbonTrendRider{},
			Name:         "EMA_Ribbon_Trend_Rider",
			Description:  "1h ribbon alignment + 15m pullback entry with CVD confirmation",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     &BollingerMeanReversion{},
			Name:         "Bollinger_Mean_Reversion",
			Description:  "Compressed BB on 15m, fade at 5m band extremes with RSI + order book wall",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"5m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     &LiquiditySweepReversal{},
			Name:         "Liquidity_Sweep_Reversal",
			Description:  "Stop-hunt spike through swing level closes back inside; CVD + funding spike",
			Regimes:      []Regime{RegimeVolatile},
			Timeframes:   []string{"1m", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:     &ADXMomentumBreakout{},
			Name:         "ADX_Momentum_Breakout",
			Description:  "4h ADX>25 confirms trend; 15m breakout with volume surge + MACD histogram",
			Regimes:      []Regime{RegimeTrending},
			Timeframes:   []string{"15m", "4h"},
			MaxPositions: 1,
		},
		{
			Strategy:     &VWAPInstitutionalFade{},
			Name:         "VWAP_Institutional_Fade",
			Description:  "Price extends >1.5×ATR from session VWAP; fade back; London/NY only",
			Regimes:      []Regime{RegimeRanging},
			Timeframes:   []string{"5m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     &CVDDivergenceSniper{},
			Name:         "CVD_Divergence_Sniper",
			Description:  "Price-CVD divergence on 5m; fires in all regimes except UNKNOWN",
			Regimes:      []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"5m", "15m"},
			MaxPositions: 1,
		},
		{
			Strategy:     &OpeningRangeBreakout{},
			Name:         "Opening_Range_Breakout",
			Description:  "NY session ORB (14:00-14:30 UTC range), breakout with volume + 1h bias",
			Regimes:      []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:   []string{"5m", "1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     &FundingRateFade{},
			Name:         "Funding_Rate_Fade",
			Description:  "Fade extreme funding rates: short when >0.05%, long when <-0.05%; requires 2 consecutive readings",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"1h"},
			MaxPositions: 1,
		},
		{
			Strategy:     &OIDivergence{},
			Name:         "OI_Divergence",
			Description:  "OI rising+price falling=bearish; OI falling+price rising=bullish/exhaustion",
			Regimes:      []Regime{RegimeTrending, RegimeRanging},
			Timeframes:   []string{"15m", "1h"},
			MaxPositions: 1,
		},
	}
}
