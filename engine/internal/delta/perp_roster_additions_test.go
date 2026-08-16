package delta

import (
	"strings"
	"testing"
)

// The roster is 33 streams from 2026-08-16.
//
// The count is pinned because it is an owner decision, and because the number
// itself carries the finding: the owner selected FOURTEEN leaderboard rows, and
// ten of them are on contracts whose price grid cannot hold a stop — eight on
// MOVEUSD at 5.7 ticks, two on LABUSD at 19.0 against a 20-tick gate. Both
// symbols are in gridBlockedSymbols, so a roster of four where fourteen were
// chosen is the correct outcome rather than a dropped edit.
//
// If this count ever climbs without the blocklist shrinking, something routed a
// symbol that cannot fill — the populated-but-unfillable roster this file has
// warned about through three separate rewrites.
func TestDefaultScalpLiveStreams_HoldsOnlyMTFStreams(t *testing.T) {
	if len(defaultScalpLiveStreams) != 33 {
		t.Errorf("roster has %d streams, want 33 "+
			"(39 selected + 1 carried over; 7 held back on XAIUSD/ZECUSD/SWARMSUSD, "+
			"which fail the grid TODAY but are not permanently blocked)", len(defaultScalpLiveStreams))
	}
	// Every retired pack name must stay off it. The Scalp100/Delta20/Curated
	// strategies no longer exist on the desk, so a roster entry naming one
	// would be an allow-list row that can never produce a signal.
	for _, st := range defaultScalpLiveStreams {
		if !strings.HasPrefix(st.Strategy, "MTF_") {
			t.Errorf("%s on %s is not from the MTF pack — the desk no longer builds it",
				st.Strategy, st.Symbol)
		}
	}
}

// THE safety property: an empty allow-list must permit NOTHING.
//
// The opposite reading — empty means unrestricted — is the single most
// dangerous way this could fail, because it would route every strategy on every
// symbol to real money the moment the roster was cleared. PerpAllowList
// documents nil as "allow nothing" precisely for this; the test makes it a fact
// rather than a comment.
func TestPerpAllowList_EmptyPermitsNothing(t *testing.T) {
	a := NewPerpAllowList()
	a.SetPairs(nil)
	for _, c := range []struct{ strategy, symbol string }{
		{"MTF_15m_TrendPullback_Long", "TSTUSD"},
		{"ANTI_Recurrence_Quantification_Signal", "BEATUSD"},
		{"anything", "ANYUSD"},
	} {
		if a.Allowed(c.strategy, c.symbol) {
			t.Errorf("empty allow-list permitted %s on %s — clearing the roster would arm everything",
				c.strategy, c.symbol)
		}
	}
	if n := a.Count(); n != 0 {
		t.Errorf("empty allow-list reports %d entries", n)
	}
}

// A stream routed via SCALP_LIVE_STREAMS must still be honoured, so an empty
// default does not mean the desk cannot be pointed at anything.
func TestScalpLiveStreams_OverrideStillWorksWithAnEmptyDefault(t *testing.T) {
	t.Setenv("SCALP_LIVE_STREAMS", "MTF_1h_TrendPullback_Long:TSTUSD")
	got := ScalpLiveStreams()
	if len(got) != 1 || got[0].Strategy != "MTF_1h_TrendPullback_Long" || !strings.EqualFold(got[0].Symbol, "TSTUSD") {
		t.Fatalf("override produced %v", got)
	}
}
