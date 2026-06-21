package scalpers

import "math"

// ── EMA ──────────────────────────────────────────────────────────────────────

// EMA computes exponential moving average over the last n closes.
// Returns 0 if not enough data.
func EMA(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	k := 2.0 / float64(period+1)
	ema := candles[0].Close
	for i := 1; i < len(candles); i++ {
		ema = candles[i].Close*k + ema*(1-k)
	}
	return ema
}

// EMASlice returns EMA values for every candle position (same length as input).
func EMASlice(candles []Candle, period int) []float64 {
	out := make([]float64, len(candles))
	if len(candles) < period {
		return out
	}
	k := 2.0 / float64(period+1)
	out[0] = candles[0].Close
	for i := 1; i < len(candles); i++ {
		out[i] = candles[i].Close*k + out[i-1]*(1-k)
	}
	return out
}

// ── RSI ──────────────────────────────────────────────────────────────────────

// RSI computes the 14-period RSI from the last n+1 closes.
func RSI(candles []Candle, period int) float64 {
	if len(candles) < period+1 {
		return 50
	}
	tail := candles[len(candles)-(period+1):]
	var gains, losses float64
	for i := 1; i < len(tail); i++ {
		diff := tail[i].Close - tail[i-1].Close
		if diff > 0 {
			gains += diff
		} else {
			losses += -diff
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 {
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// ── ATR ──────────────────────────────────────────────────────────────────────

// ATR computes average true range over the last n candles.
func ATR(candles []Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	tail := candles[len(candles)-(period+1):]
	var sum float64
	for i := 1; i < len(tail); i++ {
		hl := tail[i].High - tail[i].Low
		hpc := math.Abs(tail[i].High - tail[i-1].Close)
		lpc := math.Abs(tail[i].Low - tail[i-1].Close)
		tr := math.Max(hl, math.Max(hpc, lpc))
		sum += tr
	}
	return sum / float64(period)
}

// ── Bollinger Bands ───────────────────────────────────────────────────────────

type BollingerBands struct {
	Upper  float64
	Middle float64
	Lower  float64
	Width  float64 // (Upper-Lower)/Middle — normalised width
}

// BB computes Bollinger Bands (period, 2σ) from the last n closes.
func BB(candles []Candle, period int) BollingerBands {
	if len(candles) < period {
		return BollingerBands{}
	}
	tail := candles[len(candles)-period:]
	var sum float64
	for _, c := range tail {
		sum += c.Close
	}
	mean := sum / float64(period)
	var variance float64
	for _, c := range tail {
		d := c.Close - mean
		variance += d * d
	}
	sd := math.Sqrt(variance / float64(period))
	upper := mean + 2*sd
	lower := mean - 2*sd
	width := 0.0
	if mean != 0 {
		width = (upper - lower) / mean
	}
	return BollingerBands{Upper: upper, Middle: mean, Lower: lower, Width: width}
}

// BBWidthPercentile returns what percentile the current BB width is relative
// to the last `lookback` bars. 0 = narrowest seen, 1 = widest seen.
func BBWidthPercentile(candles []Candle, period, lookback int) float64 {
	if len(candles) < period+lookback {
		return 0.5
	}
	current := BB(candles, period).Width
	widths := make([]float64, lookback)
	for i := 0; i < lookback; i++ {
		offset := len(candles) - lookback + i
		widths[i] = BB(candles[:offset], period).Width
	}
	below := 0
	for _, w := range widths {
		if w <= current {
			below++
		}
	}
	return float64(below) / float64(lookback)
}

// ── MACD ─────────────────────────────────────────────────────────────────────

type MACDResult struct {
	MACD      float64
	Signal    float64
	Histogram float64
}

// emaOfFloats computes EMA over a float64 slice using standard smoothing factor.
// Returns 0 if fewer values than period.
func emaOfFloats(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}
	k := 2.0 / float64(period+1)
	ema := values[0]
	for i := 1; i < len(values); i++ {
		ema = values[i]*k + ema*(1-k)
	}
	return ema
}

// MACD computes standard 12/26/9 MACD using a proper float64 MACD line history
// rather than the fake-Candle approach, giving a more accurate signal line.
func MACD(candles []Candle) MACDResult {
	if len(candles) < 35 {
		return MACDResult{}
	}

	// Build MACD line (fast EMA - slow EMA) for all candle positions starting at bar 26.
	// We need at least 26 bars for the slow EMA to be valid, plus 9 more for signal EMA.
	n := len(candles)
	macdValues := make([]float64, n-25) // index 0 = candles[25], index n-26 = candles[n-1]
	for i := 26; i <= n; i++ {
		f := EMA(candles[:i], 12)
		s := EMA(candles[:i], 26)
		macdValues[i-26] = f - s
	}

	macdVal := macdValues[len(macdValues)-1]

	// Signal line: EMA-9 of the MACD values (starting after first 26 bars gives enough history)
	signal := emaOfFloats(macdValues, 9)

	return MACDResult{
		MACD:      macdVal,
		Signal:    signal,
		Histogram: macdVal - signal,
	}
}

// ── ADX ──────────────────────────────────────────────────────────────────────

// ADX computes the Average Directional Index (period typically 14).
func ADX(candles []Candle, period int) float64 {
	if len(candles) < period*2+1 {
		return 0
	}
	tail := candles[len(candles)-(period*2+1):]

	dmPlus := make([]float64, len(tail)-1)
	dmMinus := make([]float64, len(tail)-1)
	trs := make([]float64, len(tail)-1)

	for i := 1; i < len(tail); i++ {
		upMove := tail[i].High - tail[i-1].High
		downMove := tail[i-1].Low - tail[i].Low
		if upMove > downMove && upMove > 0 {
			dmPlus[i-1] = upMove
		}
		if downMove > upMove && downMove > 0 {
			dmMinus[i-1] = downMove
		}
		hl := tail[i].High - tail[i].Low
		hpc := math.Abs(tail[i].High - tail[i-1].Close)
		lpc := math.Abs(tail[i].Low - tail[i-1].Close)
		trs[i-1] = math.Max(hl, math.Max(hpc, lpc))
	}

	smoothTR := smoothed(trs, period)
	smoothDMPlus := smoothed(dmPlus, period)
	smoothDMMinus := smoothed(dmMinus, period)

	if smoothTR == 0 {
		return 0
	}
	diPlus := 100 * smoothDMPlus / smoothTR
	diMinus := 100 * smoothDMMinus / smoothTR
	diSum := diPlus + diMinus
	if diSum == 0 {
		return 0
	}
	dx := 100 * math.Abs(diPlus-diMinus) / diSum
	return dx
}

func smoothed(vals []float64, period int) float64 {
	if len(vals) < period {
		return 0
	}
	tail := vals[len(vals)-period:]
	sum := 0.0
	for _, v := range tail {
		sum += v
	}
	return sum / float64(period)
}

// ── VWAP ─────────────────────────────────────────────────────────────────────

// VWAP computes volume-weighted average price over the provided candles
// (typically the current session's candles).
func VWAP(candles []Candle) float64 {
	var sumPV, sumV float64
	for _, c := range candles {
		typical := (c.High + c.Low + c.Close) / 3.0
		sumPV += typical * c.Volume
		sumV += c.Volume
	}
	if sumV == 0 {
		return 0
	}
	return sumPV / sumV
}

// ── Volume helpers ────────────────────────────────────────────────────────────

// AvgVolume returns the mean volume over the last n candles.
func AvgVolume(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	var sum float64
	for _, c := range tail {
		sum += c.Volume
	}
	return sum / float64(period)
}

// ── Swing high/low ────────────────────────────────────────────────────────────

// SwingHigh returns the highest high over the last n candles.
func SwingHigh(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	high := tail[0].High
	for _, c := range tail[1:] {
		if c.High > high {
			high = c.High
		}
	}
	return high
}

// SwingLow returns the lowest low over the last n candles.
func SwingLow(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	low := tail[0].Low
	for _, c := range tail[1:] {
		if c.Low < low {
			low = c.Low
		}
	}
	return low
}

// ── Realized Volatility ──────────────────────────────────────────────────────

// RealizedVol computes annualized realized volatility (stdev of log returns)
// over the last `period` candles. Intended for 1h candles with period=24
// (1 day lookback) as a substitute/cross-check for Deribit DVOL (implied vol).
// Annualization assumes 1h bars: sqrt(24*365) periods/year.
func RealizedVol(candles []Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	tail := candles[len(candles)-(period+1):]
	logReturns := make([]float64, 0, period)
	for i := 1; i < len(tail); i++ {
		if tail[i-1].Close <= 0 || tail[i].Close <= 0 {
			continue
		}
		logReturns = append(logReturns, math.Log(tail[i].Close/tail[i-1].Close))
	}
	if len(logReturns) < 2 {
		return 0
	}
	var mean float64
	for _, r := range logReturns {
		mean += r
	}
	mean /= float64(len(logReturns))
	var variance float64
	for _, r := range logReturns {
		d := r - mean
		variance += d * d
	}
	variance /= float64(len(logReturns) - 1)
	stdev := math.Sqrt(variance)
	// Annualize assuming hourly bars: sqrt(24 * 365) periods/year.
	annualized := stdev * math.Sqrt(24*365) * 100 // expressed as a percentage, comparable to DVOL
	return annualized
}

// ── CVD divergence ────────────────────────────────────────────────────────────

// CVDDivergesBearish returns true when price makes a new high but CVD doesn't confirm.
func CVDDivergesBearish(priceHigh, prevPriceHigh, cvd, prevCVD float64) bool {
	return priceHigh > prevPriceHigh && cvd < prevCVD
}

// CVDDivergesBullish returns true when price makes a new low but CVD holds higher.
func CVDDivergesBullish(priceLow, prevPriceLow, cvd, prevCVD float64) bool {
	return priceLow < prevPriceLow && cvd > prevCVD
}

// ── Hurst Exponent (R/S analysis) ───────────────────────────────────────────

// HurstExponent estimates the Hurst exponent via standard rescaled-range (R/S)
// analysis: Mandelbrot/Wallis (1969) classic method, still the standard
// textbook approach for regime classification in quant trading (trending
// H>0.5, mean-reverting H<0.5, random walk H≈0.5). This implementation splits
// the input window into sub-series of varying length n, computes the mean
// R/S statistic at each n, then fits log(R/S) vs log(n) via simple linear
// regression (OLS slope = H). Kept deliberately simple (not the more
// elaborate detrended/Lo-corrected variants) — sufficient for a fast regime
// proxy, not academic-grade estimation.
//
// Returns 0.5 (neutral/random-walk) if there isn't enough data to form at
// least 2 distinct sub-period lengths.
func HurstExponent(candles []Candle, period int) float64 {
	if period > len(candles) {
		period = len(candles)
	}
	if period < 16 {
		return 0.5 // insufficient data for any meaningful R/S split
	}
	tail := candles[len(candles)-period:]

	// Log returns are the standard input series for R/S analysis (stabilizes
	// variance vs raw price levels).
	returns := make([]float64, 0, period-1)
	for i := 1; i < len(tail); i++ {
		if tail[i-1].Close <= 0 || tail[i].Close <= 0 {
			continue
		}
		returns = append(returns, math.Log(tail[i].Close/tail[i-1].Close))
	}
	if len(returns) < 16 {
		return 0.5
	}

	// Sub-period lengths: halve down from the full series length, stop below 8.
	var lengths []int
	for n := len(returns); n >= 8; n /= 2 {
		lengths = append(lengths, n)
	}
	if len(lengths) < 2 {
		return 0.5
	}

	var logN, logRS []float64
	for _, n := range lengths {
		numSegments := len(returns) / n
		if numSegments < 1 {
			continue
		}
		var rsSum float64
		var rsCount int
		for seg := 0; seg < numSegments; seg++ {
			chunk := returns[seg*n : (seg+1)*n]
			mean := 0.0
			for _, r := range chunk {
				mean += r
			}
			mean /= float64(n)

			// Cumulative deviation from mean -> range.
			cum := 0.0
			maxC, minC := 0.0, 0.0
			var variance float64
			for i, r := range chunk {
				d := r - mean
				cum += d
				if i == 0 || cum > maxC {
					maxC = cum
				}
				if i == 0 || cum < minC {
					minC = cum
				}
				variance += d * d
			}
			variance /= float64(n)
			stdev := math.Sqrt(variance)
			if stdev == 0 {
				continue
			}
			r := maxC - minC
			rsSum += r / stdev
			rsCount++
		}
		if rsCount == 0 {
			continue
		}
		avgRS := rsSum / float64(rsCount)
		if avgRS <= 0 {
			continue
		}
		logN = append(logN, math.Log(float64(n)))
		logRS = append(logRS, math.Log(avgRS))
	}

	if len(logN) < 2 {
		return 0.5
	}

	// OLS slope of log(R/S) vs log(n) = Hurst exponent estimate.
	var sumX, sumY, sumXY, sumXX float64
	k := float64(len(logN))
	for i := range logN {
		sumX += logN[i]
		sumY += logRS[i]
		sumXY += logN[i] * logRS[i]
		sumXX += logN[i] * logN[i]
	}
	denom := k*sumXX - sumX*sumX
	if denom == 0 {
		return 0.5
	}
	h := (k*sumXY - sumX*sumY) / denom

	// Clamp to sane bounds — pure R/S can occasionally produce noisy values
	// outside [0,1] on short/noisy windows.
	if h < 0 {
		h = 0
	}
	if h > 1 {
		h = 1
	}
	return h
}

// ── Multi-timeframe Z-Score ─────────────────────────────────────────────────

// smaOf returns the simple moving average of the last `period` closes.
// Returns 0 if not enough data.
func smaOf(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	var sum float64
	for _, c := range tail {
		sum += c.Close
	}
	return sum / float64(period)
}

// stdevOf returns the population stdev of the last `period` closes around
// the given mean. Returns 0 if not enough data.
func stdevOf(candles []Candle, period int, mean float64) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	var variance float64
	for _, c := range tail {
		d := c.Close - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(period))
}

// ZScoreMultiTimeframe computes the standard (close - SMA) / stdev z-score
// independently on 5m, 15m, and 1h candles using a 20-period lookback on
// each timeframe — a well-established multi-timeframe confluence technique:
// requiring statistical extremes to align simultaneously across timeframes
// is a standard high-conviction mean-reversion filter (reduces false
// positives from single-timeframe noise). Returns 0 for any timeframe with
// insufficient data.
func ZScoreMultiTimeframe(candles5m, candles15m, candles1h []Candle) (z5m, z15m, z1h float64) {
	const lookback = 20

	compute := func(candles []Candle) float64 {
		mean := smaOf(candles, lookback)
		if mean == 0 {
			return 0
		}
		sd := stdevOf(candles, lookback, mean)
		if sd == 0 {
			return 0
		}
		current := candles[len(candles)-1].Close
		return (current - mean) / sd
	}

	z5m = compute(candles5m)
	z15m = compute(candles15m)
	z1h = compute(candles1h)
	return
}
