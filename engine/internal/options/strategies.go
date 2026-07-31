package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.020
)

var strategyIDs = map[string]int{}

type buyDef struct {
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

func (d buyDef) toStrategyDef() StrategyDef {
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

// buildAllStrategies defines 50 BTC option-buying (long premium) strategies
// across scalp (~18), intraday (~20) and swing (~12) holding periods.
// Bullish signals BUY CALLs (profit if price rises above strike + premium);
// bearish signals BUY PUTs (profit if price falls below strike - premium).
// TakeProfitPct is the premium GAIN fraction that exits a winner;
// StopLossPct is the premium LOSS fraction that cuts a loser (long options
// decay with time, so losers are cut fast). Every Signal string below is a
// verified key in the Signals map (signals.go).
func buildAllStrategies() []StrategyDef {
	defs := []buyDef{
		// ── Scalping (18): 60-75m expiry, aggressive TP, tight SL, short cooldown ──
		{1, "Scalp_CallBuy_BullMomentum_60m", "Momentum", Call, 0.006, 60, 0.55, 0.35, 120, "BULL_MOMENTUM", 90},
		{2, "Scalp_PutBuy_BearMomentum_60m", "Momentum", Put, 0.006, 60, 0.55, 0.35, 120, "BEAR_MOMENTUM", 90},
		{3, "Scalp_CallBuy_StrongBullMomentum_60m", "Momentum", Call, 0.008, 60, 0.65, 0.40, 130, "STRONG_BULL_MOMENTUM", 100},
		{4, "Scalp_PutBuy_StrongBearMomentum_60m", "Momentum", Put, 0.008, 60, 0.65, 0.40, 130, "STRONG_BEAR_MOMENTUM", 100},
		{5, "Scalp_CallBuy_RSIOversold_60m", "Mean Reversion", Call, 0.005, 60, 0.45, 0.32, 110, "RSI_OVERSOLD", 60},
		{6, "Scalp_PutBuy_RSIOverbought_60m", "Mean Reversion", Put, 0.005, 60, 0.45, 0.32, 110, "RSI_OVERBOUGHT", 60},
		{7, "Scalp_CallBuy_EMABullCross_60m", "Breakout", Call, 0.007, 60, 0.58, 0.38, 130, "EMA_BULL_CROSS", 100},
		{8, "Scalp_PutBuy_EMABearCross_60m", "Breakout", Put, 0.007, 60, 0.58, 0.38, 130, "EMA_BEAR_CROSS", 100},
		{9, "Scalp_CallBuy_BBLowerTouch_60m", "Mean Reversion", Call, 0.004, 60, 0.42, 0.30, 110, "BB_LOWER_TOUCH", 60},
		{10, "Scalp_PutBuy_BBUpperTouch_60m", "Mean Reversion", Put, 0.004, 60, 0.42, 0.30, 110, "BB_UPPER_TOUCH", 60},
		{11, "Scalp_CallBuy_BBSqueezeBull_75m", "Breakout", Call, 0.010, 75, 0.70, 0.45, 140, "BB_SQUEEZE_BULL", 150},
		{12, "Scalp_PutBuy_BBSqueezeBear_75m", "Breakout", Put, 0.010, 75, 0.70, 0.45, 140, "BB_SQUEEZE_BEAR", 150},
		{13, "Scalp_CallBuy_ResistanceBreak_60m", "Breakout", Call, 0.009, 60, 0.65, 0.42, 135, "RESISTANCE_BREAK", 120},
		{14, "Scalp_PutBuy_SupportBreak_60m", "Breakout", Put, 0.009, 60, 0.65, 0.42, 135, "SUPPORT_BREAK", 120},
		{15, "Scalp_CallBuy_StochOversold_60m", "Mean Reversion", Call, 0.005, 60, 0.48, 0.34, 110, "STOCH_OVERSOLD", 75},
		{16, "Scalp_PutBuy_StochOverbought_60m", "Mean Reversion", Put, 0.005, 60, 0.48, 0.34, 110, "STOCH_OVERBOUGHT", 75},
		{17, "Scalp_CallBuy_ConsecBullBars_60m", "Momentum", Call, 0.006, 60, 0.50, 0.35, 120, "CONSEC_BULL_BARS", 90},
		{18, "Scalp_PutBuy_ConsecBearBars_60m", "Momentum", Put, 0.006, 60, 0.50, 0.35, 120, "CONSEC_BEAR_BARS", 90},

		// ── Intraday (20): 120-360m expiry, moderate TP/SL, medium cooldown ────
		{19, "Intraday_CallBuy_RSIOversoldExtreme_150m", "Mean Reversion", Call, 0.011, 150, 0.65, 0.40, 170, "RSI_OVERSOLD_EXTREME", 240},
		{20, "Intraday_PutBuy_RSIOverboughtExtreme_150m", "Mean Reversion", Put, 0.011, 150, 0.65, 0.40, 170, "RSI_OVERBOUGHT_EXTREME", 240},
		{21, "Intraday_CallBuy_EMAAboveBoth_180m", "Breakout", Call, 0.012, 180, 0.70, 0.45, 180, "EMA_ABOVE_BOTH", 300},
		{22, "Intraday_PutBuy_EMABelowBoth_180m", "Breakout", Put, 0.012, 180, 0.70, 0.45, 180, "EMA_BELOW_BOTH", 300},
		{23, "Intraday_CallBuy_VWAPAbove_150m", "Momentum", Call, 0.010, 150, 0.60, 0.42, 165, "VWAP_ABOVE", 240},
		{24, "Intraday_PutBuy_VWAPBelow_150m", "Momentum", Put, 0.010, 150, 0.60, 0.42, 165, "VWAP_BELOW", 240},
		{25, "Intraday_CallBuy_TripleBull_180m", "Hybrid", Call, 0.013, 180, 0.75, 0.48, 190, "TRIPLE_BULL", 360},
		{26, "Intraday_PutBuy_TripleBear_180m", "Hybrid", Put, 0.013, 180, 0.75, 0.48, 190, "TRIPLE_BEAR", 360},
		{27, "Intraday_CallBuy_HighIVBull_120m", "Hybrid", Call, 0.009, 120, 0.58, 0.40, 160, "HIGH_IV_BULL", 180},
		{28, "Intraday_PutBuy_HighIVBear_120m", "Hybrid", Put, 0.009, 120, 0.58, 0.40, 160, "HIGH_IV_BEAR", 180},
		{29, "Intraday_CallBuy_SharpReversalUp_150m", "Capitulation", Call, 0.010, 150, 0.62, 0.44, 170, "SHARP_REVERSAL_UP", 240},
		{30, "Intraday_PutBuy_SharpReversalDown_150m", "Capitulation", Put, 0.010, 150, 0.62, 0.44, 170, "SHARP_REVERSAL_DOWN", 240},
		{31, "Intraday_CallBuy_MomentumVWAPBull_210m", "Momentum", Call, 0.014, 210, 0.80, 0.50, 200, "MOMENTUM_VWAP_BULL", 300},
		{32, "Intraday_PutBuy_MomentumVWAPBear_210m", "Momentum", Put, 0.014, 210, 0.80, 0.50, 200, "MOMENTUM_VWAP_BEAR", 300},
		{33, "Intraday_CallBuy_BreakoutTrendBull_240m", "Breakout", Call, 0.015, 240, 0.85, 0.52, 210, "BREAKOUT_TREND_BULL", 360},
		{34, "Intraday_PutBuy_BreakdownTrendBear_240m", "Breakout", Put, 0.015, 240, 0.85, 0.52, 210, "BREAKDOWN_TREND_BEAR", 360},
		{35, "Intraday_CallBuy_CapitulationReclaim_180m", "Capitulation", Call, 0.012, 180, 0.68, 0.46, 180, "CAPITULATION_RECLAIM", 300},
		{36, "Intraday_CallBuy_CapitulationRecovery_180m", "Capitulation", Call, 0.012, 180, 0.68, 0.46, 180, "CAPITULATION_RECOVERY", 300},
		{37, "Intraday_CallBuy_MACDBullCross_210m", "Momentum", Call, 0.013, 210, 0.72, 0.47, 190, "MACD_BULL_CROSS", 300},
		{38, "Intraday_PutBuy_MACDBearCross_210m", "Momentum", Put, 0.013, 210, 0.72, 0.47, 190, "MACD_BEAR_CROSS", 300},

		// ── Swing (12): 480-1440m expiry, wider TP, long cooldown ──────────────
		{39, "Swing_CallBuy_VolCompressBull_480m", "Breakout", Call, 0.014, 480, 0.90, 0.48, 250, "VOL_COMPRESS_BULL", 600},
		{40, "Swing_PutBuy_VolCompressBear_480m", "Breakout", Put, 0.014, 480, 0.90, 0.48, 250, "VOL_COMPRESS_BEAR", 600},
		{41, "Swing_CallBuy_SessionOpenBull_540m", "Hybrid", Call, 0.013, 540, 0.85, 0.46, 240, "SESSION_OPEN_BULL", 720},
		{42, "Swing_PutBuy_SessionOpenBear_540m", "Hybrid", Put, 0.013, 540, 0.85, 0.46, 240, "SESSION_OPEN_BEAR", 720},
		{43, "Swing_CallBuy_OverextensionFadeDown_600m", "Mean Reversion", Call, 0.015, 600, 1.00, 0.50, 260, "OVEREXTENSION_FADE_DOWN", 900},
		{44, "Swing_PutBuy_OverextensionFadeUp_600m", "Mean Reversion", Put, 0.015, 600, 1.00, 0.50, 260, "OVEREXTENSION_FADE_UP", 900},
		{45, "Swing_CallBuy_ATRExpandBull_720m", "Momentum", Call, 0.016, 720, 1.10, 0.52, 270, "ATR_EXPAND_BULL", 1000},
		{46, "Swing_PutBuy_ATRExpandBear_720m", "Momentum", Put, 0.016, 720, 1.10, 0.52, 270, "ATR_EXPAND_BEAR", 1000},
		{47, "Swing_CallBuy_BreakoutTrendBull_960m", "Breakout", Call, 0.018, 960, 1.25, 0.55, 285, "BREAKOUT_TREND_BULL", 1300},
		{48, "Swing_PutBuy_BreakdownTrendBear_960m", "Breakout", Put, 0.018, 960, 1.25, 0.55, 285, "BREAKDOWN_TREND_BEAR", 1300},
		{49, "Swing_CallBuy_CapitulationReclaim_1440m", "Capitulation", Call, 0.020, 1440, 1.40, 0.58, 300, "CAPITULATION_RECLAIM", 1800},
		{50, "Swing_CallBuy_TripleBull_1440m", "Hybrid", Call, 0.020, 1440, 1.50, 0.60, 300, "TRIPLE_BULL", 1800},
	}

	out := make([]StrategyDef, 0, len(defs))
	for _, d := range defs {
		strategyIDs[d.name] = d.id
		out = append(out, d.toStrategyDef())
	}
	return out
}

// BuildStrategies returns the full 50-strategy BTC option-buying roster.
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
