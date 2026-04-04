package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.005
)

// Strategy library for the BTC option scalper.
// Each strategy is completely independent — it manages its own position,
// uses its own signal, and does NOT share state with the main futures engine.
//
// R:R rules enforced: TakeProfitPct >= 2x StopLossPct on all strategies.
// ATM options preferred for scalps (faster delta response).
// OTM only used with higher TP targets and longer expiry.
// BuildStrategies returns only the live-approved option-buying strategies.
// Weakest long-premium setups are filtered out before runtime:
// ultra-short expiries (< 60 minutes) and deeper OTM strikes (> 0.5%).
func BuildStrategies() []StrategyDef {
	all := buildAllStrategies()
	filtered := make([]StrategyDef, 0, len(all))
	for _, def := range all {
		if isLiveApprovedStrategy(def) {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func isLiveApprovedStrategy(def StrategyDef) bool {
	return def.ExpiryMinutes >= minLiveExpiryMinutes && def.StrikePctOTM <= maxLiveStrikePctOTM
}

func buildAllStrategies() []StrategyDef {
	return []StrategyDef{
		// ── MOMENTUM CALLS (1-10) ─────────────────────────────────────────────
		{
			Name: "MomentumBull_ATM_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 60,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "BULL_MOMENTUM", CooldownSecs: 300,
		},
		{
			Name: "MomentumBull_OTM1_Call", Type: Call,
			StrikePctOTM: 0.008, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.35,
			PositionUSD: 400, Signal: "BULL_MOMENTUM", CooldownSecs: 360,
		},
		{
			Name: "StrongMomentum_ATM_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.70, StopLossPct: 0.30,
			PositionUSD: 600, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 240,
		},
		{
			Name: "EMABull_ATM_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "EMA_BULL_CROSS", CooldownSecs: 600,
		},
		{
			Name: "EMABull_OTM_Call", Type: Call,
			StrikePctOTM: 0.01, ExpiryMinutes: 120,
			TakeProfitPct: 1.20, StopLossPct: 0.40,
			PositionUSD: 300, Signal: "EMA_BULL_CROSS", CooldownSecs: 600,
		},
		{
			Name: "TripleBull_ATM_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.70, StopLossPct: 0.28,
			PositionUSD: 600, Signal: "TRIPLE_BULL", CooldownSecs: 480,
		},
		{
			Name: "ResistBreak_Call", Type: Call,
			StrikePctOTM: 0.003, ExpiryMinutes: 60,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 500, Signal: "RESISTANCE_BREAK", CooldownSecs: 600,
		},
		{
			Name: "VWAP_Bull_ATM_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 450, Signal: "VWAP_ABOVE", CooldownSecs: 420,
		},
		{
			Name: "HighIV_Bull_Call", Type: Call,
			StrikePctOTM: 0.005, ExpiryMinutes: 60,
			TakeProfitPct: 0.90, StopLossPct: 0.35,
			PositionUSD: 400, Signal: "HIGH_IV_BULL", CooldownSecs: 300,
		},
		{
			Name: "Reversal_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.70, StopLossPct: 0.28,
			PositionUSD: 500, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 360,
		},

		// ── MOMENTUM PUTS (11-20) ─────────────────────────────────────────────
		{
			Name: "MomentumBear_ATM_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 60,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "BEAR_MOMENTUM", CooldownSecs: 300,
		},
		{
			Name: "MomentumBear_OTM1_Put", Type: Put,
			StrikePctOTM: 0.008, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.35,
			PositionUSD: 400, Signal: "BEAR_MOMENTUM", CooldownSecs: 360,
		},
		{
			Name: "StrongMomentum_ATM_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.70, StopLossPct: 0.30,
			PositionUSD: 600, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 240,
		},
		{
			Name: "EMABear_ATM_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "EMA_BEAR_CROSS", CooldownSecs: 600,
		},
		{
			Name: "EMABear_OTM_Put", Type: Put,
			StrikePctOTM: 0.01, ExpiryMinutes: 120,
			TakeProfitPct: 1.20, StopLossPct: 0.40,
			PositionUSD: 300, Signal: "EMA_BEAR_CROSS", CooldownSecs: 600,
		},
		{
			Name: "TripleBear_ATM_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.70, StopLossPct: 0.28,
			PositionUSD: 600, Signal: "TRIPLE_BEAR", CooldownSecs: 480,
		},
		{
			Name: "SupportBreak_Put", Type: Put,
			StrikePctOTM: 0.003, ExpiryMinutes: 60,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 500, Signal: "SUPPORT_BREAK", CooldownSecs: 600,
		},
		{
			Name: "VWAP_Bear_ATM_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 450, Signal: "VWAP_BELOW", CooldownSecs: 420,
		},
		{
			Name: "HighIV_Bear_Put", Type: Put,
			StrikePctOTM: 0.005, ExpiryMinutes: 60,
			TakeProfitPct: 0.90, StopLossPct: 0.35,
			PositionUSD: 400, Signal: "HIGH_IV_BEAR", CooldownSecs: 300,
		},
		{
			Name: "Reversal_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.70, StopLossPct: 0.28,
			PositionUSD: 500, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 360,
		},

		// ── RSI MEAN REVERSION (21-30) ────────────────────────────────────────
		{
			Name: "RSI_Oversold_ATM_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "RSI_OVERSOLD", CooldownSecs: 600,
		},
		{
			Name: "RSI_Oversold_OTM_Call", Type: Call,
			StrikePctOTM: 0.008, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.35,
			PositionUSD: 350, Signal: "RSI_OVERSOLD", CooldownSecs: 600,
		},
		{
			Name: "RSI_Extreme_Oversold_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 60,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 600, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 480,
		},
		{
			Name: "RSI_Overbought_ATM_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "RSI_OVERBOUGHT", CooldownSecs: 600,
		},
		{
			Name: "RSI_Overbought_OTM_Put", Type: Put,
			StrikePctOTM: 0.008, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.35,
			PositionUSD: 350, Signal: "RSI_OVERBOUGHT", CooldownSecs: 600,
		},
		{
			Name: "RSI_Extreme_Overbought_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 60,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 600, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 480,
		},
		{
			Name: "Stoch_Oversold_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 450, Signal: "STOCH_OVERSOLD", CooldownSecs: 540,
		},
		{
			Name: "Stoch_Overbought_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 450, Signal: "STOCH_OVERBOUGHT", CooldownSecs: 540,
		},
		{
			Name: "BB_Lower_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.70, StopLossPct: 0.28,
			PositionUSD: 500, Signal: "BB_LOWER_TOUCH", CooldownSecs: 600,
		},
		{
			Name: "BB_Upper_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.70, StopLossPct: 0.28,
			PositionUSD: 500, Signal: "BB_UPPER_TOUCH", CooldownSecs: 600,
		},

		// ── SQUEEZE / VOLATILITY (31-38) ──────────────────────────────────────
		{
			Name: "BB_Squeeze_Bull_Call", Type: Call,
			StrikePctOTM: 0.003, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.35,
			PositionUSD: 500, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 720,
		},
		{
			Name: "BB_Squeeze_Bear_Put", Type: Put,
			StrikePctOTM: 0.003, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.35,
			PositionUSD: 500, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 720,
		},
		{
			Name: "BB_Squeeze_Bull_OTM_Call", Type: Call,
			StrikePctOTM: 0.012, ExpiryMinutes: 120,
			TakeProfitPct: 1.50, StopLossPct: 0.40,
			PositionUSD: 300, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 900,
		},
		{
			Name: "BB_Squeeze_Bear_OTM_Put", Type: Put,
			StrikePctOTM: 0.012, ExpiryMinutes: 120,
			TakeProfitPct: 1.50, StopLossPct: 0.40,
			PositionUSD: 300, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 900,
		},
		{
			Name: "EMA_Above_Both_Call", Type: Call,
			StrikePctOTM: 0.005, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 400, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 600,
		},
		{
			Name: "EMA_Below_Both_Put", Type: Put,
			StrikePctOTM: 0.005, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 400, Signal: "EMA_BELOW_BOTH", CooldownSecs: 600,
		},
		{
			Name: "HighIV_Squeeze_Call", Type: Call,
			StrikePctOTM: 0.008, ExpiryMinutes: 75,
			TakeProfitPct: 1.10, StopLossPct: 0.38,
			PositionUSD: 350, Signal: "HIGH_IV_BULL", CooldownSecs: 480,
		},
		{
			Name: "HighIV_Squeeze_Put", Type: Put,
			StrikePctOTM: 0.008, ExpiryMinutes: 75,
			TakeProfitPct: 1.10, StopLossPct: 0.38,
			PositionUSD: 350, Signal: "HIGH_IV_BEAR", CooldownSecs: 480,
		},

		// ── AGGRESSIVE SCALP (39-50) ──────────────────────────────────────────
		{
			Name: "Scalp_QuickBull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 40,
			TakeProfitPct: 0.55, StopLossPct: 0.22,
			PositionUSD: 600, Signal: "BULL_MOMENTUM", CooldownSecs: 180,
		},
		{
			Name: "Scalp_QuickBear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 40,
			TakeProfitPct: 0.55, StopLossPct: 0.22,
			PositionUSD: 600, Signal: "BEAR_MOMENTUM", CooldownSecs: 180,
		},
		{
			Name: "Scalp_RSI_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.60, StopLossPct: 0.22,
			PositionUSD: 550, Signal: "RSI_OVERSOLD", CooldownSecs: 300,
		},
		{
			Name: "Scalp_RSI_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.60, StopLossPct: 0.22,
			PositionUSD: 550, Signal: "RSI_OVERBOUGHT", CooldownSecs: 300,
		},
		{
			Name: "Scalp_VWAP_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 40,
			TakeProfitPct: 0.55, StopLossPct: 0.20,
			PositionUSD: 500, Signal: "VWAP_ABOVE", CooldownSecs: 240,
		},
		{
			Name: "Scalp_VWAP_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 40,
			TakeProfitPct: 0.55, StopLossPct: 0.20,
			PositionUSD: 500, Signal: "VWAP_BELOW", CooldownSecs: 240,
		},
		{
			Name: "Scalp_Reversal_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 40,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "SHARP_REVERSAL_UP", CooldownSecs: 240,
		},
		{
			Name: "Scalp_Reversal_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 40,
			TakeProfitPct: 0.60, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "SHARP_REVERSAL_DOWN", CooldownSecs: 240,
		},
		{
			Name: "Scalp_Break_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "RESISTANCE_BREAK", CooldownSecs: 360,
		},
		{
			Name: "Scalp_Break_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 45,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "SUPPORT_BREAK", CooldownSecs: 360,
		},
		{
			Name: "Scalp_Triple_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 50,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 600, Signal: "TRIPLE_BULL", CooldownSecs: 360,
		},
		{
			Name: "Scalp_Triple_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 50,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 600, Signal: "TRIPLE_BEAR", CooldownSecs: 360,
		},

		// ── STRATEGY 1: Consecutive Candle Momentum ───────────────────────────
		// BTC momentum is autocorrelated — 4 consecutive bullish 1-min bars
		// signal continuation. Captures the "momentum burst" pattern unique
		// to high-liquidity crypto markets where algos chase moves.
		{
			Name: "ConsecBull_Momentum_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "CONSEC_BULL_BARS", CooldownSecs: 300,
		},
		{
			Name: "ConsecBear_Momentum_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "CONSEC_BEAR_BARS", CooldownSecs: 300,
		},

		// ── STRATEGY 2: Volatility Compression Breakout ───────────────────────
		// Options are cheap when vol is compressed. When price finally breaks out,
		// you earn delta gains AND vega gains (IV expands with the move).
		// Best risk-adjusted entry in options: buy cheap, ride the expansion.
		{
			Name: "VolCompress_Breakout_Call", Type: Call,
			StrikePctOTM: 0.003, ExpiryMinutes: 90,
			TakeProfitPct: 0.90, StopLossPct: 0.30,
			PositionUSD: 500, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 600,
		},
		{
			Name: "VolCompress_Breakout_Put", Type: Put,
			StrikePctOTM: 0.003, ExpiryMinutes: 90,
			TakeProfitPct: 0.90, StopLossPct: 0.30,
			PositionUSD: 500, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 600,
		},

		// ── STRATEGY 3: Session Open Momentum ────────────────────────────────
		// BTC receives fresh institutional order flow at UTC 00:00, 08:00, 13:30, 20:00.
		// The first directional move in the opening 3-18 minutes tends to persist
		// for 60-90 minutes as resting orders are filled and momentum builds.
		{
			Name: "SessionOpen_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 600, Signal: "SESSION_OPEN_BULL", CooldownSecs: 720,
		},
		{
			Name: "SessionOpen_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 600, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 720,
		},

		// ── STRATEGY 4: Capitulation V-Reversal ──────────────────────────────
		// Panic drops (>0.7% in 5 bars) trigger cascade stop-losses, clearing
		// weak hands. When price snaps back firmly, the selling is exhausted.
		// This targets the exact bottom of the V — high probability, high R:R.
		// Only fires as a CALL (buy the dip recovery).
		{
			Name: "Capitulation_VReversal_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.75, StopLossPct: 0.28,
			PositionUSD: 600, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 600,
		},

		// ── STRATEGY 5: Overextension Fade ───────────────────────────────────
		// After a 2%+ rapid move with RSI at extremes AND price at the Bollinger
		// Band, the rubber band snaps back. Pure contrarian play — requires ALL
		// three confirmations to prevent fighting strong trends.
		// Buy puts when over-extended up, buy calls when over-extended down.
		{
			Name: "Overextension_Fade_Put", Type: Put,
			StrikePctOTM: 0.003, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 450, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 900,
		},
		{
			Name: "Overextension_Fade_Call", Type: Call,
			StrikePctOTM: 0.003, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.30,
			PositionUSD: 450, Signal: "OVEREXTENSION_FADE_DOWN", CooldownSecs: 900,
		},
	}
}
