package alpha

import (
	"math"
	"sort"
)


func SMA(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func StdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := SMA(values)
	sum := 0.0
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

func ZScore(value float64, values []float64) float64 {
	std := StdDev(values)
	if std == 0 {
		return 0
	}
	return (value - SMA(values)) / std
}

func PercentileRank(value float64, values []float64) float64 {
	if len(values) == 0 {
		return 50
	}
	lessOrEqual := 0
	for _, v := range values {
		if v <= value {
			lessOrEqual++
		}
	}
	return float64(lessOrEqual) / float64(len(values)) * 100
}

func Quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	idx := int(math.Round(Clamp(q, 0, 1) * float64(len(cp)-1)))
	return cp[idx]
}

func RSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50
	}
	gain, loss := 0.0, 0.0
	for i := len(closes) - period; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gain += change
		} else {
			loss += -change
		}
	}
	if loss == 0 {
		return 100
	}
	rs := gain / loss
	return 100 - 100/(1+rs)
}

func Highest(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	high := values[0]
	for _, v := range values[1:] {
		if v > high {
			high = v
		}
	}
	return high
}

func Lowest(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	low := values[0]
	for _, v := range values[1:] {
		if v < low {
			low = v
		}
	}
	return low
}

func Tail(values []float64, n int) []float64 {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

func TailCandles(values []Candle, n int) []Candle {
	if n <= 0 || len(values) <= n {
		return values
	}
	return values[len(values)-n:]
}

// ATR returns the Average True Range over `period` candles.
// True Range = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
func ATR(candles []Candle, period int) float64 {
	if len(candles) < 2 {
		return 0
	}
	if period <= 0 {
		period = 14
	}
	start := len(candles) - period - 1
	if start < 1 {
		start = 1
	}
	sum := 0.0
	count := 0
	for i := start; i < len(candles); i++ {
		prev := candles[i-1].Close
		tr := math.Max(candles[i].High-candles[i].Low,
			math.Max(math.Abs(candles[i].High-prev), math.Abs(candles[i].Low-prev)))
		sum += tr
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

// ADX returns Wilder's Average Directional Index over `period` candles (simplified).
// Returns 0–100; > 25 = trending, < 20 = ranging.
func ADX(candles []Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	if period <= 0 {
		period = 14
	}
	start := len(candles) - period - 1
	if start < 1 {
		start = 1
	}
	var plusDM, minusDM, trSum float64
	for i := start; i < len(candles); i++ {
		prev := candles[i-1]
		curr := candles[i]
		upMove := curr.High - prev.High
		downMove := prev.Low - curr.Low
		tr := math.Max(curr.High-curr.Low,
			math.Max(math.Abs(curr.High-prev.Close), math.Abs(curr.Low-prev.Close)))
		trSum += tr
		if upMove > downMove && upMove > 0 {
			plusDM += upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM += downMove
		}
	}
	if trSum == 0 {
		return 0
	}
	plusDI := 100 * plusDM / trSum
	minusDI := 100 * minusDM / trSum
	if plusDI+minusDI == 0 {
		return 0
	}
	return 100 * math.Abs(plusDI-minusDI) / (plusDI + minusDI)
}
