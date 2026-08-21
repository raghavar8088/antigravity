package scalpers

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// randomWalk builds a noisy series with a seeded RNG so a failure reproduces.
func randomWalk(n int, start, volPct float64, seed int64) []Candle {
	r := rand.New(rand.NewSource(seed))
	out := make([]Candle, 0, n)
	px := start
	t := time.Now().UTC().Add(-time.Duration(n) * time.Hour)
	for i := 0; i < n; i++ {
		px *= 1 + (r.Float64()-0.5)*2*volPct
		o := px
		cl := px * (1 + (r.Float64()-0.5)*volPct)
		hi := math.Max(o, cl) * (1 + r.Float64()*volPct*0.5)
		lo := math.Min(o, cl) * (1 - r.Float64()*volPct*0.5)
		out = append(out, Candle{
			Open: o, High: hi, Low: lo, Close: cl,
			Volume: 800 + r.Float64()*800,
			// Hourly bars: enough spacing that any time-based logic behaves.
			OpenTime: t.Add(time.Duration(i) * time.Hour),
		})
	}
	return out
}

// newPatterns is every family added in mtf_patterns3.go.
func newPatterns() map[string]func(bool) func(string, []Candle, float64) Signal {
	return map[string]func(bool) func(string, []Candle, float64) Signal{
		"Wedge":             patWedge,
		"Pennant":           patPennant,
		"CupHandle":         patCupHandle,
		"Rounding":          patRounding,
		"Broadening":        patBroadening,
		"Diamond":           patDiamond,
		"Hammer":            patHammer,
		"Keltner":           patKeltner,
		"PriorSessionBreak": patPriorSessionBreak,
		"RoundNumber":       patRoundNumber,
		"TTMSqueeze":        patTTMSqueeze,
		"EMARibbon":         patEMARibbon,
		"ATRThrust":         patATRThrust,
		"PivotBreak":        patPivotBreak,
		"GapFade":           patGapFade,
	}
}

// A pattern must never panic, and must never signal on a series too short to
// support it. Short-history "detection" is the failure that makes a pattern
// look productive on a fresh symbol and lose money on it.
func TestNewPatternsRefuseShortHistory(t *testing.T) {
	for name, mk := range newPatterns() {
		for _, n := range []int{0, 1, 5, 20, 55} {
			c := randomWalk(n, 100, 0.01, 7)
			px := 100.0
			if n > 0 {
				px = c[len(c)-1].Close
			}
			for _, long := range []bool{true, false} {
				sig := mk(long)(name, c, px)
				if sig.Direction != DirectionNone {
					t.Errorf("%s(long=%v) signalled on %d candles — must refuse", name, long, n)
				}
			}
		}
	}
}

// Every pattern must survive a wide range of shapes without panicking and
// without producing a nonsensical signal: a stop on the wrong side of entry, or
// a target on the wrong side, is worse than no signal at all because the desk
// will happily trade it.
func TestNewPatternsProduceCoherentSignals(t *testing.T) {
	fired := map[string]int{}
	for name, mk := range newPatterns() {
		for seed := int64(1); seed <= 60; seed++ {
			for _, vol := range []float64{0.004, 0.012, 0.03} {
				c := randomWalk(200, 100, vol, seed)
				px := c[len(c)-1].Close
				for _, long := range []bool{true, false} {
					sig := mk(long)(name, c, px)
					if sig.Direction == DirectionNone {
						continue
					}
					fired[name]++
					if sig.StopLoss <= 0 || sig.TakeProfit <= 0 {
						t.Fatalf("%s: non-positive levels sl=%f tp=%f", name, sig.StopLoss, sig.TakeProfit)
					}
					if sig.Direction == DirectionLong {
						if sig.StopLoss >= px {
							t.Fatalf("%s LONG: stop %f is not below entry %f", name, sig.StopLoss, px)
						}
						if sig.TakeProfit <= px {
							t.Fatalf("%s LONG: target %f is not above entry %f", name, sig.TakeProfit, px)
						}
					} else {
						if sig.StopLoss <= px {
							t.Fatalf("%s SHORT: stop %f is not above entry %f", name, sig.StopLoss, px)
						}
						if sig.TakeProfit >= px {
							t.Fatalf("%s SHORT: target %f is not below entry %f", name, sig.TakeProfit, px)
						}
					}
				}
			}
		}
	}
	// Report rather than fail. Several of these shapes are genuinely rare and a
	// random walk is not obliged to contain a cup & handle; failing on that
	// would be testing the RNG. The count is printed so a family that fires
	// ZERO times across 360 series is visible to whoever reads the log.
	for name := range newPatterns() {
		t.Logf("%-20s fired %d times across 360 random series", name, fired[name])
	}
}

// The whole pack must build and evaluate. This is the check that would have
// caught a family registered with a nil constructor — 756 strategies that all
// look present and one that panics the first time its symbol ticks.
func TestFullMTFPackEvaluates(t *testing.T) {
	pack := BuildMTFPack()
	if len(pack) != 70*9*2 {
		t.Fatalf("pack size %d, want 70 families x 9 timeframes x 2 directions = %d", len(pack), 70*9*2)
	}
	ctx := MarketContext{
		Candles1m:  randomWalk(300, 100, 0.01, 11),
		Candles5m:  randomWalk(300, 100, 0.01, 12),
		Candles10m: randomWalk(300, 100, 0.01, 13),
		Candles15m: randomWalk(300, 100, 0.01, 14),
		Candles30m: randomWalk(300, 100, 0.01, 15),
		Candles45m: randomWalk(300, 100, 0.01, 16),
		Candles1h:  randomWalk(300, 100, 0.01, 17),
		Candles4h:  randomWalk(300, 100, 0.01, 18),
		Candles1d:  randomWalk(300, 100, 0.01, 19),
	}
	ctx.Price = ctx.Candles1h[len(ctx.Candles1h)-1].Close
	for _, e := range pack {
		if e.Strategy == nil {
			t.Fatalf("%s has a nil Strategy", e.Name)
		}
		_ = e.Strategy.Evaluate(ctx) // must not panic
	}
}

// 45m must be a real series, not a name that silently resolves to nothing.
func TestTF45mResolves(t *testing.T) {
	if TF45m.Step() != 45*time.Minute {
		t.Fatalf("TF45m.Step() = %v, want 45m", TF45m.Step())
	}
	ctx := MarketContext{Candles45m: randomWalk(TF45m.MinCandles()+5, 100, 0.01, 3)}
	c, ok := TF45m.CandlesFor(ctx)
	if !ok || len(c) == 0 {
		t.Fatalf("TF45m.CandlesFor returned ok=%v len=%d — the timeframe is registered but unfed", ok, len(c))
	}
	// And an empty context must NOT report ready.
	if _, ok := TF45m.CandlesFor(MarketContext{}); ok {
		t.Fatal("TF45m reports ready on an empty context")
	}
}
