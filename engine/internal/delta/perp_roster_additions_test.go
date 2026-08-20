package delta

import (
	"strings"
	"testing"
)

// The roster is 5 streams, all on AVAXUSD.
//
// Pinned because it is an owner decision, and because the number carries the
// finding. Fifteen streams were selected off the Top Crypto board on
// 2026-08-20; ten of them cannot be SIZED on a $10 account, so a roster of five
// where fifteen were chosen is the correct outcome rather than a dropped edit.
//
// The ten are not grid-blocked — every symbol involved clears the tick grid
// easily. They fail risk sizing, which is a different and here tighter limit;
// TestRoster_EverySymbolCanBeSizedByRisk holds that reasoning.
//
// If this count climbs without desk equity changing, something routed a stream
// the account cannot fund — the populated-but-unfillable roster this file has
// warned about through four rewrites.
func TestDefaultScalpLiveStreams_HoldsOnlyMTFStreams(t *testing.T) {
	if len(defaultScalpLiveStreams) != 5 {
		t.Errorf("roster has %d streams, want the 5 AVAXUSD streams routable at $10 equity "+
			"(15 selected from Top Crypto; 10 cannot be sized on this account)",
			len(defaultScalpLiveStreams))
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
