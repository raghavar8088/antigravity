package delta

import (
	"math"
	"testing"
)

// The measured stop must replace the strategy's, and the reward:risk it asked
// for must survive.
//
// Those are two different claims: how wide the risk should be (measurably
// wrong — 0.60% against a 1.13% median minute range) and how much reward
// justifies it (untested, and the strategy's to make).
func TestVolScaledLevels_KeepsTheRatioAndWidensTheRisk(t *testing.T) {
	entry := 0.021710
	// The live geometry: 0.6% stop, 1.8% target, 1:3.
	stop := entry * (1 + 0.006)
	target := entry * (1 - 0.018)

	// TSTUSD measured: p90 ~2.02%, so 2x is ~4%.
	newStop, newTarget := volScaledLevels(entry, stop, target, 0.04, false)

	gotRisk := math.Abs(entry-newStop) / entry
	if math.Abs(gotRisk-0.04) > 1e-6 {
		t.Errorf("stop is %.4f%% of price, want 4%%", gotRisk*100)
	}
	gotRR := math.Abs(newTarget-entry) / math.Abs(entry-newStop)
	if math.Abs(gotRR-3) > 0.01 {
		t.Errorf("reward:risk became 1:%.2f, want the 1:3 the strategy asked for", gotRR)
	}
	// A short's stop must stay ABOVE entry and its target BELOW.
	if newStop <= entry || newTarget >= entry {
		t.Errorf("short levels inverted: entry %.6f stop %.6f target %.6f", entry, newStop, newTarget)
	}
}

func TestVolScaledLevels_LongDirection(t *testing.T) {
	entry := 100.0
	newStop, newTarget := volScaledLevels(entry, 99.4, 101.8, 0.02, true)
	if newStop >= entry || newTarget <= entry {
		t.Errorf("long levels inverted: stop %.4f target %.4f", newStop, newTarget)
	}
	if math.Abs(math.Abs(entry-newStop)/entry-0.02) > 1e-9 {
		t.Error("long stop is not at the measured fraction")
	}
}

// No estimate must leave the strategy's own levels alone. Substituting a
// default here would apply one symbol's volatility to another — the exact
// error a per-symbol measurement exists to avoid, since XANUSD needs ~1.5% and
// TSTUSD ~4.0%.
func TestVolScaledLevels_NoEstimateChangesNothing(t *testing.T) {
	stop, target := volScaledLevels(100, 99.4, 101.8, 0, true)
	if stop != 99.4 || target != 101.8 {
		t.Errorf("levels changed without an estimate: %.4f / %.4f", stop, target)
	}
	if s, tg := volScaledLevels(0, 99.4, 101.8, 0.02, true); s != 99.4 || tg != 101.8 {
		t.Errorf("a zero entry produced levels: %.4f / %.4f", s, tg)
	}
}

// A widened stop must clear the 20-tick grid gate that the narrow one failed.
// If it did not, the two mechanisms would fight and nothing would trade.
func TestVolScaledLevels_ClearsTheGridGate(t *testing.T) {
	reg := riskTestRegistry(t)
	entry := 0.17258828
	// The old geometry: 8 ticks — refused.
	narrow := entry - 8*0.00001
	if _, reason := stopGridTicks(reg, "ADAUSD", entry, narrow); reason == "" {
		t.Fatal("the narrow stop should still be refused; the gate has changed")
	}
	// Volatility-scaled at 2%.
	wide, _ := volScaledLevels(entry, narrow, entry*1.02, 0.02, true)
	if _, reason := stopGridTicks(reg, "ADAUSD", entry, wide); reason != "" {
		t.Errorf("a volatility-scaled stop was still refused by the grid gate: %s", reason)
	}
}
