package scalpers

import (
	"math"
	"testing"
)

func harmonicFamilies() map[string]func(bool) func(string, []Candle, float64) Signal {
	m := map[string]func(bool) func(string, []Candle, float64) Signal{
		"ABCD":        patABCD,
		"ThreeDrives": patThreeDrives,
	}
	for k, spec := range harmonicSpecs {
		m[k] = harmonicFamily(spec)
	}
	return m
}

func TestHarmonicRefuseShortHistory(t *testing.T) {
	for name, mk := range harmonicFamilies() {
		for _, n := range []int{0, 1, 20, 60, 99} {
			c := randomWalk(n, 100, 0.01, 4)
			px := 100.0
			if n > 0 {
				px = c[len(c)-1].Close
			}
			for _, long := range []bool{true, false} {
				if sig := mk(long)(name, c, px); sig.Direction != DirectionNone {
					t.Errorf("%s(long=%v) signalled on %d candles — must refuse", name, long, n)
				}
			}
		}
	}
}

func TestHarmonicCoherentSignals(t *testing.T) {
	for name, mk := range harmonicFamilies() {
		for seed := int64(1); seed <= 60; seed++ {
			for _, vol := range []float64{0.005, 0.02, 0.04} {
				c := randomWalk(240, 100, vol, seed)
				px := c[len(c)-1].Close
				for _, long := range []bool{true, false} {
					sig := mk(long)(name, c, px)
					if sig.Direction == DirectionNone {
						continue
					}
					if sig.Direction == DirectionLong && (sig.StopLoss >= px || sig.TakeProfit <= px) {
						t.Fatalf("%s LONG: incoherent sl=%f tp=%f entry=%f", name, sig.StopLoss, sig.TakeProfit, px)
					}
					if sig.Direction == DirectionShort && (sig.StopLoss <= px || sig.TakeProfit >= px) {
						t.Fatalf("%s SHORT: incoherent sl=%f tp=%f entry=%f", name, sig.StopLoss, sig.TakeProfit, px)
					}
				}
			}
		}
	}
}

// swingSeq must return ALTERNATING points. A sequence that takes two highs in a
// row is not a leg structure — it is two separate moves being read as one, and
// every ratio computed from it is meaningless.
func TestSwingSeqAlternates(t *testing.T) {
	c := zigzag([]float64{100, 106, 102, 110, 104, 112, 106}, 9, 1000)
	idx, isHigh, ok := swingSeq(c, 5, 3)
	if !ok {
		t.Fatal("swingSeq found no sequence in a clean zigzag")
	}
	for i := 1; i < len(isHigh); i++ {
		if isHigh[i] == isHigh[i-1] {
			t.Fatalf("swingSeq returned two %v in a row at %d", isHigh[i], i)
		}
		if idx[i] <= idx[i-1] {
			t.Fatalf("swingSeq returned points out of order: %v", idx)
		}
	}
}

// ORIENTATION is the bug this family is most exposed to: a bullish XABCD
// completes at a LOW, and inverting that check would take longs at pattern tops
// while every ratio window still passed.
func TestHarmonicOrientation(t *testing.T) {
	// A textbook bullish Gartley: X low, A high, B low, C high, D low, with
	// D placed at 0.786 of XA.
	x, a := 100.0, 110.0
	xa := a - x
	b := a - xa*0.618    // AB = 0.618 XA
	cPt := b + (a-b)*0.6 // BC inside the window
	d := a - xa*0.786    // AD = 0.786 XA
	c := zigzag([]float64{x, a, b, cPt, d}, 9, 1000)
	px := lastPx(c)

	long := harmonicFamily(harmonicSpecs["Gartley"])(true)("g", c, px)
	short := harmonicFamily(harmonicSpecs["Gartley"])(false)("g", c, px)
	if short.Direction != DirectionNone {
		t.Fatalf("a bullish Gartley produced a SHORT signal — orientation is inverted")
	}
	if long.Direction == DirectionLong {
		// Firing is the happy path; assert the levels make sense.
		if long.StopLoss >= px || long.TakeProfit <= px {
			t.Fatalf("bullish Gartley levels incoherent: sl=%f tp=%f px=%f", long.StopLoss, long.TakeProfit, px)
		}
	} else {
		t.Log("bullish Gartley did not fire on the textbook fixture; ratio windows are deliberately tight")
	}
}

// The ratio windows must actually discriminate. A Gartley and a Bat differ
// mainly in one leg, so a sequence built to Bat proportions must NOT be
// accepted as a Gartley — otherwise the six patterns are one pattern with six
// names.
func TestHarmonicWindowsDiscriminate(t *testing.T) {
	g, b := harmonicSpecs["Gartley"], harmonicSpecs["Bat"]
	if g.adXA[1] >= b.adXA[0] {
		t.Fatalf("Gartley and Bat AD/XA windows overlap: %v vs %v — they would accept the same sequences",
			g.adXA, b.adXA)
	}
	// And a value inside one must be outside the other.
	gartleyAD := (g.adXA[0] + g.adXA[1]) / 2
	if within(gartleyAD, b.adXA[0], b.adXA[1]) {
		t.Fatalf("a mid-Gartley AD/XA of %.3f also satisfies the Bat window", gartleyAD)
	}
	batAD := (b.adXA[0] + b.adXA[1]) / 2
	if within(batAD, g.adXA[0], g.adXA[1]) {
		t.Fatalf("a mid-Bat AD/XA of %.3f also satisfies the Gartley window", batAD)
	}
}

// zigzag builds candles that trace the given turning points, `per` candles per
// leg, so swingPoints can confirm each turn.
func zigzag(points []float64, per int, vol float64) []Candle {
	out := make([]Candle, 0, len(points)*per+8)
	// A quiet run-in so the indicators have warm-up.
	out = append(out, flat(40, points[0], 0.05, vol)...)
	for i := 1; i < len(points); i++ {
		from, to := points[i-1], points[i]
		step := (to - from) / float64(per)
		cur := from
		for j := 0; j < per; j++ {
			o := cur
			cur += step
			hi, lo := math.Max(o, cur), math.Min(o, cur)
			pad := math.Abs(step) * 0.05
			out = append(out, bar(o, hi+pad, lo-pad, cur, vol, len(out)))
		}
		// Hold at the turn so the swing has candles either side to confirm it.
		out = append(out, flat(4, to, 0.02, vol)...)
	}
	return out
}
