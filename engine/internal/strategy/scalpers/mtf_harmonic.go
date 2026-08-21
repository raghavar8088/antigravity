package scalpers

import (
	"fmt"
	"math"
)

// mtf_harmonic.go — the XABCD harmonic patterns: Gartley, Bat, Butterfly, Crab,
// Shark, Cypher, plus ABCD and Three Drives.
//
// Harmonics are the family most often traded by eye, and the one that least
// survives it: every five-swing sequence looks like SOMETHING if the ratios are
// allowed to drift. So each pattern here is a set of numeric windows, and a
// sequence either lands inside all of them or is refused. No pattern is
// "close enough".
//
// The ratio windows below are the conventional ones (Scott Carney's
// definitions). Where a source gives a single number the window is that number
// with a tolerance; where it gives a range the range is used as-is. The
// tolerance is deliberately tight — widening it does not find more valid
// patterns, it relabels invalid ones.
//
// UNITS: mtfATR returns ATR as a FRACTION OF PRICE.

// swingSeq returns the last n ALTERNATING swing points, oldest first.
//
// Alternating matters: X-A-B-C-D is high-low-high-low-high (or its mirror), and
// a sequence that takes two highs in a row is not a harmonic leg structure — it
// is two separate moves being read as one.
func swingSeq(c []Candle, n, w int) ([]int, []bool, bool) {
	hs, ls := swingPoints(c, w)
	type pt struct {
		idx  int
		high bool
	}
	all := make([]pt, 0, len(hs)+len(ls))
	for _, i := range hs {
		all = append(all, pt{i, true})
	}
	for _, i := range ls {
		all = append(all, pt{i, false})
	}
	// Sort by index: swingPoints returns highs and lows separately.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && all[j].idx < all[j-1].idx; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	// Keep only alternating points, preferring the more extreme of a run.
	alt := make([]pt, 0, len(all))
	for _, p := range all {
		if len(alt) == 0 || alt[len(alt)-1].high != p.high {
			alt = append(alt, p)
			continue
		}
		last := alt[len(alt)-1]
		if p.high && c[p.idx].High > c[last.idx].High {
			alt[len(alt)-1] = p
		}
		if !p.high && c[p.idx].Low < c[last.idx].Low {
			alt[len(alt)-1] = p
		}
	}
	if len(alt) < n {
		return nil, nil, false
	}
	alt = alt[len(alt)-n:]
	idx := make([]int, n)
	isHigh := make([]bool, n)
	for i, p := range alt {
		idx[i], isHigh[i] = p.idx, p.high
	}
	return idx, isHigh, true
}

// priceAt returns the swing's own extreme.
func priceAt(c []Candle, i int, high bool) float64 {
	if high {
		return c[i].High
	}
	return c[i].Low
}

// within reports whether v sits inside [lo,hi]. Stated as a helper so every
// ratio check reads the same way and none quietly uses a different tolerance.
func within(v, lo, hi float64) bool { return v >= lo && v <= hi }

// harmonicSpec is one pattern's ratio windows.
type harmonicSpec struct {
	name string
	abXA [2]float64 // AB retracement of XA
	bcAB [2]float64 // BC retracement of AB
	cdBC [2]float64 // CD extension of BC
	adXA [2]float64 // D's retracement/extension of XA — the defining leg
}

// The conventional windows. Gartley and Bat differ ONLY in adXA (0.786 vs
// 0.886) and their B point, which is precisely why they must not share a
// tolerance wide enough to cover both.
var harmonicSpecs = map[string]harmonicSpec{
	"Gartley":   {"Gartley", [2]float64{0.55, 0.68}, [2]float64{0.382, 0.886}, [2]float64{1.13, 1.618}, [2]float64{0.75, 0.83}},
	"Bat":       {"Bat", [2]float64{0.35, 0.55}, [2]float64{0.382, 0.886}, [2]float64{1.618, 2.618}, [2]float64{0.85, 0.93}},
	"Butterfly": {"Butterfly", [2]float64{0.74, 0.83}, [2]float64{0.382, 0.886}, [2]float64{1.618, 2.24}, [2]float64{1.20, 1.70}},
	"Crab":      {"Crab", [2]float64{0.35, 0.65}, [2]float64{0.382, 0.886}, [2]float64{2.24, 3.618}, [2]float64{1.55, 1.70}},
	"Cypher":    {"Cypher", [2]float64{0.35, 0.65}, [2]float64{1.13, 1.414}, [2]float64{1.27, 2.00}, [2]float64{0.74, 0.83}},
	"Shark":     {"Shark", [2]float64{0.35, 0.65}, [2]float64{1.13, 1.618}, [2]float64{1.27, 2.24}, [2]float64{0.85, 1.13}},
}

// harmonicFamily builds a detector for one named pattern.
//
// D is the entry: the point where the pattern completes and the trade is taken
// AGAINST the final leg. A harmonic that has not reached D is not a setup, it is
// a drawing, so price must actually be at D.
func harmonicFamily(spec harmonicSpec) func(bool) func(string, []Candle, float64) Signal {
	return func(long bool) func(string, []Candle, float64) Signal {
		return func(name string, c []Candle, price float64) Signal {
			if len(c) < 120 {
				return NoSignal(name)
			}
			atr, ok := mtfATR(c, 14)
			if !ok || atr <= 0 {
				return NoSignal(name)
			}
			idx, isHigh, ok2 := swingSeq(c, 5, 3)
			if !ok2 {
				return NoSignal(name)
			}
			// ORIENTATION. A bullish XABCD completes at a LOW, and the five
			// points alternate, so the sequence is X low, A high, B low, C high,
			// D low. The test is therefore on D: for a long, D must be a low.
			//
			// Checking X the other way round — as an earlier draft did — inverts
			// every pattern in the file: it would take longs at pattern tops and
			// shorts at pattern bottoms while every ratio window still passed,
			// which is the most expensive way for this family to be wrong.
			if isHigh[4] == long {
				return NoSignal(name)
			}
			X := priceAt(c, idx[0], isHigh[0])
			A := priceAt(c, idx[1], isHigh[1])
			B := priceAt(c, idx[2], isHigh[2])
			C := priceAt(c, idx[3], isHigh[3])
			D := priceAt(c, idx[4], isHigh[4])

			xa, ab, bc := math.Abs(A-X), math.Abs(B-A), math.Abs(C-B)
			cd, ad := math.Abs(D-C), math.Abs(D-A)
			if xa <= 0 || ab <= 0 || bc <= 0 || cd <= 0 {
				return NoSignal(name)
			}
			// The whole structure must be worth trading, not five ticks of noise.
			if xa < 2*atr*price {
				return NoSignal(name)
			}
			if !within(ab/xa, spec.abXA[0], spec.abXA[1]) ||
				!within(bc/ab, spec.bcAB[0], spec.bcAB[1]) ||
				!within(cd/bc, spec.cdBC[0], spec.cdBC[1]) ||
				!within(ad/xa, spec.adXA[0], spec.adXA[1]) {
				return NoSignal(name)
			}
			// Price must be AT D, not somewhere past it.
			if math.Abs(price-D) > 0.75*atr*price {
				return NoSignal(name)
			}
			// Target: the conventional first objective, the 0.382 retrace of AD.
			if long {
				return mtfSignalToTarget(name, DirectionLong, price, atr, D+ad*0.382,
					fmt.Sprintf("%s completion at D: AB/XA %.3f, BC/AB %.3f, CD/BC %.3f, AD/XA %.3f",
						spec.name, ab/xa, bc/ab, cd/bc, ad/xa))
			}
			return mtfSignalToTarget(name, DirectionShort, price, atr, D-ad*0.382,
				fmt.Sprintf("%s completion at D: AB/XA %.3f, BC/AB %.3f, CD/BC %.3f, AD/XA %.3f",
					spec.name, ab/xa, bc/ab, cd/bc, ad/xa))
		}
	}
}

// patABCD: the simplest of the family — AB and CD of comparable length, with BC
// a proper retracement between them.
//
// Four points, not five, so it needs no X. The equality of AB and CD is the
// pattern, so it is a window rather than an approximation.
func patABCD(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 100 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		idx, isHigh, ok2 := swingSeq(c, 4, 3)
		if !ok2 {
			return NoSignal(name)
		}
		// Bullish ABCD completes at a low: A high, B low, C high, D low.
		if long != isHigh[0] {
			return NoSignal(name)
		}
		A := priceAt(c, idx[0], isHigh[0])
		B := priceAt(c, idx[1], isHigh[1])
		C := priceAt(c, idx[2], isHigh[2])
		D := priceAt(c, idx[3], isHigh[3])
		ab, bc, cd := math.Abs(B-A), math.Abs(C-B), math.Abs(D-C)
		if ab < 2*atr*price || bc <= 0 || cd <= 0 {
			return NoSignal(name)
		}
		if !within(bc/ab, 0.382, 0.886) || !within(cd/ab, 0.85, 1.30) {
			return NoSignal(name)
		}
		if math.Abs(price-D) > 0.75*atr*price {
			return NoSignal(name)
		}
		if long {
			return mtfSignalToTarget(name, DirectionLong, price, atr, D+cd*0.618,
				fmt.Sprintf("bullish ABCD: BC/AB %.3f, CD/AB %.3f", bc/ab, cd/ab))
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, D-cd*0.618,
			fmt.Sprintf("bearish ABCD: BC/AB %.3f, CD/AB %.3f", bc/ab, cd/ab))
	}
}

// patThreeDrives: three successive extensions with symmetric retracements —
// exhaustion by repetition.
//
// Each drive must EXTEND the last, and the pullbacks between them must be
// comparable. Drives that shrink are a wedge, and drives with erratic pullbacks
// are a trend; neither is this pattern.
func patThreeDrives(long bool) func(string, []Candle, float64) Signal {
	return func(name string, c []Candle, price float64) Signal {
		if len(c) < 120 {
			return NoSignal(name)
		}
		atr, ok := mtfATR(c, 14)
		if !ok || atr <= 0 {
			return NoSignal(name)
		}
		idx, isHigh, ok2 := swingSeq(c, 6, 3)
		if !ok2 {
			return NoSignal(name)
		}
		// For a bullish setup the drives are DOWN: the sequence ends on a low.
		if isHigh[5] == long {
			return NoSignal(name)
		}
		p := make([]float64, 6)
		for i := range idx {
			p[i] = priceAt(c, idx[i], isHigh[i])
		}
		// Three drives: p1, p3, p5 are the extremes; p0, p2, p4 the pullbacks.
		d1, d2, d3 := p[1], p[3], p[5]
		if long {
			if !(d2 < d1 && d3 < d2) {
				return NoSignal(name)
			}
		} else if !(d2 > d1 && d3 > d2) {
			return NoSignal(name)
		}
		leg1, leg2 := math.Abs(d2-d1), math.Abs(d3-d2)
		if leg1 < 1.5*atr*price || leg2 <= 0 {
			return NoSignal(name)
		}
		// Symmetry: the two extensions comparable in size.
		if !within(leg2/leg1, 0.6, 1.7) {
			return NoSignal(name)
		}
		if math.Abs(price-d3) > 0.75*atr*price {
			return NoSignal(name)
		}
		if long {
			return mtfSignalToTarget(name, DirectionLong, price, atr, d3+(d1-d3)*0.618,
				fmt.Sprintf("three drives down complete, extension symmetry %.2f", leg2/leg1))
		}
		return mtfSignalToTarget(name, DirectionShort, price, atr, d3-(d3-d1)*0.618,
			fmt.Sprintf("three drives up complete, extension symmetry %.2f", leg2/leg1))
	}
}
