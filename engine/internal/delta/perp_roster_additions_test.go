package delta

import (
	"strings"
	"testing"
)

// The roster is SIX streams from 2026-08-16, replacing the ten promoted the day
// before.
//
// The count is pinned because it is an owner decision, and because the number
// itself carries the finding: the owner selected FOURTEEN leaderboard rows and
// eight of them were on MOVEUSD, whose 0.00001 tick against a 0.00636728 mark
// makes the narrowest stop in the pack 5.7 ticks wide. Those eight are excluded
// by TestPerpRoster_ThinAndCoarseSymbolsStayOffTheVenue, so a roster of six
// where fourteen were chosen is the correct outcome rather than a dropped edit.
//
// If this count ever changes to fourteen without that exclusion test changing
// too, something added MOVEUSD back and the desk gained eight rows that can
// never fill — the populated-but-unfillable roster this file has warned about
// through three separate rewrites.
func TestDefaultScalpLiveStreams_HoldsOnlyMTFStreams(t *testing.T) {
	if len(defaultScalpLiveStreams) != 6 {
		t.Errorf("roster has %d streams, want the 6 promoted 2026-08-16 "+
			"(14 selected, 8 on MOVEUSD excluded by the tick grid)", len(defaultScalpLiveStreams))
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
