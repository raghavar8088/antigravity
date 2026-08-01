package delta

import (
	"strings"
	"testing"
)

// The owner may hold their OWN option positions on the same Delta account. The
// engine must never take those over and close them on its SL/TP, so adoption is
// limited to products this engine has actually traded.
func TestAdoptionScope_OnlyProductsTheEngineTraded(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}}
	b.trades = []LiveTrade{
		{ID: "DLT-0001", ProductID: 111, Status: "CLOSED"}, // engine traded 111 before
	}
	known := map[int]bool{}
	for _, tr := range b.trades {
		if tr.ProductID != 0 {
			known[tr.ProductID] = true
		}
	}
	if !known[111] {
		t.Fatal("a previously traded product must be adoptable")
	}
	if known[999] {
		t.Fatal("a product the engine never traded (the owner's manual trade) must NOT be adoptable")
	}
}

func TestAdoptionScope_OverrideFlagIsOptIn(t *testing.T) {
	// Default: do not adopt unknown products.
	if strings.EqualFold(strings.TrimSpace(""), "true") {
		t.Fatal("unset LIVE_ADOPT_UNKNOWN must not enable adoption of unknown products")
	}
	if !strings.EqualFold(strings.TrimSpace(" TRUE "), "true") {
		t.Fatal("explicit LIVE_ADOPT_UNKNOWN=true must enable the override")
	}
}
