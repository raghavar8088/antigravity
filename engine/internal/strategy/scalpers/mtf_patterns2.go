package scalpers

import (
	"fmt"
	"math"
)

// mtf_patterns2.go — the rest of the candlestick and chart-pattern catalogue.
//
// Same rules as the first pack: every pattern paired with a confirmation, an
// ATR-derived stop, and a STRUCTURAL target so the reward:risk falls out of the
// setup rather than being imposed on it.
//
// A note on the short timeframes these now run on. At 1m and 5m most of these
// shapes are noise — a "flag" formed over four one-minute candles on a thin
// altcoin is four ticks and a gap. The 6x round-trip fee bar will refuse the
// overwhelming majority of them automatically, because the measured move is
// smaller than the cost of taking it. That is the intended behaviour: the
// pattern is allowed to exist on every timeframe, and the economics decide
// where it is worth trading.

// ── candlestick, continued ───────────────────────────────────────────────────

// patMarubozu: a candle with almost no wick — no rejection at either end.
//
// Continuation, not reversal. A full-bodied candle says one side had the bar
// unopposed, which is only informative when it agrees with the trend; against
// the trend it is usually the last gasp of a move.
func patMarubozu(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		ema, ok1 := mtfEMA(c, 21)
		atr, ok2 := mtfATR(c, 14)
		vr, ok3 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || !ok3 || pRange(last) <= 0 {
			return NoSignal(name)
		}
		if bodyFrac(last) < 0.90 || vr < 1.2 {
			return NoSignal(name)
		}
		if long {
			if !pBull(last) || price < ema {
				return NoSignal(name)
			}
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("bullish marubozu with trend on %.1fx volume", vr))
		}
		if !pBear(last) || price > ema {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("bearish marubozu with trend on %.1fx volume", vr))
	}
}

// patOutsideBar: a bar that takes out BOTH sides of the prior bar and closes
// decisively.
//
// The close is what separates a reversal from a whipsaw: taking both sides and
// closing mid-range means both attempts failed, which is information about
// nobody rather than about direction.
func patOutsideBar(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		prev, last := c[len(c)-2], c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 {
			return NoSignal(name)
		}
		outside := last.High > prev.High && last.Low < prev.Low
		if !outside || bodyFrac(last) < 0.5 || vr < 1.2 {
			return NoSignal(name)
		}
		if long {
			if !pBull(last) {
				return NoSignal(name)
			}
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				"bullish outside bar took both sides and closed strong")
		}
		if !pBear(last) {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			"bearish outside bar took both sides and closed weak")
	}
}

// patThreeSoldiers: three consecutive same-direction closes, each extending.
//
// Requires each body to be substantial. Three tiny same-coloured candles is a
// drift, and drifts do not continue.
func patThreeSoldiers(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		a, b, d := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		adx, ok2 := mtfADX(c, 14)
		if !ok1 || !ok2 {
			return NoSignal(name)
		}
		for _, x := range []Candle{a, b, d} {
			if bodyFrac(x) < 0.5 {
				return NoSignal(name)
			}
		}
		// Early in a move, not after it has already run.
		if adx > 45 {
			return NoSignal(name)
		}
		if long {
			if !pBull(a) || !pBull(b) || !pBull(d) || b.Close <= a.Close || d.Close <= b.Close {
				return NoSignal(name)
			}
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("three advancing soldiers, ADX %.0f", adx))
		}
		if !pBear(a) || !pBear(b) || !pBear(d) || b.Close >= a.Close || d.Close >= b.Close {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("three black crows, ADX %.0f", adx))
	}
}

// patHeikinAshiFlip: a colour change on smoothed candles.
//
// Heikin Ashi averages away single-bar noise, so a flip means the recent
// several bars changed character rather than one bar printing oddly. Computed
// here rather than assumed, because the smoothing IS the signal.
func patHeikinAshiFlip(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		ha := heikinAshi(c)
		if len(ha) < 4 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		adx, ok2 := mtfADX(c, 14)
		if !ok1 || !ok2 || adx < 18 {
			return NoSignal(name)
		}
		prev2, prev1, last := ha[len(ha)-3], ha[len(ha)-2], ha[len(ha)-1]
		if long {
			// Two down, then up: a flip, not a single green bar in a downtrend.
			if !pBear(prev2) || !pBear(prev1) || !pBull(last) {
				return NoSignal(name)
			}
			tgt, ok := priorSwing(c, true)
			if !ok {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("Heikin Ashi flipped bullish, ADX %.0f", adx))
		}
		if !pBull(prev2) || !pBull(prev1) || !pBear(last) {
			return NoSignal(name)
		}
		tgt, ok := priorSwing(c, false)
		if !ok {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("Heikin Ashi flipped bearish, ADX %.0f", adx))
	}
}

// heikinAshi converts candles to their smoothed form.
func heikinAshi(c []Candle) []Candle {
	if len(c) == 0 {
		return nil
	}
	out := make([]Candle, 0, len(c))
	prevOpen := (c[0].Open + c[0].Close) / 2
	prevClose := (c[0].Open + c[0].High + c[0].Low + c[0].Close) / 4
	for _, x := range c {
		cl := (x.Open + x.High + x.Low + x.Close) / 4
		op := (prevOpen + prevClose) / 2
		out = append(out, Candle{
			Open: op, Close: cl,
			High:     math.Max(x.High, math.Max(op, cl)),
			Low:      math.Min(x.Low, math.Min(op, cl)),
			Volume:   x.Volume,
			OpenTime: x.OpenTime,
		})
		prevOpen, prevClose = op, cl
	}
	return out
}

// ── chart structure, continued ───────────────────────────────────────────────

// patHeadShoulders: three peaks with the middle highest, entered on the
// neckline break.
//
// Entered on the BREAK, never on the right shoulder: a right shoulder is only a
// right shoulder in hindsight, and calling one early is calling a top in an
// uptrend.
func patHeadShoulders(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		highs, lows := swingPoints(c, 3)
		tol := atr * 0.6

		if long {
			// INVERSE head and shoulders: three troughs, middle lowest.
			if len(lows) < 3 {
				return NoSignal(name)
			}
			l1, h, l3 := lows[len(lows)-3], lows[len(lows)-2], lows[len(lows)-1]
			ls, head, rs := c[l1].Low, c[h].Low, c[l3].Low
			if head >= ls || head >= rs {
				return NoSignal(name)
			}
			// Shoulders roughly level.
			if math.Abs(ls-rs)/price > tol {
				return NoSignal(name)
			}
			neck := 0.0
			for i := l1; i <= l3; i++ {
				neck = math.Max(neck, c[i].High)
			}
			if neck <= 0 || price <= neck {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, neck+(neck-head),
				"inverse head and shoulders, neckline broken")
		}
		if len(highs) < 3 {
			return NoSignal(name)
		}
		h1, h, h3 := highs[len(highs)-3], highs[len(highs)-2], highs[len(highs)-1]
		ls, head, rs := c[h1].High, c[h].High, c[h3].High
		if head <= ls || head <= rs {
			return NoSignal(name)
		}
		if math.Abs(ls-rs)/price > tol {
			return NoSignal(name)
		}
		neck := math.MaxFloat64
		for i := h1; i <= h3; i++ {
			neck = math.Min(neck, c[i].Low)
		}
		if neck == math.MaxFloat64 || price >= neck {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, neck-(head-neck),
			"head and shoulders, neckline broken")
	}
}

// patTripleTopBottom: three swings at a comparable level, entered on the break.
func patTripleTopBottom(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		highs, lows := swingPoints(c, 3)
		tol := atr * 0.5

		if long {
			if len(lows) < 3 {
				return NoSignal(name)
			}
			a, b, d := c[lows[len(lows)-3]].Low, c[lows[len(lows)-2]].Low, c[lows[len(lows)-1]].Low
			if math.Abs(a-b)/price > tol || math.Abs(b-d)/price > tol {
				return NoSignal(name)
			}
			neck := 0.0
			for i := lows[len(lows)-3]; i <= lows[len(lows)-1]; i++ {
				neck = math.Max(neck, c[i].High)
			}
			if neck <= 0 || price <= neck {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, neck+(neck-math.Min(a, math.Min(b, d))),
				"triple bottom, resistance broken")
		}
		if len(highs) < 3 {
			return NoSignal(name)
		}
		a, b, d := c[highs[len(highs)-3]].High, c[highs[len(highs)-2]].High, c[highs[len(highs)-1]].High
		if math.Abs(a-b)/price > tol || math.Abs(b-d)/price > tol {
			return NoSignal(name)
		}
		neck := math.MaxFloat64
		for i := highs[len(highs)-3]; i <= highs[len(highs)-1]; i++ {
			neck = math.Min(neck, c[i].Low)
		}
		if neck == math.MaxFloat64 || price >= neck {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, neck-(math.Max(a, math.Max(b, d))-neck),
			"triple top, support broken")
	}
}

// patDirectionalTriangle: one boundary flat, the other converging.
//
// Ascending (flat resistance, rising support) is the long case; descending
// (flat support, falling resistance) the short. Distinguished from the
// symmetrical triangle already in the pack, which converges on both sides.
func patDirectionalTriangle(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		highs, lows := swingPoints(c, 2)
		if len(highs) < 2 || len(lows) < 2 {
			return NoSignal(name)
		}
		h1, h2 := c[highs[len(highs)-2]].High, c[highs[len(highs)-1]].High
		l1, l2 := c[lows[len(lows)-2]].Low, c[lows[len(lows)-1]].Low
		flat := atr * 0.4

		if long {
			// Ascending: resistance flat, support rising.
			if math.Abs(h2-h1)/price > flat || l2 <= l1 {
				return NoSignal(name)
			}
			if price <= h2 {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, h2+(h2-l1),
				"ascending triangle, flat resistance broken")
		}
		// Descending: support flat, resistance falling.
		if math.Abs(l2-l1)/price > flat || h2 >= h1 {
			return NoSignal(name)
		}
		if price >= l2 {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, l2-(h1-l2),
			"descending triangle, flat support broken")
	}
}

// patFlag: a sharp impulse, then a shallow counter-drift, then resumption.
//
// The drift must be SHALLOW. A deep retracement is not a flag, it is a reversal
// that has not finished, and treating the two alike is how a continuation trade
// is taken into a turn.
func patFlag(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		// Impulse: the 10 bars before the last 5.
		imp := c[len(c)-15 : len(c)-5]
		flag := c[len(c)-5:]
		impMove := imp[len(imp)-1].Close - imp[0].Open
		if math.Abs(impMove)/price < atr*2 {
			return NoSignal(name) // not an impulse, just drift
		}
		flagHi, flagLo := flag[0].High, flag[0].Low
		for _, x := range flag {
			flagHi = math.Max(flagHi, x.High)
			flagLo = math.Min(flagLo, x.Low)
		}
		retrace := (flagHi - flagLo) / math.Abs(impMove)
		if retrace > 0.5 {
			return NoSignal(name) // too deep to be a flag
		}
		if long {
			if impMove <= 0 || price <= flagHi {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, flagHi+math.Abs(impMove),
				fmt.Sprintf("bull flag, %.0f%% retrace, target = impulse projected", retrace*100))
		}
		if impMove >= 0 || price >= flagLo {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, flagLo-math.Abs(impMove),
			fmt.Sprintf("bear flag, %.0f%% retrace, target = impulse projected", retrace*100))
	}
}

// patOpeningRangeBreak: the first bars of a session set a range; a break of it
// carries.
//
// Crypto has no session open, so the range is taken from a fixed recent window
// rather than a clock. The pattern is the same — an established range broken on
// volume — and pretending to know a session boundary that does not exist would
// be worse than admitting the substitution.
func patOpeningRangeBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || vr < 1.4 {
			return NoSignal(name)
		}
		rng := c[len(c)-13 : len(c)-1]
		hi, lo := rng[0].High, rng[0].Low
		for _, x := range rng {
			hi = math.Max(hi, x.High)
			lo = math.Min(lo, x.Low)
		}
		if hi <= lo {
			return NoSignal(name)
		}
		if long {
			if price <= hi {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, hi+(hi-lo),
				fmt.Sprintf("range break up on %.1fx volume", vr))
		}
		if price >= lo {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, lo-(hi-lo),
			fmt.Sprintf("range break down on %.1fx volume", vr))
	}
}

// patFibRetrace: a 61.8% pullback within a trend that holds.
//
// The level alone is not a signal — price passes through it constantly. What
// makes it tradable is the pullback HOLDING there while the trend structure is
// still intact, so both are required.
func patFibRetrace(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		adx, ok2 := mtfADX(c, 14)
		if !ok1 || !ok2 || adx < 20 || atr <= 0 {
			return NoSignal(name)
		}
		highs, lows := swingPoints(c, 3)
		if len(highs) == 0 || len(lows) == 0 {
			return NoSignal(name)
		}
		hi, lo := c[highs[len(highs)-1]].High, c[lows[len(lows)-1]].Low
		if hi <= lo {
			return NoSignal(name)
		}
		leg := hi - lo
		near := atr * 0.5
		last := c[len(c)-1]

		if long {
			lvl := hi - leg*0.618
			if math.Abs(price-lvl) > near || !pBull(last) {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, hi,
				fmt.Sprintf("61.8%% retrace held, ADX %.0f, target the swing high", adx))
		}
		lvl := lo + leg*0.618
		if math.Abs(price-lvl) > near || !pBear(last) {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, lo,
			fmt.Sprintf("61.8%% retrace held, ADX %.0f, target the swing low", adx))
	}
}
