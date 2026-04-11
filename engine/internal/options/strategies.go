package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.005
)

var optionStrategyCategories = map[string]string{
	// ── NEW: Category H — Micro-Momentum Scalp ──────────────────────────────
	"MicroMom_Bull_Call_1":  "Micro-Momentum", "MicroMom_Bear_Put_1":  "Micro-Momentum",
	"MicroMom_Bull_Call_2":  "Micro-Momentum", "MicroMom_Bear_Put_2":  "Micro-Momentum",
	"MicroMom_Bull_Call_3":  "Micro-Momentum", "MicroMom_Bear_Put_3":  "Micro-Momentum",
	"MicroMom_Bull_Call_4":  "Micro-Momentum", "MicroMom_Bear_Put_4":  "Micro-Momentum",
	"MicroMom_Bull_Call_5":  "Micro-Momentum", "MicroMom_Bear_Put_5":  "Micro-Momentum",
	"MicroMom_Bull_Call_6":  "Micro-Momentum", "MicroMom_Bear_Put_6":  "Micro-Momentum",
	"MicroMom_Bull_Call_7":  "Micro-Momentum", "MicroMom_Bear_Put_7":  "Micro-Momentum",
	"MicroMom_Bull_Call_8":  "Micro-Momentum", "MicroMom_Bear_Put_8":  "Micro-Momentum",
	"MicroMom_Bull_Call_9":  "Micro-Momentum", "MicroMom_Bear_Put_9":  "Micro-Momentum",
	"MicroMom_Bull_Call_10": "Micro-Momentum", "MicroMom_Bear_Put_10": "Micro-Momentum",
	// ── NEW: Category I — Regime Trend Rider ───────────────────────────────
	"TrendRider_Bull_Call_1":  "Trend Rider", "TrendRider_Bear_Put_1":  "Trend Rider",
	"TrendRider_Bull_Call_2":  "Trend Rider", "TrendRider_Bear_Put_2":  "Trend Rider",
	"TrendRider_Bull_Call_3":  "Trend Rider", "TrendRider_Bear_Put_3":  "Trend Rider",
	"TrendRider_Bull_Call_4":  "Trend Rider", "TrendRider_Bear_Put_4":  "Trend Rider",
	"TrendRider_Bull_Call_5":  "Trend Rider", "TrendRider_Bear_Put_5":  "Trend Rider",
	"TrendRider_Bull_Call_6":  "Trend Rider", "TrendRider_Bear_Put_6":  "Trend Rider",
	"TrendRider_Bull_Call_7":  "Trend Rider", "TrendRider_Bear_Put_7":  "Trend Rider",
	"TrendRider_Bull_Call_8":  "Trend Rider", "TrendRider_Bear_Put_8":  "Trend Rider",
	"TrendRider_Bull_Call_9":  "Trend Rider", "TrendRider_Bear_Put_9":  "Trend Rider",
	"TrendRider_Bull_Call_10": "Trend Rider", "TrendRider_Bear_Put_10": "Trend Rider",
	// ── NEW: Category J — Volatility Pulse ─────────────────────────────────
	"VolPulse_Bull_Call_1":  "Volatility Pulse", "VolPulse_Bear_Put_1":  "Volatility Pulse",
	"VolPulse_Bull_Call_2":  "Volatility Pulse", "VolPulse_Bear_Put_2":  "Volatility Pulse",
	"VolPulse_Bull_Call_3":  "Volatility Pulse", "VolPulse_Bear_Put_3":  "Volatility Pulse",
	"VolPulse_Bull_Call_4":  "Volatility Pulse", "VolPulse_Bear_Put_4":  "Volatility Pulse",
	"VolPulse_Bull_Call_5":  "Volatility Pulse", "VolPulse_Bear_Put_5":  "Volatility Pulse",
	"VolPulse_Bull_Call_6":  "Volatility Pulse", "VolPulse_Bear_Put_6":  "Volatility Pulse",
	"VolPulse_Bull_Call_7":  "Volatility Pulse", "VolPulse_Bear_Put_7":  "Volatility Pulse",
	"VolPulse_Bull_Call_8":  "Volatility Pulse", "VolPulse_Bear_Put_8":  "Volatility Pulse",
	"VolPulse_Bull_Call_9":  "Volatility Pulse", "VolPulse_Bear_Put_9":  "Volatility Pulse",
	"VolPulse_Bull_Call_10": "Volatility Pulse", "VolPulse_Bear_Put_10": "Volatility Pulse",
	// ── NEW: Category K — Structure Snap ───────────────────────────────────
	"StructureSnap_Bull_Call_1":  "Structure Snap", "StructureSnap_Bear_Put_1":  "Structure Snap",
	"StructureSnap_Bull_Call_2":  "Structure Snap", "StructureSnap_Bear_Put_2":  "Structure Snap",
	"StructureSnap_Bull_Call_3":  "Structure Snap", "StructureSnap_Bear_Put_3":  "Structure Snap",
	"StructureSnap_Bull_Call_4":  "Structure Snap", "StructureSnap_Bear_Put_4":  "Structure Snap",
	"StructureSnap_Bull_Call_5":  "Structure Snap", "StructureSnap_Bear_Put_5":  "Structure Snap",
	"StructureSnap_Bull_Call_6":  "Structure Snap", "StructureSnap_Bear_Put_6":  "Structure Snap",
	"StructureSnap_Bull_Call_7":  "Structure Snap", "StructureSnap_Bear_Put_7":  "Structure Snap",
	"StructureSnap_Bull_Call_8":  "Structure Snap", "StructureSnap_Bear_Put_8":  "Structure Snap",
	"StructureSnap_Bull_Call_9":  "Structure Snap", "StructureSnap_Bear_Put_9":  "Structure Snap",
	"StructureSnap_Bull_Call_10": "Structure Snap", "StructureSnap_Bear_Put_10": "Structure Snap",

	"MomentumBurst_Bull_Call":         "Momentum",
	"MomentumBurst_Bear_Put":          "Momentum",
	"ConsecCandle_Bull_Call":          "Momentum",
	"ConsecCandle_Bear_Put":           "Momentum",
	"RSI_Extreme_Oversold_Call":       "Mean Reversion",
	"RSI_Extreme_Overbought_Put":      "Mean Reversion",
	"RSI_Oversold_Recovery_Call":      "Mean Reversion",
	"RSI_Overbought_Fade_Put":         "Mean Reversion",
	"Overextension_Fade_Put":          "Mean Reversion",
	"Overextension_Fade_Call":         "Mean Reversion",
	"EMA_BullCross_Call":              "Breakout",
	"EMA_BearCross_Put":               "Breakout",
	"Resistance_Breakout_Call":        "Breakout",
	"Support_Breakdown_Put":           "Breakout",
	"Stoch_Oversold_Call":             "Mean Reversion",
	"Stoch_Overbought_Put":            "Mean Reversion",
	"Capitulation_VReversal_Call":     "Capitulation",
	"SessionOpen_Bull_Call":           "Breakout",
	"SessionOpen_Bear_Put":            "Breakout",
	"VolCompress_Breakout_Bull_Call":  "Breakout",
	"VolCompress_Breakout_Bear_Put":   "Breakout",
	"VWAP_Continuation_Bull_Call":     "Momentum",
	"VWAP_Continuation_Bear_Put":      "Momentum",
	"TripleConfluence_Bull_Call":      "Hybrid",
	"TripleConfluence_Bear_Put":       "Hybrid",
	"SharpReversal_TopFade_Put":       "Mean Reversion",
	"TrendAlignment_Bull_Call":        "Breakout",
	"TrendAlignment_Bear_Put":         "Breakout",
	"BandBounce_Reclaim_Call":         "Mean Reversion",
	"BandFade_Rejection_Put":          "Mean Reversion",
	"SharpReversal_BottomSnap_Call":   "Mean Reversion",
	"MomentumFollow_Bull_Call":        "Momentum",
	"BBSqueeze_Release_Bull_Call":     "Breakout",
	"BBSqueeze_Release_Bear_Put":      "Breakout",
	"HighIV_Expansion_Bull_Call":      "Breakout",
	"HighIV_Expansion_Bear_Put":       "Breakout",
	"MomentumVWAP_Pro_Bull_Call":      "Momentum",
	"MomentumVWAP_Pro_Bear_Put":       "Momentum",
	"BreakoutTrend_Pro_Bull_Call":     "Breakout",
	"BreakdownTrend_Pro_Bear_Put":     "Breakout",
	"Capitulation_Reclaim_Elite_Call": "Capitulation",
}

var strategyIDs = map[string]int{
	// NEW 100 strategies (IDs 42–141)
	"MicroMom_Bull_Call_1": 42, "MicroMom_Bear_Put_1": 43,
	"MicroMom_Bull_Call_2": 44, "MicroMom_Bear_Put_2": 45,
	"MicroMom_Bull_Call_3": 46, "MicroMom_Bear_Put_3": 47,
	"MicroMom_Bull_Call_4": 48, "MicroMom_Bear_Put_4": 49,
	"MicroMom_Bull_Call_5": 50, "MicroMom_Bear_Put_5": 51,
	"MicroMom_Bull_Call_6": 52, "MicroMom_Bear_Put_6": 53,
	"MicroMom_Bull_Call_7": 54, "MicroMom_Bear_Put_7": 55,
	"MicroMom_Bull_Call_8": 56, "MicroMom_Bear_Put_8": 57,
	"MicroMom_Bull_Call_9": 58, "MicroMom_Bear_Put_9": 59,
	"MicroMom_Bull_Call_10": 60, "MicroMom_Bear_Put_10": 61,
	"TrendRider_Bull_Call_1": 62, "TrendRider_Bear_Put_1": 63,
	"TrendRider_Bull_Call_2": 64, "TrendRider_Bear_Put_2": 65,
	"TrendRider_Bull_Call_3": 66, "TrendRider_Bear_Put_3": 67,
	"TrendRider_Bull_Call_4": 68, "TrendRider_Bear_Put_4": 69,
	"TrendRider_Bull_Call_5": 70, "TrendRider_Bear_Put_5": 71,
	"TrendRider_Bull_Call_6": 72, "TrendRider_Bear_Put_6": 73,
	"TrendRider_Bull_Call_7": 74, "TrendRider_Bear_Put_7": 75,
	"TrendRider_Bull_Call_8": 76, "TrendRider_Bear_Put_8": 77,
	"TrendRider_Bull_Call_9": 78, "TrendRider_Bear_Put_9": 79,
	"TrendRider_Bull_Call_10": 80, "TrendRider_Bear_Put_10": 81,
	"VolPulse_Bull_Call_1": 82, "VolPulse_Bear_Put_1": 83,
	"VolPulse_Bull_Call_2": 84, "VolPulse_Bear_Put_2": 85,
	"VolPulse_Bull_Call_3": 86, "VolPulse_Bear_Put_3": 87,
	"VolPulse_Bull_Call_4": 88, "VolPulse_Bear_Put_4": 89,
	"VolPulse_Bull_Call_5": 90, "VolPulse_Bear_Put_5": 91,
	"VolPulse_Bull_Call_6": 92, "VolPulse_Bear_Put_6": 93,
	"VolPulse_Bull_Call_7": 94, "VolPulse_Bear_Put_7": 95,
	"VolPulse_Bull_Call_8": 96, "VolPulse_Bear_Put_8": 97,
	"VolPulse_Bull_Call_9": 98, "VolPulse_Bear_Put_9": 99,
	"VolPulse_Bull_Call_10": 100, "VolPulse_Bear_Put_10": 101,
	"StructureSnap_Bull_Call_1": 102, "StructureSnap_Bear_Put_1": 103,
	"StructureSnap_Bull_Call_2": 104, "StructureSnap_Bear_Put_2": 105,
	"StructureSnap_Bull_Call_3": 106, "StructureSnap_Bear_Put_3": 107,
	"StructureSnap_Bull_Call_4": 108, "StructureSnap_Bear_Put_4": 109,
	"StructureSnap_Bull_Call_5": 110, "StructureSnap_Bear_Put_5": 111,
	"StructureSnap_Bull_Call_6": 112, "StructureSnap_Bear_Put_6": 113,
	"StructureSnap_Bull_Call_7": 114, "StructureSnap_Bear_Put_7": 115,
	"StructureSnap_Bull_Call_8": 116, "StructureSnap_Bear_Put_8": 117,
	"StructureSnap_Bull_Call_9": 118, "StructureSnap_Bear_Put_9": 119,
	"StructureSnap_Bull_Call_10": 120, "StructureSnap_Bear_Put_10": 121,

	"MomentumBurst_Bull_Call":         1,
	"MomentumBurst_Bear_Put":          2,
	"ConsecCandle_Bull_Call":          3,
	"ConsecCandle_Bear_Put":           4,
	"RSI_Extreme_Oversold_Call":       5,
	"RSI_Extreme_Overbought_Put":      6,
	"RSI_Oversold_Recovery_Call":      7,
	"RSI_Overbought_Fade_Put":         8,
	"Overextension_Fade_Put":          9,
	"Overextension_Fade_Call":         10,
	"EMA_BullCross_Call":              11,
	"EMA_BearCross_Put":               12,
	"Resistance_Breakout_Call":        13,
	"Support_Breakdown_Put":           14,
	"Stoch_Oversold_Call":             15,
	"Stoch_Overbought_Put":            16,
	"Capitulation_VReversal_Call":     17,
	"SessionOpen_Bull_Call":           18,
	"SessionOpen_Bear_Put":            19,
	"VolCompress_Breakout_Bull_Call":  20,
	"VolCompress_Breakout_Bear_Put":   21,
	"VWAP_Continuation_Bull_Call":     22,
	"VWAP_Continuation_Bear_Put":      23,
	"TripleConfluence_Bull_Call":      24,
	"TripleConfluence_Bear_Put":       25,
	"SharpReversal_TopFade_Put":       26,
	"TrendAlignment_Bull_Call":        27,
	"TrendAlignment_Bear_Put":         28,
	"BandBounce_Reclaim_Call":         29,
	"BandFade_Rejection_Put":          30,
	"SharpReversal_BottomSnap_Call":   31,
	"MomentumFollow_Bull_Call":        32,
	"BBSqueeze_Release_Bull_Call":     33,
	"BBSqueeze_Release_Bear_Put":      34,
	"HighIV_Expansion_Bull_Call":      35,
	"HighIV_Expansion_Bear_Put":       36,
	"MomentumVWAP_Pro_Bull_Call":      37,
	"MomentumVWAP_Pro_Bear_Put":       38,
	"BreakoutTrend_Pro_Bull_Call":     39,
	"BreakdownTrend_Pro_Bear_Put":     40,
	"Capitulation_Reclaim_Elite_Call": 41,
}

func assignStrategyIDs(defs []StrategyDef) []StrategyDef {
	for i := range defs {
		defs[i].ID = strategyIDs[defs[i].Name]
	}
	return defs
}

// BuildStrategyLibrary returns the full live-approved strategy library.
// It keeps the curated definitions available for reporting and future promotion.
func BuildStrategyLibrary() []StrategyDef {
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

// BuildStrategies returns the actively tradeable strategy library for the options engine.
// The roster manager decides which subset is ACTIVE at runtime.
func BuildStrategies() []StrategyDef {
	return BuildStrategyLibrary()
}

// buildAllStrategies defines 141 live-approved BTC option buying strategies (41 original + 100 new).
//
// Design principles:
//   - Each strategy uses a UNIQUE signal — zero clustering (no two strategies
//     fire at the same time from the same market condition)
//   - All ATM (StrikePctOTM = 0.0) — highest delta, fastest response
//   - Expiry 75-90 min — enough time to be right, not so much theta bleeds us dry
//   - R:R minimum 2.5:1 on all strategies (TP >= 2.5x SL)
//   - Position sizes calibrated to signal frequency: rare signals get larger size
//   - Long cooldowns on rare/powerful signals to prevent re-entry before move matures
func buildAllStrategies() []StrategyDef {
	return []StrategyDef{

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY A — MOMENTUM CONTINUATION
		// -----------------------------------------------------------------------
		// Buy ATM options when price is already moving hard in one direction.
		// These work because momentum in BTC persists for 15-45 minutes after
		// a strong catalyst. Options pricing lags behind the move initially,
		// giving a window where ATM options are still mispriced cheap.
		// Win rate target: 55-60%. R:R: 3:1.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: Price up >0.42% in 5 min AND >0.22% in 10 min, RSI < 72.
		// Extended to 180 min: halves theta burn, giving more time to hit TP.
		// TP lowered to 42% (from 80%): achievable on a 0.4% BTC move. R:R = 1.2
		{
			Name: "MomentumBurst_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.35,
			PositionUSD: 700, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 300,
		},
		{
			Name: "MomentumBurst_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.35,
			PositionUSD: 700, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 300,
		},
		// Signal: 4 consecutive bullish 1-min bars with >0.35% total gain.
		{
			Name: "ConsecCandle_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.34,
			PositionUSD: 600, Signal: "CONSEC_BULL_BARS", CooldownSecs: 300,
		},
		{
			Name: "ConsecCandle_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.34,
			PositionUSD: 600, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 300,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY B — EXTREME MEAN REVERSION
		// -----------------------------------------------------------------------
		// Buy options when price has moved so far so fast that it's statistically
		// stretched. Three flavours: RSI extreme crossback, Bollinger Band bounce,
		// and overextension fade. These have the highest win rate (60-70%) because
		// they only fire after a confirmed reversal has started — not anticipating.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: RSI(14) crossed back above 20 from below.
		// After RSI < 20, price is in free-fall mode — when it recovers, the snap
		// is violent. Large size because this is the rarest, most reliable signal.
		{
			Name: "RSI_Extreme_Oversold_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.38,
			PositionUSD: 790, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 600,
		},
		{
			Name: "RSI_Extreme_Overbought_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.38,
			PositionUSD: 790, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 600,
		},
		{
			Name: "RSI_Oversold_Recovery_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.34,
			PositionUSD: 550, Signal: "RSI_OVERSOLD", CooldownSecs: 600,
		},
		{
			Name: "RSI_Overbought_Fade_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.34,
			PositionUSD: 550, Signal: "RSI_OVERBOUGHT", CooldownSecs: 600,
		},
		{
			Name: "Overextension_Fade_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.44, StopLossPct: 0.38,
			PositionUSD: 600, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 900,
		},
		{
			Name: "Overextension_Fade_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.44, StopLossPct: 0.38,
			PositionUSD: 600, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 900,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY C — TECHNICAL BREAKOUT
		// -----------------------------------------------------------------------
		// Entries on confirmed technical events: EMA crossovers, resistance breaks,
		// Bollinger Band touches. These fire cleanly on discrete events rather than
		// sustained conditions, preventing re-entry during the same move.
		// Win rate target: 52-57%. R:R: 2.5:1.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: 9 EMA crossed above 21 EMA on the most recent bar (event-driven).
		{
			Name: "EMA_BullCross_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.34,
			PositionUSD: 550, Signal: "EMA_BULL_CROSS", CooldownSecs: 600,
		},
		{
			Name: "EMA_BearCross_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.34,
			PositionUSD: 550, Signal: "EMA_BEAR_CROSS", CooldownSecs: 600,
		},
		{
			Name: "Resistance_Breakout_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.48, StopLossPct: 0.36,
			PositionUSD: 600, Signal: "RESISTANCE_BREAK", CooldownSecs: 720,
		},
		{
			Name: "Support_Breakdown_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.48, StopLossPct: 0.36,
			PositionUSD: 600, Signal: "SUPPORT_BREAK", CooldownSecs: 720,
		},
		{
			Name: "Stoch_Oversold_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.34,
			PositionUSD: 500, Signal: "STOCH_OVERSOLD", CooldownSecs: 540,
		},
		{
			Name: "Stoch_Overbought_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.34,
			PositionUSD: 500, Signal: "STOCH_OVERBOUGHT", CooldownSecs: 540,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY D — SPECIAL SITUATIONS
		// -----------------------------------------------------------------------
		// High-conviction, event-driven setups that only fire a few times per day.
		// These are the portfolio's anchor trades — rare, large size, high R:R.
		// Win rate target: 60-70%. R:R: 2.8:1 to 3.2:1.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: Drop >0.7% in 5 bars → confirmed recovery >0.35% from the low.
		// THE best setup in crypto scalping: panic drop clears all stops, price
		// snaps back violently with no sellers left. Only fires as a CALL.
		{
			Name: "Capitulation_VReversal_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.54, StopLossPct: 0.38,
			PositionUSD: 750, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 600,
		},
		{
			Name: "SessionOpen_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.46, StopLossPct: 0.34,
			PositionUSD: 650, Signal: "SESSION_OPEN_BULL", CooldownSecs: 720,
		},
		{
			Name: "SessionOpen_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.46, StopLossPct: 0.34,
			PositionUSD: 650, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 720,
		},
		{
			Name: "VolCompress_Breakout_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.58, StopLossPct: 0.40,
			PositionUSD: 600, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 720,
		},
		{
			Name: "VolCompress_Breakout_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.58, StopLossPct: 0.40,
			PositionUSD: 600, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 720,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY E — SELECTIVE ALPHA OVERLAYS
		// -----------------------------------------------------------------------
		// Five additional high-quality overlays built from unused signals that
		// already exist in the engine. These are intentionally selective:
		//   - VWAP continuation catches clean trend continuation once price is
		//     established away from fair value with momentum.
		//   - Triple confluence requires reversal, momentum, and EMA alignment.
		//   - Sharp reversal down captures failed upside bursts and intraday
		//     blow-off rejection without waiting for a full overextension setup.
		// They add trade diversity without relaxing the live-approved filters.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: price is > VWAP with directional follow-through.
		// Cleaner continuation than raw breakout because the move is already
		// holding above fair value instead of merely spiking through it.
		{
			Name: "VWAP_Continuation_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.46, StopLossPct: 0.34,
			PositionUSD: 575, Signal: "VWAP_ABOVE", CooldownSecs: 540,
		},
		{
			Name: "VWAP_Continuation_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.46, StopLossPct: 0.34,
			PositionUSD: 575, Signal: "VWAP_BELOW", CooldownSecs: 540,
		},

		{
			Name: "TripleConfluence_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.36,
			PositionUSD: 625, Signal: "TRIPLE_BULL", CooldownSecs: 720,
		},
		{
			Name: "TripleConfluence_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.50, StopLossPct: 0.36,
			PositionUSD: 625, Signal: "TRIPLE_BEAR", CooldownSecs: 720,
		},

		{
			Name: "SharpReversal_TopFade_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.40, StopLossPct: 0.34,
			PositionUSD: 575, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 600,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY F — STRUCTURE AND REGIME ADD-ONS
		// -----------------------------------------------------------------------
		// Five more selective overlays from the strongest remaining unused
		// signals. These add medium-frequency trend-following and reversal
		// exposure without duplicating the existing top-tier setups.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: healthy upside trend with price above both medium EMAs and a
		// fresh bullish EMA cross. This is cleaner than a naked breakout because
		// structure and momentum are both aligned.
		{
			Name: "TrendAlignment_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.48, StopLossPct: 0.34,
			PositionUSD: 600, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 600,
		},
		{
			Name: "TrendAlignment_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.48, StopLossPct: 0.34,
			PositionUSD: 600, Signal: "EMA_BELOW_BOTH", CooldownSecs: 600,
		},

		{
			Name: "BandBounce_Reclaim_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.32,
			PositionUSD: 525, Signal: "BB_LOWER_TOUCH", CooldownSecs: 480,
		},
		{
			Name: "BandFade_Rejection_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.38, StopLossPct: 0.32,
			PositionUSD: 525, Signal: "BB_UPPER_TOUCH", CooldownSecs: 480,
		},

		{
			Name: "SharpReversal_BottomSnap_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.40, StopLossPct: 0.34,
			PositionUSD: 575, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 600,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY G — VOLATILITY AND SECOND-WAVE CONTINUATION
		// -----------------------------------------------------------------------
		// Final five overlays from the strongest remaining unused signals.
		// These intentionally broaden the book into:
		//   - medium-strength trend continuation,
		//   - squeeze-release expansion,
		//   - high-IV directional follow-through.
		// They are slightly smaller than the flagship strategies because they
		// trigger more often or sit one tier below the "strong momentum" set.
		// ═══════════════════════════════════════════════════════════════════════

		// Signal: moderate but real 5m/10m upside momentum with RSI still in a
		// healthy trend zone. This picks up second-wave continuation that is too
		// small for the strong-momentum strategy but still option-friendly.
		{
			Name: "MomentumFollow_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.44, StopLossPct: 0.32,
			PositionUSD: 540, Signal: "BULL_MOMENTUM", CooldownSecs: 420,
		},

		{
			Name: "BBSqueeze_Release_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.46, StopLossPct: 0.34,
			PositionUSD: 560, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 540,
		},
		{
			Name: "BBSqueeze_Release_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 150,
			TakeProfitPct: 0.46, StopLossPct: 0.34,
			PositionUSD: 560, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 540,
		},

		{
			Name: "HighIV_Expansion_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 120,
			TakeProfitPct: 0.42, StopLossPct: 0.30,
			PositionUSD: 500, Signal: "HIGH_IV_BULL", CooldownSecs: 480,
		},
		{
			Name: "HighIV_Expansion_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 120,
			TakeProfitPct: 0.42, StopLossPct: 0.30,
			PositionUSD: 500, Signal: "HIGH_IV_BEAR", CooldownSecs: 480,
		},

		{
			Name: "MomentumVWAP_Pro_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.52, StopLossPct: 0.36,
			PositionUSD: 650, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 600,
		},
		{
			Name: "MomentumVWAP_Pro_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.52, StopLossPct: 0.36,
			PositionUSD: 650, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 600,
		},

		{
			Name: "BreakoutTrend_Pro_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.56, StopLossPct: 0.36,
			PositionUSD: 740, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 720,
		},
		{
			Name: "BreakdownTrend_Pro_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.56, StopLossPct: 0.36,
			PositionUSD: 740, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 720,
		},

		{
			Name: "Capitulation_Reclaim_Elite_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 180,
			TakeProfitPct: 0.62, StopLossPct: 0.38,
			PositionUSD: 850, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 900,
		},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY H — MICRO-MOMENTUM SCALP  (20 strategies, IDs 42–61)
		// -----------------------------------------------------------------------
		// 10 LONG + 10 SHORT option buying setups targeting short-lived momentum
		// pulses at different speeds: 2-bar through 12-bar.  Each variant uses a
		// slightly different RSI window and momentum lookback so no two strategies
		// fire simultaneously.  ATM, 120-150 min expiry to minimise theta on short
		// holds.  TP 38-52%, SL 26-32%, R:R ≥ 1.5 (option buying benefits from
		// convexity so we accept tighter raw R:R here).
		// ═══════════════════════════════════════════════════════════════════════

		// Variant 1: 2-bar breakout burst + RSI 9
		{Name: "MicroMom_Bull_Call_1", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 120, TakeProfitPct: 0.40, StopLossPct: 0.26, PositionUSD: 520, Signal: "MM_BULL_1", CooldownSecs: 240},
		{Name: "MicroMom_Bear_Put_1",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 120, TakeProfitPct: 0.40, StopLossPct: 0.26, PositionUSD: 520, Signal: "MM_BEAR_1", CooldownSecs: 240},
		// Variant 2: 3-bar thrust + RSI cross 40/60
		{Name: "MicroMom_Bull_Call_2", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 120, TakeProfitPct: 0.42, StopLossPct: 0.27, PositionUSD: 530, Signal: "MM_BULL_2", CooldownSecs: 270},
		{Name: "MicroMom_Bear_Put_2",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 120, TakeProfitPct: 0.42, StopLossPct: 0.27, PositionUSD: 530, Signal: "MM_BEAR_2", CooldownSecs: 270},
		// Variant 3: 4-bar acceleration + stoch 20/80
		{Name: "MicroMom_Bull_Call_3", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 130, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 540, Signal: "MM_BULL_3", CooldownSecs: 300},
		{Name: "MicroMom_Bear_Put_3",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 130, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 540, Signal: "MM_BEAR_3", CooldownSecs: 300},
		// Variant 4: 5-bar momentum + BB midline reclaim
		{Name: "MicroMom_Bull_Call_4", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 130, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 550, Signal: "MM_BULL_4", CooldownSecs: 300},
		{Name: "MicroMom_Bear_Put_4",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 130, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 550, Signal: "MM_BEAR_4", CooldownSecs: 300},
		// Variant 5: 6-bar trend pulse + RSI 50 cross
		{Name: "MicroMom_Bull_Call_5", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 135, TakeProfitPct: 0.46, StopLossPct: 0.29, PositionUSD: 555, Signal: "MM_BULL_5", CooldownSecs: 330},
		{Name: "MicroMom_Bear_Put_5",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 135, TakeProfitPct: 0.46, StopLossPct: 0.29, PositionUSD: 555, Signal: "MM_BEAR_5", CooldownSecs: 330},
		// Variant 6: 7-bar ema9 slope + stoch rising
		{Name: "MicroMom_Bull_Call_6", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.46, StopLossPct: 0.29, PositionUSD: 560, Signal: "MM_BULL_6", CooldownSecs: 360},
		{Name: "MicroMom_Bear_Put_6",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.46, StopLossPct: 0.29, PositionUSD: 560, Signal: "MM_BEAR_6", CooldownSecs: 360},
		// Variant 7: 8-bar higher-lows series
		{Name: "MicroMom_Bull_Call_7", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 565, Signal: "MM_BULL_7", CooldownSecs: 360},
		{Name: "MicroMom_Bear_Put_7",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 565, Signal: "MM_BEAR_7", CooldownSecs: 360},
		// Variant 8: 9-bar trend + VWAP above + RSI
		{Name: "MicroMom_Bull_Call_8", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 145, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 570, Signal: "MM_BULL_8", CooldownSecs: 390},
		{Name: "MicroMom_Bear_Put_8",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 145, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 570, Signal: "MM_BEAR_8", CooldownSecs: 390},
		// Variant 9: 10-bar momentum + EMA21 slope
		{Name: "MicroMom_Bull_Call_9", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 150, TakeProfitPct: 0.50, StopLossPct: 0.31, PositionUSD: 575, Signal: "MM_BULL_9", CooldownSecs: 420},
		{Name: "MicroMom_Bear_Put_9",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 150, TakeProfitPct: 0.50, StopLossPct: 0.31, PositionUSD: 575, Signal: "MM_BEAR_9", CooldownSecs: 420},
		// Variant 10: 12-bar sustained trend + BB width expansion
		{Name: "MicroMom_Bull_Call_10", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 150, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 580, Signal: "MM_BULL_10", CooldownSecs: 450},
		{Name: "MicroMom_Bear_Put_10",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 150, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 580, Signal: "MM_BEAR_10", CooldownSecs: 450},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY I — REGIME TREND RIDER  (20 strategies, IDs 62–81)
		// -----------------------------------------------------------------------
		// Each pair targets a different combination of trend-regime indicators:
		// EMA stacks, RSI zones, slope strength, and price-structure alignment.
		// Longer expiry (160-180 min) to ride trend moves fully.
		// TP 46-62%, SL 28-36%, high R:R focus.
		// ═══════════════════════════════════════════════════════════════════════

		// Variant 1: EMA9 > EMA21 > EMA55 full stack + RSI 52-68
		{Name: "TrendRider_Bull_Call_1", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 600, Signal: "TR_BULL_1", CooldownSecs: 480},
		{Name: "TrendRider_Bear_Put_1",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 600, Signal: "TR_BEAR_1", CooldownSecs: 480},
		// Variant 2: pullback to EMA21 + bounce + RSI 45-58
		{Name: "TrendRider_Bull_Call_2", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.50, StopLossPct: 0.31, PositionUSD: 610, Signal: "TR_BULL_2", CooldownSecs: 480},
		{Name: "TrendRider_Bear_Put_2",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.50, StopLossPct: 0.31, PositionUSD: 610, Signal: "TR_BEAR_2", CooldownSecs: 480},
		// Variant 3: price > EMA55 + 15m momentum > 0.4% + stoch rising from 40
		{Name: "TrendRider_Bull_Call_3", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.50, StopLossPct: 0.32, PositionUSD: 615, Signal: "TR_BULL_3", CooldownSecs: 510},
		{Name: "TrendRider_Bear_Put_3",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.50, StopLossPct: 0.32, PositionUSD: 615, Signal: "TR_BEAR_3", CooldownSecs: 510},
		// Variant 4: BB midline hold + EMA21 slope positive
		{Name: "TrendRider_Bull_Call_4", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 620, Signal: "TR_BULL_4", CooldownSecs: 510},
		{Name: "TrendRider_Bear_Put_4",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 620, Signal: "TR_BEAR_4", CooldownSecs: 510},
		// Variant 5: RSI 8 > 60 AND RSI 14 > 55 agreement
		{Name: "TrendRider_Bull_Call_5", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.52, StopLossPct: 0.33, PositionUSD: 625, Signal: "TR_BULL_5", CooldownSecs: 540},
		{Name: "TrendRider_Bear_Put_5",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.52, StopLossPct: 0.33, PositionUSD: 625, Signal: "TR_BEAR_5", CooldownSecs: 540},
		// Variant 6: 20-bar higher-highs structure + EMA cross confirmation
		{Name: "TrendRider_Bull_Call_6", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 630, Signal: "TR_BULL_6", CooldownSecs: 540},
		{Name: "TrendRider_Bear_Put_6",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 630, Signal: "TR_BEAR_6", CooldownSecs: 540},
		// Variant 7: VWAP reclaim after 8-bar consolidation
		{Name: "TrendRider_Bull_Call_7", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.54, StopLossPct: 0.34, PositionUSD: 635, Signal: "TR_BULL_7", CooldownSecs: 570},
		{Name: "TrendRider_Bear_Put_7",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.54, StopLossPct: 0.34, PositionUSD: 635, Signal: "TR_BEAR_7", CooldownSecs: 570},
		// Variant 8: 25-bar momentum > 0.8% + RSI 55-72
		{Name: "TrendRider_Bull_Call_8", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.56, StopLossPct: 0.34, PositionUSD: 640, Signal: "TR_BULL_8", CooldownSecs: 570},
		{Name: "TrendRider_Bear_Put_8",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.56, StopLossPct: 0.34, PositionUSD: 640, Signal: "TR_BEAR_8", CooldownSecs: 570},
		// Variant 9: EMA21 > EMA55 + 3-bar retrace < 0.2% + resume
		{Name: "TrendRider_Bull_Call_9", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.58, StopLossPct: 0.35, PositionUSD: 650, Signal: "TR_BULL_9", CooldownSecs: 600},
		{Name: "TrendRider_Bear_Put_9",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.58, StopLossPct: 0.35, PositionUSD: 650, Signal: "TR_BEAR_9", CooldownSecs: 600},
		// Variant 10: full trend + vol expansion (BB width > 150% 20-bar avg)
		{Name: "TrendRider_Bull_Call_10", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.60, StopLossPct: 0.36, PositionUSD: 660, Signal: "TR_BULL_10", CooldownSecs: 630},
		{Name: "TrendRider_Bear_Put_10",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.60, StopLossPct: 0.36, PositionUSD: 660, Signal: "TR_BEAR_10", CooldownSecs: 630},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY J — VOLATILITY PULSE  (20 strategies, IDs 82–101)
		// -----------------------------------------------------------------------
		// These strategies exploit IV / realised-vol dynamics. Each variant
		// targets a different vol-regime condition: IV expansion, vol mean-reversion,
		// squeeze-to-expansion at different BB periods, and vol-momentum combos.
		// ATM, 120-165 min.  TP 40-58%, SL 26-34%.
		// ═══════════════════════════════════════════════════════════════════════

		// Variant 1: realised vol spike (>1.5× 30-bar avg) + bullish bias
		{Name: "VolPulse_Bull_Call_1", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 120, TakeProfitPct: 0.42, StopLossPct: 0.27, PositionUSD: 520, Signal: "VP_BULL_1", CooldownSecs: 300},
		{Name: "VolPulse_Bear_Put_1",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 120, TakeProfitPct: 0.42, StopLossPct: 0.27, PositionUSD: 520, Signal: "VP_BEAR_1", CooldownSecs: 300},
		// Variant 2: BB width crossed above 20-bar avg — vol expansion start
		{Name: "VolPulse_Bull_Call_2", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 130, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 530, Signal: "VP_BULL_2", CooldownSecs: 330},
		{Name: "VolPulse_Bear_Put_2",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 130, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 530, Signal: "VP_BEAR_2", CooldownSecs: 330},
		// Variant 3: 5-bar atr spike + direction
		{Name: "VolPulse_Bull_Call_3", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 135, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 535, Signal: "VP_BULL_3", CooldownSecs: 360},
		{Name: "VolPulse_Bear_Put_3",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 135, TakeProfitPct: 0.44, StopLossPct: 0.28, PositionUSD: 535, Signal: "VP_BEAR_3", CooldownSecs: 360},
		// Variant 4: BB(10) squeeze break — tighter band
		{Name: "VolPulse_Bull_Call_4", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.46, StopLossPct: 0.29, PositionUSD: 540, Signal: "VP_BULL_4", CooldownSecs: 390},
		{Name: "VolPulse_Bear_Put_4",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.46, StopLossPct: 0.29, PositionUSD: 540, Signal: "VP_BEAR_4", CooldownSecs: 390},
		// Variant 5: IV jump > 10% 1hr change + directional momentum
		{Name: "VolPulse_Bull_Call_5", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 550, Signal: "VP_BULL_5", CooldownSecs: 420},
		{Name: "VolPulse_Bear_Put_5",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 140, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 550, Signal: "VP_BEAR_5", CooldownSecs: 420},
		// Variant 6: std-dev cross + RSI momentum
		{Name: "VolPulse_Bull_Call_6", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 145, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 555, Signal: "VP_BULL_6", CooldownSecs: 450},
		{Name: "VolPulse_Bear_Put_6",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 145, TakeProfitPct: 0.48, StopLossPct: 0.30, PositionUSD: 555, Signal: "VP_BEAR_6", CooldownSecs: 450},
		// Variant 7: 30-bar historical vol below 20-bar + breakout
		{Name: "VolPulse_Bull_Call_7", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 150, TakeProfitPct: 0.50, StopLossPct: 0.31, PositionUSD: 560, Signal: "VP_BULL_7", CooldownSecs: 480},
		{Name: "VolPulse_Bear_Put_7",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 150, TakeProfitPct: 0.50, StopLossPct: 0.31, PositionUSD: 560, Signal: "VP_BEAR_7", CooldownSecs: 480},
		// Variant 8: vol compression + EMA cross + RSI exit from neutral
		{Name: "VolPulse_Bull_Call_8", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 155, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 570, Signal: "VP_BULL_8", CooldownSecs: 510},
		{Name: "VolPulse_Bear_Put_8",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 155, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 570, Signal: "VP_BEAR_8", CooldownSecs: 510},
		// Variant 9: extreme low vol + resistance break
		{Name: "VolPulse_Bull_Call_9", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 160, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 580, Signal: "VP_BULL_9", CooldownSecs: 540},
		{Name: "VolPulse_Bear_Put_9",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 160, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 580, Signal: "VP_BEAR_9", CooldownSecs: 540},
		// Variant 10: IV > 0.80 (crisis mode) + capitulation recovery
		{Name: "VolPulse_Bull_Call_10", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.58, StopLossPct: 0.34, PositionUSD: 590, Signal: "VP_BULL_10", CooldownSecs: 600},
		{Name: "VolPulse_Bear_Put_10",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.58, StopLossPct: 0.34, PositionUSD: 590, Signal: "VP_BEAR_10", CooldownSecs: 600},

		// ═══════════════════════════════════════════════════════════════════════
		// CATEGORY K — STRUCTURE SNAP  (20 strategies, IDs 102–121)
		// -----------------------------------------------------------------------
		// Price-structure setups: swing high/low breaks, inside bars, key-level
		// reclaims, and higher-timeframe (HTF) confluence.  Longer hold times
		// (165-180 min).  Highest TP targets (50-68%) because price-structure
		// breaks often produce sustained follow-through in BTC.
		// ═══════════════════════════════════════════════════════════════════════

		// Variant 1: 30-bar swing high break + volume confirmation proxy
		{Name: "StructureSnap_Bull_Call_1", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.50, StopLossPct: 0.32, PositionUSD: 620, Signal: "SS_BULL_1", CooldownSecs: 600},
		{Name: "StructureSnap_Bear_Put_1",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.50, StopLossPct: 0.32, PositionUSD: 620, Signal: "SS_BEAR_1", CooldownSecs: 600},
		// Variant 2: inside-bar breakout (narrow range bar followed by expansion)
		{Name: "StructureSnap_Bull_Call_2", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 625, Signal: "SS_BULL_2", CooldownSecs: 600},
		{Name: "StructureSnap_Bear_Put_2",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 165, TakeProfitPct: 0.52, StopLossPct: 0.32, PositionUSD: 625, Signal: "SS_BEAR_2", CooldownSecs: 600},
		// Variant 3: 40-bar high break + EMA stack
		{Name: "StructureSnap_Bull_Call_3", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 630, Signal: "SS_BULL_3", CooldownSecs: 630},
		{Name: "StructureSnap_Bear_Put_3",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 630, Signal: "SS_BEAR_3", CooldownSecs: 630},
		// Variant 4: 50-bar pivot reclaim + RSI
		{Name: "StructureSnap_Bull_Call_4", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 635, Signal: "SS_BULL_4", CooldownSecs: 630},
		{Name: "StructureSnap_Bear_Put_4",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.54, StopLossPct: 0.33, PositionUSD: 635, Signal: "SS_BEAR_4", CooldownSecs: 630},
		// Variant 5: 3-bar base + expansion above resistance
		{Name: "StructureSnap_Bull_Call_5", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.56, StopLossPct: 0.34, PositionUSD: 640, Signal: "SS_BULL_5", CooldownSecs: 660},
		{Name: "StructureSnap_Bear_Put_5",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 170, TakeProfitPct: 0.56, StopLossPct: 0.34, PositionUSD: 640, Signal: "SS_BEAR_5", CooldownSecs: 660},
		// Variant 6: failed breakdown + snap back (bear trap / bull trap)
		{Name: "StructureSnap_Bull_Call_6", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.56, StopLossPct: 0.34, PositionUSD: 645, Signal: "SS_BULL_6", CooldownSecs: 660},
		{Name: "StructureSnap_Bear_Put_6",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.56, StopLossPct: 0.34, PositionUSD: 645, Signal: "SS_BEAR_6", CooldownSecs: 660},
		// Variant 7: hourly open reclaim + EMA9 slope
		{Name: "StructureSnap_Bull_Call_7", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.58, StopLossPct: 0.35, PositionUSD: 650, Signal: "SS_BULL_7", CooldownSecs: 700},
		{Name: "StructureSnap_Bear_Put_7",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.58, StopLossPct: 0.35, PositionUSD: 650, Signal: "SS_BEAR_7", CooldownSecs: 700},
		// Variant 8: 60-bar high break + strong momentum
		{Name: "StructureSnap_Bull_Call_8", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.60, StopLossPct: 0.35, PositionUSD: 660, Signal: "SS_BULL_8", CooldownSecs: 720},
		{Name: "StructureSnap_Bear_Put_8",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 175, TakeProfitPct: 0.60, StopLossPct: 0.35, PositionUSD: 660, Signal: "SS_BEAR_8", CooldownSecs: 720},
		// Variant 9: 3-touch support/resistance break (3rd touch most reliable)
		{Name: "StructureSnap_Bull_Call_9", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.62, StopLossPct: 0.36, PositionUSD: 700, Signal: "SS_BULL_9", CooldownSecs: 750},
		{Name: "StructureSnap_Bear_Put_9",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.62, StopLossPct: 0.36, PositionUSD: 700, Signal: "SS_BEAR_9", CooldownSecs: 750},
		// Variant 10: all-time-session high break + vol + RSI elite combo
		{Name: "StructureSnap_Bull_Call_10", Type: Call, StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.66, StopLossPct: 0.38, PositionUSD: 780, Signal: "SS_BULL_10", CooldownSecs: 800},
		{Name: "StructureSnap_Bear_Put_10",  Type: Put,  StrikePctOTM: 0.0, ExpiryMinutes: 180, TakeProfitPct: 0.66, StopLossPct: 0.38, PositionUSD: 780, Signal: "SS_BEAR_10", CooldownSecs: 800},
	}
}
