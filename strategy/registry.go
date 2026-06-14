package strategy

// ── registry.go ──────────────────────────────────────────────────────────────
// This file replaces the empty stub that caused the P0 bug.
// BuildAllScalpers() returns all 7 strategies, ready for the trading loop.

// BuildAllScalpers returns every registered strategy.
// Called by the trading loop and backtest engine.
func BuildAllScalpers() []RegistryEntry {
	return []RegistryEntry{
		{
			Strategy:    &EMARibbonTrendRider{},
			Name:        "EMA_Ribbon_Trend_Rider",
			Description: "1h ribbon alignment + 15m EMA21 pullback with CVD confirmation. TRENDING only.",
			Regimes:     []Regime{RegimeTrending},
			Timeframes:  []string{"15m", "1h"},
			MaxPositions: 2,
		},
		{
			Strategy:    &BollingerMeanReversion{},
			Name:        "Bollinger_Mean_Reversion",
			Description: "BB band touch + RSI oversold/overbought + order book wall. RANGING only.",
			Regimes:     []Regime{RegimeRanging},
			Timeframes:  []string{"5m", "15m"},
			MaxPositions: 2,
		},
		{
			Strategy:    &LiquiditySweepReversal{},
			Name:        "Liquidity_Sweep_Reversal",
			Description: "Stop hunt spike through swing level + CVD divergence + funding spike. VOLATILE only.",
			Regimes:     []Regime{RegimeVolatile},
			Timeframes:  []string{"1m", "5m"},
			MaxPositions: 1,
		},
		{
			Strategy:    &ADXMomentumBreakout{},
			Name:        "ADX_Momentum_Breakout",
			Description: "4h ADX>25 rising + 15m range breakout with volume surge + CVD confirm. TRENDING only.",
			Regimes:     []Regime{RegimeTrending},
			Timeframes:  []string{"15m", "4h"},
			MaxPositions: 2,
		},
		{
			Strategy:    &VWAPInstitutionalFade{},
			Name:        "VWAP_Institutional_Fade",
			Description: "Price >1.5% from VWAP + RSI divergence + order book wall. London+NY sessions. RANGING only.",
			Regimes:     []Regime{RegimeRanging},
			Timeframes:  []string{"5m", "15m"},
			MaxPositions: 2,
		},
		{
			Strategy:    &CVDDivergenceSniper{},
			Name:        "CVD_Divergence_Sniper",
			Description: "Price new high/low without CVD confirmation + MACD flip + OB imbalance. ALL regimes.",
			Regimes:     []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:  []string{"5m", "15m"},
			MaxPositions: 3,
		},
		{
			Strategy:    &OpeningRangeBreakout{},
			Name:        "Opening_Range_Breakout",
			Description: "NY open 30m range, breakout with 2× volume + 1h EMA alignment. 14:00–20:00 UTC.",
			Regimes:     []Regime{RegimeTrending, RegimeRanging, RegimeVolatile},
			Timeframes:  []string{"5m", "1h"},
			MaxPositions: 2,
		},
	}
}
