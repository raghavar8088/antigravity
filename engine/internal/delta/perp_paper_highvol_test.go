package delta

import "testing"

// The book must not exist until a process asks for it. Every scalp container
// shares this binary, and a roster that registered itself would put nine
// streams and a $500 balance into desks whose operators never chose them.
func TestHighVolBookIsOptIn(t *testing.T) {
	resetNamedBooks(t)

	for _, id := range PaperAccountIDs() {
		if id == PaperAccountHighVol {
			t.Fatalf("%s is in PaperAccountIDs() before any registration", PaperAccountHighVol)
		}
	}
	if IsNamedPaperBook(PaperAccountHighVol) {
		t.Fatalf("%s reports as registered before any registration", PaperAccountHighVol)
	}
	if got := ScalpPaperStreamsFor(PaperAccountHighVol); len(got) != len(ScalpPaperStreams()) {
		// An unregistered id falls through to the default candidate roster,
		// which is the pre-existing behaviour for any unknown account.
		t.Fatalf("unregistered id returned %d streams, want the default roster's %d",
			len(got), len(ScalpPaperStreams()))
	}
}

func TestRegisterHighVolPaperBook(t *testing.T) {
	resetNamedBooks(t)

	n := RegisterHighVolPaperBook()
	want := len(HighVolPaperStreams())
	if n != want {
		t.Fatalf("registered %d streams, want %d", n, want)
	}
	if n != 9 {
		t.Fatalf("the owner selected nine streams, the roster holds %d", n)
	}

	ids := PaperAccountIDs()
	found := false
	for _, id := range ids {
		if id == PaperAccountHighVol {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s missing from PaperAccountIDs() after registration: %v", PaperAccountHighVol, ids)
	}
	// The numbered books and gold keep their positions, or every page that
	// renders this list in order would reshuffle.
	if ids[0] != PaperAccount01 || ids[6] != PaperAccountGold {
		t.Fatalf("existing books moved: %v", ids)
	}

	got := ScalpPaperStreamsFor(PaperAccountHighVol)
	if len(got) != want {
		t.Fatalf("book holds %d streams, want %d", len(got), want)
	}
	for _, st := range got {
		if !PerpStreamPaperPermittedFor(PaperAccountHighVol, st.Strategy, st.Symbol) {
			t.Fatalf("%s %s is in the book but not permitted in it", st.Strategy, st.Symbol)
		}
	}

	// Membership grants nothing at the venue: that is decided by the live
	// roster alone, and this book is paper by construction rather than by
	// anyone remembering to keep it that way.
	for _, st := range got {
		if PerpStreamPermitted(st.Strategy, st.Symbol) != perpStreamOnLiveRoster(st.Strategy, st.Symbol) {
			t.Fatalf("%s %s: book membership changed venue permission", st.Strategy, st.Symbol)
		}
	}

	if eq, ok := NamedPaperBookEquity(PaperAccountHighVol); !ok || eq != HighVolPaperEquityUSD {
		t.Fatalf("book equity = %.2f (registered=%v), want %.2f", eq, ok, HighVolPaperEquityUSD)
	}
	if HighVolPaperEquityUSD != 500 {
		t.Fatalf("the owner asked for a $500 book, the constant says $%.2f", HighVolPaperEquityUSD)
	}
}

// Re-registering must replace the roster rather than append a second book under
// the same id.
func TestRegisterNamedPaperBookIsIdempotent(t *testing.T) {
	resetNamedBooks(t)

	RegisterHighVolPaperBook()
	RegisterHighVolPaperBook()

	seen := 0
	for _, id := range NamedPaperBookIDs() {
		if id == PaperAccountHighVol {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("%s appears %d times after two registrations", PaperAccountHighVol, seen)
	}
	if got := len(ScalpPaperStreamsFor(PaperAccountHighVol)); got != len(HighVolPaperStreams()) {
		t.Fatalf("roster holds %d streams after two registrations, want %d", got, len(HighVolPaperStreams()))
	}
}

// Duplicates in a hand-written roster are a typo, not an instruction to watch
// the same stream twice — a duplicated row would double-count its P&L in the
// board's totals.
func TestRegisterNamedPaperBookDedupes(t *testing.T) {
	resetNamedBooks(t)

	n := RegisterNamedPaperBook("TESTBOOK", 250, []PerpStream{
		{Strategy: "A_Long", Symbol: "adausd"},
		{Strategy: "A_Long", Symbol: "ADAUSD"},
		{Strategy: "B_Long", Symbol: "LTCUSD"},
		{Strategy: "", Symbol: "LTCUSD"},
	})
	if n != 2 {
		t.Fatalf("registered %d streams, want 2 after dedupe and the blank", n)
	}
	for _, st := range ScalpPaperStreamsFor("TESTBOOK") {
		if st.Symbol != "ADAUSD" && st.Symbol != "LTCUSD" {
			t.Fatalf("symbol %q was not normalised to upper case", st.Symbol)
		}
	}
}

// The roster is written down rather than derived, so the desk has to be able to
// say which rows this process cannot serve.
func TestHighVolPaperUnservedStreams(t *testing.T) {
	all := HighVolPaperStreams()
	strategies := make([]string, 0, len(all))
	symbols := make([]string, 0, len(all))
	for _, st := range all {
		strategies = append(strategies, st.Strategy)
		symbols = append(symbols, st.Symbol)
	}

	if bad := HighVolPaperUnservedStreams(strategies, symbols); len(bad) != 0 {
		t.Fatalf("a fully-served process reported %d unserved streams: %v", len(bad), bad)
	}

	// Drop one symbol from the universe: every stream on it is unservable.
	fewer := make([]string, 0, len(symbols))
	for _, s := range symbols {
		if s != "UNIUSD" {
			fewer = append(fewer, s)
		}
	}
	bad := HighVolPaperUnservedStreams(strategies, fewer)
	if len(bad) != 3 {
		t.Fatalf("dropping UNIUSD reported %d unserved streams, want the 3 UNI rows: %v", len(bad), bad)
	}

	// And an empty pack: nothing can fill.
	if bad := HighVolPaperUnservedStreams(nil, symbols); len(bad) != len(all) {
		t.Fatalf("an empty strategy pack reported %d unserved, want all %d", len(bad), len(all))
	}
}

// perpStreamOnLiveRoster is the venue allow-list read directly, so the test can
// assert that book membership did not move it.
func perpStreamOnLiveRoster(strategy, symbol string) bool {
	key := perpStreamKey(strategy, symbol)
	for _, st := range ScalpLiveStreams() {
		if perpStreamKey(st.Strategy, st.Symbol) == key {
			return true
		}
	}
	return false
}

// resetNamedBooks clears the registry so each test starts from an unregistered
// process, and restores it afterwards.
func resetNamedBooks(t *testing.T) {
	t.Helper()
	namedBookMu.Lock()
	namedBooks = map[string]namedPaperBook{}
	namedBookOrder = nil
	namedBookMu.Unlock()
	t.Cleanup(func() {
		namedBookMu.Lock()
		namedBooks = map[string]namedPaperBook{}
		namedBookOrder = nil
		namedBookMu.Unlock()
	})
}
