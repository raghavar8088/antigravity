package delta

import "testing"

// The Gold Desk's book is paper-only by construction. These tests pin that,
// plus the two ways a symbol-scoped book quietly goes wrong: falling through to
// another book's roster when it is unset, and leaking its symbol into books that
// are supposed to be a different asset class.
func TestGoldPaperBook(t *testing.T) {
	restore := GoldPaperStreams()
	t.Cleanup(func() { SetGoldPaperStreams(restore) })

	// Unregistered must be EMPTY, never the crypto default. Falling through
	// would fill a gold book with altcoin streams and nothing would look wrong.
	SetGoldPaperStreams(nil)
	if got := ScalpPaperStreamsFor(PaperAccountGold); len(got) != 0 {
		t.Fatalf("unregistered gold book returned %d streams, want 0", len(got))
	}

	SetGoldPaperStreams([]PerpStream{
		{Strategy: "MTF_4h_TriangleBreak_Long", Symbol: "XAUTUSD"},
		{Strategy: "MTF_4h_TriangleBreak_Long", Symbol: "PAXGUSD"},
		{Strategy: "MTF_1h_TrendPullback_Long", Symbol: "XAUTUSD"},
		{Strategy: "MTF_4h_TriangleBreak_Long", Symbol: "XAUTUSD"}, // duplicate
		{Strategy: "MTF_1h_TrendPullback_Long", Symbol: "BTCUSD"},  // not gold
	})

	got := ScalpPaperStreamsFor(PaperAccountGold)
	if len(got) != 3 {
		t.Fatalf("gold book = %d streams, want 3 after dropping the duplicate and the non-gold row: %+v", len(got), got)
	}

	for _, st := range got {
		if !IsGoldSymbol(st.Symbol) {
			t.Fatalf("non-gold symbol leaked into the gold book: %s", st.Symbol)
		}
		if !PerpStreamPaperPermittedFor(PaperAccountGold, st.Strategy, st.Symbol) {
			t.Fatalf("stream is not permitted in its own book: %s/%s", st.Strategy, st.Symbol)
		}
		// The property that matters: no gold stream may reach real money. Gold
		// reaches the venue only by being added to the live roster deliberately,
		// never as a side effect of appearing on this desk.
		if PerpStreamPermitted(st.Strategy, st.Symbol) {
			t.Fatalf("gold stream is on the LIVE venue allow-list — real money is reachable: %s/%s", st.Strategy, st.Symbol)
		}
	}
}

// The gold book is registered globally, so a mistake there would show up as
// gold rows inside the crypto books rather than as an error.
func TestGoldDoesNotLeakIntoCryptoBooks(t *testing.T) {
	for _, id := range PaperAccountIDs() {
		if id == PaperAccountGold {
			continue
		}
		for _, st := range ScalpPaperStreamsFor(id) {
			if IsGoldSymbol(st.Symbol) {
				t.Fatalf("gold symbol %s found in crypto book %s", st.Symbol, id)
			}
		}
	}
}

func TestIsGoldSymbol(t *testing.T) {
	for _, ok := range []string{"XAUTUSD", "xautusd", " PAXGUSD ", "paxgusd"} {
		if !IsGoldSymbol(ok) {
			t.Errorf("IsGoldSymbol(%q) = false, want true", ok)
		}
	}
	// Silver is listed on the venue and deliberately not part of this desk.
	for _, no := range []string{"SLVONUSD", "BTCUSD", "", "XAU", "PAXG"} {
		if IsGoldSymbol(no) {
			t.Errorf("IsGoldSymbol(%q) = true, want false", no)
		}
	}
}
