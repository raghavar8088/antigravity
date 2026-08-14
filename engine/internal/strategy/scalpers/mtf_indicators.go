package scalpers

import "math"

// Indicator primitives for the multi-timeframe packs.
//
// Every function here returns (value, ok) rather than a bare float. A zero from
// an indicator with too little history is indistinguishable from a genuine
// zero reading, and this desk has already lost money to numbers that meant
// "not calculated" being read as measurements.

// mtfSMA is the simple moving average of the last n closes.
func mtfSMA(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n {
		return 0, false
	}
	s := 0.0
	for _, x := range c[len(c)-n:] {
		s += x.Close
	}
	return s / float64(n), true
}

// mtfEMA is the exponential moving average, seeded with an SMA.
//
// Seeded rather than started from the first close: starting from a single price
// leaves the average chasing it for roughly n bars, so an EMA read early in the
// series reports the recent price back to you and calls it a trend.
func mtfEMA(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n*2 {
		return 0, false
	}
	seed, ok := mtfSMA(c[:n], n)
	if !ok {
		return 0, false
	}
	k := 2.0 / float64(n+1)
	ema := seed
	for _, x := range c[n:] {
		ema = x.Close*k + ema*(1-k)
	}
	return ema, true
}

// mtfRSI is Wilder's RSI over n periods.
func mtfRSI(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n+1 {
		return 0, false
	}
	gain, loss := 0.0, 0.0
	for i := len(c) - n; i < len(c); i++ {
		d := c[i].Close - c[i-1].Close
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	if loss == 0 {
		// All gains. 100 is the correct RSI, not a divide-by-zero artefact.
		return 100, true
	}
	rs := (gain / float64(n)) / (loss / float64(n))
	return 100 - 100/(1+rs), true
}

// mtfATR is the average true range over n periods, as a FRACTION of price.
//
// Fractional rather than absolute so a threshold means the same thing on a
// $0.01 coin and a $60,000 one. Absolute ATR was how position sizes ended up
// spanning 744x across this roster.
func mtfATR(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n+1 {
		return 0, false
	}
	sum := 0.0
	for i := len(c) - n; i < len(c); i++ {
		prev := c[i-1].Close
		tr := math.Max(c[i].High-c[i].Low,
			math.Max(math.Abs(c[i].High-prev), math.Abs(c[i].Low-prev)))
		sum += tr
	}
	last := c[len(c)-1].Close
	if last <= 0 {
		return 0, false
	}
	return (sum / float64(n)) / last, true
}

// mtfADX measures trend STRENGTH without direction, over n periods.
//
// Used as a regime filter: mean-reversion entries in a strong trend and
// breakout entries in a dead range are the two ways a sound signal is applied
// to the wrong market.
func mtfADX(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n*2+1 {
		return 0, false
	}
	var dxs []float64
	for i := len(c) - n; i < len(c); i++ {
		up := c[i].High - c[i-1].High
		dn := c[i-1].Low - c[i].Low
		plusDM, minusDM := 0.0, 0.0
		if up > dn && up > 0 {
			plusDM = up
		}
		if dn > up && dn > 0 {
			minusDM = dn
		}
		prev := c[i-1].Close
		tr := math.Max(c[i].High-c[i].Low,
			math.Max(math.Abs(c[i].High-prev), math.Abs(c[i].Low-prev)))
		if tr <= 0 {
			continue
		}
		pdi, mdi := plusDM/tr*100, minusDM/tr*100
		if pdi+mdi > 0 {
			dxs = append(dxs, math.Abs(pdi-mdi)/(pdi+mdi)*100)
		}
	}
	if len(dxs) == 0 {
		return 0, false
	}
	s := 0.0
	for _, d := range dxs {
		s += d
	}
	return s / float64(len(dxs)), true
}

// mtfBollinger returns the upper and lower band and the middle SMA.
func mtfBollinger(c []Candle, n int, mult float64) (upper, mid, lower float64, ok bool) {
	mid, ok = mtfSMA(c, n)
	if !ok {
		return 0, 0, 0, false
	}
	v := 0.0
	for _, x := range c[len(c)-n:] {
		d := x.Close - mid
		v += d * d
	}
	sd := math.Sqrt(v / float64(n))
	return mid + mult*sd, mid, mid - mult*sd, true
}

// mtfDonchian returns the highest high and lowest low of the last n candles,
// EXCLUDING the current one.
//
// Excluded deliberately: including the forming candle means price breaking its
// own high, which is true by construction and fires on every bar.
func mtfDonchian(c []Candle, n int) (hi, lo float64, ok bool) {
	if n <= 0 || len(c) < n+1 {
		return 0, 0, false
	}
	w := c[len(c)-n-1 : len(c)-1]
	hi, lo = w[0].High, w[0].Low
	for _, x := range w {
		hi = math.Max(hi, x.High)
		lo = math.Min(lo, x.Low)
	}
	return hi, lo, true
}

// mtfVolumeRatio compares the last candle's volume to the average of the
// preceding n, excluding itself.
func mtfVolumeRatio(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n+1 {
		return 0, false
	}
	s := 0.0
	for _, x := range c[len(c)-n-1 : len(c)-1] {
		s += x.Volume
	}
	avg := s / float64(n)
	if avg <= 0 {
		return 0, false
	}
	return c[len(c)-1].Volume / avg, true
}
