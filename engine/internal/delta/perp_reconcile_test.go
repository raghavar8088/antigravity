package delta

import "testing"

// Reconciliation must compare NET SIZE PER SYMBOL, not position count.
//
// The real case, found by an end-to-end check on 2026-08-02: the bridge held
// two ADAUSD positions from two strategies — long 1,511 and short 75 — and
// Delta reported one netted position of 1,436. Both were correct. The old
// count-based check called it a mismatch.
//
// That is worse than having no check. A control that fires during normal
// operation teaches its operator to ignore it, and this is the control meant to
// catch real contracts the process has forgotten about. In the 2026-08-01 audit
// window the equivalent alarm fired 38 times in 20 minutes, all noise.

func TestPerpNet_NettedSymbolIsNotAMismatch(t *testing.T) {
	// The exact live positions.
	mine := perpNetByProduct(map[string]*PerpLiveTrade{
		"ANTI_M1_DoubleTop_20bp_Short|ADAUSD": {ProductID: 27, Contracts: 1511, Side: "buy"},
		"ANTI_M1_InsideBar_V20_Long|ADAUSD":   {ProductID: 27, Contracts: 75, Side: "sell"},
	})
	if mine[27] != 1436 {
		t.Fatalf("bridge net = %d contracts, want 1436 (1511 long - 75 short)", mine[27])
	}
	venue := map[int]int{27: 1436} // what Delta actually reported
	if bad := perpNetMismatches(mine, venue); len(bad) != 0 {
		t.Errorf("two netting positions on one symbol reported as a mismatch on %v — "+
			"this is normal operation and the alarm must stay silent", bad)
	}
}

// The case this control exists for: contracts on the venue the bridge does not
// know about. It must be loud.
func TestPerpNet_ForgottenVenuePositionIsCaught(t *testing.T) {
	mine := perpNetByProduct(map[string]*PerpLiveTrade{
		"a|ADAUSD": {ProductID: 27, Contracts: 100, Side: "buy"},
	})
	if bad := perpNetMismatches(mine, map[int]int{27: 200}); len(bad) == 0 {
		t.Error("100 unaccounted contracts on the venue were reported as matched")
	}
	// A product the bridge has never heard of must also be caught — iterating
	// only the bridge's own products would miss exactly the dangerous case.
	if bad := perpNetMismatches(mine, map[int]int{27: 100, 14830: 500}); len(bad) != 1 || bad[0] != 14830 {
		t.Errorf("an entirely unknown venue position was not flagged: %v", bad)
	}
}

// And a position the bridge believes in that the venue has closed.
func TestPerpNet_PhantomBridgePositionIsCaught(t *testing.T) {
	mine := perpNetByProduct(map[string]*PerpLiveTrade{
		"a|AVAXUSD": {ProductID: 14830, Contracts: 50, Side: "buy"},
	})
	if bad := perpNetMismatches(mine, map[int]int{}); len(bad) == 0 {
		t.Error("a bridge position the venue has closed was reported as matched")
	}
}

// Same-side positions must sum, not overwrite.
func TestPerpNet_SameSidePositionsSum(t *testing.T) {
	mine := perpNetByProduct(map[string]*PerpLiveTrade{
		"a|ADAUSD": {ProductID: 27, Contracts: 300, Side: "buy"},
		"b|ADAUSD": {ProductID: 27, Contracts: 200, Side: "buy"},
	})
	if mine[27] != 500 {
		t.Fatalf("300 + 200 long summed to %d", mine[27])
	}
	if bad := perpNetMismatches(mine, map[int]int{27: 500}); len(bad) != 0 {
		t.Errorf("500 long did not reconcile against a venue position of 500: %v", bad)
	}
}

// Two strategies that exactly cancel leave the venue flat — and the bridge must
// agree that flat is correct rather than expecting a row.
func TestPerpNet_FullyCancellingPositionsReconcileToFlat(t *testing.T) {
	mine := perpNetByProduct(map[string]*PerpLiveTrade{
		"a|ADAUSD": {ProductID: 27, Contracts: 400, Side: "buy"},
		"b|ADAUSD": {ProductID: 27, Contracts: 400, Side: "sell"},
	})
	if mine[27] != 0 {
		t.Fatalf("equal and opposite positions netted to %d, want 0", mine[27])
	}
	if bad := perpNetMismatches(mine, map[int]int{}); len(bad) != 0 {
		t.Errorf("a flat net was reported as a mismatch against an empty venue: %v", bad)
	}
}
