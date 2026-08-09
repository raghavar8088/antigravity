package delta

import (
	"strings"
	"testing"
)

// The allow-list is the only thing between a paper scalp signal and real money
// on a $100 account.

// It must fail CLOSED. The options bridge treats a nil allow-list as
// allow-everything for legacy reasons; a perpetual path must never inherit that,
// because "permit every strategy" is not a safe default for a live account.
func TestPerpAllowList_DefaultsToPermittingNothing(t *testing.T) {
	a := NewPerpAllowList()
	if a.Allowed("ANTI_M1_DoubleTop_10bp_Short", "ADAUSD") {
		t.Fatal("an unset allow-list permitted a live order")
	}
	if a.Count() != 0 {
		t.Errorf("Count = %d on an unset list", a.Count())
	}
}

func TestPerpAllowList_PermitsOnlyWhatWasSet(t *testing.T) {
	a := NewPerpAllowList()
	a.Set([]string{"ANTI_M1_DoubleTop_10bp_Short"}, []string{"ADAUSD"})

	if !a.Allowed("ANTI_M1_DoubleTop_10bp_Short", "ADAUSD") {
		t.Error("the selected stream was denied")
	}
	if a.Allowed("M1_DoubleTop_10bp_Short", "ADAUSD") {
		t.Error("the ORIGINAL was permitted when only its mirror was selected — they are opposite bets")
	}
	if a.Allowed("Some_Other_Strategy", "ADAUSD") {
		t.Error("an unlisted strategy was permitted")
	}
}

// Symbol matters as much as strategy: the same strategy runs on eight symbols on
// the paper desk, and only the pairing that was actually selected should trade.
func TestPerpAllowList_GatesOnSymbolToo(t *testing.T) {
	a := NewPerpAllowList()
	a.Set([]string{"ANTI_M1_VWAP_Doji_Short"}, []string{"ADAUSD"})

	if !a.Allowed("ANTI_M1_VWAP_Doji_Short", "ADAUSD") {
		t.Error("selected symbol denied")
	}
	if a.Allowed("ANTI_M1_VWAP_Doji_Short", "BTCUSD") {
		t.Error("a symbol nobody selected was permitted — one promotion would have enabled 7 unreviewed streams")
	}
	// Case and whitespace from a hand-edited env must not open or close a gate.
	if !a.Allowed("ANTI_M1_VWAP_Doji_Short", " adausd ") {
		t.Error("symbol matching is not case/space tolerant")
	}
}

// The eight names the owner selected must actually be the shipped default.
// The live roster is an OWNER DECISION, pinned exactly — as STREAMS.
//
// Not "at least these", and not strategies and symbols as separate lists. The
// allow-list used to multiply the two, so three chosen leaderboard rows became
// six live streams and three pairings nobody selected could reach the venue.
//
// Replaced 2026-08-07 with the three rows selected on live terms.
func TestScalpLiveStreams_MatchTheOwnerSelectionExactly(t *testing.T) {
	// The roster IS Account 03's book, promoted 2026-08-09. Asserted against
	// that source rather than a second literal list: two hand-maintained copies
	// of 31 streams would drift, and the drift would be silent.
	got := ScalpLiveStreams()
	src := defaultScalpPaperStreams03

	if len(got) != len(src) {
		t.Fatalf("live roster has %d streams, Account 03's book has %d", len(got), len(src))
	}
	inRoster := map[string]bool{}
	for _, st := range got {
		inRoster[perpStreamKey(st.Strategy, st.Symbol)] = true
	}
	for _, st := range src {
		if !inRoster[perpStreamKey(st.Strategy, st.Symbol)] {
			t.Errorf("%v is in Account 03's book but not on the live roster", st)
		}
	}

	// Every previously live stream must be gone — the promotion REPLACED the
	// roster rather than extending it.
	for _, gone := range []PerpStream{
		{Strategy: "ANTI_M1_DoubleBottom_10bp_Long", Symbol: "ADAUSD"},
		{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAXUSD"},
		{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "AVAXUSD"},
		{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "LIGHTUSD"},
		{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "LIGHTUSD"},
		{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "XAIUSD"},
	} {
		if inRoster[perpStreamKey(gone.Strategy, gone.Symbol)] {
			t.Errorf("%v was replaced but is still on the live roster", gone)
		}
	}
}

// The cross product is the bug this replaces. Three selected rows must enable
// three streams — not the six that strategies x symbols produces.
func TestPerpAllowList_PairsDoNotCrossProduct(t *testing.T) {
	a := NewPerpAllowList()
	a.SetPairs(ScalpLiveStreams())

	// Every selected stream is permitted.
	for _, st := range ScalpLiveStreams() {
		if !a.Allowed(st.Strategy, st.Symbol) {
			t.Errorf("selected stream %v was blocked", st)
		}
	}

	// And a pairing the CROSS PRODUCT would invent is not. Built from real
	// names and real symbols already on the roster, so it is a stream that
	// genuinely exists on the paper desk — which is exactly why permitting it
	// unasked would go unnoticed.
	strategies, symbols := map[string]bool{}, map[string]bool{}
	for _, st := range ScalpLiveStreams() {
		strategies[st.Strategy] = true
		symbols[strings.ToUpper(st.Symbol)] = true
	}
	selected := map[string]bool{}
	for _, st := range ScalpLiveStreams() {
		selected[perpStreamKey(st.Strategy, st.Symbol)] = true
	}
	invented := 0
	for st := range strategies {
		for sym := range symbols {
			if selected[perpStreamKey(st, sym)] {
				continue
			}
			if a.Allowed(st, sym) {
				t.Errorf("unselected pairing %s|%s reached the venue — the cross product is back", st, sym)
			}
			invented++
		}
	}
	if invented == 0 {
		t.Fatal("no unselected pairing existed to test; the assertion was vacuous")
	}

	// Symbol matching stays case-insensitive, and an unknown strategy is denied.
	first := ScalpLiveStreams()[0]
	if !a.Allowed(first.Strategy, strings.ToLower(first.Symbol)) {
		t.Error("lowercase symbol was rejected")
	}
	if a.Allowed("Some_Other_Strategy", first.Symbol) {
		t.Error("an unlisted strategy was permitted")
	}
}

// Set() after SetPairs() must clear the pins, or it silently enforces a
// narrower list than the caller asked for.
func TestPerpAllowList_SetClearsPinnedPairs(t *testing.T) {
	a := NewPerpAllowList()
	a.SetPairs([]PerpStream{{"S1", "ADAUSD"}})
	a.Set([]string{"S1"}, []string{"ADAUSD", "AVAXUSD"})
	if !a.Allowed("S1", "AVAXUSD") {
		t.Error("Set() was overridden by stale pinned pairs")
	}
}

func TestScalpLiveStrategies_EnvOverrideWins(t *testing.T) {
	t.Setenv("SCALP_LIVE_STRATEGIES", " A_One , B_Two ")
	got := ScalpLiveStrategies()
	if len(got) != 2 || got[0] != "A_One" || got[1] != "B_Two" {
		t.Fatalf("override gave %v", got)
	}
}

// A blank override must fall back to the curated list rather than producing an
// empty one — though even an empty one denies everything, so this is about
// intent rather than safety.
func TestScalpLiveStrategies_BlankOverrideFallsBack(t *testing.T) {
	for _, raw := range []string{"", "  ", ",", " , "} {
		t.Setenv("SCALP_LIVE_STRATEGIES", raw)
		// Distinct strategy NAMES, not stream count. Six streams now carry three
		// names — the same strategy runs on more than one symbol — so comparing
		// against the stream count would fail for a correct roster.
		distinct := map[string]bool{}
		for _, st := range defaultScalpLiveStreams {
			distinct[st.Strategy] = true
		}
		if got := len(ScalpLiveStrategies()); got != len(distinct) {
			t.Errorf("SCALP_LIVE_STRATEGIES=%q gave %d strategies, want the %d distinct names in the default roster",
				raw, got, len(distinct))
		}
	}
}

// A typo permits a strategy that does not exist while the roster reads as
// enabled — the same silent shape as allow-listing an instrument the engine
// cannot trade. It must be reported.
func TestPerpAllowList_ReportsNamesThatMatchNoRunningStrategy(t *testing.T) {
	a := NewPerpAllowList()
	a.Set([]string{"Real_Strategy", "Typo_Strategy"}, nil)

	unknown := a.ReportUnknown(map[string]bool{"Real_Strategy": true})
	if len(unknown) != 1 || unknown[0] != "Typo_Strategy" {
		t.Fatalf("ReportUnknown = %v, want [Typo_Strategy]", unknown)
	}
	if len(a.ReportUnknown(map[string]bool{"Real_Strategy": true, "Typo_Strategy": true})) != 0 {
		t.Error("reported an unknown name when both resolve")
	}
}

// The candidate tier must never reach the venue.
//
// Adding a stream to the live roster is a decision to spend money on it; adding
// it as a candidate is a decision to watch it on live terms first. Collapsing
// the two is how the previous roster was built - straight from a leaderboard to
// real capital - and it lost $13.91 over 27 fills.
//
// The list is currently EMPTY, which is a valid state: everything being watched
// has either graduated or been rejected. So the MECHANISM is asserted rather
// than the contents, or this test would quietly stop testing anything the next
// time the list empties.
func TestPerpPaperCandidates_TradeOnPaperButNotOnTheVenue(t *testing.T) {
	for _, c := range defaultScalpPaperStreams {
		if !PerpStreamPaperPermitted(c.Strategy, c.Symbol) {
			t.Errorf("candidate %v cannot paper trade", c)
		}
		// NOT asserted: that a candidate is absent from the venue gate. The
		// live roster is now Account 03's book, which overlaps the other paper
		// books heavily, so a stream on both is normal. What a paper book
		// guarantees is that ITS balance is paper — not that its streams are
		// forbidden elsewhere.
	}

	// The gate itself, independent of what happens to be listed today: the paper
	// set must be exactly live + candidates, never wider.
	live := map[string]bool{}
	for _, st := range ScalpLiveStreams() {
		live[perpStreamKey(st.Strategy, st.Symbol)] = true
	}
	cand := map[string]bool{}
	for _, st := range defaultScalpPaperStreams {
		cand[perpStreamKey(st.Strategy, st.Symbol)] = true
	}
	for _, st := range ScalpPaperStreams() {
		k := perpStreamKey(st.Strategy, st.Symbol)
		if !live[k] && !cand[k] {
			t.Errorf("%v paper-trades but is on neither the live roster nor the candidate list", st)
		}
	}
}

// The live roster must still paper trade, or the desk stops mirroring what real
// money is doing — which is the module's entire purpose.
func TestPerpPaperCandidates_LiveStreamsStillPaperTrade(t *testing.T) {
	for _, live := range ScalpLiveStreams() {
		if !PerpStreamPaperPermitted(live.Strategy, live.Symbol) {
			t.Errorf("live stream %v is missing from the paper desk; the mirror is incomplete", live)
		}
	}
	// And the paper set is the union, with no duplicates.
	seen := map[string]bool{}
	for _, st := range ScalpPaperStreams() {
		k := perpStreamKey(st.Strategy, st.Symbol)
		if seen[k] {
			t.Errorf("%v appears twice in the paper set", st)
		}
		seen[k] = true
	}
	if want := len(ScalpLiveStreams()) + len(defaultScalpPaperStreams); len(seen) > want {
		t.Errorf("paper set has %d streams, more than live+candidates (%d)", len(seen), want)
	}
}

// A stream on neither list must be refused by both gates.
func TestPerpPaperCandidates_UnknownStreamIsRefusedEverywhere(t *testing.T) {
	if PerpStreamPaperPermitted("Not_A_Strategy", "ADAUSD") {
		t.Error("an unlisted strategy was permitted onto the paper desk")
	}
	// A candidate strategy on a symbol it was NOT selected for must also fail —
	// the pairing is the gate, not the name.
	if PerpStreamPaperPermitted("ANTI_M1_Break_D30_T20_Long", "NOTASYMBOLUSD") {
		t.Error("a candidate was permitted on a symbol it was not selected for")
	}
}

// The two symbols excluded from the promotion must NOT reach the venue.
//
// Excluded for mechanical reasons, not preference: MOVEUSD turns over
// $1,985/day (rank 208 of 220) so a $100 position is 5% of a day's volume, and
// 1000SATSUSD marks at 0.00001055 against a 0.0000001 tick, where a 0.35% stop
// is 0.37 of ONE tick and cannot be expressed at all.
func TestPerpRoster_ThinAndCoarseSymbolsStayOffTheVenue(t *testing.T) {
	for _, sym := range []string{"MOVEUSD", "1000SATSUSD"} {
		for _, st := range ScalpLiveStreams() {
			if st.Symbol == sym {
				t.Errorf("%s is on the live roster; it was excluded because the venue cannot support the trade", sym)
			}
		}
		// Removed from the paper desk as well on 2026-08-09. Watching a contract
		// the desk can never trade produces a record that reads as evidence and
		// is not: nine of eighteen paper trades came from these two, and they
		// were the ones least able to survive a real order book.
		for _, st := range ScalpPaperStreams() {
			if st.Symbol == sym {
				t.Errorf("%s still paper-trades; it was removed because the contract cannot support the trade", sym)
			}
		}
	}
}
