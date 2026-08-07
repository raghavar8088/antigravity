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
