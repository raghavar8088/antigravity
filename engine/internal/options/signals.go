package options

import "math"

// SignalContext holds the market data needed to evaluate a signal
type SignalContext struct {
	Prices   []float64 // Recent price history, newest last
	IV       float64
	BTCPrice float64
}

// SignalFunc returns true when the entry condition is met
type SignalFunc func(ctx SignalContext) bool

// ── Indicator helpers ──────────────────────────────────────────────────────

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

func avgOf(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range prices {
		sum += p
	}
	return sum / float64(len(prices))
}

// ── Signal functions ────────────────────────────────────────────────────────

var Signals = map[string]SignalFunc{
	// Momentum
	"BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		recent := avgOf(ctx.Prices[len(ctx.Prices)-5:])
		prev := avgOf(ctx.Prices[len(ctx.Prices)-10 : len(ctx.Prices)-5])
		return recent > prev*1.0015
	},
	"BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		recent := avgOf(ctx.Prices[len(ctx.Prices)-5:])
		prev := avgOf(ctx.Prices[len(ctx.Prices)-10 : len(ctx.Prices)-5])
		return recent < prev*0.9985
	},
	"STRONG_BULL_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		recent := avgOf(ctx.Prices[len(ctx.Prices)-5:])
		prev := avgOf(ctx.Prices[len(ctx.Prices)-10 : len(ctx.Prices)-5])
		return recent > prev*1.003
	},
	"STRONG_BEAR_MOMENTUM": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		recent := avgOf(ctx.Prices[len(ctx.Prices)-5:])
		prev := avgOf(ctx.Prices[len(ctx.Prices)-10 : len(ctx.Prices)-5])
		return recent < prev*0.997
	},

	// RSI
	"RSI_OVERSOLD": func(ctx SignalContext) bool {
		return rsi(ctx.Prices, 14) < 30
	},
	"RSI_OVERBOUGHT": func(ctx SignalContext) bool {
		return rsi(ctx.Prices, 14) > 70
	},
	"RSI_OVERSOLD_EXTREME": func(ctx SignalContext) bool {
		return rsi(ctx.Prices, 14) < 20
	},
	"RSI_OVERBOUGHT_EXTREME": func(ctx SignalContext) bool {
		return rsi(ctx.Prices, 14) > 80
	},

	// EMA Cross
	"EMA_BULL_CROSS": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 26 {
			return false
		}
		fast := ema(ctx.Prices, 9)
		slow := ema(ctx.Prices, 21)
		pFast := ema(ctx.Prices[:len(ctx.Prices)-1], 9)
		pSlow := ema(ctx.Prices[:len(ctx.Prices)-1], 21)
		return fast > slow && pFast <= pSlow
	},
	"EMA_BEAR_CROSS": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 26 {
			return false
		}
		fast := ema(ctx.Prices, 9)
		slow := ema(ctx.Prices, 21)
		pFast := ema(ctx.Prices[:len(ctx.Prices)-1], 9)
		pSlow := ema(ctx.Prices[:len(ctx.Prices)-1], 21)
		return fast < slow && pFast >= pSlow
	},
	"EMA_ABOVE_BOTH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 50 {
			return false
		}
		return ctx.BTCPrice > ema(ctx.Prices, 20) && ctx.BTCPrice > ema(ctx.Prices, 50)
	},
	"EMA_BELOW_BOTH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 50 {
			return false
		}
		return ctx.BTCPrice < ema(ctx.Prices, 20) && ctx.BTCPrice < ema(ctx.Prices, 50)
	},

	// Bollinger Bands
	"BB_LOWER_TOUCH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		return ctx.BTCPrice <= bbLower(ctx.Prices, 20)
	},
	"BB_UPPER_TOUCH": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		return ctx.BTCPrice >= bbUpper(ctx.Prices, 20)
	},
	"BB_SQUEEZE_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		narrow := stddev(ctx.Prices[len(ctx.Prices)-10:]) < stddev(ctx.Prices[len(ctx.Prices)-30:])*0.5
		bull := ctx.BTCPrice > ema(ctx.Prices, 20)
		return narrow && bull
	},
	"BB_SQUEEZE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		narrow := stddev(ctx.Prices[len(ctx.Prices)-10:]) < stddev(ctx.Prices[len(ctx.Prices)-30:])*0.5
		bear := ctx.BTCPrice < ema(ctx.Prices, 20)
		return narrow && bear
	},

	// VWAP
	"VWAP_BELOW": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		vw := vwapOf(ctx.Prices[len(ctx.Prices)-20:])
		return ctx.BTCPrice < vw*0.9985
	},
	"VWAP_ABOVE": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		vw := vwapOf(ctx.Prices[len(ctx.Prices)-20:])
		return ctx.BTCPrice > vw*1.0015
	},

	// Breakouts
	"RESISTANCE_BREAK": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		prev := ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-1]
		hi := 0.0
		for _, p := range prev {
			if p > hi {
				hi = p
			}
		}
		return ctx.BTCPrice > hi*1.002
	},
	"SUPPORT_BREAK": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 30 {
			return false
		}
		prev := ctx.Prices[len(ctx.Prices)-30 : len(ctx.Prices)-1]
		lo := math.MaxFloat64
		for _, p := range prev {
			if p < lo {
				lo = p
			}
		}
		return ctx.BTCPrice < lo*0.998
	},

	// Stochastic
	"STOCH_OVERSOLD": func(ctx SignalContext) bool {
		return stochK(ctx.Prices, 14) < 20
	},
	"STOCH_OVERBOUGHT": func(ctx SignalContext) bool {
		return stochK(ctx.Prices, 14) > 80
	},

	// Confluence (multi-factor)
	"TRIPLE_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		rsiOk := rsi(ctx.Prices, 14) < 45
		emaOk := ctx.BTCPrice > ema(ctx.Prices, 20)
		momOk := avgOf(ctx.Prices[len(ctx.Prices)-3:]) > avgOf(ctx.Prices[len(ctx.Prices)-8:len(ctx.Prices)-3])
		return rsiOk && emaOk && momOk
	},
	"TRIPLE_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 20 {
			return false
		}
		rsiOk := rsi(ctx.Prices, 14) > 55
		emaOk := ctx.BTCPrice < ema(ctx.Prices, 20)
		momOk := avgOf(ctx.Prices[len(ctx.Prices)-3:]) < avgOf(ctx.Prices[len(ctx.Prices)-8:len(ctx.Prices)-3])
		return rsiOk && emaOk && momOk
	},

	// IV-based
	"HIGH_IV_BULL": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		return ctx.IV > 0.80 && avgOf(ctx.Prices[len(ctx.Prices)-5:]) > avgOf(ctx.Prices[len(ctx.Prices)-10:len(ctx.Prices)-5])
	},
	"HIGH_IV_BEAR": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 10 {
			return false
		}
		return ctx.IV > 0.80 && avgOf(ctx.Prices[len(ctx.Prices)-5:]) < avgOf(ctx.Prices[len(ctx.Prices)-10:len(ctx.Prices)-5])
	},

	// Price action
	"SHARP_REVERSAL_UP": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 5 {
			return false
		}
		// Big drop followed by recovery
		lo := ctx.Prices[len(ctx.Prices)-3]
		entry := ctx.BTCPrice
		prev := ctx.Prices[len(ctx.Prices)-5]
		return lo < prev*0.998 && entry > lo*1.001
	},
	"SHARP_REVERSAL_DOWN": func(ctx SignalContext) bool {
		if len(ctx.Prices) < 5 {
			return false
		}
		hi := ctx.Prices[len(ctx.Prices)-3]
		entry := ctx.BTCPrice
		prev := ctx.Prices[len(ctx.Prices)-5]
		return hi > prev*1.002 && entry < hi*0.999
	},
}
