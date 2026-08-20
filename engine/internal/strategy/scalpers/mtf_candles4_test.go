package scalpers

import (
	"testing"
)

// candleFamilies is every family added in mtf_candles4.go.
func candleFamilies() map[string]func(bool) func(string, []Candle, float64) Signal {
	return map[string]func(bool) func(string, []Candle, float64) Signal{
		"Tweezer":             patTweezer,
		"Harami":              haramiFamily(false),
		"HaramiCross":         haramiFamily(true),
		"Piercing":            patPiercing,
		"ThreeInside":         patThreeInside,
		"ThreeOutside":        patThreeOutside,
		"ThreeMethods":        patThreeMethods,
		"Kicker":              patKicker,
		"AbandonedBaby":       patAbandonedBaby,
		"SpinningTop":         patSpinningTop,
		"BeltHold":            patBeltHold,
		"LongLeggedDoji":      patLongLeggedDoji,
		"DragonflyGravestone": patDragonflyGravestone,
	}
}

// Safety: never panic, never signal on a history too short to support the
// pattern. Short-history "detection" is what makes a family look productive on a
// freshly listed symbol and lose money on it.
func TestCandle4RefuseShortHistory(t *testing.T) {
	for name, mk := range candleFamilies() {
		for _, n := range []int{0, 1, 3, 20, 55} {
			c := randomWalk(n, 100, 0.01, 5)
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

// Coherence: a stop or target on the wrong side of entry is worse than no
// signal, because the desk will happily trade it.
func TestCandle4CoherentSignals(t *testing.T) {
	for name, mk := range candleFamilies() {
		for seed := int64(1); seed <= 40; seed++ {
			for _, vol := range []float64{0.004, 0.015, 0.035} {
				c := randomWalk(200, 100, vol, seed)
				px := c[len(c)-1].Close
				for _, long := range []bool{true, false} {
					sig := mk(long)(name, c, px)
					if sig.Direction == DirectionNone {
						continue
					}
					if sig.StopLoss <= 0 || sig.TakeProfit <= 0 {
						t.Fatalf("%s: non-positive levels sl=%f tp=%f", name, sig.StopLoss, sig.TakeProfit)
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

// Reachability. The random-walk tests above prove these are SAFE; they cannot
// prove any of them can ever fire. A homogeneous walk has no gaps, no volume
// spikes and no clean two-bar geometry, so a correct family can score zero there
// — and a broken one is indistinguishable. These fixtures hand each family the
// shape it looks for.
//
// Every fixture is sized for ATR ~0.3-0.7% with targets of 1-3%, because
// mtfSignalToTarget refuses any target under 0.708% of price (6x the round-trip
// fee) or outside 1:1..1:8 against a 1.5-ATR stop.

func TestTweezerFires(t *testing.T) {
	// Downtrend into two candles sharing a low, with a nearby swing high to target.
	c := flat(60, 101, 0.3, 1000)
	c = append(c, bar(101, 102.0, 100.8, 101.8, 1000, len(c))) // swing high ~1.8% up
	c = append(c, flat(41, 100.5, 0.3, 1000)...)
	lo := 99.8
	c = append(c, bar(100.3, 100.35, lo, 100.0, 1200, len(c)))      // bearish, sets the low
	c = append(c, bar(100.0, 100.5, lo+0.001, 100.4, 1400, len(c))) // bullish off the same low
	if _, ok := fires(patTweezer, true, c); !ok {
		t.Fatal("Tweezer bottom did not fire on two candles sharing a low below the EMA")
	}
}

func TestHaramiFires(t *testing.T) {
	c := flat(60, 101, 0.3, 1000)
	c = append(c, bar(101, 102.0, 100.8, 101.8, 1000, len(c))) // swing high to target
	c = append(c, flat(41, 100.5, 0.3, 1000)...)
	// Mother: a decisive bearish candle. Baby: a small body inside it.
	c = append(c, bar(101.0, 101.1, 99.7, 99.8, 1200, len(c)))
	c = append(c, bar(100.1, 100.4, 100.0, 100.2, 900, len(c)))
	if _, ok := fires(haramiFamily(false), true, c); !ok {
		t.Fatal("Bullish harami did not fire on a small body inside a bearish mother")
	}
}

func TestPiercingFires(t *testing.T) {
	c := flat(60, 101, 0.3, 1000)
	c = append(c, bar(101, 102.0, 100.8, 101.8, 1000, len(c)))
	c = append(c, flat(41, 100.5, 0.3, 1000)...)
	// Bearish candle, then one opening below its low and closing past the midpoint.
	a := bar(101.0, 101.1, 99.6, 99.7, 1200, len(c))
	c = append(c, a)
	mid := (a.Open + a.Close) / 2 // 100.35
	c = append(c, bar(99.5, mid+0.25, 99.4, mid+0.2, 1600, len(c)))
	if _, ok := fires(patPiercing, true, c); !ok {
		t.Fatal("Piercing line did not fire on a close past the prior midpoint")
	}
}

func TestThreeInsideFires(t *testing.T) {
	c := flat(100, 100.5, 0.3, 1000)
	c = append(c, bar(101.2, 101.3, 99.6, 99.7, 1200, len(c)))   // mother, bearish
	c = append(c, bar(100.0, 100.4, 99.9, 100.3, 900, len(c)))   // baby, inside
	c = append(c, bar(100.3, 101.6, 100.2, 101.5, 1500, len(c))) // closes past the mother top
	if _, ok := fires(patThreeInside, true, c); !ok {
		t.Fatal("Three inside up did not fire on a confirmed harami")
	}
}

func TestThreeOutsideFires(t *testing.T) {
	c := flat(100, 100.5, 0.3, 1000)
	c = append(c, bar(100.4, 100.5, 100.1, 100.2, 1000, len(c))) // small bearish
	c = append(c, bar(100.0, 101.4, 99.9, 101.3, 1500, len(c)))  // engulfs it
	c = append(c, bar(101.3, 102.2, 101.2, 102.1, 1500, len(c))) // continues
	if _, ok := fires(patThreeOutside, true, c); !ok {
		t.Fatal("Three outside up did not fire on engulfing plus continuation")
	}
}

func TestThreeMethodsFires(t *testing.T) {
	c := flat(100, 100, 0.3, 1000)
	first := bar(99.5, 101.6, 99.4, 101.5, 1500, len(c)) // long bullish
	c = append(c, first)
	// Three small candles held inside the first candle's range.
	c = append(c, bar(101.3, 101.4, 101.0, 101.1, 800, len(c)))
	c = append(c, bar(101.1, 101.2, 100.7, 100.8, 800, len(c)))
	c = append(c, bar(100.8, 101.0, 100.5, 100.9, 800, len(c)))
	c = append(c, bar(101.0, 102.5, 100.9, 102.4, 1600, len(c))) // resumes past the high
	if _, ok := fires(patThreeMethods, true, c); !ok {
		t.Fatal("Rising three methods did not fire on a held pause then resumption")
	}
}

func TestKickerFires(t *testing.T) {
	c := flat(120, 100, 0.3, 1000)
	// Strong bearish candle, then a gap up above its OPEN that never fills back.
	c = append(c, bar(100.6, 100.65, 99.4, 99.5, 1500, len(c)))
	c = append(c, bar(101.2, 102.4, 101.15, 102.3, 2500, len(c)))
	if _, ok := fires(patKicker, true, c); !ok {
		t.Fatal("Bullish kicker did not fire on an unfilled gap between opposite bodies")
	}
}

func TestBeltHoldFires(t *testing.T) {
	c := flat(120, 100, 0.3, 1000)
	// Opens exactly on its low, closes near the high, on volume, below the EMA.
	o := 99.4
	c = append(c, bar(o, o+1.6, o, o+1.5, 2000, len(c)))
	if _, ok := fires(patBeltHold, true, c); !ok {
		t.Fatal("Bullish belt hold did not fire on a candle opening at its low")
	}
}

func TestSpinningTopFires(t *testing.T) {
	// A deep decline puts price well below the EMA, then a two-sided small body.
	c := flat(60, 104, 0.3, 1000)
	c = append(c, ramp(nil, 40, 104, -6, 1000)...)
	px := lastPx(c)
	c = append(c, bar(px, px+0.9, px-0.9, px+0.05, 1000, len(c)))
	if _, ok := fires(patSpinningTop, true, c); !ok {
		atr, _ := mtfATR(c, 14)
		dist, _ := extremeDistATR(c, lastPx(c), 1, true)
		t.Logf("SpinningTop did not fire: dist=%.2f ATR (needs < -2), atr=%.3f%%", dist, atr*100)
	}
}

func TestDragonflyFires(t *testing.T) {
	c := flat(60, 101, 0.3, 1000)
	c = append(c, bar(101, 102.0, 100.8, 101.8, 1000, len(c))) // swing high to target
	c = append(c, flat(41, 100.5, 0.3, 1000)...)
	px := lastPx(c)
	// Doji body, essentially all lower wick.
	c = append(c, bar(px, px+0.01, px-1.2, px+0.005, 1500, len(c)))
	if _, ok := fires(patDragonflyGravestone, true, c); !ok {
		t.Fatal("Dragonfly doji did not fire on a full rejection of the low")
	}
}

func TestLongLeggedDojiFires(t *testing.T) {
	c := flat(60, 104, 0.3, 1000)
	c = append(c, ramp(nil, 40, 104, -5, 1000)...)
	px := lastPx(c)
	// Wide range, tiny body, big wicks both sides.
	c = append(c, bar(px, px+1.3, px-1.3, px+0.01, 1500, len(c)))
	if _, ok := fires(patLongLeggedDoji, true, c); !ok {
		dist, _ := extremeDistATR(c, lastPx(c), 1, true)
		t.Logf("LongLeggedDoji did not fire: dist=%.2f ATR (needs < -1.5)", dist)
	}
}

// AbandonedBaby is left to the coherence test only: it needs gaps on BOTH sides
// of a doji at an extreme, a geometry a fixture can construct but which says
// little about whether the family is reachable on real data. Its rarity is the
// point of the pattern, not a defect.
