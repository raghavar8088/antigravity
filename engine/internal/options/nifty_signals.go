package options

import "math"

// NiftySignals overrides BTC-calibrated thresholds with NIFTY 50–appropriate
// values.  NIFTY daily range ≈ 0.5-1.2 % vs BTC ≈ 2-5 %, so momentum /
// deviation gates are scaled to ~25 % of the BTC values.
//
// Signals not present here are served from the base Signals map via
// Engine.signalFuncFor().
var NiftySignals = map[string]SignalFunc{

	// ── Momentum ──────────────────────────────────────────────────────────
	"BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 > 0.0006 && mom10 > 0.0003 && rsiVal < 64
	},
	"BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0006 && mom10 < -0.0003 && rsiVal > 36
	},
	"STRONG_BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 > 0.0010 && mom10 > 0.0005 && rsiVal < 68
	},
	"STRONG_BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0010 && mom10 < -0.0005 && rsiVal > 32
	},

	// ── VWAP ──────────────────────────────────────────────────────────────
	"VWAP_ABOVE": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (ctx.BTCPrice - vw) / vw
		return deviation > 0.0008 && momentum(ctx.Prices, 5) > 0.0005
	},
	"VWAP_BELOW": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (vw - ctx.BTCPrice) / vw
		return deviation > 0.0008 && momentum(ctx.Prices, 5) < -0.0005
	},

	// ── Momentum + VWAP confluence ────────────────────────────────────────
	"MOMENTUM_VWAP_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > vw*1.0006 && mom5 > 0.0008 && mom10 > 0.0004 && rsiVal > 50 && rsiVal < 66
	},
	"MOMENTUM_VWAP_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < vw*0.9994 && mom5 < -0.0008 && mom10 < -0.0004 && rsiVal > 34 && rsiVal < 50
	},

	// ── Breakout ──────────────────────────────────────────────────────────
	"RESISTANCE_BREAK": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 {
			return false
		}
		prev := ctx.Prices[len(ctx.Prices)-21 : len(ctx.Prices)-1]
		hi := 0.0
		for _, p := range prev {
			if p > hi {
				hi = p
			}
		}
		return ctx.BTCPrice > hi*1.0006 && momentum(ctx.Prices, 3) > 0.0005
	},
	"SUPPORT_BREAK": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 {
			return false
		}
		prev := ctx.Prices[len(ctx.Prices)-21 : len(ctx.Prices)-1]
		lo := math.MaxFloat64
		for _, p := range prev {
			if p < lo {
				lo = p
			}
		}
		return ctx.BTCPrice < lo*0.9994 && momentum(ctx.Prices, 3) < -0.0005
	},

	// ── BB Squeeze ────────────────────────────────────────────────────────
	"BB_SQUEEZE_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 {
			return false
		}
		recentStd := stddev(ctx.Prices[len(ctx.Prices)-10:])
		priorStd := stddev(ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-10])
		squeezed := recentStd < priorStd*0.75
		breakout := momentum(ctx.Prices, 3) > 0.0004
		return squeezed && breakout
	},
	"BB_SQUEEZE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 {
			return false
		}
		recentStd := stddev(ctx.Prices[len(ctx.Prices)-10:])
		priorStd := stddev(ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-10])
		squeezed := recentStd < priorStd*0.75
		breakout := momentum(ctx.Prices, 3) < -0.0004
		return squeezed && breakout
	},

	// ── Vol Compression Breakout ──────────────────────────────────────────
	"VOL_COMPRESS_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 {
			return false
		}
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-10:])
		historicalStd := stddev(ctx.Prices[n-40:])
		if historicalStd == 0 {
			return false
		}
		compressed := recentStd < historicalStd*0.65
		breakout := momentum(ctx.Prices, 5) > 0.0007
		rsiVal := rsi(ctx.Prices, 14)
		return compressed && breakout && rsiVal < 65
	},
	"VOL_COMPRESS_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 {
			return false
		}
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-10:])
		historicalStd := stddev(ctx.Prices[n-40:])
		if historicalStd == 0 {
			return false
		}
		compressed := recentStd < historicalStd*0.65
		breakout := momentum(ctx.Prices, 5) < -0.0007
		rsiVal := rsi(ctx.Prices, 14)
		return compressed && breakout && rsiVal > 35
	},

	// ── Consecutive Candle Momentum ───────────────────────────────────────
	"CONSEC_BULL_BARS": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 6 {
			return false
		}
		n := len(ctx.Prices)
		for i := n - 4; i < n; i++ {
			if ctx.Prices[i] <= ctx.Prices[i-1] {
				return false
			}
		}
		totalGain := (ctx.Prices[n-1] - ctx.Prices[n-5]) / ctx.Prices[n-5]
		rsiVal := rsi(ctx.Prices, 14)
		return totalGain > 0.0007 && rsiVal < 72
	},
	"CONSEC_BEAR_BARS": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 6 {
			return false
		}
		n := len(ctx.Prices)
		for i := n - 4; i < n; i++ {
			if ctx.Prices[i] >= ctx.Prices[i-1] {
				return false
			}
		}
		totalLoss := (ctx.Prices[n-5] - ctx.Prices[n-1]) / ctx.Prices[n-5]
		rsiVal := rsi(ctx.Prices, 14)
		return totalLoss > 0.0007 && rsiVal > 28
	},

	// ── Session Open (BTC sessions → NSE session) ─────────────────────────
	// Re-target to NSE open (03:45 UTC = 225 min) instead of BTC sessions.
	"SESSION_OPEN_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		// NSE open: 03:45 UTC (225 min), +5 to +30 min after open
		if totalMin < 230 || totalMin > 255 {
			return false
		}
		mom := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom > 0.0008 && rsiVal < 65
	},
	"SESSION_OPEN_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		if totalMin < 230 || totalMin > 255 {
			return false
		}
		mom := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom < -0.0008 && rsiVal > 35
	},

	// ── Capitulation / V-Reversal ─────────────────────────────────────────
	"CAPITULATION_RECOVERY": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 {
			return false
		}
		n := len(ctx.Prices)
		window := ctx.Prices[n-7 : n-1]
		lo := window[0]
		for _, p := range window[1:] {
			if p < lo {
				lo = p
			}
		}
		startPrice := ctx.Prices[n-8]
		if startPrice == 0 || lo == 0 {
			return false
		}
		drop := (startPrice - lo) / startPrice
		recovery := (ctx.BTCPrice - lo) / lo
		rsiVal := rsi(ctx.Prices, 14)
		return drop > 0.0014 && recovery > 0.0006 && ctx.BTCPrice > lo && rsiVal < 56
	},

	// ── Capitulation Reclaim ──────────────────────────────────────────────
	"CAPITULATION_RECLAIM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		n := len(ctx.Prices)
		window := ctx.Prices[n-7 : n-1]
		lo := window[0]
		for _, p := range window[1:] {
			if p < lo {
				lo = p
			}
		}
		startPrice := ctx.Prices[n-8]
		if startPrice == 0 || lo == 0 {
			return false
		}
		drop := (startPrice - lo) / startPrice
		recovery := (ctx.BTCPrice - lo) / lo
		shortVWAP := avgPrice(ctx.Prices[n-15:])
		ema9 := ema(ctx.Prices, 9)
		rsiVal := rsi(ctx.Prices, 14)
		return drop > 0.0014 &&
			recovery > 0.0007 &&
			ctx.BTCPrice > shortVWAP &&
			ctx.BTCPrice > ema9 &&
			rsiVal > 38 && rsiVal < 56
	},

	// ── Sharp Reversal ────────────────────────────────────────────────────
	"SHARP_REVERSAL_UP": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		window := ctx.Prices[len(ctx.Prices)-6 : len(ctx.Prices)-1]
		lo := math.MaxFloat64
		for _, p := range window {
			if p < lo {
				lo = p
			}
		}
		dropFromHigh := (ctx.Prices[len(ctx.Prices)-6] - lo) / ctx.Prices[len(ctx.Prices)-6]
		recovery := (ctx.BTCPrice - lo) / lo
		return dropFromHigh > 0.0008 && recovery > 0.0004 && ctx.BTCPrice > lo
	},
	"SHARP_REVERSAL_DOWN": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		window := ctx.Prices[len(ctx.Prices)-6 : len(ctx.Prices)-1]
		hi := 0.0
		for _, p := range window {
			if p > hi {
				hi = p
			}
		}
		riseFromLow := (hi - ctx.Prices[len(ctx.Prices)-6]) / ctx.Prices[len(ctx.Prices)-6]
		rejection := (hi - ctx.BTCPrice) / hi
		return riseFromLow > 0.0008 && rejection > 0.0004 && ctx.BTCPrice < hi
	},

	// ── Overextension Fade ────────────────────────────────────────────────
	"OVEREXTENSION_FADE_UP": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		mom30 := momentum(ctx.Prices, 30)
		rsiVal := rsi(ctx.Prices, 14)
		atUpper := ctx.BTCPrice >= bbUpper(ctx.Prices, 20)*0.999
		mom3 := momentum(ctx.Prices, 3)
		return mom30 > 0.003 && rsiVal > 72 && atUpper && mom3 < mom30/8
	},
	"OVEREXTENSION_FADE_DOWN": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		mom30 := momentum(ctx.Prices, 30)
		rsiVal := rsi(ctx.Prices, 14)
		atLower := ctx.BTCPrice <= bbLower(ctx.Prices, 20)*1.001
		mom3 := momentum(ctx.Prices, 3)
		return mom30 < -0.003 && rsiVal < 28 && atLower && mom3 > mom30/8
	},

	// ── Breakout Trend ────────────────────────────────────────────────────
	"BREAKOUT_TREND_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 55 {
			return false
		}
		prev := ctx.Prices[len(ctx.Prices)-21 : len(ctx.Prices)-1]
		hi := 0.0
		for _, p := range prev {
			if p > hi {
				hi = p
			}
		}
		ema20 := ema(ctx.Prices, 20)
		ema50 := ema(ctx.Prices, 50)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > hi*1.0006 &&
			ctx.BTCPrice > ema20 &&
			ctx.BTCPrice > ema50 &&
			momentum(ctx.Prices, 3) > 0.0006 &&
			rsiVal > 54 && rsiVal < 68
	},
	"BREAKDOWN_TREND_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 55 {
			return false
		}
		prev := ctx.Prices[len(ctx.Prices)-21 : len(ctx.Prices)-1]
		lo := math.MaxFloat64
		for _, p := range prev {
			if p < lo {
				lo = p
			}
		}
		ema20 := ema(ctx.Prices, 20)
		ema50 := ema(ctx.Prices, 50)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < lo*0.9994 &&
			ctx.BTCPrice < ema20 &&
			ctx.BTCPrice < ema50 &&
			momentum(ctx.Prices, 3) < -0.0006 &&
			rsiVal > 32 && rsiVal < 46
	},

	// ── Triple Confluence ─────────────────────────────────────────────────
	"TRIPLE_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		rsiOk := rsi(ctx.Prices, 14) < 35
		emaOk := crossedAbove(ctx.Prices, 9, 21)
		momOk := momentum(ctx.Prices, 5) > 0.0005
		return rsiOk && emaOk && momOk
	},
	"TRIPLE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		rsiOk := rsi(ctx.Prices, 14) > 65
		emaOk := crossedBelow(ctx.Prices, 9, 21)
		momOk := momentum(ctx.Prices, 5) < -0.0005
		return rsiOk && emaOk && momOk
	},

	// ── HIGH IV (NIFTY: India VIX > 20 = elevated, > 30 = extreme) ──────
	"HIGH_IV_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return ctx.IV > 0.20 && momentum(ctx.Prices, 5) > 0.0008
	},
	"HIGH_IV_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return ctx.IV > 0.20 && momentum(ctx.Prices, 5) < -0.0008
	},

	// ── NSE session signals (already correct in base; re-calibrate thresholds) ──
	"NSE_OPEN_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		total := ctx.UTCHour*60 + ctx.UTCMin
		if total < 230 || total > 258 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		rsiVal := rsi(ctx.Prices, 14)
		vw := avgPrice(ctx.Prices[max0(len(ctx.Prices)-15):])
		return ctx.BTCPrice > vw && mom5 > 0.0004 && rsiVal > 48 && rsiVal < 70
	},
	"NSE_OPEN_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		total := ctx.UTCHour*60 + ctx.UTCMin
		if total < 230 || total > 258 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		rsiVal := rsi(ctx.Prices, 14)
		vw := avgPrice(ctx.Prices[max0(len(ctx.Prices)-15):])
		return ctx.BTCPrice < vw && mom5 < -0.0004 && rsiVal > 30 && rsiVal < 52
	},
	"NSE_MIDDAY_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		total := ctx.UTCHour*60 + ctx.UTCMin
		if total < 360 || total > 405 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		rsiVal := rsi(ctx.Prices, 14)
		vw := avgPrice(ctx.Prices[max0(len(ctx.Prices)-20):])
		return ctx.BTCPrice > vw && mom5 > 0.0004 && rsiVal > 52 && rsiVal < 72
	},
	"NSE_MIDDAY_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		total := ctx.UTCHour*60 + ctx.UTCMin
		if total < 360 || total > 405 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		rsiVal := rsi(ctx.Prices, 14)
		vw := avgPrice(ctx.Prices[max0(len(ctx.Prices)-20):])
		return ctx.BTCPrice < vw && mom5 < -0.0004 && rsiVal > 28 && rsiVal < 48
	},
	"NSE_PRECLOSE_SELL_CALL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		total := ctx.UTCHour*60 + ctx.UTCMin
		if total < 525 || total > 555 {
			return false
		}
		mom15 := momentum(ctx.Prices, 15)
		rsiVal := rsi(ctx.Prices, 14)
		return mom15 > 0.0005 && rsiVal > 54 && rsiVal < 74
	},
	"NSE_PRECLOSE_SELL_PUT": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		total := ctx.UTCHour*60 + ctx.UTCMin
		if total < 525 || total > 555 {
			return false
		}
		mom15 := momentum(ctx.Prices, 15)
		rsiVal := rsi(ctx.Prices, 14)
		return mom15 < -0.0005 && rsiVal > 26 && rsiVal < 46
	},

	// ── ATR Expansion ─────────────────────────────────────────────────────
	"ATR_EXPAND_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		recent := atrApprox(ctx.Prices, 5)
		prior := atrApprox(ctx.Prices[:len(ctx.Prices)-5], 14)
		if prior <= 0 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		rsiVal := rsi(ctx.Prices, 14)
		return recent > prior*1.40 && mom5 > 0.0007 && rsiVal < 68
	},
	"ATR_EXPAND_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		recent := atrApprox(ctx.Prices, 5)
		prior := atrApprox(ctx.Prices[:len(ctx.Prices)-5], 14)
		if prior <= 0 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		rsiVal := rsi(ctx.Prices, 14)
		return recent > prior*1.40 && mom5 < -0.0007 && rsiVal > 32
	},
}
