package options

const (
	minLiveExpiryMinutes = 60
	maxLiveStrikePctOTM  = 0.005
)

var strategyIDs = map[string]int{
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

// BuildStrategies returns the live-approved strategy set.
// Filters out ultra-short expiries and deep OTM strikes.
func BuildStrategies() []StrategyDef {
	all := assignStrategyIDs(buildAllStrategies())
	filtered := make([]StrategyDef, 0, len(all))
	for _, def := range all {
		if def.ExpiryMinutes >= minLiveExpiryMinutes && def.StrikePctOTM <= maxLiveStrikePctOTM {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

// buildAllStrategies defines 41 live-approved BTC option buying strategies.
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
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.68, StopLossPct: 0.24,
			PositionUSD: 575, Signal: "VWAP_ABOVE", CooldownSecs: 540,
		},
		{
			Name: "VWAP_Continuation_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.68, StopLossPct: 0.24,
			PositionUSD: 575, Signal: "VWAP_BELOW", CooldownSecs: 540,
		},

		// Signal: oversold/overbought reversal, EMA cross, and momentum all agree.
		// This is one of the cleanest "everything aligned" entries in the book.
		{
			Name: "TripleConfluence_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.26,
			PositionUSD: 625, Signal: "TRIPLE_BULL", CooldownSecs: 720,
		},
		{
			Name: "TripleConfluence_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.80, StopLossPct: 0.26,
			PositionUSD: 625, Signal: "TRIPLE_BEAR", CooldownSecs: 720,
		},

		// Signal: fast upside burst rejects immediately and price rolls over.
		// This catches intraday blow-off tops earlier than the slower
		// overextension fade, with tighter risk and higher trade frequency.
		{
			Name: "SharpReversal_TopFade_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.72, StopLossPct: 0.24,
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
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.72, StopLossPct: 0.24,
			PositionUSD: 600, Signal: "EMA_ABOVE_BOTH", CooldownSecs: 600,
		},
		{
			Name: "TrendAlignment_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.72, StopLossPct: 0.24,
			PositionUSD: 600, Signal: "EMA_BELOW_BOTH", CooldownSecs: 600,
		},

		// Signal: band-touch rejection with immediate recovery back inside the
		// Bollinger envelope. This is a tighter, faster mean-reversion setup than
		// the extreme RSI variants and gives the book more range-trading balance.
		{
			Name: "BandBounce_Reclaim_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.62, StopLossPct: 0.22,
			PositionUSD: 525, Signal: "BB_LOWER_TOUCH", CooldownSecs: 480,
		},
		{
			Name: "BandFade_Rejection_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.62, StopLossPct: 0.22,
			PositionUSD: 525, Signal: "BB_UPPER_TOUCH", CooldownSecs: 480,
		},

		// Signal: sharp downside washout followed by immediate snapback. This is
		// the upside counterpart to the existing sharp bearish rejection strategy.
		{
			Name: "SharpReversal_BottomSnap_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.72, StopLossPct: 0.24,
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
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.60, StopLossPct: 0.22,
			PositionUSD: 540, Signal: "BULL_MOMENTUM", CooldownSecs: 420,
		},

		// Signal: recent compression in Bollinger width followed by bullish
		// breakout. This captures the first sustained expansion leg out of a
		// coiled range before the slower vol-compress setups fully mature.
		{
			Name: "BBSqueeze_Release_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.66, StopLossPct: 0.23,
			PositionUSD: 560, Signal: "BB_SQUEEZE_BULL", CooldownSecs: 540,
		},
		{
			Name: "BBSqueeze_Release_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 75,
			TakeProfitPct: 0.66, StopLossPct: 0.23,
			PositionUSD: 560, Signal: "BB_SQUEEZE_BEAR", CooldownSecs: 540,
		},

		// Signal: already-elevated IV plus strong directional push. These are
		// selective follow-through trades for environments where realized vol is
		// expanding fast enough that long premium can still outperform.
		{
			Name: "HighIV_Expansion_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 60,
			TakeProfitPct: 0.58, StopLossPct: 0.20,
			PositionUSD: 500, Signal: "HIGH_IV_BULL", CooldownSecs: 480,
		},
		{
			Name: "HighIV_Expansion_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 60,
			TakeProfitPct: 0.58, StopLossPct: 0.20,
			PositionUSD: 500, Signal: "HIGH_IV_BEAR", CooldownSecs: 480,
		},

		// CATEGORY H - HYBRID ALPHA FACTORY
		// -----------------------------------------------------------------------
		// Five higher-conviction hybrids built by combining the strongest
		// existing edges into more selective entries. These are intended to be
		// better than the prior add-ons, not just more numerous.

		// Signal: strong directional momentum already established above/below
		// VWAP. These are cleaner second-leg continuation entries than raw
		// momentum alone because fair-value alignment confirms institutional flow.
		{
			Name: "MomentumVWAP_Pro_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.78, StopLossPct: 0.24,
			PositionUSD: 650, Signal: "MOMENTUM_VWAP_BULL", CooldownSecs: 600,
		},
		{
			Name: "MomentumVWAP_Pro_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.78, StopLossPct: 0.24,
			PositionUSD: 650, Signal: "MOMENTUM_VWAP_BEAR", CooldownSecs: 600,
		},

		// Signal: structural breakout plus trend confirmation from medium EMAs.
		// These should avoid many of the fake breakouts that hurt naked breakout
		// buying.
		{
			Name: "BreakoutTrend_Pro_Bull_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.84, StopLossPct: 0.26,
			PositionUSD: 675, Signal: "BREAKOUT_TREND_BULL", CooldownSecs: 720,
		},
		{
			Name: "BreakdownTrend_Pro_Bear_Put", Type: Put,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.84, StopLossPct: 0.26,
			PositionUSD: 675, Signal: "BREAKDOWN_TREND_BEAR", CooldownSecs: 720,
		},

		// Signal: panic washout followed by immediate VWAP and EMA reclaim.
		// This is the highest-conviction long-premium reversal setup in the book.
		{
			Name: "Capitulation_Reclaim_Elite_Call", Type: Call,
			StrikePctOTM: 0.0, ExpiryMinutes: 90,
			TakeProfitPct: 0.95, StopLossPct: 0.28,
			PositionUSD: 750, Signal: "CAPITULATION_RECLAIM", CooldownSecs: 900,
		},
	}
}
