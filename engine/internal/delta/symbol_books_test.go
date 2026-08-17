package delta

import "testing"

// Per-symbol books are registered at runtime, which is the same shape that let
// the gold book ship seeding zero rows. These pin the three ways it goes wrong
// quietly: a book that falls through to another roster, a book that accepts a
// foreign symbol, and registration disturbing the numbered books.
func TestSymbolBooksResolve(t *testing.T) {
	base := len(PaperAccountIDs())
	t.Cleanup(func() { SetSymbolPaperBooks(nil, nil) })

	SetSymbolPaperBooks([]string{"BTCUSD", "ETHUSD", "XAUTUSD"}, map[string][]PerpStream{
		"BTCUSD": {
			{Strategy: "MTF_1h_Wedge_Long", Symbol: "BTCUSD"},
			{Strategy: "MTF_45m_Diamond_Short", Symbol: "BTCUSD"},
			{Strategy: "MTF_1h_Wedge_Long", Symbol: "BTCUSD"}, // duplicate
		},
		"ETHUSD": {
			{Strategy: "MTF_1h_Wedge_Long", Symbol: "ETHUSD"},
			{Strategy: "MTF_1h_Wedge_Long", Symbol: "BTCUSD"}, // foreign
		},
		"XAUTUSD": {{Strategy: "MTF_4h_Keltner_Long", Symbol: "XAUTUSD"}},
	})

	// Registration ORDER is preserved: map order would reshuffle the page's tabs
	// on every boot.
	got := SymbolBookIDs()
	if len(got) != 3 || got[0] != "BTCUSD" || got[1] != "ETHUSD" || got[2] != "XAUTUSD" {
		t.Fatalf("SymbolBookIDs = %v, want [BTCUSD ETHUSD XAUTUSD]", got)
	}
	if n := len(PaperAccountIDs()); n != base+3 {
		t.Fatalf("PaperAccountIDs = %d, want %d", n, base+3)
	}
	if !IsSymbolBook("btcusd") || IsSymbolBook("01") {
		t.Fatal("IsSymbolBook misclassified a book")
	}
	if b := ScalpPaperStreamsFor("BTCUSD"); len(b) != 2 {
		t.Fatalf("BTCUSD = %d streams, want 2 after dedupe: %+v", len(b), b)
	}
	// A foreign symbol in a symbol book is a wiring bug, not a preference.
	eth := ScalpPaperStreamsFor("ETHUSD")
	if len(eth) != 1 || eth[0].Symbol != "ETHUSD" {
		t.Fatalf("ETHUSD book leaked a foreign symbol: %+v", eth)
	}
	for _, st := range ScalpPaperStreamsFor("BTCUSD") {
		if !PerpStreamPaperPermittedFor("BTCUSD", st.Strategy, st.Symbol) {
			t.Fatalf("%s/%s is not permitted in its own book", st.Strategy, st.Symbol)
		}
		// Paper only, exactly as the gold book is: a symbol book must not be a
		// route to real money.
		if PerpStreamPermitted(st.Strategy, st.Symbol) {
			t.Fatalf("symbol-book stream is on the LIVE allow-list: %s/%s", st.Strategy, st.Symbol)
		}
	}
	// The numbered books must be untouched by any of this.
	if len(ScalpPaperStreamsFor(PaperAccount02)) == 0 {
		t.Fatal("registering symbol books emptied book 02")
	}
}

// Unregistering must actually unregister, or a redeploy that drops a symbol
// leaves a tab backed by nothing.
func TestSymbolBooksClear(t *testing.T) {
	SetSymbolPaperBooks([]string{"BTCUSD"}, map[string][]PerpStream{
		"BTCUSD": {{Strategy: "MTF_1h_Wedge_Long", Symbol: "BTCUSD"}},
	})
	if len(SymbolBookIDs()) != 1 {
		t.Fatal("registration did not take")
	}
	SetSymbolPaperBooks(nil, nil)
	if got := SymbolBookIDs(); len(got) != 0 {
		t.Fatalf("SymbolBookIDs = %v after clearing, want empty", got)
	}
	if IsSymbolBook("BTCUSD") {
		t.Fatal("BTCUSD still reports as a symbol book after clearing")
	}
}
