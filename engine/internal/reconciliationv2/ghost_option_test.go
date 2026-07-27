package reconciliationv2

import (
	"testing"
	"time"
)

// Regression: a real Live Engine option position on Delta must NOT be flagged as
// a ghost by the paper perp/spot OMS reconciliation. Before the fix, the first
// real option fill (P-BTC-64800-290726) tripped the kill switch on every cycle.
func TestPositionDrift_IgnoresLiveEngineOptionPositions(t *testing.T) {
	d := &PositionDriftDetector{}
	exch := []ExchangePosition{
		{Symbol: "P-BTC-64800-290726", Side: "LONG", Quantity: 1, EntryPrice: 0.51},
		{Symbol: "C-BTC-66000-070826", Side: "LONG", Quantity: 2, EntryPrice: 1.2},
	}
	// OMS has none of these (they're the Live Engine's, not the paper OMS's).
	got := d.Detect(exch, nil, time.Now().UTC())
	for _, m := range got {
		if m.Type == "ghost_position" {
			t.Fatalf("option position %s must not be a ghost against the paper OMS", m.Symbol)
		}
	}
}

// A real perp ghost (non-option) must still be caught — the safety net stays.
func TestPositionDrift_StillCatchesPerpGhost(t *testing.T) {
	d := &PositionDriftDetector{}
	exch := []ExchangePosition{{Symbol: "BTCUSD", Side: "LONG", Quantity: 1, EntryPrice: 64000}}
	got := d.Detect(exch, nil, time.Now().UTC())
	found := false
	for _, m := range got {
		if m.Type == "ghost_position" && m.Symbol == "BTCUSD" {
			found = true
		}
	}
	if !found {
		t.Fatal("a real perp ghost position must still be detected")
	}
}

func TestIsDeltaOptionSymbol(t *testing.T) {
	for _, s := range []string{"P-BTC-64800-290726", "C-BTC-66000-070826", "p-btc-1-1"} {
		if !isDeltaOptionSymbol(s) {
			t.Fatalf("%s should be an option symbol", s)
		}
	}
	for _, s := range []string{"BTCUSD", "BTC-USD", "ETHUSD", ""} {
		if isDeltaOptionSymbol(s) {
			t.Fatalf("%s should NOT be an option symbol", s)
		}
	}
}
