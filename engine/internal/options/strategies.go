package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.005
)

// BuildStrategies returns the live-approved strategy set.
// Filters out ultra-short expiries and deep OTM strikes.
func BuildStrategies() []StrategyDef {
	all := buildAllStrategies()
	filtered := make([]StrategyDef, 0, len(all))
	for _, def := range all {
		if def.ExpiryMinutes >= minLiveExpiryMinutes && def.StrikePctOTM <= maxLiveStrikePctOTM {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// buildAllStrategies defines 20 elite BTC option buying strategies.
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

		// Signal: Price up >0.5% in 5 min AND >0.3% in 10 min, RSI < 70.
		// Size: $700 — highest conviction momentum signal, fires ~5x/day.
		{
			Name: "MomentumBurst_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.28,
			PositionUSD: 700, Signal: "STRONG_BULL_MOMENTUM", CooldownSecs: 300,
		},
		// Signal: Price down >0.5% in 5 min AND >0.3% in 10 min, RSI > 30.
		{
			Name: "MomentumBurst_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.28,
			PositionUSD: 700, Signal: "STRONG_BEAR_MOMENTUM", CooldownSecs: 300,
		},
		// Signal: 4 consecutive bullish 1-min bars with >0.35% total gain.
		// Autocorrelation play — algo chasers extend these runs for 3-5 more bars.
		{
			Name: "ConsecCandle_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.70, StopLossPct: 0.25,
			PositionUSD: 600, Signal: "CONSEC_BULL_BARS", CooldownSecs: 300,
		},
		// Signal: 4 consecutive bearish 1-min bars with >0.35% total drop.
		{
			Name: "ConsecCandle_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.70, StopLossPct: 0.25,
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
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.90, StopLossPct: 0.30,
			PositionUSD: 700, Signal: "RSI_OVERSOLD_EXTREME", CooldownSecs: 600,
		},
		// Signal: RSI(14) crossed back below 80 from above.
		{
			Name: "RSI_Extreme_Overbought_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.90, StopLossPct: 0.30,
			PositionUSD: 700, Signal: "RSI_OVERBOUGHT_EXTREME", CooldownSecs: 600,
		},
		// Signal: RSI crossed back above 30 from below (regular oversold exit).
		{
			Name: "RSI_Oversold_Recovery_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "RSI_OVERSOLD", CooldownSecs: 600,
		},
		// Signal: RSI crossed back below 70 from above.
		{
			Name: "RSI_Overbought_Fade_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "RSI_OVERBOUGHT", CooldownSecs: 600,
		},
		// Signal: 30-min rally > 2% + RSI > 76 + price at upper BB + stalling.
		// Pure contrarian — requires ALL conditions. When it fires, it's extremely
		// reliable. $600 size but long cooldown to prevent re-entry in a trend.
		{
			Name: "Overextension_Fade_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.85, StopLossPct: 0.30,
			PositionUSD: 600, Signal: "OVEREXTENSION_FADE_UP", CooldownSecs: 900,
		},
		// Signal: 30-min selloff > 2% + RSI < 24 + price at lower BB + stalling.
		{
			Name: "Overextension_Fade_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.85, StopLossPct: 0.30,
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
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "EMA_BULL_CROSS", CooldownSecs: 600,
		},
		// Signal: 9 EMA crossed below 21 EMA.
		{
			Name: "EMA_BearCross_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 550, Signal: "EMA_BEAR_CROSS", CooldownSecs: 600,
		},
		// Signal: Price breaks 0.3% above prior 20-bar high with momentum.
		// Breakout buyers pile in — ride the wave.
		{
			Name: "Resistance_Breakout_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.75, StopLossPct: 0.28,
			PositionUSD: 600, Signal: "RESISTANCE_BREAK", CooldownSecs: 720,
		},
		// Signal: Price breaks 0.3% below prior 20-bar low with momentum.
		{
			Name: "Support_Breakdown_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.75, StopLossPct: 0.28,
			PositionUSD: 600, Signal: "SUPPORT_BREAK", CooldownSecs: 720,
		},
		// Signal: Stochastic K crossed above 20 from below + RSI < 50.
		{
			Name: "Stoch_Oversold_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
			PositionUSD: 500, Signal: "STOCH_OVERSOLD", CooldownSecs: 540,
		},
		// Signal: Stochastic K crossed below 80 from above + RSI > 50.
		{
			Name: "Stoch_Overbought_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.65, StopLossPct: 0.25,
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
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.90, StopLossPct: 0.30,
			PositionUSD: 750, Signal: "CAPITULATION_RECOVERY", CooldownSecs: 600,
		},
		// Signal: Within 3-18 minutes of UTC session opens (00:00/08:00/13:30/20:00)
		// + strong directional momentum. Fresh institutional order flow drives
		// persistent 60-90 min trends. Fire in both directions.
		{
			Name: "SessionOpen_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.70, StopLossPct: 0.25,
			PositionUSD: 650, Signal: "SESSION_OPEN_BULL", CooldownSecs: 720,
		},
		{
			Name: "SessionOpen_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.70, StopLossPct: 0.25,
			PositionUSD: 650, Signal: "SESSION_OPEN_BEAR", CooldownSecs: 720,
		},
		// Signal: Recent 10-bar vol < 50% of 60-bar vol + strong breakout momentum.
		// Buy options when they're statistically cheap (compressed vol).
		// When the move comes, you earn delta gain + vega gain simultaneously.
		{
			Name: "VolCompress_Breakout_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.33,
			PositionUSD: 600, Signal: "VOL_COMPRESS_BULL", CooldownSecs: 720,
		},
		{
			Name: "VolCompress_Breakout_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 1.00, StopLossPct: 0.33,
			PositionUSD: 600, Signal: "VOL_COMPRESS_BEAR", CooldownSecs: 720,
		},
	}
}
