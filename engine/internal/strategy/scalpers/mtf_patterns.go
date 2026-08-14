package scalpers

import (
	"fmt"
	"math"
)

// mtf_patterns.go — candlestick and chart-pattern strategies on 15m/30m/1h/4h/1d.
//
// These patterns existed in the retired 1-minute packs (s87 inside bar, s90
// three-bar reversal, s92 engulfing, s93 pin bar, s94 morning/evening star) and
// lost money. The pattern logic was not necessarily wrong; the timeframe was. A
// pin bar on a 1m candle of a thin altcoin is frequently one tick of noise with
// a wick attached, and its rejection means nothing because almost nobody
// participated in forming it. On 15m and above each candle represents real
// accumulated participation, so the same shape carries information.
//
// Every pattern here is paired with a CONFIRMATION. That is the substance: an
// engulfing candle in a directionless range is noise, and the same candle at a
// pullback within a trend is a signal. Pattern alone is what fires constantly
// and wins rarely, which is what the 1m record showed.
//
// All of them inherit the pack rules: ATR-derived stops, and a target that must
// clear 6 round-trip fees or the setup is refused rather than traded small.

// ── candle shape helpers ─────────────────────────────────────────────────────

func pBody(c Candle) float64   { return math.Abs(c.Close - c.Open) }
func pRange(c Candle) float64  { return c.High - c.Low }
func pBull(c Candle) bool      { return c.Close > c.Open }
func pBear(c Candle) bool      { return c.Close < c.Open }
func pUpWick(c Candle) float64 { return c.High - math.Max(c.Open, c.Close) }
func pLoWick(c Candle) float64 { return math.Min(c.Open, c.Close) - c.Low }

// bodyFrac is the body as a share of the full range. Near 1 is decisive, near 0
// is indecision.
func bodyFrac(c Candle) float64 {
	r := pRange(c)
	if r <= 0 {
		return 0
	}
	return pBody(c) / r
}

// ── candlestick families ─────────────────────────────────────────────────────

// patEngulfing: the last body fully covers the prior body, against it.
//
// Confirmed by trend agreement. A bullish engulfing inside a downtrend is a
// counter-trend bet that needs far more than one candle to justify it.
func patEngulfing(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		prev, last := c[len(c)-2], c[len(c)-1]
		ema, ok1 := mtfEMA(c, 55)
		atr, ok2 := mtfATR(c, 14)
		vr, ok3 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 || !ok3 {
			return NoSignal(name)
		}
		// Conviction on the engulfing bar itself.
		if vr < 1.2 || bodyFrac(last) < 0.55 {
			return NoSignal(name)
		}
		if long {
			ok := pBull(last) && pBear(prev) && last.Close > prev.Open && last.Open < prev.Close
			if !ok || price < ema {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				fmt.Sprintf("bullish engulfing above EMA55 on %.1fx volume", vr))
		}
		ok := pBear(last) && pBull(prev) && last.Close < prev.Open && last.Open > prev.Close
		if !ok || price > ema {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			fmt.Sprintf("bearish engulfing below EMA55 on %.1fx volume", vr))
	}
}

// patPinBar: a long rejection wick, taken only where it rejects a LEVEL.
//
// The level requirement separates a pin bar from a candle that happened to have
// a wick. Rejecting mid-range means nothing was defended.
func patPinBar(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		hi, lo, ok2 := mtfDonchian(c, 20)
		if !ok1 || !ok2 || pRange(last) <= 0 {
			return NoSignal(name)
		}
		if long {
			// Long lower wick, small body, probed BELOW the 20-candle low, and
			// closed back inside — the rejection has to have succeeded.
			if pLoWick(last) < pRange(last)*0.55 || bodyFrac(last) > 0.35 {
				return NoSignal(name)
			}
			if last.Low > lo || last.Close < lo {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				"hammer rejecting the 20-candle low and closing back inside")
		}
		if pUpWick(last) < pRange(last)*0.55 || bodyFrac(last) > 0.35 {
			return NoSignal(name)
		}
		if last.High < hi || last.Close > hi {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			"shooting star rejecting the 20-candle high and closing back inside")
	}
}

// patInsideBarBreak: an inside bar formed during compression, then broken.
//
// Compression before the break is the filter. An inside bar in an already-quiet
// market is just another quiet candle.
func patInsideBarBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		mother, inside, last := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		atrNow, ok1 := mtfATR(c, 14)
		atrPrior, ok2 := mtfATR(c[:len(c)-5], 14)
		if !ok1 || !ok2 || atrPrior <= 0 {
			return NoSignal(name)
		}
		// The inside bar must actually be inside.
		if inside.High > mother.High || inside.Low < mother.Low {
			return NoSignal(name)
		}
		// Formed during compression, not during an expansion.
		if atrNow > atrPrior*1.1 {
			return NoSignal(name)
		}
		if long {
			if last.Close <= inside.High || last.Close <= mother.High {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atrNow, 3.0,
				"inside bar in compression, broken upward")
		}
		if last.Close >= inside.Low || last.Close >= mother.Low {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atrNow, 3.0,
			"inside bar in compression, broken downward")
	}
}

// patThreeBarReversal: thrust, pause, reversal — confirmed by RSI LEAVING an
// extreme rather than sitting in one.
func patThreeBarReversal(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		a, b, d := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		rsi, ok1 := mtfRSI(c, 14)
		atr, ok2 := mtfATR(c, 14)
		if !ok1 || !ok2 {
			return NoSignal(name)
		}
		if long {
			if !pBear(a) || !pBear(b) || !pBull(d) || d.Close < a.Open {
				return NoSignal(name)
			}
			// Recovering from oversold, not still falling into it.
			if rsi < 30 || rsi > 50 {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				fmt.Sprintf("three-bar reversal up, RSI recovering to %.0f", rsi))
		}
		if !pBull(a) || !pBull(b) || !pBear(d) || d.Close > a.Open {
			return NoSignal(name)
		}
		if rsi > 70 || rsi < 50 {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			fmt.Sprintf("three-bar reversal down, RSI cooling to %.0f", rsi))
	}
}

// patStar: morning/evening star — thrust, indecision, reversal past the
// midpoint of the thrust.
func patStar(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		a, star, d := c[len(c)-3], c[len(c)-2], c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 {
			return NoSignal(name)
		}
		// The middle candle must be genuinely indecisive, the reversal decisive.
		if bodyFrac(star) > 0.3 || bodyFrac(d) < 0.55 || vr < 1.2 {
			return NoSignal(name)
		}
		mid := (a.Open + a.Close) / 2
		if long {
			if !pBear(a) || !pBull(d) || d.Close < mid {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 3.0,
				"morning star closing past the midpoint of the down thrust")
		}
		if !pBull(a) || !pBear(d) || d.Close > mid {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 3.0,
			"evening star closing past the midpoint of the up thrust")
	}
}

// patDojiBreak: indecision, then resolution.
//
// The doji is the SETUP, never the entry. Trading the doji itself is trading
// the absence of a decision; this waits for the decision.
func patDojiBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		doji, last := c[len(c)-2], c[len(c)-1]
		atr, ok1 := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok1 || !ok2 {
			return NoSignal(name)
		}
		if bodyFrac(doji) > 0.12 || vr < 1.3 || bodyFrac(last) < 0.6 {
			return NoSignal(name)
		}
		if long {
			if !pBull(last) || last.Close <= doji.High {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				fmt.Sprintf("doji resolved upward on %.1fx volume", vr))
		}
		if !pBear(last) || last.Close >= doji.Low {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			fmt.Sprintf("doji resolved downward on %.1fx volume", vr))
	}
}

// ── chart-structure families ─────────────────────────────────────────────────

// swingPoints returns the indices of confirmed swing highs and lows.
//
// A swing needs candles on BOTH sides to confirm it, so the most recent bars
// can never be swings — recognising one before its right-hand side exists is
// how a "double top" gets drawn on a market still making highs.
func swingPoints(c []Candle, w int) (highs, lows []int) {
	for i := w; i < len(c)-w; i++ {
		isHigh, isLow := true, true
		for j := i - w; j <= i+w; j++ {
			if j == i {
				continue
			}
			if c[j].High >= c[i].High {
				isHigh = false
			}
			if c[j].Low <= c[i].Low {
				isLow = false
			}
		}
		if isHigh {
			highs = append(highs, i)
		}
		if isLow {
			lows = append(lows, i)
		}
	}
	return highs, lows
}

// patDoubleTopBottom: two swings at a comparable level, entered on the break of
// the intervening extreme — never on the second touch.
//
// Entering on the touch is predicting the pattern will complete. Waiting for
// the neckline break means the market has already agreed.
func patDoubleTopBottom(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		highs, lows := swingPoints(c, 3)
		tol := atr * 0.5 // "comparable" scaled to volatility, not a fixed percent

		if long {
			// Double BOTTOM: two lows within tolerance, break above the high
			// between them.
			if len(lows) < 2 {
				return NoSignal(name)
			}
			l2, l1 := lows[len(lows)-1], lows[len(lows)-2]
			a, b := c[l1].Low, c[l2].Low
			if math.Abs(a-b)/price > tol {
				return NoSignal(name)
			}
			neck := 0.0
			for i := l1; i <= l2; i++ {
				neck = math.Max(neck, c[i].High)
			}
			if neck <= 0 || price <= neck {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 3.0,
				"double bottom, neckline broken")
		}
		if len(highs) < 2 {
			return NoSignal(name)
		}
		h2, h1 := highs[len(highs)-1], highs[len(highs)-2]
		a, b := c[h1].High, c[h2].High
		if math.Abs(a-b)/price > tol {
			return NoSignal(name)
		}
		neck := math.MaxFloat64
		for i := h1; i <= h2; i++ {
			neck = math.Min(neck, c[i].Low)
		}
		if neck == math.MaxFloat64 || price >= neck {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 3.0,
			"double top, neckline broken")
	}
}

// patStructureBreak: higher-high / higher-low structure, entered on the break of
// the last swing high.
//
// Structure, not slope. An EMA can rise through a market making lower lows;
// swing structure cannot.
func patStructureBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok {
			return NoSignal(name)
		}
		highs, lows := swingPoints(c, 3)
		if len(highs) < 2 || len(lows) < 2 {
			return NoSignal(name)
		}
		hPrev, hLast := c[highs[len(highs)-2]].High, c[highs[len(highs)-1]].High
		lPrev, lLast := c[lows[len(lows)-2]].Low, c[lows[len(lows)-1]].Low

		if long {
			// Higher highs AND higher lows, then a break of the last high.
			if hLast <= hPrev || lLast <= lPrev || price <= hLast {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 3.0,
				"higher-high / higher-low structure, last swing high broken")
		}
		if hLast >= hPrev || lLast >= lPrev || price >= lLast {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 3.0,
			"lower-high / lower-low structure, last swing low broken")
	}
}

// patTriangleBreak: a converging range that breaks.
//
// Convergence is measured as the recent range being a fraction of the earlier
// range — a triangle is a range getting smaller, and that is checkable without
// drawing lines.
func patTriangleBreak(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok {
			return NoSignal(name)
		}
		rangeOf := func(w []Candle) (float64, float64) {
			hi, lo := w[0].High, w[0].Low
			for _, x := range w {
				hi = math.Max(hi, x.High)
				lo = math.Min(lo, x.Low)
			}
			return hi, lo
		}
		earlyHi, earlyLo := rangeOf(c[len(c)-30 : len(c)-15])
		lateHi, lateLo := rangeOf(c[len(c)-15 : len(c)-1])
		early, late := earlyHi-earlyLo, lateHi-lateLo
		if early <= 0 || late <= 0 {
			return NoSignal(name)
		}
		// Genuinely converging.
		if late > early*0.65 {
			return NoSignal(name)
		}
		if long {
			if price <= lateHi {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 3.0,
				fmt.Sprintf("range compressed to %.0f%% then broke up", late/early*100))
		}
		if price >= lateLo {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 3.0,
			fmt.Sprintf("range compressed to %.0f%% then broke down", late/early*100))
	}
}

// patLevelRetest: a broken level revisited and held.
//
// The retest is the trade, not the break. A break entered directly is entered
// at its worst price and with no evidence the level flipped; a retest that
// HOLDS is the market confirming it did.
func patLevelRetest(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 60 {
			return NoSignal(name)
		}
		atr, ok1 := mtfATR(c, 14)
		if !ok1 || atr <= 0 {
			return NoSignal(name)
		}
		// The level: the extreme of the window BEFORE the recent action.
		window := c[len(c)-40 : len(c)-10]
		hi, lo := window[0].High, window[0].Low
		for _, x := range window {
			hi = math.Max(hi, x.High)
			lo = math.Min(lo, x.Low)
		}
		recent := c[len(c)-10:]
		last := c[len(c)-1]
		near := atr * 0.6

		if long {
			// Broke above, came back to the level, and held it.
			broke := false
			for _, x := range recent {
				if x.Close > hi {
					broke = true
				}
			}
			if !broke || price < hi || math.Abs(price-hi)/price > near {
				return NoSignal(name)
			}
			if !pBull(last) {
				return NoSignal(name)
			}
			return mtfSignal(name, DirectionLong, price, atr, 2.5,
				"broken resistance retested and held as support")
		}
		broke := false
		for _, x := range recent {
			if x.Close < lo {
				broke = true
			}
		}
		if !broke || price > lo || math.Abs(price-lo)/price > near {
			return NoSignal(name)
		}
		if !pBear(last) {
			return NoSignal(name)
		}
		return mtfSignal(name, DirectionShort, price, atr, 2.5,
			"broken support retested and held as resistance")
	}
}
