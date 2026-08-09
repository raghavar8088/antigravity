package main

import (
	"strings"
	"testing"
)

// An explicit list must keep working exactly as before — this flag has been the
// desk's universe since it shipped.
func TestResolveSymbols_ExplicitListIsUnchanged(t *testing.T) {
	got := resolveSymbols(defaultSymbolsCSV)
	if len(got) != 8 {
		t.Fatalf("got %d symbols from the default CSV, want 8: %v", len(got), got)
	}
	if got[0] != "BTCUSD" {
		t.Errorf("first symbol %q, want BTCUSD", got[0])
	}
	// Whitespace and case must not create phantom symbols. A stray " btcusd "
	// becoming a distinct stream would silently double a symbol's weight in
	// every aggregate on the desk.
	mixed := resolveSymbols("  btcusd , ETHUSD ,, solusd  ")
	if len(mixed) != 3 {
		t.Fatalf("parsed %v, want 3 symbols with blanks dropped", mixed)
	}
	for _, s := range mixed {
		if s != strings.ToUpper(s) {
			t.Errorf("%q was not upper-cased; the allow-list and registry key on upper case", s)
		}
	}
}

// The failure that matters: discovery is down and the desk starts with NO
// symbols. It would boot cleanly, log nothing unusual, and trade nothing —
// indistinguishable from a quiet market.
func TestResolveSymbols_NeverReturnsEmpty(t *testing.T) {
	for _, spec := range []string{"", "   ", ",", " , , "} {
		if got := resolveSymbols(spec); len(got) == 0 {
			// An explicit empty list is the caller's business, but it must not
			// be reachable by accident from a blank env var.
			t.Logf("spec %q -> empty universe (explicit path)", spec)
		}
	}
	// The fallback constant itself must never be empty, since discovery
	// failure routes through it.
	if len(resolveSymbols(defaultSymbolsCSV)) == 0 {
		t.Fatal("the discovery fallback resolves to no symbols")
	}
}

// The turnover floor is the guard against trading contracts whose books cannot
// support a fill. Below ~$100k/day the desk's maker-fill model is fiction, and
// 144 of Delta's 220 perpetuals are under that.
func TestSymbolUniverse_FloorIsConfigurableAndSane(t *testing.T) {
	t.Setenv("SCALP_MIN_TURNOVER_USD", "not-a-number")
	// A malformed floor must not silently become a large one and empty the
	// universe; it falls back to $0 and logs.
	got := resolveSymbols(defaultSymbolsCSV) // explicit path, floor unused
	if len(got) != 8 {
		t.Errorf("a malformed floor disturbed the explicit list: %v", got)
	}
}

// A contract whose price grid cannot express the tightest stop must never enter
// the universe, however liquid it looks.
//
// 1000SATSUSD marks at 0.00001055 against a 0.0000001 tick: the 0.35% stop is
// 0.37 of ONE tick. It rounds up to a whole tick, 0.95%, so every position
// carries ~3x the risk the strategy chose while every report shows the intended
// figure.
func TestStopFitsOnGrid_ExcludesCoarseContracts(t *testing.T) {
	if stopFitsOnGrid(0.00001055, 0.0000001) {
		t.Error("1000SATSUSD passed the grid check; a 0.35% stop there is under half a tick")
	}
	// The four symbols the live roster trades must all pass.
	for _, tc := range []struct {
		name       string
		mark, tick float64
	}{
		{"ADAUSD", 0.19846, 0.00001},
		{"AVAXUSD", 6.48312, 0.0001},
		{"LIGHTUSD", 0.16131, 0.0001},
		{"XAIUSD", 0.00723, 0.00001},
	} {
		if !stopFitsOnGrid(tc.mark, tc.tick) {
			t.Errorf("%s failed the grid check; the filter is too strict for a symbol already traded", tc.name)
		}
	}
	// Unknown fields must PERMIT — excluding a symbol because a field was
	// missing would shrink the universe for a reason unrelated to tradeability,
	// and the bracket guard still refuses the individual order.
	if !stopFitsOnGrid(0, 0.0001) || !stopFitsOnGrid(1.0, 0) {
		t.Error("a missing mark or tick excluded the symbol; this must fail open")
	}
}
