package scalpers

import "testing"

// The scalp desk shipped running BuildScalp100 — 100 of the 151 strategies
// registered in this application. The other 51 were built and never run, so the
// hunt could not have found them however good they were.

func TestBuildHuntPack_IncludesEveryRegisteredPack(t *testing.T) {
	pack := BuildHuntPack()

	if len(pack) <= len(BuildScalp100()) {
		t.Fatalf("hunt pack has %d strategies, no more than Scalp100's %d — the extra packs are missing",
			len(pack), len(BuildScalp100()))
	}

	names := make(map[string]bool, len(pack))
	for _, e := range pack {
		names[e.Name] = true
	}

	// Every strategy from every source pack must be reachable.
	for _, src := range []struct {
		name    string
		entries []RegistryEntry
	}{
		{"Scalp100", BuildScalp100()},
		{"Delta20", BuildDelta20Pack()},
		{"Curated", BuildCuratedScalpers()},
	} {
		for _, e := range src.entries {
			if !names[e.Name] {
				t.Errorf("%s strategy %q is missing from the hunt pack", src.name, e.Name)
			}
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
