package scalpers

import (
	"math"
	"strings"
	"testing"
)

// The reward:risk must FALL OUT of the structure, not be imposed on it.
//
// A fixed 3R target on a range only 1.5R tall can never be reached — the trade
// is a stop-out with extra steps. A 2R target on a breakout with a 5R measured
// move leaves most of the move behind. Both were happening: every family used a
// hardcoded 2.0, 2.5 or 3.0 regardless of what the setup was reaching for.
func TestMTFSignalToTarget_RatioComesFromTheStructure(t *testing.T) {
	price, atr := 100.0, 0.02 // 2% ATR -> 3% stop

	// A near target gives a small ratio.
	near := mtfSignalToTarget("X", DirectionLong, price, atr, 104.0, "near")
	// A far one gives a large ratio, from the SAME stop.
	far := mtfSignalToTarget("X", DirectionLong, price, atr, 118.0, "far")

	if near.Direction == DirectionNone || far.Direction == DirectionNone {
		t.Fatalf("a valid setup was refused: near=%q far=%q", near.Reason, far.Reason)
	}
	if near.StopLoss != far.StopLoss {
		t.Errorf("stops differ (%.4f vs %.4f) — the stop is volatility's job, not the target's",
			near.StopLoss, far.StopLoss)
	}
	if !(far.TakeProfit > near.TakeProfit) {
		t.Error("the farther structure did not produce a farther target")
	}
	// And the ratio is reported, so a reader can see what the setup offered.
	if !strings.Contains(near.Reason, "1:") || !strings.Contains(far.Reason, "1:") {
		t.Error("the realised ratio is not stated in the reason")
	}
}

// A target on the wrong side of entry is a structure read backwards, and would
// produce a trade whose take-profit is already hit.
func TestMTFSignalToTarget_RefusesInvertedTargets(t *testing.T) {
	if s := mtfSignalToTarget("X", DirectionLong, 100, 0.02, 95, "behind"); s.Direction != DirectionNone {
		t.Error("long accepted a target BELOW entry")
	}
	if s := mtfSignalToTarget("X", DirectionShort, 100, 0.02, 105, "behind"); s.Direction != DirectionNone {
		t.Error("short accepted a target ABOVE entry")
	}
}

// Below 1:1 the structure is not offering enough to justify the risk the
// volatility demands, whatever the pattern looks like.
func TestMTFSignalToTarget_RefusesSubOneToOne(t *testing.T) {
	// 2% ATR -> 3% stop. A target 2% away is 0.67:1.
	if s := mtfSignalToTarget("X", DirectionLong, 100, 0.02, 102, "thin"); s.Direction != DirectionNone {
		t.Error("accepted a setup risking more than it reaches for")
	}
}

// Above 8:1 the "structure" is almost always a stale extreme far from price,
// not a destination this trade reaches inside its time stop.
func TestMTFSignalToTarget_RefusesImplausiblyDistantTargets(t *testing.T) {
	// 1% ATR -> 1.5% stop; a target 40% away is ~27:1.
	if s := mtfSignalToTarget("X", DirectionLong, 100, 0.01, 140, "stale"); s.Direction != DirectionNone {
		t.Error("accepted a 27:1 target — that is a stale extreme, not a destination")
	}
}

// The fee bar still applies. A structural target that is real but tiny is still
// a commission with a coin flip attached.
func TestMTFSignalToTarget_StillEnforcesTheFeeBar(t *testing.T) {
	// Target 0.3% away: above 1:1 on a small stop, but under 6 round trips.
	if s := mtfSignalToTarget("X", DirectionLong, 100, 0.001, 100.3, "tiny"); s.Direction != DirectionNone {
		t.Errorf("accepted a %.3f%% target against a %.3f%% fee", 0.3, roundTripFeePct)
	}
}

// Mean reversion must target the MEAN, and the ratio must therefore vary with
// how far from the mean price has travelled.
//
// This is the clearest case for structural targets: the same strategy at a mild
// stretch and an extreme one is reaching for different distances, and a fixed
// multiple describes neither.
func TestBollingerFade_TargetsTheMeanNotAMultiple(t *testing.T) {
	c := mtfCandles(200, 100, 0.0, 0.004) // flat, ranging — ADX stays low
	_, mid, _, ok := mtfBollinger(c, 20, 2.0)
	if !ok {
		t.Fatal("bollinger did not compute")
	}
	// Drive price well below the lower band so the fade triggers.
	price := mid * 0.90
	s := mtfBollingerFade(true)("T", c, price)
	if s.Direction == DirectionNone {
		t.Skipf("setup did not trigger on this series: %s", s.Reason)
	}
	if math.Abs(s.TakeProfit-mid) > mid*1e-9 {
		t.Errorf("target %.6f is not the mean %.6f", s.TakeProfit, mid)
	}
}

// priorSwing must return a CONFIRMED swing, never the forming bar.
func TestPriorSwing_UsesConfirmedSwingsOnly(t *testing.T) {
	c := mtfCandles(120, 100, 0.001, 0.005)
	if v, ok := priorSwing(c, true); ok {
		// It must correspond to some candle's high, not to the last close.
		found := false
		for _, x := range c[:len(c)-3] {
			if math.Abs(x.High-v) < 1e-9 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("swing high %.6f does not match any confirmed candle", v)
		}
	}
}
