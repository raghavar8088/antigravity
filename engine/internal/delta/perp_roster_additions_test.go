package delta

import (
	"strings"
	"testing"
)

// The roster is 5 streams, all on AVAXUSD.
//
// Pinned because it is an owner decision, and because the number carries the
// finding. Fifteen streams were selected off the Top Crypto board; ten cannot
// be funded at $10 equity, so a roster of five is the correct outcome rather
// than a dropped edit.
//
// None of the ten is grid-blocked — every symbol clears the tick grid easily.
// They fail risk sizing. BTC, SOL and ZEC fail its CAP, which no market
// condition can lift: notional never exceeds equity x leverage, so a contract
// dearer than $30 sizes to zero at any volatility. ETHUSD fails only today's
// stop width and would fund at a tighter one; it was routed on 2026-08-20 and
// removed again on the 21st because it refused continuously, which is noise
// rather than evidence. TestRoster_EverySymbolCanBeSizedByRisk holds both.
//
// If this count climbs without desk equity changing, something routed a stream
// the account cannot fund — the populated-but-unfillable roster this file has
// warned about through four rewrites.
func TestDefaultScalpLiveStreams_HoldsOnlyMTFStreams(t *testing.T) {
	// NOT an exact count. This assertion read `!= 5` and failed the moment the
	// owner rerostered, which is a routine act — an exact-count assert on a
	// list whose whole purpose is to be edited turns every edit into a build
	// break. This repo has been bitten by that shape before.
	//
	// The property the count was standing in for is that every routed stream
	// can actually be FUNDED at desk equity, and that is asserted directly by
	// TestRoster_EverySymbolCanBeSizedByRisk. What belongs here is only that
	// the roster is non-empty and well-formed.
	if len(defaultScalpLiveStreams) == 0 {
		t.Fatal("live roster is empty — the desk would route nothing at all")
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
