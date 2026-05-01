package options

// buildNiftyNativeStrategies returns 100 NIFTY 50–calibrated option-selling strategies.
//
// Parameter rationale for NIFTY vs BTC:
//   - NIFTY annualised IV ≈ 14-22% (BTC ≈ 60-100%)
//   - Strike OTM: 0.0008–0.0018 (≈ 19–43 pts on NIFTY 24000)
//   - ExpiryMinutes: 90–210 (intraday scalps to end-of-day holds)
//   - TakeProfitPct: 0.32–0.50 (bank theta decay quickly)
//   - StopLossPct:   0.60–0.82 (cut expansion early)
//
// Strategy IDs 201–300 (NIFTY-exclusive, no collision with BTC IDs 101–130).
func buildNiftyNativeStrategies() []StrategyDef {
	return []StrategyDef{

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 1 — STRONG MOMENTUM (IDs 201–210)
		// Signal fires on rapid directional thrust; sell far-OTM premium.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 201, Name: "N50_StrongMom_Bull_Put_S1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.32, StopLossPct: 0.62, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 300},
		{ID: 202, Name: "N50_StrongMom_Bear_Call_S1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.32, StopLossPct: 0.62, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 300},
		{ID: 203, Name: "N50_StrongMom_Bull_Put_S2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.34, StopLossPct: 0.64, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 360},
		{ID: 204, Name: "N50_StrongMom_Bear_Call_S2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.34, StopLossPct: 0.64, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 360},
		{ID: 205, Name: "N50_StrongMom_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 450},
		{ID: 206, Name: "N50_StrongMom_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 450},
		{ID: 207, Name: "N50_StrongMom_Bull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 540},
		{ID: 208, Name: "N50_StrongMom_Bear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 540},
		{ID: 209, Name: "N50_StrongMom_Bull_Put_W1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 600},
		{ID: 210, Name: "N50_StrongMom_Bear_Call_W1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 600},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 2 — EMA CROSS (IDs 211–220)
		// Trend-change crossover events; immediate premium sell after confirm.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 211, Name: "N50_EMACross_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.33, StopLossPct: 0.63, PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 360},
		{ID: 212, Name: "N50_EMACross_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.33, StopLossPct: 0.63, PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 360},
		{ID: 213, Name: "N50_EMACross_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 480},
		{ID: 214, Name: "N50_EMACross_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 480},
		{ID: 215, Name: "N50_EMAAboveBoth_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 480},
		{ID: 216, Name: "N50_EMABelowBoth_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "EMA_BELOW_BOTH", CooldownSecs: 480},
		{ID: 217, Name: "N50_EMACross_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 540},
		{ID: 218, Name: "N50_EMACross_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 540},
		{ID: 219, Name: "N50_EMAAboveBoth_Bull_Put_W1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 600},
		{ID: 220, Name: "N50_EMABelowBoth_Bear_Call_W1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "EMA_BELOW_BOTH", CooldownSecs: 600},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 3 — RSI MEAN REVERSION (IDs 221–230)
		// Extreme RSI readings; sell the opposite-direction option.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 221, Name: "N50_RSIRecov_Bull_Put_S1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.65, PositionUSD: 10000, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 600},
		{ID: 222, Name: "N50_RSIFade_Bear_Call_S1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.65, PositionUSD: 10000, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 600},
		{ID: 223, Name: "N50_RSIRecov_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.68, PositionUSD: 10000, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 720},
		{ID: 224, Name: "N50_RSIFade_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.68, PositionUSD: 10000, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 720},
		{ID: 225, Name: "N50_RSIOversold_Bull_Put_M2", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.66, PositionUSD: 10000, Signal: "RSI_OVERSOLD", CooldownSecs: 540},
		{ID: 226, Name: "N50_RSIOverbought_Bear_Call_M2", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.66, PositionUSD: 10000, Signal: "RSI_OVERBOUGHT", CooldownSecs: 540},
		{ID: 227, Name: "N50_StochOS_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.64, PositionUSD: 10000, Signal: "STOCH_OVERSOLD", CooldownSecs: 480},
		{ID: 228, Name: "N50_StochOB_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.64, PositionUSD: 10000, Signal: "STOCH_OVERBOUGHT", CooldownSecs: 480},
		{ID: 229, Name: "N50_OverextUp_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 900},
		{ID: 230, Name: "N50_OverextDn_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 900},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 4 — BOLLINGER BAND & VOL-COMPRESSION (IDs 231–240)
		// Sell premium after squeeze breakout or band-touch bounce.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 231, Name: "N50_BBSqueeze_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0009, ExpiryMinutes: 90, TakeProfitPct: 0.35, StopLossPct: 0.65, PositionUSD: 10000, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 360},
		{ID: 232, Name: "N50_BBSqueeze_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0009, ExpiryMinutes: 90, TakeProfitPct: 0.35, StopLossPct: 0.65, PositionUSD: 10000, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 360},
		{ID: 233, Name: "N50_BBSqueeze_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 480},
		{ID: 234, Name: "N50_BBSqueeze_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 480},
		{ID: 235, Name: "N50_BBLower_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.36, StopLossPct: 0.63, PositionUSD: 10000, Signal: "BB_LOWER_TOUCH", CooldownSecs: 420},
		{ID: 236, Name: "N50_BBUpper_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.36, StopLossPct: 0.63, PositionUSD: 10000, Signal: "BB_UPPER_TOUCH", CooldownSecs: 420},
		{ID: 237, Name: "N50_VolCompress_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 540},
		{ID: 238, Name: "N50_VolCompress_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 540},
		{ID: 239, Name: "N50_VolCompress_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 600},
		{ID: 240, Name: "N50_VolCompress_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 600},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 5 — VWAP CONTINUATION (IDs 241–250)
		// Price significantly above/below VWAP with momentum confirmation.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 241, Name: "N50_VWAP_Bull_Put_S1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.33, StopLossPct: 0.62, PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 300},
		{ID: 242, Name: "N50_VWAP_Bear_Call_S1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.33, StopLossPct: 0.62, PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 300},
		{ID: 243, Name: "N50_VWAP_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.64, PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 420},
		{ID: 244, Name: "N50_VWAP_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.64, PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 420},
		{ID: 245, Name: "N50_MomVWAP_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 480},
		{ID: 246, Name: "N50_MomVWAP_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 480},
		{ID: 247, Name: "N50_MomVWAP_Bull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 540},
		{ID: 248, Name: "N50_MomVWAP_Bear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 540},
		{ID: 249, Name: "N50_VWAP_Bull_Put_W1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 600},
		{ID: 250, Name: "N50_VWAP_Bear_Call_W1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 600},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 6 — RESISTANCE / SUPPORT BREAKOUT (IDs 251–260)
		// 20-bar high/low break + confirmed momentum.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 251, Name: "N50_ResBreak_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0009, ExpiryMinutes: 90, TakeProfitPct: 0.34, StopLossPct: 0.64, PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 360},
		{ID: 252, Name: "N50_SupBreak_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0009, ExpiryMinutes: 90, TakeProfitPct: 0.34, StopLossPct: 0.64, PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 360},
		{ID: 253, Name: "N50_ResBreak_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.37, StopLossPct: 0.67, PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 480},
		{ID: 254, Name: "N50_SupBreak_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.37, StopLossPct: 0.67, PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 480},
		{ID: 255, Name: "N50_BrkTrend_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 540},
		{ID: 256, Name: "N50_BkdnTrend_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 540},
		{ID: 257, Name: "N50_BrkTrend_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 600},
		{ID: 258, Name: "N50_BkdnTrend_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 600},
		{ID: 259, Name: "N50_ConsecBull_Put_S1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0009, ExpiryMinutes: 105, TakeProfitPct: 0.35, StopLossPct: 0.63, PositionUSD: 10000, Signal: "CONSEC_BULL_BARS", CooldownSecs: 300},
		{ID: 260, Name: "N50_ConsecBear_Call_S1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0009, ExpiryMinutes: 105, TakeProfitPct: 0.35, StopLossPct: 0.63, PositionUSD: 10000, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 300},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 7 — SESSION OPEN & BASE MOMENTUM (IDs 261–270)
		// NSE open (03:45 UTC) and midday (06:30 UTC) directional bias.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 261, Name: "N50_SessOpen_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.32, StopLossPct: 0.60, PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 600},
		{ID: 262, Name: "N50_SessOpen_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.32, StopLossPct: 0.60, PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 600},
		{ID: 263, Name: "N50_SessOpen_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.35, StopLossPct: 0.63, PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 720},
		{ID: 264, Name: "N50_SessOpen_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.35, StopLossPct: 0.63, PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 720},
		{ID: 265, Name: "N50_SessOpen_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 900},
		{ID: 266, Name: "N50_SessOpen_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 900},
		{ID: 267, Name: "N50_BullMom_Put_Scalp_1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.32, StopLossPct: 0.62, PositionUSD: 10000, Signal: "BULL_MOMENTUM", CooldownSecs: 240},
		{ID: 268, Name: "N50_BearMom_Call_Scalp_1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0008, ExpiryMinutes: 90, TakeProfitPct: 0.32, StopLossPct: 0.62, PositionUSD: 10000, Signal: "BEAR_MOMENTUM", CooldownSecs: 240},
		{ID: 269, Name: "N50_BullMom_Put_Scalp_2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.34, StopLossPct: 0.64, PositionUSD: 10000, Signal: "BULL_MOMENTUM", CooldownSecs: 300},
		{ID: 270, Name: "N50_BearMom_Call_Scalp_2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.34, StopLossPct: 0.64, PositionUSD: 10000, Signal: "BEAR_MOMENTUM", CooldownSecs: 300},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 8 — CAPITULATION & REVERSAL (IDs 271–280)
		// Sharp flush events + V-reversal confirmation.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 271, Name: "N50_CapRecov_Bull_Put_M1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 900},
		{ID: 272, Name: "N50_CapReclaim_Bull_Put_M1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 165, TakeProfitPct: 0.46, StopLossPct: 0.74, PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1200},
		{ID: 273, Name: "N50_SharpRevUp_Bull_Put_S1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 480},
		{ID: 274, Name: "N50_SharpRevDn_Bear_Call_S1", Category: "Capitulation", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 105, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 480},
		{ID: 275, Name: "N50_CapRecov_Bull_Put_W1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.48, StopLossPct: 0.78, PositionUSD: 10000, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 1200},
		{ID: 276, Name: "N50_CapReclaim_Bull_Put_W1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0018, ExpiryMinutes: 180, TakeProfitPct: 0.50, StopLossPct: 0.80, PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1500},
		{ID: 277, Name: "N50_SharpRevUp_Bull_Put_M1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 600},
		{ID: 278, Name: "N50_SharpRevDn_Bear_Call_M1", Category: "Capitulation", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 600},
		{ID: 279, Name: "N50_OverextUp_Bear_Call_M2", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.46, StopLossPct: 0.76, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 1200},
		{ID: 280, Name: "N50_OverextDn_Bull_Put_M2", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.46, StopLossPct: 0.76, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 1200},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 9 — TRIPLE CONFLUENCE & HYBRID (IDs 281–290)
		// Multi-condition signals with highest conviction; wider premium range.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 281, Name: "N50_Triple_Bull_Put_S1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 600},
		{ID: 282, Name: "N50_Triple_Bear_Call_S1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 120, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 600},
		{ID: 283, Name: "N50_Triple_Bull_Put_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 720},
		{ID: 284, Name: "N50_Triple_Bear_Call_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 720},
		{ID: 285, Name: "N50_MomVWAP_Pro_Bull_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 540},
		{ID: 286, Name: "N50_MomVWAP_Pro_Bear_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 540},
		{ID: 287, Name: "N50_BrkTrend_Bull_Put_W1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 165, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 720},
		{ID: 288, Name: "N50_BkdnTrend_Bear_Call_W1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 165, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 720},
		{ID: 289, Name: "N50_HighIV_Bull_Put_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "HIGH_IV_BULL", CooldownSecs: 600},
		{ID: 290, Name: "N50_HighIV_Bear_Call_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "HIGH_IV_BEAR", CooldownSecs: 600},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 10 — ADVANCED & WIDE-STRIKE ELITE (IDs 291–300)
		// Higher-conviction, longer expiry, wider strikes for max theta capture.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 291, Name: "N50_ConsecBull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.37, StopLossPct: 0.65, PositionUSD: 10000, Signal: "CONSEC_BULL_BARS", CooldownSecs: 420},
		{ID: 292, Name: "N50_ConsecBear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.37, StopLossPct: 0.65, PositionUSD: 10000, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 420},
		{ID: 293, Name: "N50_Triple_Bull_Put_W1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.48, StopLossPct: 0.76, PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 900},
		{ID: 294, Name: "N50_Triple_Bear_Call_W1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.48, StopLossPct: 0.76, PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 900},
		{ID: 295, Name: "N50_VolComp_Bull_Put_W1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 720},
		{ID: 296, Name: "N50_VolComp_Bear_Call_W1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 720},
		{ID: 297, Name: "N50_ResBreak_Bull_Put_W1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 600},
		{ID: 298, Name: "N50_SupBreak_Bear_Call_W1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 600},
		{ID: 299, Name: "N50_CapReclaim_Elite_Put", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0018, ExpiryMinutes: 210, TakeProfitPct: 0.50, StopLossPct: 0.82, PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1800},
		{ID: 300, Name: "N50_BkdnTrend_Elite_Call", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0018, ExpiryMinutes: 210, TakeProfitPct: 0.50, StopLossPct: 0.82, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 1800},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 11 — NSE OPEN (IDs 301–306)
		// Fire in first 33 min of NSE session (03:45–04:18 UTC).
		// ═══════════════════════════════════════════════════════════════════
		{ID: 301, Name: "N50_NSEOpen_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0009, ExpiryMinutes: 90, TakeProfitPct: 0.34, StopLossPct: 0.62, PositionUSD: 10000, Signal: "NSE_OPEN_BULL", CooldownSecs: 600},
		{ID: 302, Name: "N50_NSEOpen_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0009, ExpiryMinutes: 90, TakeProfitPct: 0.34, StopLossPct: 0.62, PositionUSD: 10000, Signal: "NSE_OPEN_BEAR", CooldownSecs: 600},
		{ID: 303, Name: "N50_NSEOpen_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.37, StopLossPct: 0.65, PositionUSD: 10000, Signal: "NSE_OPEN_BULL", CooldownSecs: 720},
		{ID: 304, Name: "N50_NSEOpen_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.37, StopLossPct: 0.65, PositionUSD: 10000, Signal: "NSE_OPEN_BEAR", CooldownSecs: 720},
		{ID: 305, Name: "N50_NSEOpen_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "NSE_OPEN_BULL", CooldownSecs: 900},
		{ID: 306, Name: "N50_NSEOpen_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "NSE_OPEN_BEAR", CooldownSecs: 900},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 12 — NSE MIDDAY (IDs 307–310)
		// 06:00–06:45 UTC liquidity reset — directional continuation plays.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 307, Name: "N50_NSEMidday_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.64, PositionUSD: 10000, Signal: "NSE_MIDDAY_BULL", CooldownSecs: 720},
		{ID: 308, Name: "N50_NSEMidday_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.64, PositionUSD: 10000, Signal: "NSE_MIDDAY_BEAR", CooldownSecs: 720},
		{ID: 309, Name: "N50_NSEMidday_Bull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.39, StopLossPct: 0.67, PositionUSD: 10000, Signal: "NSE_MIDDAY_BULL", CooldownSecs: 900},
		{ID: 310, Name: "N50_NSEMidday_Bear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 135, TakeProfitPct: 0.39, StopLossPct: 0.67, PositionUSD: 10000, Signal: "NSE_MIDDAY_BEAR", CooldownSecs: 900},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 13 — NSE PRE-CLOSE PREMIUM SELL (IDs 311–314)
		// 08:45–09:15 UTC — sell inflated puts/calls before time-value crush.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 311, Name: "N50_PreClose_SellCall_S1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 45, TakeProfitPct: 0.40, StopLossPct: 0.65, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_CALL", CooldownSecs: 900},
		{ID: 312, Name: "N50_PreClose_SellPut_S1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 45, TakeProfitPct: 0.40, StopLossPct: 0.65, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_PUT", CooldownSecs: 900},
		{ID: 313, Name: "N50_PreClose_SellCall_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 60, TakeProfitPct: 0.44, StopLossPct: 0.70, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_CALL", CooldownSecs: 1200},
		{ID: 314, Name: "N50_PreClose_SellPut_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 60, TakeProfitPct: 0.44, StopLossPct: 0.70, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_PUT", CooldownSecs: 1200},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 14 — MACD CROSSOVER FOR NIFTY (IDs 315–318)
		// MACD signal crossover with NIFTY-calibrated OTM and expiry.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 315, Name: "N50_MACD_Bull_Put_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "MACD_BULL_CROSS", CooldownSecs: 600},
		{ID: 316, Name: "N50_MACD_Bear_Call_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 120, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "MACD_BEAR_CROSS", CooldownSecs: 600},
		{ID: 317, Name: "N50_MACD_Bull_Put_M2", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "MACD_BULL_CROSS", CooldownSecs: 720},
		{ID: 318, Name: "N50_MACD_Bear_Call_M2", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "MACD_BEAR_CROSS", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 15 — ATR EXPANSION FOR NIFTY (IDs 319–320)
		// Intraday vol burst; sell OTM premium into the expansion spike.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 319, Name: "N50_ATRExp_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "ATR_EXPAND_BULL", CooldownSecs: 540},
		{ID: 320, Name: "N50_ATRExp_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 135, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "ATR_EXPAND_BEAR", CooldownSecs: 540},
	}
}
