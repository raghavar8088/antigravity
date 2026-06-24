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

// ── Coppock Curve (S81) ──────────────────────────────────────────────────────

// CoppockCurve computes Edwin Coppock's momentum oscillator (Barron's, 1962):
// a weighted moving average of the sum of two rate-of-change values.
// WMA(ROC(roc1) + ROC(roc2), wmaLen). Returns 0 if insufficient data.
func CoppockCurve(candles []Candle, roc1, roc2, wmaLen int) float64 {
	needed := roc1 + wmaLen + 1
	if roc1 < roc2 {
		needed = roc2 + wmaLen + 1
	}
	if len(candles) < needed {
		return 0
	}
	rocSum := make([]float64, wmaLen)
	for i := 0; i < wmaLen; i++ {
		offset := len(candles) - wmaLen + i
		if offset < roc1 || offset < roc2 {
			return 0
		}
		slice1 := candles[:offset+1]
		slice2 := candles[:offset+1]
		r1 := 0.0
		if len(slice1) > roc1 && slice1[len(slice1)-roc1-1].Close != 0 {
			r1 = (slice1[len(slice1)-1].Close/slice1[len(slice1)-roc1-1].Close - 1) * 100
		}
		r2 := 0.0
		if len(slice2) > roc2 && slice2[len(slice2)-roc2-1].Close != 0 {
			r2 = (slice2[len(slice2)-1].Close/slice2[len(slice2)-roc2-1].Close - 1) * 100
		}
		rocSum[i] = r1 + r2
	}
	// Weighted moving average: weight = position index + 1
	var weightedSum, weightSum float64
	for i, v := range rocSum {
		w := float64(i + 1)
		weightedSum += v * w
		weightSum += w
	}
	if weightSum == 0 {
		return 0
	}
	return weightedSum / weightSum
}

// ── Chande Momentum Oscillator (S82) ────────────────────────────────────────

// CMO computes Tushar Chande's Momentum Oscillator ("The New Technical Trader", 1994).
// CMO = 100 * (sumUp - sumDown) / (sumUp + sumDown) over `period` bars. Range -100..+100.
func CMO(candles []Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	tail := candles[len(candles)-(period+1):]
	var sumUp, sumDown float64
	for i := 1; i < len(tail); i++ {
		diff := tail[i].Close - tail[i-1].Close
		if diff > 0 {
			sumUp += diff
		} else {
			sumDown += -diff
		}
	}
	denom := sumUp + sumDown
	if denom == 0 {
		return 0
	}
	return 100 * (sumUp - sumDown) / denom
}

// ── KST — Know Sure Thing (S83) ─────────────────────────────────────────────

// KSTResult holds the KST oscillator value and its signal line.
type KSTResult struct {
	KST    float64
	Signal float64
}

// rocN returns the simple rate-of-change for `period` bars as a percentage.
func rocN(candles []Candle, period int) float64 {
	if len(candles) < period+1 {
		return 0
	}
	base := candles[len(candles)-period-1].Close
	if base == 0 {
		return 0
	}
	return (candles[len(candles)-1].Close/base - 1) * 100
}

// KST computes Martin Pring's Know Sure Thing indicator ("Market Momentum", 1993).
// Uses standard Pring parameters: ROC(10,10), ROC(15,10), ROC(20,10), ROC(30,15)
// with weights 1, 2, 3, 4. Signal = EMA(KST, 9).
func KST(candles []Candle) KSTResult {
	// Need enough candles: largest ROC period (30) + largest EMA smoothing (15) + signal EMA (9) + buffer
	if len(candles) < 60 {
		return KSTResult{}
	}
	// Build KST series for last 9+1 bars (enough for signal EMA)
	kstVals := make([]float64, 0, 12)
	for i := 9; i >= 0; i-- {
		offset := len(candles) - i
		if offset < 45 {
			continue
		}
		sub := candles[:offset]
		// Each RCMA = EMA(ROC(n), smoothPeriod)
		// We approximate by computing ROC at the current point then smoothing
		// Simplified: compute KST as weighted sum of 4 ROC-EMA composites using trailing slices
		rcma1 := emaOfFloats(rocSlice(sub, 10, 10), 10)
		rcma2 := emaOfFloats(rocSlice(sub, 15, 10), 10)
		rcma3 := emaOfFloats(rocSlice(sub, 20, 10), 10)
		rcma4 := emaOfFloats(rocSlice(sub, 30, 15), 15)
		kstVals = append(kstVals, rcma1*1+rcma2*2+rcma3*3+rcma4*4)
	}
	if len(kstVals) < 2 {
		return KSTResult{}
	}
	current := kstVals[len(kstVals)-1]
	signal := emaOfFloats(kstVals, 9)
	return KSTResult{KST: current, Signal: signal}
}

// rocSlice builds a slice of ROC(period) values for the last `count` positions.
func rocSlice(candles []Candle, period, count int) []float64 {
	if len(candles) < period+count {
		return nil
	}
	out := make([]float64, count)
	for i := 0; i < count; i++ {
		offset := len(candles) - count + i
		sub := candles[:offset+1]
		out[i] = rocN(sub, period)
	}
	return out
}

// ── Aroon Indicator (S84) ───────────────────────────────────────────────────

// AroonResult holds Aroon Up and Aroon Down values (range 0–100).
type AroonResult struct {
	Up   float64
	Down float64
}

// Aroon computes Tushar Chande's Aroon indicator (Stocks & Commodities, 1995).
// AroonUp = (period - bars since period-bar high) / period * 100.
// AroonDown = (period - bars since period-bar low) / period * 100.
func Aroon(candles []Candle, period int) AroonResult {
	if len(candles) < period+1 {
		return AroonResult{}
	}
	tail := candles[len(candles)-(period+1):]
	highIdx, lowIdx := 0, 0
	for i := 1; i < len(tail); i++ {
		if tail[i].High >= tail[highIdx].High {
			highIdx = i
		}
		if tail[i].Low <= tail[lowIdx].Low {
			lowIdx = i
		}
	}
	barsSinceHigh := len(tail) - 1 - highIdx
	barsSinceLow := len(tail) - 1 - lowIdx
	up := float64(period-barsSinceHigh) / float64(period) * 100
	down := float64(period-barsSinceLow) / float64(period) * 100
	return AroonResult{Up: up, Down: down}
}

// ── Narrow Range N (S85) ────────────────────────────────────────────────────

// NarrowRangeN returns true if the last candle's range (high-low) is the
// narrowest of the last n candles. Crabel "Day Trading with Short Term Price
// Patterns" (1990).
func NarrowRangeN(candles []Candle, n int) bool {
	if len(candles) < n {
		return false
	}
	tail := candles[len(candles)-n:]
	last := tail[len(tail)-1]
	lastRange := last.High - last.Low
	for _, c := range tail[:len(tail)-1] {
		if c.High-c.Low <= lastRange {
			return false
		}
	}
	return true
}

// ── Squeeze Momentum Detector (S88) ─────────────────────────────────────────

// SqueezeResult reports whether a Bollinger Band / Keltner Channel squeeze is
// active or just fired, plus the current momentum direction.
type SqueezeResult struct {
	Active   bool    // BB is inside Keltner (squeeze on)
	Fired    bool    // BB just expanded outside Keltner (squeeze off, expansion begins)
	Momentum float64 // positive = bullish, negative = bearish
}

// SqueezeDetector implements John Carter's Squeeze Momentum indicator
// ("Mastering the Trade", 2006). BB(20,2) inside Keltner(EMA20, 1.5×ATR14)
// = squeeze on. Momentum = close - midpoint of (highest high + lowest low / 2 + SMA20) / 2.
func SqueezeDetector(candles []Candle) SqueezeResult {
	if len(candles) < 22 {
		return SqueezeResult{}
	}
	bb := BB(candles, 20)
	atr14 := ATR(candles, 14)
	ema20 := EMA(candles, 20)
	if atr14 == 0 || ema20 == 0 {
		return SqueezeResult{}
	}
	kUpper := ema20 + 1.5*atr14
	kLower := ema20 - 1.5*atr14

	activeNow := bb.Upper < kUpper && bb.Lower > kLower

	// Check if squeeze was active on prior bar
	var activePrev bool
	if len(candles) >= 23 {
		bbPrev := BB(candles[:len(candles)-1], 20)
		atrPrev := ATR(candles[:len(candles)-1], 14)
		emaPrev := EMA(candles[:len(candles)-1], 20)
		if atrPrev > 0 && emaPrev > 0 {
			kUpPrev := emaPrev + 1.5*atrPrev
			kLoPrev := emaPrev - 1.5*atrPrev
			activePrev = bbPrev.Upper < kUpPrev && bbPrev.Lower > kLoPrev
		}
	}

	fired := activePrev && !activeNow

	// Momentum = close - midpoint of the combined channel
	tail20 := candles[len(candles)-20:]
	hi20, lo20 := tail20[0].High, tail20[0].Low
	for _, c := range tail20 {
		if c.High > hi20 {
			hi20 = c.High
		}
		if c.Low < lo20 {
			lo20 = c.Low
		}
	}
	midpoint := (hi20+lo20)/2 + bb.Middle
	momentum := candles[len(candles)-1].Close - midpoint/2

	return SqueezeResult{Active: activeNow, Fired: fired, Momentum: momentum}
}

// ── Fibonacci Retracement Levels (S91) ───────────────────────────────────────

// FibRetracement computes Fibonacci retracement price levels between high and
// low for the given ratios (default 0.382, 0.5, 0.618 if none provided).
// Carney "Harmonic Trading" (2010).
func FibRetracement(high, low float64, levels ...float64) []float64 {
	if len(levels) == 0 {
		levels = []float64{0.382, 0.500, 0.618}
	}
	rang := high - low
	out := make([]float64, len(levels))
	for i, l := range levels {
		out[i] = high - l*rang
	}
	return out
}

// ── Volume Profile POC (S95) ─────────────────────────────────────────────────

// VolumeProfilePOC computes a simplified Point of Control (highest-volume price
// level) from the provided candles. Steidlmayer "Markets and Market Logic" (1986).
// Price levels are rounded to the nearest $100. Returns 0 if no volume data.
func VolumeProfilePOC(candles []Candle) float64 {
	if len(candles) == 0 {
		return 0
	}
	bucket := make(map[int]float64)
	for _, c := range candles {
		// Distribute volume evenly across the candle's price range
		hi := int(math.Round(c.High/100)) * 100
		lo := int(math.Round(c.Low/100)) * 100
		if hi == lo {
			bucket[hi] += c.Volume
			continue
		}
		steps := (hi - lo) / 100
		if steps <= 0 {
			steps = 1
		}
		volPerLevel := c.Volume / float64(steps)
		for p := lo; p <= hi; p += 100 {
			bucket[p] += volPerLevel
		}
	}
	var pocPrice int
	var maxVol float64
	for price, vol := range bucket {
		if vol > maxVol {
			maxVol = vol
			pocPrice = price
		}
	}
	return float64(pocPrice)
}

// ── On Balance Volume (S96) ──────────────────────────────────────────────────

// OBV computes Joseph Granville's On Balance Volume ("New Key to Stock Market
// Profits", 1963). Cumulative sum: add volume on up days, subtract on down days.
func OBV(candles []Candle) float64 {
	if len(candles) < 2 {
		return 0
	}
	var obv float64
	for i := 1; i < len(candles); i++ {
		if candles[i].Close > candles[i-1].Close {
			obv += candles[i].Volume
		} else if candles[i].Close < candles[i-1].Close {
			obv -= candles[i].Volume
		}
	}
	return obv
}

// ── Chaikin Money Flow (S97) ─────────────────────────────────────────────────

// ChaikinMoneyFlow computes Marc Chaikin's Money Flow indicator.
// CMF = sum(MFV, period) / sum(volume, period)
// MFV = ((close - low) - (high - close)) / (high - low) * volume
// Returns 0 if insufficient data or zero volume.
func ChaikinMoneyFlow(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	var mfvSum, volSum float64
	for _, c := range tail {
		hl := c.High - c.Low
		if hl == 0 {
			continue
		}
		mfv := ((c.Close - c.Low) - (c.High - c.Close)) / hl * c.Volume
		mfvSum += mfv
		volSum += c.Volume
	}
	if volSum == 0 {
		return 0
	}
	return mfvSum / volSum
}
