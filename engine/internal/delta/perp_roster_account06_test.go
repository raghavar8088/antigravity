package delta

import "testing"

// Account 06 exists to answer a question the other five books cannot: whether
// the streams no existing book has funded are worth funding.
//
// That only holds while its watch list shares no (strategy, symbol) pair with
// Accounts 01-05. Every stream it shares is a row whose result is already being
// produced elsewhere, and the book stops being independent evidence by exactly
// that much. The rule is invisible in the list itself — 95 literals that look
// like any other book's — so nothing but a test keeps it true through the next
// roster edit.
func TestPaperAccount06_SharesNoStreamWithTheOtherBooks(t *testing.T) {
	held := map[string][]string{}
	for _, id := range PaperAccountIDs() {
		if id == PaperAccount06 {
			continue
		}
		for _, st := range ScalpPaperStreamsFor(id) {
			k := perpStreamKey(st.Strategy, st.Symbol)
			held[k] = append(held[k], id)
		}
	}

	six := ScalpPaperStreamsFor(PaperAccount06)
	if len(six) == 0 {
		t.Fatal("Account 06 has no streams — the book is empty, not independent")
	}
	for _, st := range six {
		if books, dup := held[perpStreamKey(st.Strategy, st.Symbol)]; dup {
			t.Errorf("%s on %s is already watched by account(s) %v — Account 06 must hold only pairs no other book holds",
				st.Strategy, st.Symbol, books)
		}
	}
}

// The exclusion is per PAIR, not per strategy.
//
// Reading it as "no strategy that appears elsewhere" would empty the book:
// ANTI_M1_RSI2_5_95_T50_Long alone is spread across five other accounts, and
// dropping it everywhere would discard its BMTUSD, NEIROUSD and MMTUSD rows —
// which are different instruments, different liquidity and a different result.
// This asserts the book actually exercises that distinction, so a future
// "cleanup" that switches to strategy-level exclusion fails here rather than
// quietly shrinking the desk's evidence.
func TestPaperAccount06_KeepsSharedStrategiesOnUnsharedSymbols(t *testing.T) {
	elsewhere := map[string]bool{}
	for _, id := range PaperAccountIDs() {
		if id == PaperAccount06 {
			continue
		}
		for _, st := range ScalpPaperStreamsFor(id) {
			elsewhere[st.Strategy] = true
		}
	}

	shared := 0
	for _, st := range ScalpPaperStreamsFor(PaperAccount06) {
		if elsewhere[st.Strategy] {
			shared++
		}
	}
	if shared == 0 {
		t.Error("no strategy in Account 06 appears in another book on another symbol — " +
			"the list was built by excluding strategy names, not (strategy, symbol) pairs")
	}
}

// A book that watches a stream twice double-counts it: the desk creates one
// account row per pair, so a duplicate is silently dropped and the stream count
// an operator reads on the page stops matching the list in source.
func TestPaperAccount06_HasNoDuplicateStreams(t *testing.T) {
	seen := map[string]bool{}
	for _, st := range defaultScalpPaperStreams06 {
		k := perpStreamKey(st.Strategy, st.Symbol)
		if seen[k] {
			t.Errorf("%s on %s is listed twice", st.Strategy, st.Symbol)
		}
		seen[k] = true
	}
}

// Membership of a paper book must never grant venue access. The live roster is
// the only thing that decides what can place a real order, and Account 06 is the
// newest, least-examined list on the desk — the one most likely to be edited by
// someone who has not read PerpStreamPermitted.
func TestPaperAccount06_GrantsNoVenueAccess(t *testing.T) {
	for _, id := range PaperAccountIDs() {
		if id != PaperAccount06 {
			continue
		}
		for _, st := range ScalpPaperStreamsFor(id) {
			if PerpStreamPermitted(st.Strategy, st.Symbol) && !liveRosterHolds(st.Strategy, st.Symbol) {
				t.Errorf("%s on %s can reach the venue but is not on the live roster", st.Strategy, st.Symbol)
			}
		}
	}
}

// liveRosterHolds reports whether the LIVE roster — not any paper book — lists
// the stream.
func liveRosterHolds(strategy, symbol string) bool {
	key := perpStreamKey(strategy, symbol)
	for _, st := range ScalpLiveStreams() {
		if perpStreamKey(st.Strategy, st.Symbol) == key {
			return true
		}
	}
	return false
}
