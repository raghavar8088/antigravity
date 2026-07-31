package options_selling

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.020
)

var strategyIDs = map[string]int{}

type sellDef struct {
	id       int
	name     string
	category string
	typ      OptionType
	strike   float64
	expiry   int
	tp       float64
	sl       float64
	posUSD   float64
	signal   string
	cooldown int
}

func (d sellDef) toStrategyDef() StrategyDef {
	return StrategyDef{
		ID:            d.id,
		Name:          d.name,
		Category:      d.category,
		Type:          d.typ,
		StrikePctOTM:  d.strike,
		ExpiryMinutes: d.expiry,
		TakeProfitPct: d.tp,
		StopLossPct:   d.sl,
		PositionUSD:   d.posUSD,
		Signal:        d.signal,
		CooldownSecs:  d.cooldown,
	}
}

// buildAllStrategies defines 50 BTC option-selling strategies across
// scalp (~18), intraday (~20) and swing (~12) holding periods.
// Bullish signals sell PUTs (profit if price holds above strike); bearish
// signals sell CALLs (profit if price holds below strike). Every Signal
// string below is a verified key in the Signals map (signals.go).
func buildAllStrategies() []StrategyDef {
	defs := []sellDef{
		// ── Scalping (18): 60-90m expiry, tight TP/SL, short cooldown ──────────
		{1, "Scalp_PutSell_BullMomentum_60m", "Momentum", Put, 0.006, 60, 0.40, 0.70, 150, "BULL_MOMENTUM", 90},
		{2, "Scalp_CallSell_BearMomentum_60m", "Momentum", Call, 0.006, 60, 0.40, 0.70, 150, "BEAR_MOMENTUM", 90},
		{3, "Scalp_PutSell_StrongBullMomentum_75m", "Momentum", Put, 0.008, 75, 0.45, 0.75, 160, "STRONG_BULL_MOMENTUM", 120},
		{4, "Scalp_CallSell_StrongBearMomentum_75m", "Momentum", Call, 0.008, 75, 0.45, 0.75, 160, "STRONG_BEAR_MOMENTUM", 120},
		{5, "Scalp_PutSell_RSIOversold_60m", "Mean Reversion", Put, 0.005, 60, 0.35, 0.60, 150, "RSI_OVERSOLD", 60},
		{6, "Scalp_CallSell_RSIOverbought_60m", "Mean Reversion", Call, 0.005, 60, 0.35, 0.60, 150, "RSI_OVERBOUGHT", 60},
		{7, "Scalp_PutSell_EMABullCross_75m", "Breakout", Put, 0.007, 75, 0.42, 0.72, 160, "EMA_BULL_CROSS", 100},
		{8, "Scalp_CallSell_EMABearCross_75m", "Breakout", Call, 0.007, 75, 0.42, 0.72, 160, "EMA_BEAR_CROSS", 100},
		{9, "Scalp_PutSell_BBLowerTouch_60m", "Mean Reversion", Put, 0.004, 60, 0.35, 0.60, 150, "BB_LOWER_TOUCH", 60},
		{10, "Scalp_CallSell_BBUpperTouch_60m", "Mean Reversion", Call, 0.004, 60, 0.35, 0.60, 150, "BB_UPPER_TOUCH", 60},
		{11, "Scalp_PutSell_BBSqueezeBull_90m", "Breakout", Put, 0.010, 90, 0.50, 0.85, 170, "BB_SQUEEZE_BULL", 150},
		{12, "Scalp_CallSell_BBSqueezeBear_90m", "Breakout", Call, 0.010, 90, 0.50, 0.85, 170, "BB_SQUEEZE_BEAR", 150},
		{13, "Scalp_PutSell_ResistanceBreak_75m", "Breakout", Put, 0.009, 75, 0.48, 0.80, 165, "RESISTANCE_BREAK", 120},
		{14, "Scalp_CallSell_SupportBreak_75m", "Breakout", Call, 0.009, 75, 0.48, 0.80, 165, "SUPPORT_BREAK", 120},
		{15, "Scalp_PutSell_StochOversold_60m", "Mean Reversion", Put, 0.005, 60, 0.35, 0.65, 150, "STOCH_OVERSOLD", 75},
		{16, "Scalp_CallSell_StochOverbought_60m", "Mean Reversion", Call, 0.005, 60, 0.35, 0.65, 150, "STOCH_OVERBOUGHT", 75},
		{17, "Scalp_PutSell_ConsecBullBars_60m", "Momentum", Put, 0.006, 60, 0.38, 0.68, 150, "CONSEC_BULL_BARS", 90},
		{18, "Scalp_CallSell_ConsecBearBars_60m", "Momentum", Call, 0.006, 60, 0.38, 0.68, 150, "CONSEC_BEAR_BARS", 90},

		// ── Intraday (20): 120-360m expiry, moderate TP/SL, medium cooldown ────
		{19, "Intraday_PutSell_RSIOversoldExtreme_150m", "Mean Reversion", Put, 0.011, 150, 0.55, 0.95, 220, "RSI_OVERSOLD_EXTREME", 240},
		{20, "Intraday_CallSell_RSIOverboughtExtreme_150m", "Mean Reversion", Call, 0.011, 150, 0.55, 0.95, 220, "RSI_OVERBOUGHT_EXTREME", 240},
		{21, "Intraday_PutSell_EMAAboveBoth_180m", "Breakout", Put, 0.012, 180, 0.55, 1.00, 230, "EMA_ABOVE_BOTH", 300},
		{22, "Intraday_CallSell_EMABelowBoth_180m", "Breakout", Call, 0.012, 180, 0.55, 1.00, 230, "EMA_BELOW_BOTH", 300},
		{23, "Intraday_PutSell_VWAPAbove_150m", "Momentum", Put, 0.010, 150, 0.50, 0.90, 210, "VWAP_ABOVE", 240},
		{24, "Intraday_CallSell_VWAPBelow_150m", "Momentum", Call, 0.010, 150, 0.50, 0.90, 210, "VWAP_BELOW", 240},
		{25, "Intraday_PutSell_TripleBull_180m", "Hybrid", Put, 0.013, 180, 0.60, 1.05, 240, "TRIPLE_BULL", 360},
		{26, "Intraday_CallSell_TripleBear_180m", "Hybrid", Call, 0.013, 180, 0.60, 1.05, 240, "TRIPLE_BEAR", 360},
		{27, "Intraday_PutSell_HighIVBull_120m", "Hybrid", Put, 0.009, 120, 0.48, 0.85, 200, "HIGH_IV_BULL", 180},
		{28, "Intraday_CallSell_HighIVBear_120m", "Hybrid", Call, 0.009, 120, 0.48, 0.85, 200, "HIGH_IV_BEAR", 180},
		{29, "Intraday_PutSell_SharpReversalUp_150m", "Capitulation", Put, 0.010, 150, 0.52, 0.92, 215, "SHARP_REVERSAL_UP", 240},
		{30, "Intraday_CallSell_SharpReversalDown_150m", "Capitulation", Call, 0.010, 150, 0.52, 0.92, 215, "SHARP_REVERSAL_DOWN", 240},
		{31, "Intraday_PutSell_MomentumVWAPBull_210m", "Momentum", Put, 0.014, 210, 0.58, 1.02, 250, "MOMENTUM_VWAP_BULL", 300},
		{32, "Intraday_CallSell_MomentumVWAPBear_210m", "Momentum", Call, 0.014, 210, 0.58, 1.02, 250, "MOMENTUM_VWAP_BEAR", 300},
		{33, "Intraday_PutSell_BreakoutTrendBull_240m", "Breakout", Put, 0.015, 240, 0.60, 1.10, 260, "BREAKOUT_TREND_BULL", 360},
		{34, "Intraday_CallSell_BreakdownTrendBear_240m", "Breakout", Call, 0.015, 240, 0.60, 1.10, 260, "BREAKDOWN_TREND_BEAR", 360},
		{35, "Intraday_PutSell_CapitulationReclaim_180m", "Capitulation", Put, 0.012, 180, 0.55, 1.00, 230, "CAPITULATION_RECLAIM", 300},
		{36, "Intraday_CallSell_CapitulationRecovery_180m", "Capitulation", Put, 0.012, 180, 0.55, 1.00, 230, "CAPITULATION_RECOVERY", 300},
		{37, "Intraday_PutSell_MACDBullCross_210m", "Momentum", Put, 0.013, 210, 0.56, 1.00, 245, "MACD_BULL_CROSS", 300},
		{38, "Intraday_CallSell_MACDBearCross_210m", "Momentum", Call, 0.013, 210, 0.56, 1.00, 245, "MACD_BEAR_CROSS", 300},

		// ── Swing (12): 480-1440m expiry, wider TP/SL, long cooldown ────────────
		{39, "Swing_PutSell_VolCompressBull_480m", "Breakout", Put, 0.016, 480, 0.60, 1.10, 320, "VOL_COMPRESS_BULL", 600},
		{40, "Swing_CallSell_VolCompressBear_480m", "Breakout", Call, 0.016, 480, 0.60, 1.10, 320, "VOL_COMPRESS_BEAR", 600},
		{41, "Swing_PutSell_SessionOpenBull_540m", "Hybrid", Put, 0.014, 540, 0.55, 1.05, 300, "SESSION_OPEN_BULL", 720},
		{42, "Swing_CallSell_SessionOpenBear_540m", "Hybrid", Call, 0.014, 540, 0.55, 1.05, 300, "SESSION_OPEN_BEAR", 720},
		{43, "Swing_PutSell_OverextensionFadeUp_600m", "Mean Reversion", Put, 0.018, 600, 0.65, 1.30, 350, "OVEREXTENSION_FADE_UP", 900},
		{44, "Swing_CallSell_OverextensionFadeDown_600m", "Mean Reversion", Call, 0.018, 600, 0.65, 1.30, 350, "OVEREXTENSION_FADE_DOWN", 900},
		{45, "Swing_PutSell_ATRExpandBull_720m", "Momentum", Put, 0.017, 720, 0.62, 1.20, 340, "ATR_EXPAND_BULL", 1000},
		{46, "Swing_CallSell_ATRExpandBear_720m", "Momentum", Call, 0.017, 720, 0.62, 1.20, 340, "ATR_EXPAND_BEAR", 1000},
		{47, "Swing_PutSell_BreakoutTrendBull_960m", "Breakout", Put, 0.019, 960, 0.68, 1.40, 380, "BREAKOUT_TREND_BULL", 1300},
		{48, "Swing_CallSell_BreakdownTrendBear_960m", "Breakout", Call, 0.019, 960, 0.68, 1.40, 380, "BREAKDOWN_TREND_BEAR", 1300},
		{49, "Swing_PutSell_CapitulationReclaim_1440m", "Capitulation", Put, 0.020, 1440, 0.72, 1.55, 400, "CAPITULATION_RECLAIM", 1800},
		{50, "Swing_CallSell_TripleBear_1440m", "Hybrid", Call, 0.020, 1440, 0.75, 1.80, 400, "TRIPLE_BEAR", 1800},
	}

	out := make([]StrategyDef, 0, len(defs))
	for _, d := range defs {
		strategyIDs[d.name] = d.id
		out = append(out, d.toStrategyDef())
	}
	return out
}

// BuildStrategies returns the full 50-strategy BTC option-selling roster.
func BuildStrategies() []StrategyDef {
	defs := buildAllStrategies()
	// Every strategy also runs as its mirror: same signal and strike selection,
	// opposite side, stop and target swapped. The pair answers a question the
	// original alone cannot — whether a losing strategy loses because its edge is
	// genuinely negative (mirror wins) or because fees eat a positive edge
	// (mirror loses too, since both sides pay). Disable with ANTI_STRATEGIES=false.
	if !antiStrategiesEnabled() {
		return defs
	}
	return WithAntiStrategies(defs)
}
