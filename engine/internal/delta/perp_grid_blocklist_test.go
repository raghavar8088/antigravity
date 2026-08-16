package delta

import (
	"strings"
	"testing"
)

// The blocklist must gate EVERY path a symbol can enter through.
//
// Not just the order path. The reason these symbols are excluded at all is that
// blocking only the order left them in the universe, where they still ran every
// strategy, still booked paper fills and still ranked — and the coarse grid
// that blocks the order also flatters the fill, so they ranked HIGH. Eight of
// the fourteen streams the owner picked on 2026-08-16 were selected that way.
func TestGridBlocklist_GatesTheRosterAndTheOverride(t *testing.T) {
	for _, sym := range GridBlockedSymbols() {
		if !IsGridBlocked(sym) {
			t.Errorf("%s is listed but IsGridBlocked says otherwise", sym)
		}
		if GridBlockedReason(sym) == "" {
			t.Errorf("%s is blocked with no reason; a refusal without a cause is how a desk goes quiet unexplained", sym)
		}
		// Case and whitespace must not open the gate — these arrive from env
		// vars and command lines that people edit by hand.
		if !IsGridBlocked(" " + strings.ToLower(sym) + " ") {
			t.Errorf("%s escaped the blocklist when lowercased and padded", sym)
		}
	}

	// The built-in roster must be clean.
	for _, st := range ScalpLiveStreams() {
		if IsGridBlocked(st.Symbol) {
			t.Errorf("%s on %s reached the live roster despite the blocklist", st.Strategy, st.Symbol)
		}
	}
	for _, st := range ScalpPaperStreams() {
		if IsGridBlocked(st.Symbol) {
			t.Errorf("%s on %s reached the PAPER roster; a blocked symbol still produces a leaderboard row", st.Strategy, st.Symbol)
		}
	}
}

// SCALP_LIVE_STREAMS is the path that bypasses review: it lives in a file on
// the box, not in the repo, and exists precisely to route a stream without a
// redeploy. It must not be able to route a blocked symbol.
func TestGridBlocklist_EnvOverrideCannotReintroduceABlockedSymbol(t *testing.T) {
	t.Setenv("SCALP_LIVE_STREAMS", "MTF_1h_TrendPullback_Long:LABUSD,MTF_1h_TrendPullback_Long:CROSSUSD")
	got := ScalpLiveStreams()
	for _, st := range got {
		if strings.EqualFold(st.Symbol, "LABUSD") {
			t.Fatal("SCALP_LIVE_STREAMS routed LABUSD, which measures 19.0 ticks against a 20-tick gate")
		}
	}
	if len(got) != 1 || !strings.EqualFold(got[0].Symbol, "CROSSUSD") {
		t.Fatalf("the permitted half of the override was lost: %v", got)
	}
}

// An override naming ONLY blocked symbols must not fall through to the built-in
// roster as though nothing had been asked for.
//
// It does fall back, and that is the safe direction — but the fallback must not
// silently include the blocked symbol, which is what would happen if the filter
// ran before the emptiness check rather than after.
func TestGridBlocklist_AnAllBlockedOverrideFallsBackClean(t *testing.T) {
	t.Setenv("SCALP_LIVE_STREAMS", "S:LABUSD,S:DOTUSD")
	for _, st := range ScalpLiveStreams() {
		if IsGridBlocked(st.Symbol) {
			t.Errorf("blocked symbol %s survived an all-blocked override", st.Symbol)
		}
	}
}

// BNBUSD must NOT be blocked.
//
// It was on the first draft of this list, on a single 20.0-tick reading taken
// during the opening run. Three later runs measured 53.9, 59.9 and 59.9. One
// reading at the boundary is not a property of the contract, and banning the
// venue's second-largest perpetual on a measurement error is a real cost.
//
// Pinned as a test because the next person to see a low BNBUSD reading will be
// tempted to add it, and this is where they should find out that it flickers.
func TestGridBlocklist_BNBUSDSurvivedTheRemeasure(t *testing.T) {
	if IsGridBlocked("BNBUSD") {
		t.Error("BNBUSD is blocked; it measured 53.9-59.9 ticks on three runs after a single 20.0 outlier")
	}
}
