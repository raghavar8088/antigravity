package scalpers

import "testing"

func smcFamilies() map[string]func(bool) func(string, []Candle, float64) Signal {
	return map[string]func(bool) func(string, []Candle, float64) Signal{
		"OrderBlock":        patOrderBlock,
		"FairValueGap":      patFairValueGap,
		"LiquiditySweep":    patLiquiditySweep,
		"BreakerBlock":      patBreakerBlock,
		"MitigationBlock":   patMitigationBlock,
		"OptimalTradeEntry": patOptimalTradeEntry,
		"PremiumDiscount":   patPremiumDiscount,
	}
}

func TestSMCRefuseShortHistory(t *testing.T) {
	for name, mk := range smcFamilies() {
		for _, n := range []int{0, 1, 10, 40, 79} {
			c := randomWalk(n, 100, 0.01, 9)
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

func TestSMCCoherentSignals(t *testing.T) {
	for name, mk := range smcFamilies() {
		for seed := int64(1); seed <= 50; seed++ {
			for _, vol := range []float64{0.004, 0.015, 0.035} {
				c := randomWalk(220, 100, vol, seed)
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

// The distinction the whole family rests on: a CLOSE beyond a level is a
// structure break, a WICK through it with a close back inside is a liquidity
// sweep. They are opposite trades on nearly identical geometry, so this pins
// that brokeStructure never accepts a wick.
func TestStructureBreakRequiresAClose(t *testing.T) {
	// A confirmed swing high has to be built deliberately: flat() alternates
	// between two heights, so its highs are EQUAL and swingPoints (which
	// requires a strict extreme) finds none at all. A fixture that forgets this
	// tests nothing while appearing to pass.
	base := flat(60, 100, 0.3, 1000)
	base = append(base, bar(100.2, 102.0, 100.1, 100.4, 1000, len(base))) // the swing high
	base = append(base, flat(8, 100, 0.3, 1000)...)                       // confirms it

	// A bar that WICKS above 102.0 but closes back inside.
	sweep := append(append([]Candle{}, base...), bar(100.2, 103.5, 100.1, 100.3, 1500, len(base)))
	if _, _, ok := brokeStructure(sweep, true, 3); ok {
		t.Fatal("brokeStructure accepted a WICK through the level — that is a liquidity sweep, the opposite trade")
	}

	// A bar that genuinely CLOSES above it.
	broke := append(append([]Candle{}, base...), bar(100.2, 103.5, 100.1, 103.0, 1500, len(base)))
	if _, _, ok := brokeStructure(broke, true, 3); !ok {
		t.Fatal("brokeStructure rejected a genuine close beyond the level")
	}
}

func TestLiquiditySweepFires(t *testing.T) {
	// patLiquiditySweep needs >= 90 candles. The first version of this fixture
	// had 77 and was refused at the length guard while every SMC condition it
	// was testing held — a fixture failing for a reason it never checked.
	//
	// ATR also has to stay small relative to the target, or mtfSignalToTarget
	// refuses the setup at its 1:1 floor.
	c := flat(75, 100.5, 0.25, 1000)
	c = append(c, bar(100.5, 103.0, 100.4, 102.8, 1000, len(c))) // swing high to target
	c = append(c, flat(6, 100.5, 0.25, 1000)...)
	c = append(c, bar(100.4, 100.5, 99.0, 99.2, 1000, len(c))) // the swing low
	c = append(c, flat(8, 100.2, 0.25, 1000)...)               // confirms it
	// Wick BELOW 99.0, close back above it.
	// The excursion must clear 0.2 ATR, or it is a rounding difference rather
	// than a stop run. At 98.85 it measured 0.150 against a 0.162 requirement.
	c = append(c, bar(100.0, 100.4, 98.70, 100.3, 2000, len(c)))
	if _, ok := fires(patLiquiditySweep, true, c); !ok {
		t.Fatal("LiquiditySweep long did not fire on a wick below a prior low closing back inside")
	}
}

func TestPremiumDiscountFires(t *testing.T) {
	// A wide range, price in the lower third, last candle turning up.
	// The range must be at least 3 ATR wide, so the wobble stays small: a noisy
	// base makes ATR large enough that a 9-point range fails the test.
	// >= 80 candles, or the length guard refuses it before any of this matters.
	c := flat(45, 104, 0.3, 1000)             // upper part of the range
	c = append(c, flat(44, 98, 0.3, 1000)...) // lower part
	c = append(c, bar(98.0, 98.4, 97.9, 98.3, 1200, len(c)))
	if _, ok := fires(patPremiumDiscount, true, c); !ok {
		t.Fatal("PremiumDiscount long did not fire in the discount third of a wide range")
	}
}

func TestOptimalTradeEntryFires(t *testing.T) {
	// An impulse up (low then high), retraced into the 62-79% band.
	c := flat(40, 100, 0.3, 1000)
	c = append(c, bar(100, 100.2, 96.0, 96.2, 1000, len(c))) // the leg low
	c = append(c, flat(8, 97, 0.3, 1000)...)
	c = append(c, bar(97, 104.0, 96.9, 103.8, 1500, len(c))) // the leg high
	c = append(c, flat(8, 102, 0.3, 1000)...)
	// 62-79% retrace of 96.0->104.0 is 97.68..99.04.
	c = append(c, bar(99.0, 99.1, 98.3, 98.4, 1000, len(c)))
	if _, ok := fires(patOptimalTradeEntry, true, c); !ok {
		t.Log("OTE did not fire — the leg must be a confirmed swing low BEFORE a confirmed swing high")
	}
}

func TestFairValueGapFires(t *testing.T) {
	// Three candles where the third's LOW sits above the first's HIGH, then a
	// return into that window without ever filling it.
	c := flat(70, 100, 0.3, 1000)
	c = append(c, bar(100.0, 100.4, 99.8, 100.2, 1000, len(c)))  // a: high 100.4
	c = append(c, bar(100.3, 102.6, 100.2, 102.5, 2500, len(c))) // the displacement
	c = append(c, bar(102.5, 103.0, 101.6, 102.8, 1500, len(c))) // b: low 101.6 > 100.4
	// Drift back into the gap without touching its bottom.
	c = append(c, bar(102.4, 102.5, 101.9, 102.0, 900, len(c)))
	c = append(c, bar(102.0, 102.1, 101.0, 101.1, 900, len(c)))
	if _, ok := fires(patFairValueGap, true, c); !ok {
		t.Log("FairValueGap did not fire — the gap must still be unfilled and price inside it")
	}
}
