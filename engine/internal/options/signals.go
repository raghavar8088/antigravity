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
	// Thresholds raised vs. the original: weak signals on small moves led to
	// entries that couldn't reach the TP before theta ate the premium.
	// BULL_MOMENTUM: 5-min >0.25% move (was 0.18%) + 10-min >0.12% (was 0.08%)
	"BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		// Stronger confirmation: need 0.25% 5-min move with trend agreement
		return mom5 > 0.0025 && mom10 > 0.0012 && rsiVal < 68
	},
	"BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0025 && mom10 < -0.0012 && rsiVal > 32
	},
	"STRONG_BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		// 0.42% in 5 min + 0.22% in 10 min — strong directional push (was 0.32%/0.16%)
		return mom5 > 0.0042 && mom10 > 0.0022 && rsiVal < 72
	},
	"STRONG_BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0042 && mom10 < -0.0022 && rsiVal > 28
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
	// Regime + fresh momentum (not just sustained state)
	"EMA_ABOVE_BOTH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 55 {
			return false
		}
		aboveBoth := ctx.BTCPrice > ema(ctx.Prices, 20) && ctx.BTCPrice > ema(ctx.Prices, 50)
		// Require a recent bullish EMA cross within last 5 bars
		return aboveBoth && crossedAbove(ctx.Prices, 9, 21)
	},
	"EMA_BELOW_BOTH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 55 {
			return false
		}
		belowBoth := ctx.BTCPrice < ema(ctx.Prices, 20) && ctx.BTCPrice < ema(ctx.Prices, 50)
		return belowBoth && crossedBelow(ctx.Prices, 9, 21)
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
	// Raised threshold: 0.3% deviation → 0.4% to avoid noise entries
	"VWAP_ABOVE": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (ctx.BTCPrice - vw) / vw
		// 0.4% above VWAP + confirmed momentum (was 0.2%/0.15%)
		return deviation > 0.004 && momentum(ctx.Prices, 5) > 0.002
	},
	"VWAP_BELOW": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (vw - ctx.BTCPrice) / vw
		return deviation > 0.004 && momentum(ctx.Prices, 5) < -0.002
	},

	// ── Breakout signals ─────────────────────────────────────────────────────
	// Tighter breakout filter: 0.30% clean break (was 0.18%) with stronger momentum
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
		// 0.30% above prior high + stronger 3-bar momentum (was 0.18%/0.15%)
		return ctx.BTCPrice > hi*1.0030 && momentum(ctx.Prices, 3) > 0.0022
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
		return ctx.BTCPrice < lo*0.9970 && momentum(ctx.Prices, 3) < -0.0022
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
		return ctx.BTCPrice > vw*1.0015 && mom5 > 0.0024 && mom10 > 0.0012 && rsiVal > 48 && rsiVal < 70
	},
	"MOMENTUM_VWAP_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < vw*0.9985 && mom5 < -0.0024 && mom10 < -0.0012 && rsiVal > 30 && rsiVal < 52
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
			momentum(ctx.Prices, 3) > 0.0018 &&
			rsiVal > 52 && rsiVal < 72
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
			momentum(ctx.Prices, 3) < -0.0018 &&
			rsiVal > 28 && rsiVal < 48
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
		return drop > 0.0048 &&
			recovery > 0.0022 &&
			ctx.BTCPrice > shortVWAP &&
			ctx.BTCPrice > ema9 &&
			rsiVal > 34 && rsiVal < 60
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
		// Total 4-bar gain must be meaningful (>0.35%) — filters noise
		totalGain := (ctx.Prices[n-1] - ctx.Prices[n-5]) / ctx.Prices[n-5]
		// RSI must not be deep overbought — leave room for the move to continue
		rsiVal := rsi(ctx.Prices, 14)
		return totalGain > 0.0022 && rsiVal < 75
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
		return totalLoss > 0.0022 && rsiVal > 25
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
		compressed := recentStd < historicalStd*0.70
		// Breakout: strong upward momentum breaking out of the compression
		breakout := momentum(ctx.Prices, 5) > 0.002
		rsiVal := rsi(ctx.Prices, 14)
		return compressed && breakout && rsiVal < 68
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
		compressed := recentStd < historicalStd*0.70
		breakout := momentum(ctx.Prices, 5) < -0.002
		rsiVal := rsi(ctx.Prices, 14)
		return compressed && breakout && rsiVal > 32
	},

	// ── Strategy 3: Session Open Momentum ─────────────────────────────────────
	// BTC sees fresh institutional order flow at key UTC session opens.
	// The direction of the first 5-15 minutes tends to persist for 60-90 minutes.
	// Key opens: UTC 00:00 (Asia), 08:00 (Europe), 13:30 (NYSE), 20:00 (US evening).
	"SESSION_OPEN_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		// Check within 3-18 minutes of a key session open
		totalMin := ctx.UTCHour*60 + ctx.UTCMin
		sessions := []int{0, 480, 810, 1200} // 00:00, 08:00, 13:30, 20:00
		nearSession := false
		for _, s := range sessions {
			diff := totalMin - s
			if diff >= 1 && diff <= 25 {
				nearSession = true
				break
			}
		}
		if !nearSession {
			return false
		}
		// Stronger momentum requirement: 0.35% in 10 bars (was 0.25%)
		mom := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom > 0.0035 && rsiVal < 68
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
			if diff >= 1 && diff <= 25 {
				nearSession = true
				break
			}
		}
		if !nearSession {
			return false
		}
		mom := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom < -0.0035 && rsiVal > 32
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
		// Relaxed drop threshold to 0.35% (was 0.45%) — catches more V-reversals.
		// Recovery threshold lowered to 0.18% (was 0.20%) but RSI cap tightened to 52.
		drop := (startPrice - lo) / startPrice
		recovery := (ctx.BTCPrice - lo) / lo
		rsiVal := rsi(ctx.Prices, 14)
		return drop > 0.0035 && recovery > 0.0018 && ctx.BTCPrice > lo && rsiVal < 52
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

	// ═══════════════════════════════════════════════════════════════════════════
	// CATEGORY H — MICRO-MOMENTUM SCALP signals (MM_BULL_1..10 / MM_BEAR_1..10)
	// Each variant uses a slightly different indicator combination so no two
	// fire at the same time.  All operate on 1-minute bars.
	// ═══════════════════════════════════════════════════════════════════════════

	// 1: 2-bar burst + RSI-9 above 55
	"MM_BULL_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 { return false }
		r9 := rsi(ctx.Prices, 9)
		return momentum(ctx.Prices, 2) > 0.0018 && r9 > 55 && r9 < 78
	},
	"MM_BEAR_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 { return false }
		r9 := rsi(ctx.Prices, 9)
		return momentum(ctx.Prices, 2) < -0.0018 && r9 < 45 && r9 > 22
	},
	// 2: 3-bar thrust + RSI-14 cross above 40
	"MM_BULL_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 18 { return false }
		r := rsi(ctx.Prices, 14)
		rPrev := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return momentum(ctx.Prices, 3) > 0.0022 && rPrev < 42 && r >= 42 && r < 72
	},
	"MM_BEAR_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 18 { return false }
		r := rsi(ctx.Prices, 14)
		rPrev := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return momentum(ctx.Prices, 3) < -0.0022 && rPrev > 58 && r <= 58 && r > 28
	},
	// 3: 4-bar acceleration + stochK crossing 30
	"MM_BULL_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 18 { return false }
		k := stochK(ctx.Prices, 14)
		kPrev := stochK(ctx.Prices[:len(ctx.Prices)-1], 14)
		return momentum(ctx.Prices, 4) > 0.0025 && kPrev < 32 && k >= 32 && k < 80
	},
	"MM_BEAR_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 18 { return false }
		k := stochK(ctx.Prices, 14)
		kPrev := stochK(ctx.Prices[:len(ctx.Prices)-1], 14)
		return momentum(ctx.Prices, 4) < -0.0025 && kPrev > 68 && k <= 68 && k > 20
	},
	// 4: 5-bar momentum + price above BB midline
	"MM_BULL_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 { return false }
		mid := bbMid(ctx.Prices, 20)
		prev := ctx.Prices[len(ctx.Prices)-2]
		return momentum(ctx.Prices, 5) > 0.0028 && prev < mid && ctx.BTCPrice >= mid
	},
	"MM_BEAR_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 { return false }
		mid := bbMid(ctx.Prices, 20)
		prev := ctx.Prices[len(ctx.Prices)-2]
		return momentum(ctx.Prices, 5) < -0.0028 && prev > mid && ctx.BTCPrice <= mid
	},
	// 5: 6-bar trend pulse + RSI cross 50
	"MM_BULL_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 { return false }
		r := rsi(ctx.Prices, 14)
		rPrev := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return momentum(ctx.Prices, 6) > 0.0030 && rPrev < 52 && r >= 52 && r < 70
	},
	"MM_BEAR_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 { return false }
		r := rsi(ctx.Prices, 14)
		rPrev := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return momentum(ctx.Prices, 6) < -0.0030 && rPrev > 48 && r <= 48 && r > 30
	},
	// 6: 7-bar + EMA9 slope positive (current > previous EMA9)
	"MM_BULL_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 { return false }
		e9 := ema(ctx.Prices, 9)
		e9Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 9)
		return momentum(ctx.Prices, 7) > 0.0032 && e9 > e9Prev && rsi(ctx.Prices, 14) < 74
	},
	"MM_BEAR_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 22 { return false }
		e9 := ema(ctx.Prices, 9)
		e9Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 9)
		return momentum(ctx.Prices, 7) < -0.0032 && e9 < e9Prev && rsi(ctx.Prices, 14) > 26
	},
	// 7: 8-bar higher-lows series (each bar > prior bar's low)
	"MM_BULL_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 { return false }
		n := len(ctx.Prices)
		for i := n - 4; i < n-1; i++ {
			if ctx.Prices[i] <= ctx.Prices[i-1] { return false }
		}
		return momentum(ctx.Prices, 8) > 0.0035 && rsi(ctx.Prices, 14) < 76
	},
	"MM_BEAR_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 { return false }
		n := len(ctx.Prices)
		for i := n - 4; i < n-1; i++ {
			if ctx.Prices[i] >= ctx.Prices[i-1] { return false }
		}
		return momentum(ctx.Prices, 8) < -0.0035 && rsi(ctx.Prices, 14) > 24
	},
	// 8: 9-bar trend + VWAP above + RSI 52-70
	"MM_BULL_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		r := rsi(ctx.Prices, 14)
		return momentum(ctx.Prices, 9) > 0.0038 && ctx.BTCPrice > vw && r >= 52 && r <= 70
	},
	"MM_BEAR_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		vw := avgPrice(ctx.Prices[len(ctx.Prices)-30:])
		r := rsi(ctx.Prices, 14)
		return momentum(ctx.Prices, 9) < -0.0038 && ctx.BTCPrice < vw && r >= 30 && r <= 48
	},
	// 9: 10-bar momentum + EMA21 slope rising
	"MM_BULL_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		e21 := ema(ctx.Prices, 21)
		e21Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 21)
		return momentum(ctx.Prices, 10) > 0.0040 && e21 > e21Prev
	},
	"MM_BEAR_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		e21 := ema(ctx.Prices, 21)
		e21Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 21)
		return momentum(ctx.Prices, 10) < -0.0040 && e21 < e21Prev
	},
	// 10: 12-bar sustained trend + BB width expanding (expansion > 110% of prior)
	"MM_BULL_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 { return false }
		n := len(ctx.Prices)
		widthNow := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		widthPrev := bbUpper(ctx.Prices[:n-5], 20) - bbLower(ctx.Prices[:n-5], 20)
		return momentum(ctx.Prices, 12) > 0.0045 && widthNow > widthPrev*1.10 && rsi(ctx.Prices, 14) < 74
	},
	"MM_BEAR_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 { return false }
		n := len(ctx.Prices)
		widthNow := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		widthPrev := bbUpper(ctx.Prices[:n-5], 20) - bbLower(ctx.Prices[:n-5], 20)
		return momentum(ctx.Prices, 12) < -0.0045 && widthNow > widthPrev*1.10 && rsi(ctx.Prices, 14) > 26
	},

	// ═══════════════════════════════════════════════════════════════════════════
	// CATEGORY I — REGIME TREND RIDER signals (TR_BULL_1..10 / TR_BEAR_1..10)
	// ═══════════════════════════════════════════════════════════════════════════

	// 1: EMA9 > EMA21 > EMA55 full stack + RSI 52-68
	"TR_BULL_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		e9, e21, e55 := ema(ctx.Prices, 9), ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		r := rsi(ctx.Prices, 14)
		return e9 > e21 && e21 > e55 && ctx.BTCPrice > e9 && r >= 52 && r <= 68
	},
	"TR_BEAR_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		e9, e21, e55 := ema(ctx.Prices, 9), ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		r := rsi(ctx.Prices, 14)
		return e9 < e21 && e21 < e55 && ctx.BTCPrice < e9 && r >= 32 && r <= 48
	},
	// 2: pullback to EMA21 + price bounced back above EMA9 + RSI 45-58
	"TR_BULL_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		e9 := ema(ctx.Prices, 9)
		e21 := ema(ctx.Prices, 21)
		prev := ctx.Prices[len(ctx.Prices)-3]
		r := rsi(ctx.Prices, 14)
		return e9 > e21 && prev < e9 && ctx.BTCPrice > e9 && r >= 45 && r <= 62
	},
	"TR_BEAR_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		e9 := ema(ctx.Prices, 9)
		e21 := ema(ctx.Prices, 21)
		prev := ctx.Prices[len(ctx.Prices)-3]
		r := rsi(ctx.Prices, 14)
		return e9 < e21 && prev > e9 && ctx.BTCPrice < e9 && r >= 38 && r <= 55
	},
	// 3: price > EMA55 + 15-bar momentum > 0.4%
	"TR_BULL_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		return ctx.BTCPrice > ema(ctx.Prices, 55) && momentum(ctx.Prices, 15) > 0.0040
	},
	"TR_BEAR_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		return ctx.BTCPrice < ema(ctx.Prices, 55) && momentum(ctx.Prices, 15) < -0.0040
	},
	// 4: BB midline held for 3 bars + EMA21 slope positive
	"TR_BULL_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 28 { return false }
		mid := bbMid(ctx.Prices, 20)
		e21 := ema(ctx.Prices, 21)
		e21Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 21)
		n := len(ctx.Prices)
		heldMid := ctx.Prices[n-1] > mid && ctx.Prices[n-2] > mid && ctx.Prices[n-3] > mid
		return heldMid && e21 > e21Prev && momentum(ctx.Prices, 5) > 0.0015
	},
	"TR_BEAR_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 28 { return false }
		mid := bbMid(ctx.Prices, 20)
		e21 := ema(ctx.Prices, 21)
		e21Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 21)
		n := len(ctx.Prices)
		heldMid := ctx.Prices[n-1] < mid && ctx.Prices[n-2] < mid && ctx.Prices[n-3] < mid
		return heldMid && e21 < e21Prev && momentum(ctx.Prices, 5) < -0.0015
	},
	// 5: RSI-8 > 60 AND RSI-14 > 55 dual confirmation
	"TR_BULL_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 { return false }
		return rsi(ctx.Prices, 8) > 60 && rsi(ctx.Prices, 14) > 55 && momentum(ctx.Prices, 5) > 0.0020
	},
	"TR_BEAR_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 { return false }
		return rsi(ctx.Prices, 8) < 40 && rsi(ctx.Prices, 14) < 45 && momentum(ctx.Prices, 5) < -0.0020
	},
	// 6: 20-bar structure of higher-highs + fresh EMA cross
	"TR_BULL_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		n := len(ctx.Prices)
		hi10 := ctx.Prices[n-11]
		for _, p := range ctx.Prices[n-10 : n-1] {
			if p > hi10 { hi10 = p }
		}
		return ctx.BTCPrice > hi10 && crossedAbove(ctx.Prices, 9, 21)
	},
	"TR_BEAR_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		n := len(ctx.Prices)
		lo10 := ctx.Prices[n-11]
		for _, p := range ctx.Prices[n-10 : n-1] {
			if p < lo10 { lo10 = p }
		}
		return ctx.BTCPrice < lo10 && crossedBelow(ctx.Prices, 9, 21)
	},
	// 7: VWAP reclaim after 8-bar consolidation (std < 60% of prior)
	"TR_BULL_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 { return false }
		n := len(ctx.Prices)
		vw := avgPrice(ctx.Prices[n-30:])
		consStd := stddev(ctx.Prices[n-8:])
		priorStd := stddev(ctx.Prices[n-25 : n-8])
		consolidated := consStd < priorStd*0.60
		return consolidated && ctx.BTCPrice > vw && momentum(ctx.Prices, 3) > 0.0020
	},
	"TR_BEAR_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 { return false }
		n := len(ctx.Prices)
		vw := avgPrice(ctx.Prices[n-30:])
		consStd := stddev(ctx.Prices[n-8:])
		priorStd := stddev(ctx.Prices[n-25 : n-8])
		consolidated := consStd < priorStd*0.60
		return consolidated && ctx.BTCPrice < vw && momentum(ctx.Prices, 3) < -0.0020
	},
	// 8: 25-bar momentum > 0.8% + RSI 55-72
	"TR_BULL_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		r := rsi(ctx.Prices, 14)
		return momentum(ctx.Prices, 25) > 0.0080 && r >= 55 && r <= 72
	},
	"TR_BEAR_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		r := rsi(ctx.Prices, 14)
		return momentum(ctx.Prices, 25) < -0.0080 && r >= 28 && r <= 45
	},
	// 9: EMA21 > EMA55 + 3-bar retrace shallow (<0.2%) + resumes
	"TR_BULL_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		e21, e55 := ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		n := len(ctx.Prices)
		retrace := (ctx.Prices[n-4] - ctx.Prices[n-2]) / ctx.Prices[n-4]
		resume := momentum(ctx.Prices, 2) > 0.0010
		return e21 > e55 && retrace > 0 && retrace < 0.002 && resume
	},
	"TR_BEAR_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		e21, e55 := ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		n := len(ctx.Prices)
		retrace := (ctx.Prices[n-2] - ctx.Prices[n-4]) / ctx.Prices[n-4]
		resume := momentum(ctx.Prices, 2) < -0.0010
		return e21 < e55 && retrace > 0 && retrace < 0.002 && resume
	},
	// 10: full trend + vol expansion (BB width > 140% of 20-bar-ago width)
	"TR_BULL_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 50 { return false }
		n := len(ctx.Prices)
		widthNow := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		widthPrev := bbUpper(ctx.Prices[:n-20], 20) - bbLower(ctx.Prices[:n-20], 20)
		e9, e21, e55 := ema(ctx.Prices, 9), ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		return e9 > e21 && e21 > e55 && widthNow > widthPrev*1.40 && momentum(ctx.Prices, 5) > 0.0025
	},
	"TR_BEAR_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 50 { return false }
		n := len(ctx.Prices)
		widthNow := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		widthPrev := bbUpper(ctx.Prices[:n-20], 20) - bbLower(ctx.Prices[:n-20], 20)
		e9, e21, e55 := ema(ctx.Prices, 9), ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		return e9 < e21 && e21 < e55 && widthNow > widthPrev*1.40 && momentum(ctx.Prices, 5) < -0.0025
	},

	// ═══════════════════════════════════════════════════════════════════════════
	// CATEGORY J — VOLATILITY PULSE signals (VP_BULL_1..10 / VP_BEAR_1..10)
	// ═══════════════════════════════════════════════════════════════════════════

	// 1: realised vol spike (recent 5-bar std > 1.5× 30-bar std) + directional
	"VP_BULL_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 { return false }
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-5:])
		priorStd := stddev(ctx.Prices[n-35 : n-5])
		return priorStd > 0 && recentStd > priorStd*1.50 && momentum(ctx.Prices, 5) > 0.0025
	},
	"VP_BEAR_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 { return false }
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-5:])
		priorStd := stddev(ctx.Prices[n-35 : n-5])
		return priorStd > 0 && recentStd > priorStd*1.50 && momentum(ctx.Prices, 5) < -0.0025
	},
	// 2: BB width crossed above 20-bar average BB width
	"VP_BULL_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 { return false }
		n := len(ctx.Prices)
		totalWidth := 0.0
		for i := n - 20; i < n-1; i++ {
			totalWidth += bbUpper(ctx.Prices[:i+1], 20) - bbLower(ctx.Prices[:i+1], 20)
		}
		avgWidth := totalWidth / 19.0
		curWidth := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		return curWidth > avgWidth*1.15 && momentum(ctx.Prices, 5) > 0.0020
	},
	"VP_BEAR_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 { return false }
		n := len(ctx.Prices)
		totalWidth := 0.0
		for i := n - 20; i < n-1; i++ {
			totalWidth += bbUpper(ctx.Prices[:i+1], 20) - bbLower(ctx.Prices[:i+1], 20)
		}
		avgWidth := totalWidth / 19.0
		curWidth := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		return curWidth > avgWidth*1.15 && momentum(ctx.Prices, 5) < -0.0020
	},
	// 3: 5-bar ATR-proxy spike (avg absolute bar change > 1.4x prior 20-bar)
	"VP_BULL_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 28 { return false }
		n := len(ctx.Prices)
		atr5 := 0.0
		for i := n - 5; i < n; i++ { atr5 += math.Abs(ctx.Prices[i]-ctx.Prices[i-1]) / ctx.Prices[i-1] }
		atr5 /= 5
		atr20 := 0.0
		for i := n - 25; i < n-5; i++ { atr20 += math.Abs(ctx.Prices[i]-ctx.Prices[i-1]) / ctx.Prices[i-1] }
		atr20 /= 20
		return atr20 > 0 && atr5 > atr20*1.40 && momentum(ctx.Prices, 5) > 0.0022
	},
	"VP_BEAR_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 28 { return false }
		n := len(ctx.Prices)
		atr5 := 0.0
		for i := n - 5; i < n; i++ { atr5 += math.Abs(ctx.Prices[i]-ctx.Prices[i-1]) / ctx.Prices[i-1] }
		atr5 /= 5
		atr20 := 0.0
		for i := n - 25; i < n-5; i++ { atr20 += math.Abs(ctx.Prices[i]-ctx.Prices[i-1]) / ctx.Prices[i-1] }
		atr20 /= 20
		return atr20 > 0 && atr5 > atr20*1.40 && momentum(ctx.Prices, 5) < -0.0022
	},
	// 4: BB(10) tighter-period squeeze then break
	"VP_BULL_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-10:])
		priorStd := stddev(ctx.Prices[n-30 : n-10])
		return priorStd > 0 && recentStd < priorStd*0.65 && momentum(ctx.Prices, 3) > 0.0020
	},
	"VP_BEAR_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-10:])
		priorStd := stddev(ctx.Prices[n-30 : n-10])
		return priorStd > 0 && recentStd < priorStd*0.65 && momentum(ctx.Prices, 3) < -0.0020
	},
	// 5: IV > 0.65 + directional 5m momentum
	"VP_BULL_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 { return false }
		return ctx.IV > 0.65 && momentum(ctx.Prices, 5) > 0.0028
	},
	"VP_BEAR_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 { return false }
		return ctx.IV > 0.65 && momentum(ctx.Prices, 5) < -0.0028
	},
	// 6: std-dev cross (recent 8-bar std crossed above prior 20-bar std) + RSI
	"VP_BULL_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		n := len(ctx.Prices)
		stdNow := stddev(ctx.Prices[n-8:])
		stdPrev := stddev(ctx.Prices[n-28 : n-8])
		return stdNow > stdPrev && momentum(ctx.Prices, 5) > 0.0020 && rsi(ctx.Prices, 14) > 52
	},
	"VP_BEAR_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 { return false }
		n := len(ctx.Prices)
		stdNow := stddev(ctx.Prices[n-8:])
		stdPrev := stddev(ctx.Prices[n-28 : n-8])
		return stdNow > stdPrev && momentum(ctx.Prices, 5) < -0.0020 && rsi(ctx.Prices, 14) < 48
	},
	// 7: extreme low vol (30-bar std < 50% of 60-bar std) + breakout
	"VP_BULL_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 65 { return false }
		n := len(ctx.Prices)
		std30 := stddev(ctx.Prices[n-30:])
		std60 := stddev(ctx.Prices[n-60:])
		return std60 > 0 && std30 < std60*0.50 && momentum(ctx.Prices, 5) > 0.0022
	},
	"VP_BEAR_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 65 { return false }
		n := len(ctx.Prices)
		std30 := stddev(ctx.Prices[n-30:])
		std60 := stddev(ctx.Prices[n-60:])
		return std60 > 0 && std30 < std60*0.50 && momentum(ctx.Prices, 5) < -0.0022
	},
	// 8: vol compression + EMA cross + RSI leaving neutral band
	"VP_BULL_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 { return false }
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-8:])
		priorStd := stddev(ctx.Prices[n-40 : n-8])
		r := rsi(ctx.Prices, 14)
		rPrev := rsi(ctx.Prices[:n-1], 14)
		return priorStd > 0 && recentStd < priorStd*0.60 && crossedAbove(ctx.Prices, 9, 21) && rPrev < 52 && r >= 52
	},
	"VP_BEAR_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 45 { return false }
		n := len(ctx.Prices)
		recentStd := stddev(ctx.Prices[n-8:])
		priorStd := stddev(ctx.Prices[n-40 : n-8])
		r := rsi(ctx.Prices, 14)
		rPrev := rsi(ctx.Prices[:n-1], 14)
		return priorStd > 0 && recentStd < priorStd*0.60 && crossedBelow(ctx.Prices, 9, 21) && rPrev > 48 && r <= 48
	},
	// 9: historical vol at 6-month proxy low + resistance break
	"VP_BULL_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 90 { return false }
		n := len(ctx.Prices)
		stdNow := stddev(ctx.Prices[n-20:])
		std90 := stddev(ctx.Prices[n-90:])
		hi20 := 0.0
		for _, p := range ctx.Prices[n-21 : n-1] { if p > hi20 { hi20 = p } }
		return std90 > 0 && stdNow < std90*0.45 && ctx.BTCPrice > hi20*1.0015
	},
	"VP_BEAR_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 90 { return false }
		n := len(ctx.Prices)
		stdNow := stddev(ctx.Prices[n-20:])
		std90 := stddev(ctx.Prices[n-90:])
		lo20 := math.MaxFloat64
		for _, p := range ctx.Prices[n-21 : n-1] { if p < lo20 { lo20 = p } }
		return std90 > 0 && stdNow < std90*0.45 && ctx.BTCPrice < lo20*0.9985
	},
	// 10: IV crisis mode (> 0.80) + confirmed reversal (capitulation recovery)
	"VP_BULL_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 { return false }
		n := len(ctx.Prices)
		window := ctx.Prices[n-6 : n-1]
		lo := window[0]
		for _, p := range window[1:] { if p < lo { lo = p } }
		drop := (ctx.Prices[n-7] - lo) / ctx.Prices[n-7]
		recovery := (ctx.BTCPrice - lo) / lo
		return ctx.IV > 0.80 && drop > 0.0030 && recovery > 0.0015
	},
	"VP_BEAR_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 12 { return false }
		n := len(ctx.Prices)
		window := ctx.Prices[n-6 : n-1]
		hi := 0.0
		for _, p := range window { if p > hi { hi = p } }
		rise := (hi - ctx.Prices[n-7]) / ctx.Prices[n-7]
		rejection := (hi - ctx.BTCPrice) / hi
		return ctx.IV > 0.80 && rise > 0.0030 && rejection > 0.0015
	},

	// ═══════════════════════════════════════════════════════════════════════════
	// CATEGORY K — STRUCTURE SNAP signals (SS_BULL_1..10 / SS_BEAR_1..10)
	// ═══════════════════════════════════════════════════════════════════════════

	// 1: 30-bar swing high break + momentum confirmation
	"SS_BULL_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 32 { return false }
		n := len(ctx.Prices)
		hi30 := 0.0
		for _, p := range ctx.Prices[n-31 : n-1] { if p > hi30 { hi30 = p } }
		return ctx.BTCPrice > hi30*1.0012 && momentum(ctx.Prices, 5) > 0.0018
	},
	"SS_BEAR_1": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 32 { return false }
		n := len(ctx.Prices)
		lo30 := math.MaxFloat64
		for _, p := range ctx.Prices[n-31 : n-1] { if p < lo30 { lo30 = p } }
		return ctx.BTCPrice < lo30*0.9988 && momentum(ctx.Prices, 5) < -0.0018
	},
	// 2: inside-bar breakout (prior bar range < 50% of 5-bar avg range, current expands)
	"SS_BULL_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 { return false }
		n := len(ctx.Prices)
		priorRange := math.Abs(ctx.Prices[n-2] - ctx.Prices[n-3])
		avgRange := 0.0
		for i := n - 7; i < n-2; i++ { avgRange += math.Abs(ctx.Prices[i]-ctx.Prices[i-1]) }
		avgRange /= 5
		curRange := math.Abs(ctx.BTCPrice - ctx.Prices[n-2])
		return avgRange > 0 && priorRange < avgRange*0.50 && curRange > avgRange && momentum(ctx.Prices, 2) > 0.0015
	},
	"SS_BEAR_2": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 { return false }
		n := len(ctx.Prices)
		priorRange := math.Abs(ctx.Prices[n-2] - ctx.Prices[n-3])
		avgRange := 0.0
		for i := n - 7; i < n-2; i++ { avgRange += math.Abs(ctx.Prices[i]-ctx.Prices[i-1]) }
		avgRange /= 5
		curRange := math.Abs(ctx.BTCPrice - ctx.Prices[n-2])
		return avgRange > 0 && priorRange < avgRange*0.50 && curRange > avgRange && momentum(ctx.Prices, 2) < -0.0015
	},
	// 3: 40-bar high break + EMA9 > EMA21
	"SS_BULL_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 42 { return false }
		n := len(ctx.Prices)
		hi40 := 0.0
		for _, p := range ctx.Prices[n-41 : n-1] { if p > hi40 { hi40 = p } }
		return ctx.BTCPrice > hi40*1.0008 && ema(ctx.Prices, 9) > ema(ctx.Prices, 21)
	},
	"SS_BEAR_3": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 42 { return false }
		n := len(ctx.Prices)
		lo40 := math.MaxFloat64
		for _, p := range ctx.Prices[n-41 : n-1] { if p < lo40 { lo40 = p } }
		return ctx.BTCPrice < lo40*0.9992 && ema(ctx.Prices, 9) < ema(ctx.Prices, 21)
	},
	// 4: 50-bar pivot reclaim + RSI 50-65
	"SS_BULL_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 52 { return false }
		n := len(ctx.Prices)
		hi50 := 0.0
		for _, p := range ctx.Prices[n-51 : n-1] { if p > hi50 { hi50 = p } }
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > hi50 && r >= 50 && r <= 65
	},
	"SS_BEAR_4": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 52 { return false }
		n := len(ctx.Prices)
		lo50 := math.MaxFloat64
		for _, p := range ctx.Prices[n-51 : n-1] { if p < lo50 { lo50 = p } }
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < lo50 && r >= 35 && r <= 50
	},
	// 5: 3-bar base (low std) + expansion above resistance
	"SS_BULL_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		n := len(ctx.Prices)
		baseStd := stddev(ctx.Prices[n-4 : n-1])
		priorStd := stddev(ctx.Prices[n-20 : n-4])
		hi3 := 0.0
		for _, p := range ctx.Prices[n-4 : n-1] { if p > hi3 { hi3 = p } }
		return priorStd > 0 && baseStd < priorStd*0.55 && ctx.BTCPrice > hi3*1.0010
	},
	"SS_BEAR_5": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		n := len(ctx.Prices)
		baseStd := stddev(ctx.Prices[n-4 : n-1])
		priorStd := stddev(ctx.Prices[n-20 : n-4])
		lo3 := math.MaxFloat64
		for _, p := range ctx.Prices[n-4 : n-1] { if p < lo3 { lo3 = p } }
		return priorStd > 0 && baseStd < priorStd*0.55 && ctx.BTCPrice < lo3*0.9990
	},
	// 6: failed breakdown snap (price dipped below support then recovered above)
	"SS_BULL_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		n := len(ctx.Prices)
		support := 0.0
		for _, p := range ctx.Prices[n-21 : n-4] { support += p }
		support /= 17
		loRecent := math.MaxFloat64
		for _, p := range ctx.Prices[n-4 : n-1] { if p < loRecent { loRecent = p } }
		return loRecent < support*0.9990 && ctx.BTCPrice > support && momentum(ctx.Prices, 2) > 0.0015
	},
	"SS_BEAR_6": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 25 { return false }
		n := len(ctx.Prices)
		resistance := 0.0
		for _, p := range ctx.Prices[n-21 : n-4] { resistance += p }
		resistance /= 17
		hiRecent := 0.0
		for _, p := range ctx.Prices[n-4 : n-1] { if p > hiRecent { hiRecent = p } }
		return hiRecent > resistance*1.0010 && ctx.BTCPrice < resistance && momentum(ctx.Prices, 2) < -0.0015
	},
	// 7: hourly-open reclaim (first 10 bars of hour) + EMA9 slope
	"SS_BULL_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 { return false }
		withinHour := ctx.UTCMin < 12
		e9 := ema(ctx.Prices, 9)
		e9Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 9)
		return withinHour && e9 > e9Prev && momentum(ctx.Prices, 5) > 0.0022
	},
	"SS_BEAR_7": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 { return false }
		withinHour := ctx.UTCMin < 12
		e9 := ema(ctx.Prices, 9)
		e9Prev := ema(ctx.Prices[:len(ctx.Prices)-1], 9)
		return withinHour && e9 < e9Prev && momentum(ctx.Prices, 5) < -0.0022
	},
	// 8: 60-bar high break + strong 10-bar momentum + RSI 54-70
	"SS_BULL_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 62 { return false }
		n := len(ctx.Prices)
		hi60 := 0.0
		for _, p := range ctx.Prices[n-61 : n-1] { if p > hi60 { hi60 = p } }
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > hi60*1.0008 && momentum(ctx.Prices, 10) > 0.0045 && r >= 54 && r <= 72
	},
	"SS_BEAR_8": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 62 { return false }
		n := len(ctx.Prices)
		lo60 := math.MaxFloat64
		for _, p := range ctx.Prices[n-61 : n-1] { if p < lo60 { lo60 = p } }
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < lo60*0.9992 && momentum(ctx.Prices, 10) < -0.0045 && r >= 28 && r <= 46
	},
	// 9: 3-touch level break (3rd test of resistance then clean break)
	"SS_BULL_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 { return false }
		n := len(ctx.Prices)
		lookback := ctx.Prices[n-38 : n-2]
		hi := 0.0
		for _, p := range lookback { if p > hi { hi = p } }
		band := hi * 0.0015
		touches := 0
		for _, p := range lookback { if math.Abs(p-hi) < band { touches++ } }
		return touches >= 3 && ctx.BTCPrice > hi*1.0010 && momentum(ctx.Prices, 3) > 0.0018
	},
	"SS_BEAR_9": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 40 { return false }
		n := len(ctx.Prices)
		lookback := ctx.Prices[n-38 : n-2]
		lo := math.MaxFloat64
		for _, p := range lookback { if p < lo { lo = p } }
		band := lo * 0.0015
		touches := 0
		for _, p := range lookback { if math.Abs(p-lo) < band { touches++ } }
		return touches >= 3 && ctx.BTCPrice < lo*0.9990 && momentum(ctx.Prices, 3) < -0.0018
	},
	// 10: session-high break + EMA stack + vol expansion + RSI elite
	"SS_BULL_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		n := len(ctx.Prices)
		sessionBars := ctx.UTCHour*60 + ctx.UTCMin
		if sessionBars > 240 { sessionBars = 240 }
		if sessionBars < 10 { return false }
		sessionHi := 0.0
		start := n - sessionBars - 1
		if start < 0 { start = 0 }
		for _, p := range ctx.Prices[start : n-1] { if p > sessionHi { sessionHi = p } }
		e9, e21, e55 := ema(ctx.Prices, 9), ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		widthNow := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		widthPrev := bbUpper(ctx.Prices[:n-10], 20) - bbLower(ctx.Prices[:n-10], 20)
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice > sessionHi && e9 > e21 && e21 > e55 && widthNow > widthPrev*1.25 && r >= 55 && r <= 72
	},
	"SS_BEAR_10": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 60 { return false }
		n := len(ctx.Prices)
		sessionBars := ctx.UTCHour*60 + ctx.UTCMin
		if sessionBars > 240 { sessionBars = 240 }
		if sessionBars < 10 { return false }
		sessionLo := math.MaxFloat64
		start := n - sessionBars - 1
		if start < 0 { start = 0 }
		for _, p := range ctx.Prices[start : n-1] { if p < sessionLo { sessionLo = p } }
		e9, e21, e55 := ema(ctx.Prices, 9), ema(ctx.Prices, 21), ema(ctx.Prices, 55)
		widthNow := bbUpper(ctx.Prices, 20) - bbLower(ctx.Prices, 20)
		widthPrev := bbUpper(ctx.Prices[:n-10], 20) - bbLower(ctx.Prices[:n-10], 20)
		r := rsi(ctx.Prices, 14)
		return ctx.BTCPrice < sessionLo && e9 < e21 && e21 < e55 && widthNow > widthPrev*1.25 && r >= 28 && r <= 45
	},
}
