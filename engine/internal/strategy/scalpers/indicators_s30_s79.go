package scalpers

import "math"

// This file adds the indicator primitives needed by the S30-S79 shadow
// strategy families. Each function follows the existing indicators.go
// convention: return a neutral/zero value on insufficient data rather than
// panicking, so a strategy can simply check for the neutral value and bail
// out to NoSignal.

// ── Kalman Filter (S32) ──────────────────────────────────────────────────────
// 1-D constant-velocity-free Kalman filter on closing price. Reference:
// Kalman, "A New Approach to Linear Filtering and Prediction Problems" (1960).
// processVar/measVar control how aggressively the filter trusts new prices
// vs. its own smoothed estimate; defaults below are tuned for 15m BTC closes.
func KalmanFilter(candles []Candle) float64 {
	if len(candles) < 5 {
		return 0
	}
	const processVar = 1e-5
	const measVar = 1e-2
	estimate := candles[0].Close
	errCov := 1.0
	for i := 1; i < len(candles); i++ {
		// Predict
		errCov += processVar
		// Update
		kalmanGain := errCov / (errCov + measVar)
		estimate += kalmanGain * (candles[i].Close - estimate)
		errCov *= (1 - kalmanGain)
	}
	return estimate
}

// ── Kaufman Adaptive Moving Average (S33) ────────────────────────────────────
// Kaufman, "Smarter Trading" (1995). Efficiency ratio = net change / sum of
// absolute changes over the period; KAMA speeds up (toward fast EMA) when ER
// is high (trending) and slows down (toward slow EMA) when ER is low (choppy).
// Returns (kamaValue, efficiencyRatio). kamaValue is 0 if insufficient data.
func KAMA(candles []Candle, period int) (float64, float64) {
	if len(candles) < period+2 {
		return 0, 0
	}
	tail := candles[len(candles)-(period+1):]
	change := math.Abs(tail[len(tail)-1].Close - tail[0].Close)
	var volatility float64
	for i := 1; i < len(tail); i++ {
		volatility += math.Abs(tail[i].Close - tail[i-1].Close)
	}
	if volatility == 0 {
		return tail[len(tail)-1].Close, 0
	}
	er := change / volatility
	const fastSC = 2.0 / (2.0 + 1.0)
	const slowSC = 2.0 / (30.0 + 1.0)
	sc := math.Pow(er*(fastSC-slowSC)+slowSC, 2)

	kama := tail[0].Close
	for i := 1; i < len(tail); i++ {
		kama = kama + sc*(tail[i].Close-kama)
	}
	return kama, er
}

// ── SuperTrend (S36) ──────────────────────────────────────────────────────────
// Seban's SuperTrend: ATR-based dynamic band that flips when price closes
// through it. Returns (supertrendValue, isUptrend).
func SuperTrend(candles []Candle, period int, multiplier float64) (float64, bool) {
	if len(candles) < period+2 {
		return 0, true
	}
	tail := candles[len(candles)-(period+10):]
	if len(tail) < period+2 {
		tail = candles
	}
	uptrend := true
	var st float64
	for i := period; i < len(tail); i++ {
		window := tail[:i+1]
		atr := ATR(window, period)
		hl2 := (tail[i].High + tail[i].Low) / 2
		upperBand := hl2 + multiplier*atr
		lowerBand := hl2 - multiplier*atr

		if i == period {
			if tail[i].Close <= upperBand {
				st = upperBand
				uptrend = false
			} else {
				st = lowerBand
				uptrend = true
			}
			continue
		}

		if uptrend {
			if lowerBand > st {
				st = lowerBand
			}
			if tail[i].Close < st {
				uptrend = false
				st = upperBand
			}
		} else {
			if upperBand < st {
				st = upperBand
			}
			if tail[i].Close > st {
				uptrend = true
				st = lowerBand
			}
		}
	}
	return st, uptrend
}

// ── Ichimoku Kinko Hyo (S37) ──────────────────────────────────────────────────
// Tenkan (9), Kijun (26), Senkou Span A/B (cloud), Chikou (lagging, just the
// current close for our purposes since we don't shift series in this engine).
type IchimokuResult struct {
	Tenkan, Kijun, SpanA, SpanB, Chikou float64
}

func midpoint(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	hi, lo := tail[0].High, tail[0].Low
	for _, c := range tail {
		if c.High > hi {
			hi = c.High
		}
		if c.Low < lo {
			lo = c.Low
		}
	}
	return (hi + lo) / 2
}

func Ichimoku(candles []Candle) IchimokuResult {
	if len(candles) < 52 {
		return IchimokuResult{}
	}
	tenkan := midpoint(candles, 9)
	kijun := midpoint(candles, 26)
	spanA := (tenkan + kijun) / 2
	spanB := midpoint(candles, 52)
	return IchimokuResult{
		Tenkan: tenkan, Kijun: kijun, SpanA: spanA, SpanB: spanB,
		Chikou: candles[len(candles)-1].Close,
	}
}

// ── Donchian Channel (S38) ────────────────────────────────────────────────────
// Donchian breakout — foundation of the Turtle Trading system.
func DonchianChannel(candles []Candle, period int) (high, low float64) {
	if len(candles) < period {
		return 0, 0
	}
	tail := candles[len(candles)-period:]
	high, low = tail[0].High, tail[0].Low
	for _, c := range tail {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	return high, low
}

// ── Parabolic SAR (S39) ───────────────────────────────────────────────────────
// Wilder, "New Concepts in Technical Trading Systems" (1978). Standard
// acceleration factor 0.02, max 0.20. Returns (sarValue, isUptrend).
func ParabolicSAR(candles []Candle) (float64, bool) {
	if len(candles) < 5 {
		return 0, true
	}
	const afStart = 0.02
	const afMax = 0.20
	const afStep = 0.02

	uptrend := candles[1].Close > candles[0].Close
	af := afStart
	var sar float64
	var ep float64 // extreme point
	if uptrend {
		sar = candles[0].Low
		ep = candles[0].High
	} else {
		sar = candles[0].High
		ep = candles[0].Low
	}

	for i := 1; i < len(candles); i++ {
		sar = sar + af*(ep-sar)
		c := candles[i]
		if uptrend {
			if c.Low < sar {
				uptrend = false
				sar = ep
				ep = c.Low
				af = afStart
			} else {
				if c.High > ep {
					ep = c.High
					af = math.Min(af+afStep, afMax)
				}
			}
		} else {
			if c.High > sar {
				uptrend = true
				sar = ep
				ep = c.High
				af = afStart
			} else {
				if c.Low < ep {
					ep = c.Low
					af = math.Min(af+afStep, afMax)
				}
			}
		}
	}
	return sar, uptrend
}

// ── Ornstein-Uhlenbeck parameter estimation (S40) ────────────────────────────
// Vasicek (1977); Avellaneda & Lee (2010) "Statistical Arbitrage in the US
// Equities Market." Fits dX = theta*(mu - X)*dt + sigma*dW via discrete-time
// AR(1) regression on deviations from rolling VWAP. Returns (theta, mu, sigma,
// halfLifeBars). theta <= 0 means no detectable mean reversion.
type OUParams struct {
	Theta, Mu, Sigma, HalfLifeBars float64
}

func OUParameters(candles []Candle, period int) OUParams {
	if len(candles) < period+2 {
		return OUParams{}
	}
	tail := candles[len(candles)-period:]
	vwap := VWAP(tail)
	// Deviation series X_t = price_t - vwap
	x := make([]float64, len(tail))
	for i, c := range tail {
		x[i] = c.Close - vwap
	}
	// AR(1): x_t = a + b*x_{t-1} + e ; theta = -ln(b), mu implied ~ 0 (vwap-centered)
	var sumX, sumY, sumXY, sumXX float64
	n := float64(len(x) - 1)
	for i := 1; i < len(x); i++ {
		sumX += x[i-1]
		sumY += x[i]
		sumXY += x[i-1] * x[i]
		sumXX += x[i-1] * x[i-1]
	}
	if n == 0 || sumXX*n-sumX*sumX == 0 {
		return OUParams{}
	}
	b := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	a := (sumY - b*sumX) / n
	if b <= 0 || b >= 1 {
		return OUParams{} // no mean reversion detected
	}
	theta := -math.Log(b)
	mu := a / (1 - b)
	var resSumSq float64
	for i := 1; i < len(x); i++ {
		pred := a + b*x[i-1]
		resSumSq += (x[i] - pred) * (x[i] - pred)
	}
	sigma := math.Sqrt(resSumSq / n)
	halfLife := math.Log(2) / theta
	return OUParams{Theta: theta, Mu: mu, Sigma: sigma, HalfLifeBars: halfLife}
}

// ── Slow Stochastic Oscillator (S43) ─────────────────────────────────────────
// Lane (1984). %K = 100*(close - lowN)/(highN - lowN), smoothed; %D = SMA(%K,3).
func SlowStochastic(candles []Candle, kPeriod, dPeriod int) (k, d float64) {
	if len(candles) < kPeriod+dPeriod {
		return 50, 50
	}
	kVals := make([]float64, 0, dPeriod)
	for i := len(candles) - dPeriod; i < len(candles); i++ {
		window := candles[i-kPeriod+1 : i+1]
		hi, lo := window[0].High, window[0].Low
		for _, c := range window {
			if c.High > hi {
				hi = c.High
			}
			if c.Low < lo {
				lo = c.Low
			}
		}
		if hi == lo {
			kVals = append(kVals, 50)
			continue
		}
		kVals = append(kVals, 100*(candles[i].Close-lo)/(hi-lo))
	}
	k = kVals[len(kVals)-1]
	var sum float64
	for _, v := range kVals {
		sum += v
	}
	d = sum / float64(len(kVals))
	return k, d
}

// ── Commodity Channel Index (S45) ─────────────────────────────────────────────
// Lambert (1980). CCI = (typicalPrice - SMA(typicalPrice)) / (0.015 * meanDeviation).
func CCI(candles []Candle, period int) float64 {
	if len(candles) < period {
		return 0
	}
	tail := candles[len(candles)-period:]
	tp := make([]float64, len(tail))
	var sum float64
	for i, c := range tail {
		tp[i] = (c.High + c.Low + c.Close) / 3
		sum += tp[i]
	}
	mean := sum / float64(len(tp))
	var meanDev float64
	for _, v := range tp {
		meanDev += math.Abs(v - mean)
	}
	meanDev /= float64(len(tp))
	if meanDev == 0 {
		return 0
	}
	return (tp[len(tp)-1] - mean) / (0.015 * meanDev)
}

// ── Weighted Order Book Imbalance (S54) ──────────────────────────────────────
// Cartea, Jaimungal & Penalva (2015) "Algorithmic and High-Frequency Trading."
// Weights closer-to-mid depth tiers more heavily than far tiers. Range -1..1.
func WeightedOBImbalance(ob OrderBookSnapshot) float64 {
	const w01, w05, w1 = 0.6, 0.3, 0.1
	bidW := w01*ob.BidDepth01pct + w05*ob.BidDepth05pct + w1*ob.BidDepth1pct
	askW := w01*ob.AskDepth01pct + w05*ob.AskDepth05pct + w1*ob.AskDepth1pct
	if bidW+askW == 0 {
		return 0
	}
	return (bidW - askW) / (bidW + askW)
}

// ── Linear Regression Channel (S47) ──────────────────────────────────────────
// Least-squares fit of close price over `period` bars +/- standard error
// bands. Murphy, "Technical Analysis of the Financial Markets" (1999).
type LinRegChannel struct {
	Slope, Intercept, StdErr, ValueAtEnd float64
}

func LinearRegressionChannel(candles []Candle, period int) LinRegChannel {
	if len(candles) < period {
		return LinRegChannel{}
	}
	tail := candles[len(candles)-period:]
	n := float64(len(tail))
	var sumX, sumY, sumXY, sumXX float64
	for i, c := range tail {
		x := float64(i)
		sumX += x
		sumY += c.Close
		sumXY += x * c.Close
		sumXX += x * x
	}
	denom := n*sumXX - sumX*sumX
	if denom == 0 {
		return LinRegChannel{}
	}
	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n
	var sumSqErr float64
	for i, c := range tail {
		pred := intercept + slope*float64(i)
		sumSqErr += (c.Close - pred) * (c.Close - pred)
	}
	stdErr := math.Sqrt(sumSqErr / n)
	valueAtEnd := intercept + slope*(n-1)
	return LinRegChannel{Slope: slope, Intercept: intercept, StdErr: stdErr, ValueAtEnd: valueAtEnd}
}

// ── Classic Floor Pivot Points (S48) ─────────────────────────────────────────
// Standard institutional floor-trader formula from the prior period's OHLC.
type PivotLevels struct {
	PP, R1, R2, S1, S2 float64
}

func PivotPoints(prevPeriod Candle) PivotLevels {
	pp := (prevPeriod.High + prevPeriod.Low + prevPeriod.Close) / 3
	r1 := 2*pp - prevPeriod.Low
	s1 := 2*pp - prevPeriod.High
	r2 := pp + (prevPeriod.High - prevPeriod.Low)
	s2 := pp - (prevPeriod.High - prevPeriod.Low)
	return PivotLevels{PP: pp, R1: r1, R2: r2, S1: s1, S2: s2}
}

// ── Autocorrelation (S63) ─────────────────────────────────────────────────────
// Biais et al. (2016) "Equilibrium Fast Trading." Pearson autocorrelation of
// 1-bar returns at the given lag, computed over the trailing window.
func Autocorrelation(candles []Candle, lag, window int) float64 {
	if len(candles) < window+lag+1 {
		return 0
	}
	tail := candles[len(candles)-(window+lag):]
	rets := make([]float64, len(tail)-1)
	for i := 1; i < len(tail); i++ {
		if tail[i-1].Close == 0 {
			rets[i-1] = 0
			continue
		}
		rets[i-1] = (tail[i].Close - tail[i-1].Close) / tail[i-1].Close
	}
	if len(rets) < lag+2 {
		return 0
	}
	a := rets[:len(rets)-lag]
	b := rets[lag:]
	n := float64(len(a))
	var meanA, meanB float64
	for i := range a {
		meanA += a[i]
		meanB += b[i]
	}
	meanA /= n
	meanB /= n
	var cov, varA, varB float64
	for i := range a {
		da, db := a[i]-meanA, b[i]-meanB
		cov += da * db
		varA += da * da
		varB += db * db
	}
	if varA == 0 || varB == 0 {
		return 0
	}
	return cov / math.Sqrt(varA*varB)
}

// ── Shannon Entropy of return distribution (S64) ─────────────────────────────
// Lo & MacKinlay, "A Non-Random Walk Down Wall Street" (1999). Discretizes
// trailing returns into `bins` buckets and computes normalized entropy (0..1,
// 1 = maximally disordered/uniform).
func ShannonEntropy(candles []Candle, period, bins int) float64 {
	if len(candles) < period+1 {
		return 1 // treat unknown as max entropy (avoid trading)
	}
	tail := candles[len(candles)-(period+1):]
	rets := make([]float64, len(tail)-1)
	minR, maxR := math.MaxFloat64, -math.MaxFloat64
	for i := 1; i < len(tail); i++ {
		if tail[i-1].Close == 0 {
			continue
		}
		r := (tail[i].Close - tail[i-1].Close) / tail[i-1].Close
		rets[i-1] = r
		if r < minR {
			minR = r
		}
		if r > maxR {
			maxR = r
		}
	}
	if maxR == minR {
		return 0
	}
	counts := make([]int, bins)
	width := (maxR - minR) / float64(bins)
	for _, r := range rets {
		idx := int((r - minR) / width)
		if idx >= bins {
			idx = bins - 1
		}
		if idx < 0 {
			idx = 0
		}
		counts[idx]++
	}
	var entropy float64
	n := float64(len(rets))
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		entropy -= p * math.Log2(p)
	}
	maxEntropy := math.Log2(float64(bins))
	if maxEntropy == 0 {
		return 0
	}
	return entropy / maxEntropy
}

// ── Haar Wavelet trend decomposition (S66) ───────────────────────────────────
// Gallegati & Semmler, "Wavelet Applications in Economics and Finance" (2014).
// Simplified 3-level Haar decomposition: returns the level-3 (lowest
// frequency) approximation coefficient sequence's final value as the
// long-horizon trend estimate, and the level-1 detail (high-frequency noise)
// magnitude for confidence weighting.
func HaarWavelet(candles []Candle, levels int) (trendEstimate, noiseLevel float64) {
	n := len(candles)
	size := 1
	for size*2 <= n {
		size *= 2
	}
	if size < 8 {
		return 0, 0
	}
	data := make([]float64, size)
	for i := 0; i < size; i++ {
		data[i] = candles[n-size+i].Close
	}
	approx := data
	var firstDetail []float64
	for l := 0; l < levels && len(approx) >= 2; l++ {
		half := len(approx) / 2
		newApprox := make([]float64, half)
		detail := make([]float64, half)
		for i := 0; i < half; i++ {
			a, b := approx[2*i], approx[2*i+1]
			newApprox[i] = (a + b) / math.Sqrt2
			detail[i] = (a - b) / math.Sqrt2
		}
		if l == 0 {
			firstDetail = detail
		}
		approx = newApprox
	}
	trendEstimate = approx[len(approx)-1] / math.Pow(math.Sqrt2, float64(levels))
	var noiseSum float64
	for _, d := range firstDetail {
		noiseSum += math.Abs(d)
	}
	if len(firstDetail) > 0 {
		noiseLevel = noiseSum / float64(len(firstDetail))
	}
	return trendEstimate, noiseLevel
}

// ── CUSUM structural break detection (S69) ───────────────────────────────────
// Page (1954) "Continuous Inspection Schemes"; Lopez de Prado (2018) financial
// applications. Tracks cumulative positive/negative deviation from the
// trailing mean return; a break is signalled when either cumulative sum
// exceeds `threshold` standard deviations. Returns (breakDetected, direction
// where +1 = upward break, -1 = downward break, 0 = none).
func CUSUM(candles []Candle, period int, threshold float64) (bool, int) {
	if len(candles) < period+1 {
		return false, 0
	}
	tail := candles[len(candles)-(period+1):]
	rets := make([]float64, len(tail)-1)
	var mean float64
	for i := 1; i < len(tail); i++ {
		if tail[i-1].Close == 0 {
			continue
		}
		rets[i-1] = (tail[i].Close - tail[i-1].Close) / tail[i-1].Close
		mean += rets[i-1]
	}
	mean /= float64(len(rets))
	var variance float64
	for _, r := range rets {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(rets))
	sd := math.Sqrt(variance)
	if sd == 0 {
		return false, 0
	}
	var posCusum, negCusum float64
	for _, r := range rets {
		dev := r - mean
		posCusum = math.Max(0, posCusum+dev)
		negCusum = math.Min(0, negCusum+dev)
	}
	thresholdAbs := threshold * sd
	if posCusum > thresholdAbs {
		return true, 1
	}
	if -negCusum > thresholdAbs {
		return true, -1
	}
	return false, 0
}
