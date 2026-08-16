package delta

import (
	"math"
	"testing"
)

// The volatility scaler must never make a stop NARROWER than the strategy asked
// for.
//
// It exists to push a stop OUT of the noise. Applying the measured distance
// unconditionally also pulled stops IN on quiet symbols, which is the same
// mistake pointing the other way: it puts the stop deeper into the noise.
//
// The live case: ARCUSD printed a p90 one-minute range of 0.014%, so 2x gave a
// 0.028% stop — 2.1 ticks on a contract with 65 ticks of room. The grid gate
// refused it and the symbol looked broken.
func TestVolScaledLevels_NeverNarrowsTheStop(t *testing.T) {
	const entry = 0.07209041 // ARCUSD
	// Strategy asked for 0.9% with a 3:1 target.
	stop, target := entry*(1-0.009), entry*(1+0.027)

	// Measured noise is far SMALLER than the strategy's stop.
	gotStop, gotTarget := volScaledLevels(entry, stop, target, 0.00028, true)

	if gotStop != stop || gotTarget != target {
		t.Errorf("a tiny volatility reading rewrote the levels: stop %.8f->%.8f target %.8f->%.8f;\n"+
			"the scaler is a floor, not a replacement", stop, gotStop, target, gotTarget)
	}
}

// The behaviour it was built for must survive: a stop INSIDE the noise is
// widened. TSTUSD ran a 0.60% stop against a 1.13% median minute range and
// closed 9 of 9 at SL.
func TestVolScaledLevels_StillWidensAStopInsideTheNoise(t *testing.T) {
	const entry = 100.0
	stop, target := entry*(1-0.006), entry*(1+0.018) // 0.6% stop, 3:1

	gotStop, gotTarget := volScaledLevels(entry, stop, target, 0.0226, true) // 2x p90 = 2.26%

	wantStop := entry * (1 - 0.0226)
	if math.Abs(gotStop-wantStop) > 1e-9 {
		t.Errorf("stop %.6f, want %.6f — the widening this type exists for stopped working", gotStop, wantStop)
	}
	// And the reward:risk the strategy chose is preserved.
	gotRR := (gotTarget - entry) / (entry - gotStop)
	if math.Abs(gotRR-3.0) > 1e-6 {
		t.Errorf("reward:risk became %.3f, want the strategy's 3.0", gotRR)
	}
}

// Both directions, and the equal case, which must not thrash the levels.
func TestVolScaledLevels_ShortAndTheEqualCase(t *testing.T) {
	const entry = 100.0
	// SHORT: stop above, target below.
	stop, target := entry*(1+0.006), entry*(1-0.018)
	gotStop, _ := volScaledLevels(entry, stop, target, 0.0226, false)
	if gotStop <= entry {
		t.Errorf("short stop %.4f is not above the entry", gotStop)
	}
	if want := entry * (1 + 0.0226); math.Abs(gotStop-want) > 1e-9 {
		t.Errorf("short stop %.6f, want %.6f", gotStop, want)
	}

	// Exactly equal: no change, and specifically no drift from recomputing.
	stop2, target2 := entry*(1-0.02), entry*(1+0.06)
	gs, gt := volScaledLevels(entry, stop2, target2, 0.02, true)
	if gs != stop2 || gt != target2 {
		t.Errorf("an equal reading rewrote the levels: %.8f/%.8f -> %.8f/%.8f", stop2, target2, gs, gt)
	}
}

// A stop that survives the scaler must still be judged by the grid gate at the
// width that will actually be sent — the ordering the bridge uses.
func TestVolScaledLevels_ArcusdWouldNowClearTheGrid(t *testing.T) {
	const entry, tick = 0.07209041, 1e-05
	reg := &PerpRegistry{bySymbol: map[string]PerpProduct{
		"ARCUSD": {Symbol: "ARCUSD", MarkPrice: entry, TickSize: tick, ContractValue: 1},
	}}

	stop, target := entry*(1-0.009), entry*(1+0.027)

	// Before: the measured 0.028% replaced the strategy's stop.
	narrow := entry * (1 - 0.00028)
	if ticks, reason := stopGridTicks(reg, "ARCUSD", entry, narrow); reason == "" {
		t.Fatalf("premise wrong: a 0.028%% stop measured %.1f ticks and was NOT refused", ticks)
	}

	// After: the strategy's own stop is kept and clears the gate.
	gotStop, _ := volScaledLevels(entry, stop, target, 0.00028, true)
	ticks, reason := stopGridTicks(reg, "ARCUSD", entry, gotStop)
	if reason != "" {
		t.Errorf("ARCUSD still refused at %.1f ticks: %s", ticks, reason)
	}
}
