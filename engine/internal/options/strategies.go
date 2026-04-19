package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.020 // Sellers can go further out
)

var optionStrategyCategories = map[string]string{
	"MomentumBurst_Bull_Put_Sell":     "Momentum",
	"MomentumBurst_Bear_Call_Sell":    "Momentum",
	"ConsecCandle_Bull_Put_Sell":      "Momentum",
	"ConsecCandle_Bear_Call_Sell":     "Momentum",
	"RSIRecovery_Bull_Put_Sell":       "Mean Reversion",
	"RSIFade_Bear_Call_Sell":          "Mean Reversion",
	"Overextension_Fade_Call_Sell":    "Mean Reversion",
	"Overextension_Fade_Put_Sell":     "Mean Reversion",
	"EMA_BullCross_Put_Sell":          "Breakout",
	"EMA_BearCross_Call_Sell":         "Breakout",
	"Resistance_Breakout_Put_Sell":    "Breakout",
	"Support_Breakdown_Call_Sell":     "Breakout",
	"Capitulation_VReversal_Put_Sell": "Capitulation",
	"SessionOpen_Bull_Put_Sell":       "Breakout",
	"SessionOpen_Bear_Call_Sell":      "Breakout",
	"VolCompress_Breakout_Bull_Put":   "Breakout",
	"VolCompress_Breakout_Bear_Call":  "Breakout",
	"VWAP_Continuation_Bull_Put":      "Momentum",
	"VWAP_Continuation_Bear_Call":     "Momentum",
	"TripleConfluence_Bull_Put":       "Hybrid",
	"TripleConfluence_Bear_Call":      "Hybrid",
	"TrendAlignment_Bull_Put":         "Breakout",
	"TrendAlignment_Bear_Call":        "Breakout",
	"BBSqueeze_Release_Bull_Put":      "Breakout",
	"BBSqueeze_Release_Bear_Call":     "Breakout",
	"MomentumVWAP_Pro_Bull_Put":       "Momentum",
	"MomentumVWAP_Pro_Bear_Call":      "Momentum",
	"BreakoutTrend_Pro_Bull_Put":      "Breakout",
	"BreakdownTrend_Pro_Bear_Call":    "Breakout",
	"Capitulation_Reclaim_Elite_Put":  "Capitulation",
}

var strategyIDs = map[string]int{
	"MomentumBurst_Bull_Put_Sell":     101,
	"MomentumBurst_Bear_Call_Sell":    102,
	"ConsecCandle_Bull_Put_Sell":      103,
	"ConsecCandle_Bear_Call_Sell":     104,
	"RSIRecovery_Bull_Put_Sell":       105,
	"RSIFade_Bear_Call_Sell":          106,
	"Overextension_Fade_Call_Sell":    107,
	"Overextension_Fade_Put_Sell":     108,
	"EMA_BullCross_Put_Sell":          109,
	"EMA_BearCross_Call_Sell":         110,
	"Resistance_Breakout_Put_Sell":    111,
	"Support_Breakdown_Call_Sell":     112,
	"Capitulation_VReversal_Put_Sell": 113,
	"SessionOpen_Bull_Put_Sell":       114,
	"SessionOpen_Bear_Call_Sell":      115,
	"VolCompress_Breakout_Bull_Put":   116,
	"VolCompress_Breakout_Bear_Call":  117,
	"VWAP_Continuation_Bull_Put":      118,
	"VWAP_Continuation_Bear_Call":     119,
	"TripleConfluence_Bull_Put":       120,
	"TripleConfluence_Bear_Call":      121,
	"TrendAlignment_Bull_Put":         122,
	"TrendAlignment_Bear_Call":        123,
	"BBSqueeze_Release_Bull_Put":      124,
	"BBSqueeze_Release_Bear_Call":     125,
	"MomentumVWAP_Pro_Bull_Put":       126,
	"MomentumVWAP_Pro_Bear_Call":      127,
	"BreakoutTrend_Pro_Bull_Put":      128,
	"BreakdownTrend_Pro_Bear_Call":    129,
	"Capitulation_Reclaim_Elite_Put":  130,
}

func assignStrategyIDs(defs []StrategyDef) []StrategyDef {
	for i := range defs {
		defs[i].ID = strategyIDs[defs[i].Name]
	}
	return defs
}

func BuildStrategies() []StrategyDef {
	all := assignStrategyIDs(buildAllStrategies())
	filtered := make([]StrategyDef, 0, len(all))
	for _, def := range all {
		if def.ExpiryMinutes >= minLiveExpiryMinutes && def.StrikePctOTM <= maxLiveStrikePctOTM {
			if category, ok := optionStrategyCategories[def.Name]; ok {
				def.Category = category
			}
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// BuildNiftyStrategies returns 100 NIFTY 50–native option-selling strategies.
// These are purpose-built for NIFTY's characteristics (IV ≈ 14-22%) rather
// than BTC-scaled proxies. Strike OTM 0.08-0.18% (≈ 19-43 pts on NIFTY 24000),
// expiry 90-210 min, signals from NiftySignals map.
func BuildNiftyStrategies() []StrategyDef {
	return buildNiftyNativeStrategies()
}

// buildAllStrategies defines BTC option SELLING (writing) strategies.
//
// Efficiency Improvements:
//   - All strategies use safer OTM strikes (1.0% - 1.8%) for higher Probability of Profit (POP).
//   - Bullish signals trigger SOLD PUTS.
//   - Bearish signals trigger SOLD CALLS.
//   - TakeProfitPct: 0.36 - 0.52 (bank decay earlier before gamma can reverse it).
//   - StopLossPct: 0.65 - 0.88 (cut premium expansion sooner instead of tolerating doubles).
//   - ExpiryMinutes: 135 - 210 to reduce gamma spikes and keep shorts more stable.
func buildAllStrategies() []StrategyDef {
	return []StrategyDef{

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY A — MOMENTUM CONTINUATION (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "MomentumBurst_Bull_Put_Sell", Type: Put,
			StrikePctOTM: 0.011, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.70,
			PositionUSD: 10000, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 600,
		},
		{
			Name: "MomentumBurst_Bear_Call_Sell", Type: Call,
			StrikePctOTM: 0.011, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.70,
			PositionUSD: 10000, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 600,
		},
		{
			Name: "ConsecCandle_Bull_Put_Sell", Type: Put,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.40, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "CONSEC_BULL_BARS", CooldownSecs: 420,
		},
		{
			Name: "ConsecCandle_Bear_Call_Sell", Type: Call,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.40, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 420,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY B — MEAN REVERSION (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "RSIRecovery_Bull_Put_Sell", Type: Put,
			StrikePctOTM: 0.016, ExpiryMinutes: 150,
			TakeProfitPct: 0.42, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 900,
		},
		{
			Name: "RSIFade_Bear_Call_Sell", Type: Call,
			StrikePctOTM: 0.016, ExpiryMinutes: 150,
			TakeProfitPct: 0.42, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 900,
		},
		{
			Name: "Overextension_Fade_Call_Sell", Type: Call,
			StrikePctOTM: 0.015, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.85,
			PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 1200,
		},
		{
			Name: "Overextension_Fade_Put_Sell", Type: Put,
			StrikePctOTM: 0.015, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.85,
			PositionUSD: 10000, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 1200,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY C — TECHNICAL BREAKOUT (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "EMA_BullCross_Put_Sell", Type: Put,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.38, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "EMA_BULL_CROSS", CooldownSecs: 600,
		},
		{
			Name: "EMA_BearCross_Call_Sell", Type: Call,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.38, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "EMA_BEAR_CROSS", CooldownSecs: 600,
		},
		{
			Name: "Resistance_Breakout_Put_Sell", Type: Put,
			StrikePctOTM: 0.011, ExpiryMinutes: 150,
			TakeProfitPct: 0.40, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "RESISTANCE_BREAK", CooldownSecs: 720,
		},
		{
			Name: "Support_Breakdown_Call_Sell", Type: Call,
			StrikePctOTM: 0.011, ExpiryMinutes: 150,
			TakeProfitPct: 0.40, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "SUPPORT_BREAK", CooldownSecs: 720,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY D — SPECIAL SITUATIONS (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "Capitulation_VReversal_Put_Sell", Type: Put,
			StrikePctOTM: 0.018, ExpiryMinutes: 180,
			TakeProfitPct: 0.48, StopLossPct: 0.80,
			PositionUSD: 10000, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 1200,
		},
		{
			Name: "SessionOpen_Bull_Put_Sell", Type: Put,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.36, StopLossPct: 0.65,
			PositionUSD: 10000, Signal: "SESSION_OPEN_BULL", CooldownSecs: 720,
		},
		{
			Name: "SessionOpen_Bear_Call_Sell", Type: Call,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.36, StopLossPct: 0.65,
			PositionUSD: 10000, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 720,
		},
		{
			Name: "VolCompress_Breakout_Bull_Put", Type: Put,
			StrikePctOTM: 0.013, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.78,
			PositionUSD: 10000, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 900,
		},
		{
			Name: "VolCompress_Breakout_Bear_Call", Type: Call,
			StrikePctOTM: 0.013, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.78,
			PositionUSD: 10000, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 900,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY E — SELECTIVE ALPHA (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "VWAP_Continuation_Bull_Put", Type: Put,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.38, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "VWAP_ABOVE", CooldownSecs: 540,
		},
		{
			Name: "VWAP_Continuation_Bear_Call", Type: Call,
			StrikePctOTM: 0.010, ExpiryMinutes: 135,
			TakeProfitPct: 0.38, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "VWAP_BELOW", CooldownSecs: 540,
		},
		{
			Name: "TripleConfluence_Bull_Put", Type: Put,
			StrikePctOTM: 0.014, ExpiryMinutes: 165,
			TakeProfitPct: 0.44, StopLossPct: 0.74,
			PositionUSD: 10000, Signal: "TRIPLE_BULL", CooldownSecs: 900,
		},
		{
			Name: "TripleConfluence_Bear_Call", Type: Call,
			StrikePctOTM: 0.014, ExpiryMinutes: 165,
			TakeProfitPct: 0.44, StopLossPct: 0.74,
			PositionUSD: 10000, Signal: "TRIPLE_BEAR", CooldownSecs: 900,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY F — TREND AND STRUCTURE (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "TrendAlignment_Bull_Put", Type: Put,
			StrikePctOTM: 0.010, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 600,
		},
		{
			Name: "TrendAlignment_Bear_Call", Type: Call,
			StrikePctOTM: 0.010, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.68,
			PositionUSD: 10000, Signal: "EMA_BELOW_BOTH", CooldownSecs: 600,
		},
		{
			Name: "BBSqueeze_Release_Bull_Put", Type: Put,
			StrikePctOTM: 0.012, ExpiryMinutes: 150,
			TakeProfitPct: 0.42, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 540,
		},
		{
			Name: "BBSqueeze_Release_Bear_Call", Type: Call,
			StrikePctOTM: 0.012, ExpiryMinutes: 150,
			TakeProfitPct: 0.42, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 540,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY G — HYBRID PRO SERIES (SELLERS)
		// ═══════════════════════════════════════════════════════════════════════

		{
			Name: "MomentumVWAP_Pro_Bull_Put", Type: Put,
			StrikePctOTM: 0.012, ExpiryMinutes: 150,
			TakeProfitPct: 0.40, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 600,
		},
		{
			Name: "MomentumVWAP_Pro_Bear_Call", Type: Call,
			StrikePctOTM: 0.012, ExpiryMinutes: 150,
			TakeProfitPct: 0.40, StopLossPct: 0.72,
			PositionUSD: 10000, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 600,
		},
		{
			Name: "BreakoutTrend_Pro_Bull_Put", Type: Put,
			StrikePctOTM: 0.013, ExpiryMinutes: 165,
			TakeProfitPct: 0.44, StopLossPct: 0.78,
			PositionUSD: 10000, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 720,
		},
		{
			Name: "BreakdownTrend_Pro_Bear_Call", Type: Call,
			StrikePctOTM: 0.013, ExpiryMinutes: 165,
			TakeProfitPct: 0.44, StopLossPct: 0.78,
			PositionUSD: 10000, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 720,
		},
		{
			Name: "Capitulation_Reclaim_Elite_Put", Type: Put,
			StrikePctOTM: 0.018, ExpiryMinutes: 210,
			TakeProfitPct: 0.52, StopLossPct: 0.88,
			PositionUSD: 10000, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 1800,
		},
	}
}
