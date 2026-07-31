package main

import (
	"strings"
	"testing"

	"antigravity-engine/internal/options"
)

// The allow-list is the only thing standing between a paper strategy and real
// money on the current configuration: the eligibility gate exists but is not
// enforced (LIVE_ENGINE_ENFORCE_GATE defaults false), so a name on this list can
// place a live order regardless of what its record says.
//
// It is also a list of long, hand-typed strings compared by equality. A typo
// does not fail — it permits a strategy that does not exist while the roster
// reads as if it were widened. These tests exist so that failure is loud.

// Every default name must match a strategy the buying engine actually
// registers. This is the check that catches a rename or a mistyped entry.
func TestDefaultLiveStrategies_AllResolveToRealStrategies(t *testing.T) {
	known := map[string]bool{}
	for _, d := range options.BuildStrategies() {
		known[d.Name] = true
	}
	if len(known) == 0 {
		t.Fatal("BuildStrategies returned nothing; the check would pass vacuously")
	}

	for _, name := range defaultLiveStrategyNames {
		if !known[name] {
			t.Errorf("allow-listed %q matches no registered strategy — it permits nothing, "+
				"but the roster would show the list as widened", name)
		}
	}
}

// A mirror must never reach the live allow-list by accident. ANTI_ strategies
// are a measurement device — they exist to find out whether an original has a
// negative gross edge — and were never meant to spend real capital.
func TestDefaultLiveStrategies_ContainNoMirrors(t *testing.T) {
	for _, name := range defaultLiveStrategyNames {
		if options.IsAnti(name) {
			t.Errorf("%q is a mirror; anti-strategies are for measurement, not real money", name)
		}
	}
}

// Duplicates would misreport the roster size and make "9 strategies allowed"
// mean something other than nine distinct strategies.
func TestDefaultLiveStrategies_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range defaultLiveStrategyNames {
		if seen[name] {
			t.Errorf("duplicate allow-list entry %q", name)
		}
		seen[name] = true
	}
}

// An empty allow-list is NOT "allow nothing" — strategyAllowedLocked treats a
// nil/empty map as legacy allow-all. Emptying this slice would therefore open
// the desk to every strategy rather than close it, which is the opposite of what
// deleting entries looks like it should do.
func TestDefaultLiveStrategies_IsNeverEmpty(t *testing.T) {
	if len(defaultLiveStrategies()) == 0 {
		t.Fatal("empty allow-list: the bridge reads that as allow-ALL, not allow-none")
	}
}

// The env override must win, so an operator can narrow the list without a
// deploy — and must not silently fall back to the wider default on a value that
// is merely untidy.
func TestDefaultLiveStrategies_EnvOverrideWins(t *testing.T) {
	t.Setenv("LIVE_ENGINE_STRATEGIES", " Foo_One , Foo_Two ")
	got := defaultLiveStrategies()
	if len(got) != 2 || got[0] != "Foo_One" || got[1] != "Foo_Two" {
		t.Fatalf("env override gave %v; want [Foo_One Foo_Two] with whitespace trimmed", got)
	}
}

// A blank or comma-only override must fall back to the curated default rather
// than producing an empty list, which the bridge would read as allow-all.
func TestDefaultLiveStrategies_BlankOverrideFallsBack(t *testing.T) {
	for _, raw := range []string{"", "   ", ",", " , , "} {
		t.Setenv("LIVE_ENGINE_STRATEGIES", raw)
		got := defaultLiveStrategies()
		if len(got) != len(defaultLiveStrategyNames) {
			t.Errorf("LIVE_ENGINE_STRATEGIES=%q produced %d names; want the %d-name default",
				raw, len(got), len(defaultLiveStrategyNames))
		}
	}
}

// The four names added on 2026-07-31 must actually be present. Pinned by name
// because the request was specific, and a silent revert would look identical to
// a working deploy from the outside.
func TestDefaultLiveStrategies_IncludesTheStrategiesAddedForTheOwner(t *testing.T) {
	want := []string{
		"Swing_PutBuy_BreakdownTrendBear_960m",
		"Intraday_CallBuy_TripleBull_180m",
		"Swing_PutBuy_ATRExpandBear_720m",
		"Intraday_PutBuy_BreakdownTrendBear_240m",
	}
	have := strings.Join(defaultLiveStrategyNames, "\n")
	for _, w := range want {
		if !strings.Contains(have, w) {
			t.Errorf("%q is missing from the live allow-list", w)
		}
	}
	if len(defaultLiveStrategyNames) != 9 {
		t.Errorf("allow-list holds %d strategies, expected 9 (5 original + 4 added)",
			len(defaultLiveStrategyNames))
	}
}
