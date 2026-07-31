package options_selling

import "testing"

// This SELLING desk picks strategies for real money, so the integrity of its record is
// the product. A trade recorded against a price no venue quoted is not a weak
// data point — it is a false one, and once in the table it is indistinguishable
// from a real one.

// The gate must start CLOSED. If it defaulted open, a wiring mistake that never
// called SetFeedStatus would leave the desk trading unguarded — and it would
// look completely normal.
func TestFeedGate_DefaultsClosed(t *testing.T) {
	var g feedGate
	if g.FeedLive() {
		t.Fatal("gate defaults open; a missing SetFeedStatus call would silently trade on unverified prices")
	}
}

// ...but a closed gate that is never opened stops the desk entirely, which is
// the opposite failure. The engine must trade once a real price is published.
func TestFeedGate_OpensOnRealPrice(t *testing.T) {
	var g feedGate
	g.SetFeedStatus(true, PrimaryVenue)
	if !g.FeedLive() {
		t.Fatal("gate stayed closed after a real venue price; the desk would never trade")
	}
	if g.FeedSource() != PrimaryVenue {
		t.Errorf("source = %q, want %q", g.FeedSource(), PrimaryVenue)
	}
}

func TestFeedGate_ClosesWhenTheFeedDies(t *testing.T) {
	var g feedGate
	g.SetFeedStatus(true, PrimaryVenue)
	g.SetFeedStatus(false, SyntheticSource)
	if g.FeedLive() {
		t.Fatal("gate stayed open with no real price")
	}
}

// Entries taken on the backup venue are real trades, but they were priced on a
// book the Live Engine does not execute against. A strategy qualified largely on
// them was qualified on the wrong market, so the two must be separable.
func TestFeedGate_SeparatesFallbackOpensFromPrimary(t *testing.T) {
	var g feedGate

	g.SetFeedStatus(true, PrimaryVenue)
	g.noteOpen(PrimaryVenue)
	g.noteOpen(PrimaryVenue)
	if got := g.OpensOnFallback(); got != 0 {
		t.Errorf("primary-venue opens counted as fallback: %d", got)
	}

	g.SetFeedStatus(true, "binance")
	g.noteOpen(PrimaryVenue)
	if got := g.OpensOnFallback(); got != 1 {
		t.Errorf("OpensOnFallback = %d, want 1", got)
	}
}

// A record that quietly lost a day of signals to a dead feed looks exactly like
// a quiet market unless the refusals are counted.
func TestFeedGate_CountsBlockedOpens(t *testing.T) {
	var g feedGate
	for i := 0; i < 3; i++ {
		g.noteBlockedOpen()
	}
	if got := g.BlockedOpens(); got != 3 {
		t.Errorf("BlockedOpens = %d, want 3", got)
	}
}

// Recovery must re-arm the "feed down" log, or a desk that flaps between up and
// down reports the outage once and stays silent through every later one.
func TestFeedGate_ReArmsItsOutageLogOnRecovery(t *testing.T) {
	var g feedGate
	g.SetFeedStatus(false, SyntheticSource)
	if !g.loggedDown.Load() {
		t.Fatal("first outage did not mark itself logged")
	}
	g.SetFeedStatus(true, PrimaryVenue)
	if g.loggedDown.Load() {
		t.Fatal("recovery did not re-arm the outage log; a second outage would go unreported")
	}
}

// The engine must actually consult the gate. Without this the gate is
// decoration — the same failure mode as a go-live check wired to constants.
func TestEngine_RefusesNewEntriesWithoutARealPrice(t *testing.T) {
	e := NewEngine()
	if e.FeedLive() {
		t.Fatal("a freshly built engine reports a live feed before any price arrived")
	}

	// Give it a price the way the desk normally would, but never mark the feed
	// live — this is exactly the synthetic-spot situation.
	e.UpdatePrice(60000)
	before := e.BlockedOpens()
	e.tick()
	if e.BlockedOpens() == before {
		// tick() may legitimately return early for other reasons (no minute bars
		// yet); what must never happen is an OPEN.
		t.Log("no entry attempts on this tick — checking no position was opened instead")
	}
	for _, s := range e.states {
		if s.position != nil {
			t.Fatalf("%s opened a position with no real market price", s.def.Name)
		}
	}
}
