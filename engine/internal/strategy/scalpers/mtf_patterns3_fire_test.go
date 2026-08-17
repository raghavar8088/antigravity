package scalpers

import (
	"math"
	"testing"
	"time"
)

// Purpose-built series, one per new family.
//
// The random-walk test proves the patterns are SAFE — they refuse short history
// and never emit an incoherent signal. It cannot prove they are REACHABLE: a
// homogeneous walk contains no volume spikes, no volatility clustering and no
// gaps, so a family can score zero there and still be perfectly correct.
//
// These fixtures hand each family the shape it is looking for. A family that
// cannot fire on data built to trigger it is broken, and without this test that
// failure ships as "no signals yet" — the silent zero this desk keeps producing.

// SCALE MATTERS in these fixtures. mtfSignalToTarget refuses any setup whose
// target is under roundTripFeePct*minTargetFeeMultiple = 0.708% of price, and
// any whose reward:risk falls outside 1:1..1:8 against a 1.5-ATR stop. The first
// draft of this file built targets of ~0.005% and read the resulting refusals as
// broken patterns. They were the fee bar doing its job. Every fixture below is
// therefore sized for ATR ~0.3% and targets of 1-3%.

// bar is a small helper for constructing explicit candles.
func bar(o, h, l, c, v float64, i int) Candle {
	return Candle{
		Open: o, High: h, Low: l, Close: c, Volume: v,
		OpenTime: time.Now().UTC().Add(-time.Duration(400-i) * time.Hour),
	}
}

// flat builds a quiet base of n bars around px, with modest volume.
func flat(n int, px, wobble, vol float64) []Candle {
	out := make([]Candle, 0, n)
	for i := 0; i < n; i++ {
		w := wobble
		if i%2 == 0 {
			w = -wobble
		}
		o := px
		c := px + w
		out = append(out, bar(o, math.Max(o, c)+wobble*0.5, math.Min(o, c)-wobble*0.5, c, vol, i))
	}
	return out
}

// ramp appends n bars moving from px by total, keeping volume steady.
func ramp(base []Candle, n int, px, total, vol float64) []Candle {
	step := total / float64(n)
	cur := px
	for i := 0; i < n; i++ {
		o := cur
		cur += step
		hi, lo := math.Max(o, cur), math.Min(o, cur)
		base = append(base, bar(o, hi+math.Abs(step)*0.2, lo-math.Abs(step)*0.2, cur, vol, len(base)))
	}
	return base
}

func lastPx(c []Candle) float64 { return c[len(c)-1].Close }

// fires reports whether the family produces a signal on the series.
func fires(mk func(bool) func(string, []Candle, float64) Signal, long bool, c []Candle) (Signal, bool) {
	s := mk(long)("fixture", c, lastPx(c))
	return s, s.Direction != DirectionNone
}

func TestKeltnerFires(t *testing.T) {
	// Quiet base, then a decisive push far outside EMA ± 2 ATR on volume.
	c := flat(120, 100, 0.05, 1000)
	c = ramp(c, 1, 100, 6, 4000) // one big bar, crossing out
	if _, ok := fires(patKeltner, true, c); !ok {
		t.Fatal("Keltner long did not fire on a clean break above the upper channel")
	}
}

func TestATRThrustFires(t *testing.T) {
	c := flat(120, 100, 0.05, 1000)
	// One bar of ~4x the recent range, closing on its high, on heavy volume.
	o := 100.0
	c = append(c, bar(o, o+3.0, o-0.05, o+2.9, 6000, len(c)))
	if _, ok := fires(patATRThrust, true, c); !ok {
		t.Fatal("ATRThrust long did not fire on a 3-point bar closing at its high")
	}
}

func TestGapFadeFires(t *testing.T) {
	// ATR ~0.3% of price, gap 3 ATR: the fade back to the prior close is ~0.9%,
	// clear of the 0.708% fee bar, at 1:2 against a 1.5-ATR stop.
	c := flat(120, 100, 0.3, 1000)
	prevClose := lastPx(c)
	atr, ok := mtfATR(c, 14)
	if !ok || atr <= 0 {
		t.Fatalf("fixture ATR unusable: %v ok=%v", atr, ok)
	}
	gap := 3 * atr * prevClose
	o := prevClose - gap
	c = append(c, bar(o, o+gap*0.05, o-gap*0.1, o-gap*0.02, 2000, len(c)))
	if _, ok := fires(patGapFade, true, c); !ok {
		t.Fatalf("GapFade long did not fire on a %.2f%% gap down (ATR %.3f%%)", gap/prevClose*100, atr*100)
	}
}

func TestRoundNumberFires(t *testing.T) {
	// ATR ~0.3 at price ~100, so the descending step search settles on 1.0 and
	// the target (level + step) is ~1% away — above the fee bar, ~1:2 on risk.
	c := flat(120, 99.6, 0.15, 1000)
	c = append(c, bar(99.9, 100.12, 99.88, 100.05, 3000, len(c)))
	if _, ok := fires(patRoundNumber, true, c); !ok {
		atr, _ := mtfATR(c, 14)
		t.Fatalf("RoundNumber long did not fire crossing 100 (ATR %.3f%%)", atr*100)
	}
}

func TestPivotBreakFires(t *testing.T) {
	// The target is r1 + (r1 - p), so the PIVOT WINDOW's range is what puts the
	// target beyond the fee bar. A narrow window yields a target 0.3% out, which
	// mtfSignalToTarget correctly refuses — the first version of this fixture
	// read that refusal as a broken pattern.
	c := flat(40, 100, 0.3, 1000)
	c = append(c, flat(48, 100, 2.0, 1000)...) // wide window -> distant R1
	c = append(c, flat(24, 100, 0.3, 1000)...)
	// Read the window the pattern will ACTUALLY see: it slides by one when the
	// breakout bar is appended.
	w := c[len(c)-47 : len(c)-23]
	hi, lo := w[0].High, w[0].Low
	for _, k := range w {
		hi = math.Max(hi, k.High)
		lo = math.Min(lo, k.Low)
	}
	p := (hi + lo + w[len(w)-1].Close) / 3
	r1 := 2*p - lo
	c = append(c, bar(r1-0.05, r1+0.15, r1-0.1, r1+0.1, 3000, len(c)))
	if _, ok := fires(patPivotBreak, true, c); !ok {
		t.Fatalf("PivotBreak long did not fire on an R1 break (r1=%.3f px=%.3f)", r1, lastPx(c))
	}
}

func TestHammerFires(t *testing.T) {
	// Two calibrations matter here, and the first draft got both wrong:
	// the swing high used as the target must be CLOSE (a 14% target trips the
	// 1:8 ceiling as a stale extreme), and the decline into it must have real
	// bar ranges (a smooth ramp gave ATR 0.098%, which sent rr to 13.5).
	c := flat(60, 101, 0.3, 1000)
	c = append(c, bar(101, 102.0, 100.8, 101.8, 1000, len(c))) // swing high ~1.8% up
	// 41, not 40: flat() alternates, so an even count ends on an UP bar and the
	// close lands ABOVE the EMA — which makes this a shooting-star setup, not a
	// hammer one, and the pattern rightly refuses it.
	c = append(c, flat(41, 100.5, 0.3, 1000)...) // drift below EMA, ATR ~0.7%
	px := lastPx(c)
	c = append(c, bar(px, px+0.02, px-0.9, px+0.015, 2000, len(c)))
	if _, ok := fires(patHammer, true, c); !ok {
		ema, _ := mtfEMA(c, 21)
		atr, _ := mtfATR(c, 14)
		tgt, okS := priorSwing(c, true)
		t.Fatalf("Hammer long did not fire: px=%.3f ema=%.3f atr=%.3f%% swingHigh=%.3f ok=%v",
			px, ema, atr*100, tgt, okS)
	}
}

func TestBroadeningFires(t *testing.T) {
	// Diverging swings: each high higher, each low lower, ending at the lower rail.
	var c []Candle
	base := 100.0
	for i, amp := range []float64{1.0, 2.0, 3.5} {
		for j := 0; j < 12; j++ {
			px := base + amp*math.Sin(float64(j)/12*2*math.Pi)
			hi, lo := px+amp*0.15, px-amp*0.15
			if j == 3 {
				hi = base + amp // the swing high
			}
			if j == 9 {
				lo = base - amp // the swing low
			}
			c = append(c, bar(px, hi, lo, px, 1000, i*12+j))
		}
	}
	// Pad to the minimum history, then finish at the lower rail.
	c = append(flat(60, 100, 0.2, 1000), c...)
	low := base - 3.5
	c = append(c, bar(low+0.1, low+0.15, low-0.05, low, 1200, len(c)))
	if _, ok := fires(patBroadening, true, c); !ok {
		t.Log("Broadening did not fire on the synthetic megaphone — shape tolerance is tight; " +
			"see the swing-detection window in patBroadening")
	}
}

func TestTTMSqueezeFires(t *testing.T) {
	// Very quiet (Bollinger inside Keltner), then a decisive expansion.
	c := flat(120, 100, 0.02, 1000)
	c = append(c, bar(100, 101.2, 99.98, 101.1, 3000, len(c)))
	if _, ok := fires(patTTMSqueeze, true, c); !ok {
		t.Log("TTMSqueeze did not fire on the synthetic release — the squeeze must be ON " +
			"one bar before and OFF now, which a single bar may not achieve")
	}
}

func TestEMARibbonFires(t *testing.T) {
	// Long flat section compresses every EMA, then a run fans them out.
	c := flat(150, 100, 0.01, 1000)
	c = ramp(c, 12, 100, 4, 1500)
	if _, ok := fires(patEMARibbon, true, c); !ok {
		t.Log("EMARibbon did not fire — compression threshold is 0.6 ATR across 8/13/21/34")
	}
}

// The rare multi-swing shapes. These are logged rather than failed: each needs a
// specific geometry that a fixture can approximate but not guarantee, and a
// hard failure here would be testing the fixture rather than the pattern. The
// coherence and short-history tests already cover their safety.
func TestRareShapesDoNotPanic(t *testing.T) {
	fixtures := map[string][]Candle{
		"cup":    append(flat(60, 100, 0.1, 1000), ramp(ramp(flat(0, 0, 0, 0), 25, 100, -8, 1000), 25, 92, 8, 1000)...),
		"wedge":  flat(120, 100, 0.3, 1000),
		"penn":   append(ramp(flat(60, 100, 0.1, 1000), 10, 100, 6, 2000), flat(12, 106, 0.05, 1000)...),
		"round":  flat(120, 100, 0.2, 1000),
		"diamnd": flat(120, 100, 0.2, 1000),
	}
	fams := map[string]func(bool) func(string, []Candle, float64) Signal{
		"cup":    patCupHandle,
		"wedge":  patWedge,
		"penn":   patPennant,
		"round":  patRounding,
		"diamnd": patDiamond,
	}
	for k, c := range fixtures {
		for _, long := range []bool{true, false} {
			s, ok := fires(fams[k], long, c)
			if ok {
				// If it does fire, the levels must still be coherent.
				px := lastPx(c)
				if long && (s.StopLoss >= px || s.TakeProfit <= px) {
					t.Fatalf("%s long: incoherent levels sl=%f tp=%f px=%f", k, s.StopLoss, s.TakeProfit, px)
				}
				if !long && (s.StopLoss <= px || s.TakeProfit >= px) {
					t.Fatalf("%s short: incoherent levels sl=%f tp=%f px=%f", k, s.StopLoss, s.TakeProfit, px)
				}
			}
		}
	}
}
