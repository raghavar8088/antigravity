package scalpers

import (
	"testing"
	"time"
)

// vcpSeries builds a base with the given pullback depths, in order.
//
// Each leg rallies to a new high and then retraces `depth` of it, so the
// sequence of depths IS the pattern under test. Volume is high through the
// early legs and low through the last, which is the dry-up the family requires.
func vcpSeries(depths []float64, breakout bool, lastVol float64) []Candle {
	c := make([]Candle, 0, 220)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	add := func(h, l, cl, v float64) {
		c = append(c, Candle{
			OpenTime: t0.Add(time.Duration(len(c)) * time.Hour),
			Open:     cl, High: h, Low: l, Close: cl, Volume: v,
		})
	}
	px := 100.0

	// A long, quiet uptrend so EMA50 sits below price and there is history.
	for i := 0; i < 90; i++ {
		px *= 1.004
		add(px*1.001, px*0.997, px, 1000)
	}

	// Each contraction rallies to a DISTINCT peak, falls to a DISTINCT trough,
	// then recovers. The peak and trough bars are made strictly extreme on
	// purpose: swingPoints requires a strict local maximum, so a flat-topped
	// turn where two bars tie produces no swing at all and the whole family
	// silently sees an empty structure.
	for _, d := range depths {
		for i := 0; i < 5; i++ { // rally
			px *= 1.008
			add(px*1.001, px*0.998, px, 1200)
		}
		peak := px * 1.02
		add(peak*1.004, px*0.999, peak, 1300) // the swing high
		px = peak

		bottom := peak * (1 - d)
		for i := 0; i < 4; i++ { // fall
			px -= (peak - bottom) / 5
			add(px*1.001, px*0.998, px, 1200)
		}
		add(px*1.001, bottom*0.996, bottom, 1250) // the swing low
		px = bottom

		for i := 0; i < 5; i++ { // recover, staying under the peak
			px *= 1.004
			add(px*1.001, px*0.998, px, 1200)
		}
	}

	pivot := 0.0
	for _, x := range c {
		if x.High > pivot {
			pivot = x.High
		}
	}
	if breakout {
		// Just THROUGH the pivot, which is where a VCP is entered — the chart
		// this was specified from buys 2016.0 against a 2010.0 pivot, +0.3%.
		// An entry far above the pivot is a different trade: the measured move
		// can already be behind it, and the family then correctly refuses.
		add(pivot*1.006, px*0.999, pivot*1.003, lastVol)
	} else {
		add(px*1.001, px*0.999, px, lastVol)
	}
	return c
}

func shrinking() []float64 { return []float64{0.12, 0.08, 0.05} }

// The real thing must be recognised, or every refusal test below passes on a
// family that never fires at all.
func TestPatVCP_RecognisesAShrinkingBaseBreakingOut(t *testing.T) {
	c := vcpSeries(shrinking(), true, 300) // low final volume = dry-up
	s := patVCP(true)("T", c, c[len(c)-1].Close)
	if s.Direction != DirectionLong {
		t.Fatalf("a shrinking base breaking out on dry volume was not recognised: %q", s.Reason)
	}
	if s.StopLoss >= s.TakeProfit {
		t.Errorf("levels inverted: stop %.4f target %.4f", s.StopLoss, s.TakeProfit)
	}
	// The stop must sit BELOW the entry for a long. A VCP whose stop is above
	// the entry is not a tightening base, it is a bookkeeping error.
	if px := c[len(c)-1].Close; s.StopLoss >= px {
		t.Errorf("stop %.4f is not below the entry %.4f", s.StopLoss, px)
	}
}

// Contractions that do NOT shrink are not a VCP. This is the single condition
// that separates the pattern from "a chart with three dips in it", so it is
// the one most worth pinning.
func TestPatVCP_RefusesContractionsThatDoNotTighten(t *testing.T) {
	for _, depths := range [][]float64{
		{0.05, 0.08, 0.12},   // widening
		{0.10, 0.10, 0.10},   // flat
		{0.10, 0.099, 0.098}, // shrinking by less than the threshold — noise
	} {
		c := vcpSeries(depths, true, 300)
		if s := patVCP(true)("T", c, c[len(c)-1].Close); s.Direction != DirectionNone {
			t.Errorf("depths %v were accepted as a contraction sequence: %s", depths, s.Reason)
		}
	}
}

// Volume must dry up. Without it the base is a pause with sellers still in it,
// which is the case Minervini's rule exists to exclude.
func TestPatVCP_RefusesWithoutVolumeDryUp(t *testing.T) {
	c := vcpSeries(shrinking(), true, 5000) // heavy final volume
	if s := patVCP(true)("T", c, c[len(c)-1].Close); s.Direction != DirectionNone {
		t.Errorf("signalled with volume ABOVE average: %s", s.Reason)
	}
}

// The breakout is the entry. Buying inside the base is buying a pattern that
// has not yet done the one thing it predicts, and it is where a detector that
// merely recognises shapes would fire.
func TestPatVCP_RefusesInsideTheBase(t *testing.T) {
	c := vcpSeries(shrinking(), false, 300)
	if s := patVCP(true)("T", c, c[len(c)-1].Close); s.Direction != DirectionNone {
		t.Errorf("signalled without breaking the pivot: %s", s.Reason)
	}
}

// A contraction in a downtrend is a downtrend getting quieter. The trend filter
// must refuse it.
func TestPatVCP_RefusesBelowTheTrend(t *testing.T) {
	c := vcpSeries(shrinking(), true, 300)
	// Drag the last bar under EMA50 while leaving the base intact.
	ema, ok := mtfEMA(c, 50)
	if !ok {
		t.Skip("no EMA on this series")
	}
	price := ema * 0.9
	if s := patVCP(true)("T", c, price); s.Direction != DirectionNone {
		t.Errorf("signalled with price below EMA50: %s", s.Reason)
	}
}

// A base needs room to have formed. Three contractions cannot be measured in a
// short window without calling noise a structure.
func TestPatVCP_RefusesShortHistory(t *testing.T) {
	short := mtfCandles(60, 100, 0.001, 0.003)
	for _, long := range []bool{true, false} {
		if s := patVCP(long)("T", short, 100); s.Direction != DirectionNone {
			t.Errorf("long=%v signalled on 60 candles", long)
		}
	}
}

// Depth is a FRACTION of the high it fell from, never a price distance.
//
// Without this a base that forms while the symbol doubles is not comparable
// with one that forms flat: the same "9% then 6% then 4%" shape measured in
// rupees looks like it is WIDENING as price rises, and the family would refuse
// every base in a strong uptrend — exactly the ones it exists to find.
func TestVCPDepths_AreFractionsNotPriceDistances(t *testing.T) {
	cheap := vcpSeries(shrinking(), true, 300)
	dear := make([]Candle, len(cheap))
	for i, x := range cheap {
		dear[i] = Candle{
			OpenTime: x.OpenTime,
			Open:     x.Open * 1000, High: x.High * 1000,
			Low: x.Low * 1000, Close: x.Close * 1000, Volume: x.Volume,
		}
	}
	a := patVCP(true)("T", cheap, cheap[len(cheap)-1].Close)
	b := patVCP(true)("T", dear, dear[len(dear)-1].Close)
	if a.Direction != b.Direction {
		t.Errorf("the same base scaled 1000x gave different verdicts: %v vs %v (%q / %q)",
			a.Direction, b.Direction, a.Reason, b.Reason)
	}
}

// A breakout that has already travelled its measured move must be refused.
//
// The VCP objective is the base's own depth projected from the pivot. If price
// is already past that, the trade being offered is the one AFTER the move, with
// the stop still down at the base — the worst risk:reward the pattern can
// produce, and precisely the entry a detector that only recognises shapes would
// take. mtfSignalToTarget refuses it because the target sits behind the price.
func TestPatVCP_RefusesABreakoutPastItsMeasuredMove(t *testing.T) {
	c := vcpSeries(shrinking(), true, 300)
	// Push the entry far above the pivot, leaving the base untouched.
	stretched := c[len(c)-1].Close * 1.10
	if s := patVCP(true)("T", c, stretched); s.Direction != DirectionNone {
		t.Errorf("chased a breakout already past its measured move: %s", s.Reason)
	}
}
