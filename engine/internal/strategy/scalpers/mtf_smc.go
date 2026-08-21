package scalpers

import (
	"fmt"
	"math"
)

// mtf_smc.go — Smart Money Concepts: order blocks, fair value gaps, liquidity
// sweeps, breaker and mitigation blocks, optimal trade entry, and premium /
// discount positioning.
//
// These ideas are usually taught by drawing on a chart, which is exactly why
// they need stating as arithmetic before a machine can trade them. Every rule
// below is a measurement: an order block is the last opposing candle before a
// move that CLOSED beyond a prior swing; a fair value gap is a three-candle
// window whose outer wicks do not overlap; a liquidity sweep is a wick through a
// prior extreme with a close back inside it. Where a definition is genuinely
// contested, the stricter reading is used — a looser one matches half the chart.
//
// UNITS: mtfATR returns ATR as a FRACTION OF PRICE. Compare price gaps against
// atr*price; only mtfSignalToTarget takes the fraction.

// lastSwing returns the index of the most recent confirmed swing high/low.
// Confirmed means it has `w` candles either side, so it cannot be the bar still
// forming — using an unconfirmed swing is look-ahead dressed as structure.
func lastSwing(c []Candle, high bool, w int) (int, bool) {
	hs, ls := swingPoints(c, w)
	idx := ls
	if high {
		idx = hs
	}
	if len(idx) == 0 {
		return 0, false
	}
	return idx[len(idx)-1], true
}

// brokeStructure reports whether price has CLOSED beyond the last swing in the
// given direction, and returns the index of the candle that did it.
//
// A close, not a wick. A wick through a level is a liquidity sweep — the
// opposite trade — and conflating the two is the most expensive mistake in this
// whole family.
func brokeStructure(c []Candle, long bool, w int) (breakIdx int, level float64, ok bool) {
	si, ok1 := lastSwing(c, long, w)
	if !ok1 {
		return 0, 0, false
	}
	level = c[si].Low
	if long {
		level = c[si].High
	}
	for i := si + w + 1; i < len(c); i++ {
		if long && c[i].Close > level {
			return i, level, true
		}
		if !long && c[i].Close < level {
			return i, level, true
		}
	}
	return 0, 0, false
}

// patOrderBlock: the last opposing candle before a move that broke structure,
// re-entered from the other side.
//
// The structure break is what makes a candle an order block rather than merely
// the last down candle before an up move — which every up move has. Entry is on
// the RETURN to that candle body, not on the break itself.
func patOrderBlock(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		bi, level, ok2 := brokeStructure(c, long, 3)
		if !ok2 || bi < 3 {
			return NoSignal(name)
		}
		// Walk back from the breaking candle for the last opposing body.
		obIdx := -1
		for i := bi - 1; i >= bi-10 && i >= 0; i-- {
			if long && pBear(c[i]) {
				obIdx = i
				break
			}
			if !long && pBull(c[i]) {
				obIdx = i
				break
			}
		}
		if obIdx < 0 {
			return NoSignal(name)
		}
		ob := c[obIdx]
		obTop, obBot := math.Max(ob.Open, ob.Close), math.Min(ob.Open, ob.Close)
		if obTop-obBot < 0.3*atr*price {
			return NoSignal(name)
		}
		if long {
			// Price must have come BACK into the block, not still be running.
			if price > obTop || price < obBot-0.5*atr*price {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, level+(level-obBot),
				fmt.Sprintf("bullish order block retest after a close above %.6g", level))
		}
		if price < obBot || price > obTop+0.5*atr*price {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, level-(obTop-level),
			fmt.Sprintf("bearish order block retest after a close below %.6g", level))
	}
}

// patFairValueGap: a three-candle imbalance where the first and third candles do
// not overlap, entered when price returns to fill it.
//
// The non-overlap IS the gap: price moved so fast that one side never traded
// there. Requiring a minimum width in ATR keeps out the one-tick gaps that exist
// on every chart and mean nothing.
func patFairValueGap(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		atrAbs := atr * price
		// Scan recent history for the most recent unfilled gap.
		for i := len(c) - 4; i >= len(c)-40 && i >= 1; i-- {
			a, b := c[i-1], c[i+1]
			if long {
				// Bullish FVG: candle i+1's low sits ABOVE candle i-1's high.
				gapLo, gapHi := a.High, b.Low
				if gapHi-gapLo < 0.5*atrAbs {
					continue
				}
				// Unfilled until now, and price is inside it.
				filled := false
				for j := i + 2; j < len(c)-1; j++ {
					if c[j].Low <= gapLo {
						filled = true
						break
					}
				}
				if filled || price > gapHi || price < gapLo {
					continue
				}
				return mtfSignalToTarget(name, DirectionLong, price, atr, price+(gapHi-gapLo)*3,
					fmt.Sprintf("bullish FVG fill, %.1f ATR gap", (gapHi-gapLo)/atrAbs))
			}
			gapHi, gapLo := a.Low, b.High
			if gapHi-gapLo < 0.5*atrAbs {
				continue
			}
			filled := false
			for j := i + 2; j < len(c)-1; j++ {
				if c[j].High >= gapHi {
					filled = true
					break
				}
			}
			if filled || price < gapLo || price > gapHi {
				continue
			}
			return mtfSignalToTarget(name, DirectionShort, price, atr, price-(gapHi-gapLo)*3,
				fmt.Sprintf("bearish FVG fill, %.1f ATR gap", (gapHi-gapLo)/atrAbs))
		}
		return NoSignal(name)
	}
}

// patLiquiditySweep: a wick through a prior extreme that CLOSES back inside it.
//
// The close is the entire pattern. A close beyond the level is a break of
// structure and a continuation trade; a wick through with a close back inside is
// a stop run and a reversal trade. Same geometry, opposite direction — which is
// why brokeStructure above insists on a close, and this insists on the reverse.
func patLiquiditySweep(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		vr, ok2 := mtfVolumeRatio(c, 20)
		if !ok || !ok2 || atr <= 0 {
			return NoSignal(name)
		}
		last := c[len(c)-1]
		if long {
			// Sweep of a prior LOW, then close back above it.
			si, ok3 := lastSwing(c[:len(c)-1], false, 3)
			if !ok3 {
				return NoSignal(name)
			}
			level := c[si].Low
			if last.Low >= level || last.Close <= level || vr < 1.2 {
				return NoSignal(name)
			}
			// The wick must be a real excursion, not a rounding difference.
			if level-last.Low < 0.2*atr*price {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("liquidity swept %.1f ATR below %.6g, closed back inside on %.1fx volume",
					(level-last.Low)/(atr*price), level, vr))
		}
		si, ok3 := lastSwing(c[:len(c)-1], true, 3)
		if !ok3 {
			return NoSignal(name)
		}
		level := c[si].High
		if last.High <= level || last.Close >= level || vr < 1.2 {
			return NoSignal(name)
		}
		if last.High-level < 0.2*atr*price {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("liquidity swept %.1f ATR above %.6g, closed back inside on %.1fx volume",
				(last.High-level)/(atr*price), level, vr))
	}
}

// patBreakerBlock: an order block that FAILED, then flipped.
//
// Where an order block is defended, a breaker is one that was overrun — so the
// level that used to reject price now supports it from the other side. The
// failure is required: without it this is just an order block read backwards.
func patBreakerBlock(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 100 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		// For a bullish breaker: a swing HIGH that price closed above (the level
		// that failed to hold sellers), now retested from above.
		si, ok2 := lastSwing(c[:len(c)-2], !long, 3)
		if !ok2 {
			return NoSignal(name)
		}
		level := c[si].Low
		if !long {
			level = c[si].High
		}
		broke := false
		for i := si + 4; i < len(c)-1; i++ {
			if long && c[i].Close > level {
				broke = true
			}
			if !long && c[i].Close < level {
				broke = true
			}
		}
		if !broke {
			return NoSignal(name)
		}
		tol := 0.4 * atr * price
		if math.Abs(price-level) > tol {
			return NoSignal(name)
		}
		if long {
			if price < level-tol {
				return NoSignal(name)
			}
			tgt, okT := priorSwing(c, true)
			if !okT {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, tgt,
				fmt.Sprintf("bullish breaker: %.6g failed as resistance, retested as support", level))
		}
		if price > level+tol {
			return NoSignal(name)
		}
		tgt, okT := priorSwing(c, false)
		if !okT {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, tgt,
			fmt.Sprintf("bearish breaker: %.6g failed as support, retested as resistance", level))
	}
}

// patOptimalTradeEntry: the 62-79% retracement of the most recent impulse leg.
//
// A band, not a line. The window is narrow enough to be a real filter and wide
// enough that a tick either side does not disqualify a setup, which is the
// practical difference between a rule and a drawing.
func patOptimalTradeEntry(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 90 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		hi, lo := swingPoints(c, 3)
		if len(hi) == 0 || len(lo) == 0 {
			return NoSignal(name)
		}
		hIdx, lIdx := hi[len(hi)-1], lo[len(lo)-1]
		legHigh, legLow := c[hIdx].High, c[lIdx].Low
		if legHigh-legLow < 2*atr*price {
			return NoSignal(name)
		}
		leg := legHigh - legLow
		if long {
			// Impulse UP means the low came first; retracing into 62-79% of it.
			if lIdx > hIdx {
				return NoSignal(name)
			}
			lower, upper := legHigh-leg*0.79, legHigh-leg*0.62
			if price < lower || price > upper {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, legHigh,
				fmt.Sprintf("OTE long, %.0f%% retrace of a %.1f ATR leg", (legHigh-price)/leg*100, leg/(atr*price)))
		}
		if hIdx > lIdx {
			return NoSignal(name)
		}
		lower, upper := legLow+leg*0.62, legLow+leg*0.79
		if price < lower || price > upper {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, legLow,
			fmt.Sprintf("OTE short, %.0f%% retrace of a %.1f ATR leg", (price-legLow)/leg*100, leg/(atr*price)))
	}
}

// patPremiumDiscount: position within the dealing range — buy the lower third,
// sell the upper third, and only when the range is wide enough to have thirds
// worth distinguishing.
//
// The simplest idea in this file and the one that most needs its range defined:
// "the range" here is the last 60 candles, stated rather than eyeballed.
func patPremiumDiscount(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 80 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		w := c[len(c)-60:]
		hi, lo := w[0].High, w[0].Low
		for _, k := range w {
			hi = math.Max(hi, k.High)
			lo = math.Min(lo, k.Low)
		}
		rng := hi - lo
		if rng < 3*atr*price {
			return NoSignal(name)
		}
		mid := (hi + lo) / 2
		if long {
			// Discount: the lower third, and turning back up.
			if price > lo+rng/3 || !pBull(c[len(c)-1]) {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, mid,
				fmt.Sprintf("discount zone long, %.0f%% of a %.1f ATR range", (price-lo)/rng*100, rng/(atr*price)))
		}
		if price < hi-rng/3 || !pBear(c[len(c)-1]) {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, mid,
			fmt.Sprintf("premium zone short, %.0f%% of a %.1f ATR range", (price-lo)/rng*100, rng/(atr*price)))
	}
}

// patMitigationBlock: return to the ORIGIN of the move that broke structure.
//
// Distinct from the order block, which is the last opposing candle before that
// move. The mitigation block is where the move began — the candle whose low (or
// high) launched it — so the two sit at different prices and disagree often
// enough to be separate streams rather than one with a parameter.
func patMitigationBlock(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 100 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		bi, level, ok2 := brokeStructure(c, long, 3)
		if !ok2 || bi < 6 {
			return NoSignal(name)
		}
		// Origin: the extreme of the leg running into the break.
		start := bi - 10
		if start < 0 {
			start = 0
		}
		origin := c[start].Low
		if !long {
			origin = c[start].High
		}
		for _, k := range c[start:bi] {
			if long {
				origin = math.Min(origin, k.Low)
			} else {
				origin = math.Max(origin, k.High)
			}
		}
		tol := 0.5 * atr * price
		if long {
			if price > origin+tol || price < origin-tol {
				return NoSignal(name)
			}
			return mtfSignalToTarget(name, DirectionLong, price, atr, level,
				fmt.Sprintf("mitigation block: back at the origin %.6g of the leg that broke %.6g", origin, level))
		}
		if price < origin-tol || price > origin+tol {
			return NoSignal(name)
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, level,
			fmt.Sprintf("mitigation block: back at the origin %.6g of the leg that broke %.6g", origin, level))
	}
}
