package main

import (
	"strings"
	"testing"

	"antigravity-engine/internal/delta"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// roster_registry_test.go — the roster names a strategy that does not exist.
//
// WHY THIS TEST EXISTS
//
// A rostered stream is a STRING matched against the registry at runtime. A typo
// is therefore not a compile error and not a startup error: the allow-list
// simply holds an entry nothing ever matches, so the stream shows up in the UI
// as live, is reported as routed, and never trades. That is indistinguishable
// from a strategy that found no setups, which is the same silent-nothing that
// once hid 96 dead strategies behind "no signal".
//
// Real money is routed off this list. A name in it that resolves to nothing
// should break the build, not wait to be noticed.

// TestRoster_EveryStrategyExistsInTheRegistry is the important one.
func TestRoster_EveryStrategyExistsInTheRegistry(t *testing.T) {
	known := map[string]bool{}
	for _, e := range scalers.BuildHuntPack() {
		known[e.Name] = true
	}
	if len(known) == 0 {
		t.Fatal("the strategy pack is empty — nothing could trade whatever the roster says")
	}

	for _, st := range delta.ScalpLiveStreams() {
		if !known[st.Strategy] {
			t.Errorf("roster routes %q on %s, but no strategy by that name is registered — "+
				"this stream would be allow-listed and never signal",
				st.Strategy, st.Symbol)
		}
	}
}

// TestRoster_ConcentrationIsFundableInSlots guards the failure this desk has
// already had: a roster whose streams cannot all hold a position, so the ones
// that trade are chosen by signal speed rather than by the operator.
func TestRoster_ConcentrationIsFundableInSlots(t *testing.T) {
	streams := delta.ScalpLiveStreams()
	if len(streams) == 0 {
		t.Skip("no rostered streams")
	}

	perSymbol := map[string]int{}
	for _, st := range streams {
		perSymbol[strings.ToUpper(strings.TrimSpace(st.Symbol))]++
	}

	// Every stream on the busiest symbol must be able to hold a position at the
	// same time, or the surplus streams are rostered but structurally unable to
	// trade.
	if got, want := scalpLiveMaxPerSymbol(), maxCount(perSymbol); got < want {
		t.Errorf("per-symbol cap is %d but the roster puts %d streams on one symbol — "+
			"%d of them can never open a position", got, want, want-got)
	}

	// And the concurrency cap must not be the thing that throttles the roster.
	if got := scalpLiveMaxConcurrent(); got > 0 && got < len(streams) {
		t.Errorf("concurrency cap is %d for %d rostered streams — the slowest %d collect no record",
			got, len(streams), len(streams)-got)
	}
}

// TestRoster_AggregateCeilingCanFundTheSlots checks the control that actually
// binds once the count caps follow the roster. A ceiling too low to fund the
// slots re-creates the block it was meant to replace, just one layer down.
func TestRoster_AggregateCeilingCanFundTheSlots(t *testing.T) {
	slots := scalpLiveMaxConcurrent()
	if slots <= 3 {
		t.Skip("built-in ceiling covers a three-slot book")
	}
	x := scalpLiveMaxAggregateLeverage()
	if x <= 0 {
		t.Fatalf("%d slots but no aggregate ceiling was derived", slots)
	}
	// Not a P&L claim — only that the ceiling is in the right order of magnitude
	// for the number of positions the desk is now allowed to hold.
	if x < 2.0 {
		t.Errorf("aggregate ceiling %.2fx cannot fund %d concurrent positions", x, slots)
	}
	if x > 10.0 {
		t.Errorf("aggregate ceiling %.2fx commits far more than the account holds", x)
	}
}

func maxCount(m map[string]int) int {
	most := 0
	for _, n := range m {
		if n > most {
			most = n
		}
	}
	return most
}
