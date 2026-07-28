package delta

import "testing"

// CRITICAL regression: openByPaperID used to store a slice INDEX, but trades are
// PREPENDED on every open and adoption. One new trade shifted every element, so
// a close for an older position either silently no-opped — leaving real money
// open with nothing managing it while the monitor retried forever — or resolved
// to the wrong trade. IDs are stable; indices are not.
func TestCloseLookup_SurvivesPrependedTrades(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{{ID: "DLT-0001", PaperTradeID: "paper-1", Status: "OPEN"}}
	b.RegisterOpenMapping("paper-1", "DLT-0001")

	// Two more trades arrive and are prepended — paper-1 is now at index 2.
	b.trades = append([]LiveTrade{{ID: "DLT-0003", PaperTradeID: "paper-3", Status: "OPEN"}}, b.trades...)
	b.trades = append([]LiveTrade{{ID: "DLT-0002", PaperTradeID: "paper-2", Status: "OPEN"}}, b.trades...)

	idx := b.openIndexForPaperID("paper-1")
	if idx < 0 {
		t.Fatal("the original position became uncloseable after prepends — the exact bug that stranded a live position")
	}
	if got := b.trades[idx].ID; got != "DLT-0001" {
		t.Fatalf("resolved the WRONG trade: got %s want DLT-0001", got)
	}
}

// Even with no mapping at all (restored/adopted state), a close must still find
// its position by scanning — a real position must never be uncloseable.
func TestCloseLookup_FallsBackToScanWhenMappingMissing(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{
		{ID: "DLT-0009", PaperTradeID: "paper-9", Status: "OPEN"},
	}
	if idx := b.openIndexForPaperID("paper-9"); idx != 0 {
		t.Fatalf("scan fallback must find the open trade, got idx=%d", idx)
	}
}

// A stale mapping pointing at a CLOSED trade must not be trusted.
func TestCloseLookup_IgnoresStaleMappingToClosedTrade(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{"paper-1": "DLT-0001"}}
	b.trades = []LiveTrade{
		{ID: "DLT-0001", PaperTradeID: "paper-1", Status: "CLOSED"},
		{ID: "DLT-0002", PaperTradeID: "paper-1", Status: "OPEN"},
	}
	idx := b.openIndexForPaperID("paper-1")
	if idx < 0 || b.trades[idx].ID != "DLT-0002" {
		t.Fatalf("must resolve the OPEN trade, got idx=%d", idx)
	}
}

func TestCloseLookup_UnknownPaperIDReturnsNegative(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	if idx := b.openIndexForPaperID("nope"); idx != -1 {
		t.Fatalf("unknown paper id must return -1, got %d", idx)
	}
}
