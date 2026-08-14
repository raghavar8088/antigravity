package scalpers

import (
	"strings"
	"testing"
)

// The scalp desk shipped running BuildScalp100 — 100 of the 151 strategies
// registered in this application. The other 51 were built and never run, so the
// hunt could not have found them however good they were.

// The desk trades exactly the packs BuildHuntPack registers, and no others.
//
// Rewritten 2026-08-14. It previously asserted that Scalp100, Delta20 and
// Curated were all reachable — correct while they were registered, and exactly
// backwards once they were removed at the owner's direction. A test that
// asserts the presence of packs the desk has deliberately retired would have to
// be deleted or ignored on every future roster change; asserting what IS
// registered survives that.
func TestBuildHuntPack_RegistersTheMTFPackOnly(t *testing.T) {
	pack := BuildHuntPack()
	mtf := BuildMTFPack()

	if len(pack) != len(mtf) {
		t.Fatalf("hunt pack has %d strategies, MTF pack has %d — something else is registered", len(pack), len(mtf))
	}

	names := make(map[string]bool, len(pack))
	for _, e := range pack {
		names[e.Name] = true
		if !strings.HasPrefix(e.Name, "MTF_") {
			t.Errorf("%s is registered but is not from the MTF pack", e.Name)
		}
	}
	for _, e := range mtf {
		if !names[e.Name] {
			t.Errorf("%s is in the MTF pack but not reachable from the hunt pack", e.Name)
		}
	}

	// The retired packs must still BUILD — they are the record of what was
	// tried, and re-registering one should be a single line rather than an
	// archaeology exercise.
	for _, b := range []func() []RegistryEntry{BuildScalp100, BuildDelta20Pack, BuildCuratedScalpers} {
		if len(b()) == 0 {
			t.Error("a retired pack no longer builds; it should remain in the tree, just unregistered")
		}
	}
}

// A duplicate name would create two accounts for one strategy, splitting its
// trade record in half and keeping BOTH halves under the gate's 200-trade
// minimum — the strategy would be permanently unpromotable through an
// accounting artefact rather than performance.
func TestBuildHuntPack_NamesAreUnique(t *testing.T) {
	seen := map[string]int{}
	for _, e := range BuildHuntPack() {
		seen[e.Name]++
	}
	for name, n := range seen {
		if n > 1 {
			t.Errorf("strategy %q appears %d times; its trade record would be split", name, n)
		}
	}
}

// Every entry must be usable: an unnamed or signal-less entry would occupy an
// account slot and never trade, quietly diluting the hunt.
func TestBuildHuntPack_EntriesAreUsable(t *testing.T) {
	for i, e := range BuildHuntPack() {
		if e.Name == "" {
			t.Errorf("entry %d has no name", i)
		}
		if e.Strategy == nil {
			t.Errorf("strategy %q has no Strategy implementation and can never trade", e.Name)
		}
	}
}

// Deterministic order keeps run-to-run comparisons meaningful.
func TestBuildHuntPack_IsDeterministic(t *testing.T) {
	a, b := BuildHuntPack(), BuildHuntPack()
	if len(a) != len(b) {
		t.Fatalf("pack size varies between calls: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Fatalf("pack order varies at %d: %q vs %q", i, a[i].Name, b[i].Name)
		}
	}
}
