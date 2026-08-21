package scalpers

import "math"

// mtf_indicators2.go — the indicator maths the strategy families below need.
//
// Every function returns (value, ok) and returns ok=false rather than a
// confident number when the window is too short. That convention is not
// decoration: an indicator computed over half its period is not "approximate",
// it is a different indicator wearing the right name, and the packs in this
// directory are explicitly required to refuse rather than emit one.
//
// Values are returned in the indicator's own natural units. Anything expressed
// as a PRICE (Supertrend, SAR, Ichimoku lines, Keltner/regression bands) comes
// back as a price so it can be compared to price directly; anything oscillating
// (CCI, %R, MFI, TSI) comes back on its conventional scale.

// ── moving averages ──────────────────────────────────────────────────────────

// mtfWMA is a linearly weighted moving average — the building block of the Hull.
func mtfWMA(c []Candle, n int) (float64, bool) {
	if n <= 0 || len(c) < n {
		return 0, false
	}
	var num, den float64
	for i := 0; i < n; i++ {
		w := float64(i + 1)
		num += c[len(c)-n+i].Close * w
		den += w
	}
	if den == 0 {
		return 0, false
	}
	return num / den, true
}

// mtfHMA is the Hull moving average: WMA(2*WMA(n/2) - WMA(n)) over sqrt(n).
//
// Faster than an EMA at the same length with less lag, which is the entire
// reason it is worth a separate family from the EMA cross.
func mtfHMA(c []Candle, n int) (float64, bool) {
	half, root := n/2, int(math.Sqrt(float64(n)))
	if half < 1 || root < 1 || len(c) < n+root {
		return 0, false
	}
	raw := make([]Candle, 0, root)
	for k := root - 1; k >= 0; k-- {
		end := len(c) - k
		w1, ok1 := mtfWMA(c[:end], half)
		w2, ok2 := mtfWMA(c[:end], n)
		if !ok1 || !ok2 {
			return 0, false
		}
		raw = append(raw, Candle{Close: 2*w1 - w2})
	}
	return mtfWMA(raw, root)
}

// emaSeries returns the EMA at each of the last `out` positions, so a strategy
// can see the CROSS rather than only the current relationship.
func emaSeries(c []Candle, n, out int) ([]float64, bool) {
	if n <= 0 || out <= 0 || len(c) < n+out {
		return nil, false
	}
	k := 2.0 / float64(n+1)
	var sum float64
	for i := 0; i < n; i++ {
		sum += c[i].Close
	}
	ema := sum / float64(n)
	vals := make([]float64, 0, out)
	for i := n; i < len(c); i++ {
		ema = c[i].Close*k + ema*(1-k)
		if i >= len(c)-out {
			vals = append(vals, ema)
		}
	}
	if len(vals) < out {
		return nil, false
	}
	return vals, true
}

// mtfDEMA and mtfTEMA reduce lag by subtracting the EMA's own lag, once and
// twice respectively.
func mtfDEMA(c []Candle, n int) (float64, bool) {
	e1, ok := emaSeries(c, n, 1)
	if !ok {
		return 0, false
	}
	inner := make([]Candle, 0, len(c))
	vals, ok2 := emaSeries(c, n, len(c)-n)
	if !ok2 {
		return 0, false
	}
	for _, v := range vals {
		inner = append(inner, Candle{Close: v})
	}
	e2, ok3 := emaSeries(inner, n, 1)
	if !ok3 {
		return 0, false
	}
	return 2*e1[0] - e2[0], true
}

func mtfTEMA(c []Candle, n int) (float64, bool) {
	vals1, ok := emaSeries(c, n, len(c)-n)
	if !ok || len(vals1) < n+1 {
		return 0, false
	}
	in1 := make([]Candle, 0, len(vals1))
	for _, v := range vals1 {
		in1 = append(in1, Candle{Close: v})
	}
	vals2, ok2 := emaSeries(in1, n, len(in1)-n)
	if !ok2 || len(vals2) < n+1 {
		return 0, false
	}
	in2 := make([]Candle, 0, len(vals2))
	for _, v := range vals2 {
		in2 = append(in2, Candle{Close: v})
	}
	e3, ok3 := emaSeries(in2, n, 1)
	if !ok3 {
		return 0, false
	}
	return 3*vals1[len(vals1)-1] - 3*vals2[len(vals2)-1] + e3[0], true
}

// mtfKAMA is Kaufman's adaptive MA: fast when price trends, slow when it chops.
func mtfKAMA(c []Candle, n int) (float64, bool) {
	if len(c) < n*2 {
		return 0, false
	}
	const fast, slow = 2.0, 30.0
	fastSC, slowSC := 2/(fast+1), 2/(slow+1)
	kama := c[len(c)-n*2].Close
	for i := len(c) - n*2 + n; i < len(c); i++ {
		change := math.Abs(c[i].Close - c[i-n].Close)
		var vol float64
		for j := i - n + 1; j <= i; j++ {
			vol += math.Abs(c[j].Close - c[j-1].Close)
		}
		er := 0.0
		if vol > 0 {
			er = change / vol
		}
		sc := math.Pow(er*(fastSC-slowSC)+slowSC, 2)
		kama += sc * (c[i].Close - kama)
	}
	return kama, true
}

// ── trend ────────────────────────────────────────────────────────────────────

// mtfSupertrend returns the Supertrend line and whether it is in an UP regime.
func mtfSupertrend(c []Candle, period int, mult float64) (line float64, up bool, ok bool) {
	if len(c) < period*3 {
		return 0, false, false
	}
	start := len(c) - period*3
	var upper, lower, st float64
	trendUp := true
	for i := start; i < len(c); i++ {
		atr, okA := mtfATR(c[:i+1], period)
		if !okA {
			continue
		}
		atrAbs := atr * c[i].Close
		mid := (c[i].High + c[i].Low) / 2
		bu, bl := mid+mult*atrAbs, mid-mult*atrAbs
		if i == start {
			upper, lower, st = bu, bl, bl
			continue
		}
		if bu < upper || c[i-1].Close > upper {
			upper = bu
		}
		if bl > lower || c[i-1].Close < lower {
			lower = bl
		}
		if trendUp {
			if c[i].Close < lower {
				trendUp = false
				st = upper
			} else {
				st = lower
			}
		} else {
			if c[i].Close > upper {
				trendUp = true
				st = lower
			} else {
				st = upper
			}
		}
	}
	return st, trendUp, true
}

// mtfPSAR returns the parabolic SAR and whether it sits BELOW price (long regime).
func mtfPSAR(c []Candle, step, maxStep float64) (sar float64, below bool, ok bool) {
	if len(c) < 30 {
		return 0, false, false
	}
	seg := c[len(c)-30:]
	rising := seg[1].Close > seg[0].Close
	sar = seg[0].Low
	ep := seg[0].High
	if !rising {
		sar, ep = seg[0].High, seg[0].Low
	}
	af := step
	for i := 1; i < len(seg); i++ {
		sar += af * (ep - sar)
		if rising {
			if seg[i].Low < sar {
				rising, sar, ep, af = false, ep, seg[i].Low, step
				continue
			}
			if seg[i].High > ep {
				ep = seg[i].High
				af = math.Min(af+step, maxStep)
			}
		} else {
			if seg[i].High > sar {
				rising, sar, ep, af = true, ep, seg[i].High, step
				continue
			}
			if seg[i].Low < ep {
				ep = seg[i].Low
				af = math.Min(af+step, maxStep)
			}
		}
	}
	return sar, rising, true
}

// mtfIchimoku returns the conversion line, base line and the two cloud edges.
//
// The cloud is projected forward by convention; here the CURRENT cloud is
// returned — the span values computed from data that has already closed — so no
// strategy can read a level that does not exist yet.
func mtfIchimoku(c []Candle) (tenkan, kijun, spanA, spanB float64, ok bool) {
	hl := func(n int) (float64, float64, bool) {
		if len(c) < n {
			return 0, 0, false
		}
		hi, lo := c[len(c)-n].High, c[len(c)-n].Low
		for _, k := range c[len(c)-n:] {
			hi = math.Max(hi, k.High)
			lo = math.Min(lo, k.Low)
		}
		return hi, lo, true
	}
	h9, l9, ok1 := hl(9)
	h26, l26, ok2 := hl(26)
	h52, l52, ok3 := hl(52)
	if !ok1 || !ok2 || !ok3 {
		return 0, 0, 0, 0, false
	}
	tenkan = (h9 + l9) / 2
	kijun = (h26 + l26) / 2
	spanA = (tenkan + kijun) / 2
	spanB = (h52 + l52) / 2
	return tenkan, kijun, spanA, spanB, true
}

// mtfAroon returns Aroon Up and Down (0-100).
func mtfAroon(c []Candle, n int) (up, down float64, ok bool) {
	if len(c) < n+1 {
		return 0, 0, false
	}
	seg := c[len(c)-n-1:]
	hiIdx, loIdx := 0, 0
	for i, k := range seg {
		if k.High >= seg[hiIdx].High {
			hiIdx = i
		}
		if k.Low <= seg[loIdx].Low {
			loIdx = i
		}
	}
	last := float64(len(seg) - 1)
	up = 100 * (float64(hiIdx) / last)
	down = 100 * (float64(loIdx) / last)
	return up, down, true
}

// mtfVortex returns VI+ and VI-.
func mtfVortex(c []Candle, n int) (viPlus, viMinus float64, ok bool) {
	if len(c) < n+1 {
		return 0, 0, false
	}
	var vmP, vmM, tr float64
	for i := len(c) - n; i < len(c); i++ {
		vmP += math.Abs(c[i].High - c[i-1].Low)
		vmM += math.Abs(c[i].Low - c[i-1].High)
		tr += math.Max(c[i].High-c[i].Low,
			math.Max(math.Abs(c[i].High-c[i-1].Close), math.Abs(c[i].Low-c[i-1].Close)))
	}
	if tr == 0 {
		return 0, 0, false
	}
	return vmP / tr, vmM / tr, true
}

// ── oscillators ──────────────────────────────────────────────────────────────

// mtfCCI — Commodity Channel Index on the typical price.
func mtfCCI(c []Candle, n int) (float64, bool) {
	if len(c) < n {
		return 0, false
	}
	tp := func(k Candle) float64 { return (k.High + k.Low + k.Close) / 3 }
	seg := c[len(c)-n:]
	var mean float64
	for _, k := range seg {
		mean += tp(k)
	}
	mean /= float64(n)
	var dev float64
	for _, k := range seg {
		dev += math.Abs(tp(k) - mean)
	}
	dev /= float64(n)
	if dev == 0 {
		return 0, false
	}
	return (tp(seg[len(seg)-1]) - mean) / (0.015 * dev), true
}

// mtfWilliamsR — %R, from -100 (at the low) to 0 (at the high).
func mtfWilliamsR(c []Candle, n int) (float64, bool) {
	if len(c) < n {
		return 0, false
	}
	seg := c[len(c)-n:]
	hi, lo := seg[0].High, seg[0].Low
	for _, k := range seg {
		hi = math.Max(hi, k.High)
		lo = math.Min(lo, k.Low)
	}
	if hi == lo {
		return 0, false
	}
	return -100 * (hi - seg[len(seg)-1].Close) / (hi - lo), true
}

// mtfStochastic returns %K and %D.
func mtfStochastic(c []Candle, n, smooth int) (k, d float64, ok bool) {
	if len(c) < n+smooth {
		return 0, 0, false
	}
	raw := make([]float64, 0, smooth)
	for s := smooth - 1; s >= 0; s-- {
		seg := c[len(c)-n-s : len(c)-s]
		hi, lo := seg[0].High, seg[0].Low
		for _, x := range seg {
			hi = math.Max(hi, x.High)
			lo = math.Min(lo, x.Low)
		}
		if hi == lo {
			return 0, 0, false
		}
		raw = append(raw, 100*(seg[len(seg)-1].Close-lo)/(hi-lo))
	}
	k = raw[len(raw)-1]
	var sum float64
	for _, v := range raw {
		sum += v
	}
	return k, sum / float64(len(raw)), true
}

// mtfCMO — Chande Momentum Oscillator, -100..100.
func mtfCMO(c []Candle, n int) (float64, bool) {
	if len(c) < n+1 {
		return 0, false
	}
	var up, down float64
	for i := len(c) - n; i < len(c); i++ {
		d := c[i].Close - c[i-1].Close
		if d > 0 {
			up += d
		} else {
			down -= d
		}
	}
	if up+down == 0 {
		return 0, false
	}
	return 100 * (up - down) / (up + down), true
}

// mtfROC — rate of change, in percent.
func mtfROC(c []Candle, n int) (float64, bool) {
	if len(c) < n+1 {
		return 0, false
	}
	prev := c[len(c)-1-n].Close
	if prev == 0 {
		return 0, false
	}
	return 100 * (c[len(c)-1].Close - prev) / prev, true
}

// mtfTSI — True Strength Index, a doubly smoothed momentum ratio.
func mtfTSI(c []Candle, long, short int) (float64, bool) {
	if len(c) < long+short+2 {
		return 0, false
	}
	mom := make([]Candle, 0, len(c)-1)
	absMom := make([]Candle, 0, len(c)-1)
	for i := 1; i < len(c); i++ {
		d := c[i].Close - c[i-1].Close
		mom = append(mom, Candle{Close: d})
		absMom = append(absMom, Candle{Close: math.Abs(d)})
	}
	smooth := func(src []Candle, n1, n2 int) (float64, bool) {
		v1, ok := emaSeries(src, n1, len(src)-n1)
		if !ok {
			return 0, false
		}
		in := make([]Candle, 0, len(v1))
		for _, v := range v1 {
			in = append(in, Candle{Close: v})
		}
		v2, ok2 := emaSeries(in, n2, 1)
		if !ok2 {
			return 0, false
		}
		return v2[0], true
	}
	num, ok1 := smooth(mom, long, short)
	den, ok2 := smooth(absMom, long, short)
	if !ok1 || !ok2 || den == 0 {
		return 0, false
	}
	return 100 * num / den, true
}

// mtfTRIX — the rate of change of a triple-smoothed EMA, in percent.
func mtfTRIX(c []Candle, n int) (float64, bool) {
	v1, ok := emaSeries(c, n, len(c)-n)
	if !ok || len(v1) < n+2 {
		return 0, false
	}
	in1 := make([]Candle, 0, len(v1))
	for _, v := range v1 {
		in1 = append(in1, Candle{Close: v})
	}
	v2, ok2 := emaSeries(in1, n, len(in1)-n)
	if !ok2 || len(v2) < n+2 {
		return 0, false
	}
	in2 := make([]Candle, 0, len(v2))
	for _, v := range v2 {
		in2 = append(in2, Candle{Close: v})
	}
	v3, ok3 := emaSeries(in2, n, 2)
	if !ok3 || v3[0] == 0 {
		return 0, false
	}
	return 100 * (v3[1] - v3[0]) / v3[0], true
}

// mtfFisher — the Fisher Transform, which sharpens turns by making the price
// distribution closer to normal.
func mtfFisher(c []Candle, n int) (fish, prev float64, ok bool) {
	if len(c) < n+3 {
		return 0, 0, false
	}
	value, f := 0.0, 0.0
	var last float64
	for i := len(c) - n - 2; i < len(c); i++ {
		seg := c[i-n+1 : i+1]
		hi, lo := seg[0].High, seg[0].Low
		for _, k := range seg {
			hi = math.Max(hi, k.High)
			lo = math.Min(lo, k.Low)
		}
		if hi == lo {
			return 0, 0, false
		}
		mid := (c[i].High + c[i].Low) / 2
		x := 2*((mid-lo)/(hi-lo)) - 1
		value = 0.66*x + 0.67*value
		value = math.Max(-0.999, math.Min(0.999, value))
		last = f
		f = 0.5*math.Log((1+value)/(1-value)) + 0.5*f
	}
	return f, last, true
}

// ── volume ───────────────────────────────────────────────────────────────────

// mtfOBV returns on-balance volume now and `back` bars ago, so a strategy can
// compare the two rather than read a level with no scale.
func mtfOBV(c []Candle, back int) (now, then float64, ok bool) {
	if len(c) < back+2 {
		return 0, 0, false
	}
	var obv float64
	for i := 1; i < len(c); i++ {
		switch {
		case c[i].Close > c[i-1].Close:
			obv += c[i].Volume
		case c[i].Close < c[i-1].Close:
			obv -= c[i].Volume
		}
		if i == len(c)-1-back {
			then = obv
		}
	}
	return obv, then, true
}

// mtfMFI — Money Flow Index, volume-weighted RSI on the typical price.
func mtfMFI(c []Candle, n int) (float64, bool) {
	if len(c) < n+1 {
		return 0, false
	}
	tp := func(k Candle) float64 { return (k.High + k.Low + k.Close) / 3 }
	var pos, neg float64
	for i := len(c) - n; i < len(c); i++ {
		t, p := tp(c[i]), tp(c[i-1])
		flow := t * c[i].Volume
		if t > p {
			pos += flow
		} else if t < p {
			neg += flow
		}
	}
	if neg == 0 {
		if pos == 0 {
			return 0, false
		}
		return 100, true
	}
	return 100 - 100/(1+pos/neg), true
}

// mtfCMF — Chaikin Money Flow, -1..1.
func mtfCMF(c []Candle, n int) (float64, bool) {
	if len(c) < n {
		return 0, false
	}
	var mfv, vol float64
	for _, k := range c[len(c)-n:] {
		rng := k.High - k.Low
		if rng <= 0 {
			continue
		}
		mfv += ((k.Close - k.Low) - (k.High - k.Close)) / rng * k.Volume
		vol += k.Volume
	}
	if vol == 0 {
		return 0, false
	}
	return mfv / vol, true
}

// ── statistical ──────────────────────────────────────────────────────────────

// mtfZScore — how many standard deviations price sits from its rolling mean.
func mtfZScore(c []Candle, n int) (float64, bool) {
	if len(c) < n {
		return 0, false
	}
	seg := c[len(c)-n:]
	var mean float64
	for _, k := range seg {
		mean += k.Close
	}
	mean /= float64(n)
	var v float64
	for _, k := range seg {
		d := k.Close - mean
		v += d * d
	}
	sd := math.Sqrt(v / float64(n))
	if sd == 0 {
		return 0, false
	}
	return (seg[len(seg)-1].Close - mean) / sd, true
}

// mtfLinReg returns the regression value at the last bar, the per-bar slope and
// the standard error of the fit — the three numbers a regression channel needs.
func mtfLinReg(c []Candle, n int) (value, slope, stderr float64, ok bool) {
	if len(c) < n {
		return 0, 0, 0, false
	}
	seg := c[len(c)-n:]
	fn := float64(n)
	var sx, sy, sxx, sxy float64
	for i, k := range seg {
		x := float64(i)
		sx += x
		sy += k.Close
		sxx += x * x
		sxy += x * k.Close
	}
	den := fn*sxx - sx*sx
	if den == 0 {
		return 0, 0, 0, false
	}
	slope = (fn*sxy - sx*sy) / den
	intercept := (sy - slope*sx) / fn
	var se float64
	for i, k := range seg {
		fit := intercept + slope*float64(i)
		d := k.Close - fit
		se += d * d
	}
	stderr = math.Sqrt(se / fn)
	return intercept + slope*(fn-1), slope, stderr, true
}

// mtfChoppiness — 100 means pure chop, 0 means pure trend.
//
// Used as a REGIME GATE rather than a signal: mean-reversion families are
// allowed to fire only when it is high, breakout families only when it is low.
func mtfChoppiness(c []Candle, n int) (float64, bool) {
	if len(c) < n+1 {
		return 0, false
	}
	var trSum float64
	hi, lo := c[len(c)-n].High, c[len(c)-n].Low
	for i := len(c) - n; i < len(c); i++ {
		trSum += math.Max(c[i].High-c[i].Low,
			math.Max(math.Abs(c[i].High-c[i-1].Close), math.Abs(c[i].Low-c[i-1].Close)))
		hi = math.Max(hi, c[i].High)
		lo = math.Min(lo, c[i].Low)
	}
	rng := hi - lo
	if rng <= 0 || trSum <= 0 {
		return 0, false
	}
	return 100 * math.Log10(trSum/rng) / math.Log10(float64(n)), true
}
