package options

import "math"

// SignalContext holds market data for signal evaluation.
// Prices contains 1-minute bar closes (not raw ticks).
type SignalContext struct {
	Prices   []float64 // 1-minute sampled price bars
	IV       float64
	BTCPrice float64
	UTCHour  int // current UTC hour (0-23), for session-aware signals
	UTCMin   int // current UTC minute (0-59)
}

type SignalFunc func(ctx SignalContext) bool

// ── Indicator helpers (operate on minute bars) ─────────────────────────────

func ema(prices []float64, period int) float64 {
	if len(prices) == 0 {
		return 0
	}
	if len(prices) < period {
		period = len(prices)
	}
	k := 2.0 / float64(period+1)
	val := prices[0]
	for _, p := range prices[1:] {
		val = p*k + val*(1-k)
	}
	return val
}

func rsi(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50
	}
	slice := prices[len(prices)-period-1:]
	var gains, losses float64
	for i := 1; i < len(slice); i++ {
		ch := slice[i] - slice[i-1]
		if ch > 0 {
			gains += ch
		} else {
			losses -= ch
		}
	}
	if losses == 0 {
		return 100
	}
	rs := (gains / float64(period)) / (losses / float64(period))
	return 100 - 100/(1+rs)
}

func stddev(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}
	mean := 0.0
	for _, p := range prices {
		mean += p
	}
	mean /= float64(len(prices))
	v := 0.0
	for _, p := range prices {
		d := p - mean
		v += d * d
	}
	return math.Sqrt(v / float64(len(prices)))
}

func bbUpper(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1]
	}
	s := prices[len(prices)-period:]
	mean := 0.0
	for _, p := range s {
		mean += p
	}
	mean /= float64(period)
	return mean + 2*stddev(s)
}

func bbLower(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1]
	}
	s := prices[len(prices)-period:]
	mean := 0.0
	for _, p := range s {
		mean += p
	}
	mean /= float64(period)
	return mean - 2*stddev(s)
}

func bbMid(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1]
	}
	s := prices[len(prices)-period:]
	mean := 0.0
	for _, p := range s {
		mean += p
	}
	return mean / float64(period)
}

// avgPrice computes the simple average of the given price slice.
// Note: this is an SMA proxy for VWAP since volume data is not available
// in SignalContext. In production with real volume, replace with true VWAP.
func avgPrice(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range prices {
		sum += p
	}
	return sum / float64(len(prices))
}

func stochK(prices []float64, period int) float64 {
	if len(prices) < period {
		return 50
	}
	s := prices[len(prices)-period:]
	lo, hi := math.MaxFloat64, 0.0
	for _, p := range s {
		if p < lo {
			lo = p
		}
		if p > hi {
			hi = p
		}
	}
	if hi == lo {
		return 50
	}
	return (prices[len(prices)-1] - lo) / (hi - lo) * 100
}

// momentum returns the % change from n bars ago to now
func momentum(prices []float64, n int) float64 {
	if len(prices) <= n {
		return 0
	}
	prev := prices[len(prices)-1-n]
	if prev == 0 {
		return 0
	}
	return (prices[len(prices)-1] - prev) / prev
}

// crossedAbove returns true if fast crossed above slow on the most recent bar
func crossedAbove(prices []float64, fastP, slowP int) bool {
	if len(prices) < slowP+2 {
		return false
	}
	fast := ema(prices, fastP)
	slow := ema(prices, slowP)
	pFast := ema(prices[:len(prices)-1], fastP)
	pSlow := ema(prices[:len(prices)-1], slowP)
	return fast > slow && pFast <= pSlow
}

func crossedBelow(prices []float64, fastP, slowP int) bool {
	if len(prices) < slowP+2 {
		return false
	}
	fast := ema(prices, fastP)
	slow := ema(prices, slowP)
	pFast := ema(prices[:len(prices)-1], fastP)
	pSlow := ema(prices[:len(prices)-1], slowP)
	return fast < slow && pFast >= pSlow
}

// ── Signal functions (all computed on 1-minute bars) ─────────────────────────

var Signals = map[string]SignalFunc{

	// ── Momentum signals ────────────────────────────────────────────────────
	// Require meaningful 5-minute momentum (0.25% = $25 on $10k BTC)
	"BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)   // 5-min momentum
		mom10 := momentum(ctx.Prices, 10) // 10-min momentum
		rsiVal := rsi(ctx.Prices, 14)
		// Price rising on both timeframes, RSI not overbought yet
		return mom5 > 0.0010 && mom10 > 0.0005 && rsiVal < 68
	},
	"BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0010 && mom10 < -0.0005 && rsiVal > 32
	},
	"STRONG_BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 > 0.0015 && mom10 > 0.0008 && rsiVal < 70
	},
	"STRONG_BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0015 && mom10 < -0.0008 && rsiVal > 30
	},

	// ── RSI signals ──────────────────────────────────────────────────────────
	// Use proper oversold/overbought thresholds with confirmation
	"RSI_OVERSOLD": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		// RSI crossed back above 30 from below (actual reversal signal, not just in oversold)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR < 34 && r >= 34 && r < 45
	},
	"RSI_OVERBOUGHT": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR > 66 && r <= 66 && r > 55
	},
	"RSI_OVERSOLD_EXTREME": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR < 25 && r >= 25
	},
	"RSI_OVERBOUGHT_EXTREME": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR > 75 && r <= 75
	},

	// ── EMA cross signals ────────────────────────────────────────────────────
	// Actual crossover events (not sustained state)
	"EMA_BULL_CROSS": func(ctx SignalContext) bool {
		return crossedAbove(ctx.Prices, 9, 21)
	},
	"EMA_BEAR_CROSS": func(ctx SignalContext) bool {
		return crossedBelow(ctx.Prices, 9, 21)
	},
	// Sustained EMA state — fires every bar while price is above both EMAs
	// with positive momentum. Does NOT require a crossover event.
	"EMA_ABOVE_BOTH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 {
			return false
		}
		e9 := ema(ctx.Prices, 9)
		e21 := ema(ctx.Prices, 21)
		mom3 := momentum(ctx.Prices, 3)
		return ctx.BTCPrice > e9 && e9 > e21 && mom3 > 0.00005
	},
	"EMA_BELOW_BOTH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 {
			return false
		}
		e9 := ema(ctx.Prices, 9)
		e21 := ema(ctx.Prices, 21)
		mom3 := momentum(ctx.Prices, 3)
		return ctx.BTCPrice < e9 && e9 < e21 && mom3 < -0.00005
	},

	// ── Bollinger Band signals ───────────────────────────────────────────────
	// Price touched band AND is now bouncing back
	"BB_LOWER_TOUCH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 {
			return false
		}
		prevPrice := ctx.Prices[len(ctx.Prices)-2]
		lower := bbLower(ctx.Prices, 20)
		mid := bbMid(ctx.Prices, 20)
		// Previous bar touched lower band, current bar is recovering toward midline
		return prevPrice <= lower && ctx.BTCPrice > prevPrice && ctx.BTCPrice < mid
	},
	"BB_UPPER_TOUCH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 {
			return false
		}
		prevPrice := ctx.Prices[len(ctx.Prices)-2]
		upper := bbUpper(ctx.Prices, 20)
		mid := bbMid(ctx.Prices, 20)
		return prevPrice >= upper && ctx.BTCPrice < prevPrice && ctx.BTCPrice > mid
	},
	// BB squeeze breakout: bands were tight AND price just broke out
	"BB_SQUEEZE_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 {
			return false
		}
		recentStd := stddev(ctx.Prices[len(ctx.Prices)-10:])
		priorStd := stddev(ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-10])
		squeezed := recentStd < priorStd*0.75
		breakout := momentum(ctx.Prices, 3) > 0.0018
		return squeezed && breakout
	},
	"BB_SQUEEZE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 {
			return false
		}
		recentStd := stddev(ctx.Prices[len(ctx.Prices)-10:])
		priorStd := stddev(ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-10])
		squeezed := recentStd < priorStd*0.75
		breakout := momentum(ctx.Prices, 3) < -0.0018
		return squeezed && breakout
	},

	// ── VWAP signals (require meaningful deviation) ───────────────────────────
	// Price must be significantly above/below VWAP AND trending in that direction
	"VWAP_ABOVE": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (ctx.BTCPrice - vw) / vw
		// Must be 0.2% above VWAP AND have upward momentum
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

	// ── Breakout signals ─────────────────────────────────────────────────────
	// Price breaks above/below the prior 20-bar high/low
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
		// Clean break above prior high with momentum
		return ctx.BTCPrice > hi*1.0018 && momentum(ctx.Prices, 3) > 0.0014
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
		return ctx.BTCPrice < lo*0.9982 && momentum(ctx.Prices, 3) < -0.0014
	},

	// ── Stochastic signals ────────────────────────────────────────────────────
	// Stoch crossed from oversold/overbought with RSI confirmation
	"STOCH_OVERSOLD": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		k := stochK(ctx.Prices, 14)
		prevK := stochK(ctx.Prices[:len(ctx.Prices)-1], 14)
		rsiVal := rsi(ctx.Prices, 14)
		return prevK < 25 && k >= 25 && rsiVal < 55
	},
	"STOCH_OVERBOUGHT": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		k := stochK(ctx.Prices, 14)
		prevK := stochK(ctx.Prices[:len(ctx.Prices)-1], 14)
		rsiVal := rsi(ctx.Prices, 14)
		return prevK > 75 && k <= 75 && rsiVal > 45
	},

	// ── Confluence signals ────────────────────────────────────────────────────
	// TRIPLE_BULL: 3 independent conditions all agree bullish
	"TRIPLE_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		// RSI genuinely oversold (below 35, not just below 45)
		rsiOk := rsi(ctx.Prices, 14) < 35
		// EMA cross bullish
		emaOk := crossedAbove(ctx.Prices, 9, 21)
		// 5-min positive momentum
		momOk := momentum(ctx.Prices, 5) > 0.002
		return rsiOk && emaOk && momOk
	},
	"TRIPLE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		rsiOk := rsi(ctx.Prices, 14) > 65
		emaOk := crossedBelow(ctx.Prices, 9, 21)
		momOk := momentum(ctx.Prices, 5) < -0.002
		return rsiOk && emaOk && momOk
	},

	// ── IV-based signals ──────────────────────────────────────────────────────
	// High IV + directional momentum = vol expansion play
	// Hybrid high-conviction signals combine multiple confirmed edges.
	"MOMENTUM_VWAP_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > vw*1.0015 && mom5 > 0.0020 && mom10 > 0.0012 && rsiVal > 48 && rsiVal < 68
	},
	"MOMENTUM_VWAP_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < vw*0.9985 && mom5 < -0.0020 && mom10 < -0.0012 && rsiVal > 32 && rsiVal < 52
	},
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
		return ctx.BTCPrice > hi*1.0018 &&
			ctx.BTCPrice > ema20 &&
			ctx.BTCPrice > ema50 &&
			momentum(ctx.Prices, 3) > 0.0014 &&
			rsiVal > 52 && rsiVal < 70
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
		return ctx.BTCPrice < lo*0.9982 &&
			ctx.BTCPrice < ema20 &&
			ctx.BTCPrice < ema50 &&
			momentum(ctx.Prices, 3) < -0.0014 &&
			rsiVal > 30 && rsiVal < 48
	},
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
		return drop > 0.0040 &&
			recovery > 0.0018 &&
			ctx.BTCPrice > shortVWAP &&
			ctx.BTCPrice > ema9 &&
			rsiVal > 38 && rsiVal < 56
	},
	"HIGH_IV_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		// IV > 60% annualized (elevated but not extreme)
		return ctx.IV > 0.60 && momentum(ctx.Prices, 5) > 0.003
	},
	"HIGH_IV_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return ctx.IV > 0.60 && momentum(ctx.Prices, 5) < -0.003
	},

	// ── Price action reversal signals ─────────────────────────────────────────
	// Sharp drop followed by confirmed recovery (V-reversal)
	"SHARP_REVERSAL_UP": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		// Find the low in last 5 bars
		window := ctx.Prices[len(ctx.Prices)-6 : len(ctx.Prices)-1]
		lo := math.MaxFloat64
		for _, p := range window {
			if p < lo {
				lo = p
			}
		}
		dropFromHigh := (ctx.Prices[len(ctx.Prices)-6] - lo) / ctx.Prices[len(ctx.Prices)-6]
		recovery := (ctx.BTCPrice - lo) / lo
		// Must have dropped at least 0.3% and recovered at least 0.15%
		return dropFromHigh > 0.003 && recovery > 0.0015 && ctx.BTCPrice > lo
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
		return riseFromLow > 0.003 && rejection > 0.0015 && ctx.BTCPrice < hi
	},

	// ── Strategy 1: Consecutive Candle Momentum ────────────────────────────────
	// BTC momentum is autocorrelated: 4 consecutive bullish/bearish 1-min bars
	// signal continuation of the move for the next 3-5 bars.
	// This captures the "momentum burst" phenomenon seen in liquid crypto markets.
	"CONSEC_BULL_BARS": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 6 {
			return false
		}
		n := len(ctx.Prices)
		// All 4 recent bars must close higher than the previous bar
		for i := n - 4; i < n; i++ {
			if ctx.Prices[i] <= ctx.Prices[i-1] {
				return false
			}
		}
		// Total 4-bar gain must be meaningful (>0.18%) — filters noise
		totalGain := (ctx.Prices[n-1] - ctx.Prices[n-5]) / ctx.Prices[n-5]
		// RSI must not be deep overbought — leave room for the move to continue
		rsiVal := rsi(ctx.Prices, 14)
		return totalGain > 0.0018 && rsiVal < 72
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
		return totalLoss > 0.0018 && rsiVal > 28
	},

	// ── Strategy 2: Volatility Compression Breakout ────────────────────────────
	// When price squeezes into a tight range (low realised vol), energy builds.
	// The first directional move out of the compression tends to be explosive.
	// Buying when options are cheap (vol compressed) gives: delta gain + vega gain.
	"VOL_COMPRESS_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 {
			return false
		}
		n := len(ctx.Prices)
		// Compression: recent 10-bar std is less than 50% of the 60-bar historical std
		recentStd := stddev(ctx.Prices[n-10:])
		historicalStd := stddev(ctx.Prices[n-40:])
		if historicalStd == 0 {
			return false
		}
		compressed := recentStd < historicalStd*0.65
		// Breakout: strong upward momentum breaking out of the compression
		breakout := momentum(ctx.Prices, 5) > 0.0016
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
		breakout := momentum(ctx.Prices, 5) < -0.0016
		rsiVal := rsi(ctx.Prices, 14)
		return compressed && breakout && rsiVal > 35
	},

	// ── Strategy 3: Session Open Momentum ─────────────────────────────────────
	// BTC sees fresh institutional order flow at key UTC session opens.
	// The direction of the first 5-15 minutes tends to persist for 60-90 minutes.
	// Key opens: UTC 00:00 (Asia), 08:00 (Europe), 13:30 (NYSE), 20:00 (US evening).
	"SESSION_OPEN_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		// Check within 1-30 minutes of a key session open
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		sessions := []int{0, 480, 810, 1200} // 00:00, 08:00, 13:30, 20:00
		nearSession := false
		for _, s := range sessions {
			diff := totalMin - s
			if diff >= 1 && diff <= 30 {
				nearSession = true
				break
			}
		}
		if !nearSession {
			return false
		}
		// Strong bullish momentum in the opening bars
		mom := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom > 0.0020 && rsiVal < 65
	},
	"SESSION_OPEN_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		sessions := []int{0, 480, 810, 1200}
		nearSession := false
		for _, s := range sessions {
			diff := totalMin - s
			if diff >= 1 && diff <= 30 {
				nearSession = true
				break
			}
		}
		if !nearSession {
			return false
		}
		mom := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom < -0.0020 && rsiVal > 35
	},

	// ── Strategy 4: Capitulation V-Reversal ───────────────────────────────────
	// Sharp panic drops (>0.7% in 5 bars) clear weak longs via stop-hunting.
	// When price snaps back firmly (>0.35% recovery), the selling is exhausted
	// and the path of least resistance flips back up.
	// This targets the "V" bottom — one of the highest-probability setups in crypto.
	"CAPITULATION_RECOVERY": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 {
			return false
		}
		n := len(ctx.Prices)
		// Find the lowest point in the 5-bar window ending 1 bar before current
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
		// Drop from start to the low must be at least 0.4%
		drop := (startPrice - lo) / startPrice
		// Current price must have recovered at least 0.18% from the low
		recovery := (ctx.BTCPrice - lo) / lo
		// RSI not yet overbought — means the recovery can continue
		rsiVal := rsi(ctx.Prices, 14)
		return drop > 0.0040 && recovery > 0.0018 && ctx.BTCPrice > lo && rsiVal < 56
	},

	// ── Strategy 5: Overextension Fade ────────────────────────────────────────
	// BTC mean-reverts after rapid >2% moves in either direction.
	// When RSI is at an extreme AND price is at the Bollinger Band AND
	// the 30-minute move is outsized, the rubber band effect kicks in.
	// Buy puts after excessive rallies, calls after excessive selloffs.
	// This is a contrarian strategy — only valid with ALL three confirmations.
	"OVEREXTENSION_FADE_UP": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		// 30-min move > 2.0% upward
		mom30 := momentum(ctx.Prices, 30)
		// RSI deeply overbought
		rsiVal := rsi(ctx.Prices, 14)
		// Price at or above upper Bollinger Band
		atUpper := ctx.BTCPrice >= bbUpper(ctx.Prices, 20)*0.999
		// Momentum starting to stall: last 3 bars not accelerating
		mom3 := momentum(ctx.Prices, 3)
		return mom30 > 0.012 && rsiVal > 72 && atUpper && mom3 < mom30/8
	},
	"OVEREXTENSION_FADE_DOWN": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		// 30-min move > 2.0% downward
		mom30 := momentum(ctx.Prices, 30)
		rsiVal := rsi(ctx.Prices, 14)
		atLower := ctx.BTCPrice <= bbLower(ctx.Prices, 20)*1.001
		mom3 := momentum(ctx.Prices, 3)
		return mom30 < -0.012 && rsiVal < 28 && atLower && mom3 > mom30/8
	},
}

// NiftySignals contains signal functions calibrated for NIFTY 50.
// NIFTY IV ≈ 18% vs BTC ≈ 80%, so momentum thresholds scale by ~0.25×.
// RSI, EMA cross, BB, and Stochastic signals are reused unchanged.
var NiftySignals map[string]SignalFunc

func init() {
	NiftySignals = make(map[string]SignalFunc, len(Signals))
	for k, v := range Signals {
		NiftySignals[k] = v
	}

	NiftySignals["BULL_MOMENTUM"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return momentum(ctx.Prices, 5) > 0.0006 && momentum(ctx.Prices, 10) > 0.0003 && rsi(ctx.Prices, 14) < 64
	}
	NiftySignals["BEAR_MOMENTUM"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return momentum(ctx.Prices, 5) < -0.0006 && momentum(ctx.Prices, 10) < -0.0003 && rsi(ctx.Prices, 14) > 36
	}
	NiftySignals["STRONG_BULL_MOMENTUM"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return momentum(ctx.Prices, 5) > 0.0010 && momentum(ctx.Prices, 10) > 0.0005 && rsi(ctx.Prices, 14) < 68
	}
	NiftySignals["STRONG_BEAR_MOMENTUM"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return momentum(ctx.Prices, 5) < -0.0010 && momentum(ctx.Prices, 10) < -0.0005 && rsi(ctx.Prices, 14) > 32
	}
	NiftySignals["VWAP_ABOVE"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		return (ctx.BTCPrice-vw)/vw > 0.0010 && momentum(ctx.Prices, 5) > 0.0006
	}
	NiftySignals["VWAP_BELOW"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		return (vw-ctx.BTCPrice)/vw > 0.0010 && momentum(ctx.Prices, 5) < -0.0006
	}
	NiftySignals["RESISTANCE_BREAK"] = func(ctx SignalContext) bool {
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
		return ctx.BTCPrice > hi*1.0007 && momentum(ctx.Prices, 3) > 0.0006
	}
	NiftySignals["SUPPORT_BREAK"] = func(ctx SignalContext) bool {
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
		return ctx.BTCPrice < lo*0.9993 && momentum(ctx.Prices, 3) < -0.0006
	}
	NiftySignals["BB_SQUEEZE_BULL"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 {
			return false
		}
		n := len(ctx.Prices)
		return stddev(ctx.Prices[n-10:]) < stddev(ctx.Prices[n-30:n-10])*0.75 && momentum(ctx.Prices, 3) > 0.0005
	}
	NiftySignals["BB_SQUEEZE_BEAR"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 {
			return false
		}
		n := len(ctx.Prices)
		return stddev(ctx.Prices[n-10:]) < stddev(ctx.Prices[n-30:n-10])*0.75 && momentum(ctx.Prices, 3) < -0.0005
	}
	NiftySignals["TRIPLE_BULL"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		return rsi(ctx.Prices, 14) < 35 && crossedAbove(ctx.Prices, 9, 21) && momentum(ctx.Prices, 5) > 0.0005
	}
	NiftySignals["TRIPLE_BEAR"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 {
			return false
		}
		return rsi(ctx.Prices, 14) > 65 && crossedBelow(ctx.Prices, 9, 21) && momentum(ctx.Prices, 5) < -0.0005
	}
	NiftySignals["MOMENTUM_VWAP_BULL"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > vw*1.0006 && momentum(ctx.Prices, 5) > 0.0008 && momentum(ctx.Prices, 10) > 0.0004 && r > 50 && r < 66
	}
	NiftySignals["MOMENTUM_VWAP_BEAR"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < vw*0.9994 && momentum(ctx.Prices, 5) < -0.0008 && momentum(ctx.Prices, 10) < -0.0004 && r > 34 && r < 50
	}
	NiftySignals["BREAKOUT_TREND_BULL"] = func(ctx SignalContext) bool {
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
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > hi*1.0007 && ctx.BTCPrice > ema(ctx.Prices, 20) && ctx.BTCPrice > ema(ctx.Prices, 50) && momentum(ctx.Prices, 3) > 0.0006 && r > 54 && r < 68
	}
	NiftySignals["BREAKDOWN_TREND_BEAR"] = func(ctx SignalContext) bool {
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
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < lo*0.9993 && ctx.BTCPrice < ema(ctx.Prices, 20) && ctx.BTCPrice < ema(ctx.Prices, 50) && momentum(ctx.Prices, 3) < -0.0006 && r > 32 && r < 46
	}
	NiftySignals["VOL_COMPRESS_BULL"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 {
			return false
		}
		n := len(ctx.Prices)
		h := stddev(ctx.Prices[n-40:])
		return h > 0 && stddev(ctx.Prices[n-10:]) < h*0.65 && momentum(ctx.Prices, 5) > 0.0007 && rsi(ctx.Prices, 14) < 65
	}
	NiftySignals["VOL_COMPRESS_BEAR"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 {
			return false
		}
		n := len(ctx.Prices)
		h := stddev(ctx.Prices[n-40:])
		return h > 0 && stddev(ctx.Prices[n-10:]) < h*0.65 && momentum(ctx.Prices, 5) < -0.0007 && rsi(ctx.Prices, 14) > 35
	}
	NiftySignals["CONSEC_BULL_BARS"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 6 {
			return false
		}
		n := len(ctx.Prices)
		for i := n - 4; i < n; i++ {
			if ctx.Prices[i] <= ctx.Prices[i-1] {
				return false
			}
		}
		return (ctx.Prices[n-1]-ctx.Prices[n-5])/ctx.Prices[n-5] > 0.0008 && rsi(ctx.Prices, 14) < 72
	}
	NiftySignals["CONSEC_BEAR_BARS"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 6 {
			return false
		}
		n := len(ctx.Prices)
		for i := n - 4; i < n; i++ {
			if ctx.Prices[i] >= ctx.Prices[i-1] {
				return false
			}
		}
		return (ctx.Prices[n-5]-ctx.Prices[n-1])/ctx.Prices[n-5] > 0.0008 && rsi(ctx.Prices, 14) > 28
	}
	NiftySignals["SHARP_REVERSAL_UP"] = func(ctx SignalContext) bool {
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
		return dropFromHigh > 0.0008 && (ctx.BTCPrice-lo)/lo > 0.0004 && ctx.BTCPrice > lo
	}
	NiftySignals["SHARP_REVERSAL_DOWN"] = func(ctx SignalContext) bool {
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
		return riseFromLow > 0.0008 && (hi-ctx.BTCPrice)/hi > 0.0004 && ctx.BTCPrice < hi
	}
	NiftySignals["OVEREXTENSION_FADE_UP"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		mom30 := momentum(ctx.Prices, 30)
		mom3 := momentum(ctx.Prices, 3)
		return mom30 > 0.0030 && rsi(ctx.Prices, 14) > 72 && ctx.BTCPrice >= bbUpper(ctx.Prices, 20)*0.999 && mom3 < mom30/8
	}
	NiftySignals["OVEREXTENSION_FADE_DOWN"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		mom30 := momentum(ctx.Prices, 30)
		mom3 := momentum(ctx.Prices, 3)
		return mom30 < -0.0030 && rsi(ctx.Prices, 14) < 28 && ctx.BTCPrice <= bbLower(ctx.Prices, 20)*1.001 && mom3 > mom30/8
	}
	NiftySignals["CAPITULATION_RECOVERY"] = func(ctx SignalContext) bool {
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
		return (startPrice-lo)/startPrice > 0.0014 && (ctx.BTCPrice-lo)/lo > 0.0006 && ctx.BTCPrice > lo && rsi(ctx.Prices, 14) < 56
	}
	NiftySignals["CAPITULATION_RECLAIM"] = func(ctx SignalContext) bool {
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
		shortVWAP := avgPrice(ctx.Prices[n-15:])
		r := rsi(ctx.Prices, 14)
		return (startPrice-lo)/startPrice > 0.0014 && (ctx.BTCPrice-lo)/lo > 0.0007 && ctx.BTCPrice > shortVWAP && ctx.BTCPrice > ema(ctx.Prices, 9) && r > 38 && r < 56
	}
	NiftySignals["HIGH_IV_BULL"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return ctx.IV > 0.25 && momentum(ctx.Prices, 5) > 0.0008
	}
	NiftySignals["HIGH_IV_BEAR"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		return ctx.IV > 0.25 && momentum(ctx.Prices, 5) < -0.0008
	}
	// NSE sessions: 03:45 UTC (open), 06:30 UTC (midday)
	NiftySignals["SESSION_OPEN_BULL"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		for _, s := range []int{225, 390} {
			if d := totalMin - s; d >= 1 && d <= 20 {
				return momentum(ctx.Prices, 10) > 0.0009 && rsi(ctx.Prices, 14) < 65
			}
		}
		return false
	}
	NiftySignals["SESSION_OPEN_BEAR"] = func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		for _, s := range []int{225, 390} {
			if d := totalMin - s; d >= 1 && d <= 20 {
				return momentum(ctx.Prices, 10) < -0.0009 && rsi(ctx.Prices, 14) > 35
			}
		}
		return false
	}
}
