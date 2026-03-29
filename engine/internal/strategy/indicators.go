package strategy

import "math"

// =============================================================================
// SHARED TECHNICAL INDICATOR LIBRARY
// All scalping strategies draw from this common math toolkit.
// =============================================================================

// EMA calculates the Exponential Moving Average for the latest value.
func EMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return SMA(prices)
	}
	k := 2.0 / float64(period+1)
	ema := SMA(prices[:period])
	for i := period; i < len(prices); i++ {
		ema = prices[i]*k + ema*(1-k)
	}
	return ema
}

// SMA calculates Simple Moving Average over the entire slice.
func SMA(prices []float64) float64 {
	if len(prices) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range prices {
		sum += p
	}
	return sum / float64(len(prices))
}

// StdDev calculates the standard deviation over the entire slice.
func StdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := SMA(values)
	variance := 0.0
	for _, value := range values {
		variance += (value - mean) * (value - mean)
	}
	return math.Sqrt(variance / float64(len(values)))
}

// RollingVWAP calculates a rolling volume-weighted average price.
func RollingVWAP(prices, volumes []float64, period int) float64 {
	if len(prices) == 0 || len(volumes) == 0 {
		return 0
	}

	start := 0
	if period > 0 && len(prices) > period {
		start = len(prices) - period
	}

	priceWindow := prices[start:]
	volumeWindow := volumes
	if len(volumeWindow) > len(priceWindow) {
		volumeWindow = volumeWindow[len(volumeWindow)-len(priceWindow):]
	}
	if len(priceWindow) > len(volumeWindow) {
		priceWindow = priceWindow[len(priceWindow)-len(volumeWindow):]
	}

	numerator := 0.0
	denominator := 0.0
	for i := range priceWindow {
		numerator += priceWindow[i] * volumeWindow[i]
		denominator += volumeWindow[i]
	}
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

// RSI calculates the Relative Strength Index.
func RSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50
	}
	gains, losses := 0.0, 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += math.Abs(change)
		}
	}
	if losses == 0 {
		return 100
	}
	rs := (gains / float64(period)) / (losses / float64(period))
	return 100 - (100 / (1 + rs))
}

// BollingerBands returns (upper, middle, lower) bands.
func BollingerBands(prices []float64, period int, stdDevMultiplier float64) (float64, float64, float64) {
	if len(prices) < period {
		mid := SMA(prices)
		return mid, mid, mid
	}
	slice := prices[len(prices)-period:]
	mid := SMA(slice)
	variance := 0.0
	for _, p := range slice {
		variance += (p - mid) * (p - mid)
	}
	stdDev := math.Sqrt(variance / float64(period))
	return mid + stdDevMultiplier*stdDev, mid, mid - stdDevMultiplier*stdDev
}

// ATR calculates Average True Range from high, low, close arrays.
// For tick-based strategies, we approximate using price volatility.
func ATR(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 0
	}
	sum := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		sum += math.Abs(prices[i] - prices[i-1])
	}
	return sum / float64(period)
}

// MACD returns (macdLine, signalLine, histogram).
func MACD(prices []float64, fastPeriod, slowPeriod, signalPeriod int) (float64, float64, float64) {
	if len(prices) < slowPeriod {
		return 0, 0, 0
	}
	fastEMA := EMA(prices, fastPeriod)
	slowEMA := EMA(prices, slowPeriod)
	macdLine := fastEMA - slowEMA

	// Build MACD history for signal line
	macdHistory := make([]float64, 0)
	for i := slowPeriod; i <= len(prices); i++ {
		f := EMA(prices[:i], fastPeriod)
		s := EMA(prices[:i], slowPeriod)
		macdHistory = append(macdHistory, f-s)
	}
	signalLine := EMA(macdHistory, signalPeriod)
	histogram := macdLine - signalLine
	return macdLine, signalLine, histogram
}

// StochasticRSI returns the stochastic oscillator applied to RSI values.
func StochasticRSI(prices []float64, rsiPeriod, stochPeriod int) float64 {
	if len(prices) < rsiPeriod+stochPeriod {
		return 50
	}
	rsiValues := make([]float64, 0)
	for i := rsiPeriod + 1; i <= len(prices); i++ {
		rsiValues = append(rsiValues, RSI(prices[:i], rsiPeriod))
	}
	if len(rsiValues) < stochPeriod {
		return 50
	}
	recent := rsiValues[len(rsiValues)-stochPeriod:]
	minRSI, maxRSI := recent[0], recent[0]
	for _, v := range recent {
		if v < minRSI {
			minRSI = v
		}
		if v > maxRSI {
			maxRSI = v
		}
	}
	if maxRSI == minRSI {
		return 50
	}
	currentRSI := rsiValues[len(rsiValues)-1]
	return ((currentRSI - minRSI) / (maxRSI - minRSI)) * 100
}

// CloseStochastic calculates a close-based stochastic oscillator using the
// highest and lowest closes in the lookback window.
func CloseStochastic(prices []float64, period int) float64 {
	if len(prices) < period {
		return 50
	}
	slice := prices[len(prices)-period:]
	highest, lowest := slice[0], slice[0]
	for _, price := range slice {
		if price > highest {
			highest = price
		}
		if price < lowest {
			lowest = price
		}
	}
	if highest == lowest {
		return 50
	}
	return ((prices[len(prices)-1] - lowest) / (highest - lowest)) * 100
}

// WilliamsR calculates Williams %R oscillator.
func WilliamsR(prices []float64, period int) float64 {
	if len(prices) < period {
		return -50
	}
	slice := prices[len(prices)-period:]
	highest, lowest := slice[0], slice[0]
	for _, p := range slice {
		if p > highest {
			highest = p
		}
		if p < lowest {
			lowest = p
		}
	}
	if highest == lowest {
		return -50
	}
	return ((highest - prices[len(prices)-1]) / (highest - lowest)) * -100
}

// CCI calculates the Commodity Channel Index.
func CCI(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}
	slice := prices[len(prices)-period:]
	mean := SMA(slice)
	meanDev := 0.0
	for _, p := range slice {
		meanDev += math.Abs(p - mean)
	}
	meanDev /= float64(period)
	if meanDev == 0 {
		return 0
	}
	return (prices[len(prices)-1] - mean) / (0.015 * meanDev)
}

// DonchianChannel returns (upper, lower) over the lookback period.
func DonchianChannel(prices []float64, period int) (float64, float64) {
	if len(prices) < period {
		return prices[len(prices)-1], prices[len(prices)-1]
	}
	slice := prices[len(prices)-period:]
	high, low := slice[0], slice[0]
	for _, p := range slice {
		if p > high {
			high = p
		}
		if p < low {
			low = p
		}
	}
	return high, low
}

// HullMA calculates the Hull Moving Average for smoother signals.
func HullMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return SMA(prices)
	}
	halfPeriod := period / 2
	sqrtPeriod := int(math.Sqrt(float64(period)))

	wma1 := EMA(prices, halfPeriod)
	wma2 := EMA(prices, period)

	diff := 2*wma1 - wma2
	// Simplified: use the diff as the hull value
	_ = sqrtPeriod
	return diff
}

// ROC calculates Rate of Change percentage.
func ROC(prices []float64, period int) float64 {
	if len(prices) <= period {
		return 0
	}
	prev := prices[len(prices)-1-period]
	if prev == 0 {
		return 0
	}
	return ((prices[len(prices)-1] - prev) / prev) * 100
}

// ParabolicSAR returns a simplified Parabolic SAR value.
func ParabolicSAR(prices []float64, af, maxAF float64) float64 {
	if len(prices) < 3 {
		return prices[len(prices)-1]
	}
	sar := prices[0]
	ep := prices[0]
	currentAF := af
	isLong := prices[1] > prices[0]

	for i := 1; i < len(prices); i++ {
		sar = sar + currentAF*(ep-sar)
		if isLong {
			if prices[i] > ep {
				ep = prices[i]
				currentAF = math.Min(currentAF+af, maxAF)
			}
			if prices[i] < sar {
				isLong = false
				sar = ep
				ep = prices[i]
				currentAF = af
			}
		} else {
			if prices[i] < ep {
				ep = prices[i]
				currentAF = math.Min(currentAF+af, maxAF)
			}
			if prices[i] > sar {
				isLong = true
				sar = ep
				ep = prices[i]
				currentAF = af
			}
		}
	}
	return sar
}

// ADX calculates the Average Directional Index (simplified).
func ADX(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 25
	}
	plusDM, minusDM := 0.0, 0.0
	tr := 0.0
	for i := len(prices) - period; i < len(prices); i++ {
		up := prices[i] - prices[i-1]
		down := prices[i-1] - prices[i]
		if up > down && up > 0 {
			plusDM += up
		}
		if down > up && down > 0 {
			minusDM += down
		}
		tr += math.Abs(prices[i] - prices[i-1])
	}
	if tr == 0 {
		return 25
	}
	plusDI := (plusDM / tr) * 100
	minusDI := (minusDM / tr) * 100
	if plusDI+minusDI == 0 {
		return 25
	}
	dx := math.Abs(plusDI-minusDI) / (plusDI + minusDI) * 100
	return dx
}

// KeltnerChannels returns (upper, middle, lower) using EMA and ATR.
func KeltnerChannels(prices []float64, emaPeriod, atrPeriod int, multiplier float64) (float64, float64, float64) {
	mid := EMA(prices, emaPeriod)
	atr := ATR(prices, atrPeriod)
	return mid + multiplier*atr, mid, mid - multiplier*atr
}

// PivotPoints calculates classic floor pivot from a price window.
func PivotPoints(prices []float64, period int) (float64, float64, float64) {
	if len(prices) < period {
		p := prices[len(prices)-1]
		return p, p, p
	}
	slice := prices[len(prices)-period:]
	high, low := slice[0], slice[0]
	for _, p := range slice {
		if p > high {
			high = p
		}
		if p < low {
			low = p
		}
	}
	close := prices[len(prices)-1]
	pivot := (high + low + close) / 3
	r1 := 2*pivot - low
	s1 := 2*pivot - high
	return pivot, r1, s1
}
