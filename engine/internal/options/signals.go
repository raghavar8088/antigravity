package options

import "math"

// SignalContext holds market data for signal evaluation.
// Prices contains 1-minute bar closes (not raw ticks).
type SignalContext struct {
	Prices   []float64 // 1-minute sampled price bars
	IV       float64
	BTCPrice float64
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

func vwapOf(prices []float64) float64 {
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
		return mom5 > 0.0025 && mom10 > 0.001 && rsiVal < 65
	},
	"BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.0025 && mom10 < -0.001 && rsiVal > 35
	},
	"STRONG_BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 > 0.005 && mom10 > 0.003 && rsiVal < 70
	},
	"STRONG_BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 15 {
			return false
		}
		mom5 := momentum(ctx.Prices, 5)
		mom10 := momentum(ctx.Prices, 10)
		rsiVal := rsi(ctx.Prices, 14)
		return mom5 < -0.005 && mom10 < -0.003 && rsiVal > 30
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
		return prevR < 30 && r >= 30 && r < 40
	},
	"RSI_OVERBOUGHT": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR > 70 && r <= 70 && r > 60
	},
	"RSI_OVERSOLD_EXTREME": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR < 20 && r >= 20
	},
	"RSI_OVERBOUGHT_EXTREME": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		r := rsi(ctx.Prices, 14)
		prevR := rsi(ctx.Prices[:len(ctx.Prices)-1], 14)
		return prevR > 80 && r <= 80
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
		if len(ctx.Prices) < 35 {
			return false
		}
		recentStd := stddev(ctx.Prices[len(ctx.Prices)-10:])
		priorStd := stddev(ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-10])
		squeezed := recentStd < priorStd*0.6
		breakout := momentum(ctx.Prices, 3) > 0.003
		return squeezed && breakout
	},
	"BB_SQUEEZE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 35 {
			return false
		}
		recentStd := stddev(ctx.Prices[len(ctx.Prices)-10:])
		priorStd := stddev(ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-10])
		squeezed := recentStd < priorStd*0.6
		breakout := momentum(ctx.Prices, 3) < -0.003
		return squeezed && breakout
	},

	// ── VWAP signals (require meaningful deviation) ───────────────────────────
	// Price must be significantly above/below VWAP AND trending in that direction
	"VWAP_ABOVE": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := vwapOf(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (ctx.BTCPrice - vw) / vw
		// Must be 0.3% above VWAP AND have upward momentum
		return deviation > 0.003 && momentum(ctx.Prices, 5) > 0.002
	},
	"VWAP_BELOW": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		vw := vwapOf(ctx.Prices[len(ctx.Prices)-30:])
		deviation := (vw - ctx.BTCPrice) / vw
		return deviation > 0.003 && momentum(ctx.Prices, 5) < -0.002
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
		return ctx.BTCPrice > hi*1.003 && momentum(ctx.Prices, 3) > 0.002
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
		return ctx.BTCPrice < lo*0.997 && momentum(ctx.Prices, 3) < -0.002
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
		return prevK < 20 && k >= 20 && rsiVal < 50
	},
	"STOCH_OVERBOUGHT": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		k := stochK(ctx.Prices, 14)
		prevK := stochK(ctx.Prices[:len(ctx.Prices)-1], 14)
		rsiVal := rsi(ctx.Prices, 14)
		return prevK > 80 && k <= 80 && rsiVal > 50
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
}
