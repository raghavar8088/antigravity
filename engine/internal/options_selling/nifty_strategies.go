package options_selling

// buildNiftyNativeStrategies returns 100 NIFTY 50–calibrated option-SELLING
// (theta-decay) strategies.
//
// Parameter rationale vs options scalper package:
//   - Slightly wider strikes (0.0010–0.0022) for higher Probability of Profit.
//   - Longer expiry (120–300 min) to harvest more theta decay per trade.
//   - Higher TP% (0.35–0.55) — capture majority of initial premium before gamma risk.
//   - Higher SL% (0.65–0.85) — allow moderate premium expansion before cutting.
//
// Strategy IDs 201–300 (NIFTY-exclusive; no collision with BTC IDs 101–130).
func buildNiftyNativeStrategies() []StrategyDef {
	return []StrategyDef{

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 1 — STRONG MOMENTUM SELLERS (IDs 201–210)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 201, Name: "N50S_StrongMom_Bull_Put_S1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.35, StopLossPct: 0.65, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 360},
		{ID: 202, Name: "N50S_StrongMom_Bear_Call_S1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.35, StopLossPct: 0.65, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 360},
		{ID: 203, Name: "N50S_StrongMom_Bull_Put_S2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.37, StopLossPct: 0.67, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 450},
		{ID: 204, Name: "N50S_StrongMom_Bear_Call_S2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.37, StopLossPct: 0.67, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 450},
		{ID: 205, Name: "N50S_StrongMom_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 540},
		{ID: 206, Name: "N50S_StrongMom_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 540},
		{ID: 207, Name: "N50S_StrongMom_Bull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 600},
		{ID: 208, Name: "N50S_StrongMom_Bear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 600},
		{ID: 209, Name: "N50S_StrongMom_Bull_Put_W1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0018, ExpiryMinutes: 240, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 720},
		{ID: 210, Name: "N50S_StrongMom_Bear_Call_W1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0018, ExpiryMinutes: 240, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 2 — EMA CROSS SELLERS (IDs 211–220)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 211, Name: "N50S_EMACross_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 420},
		{ID: 212, Name: "N50S_EMACross_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.36, StopLossPct: 0.66, PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 420},
		{ID: 213, Name: "N50S_EMACross_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 540},
		{ID: 214, Name: "N50S_EMACross_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 540},
		{ID: 215, Name: "N50S_EMAAboveBoth_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 540},
		{ID: 216, Name: "N50S_EMABelowBoth_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "EMA_BELOW_BOTH", CooldownSecs: 540},
		{ID: 217, Name: "N50S_EMACross_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 600},
		{ID: 218, Name: "N50S_EMACross_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 600},
		{ID: 219, Name: "N50S_EMAAboveBoth_Bull_Put_W1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 720},
		{ID: 220, Name: "N50S_EMABelowBoth_Bear_Call_W1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "EMA_BELOW_BOTH", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 3 — RSI MEAN REVERSION SELLERS (IDs 221–230)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 221, Name: "N50S_RSIRecov_Bull_Put_S1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 720},
		{ID: 222, Name: "N50S_RSIFade_Bear_Call_S1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 720},
		{ID: 223, Name: "N50S_RSIRecov_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 900},
		{ID: 224, Name: "N50S_RSIFade_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 900},
		{ID: 225, Name: "N50S_RSIOversold_Bull_Put_M2", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "RSI_OVERSOLD", CooldownSecs: 600},
		{ID: 226, Name: "N50S_RSIOverbought_Bear_Call_M2", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "RSI_OVERBOUGHT", CooldownSecs: 600},
		{ID: 227, Name: "N50S_StochOS_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "STOCH_OVERSOLD", CooldownSecs: 600},
		{ID: 228, Name: "N50S_StochOB_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "STOCH_OVERBOUGHT", CooldownSecs: 600},
		{ID: 229, Name: "N50S_OverextUp_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.48, StopLossPct: 0.78, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 1200},
		{ID: 230, Name: "N50S_OverextDn_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.48, StopLossPct: 0.78, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 1200},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 4 — BB & VOL-COMPRESSION SELLERS (IDs 231–240)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 231, Name: "N50S_BBSqueeze_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 480},
		{ID: 232, Name: "N50S_BBSqueeze_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.38, StopLossPct: 0.68, PositionUSD: 10000, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 480},
		{ID: 233, Name: "N50S_BBSqueeze_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 600},
		{ID: 234, Name: "N50S_BBSqueeze_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 600},
		{ID: 235, Name: "N50S_BBLower_Bull_Put_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "BB_LOWER_TOUCH", CooldownSecs: 540},
		{ID: 236, Name: "N50S_BBUpper_Bear_Call_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "BB_UPPER_TOUCH", CooldownSecs: 540},
		{ID: 237, Name: "N50S_VolCompress_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 660},
		{ID: 238, Name: "N50S_VolCompress_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 660},
		{ID: 239, Name: "N50S_VolCompress_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 720},
		{ID: 240, Name: "N50S_VolCompress_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 5 — VWAP SELLERS (IDs 241–250)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 241, Name: "N50S_VWAP_Bull_Put_S1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.36, StopLossPct: 0.65, PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 420},
		{ID: 242, Name: "N50S_VWAP_Bear_Call_S1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.36, StopLossPct: 0.65, PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 420},
		{ID: 243, Name: "N50S_VWAP_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 165, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 540},
		{ID: 244, Name: "N50S_VWAP_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 165, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 540},
		{ID: 245, Name: "N50S_MomVWAP_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 600},
		{ID: 246, Name: "N50S_MomVWAP_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 600},
		{ID: 247, Name: "N50S_MomVWAP_Bull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 660},
		{ID: 248, Name: "N50S_MomVWAP_Bear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 660},
		{ID: 249, Name: "N50S_VWAP_Bull_Put_W1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0017, ExpiryMinutes: 210, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 720},
		{ID: 250, Name: "N50S_VWAP_Bear_Call_W1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0017, ExpiryMinutes: 210, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 6 — BREAKOUT / BREAKDOWN SELLERS (IDs 251–260)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 251, Name: "N50S_ResBreak_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.37, StopLossPct: 0.67, PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 480},
		{ID: 252, Name: "N50S_SupBreak_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.37, StopLossPct: 0.67, PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 480},
		{ID: 253, Name: "N50S_ResBreak_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 600},
		{ID: 254, Name: "N50S_SupBreak_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 600},
		{ID: 255, Name: "N50S_BrkTrend_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 600},
		{ID: 256, Name: "N50S_BkdnTrend_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 600},
		{ID: 257, Name: "N50S_BrkTrend_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 720},
		{ID: 258, Name: "N50S_BkdnTrend_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 720},
		{ID: 259, Name: "N50S_ConsecBull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "CONSEC_BULL_BARS", CooldownSecs: 420},
		{ID: 260, Name: "N50S_ConsecBear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 420},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 7 — SESSION OPEN & BASE MOMENTUM SELLERS (IDs 261–270)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 261, Name: "N50S_SessOpen_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.36, StopLossPct: 0.65, PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 720},
		{ID: 262, Name: "N50S_SessOpen_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.36, StopLossPct: 0.65, PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 720},
		{ID: 263, Name: "N50S_SessOpen_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 165, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 900},
		{ID: 264, Name: "N50S_SessOpen_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 165, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 900},
		{ID: 265, Name: "N50S_SessOpen_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 1080},
		{ID: 266, Name: "N50S_SessOpen_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 1080},
		{ID: 267, Name: "N50S_BullMom_Put_S1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.36, StopLossPct: 0.65, PositionUSD: 10000, Signal: "BULL_MOMENTUM", CooldownSecs: 360},
		{ID: 268, Name: "N50S_BearMom_Call_S1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 135, TakeProfitPct: 0.36, StopLossPct: 0.65, PositionUSD: 10000, Signal: "BEAR_MOMENTUM", CooldownSecs: 360},
		{ID: 269, Name: "N50S_BullMom_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 165, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "BULL_MOMENTUM", CooldownSecs: 480},
		{ID: 270, Name: "N50S_BearMom_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 165, TakeProfitPct: 0.38, StopLossPct: 0.67, PositionUSD: 10000, Signal: "BEAR_MOMENTUM", CooldownSecs: 480},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 8 — CAPITULATION & REVERSAL SELLERS (IDs 271–280)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 271, Name: "N50S_CapRecov_Bull_Put_M1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.46, StopLossPct: 0.74, PositionUSD: 10000, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 1200},
		{ID: 272, Name: "N50S_CapReclaim_Bull_Put_M1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0018, ExpiryMinutes: 210, TakeProfitPct: 0.48, StopLossPct: 0.76, PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1500},
		{ID: 273, Name: "N50S_SharpRevUp_Bull_Put_M1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 600},
		{ID: 274, Name: "N50S_SharpRevDn_Bear_Call_M1", Category: "Capitulation", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 600},
		{ID: 275, Name: "N50S_CapRecov_Bull_Put_W1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0018, ExpiryMinutes: 240, TakeProfitPct: 0.50, StopLossPct: 0.80, PositionUSD: 10000, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 1800},
		{ID: 276, Name: "N50S_CapReclaim_Bull_Put_W1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0020, ExpiryMinutes: 240, TakeProfitPct: 0.52, StopLossPct: 0.82, PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1800},
		{ID: 277, Name: "N50S_SharpRevUp_Bull_Put_W1", Category: "Capitulation", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 900},
		{ID: 278, Name: "N50S_SharpRevDn_Bear_Call_W1", Category: "Capitulation", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 900},
		{ID: 279, Name: "N50S_OverextUp_Bear_Call_W1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.50, StopLossPct: 0.82, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 1500},
		{ID: 280, Name: "N50S_OverextDn_Bull_Put_W1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.50, StopLossPct: 0.82, PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 1500},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 9 — TRIPLE CONFLUENCE & HYBRID SELLERS (IDs 281–290)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 281, Name: "N50S_Triple_Bull_Put_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 165, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 720},
		{ID: 282, Name: "N50S_Triple_Bear_Call_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 165, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 720},
		{ID: 283, Name: "N50S_Triple_Bull_Put_M2", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.46, StopLossPct: 0.76, PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 900},
		{ID: 284, Name: "N50S_Triple_Bear_Call_M2", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 180, TakeProfitPct: 0.46, StopLossPct: 0.76, PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 900},
		{ID: 285, Name: "N50S_MomVWAP_Pro_Bull_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 660},
		{ID: 286, Name: "N50S_MomVWAP_Pro_Bear_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.42, StopLossPct: 0.72, PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 660},
		{ID: 287, Name: "N50S_BrkTrend_Bull_Put_W1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0017, ExpiryMinutes: 210, TakeProfitPct: 0.46, StopLossPct: 0.76, PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 900},
		{ID: 288, Name: "N50S_BkdnTrend_Bear_Call_W1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0017, ExpiryMinutes: 210, TakeProfitPct: 0.46, StopLossPct: 0.76, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 900},
		{ID: 289, Name: "N50S_HighIV_Bull_Put_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "HIGH_IV_BULL", CooldownSecs: 720},
		{ID: 290, Name: "N50S_HighIV_Bear_Call_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0015, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "HIGH_IV_BEAR", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 10 — WIDE-STRIKE ELITE THETA SELLERS (IDs 291–300)
		// Highest-conviction signals; furthest OTM, maximum theta capture.
		// ═══════════════════════════════════════════════════════════════════
		{ID: 291, Name: "N50S_ConsecBull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "CONSEC_BULL_BARS", CooldownSecs: 540},
		{ID: 292, Name: "N50S_ConsecBear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 165, TakeProfitPct: 0.40, StopLossPct: 0.70, PositionUSD: 10000, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 540},
		{ID: 293, Name: "N50S_Triple_Bull_Put_W1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0018, ExpiryMinutes: 240, TakeProfitPct: 0.50, StopLossPct: 0.80, PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 1200},
		{ID: 294, Name: "N50S_Triple_Bear_Call_W1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0018, ExpiryMinutes: 240, TakeProfitPct: 0.50, StopLossPct: 0.80, PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 1200},
		{ID: 295, Name: "N50S_VolComp_Bull_Put_W1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 900},
		{ID: 296, Name: "N50S_VolComp_Bear_Call_W1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 900},
		{ID: 297, Name: "N50S_ResBreak_Bull_Put_W1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 840},
		{ID: 298, Name: "N50S_SupBreak_Bear_Call_W1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 210, TakeProfitPct: 0.44, StopLossPct: 0.74, PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 840},
		{ID: 299, Name: "N50S_CapReclaim_Elite_Put", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0020, ExpiryMinutes: 300, TakeProfitPct: 0.55, StopLossPct: 0.85, PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1800},
		{ID: 300, Name: "N50S_BkdnTrend_Elite_Call", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0020, ExpiryMinutes: 300, TakeProfitPct: 0.55, StopLossPct: 0.85, PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 1800},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 11 — NSE OPEN (IDs 301–306)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 301, Name: "N50S_NSEOpen_Bull_Put_S1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.35, StopLossPct: 0.63, PositionUSD: 10000, Signal: "NSE_OPEN_BULL", CooldownSecs: 600},
		{ID: 302, Name: "N50S_NSEOpen_Bear_Call_S1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0010, ExpiryMinutes: 120, TakeProfitPct: 0.35, StopLossPct: 0.63, PositionUSD: 10000, Signal: "NSE_OPEN_BEAR", CooldownSecs: 600},
		{ID: 303, Name: "N50S_NSEOpen_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "NSE_OPEN_BULL", CooldownSecs: 720},
		{ID: 304, Name: "N50S_NSEOpen_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.38, StopLossPct: 0.66, PositionUSD: 10000, Signal: "NSE_OPEN_BEAR", CooldownSecs: 720},
		{ID: 305, Name: "N50S_NSEOpen_Bull_Put_M2", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "NSE_OPEN_BULL", CooldownSecs: 900},
		{ID: 306, Name: "N50S_NSEOpen_Bear_Call_M2", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "NSE_OPEN_BEAR", CooldownSecs: 900},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 12 — NSE MIDDAY (IDs 307–310)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 307, Name: "N50S_NSEMidday_Bull_Put_M1", Category: "Momentum", Type: Put, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.37, StopLossPct: 0.65, PositionUSD: 10000, Signal: "NSE_MIDDAY_BULL", CooldownSecs: 720},
		{ID: 308, Name: "N50S_NSEMidday_Bear_Call_M1", Category: "Momentum", Type: Call, StrikePctOTM: 0.0011, ExpiryMinutes: 135, TakeProfitPct: 0.37, StopLossPct: 0.65, PositionUSD: 10000, Signal: "NSE_MIDDAY_BEAR", CooldownSecs: 720},
		{ID: 309, Name: "N50S_NSEMidday_Bull_Put_M2", Category: "Momentum", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "NSE_MIDDAY_BULL", CooldownSecs: 900},
		{ID: 310, Name: "N50S_NSEMidday_Bear_Call_M2", Category: "Momentum", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "NSE_MIDDAY_BEAR", CooldownSecs: 900},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 13 — NSE PRE-CLOSE PREMIUM SELL (IDs 311–314)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 311, Name: "N50S_PreClose_SellCall_S1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0013, ExpiryMinutes: 60, TakeProfitPct: 0.42, StopLossPct: 0.66, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_CALL", CooldownSecs: 900},
		{ID: 312, Name: "N50S_PreClose_SellPut_S1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0013, ExpiryMinutes: 60, TakeProfitPct: 0.42, StopLossPct: 0.66, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_PUT", CooldownSecs: 900},
		{ID: 313, Name: "N50S_PreClose_SellCall_M1", Category: "Mean Reversion", Type: Call, StrikePctOTM: 0.0016, ExpiryMinutes: 75, TakeProfitPct: 0.46, StopLossPct: 0.72, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_CALL", CooldownSecs: 1200},
		{ID: 314, Name: "N50S_PreClose_SellPut_M1", Category: "Mean Reversion", Type: Put, StrikePctOTM: 0.0016, ExpiryMinutes: 75, TakeProfitPct: 0.46, StopLossPct: 0.72, PositionUSD: 10000, Signal: "NSE_PRECLOSE_SELL_PUT", CooldownSecs: 1200},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 14 — MACD CROSSOVER FOR NIFTY (IDs 315–318)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 315, Name: "N50S_MACD_Bull_Put_M1", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "MACD_BULL_CROSS", CooldownSecs: 600},
		{ID: 316, Name: "N50S_MACD_Bear_Call_M1", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0012, ExpiryMinutes: 150, TakeProfitPct: 0.40, StopLossPct: 0.68, PositionUSD: 10000, Signal: "MACD_BEAR_CROSS", CooldownSecs: 600},
		{ID: 317, Name: "N50S_MACD_Bull_Put_M2", Category: "Hybrid", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "MACD_BULL_CROSS", CooldownSecs: 720},
		{ID: 318, Name: "N50S_MACD_Bear_Call_M2", Category: "Hybrid", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 180, TakeProfitPct: 0.44, StopLossPct: 0.72, PositionUSD: 10000, Signal: "MACD_BEAR_CROSS", CooldownSecs: 720},

		// ═══════════════════════════════════════════════════════════════════
		// GROUP 15 — ATR EXPANSION FOR NIFTY (IDs 319–320)
		// ═══════════════════════════════════════════════════════════════════
		{ID: 319, Name: "N50S_ATRExp_Bull_Put_M1", Category: "Breakout", Type: Put, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "ATR_EXPAND_BULL", CooldownSecs: 540},
		{ID: 320, Name: "N50S_ATRExp_Bear_Call_M1", Category: "Breakout", Type: Call, StrikePctOTM: 0.0014, ExpiryMinutes: 150, TakeProfitPct: 0.42, StopLossPct: 0.70, PositionUSD: 10000, Signal: "ATR_EXPAND_BEAR", CooldownSecs: 540},
	}
}
