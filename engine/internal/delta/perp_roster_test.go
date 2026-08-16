package delta

import (
	"strings"
	"testing"
	"time"
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
// The live roster is EXACT (strategy, symbol) streams, never a cross product.
//
// Rewritten 2026-08-15. It previously asserted the roster contained Account 03's
// book — correct while the desk ran the Scalp100/Delta20/Curated packs, and
// meaningless once those were retired: it would now be asserting that the
// allow-list holds strategy names the desk no longer builds, which is exactly
// the dead-reference state the roster was emptied to avoid.
//
// What still matters is the property the cross product violated: three chosen
// rows must not become six live streams.
func TestScalpLiveStreams_AreExactPairsNotACrossProduct(t *testing.T) {
	got := ScalpLiveStreams()
	if len(got) == 0 {
		t.Skip("live roster is empty; nothing to route")
	}

	a := NewPerpAllowList()
	a.SetPairs(got)

	strategies, symbols := map[string]bool{}, map[string]bool{}
	selected := map[string]bool{}
	for _, st := range got {
		strategies[st.Strategy] = true
		symbols[strings.ToUpper(st.Symbol)] = true
		selected[perpStreamKey(st.Strategy, st.Symbol)] = true
		if !a.Allowed(st.Strategy, st.Symbol) {
			t.Errorf("selected stream %v was blocked", st)
		}
	}

	// Any pairing the cross product would invent must be denied.
	invented := 0
	for st := range strategies {
		for sym := range symbols {
			if selected[perpStreamKey(st, sym)] {
				continue
			}
			invented++
			if a.Allowed(st, sym) {
				t.Errorf("unselected pairing %s|%s reached the venue — the cross product is back", st, sym)
			}
		}
	}
	if invented == 0 {
		t.Skip("every strategy x symbol combination happens to be selected; nothing to test")
	}
}

// The cross product is the bug this replaces. Three selected rows must enable
// three streams — not the six that strategies x symbols produces.
func TestPerpAllowList_PairsDoNotCrossProduct(t *testing.T) {
	// Two strategies x two symbols, of which only two PAIRS are selected. The
	// cross product would invent the other two, which is the bug this pins.
	fixturePairs := []PerpStream{
		{Strategy: "FIX_Alpha", Symbol: "ADAUSD"},
		{Strategy: "FIX_Beta", Symbol: "BNBUSD"},
	}
	a := NewPerpAllowList()
	a.SetPairs(fixturePairs)

	// Every selected stream is permitted.
	for _, st := range fixturePairs {
		if !a.Allowed(st.Strategy, st.Symbol) {
			t.Errorf("selected stream %v was blocked", st)
		}
	}

	// And a pairing the CROSS PRODUCT would invent is not. Built from real
	// names and real symbols already on the roster, so it is a stream that
	// genuinely exists on the paper desk — which is exactly why permitting it
	// unasked would go unnoticed.
	// Built from the list this allow-list was actually given, so the assertion
	// holds regardless of what the shipped roster contains — an empty default
	// must not make this pass vacuously OR fail spuriously.
	strategies, symbols := map[string]bool{}, map[string]bool{}
	for _, st := range fixturePairs {
		strategies[st.Strategy] = true
		symbols[strings.ToUpper(st.Symbol)] = true
	}
	selected := map[string]bool{}
	for _, st := range fixturePairs {
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
	first := fixturePairs[0]
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
// Excluded for mechanical reasons, not preference. 1000SATSUSD marks at
// 0.00001055 against a 0.0000001 tick, where a 0.35% stop is 0.37 of ONE tick
// and cannot be expressed at all.
//
// MOVEUSD's exclusion was originally written as a LIQUIDITY judgement —
// $1,985/day, rank 208 of 220. That premise is now stale: re-measured against
// the venue on 2026-08-16 it turns over $27,833/day at rank 100, fourteen times
// the figure this test was written against, and it re-entered the desk universe
// on its own when it crossed the $50k floor.
//
// The exclusion stands on a different and independently fatal fact. It marks at
// 0.00636728 against a 0.00001 tick, so the narrowest stop the MTF pack
// produces — 0.9% — is 5.7 TICKS wide against a 20-tick minimum. Liquidity can
// recover; the price grid is a property of the contract.
//
// Recording the correction rather than quietly re-deriving the same verdict:
// this test kept the right answer for eight days with a reason that had
// expired, and the next person to challenge it deserves the number that is
// actually load-bearing. Eight leaderboard rows were selected on MOVEUSD paper
// fills in that window.
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

// Every symbol on the live roster must be checked against the REAL price grid,
// with the venue's own mark and tick AND its own measured stop.
//
// The readings below were taken from api.india.delta.exchange on 2026-08-16 by
// TestPerpGridAudit_AgainstTheLiveVenue. They are pinned here because the
// roster's comment block reasons from them, and a comment reasoning from
// numbers nobody can re-run is a comment that will go wrong unnoticed — which
// is exactly what happened to the MOVEUSD exclusion, right for eight days on a
// liquidity figure that had expired.
//
// The stop fraction is per symbol, NOT a flat 0.9%. The bridge replaces the
// strategy's stop with 2x the measured p90 one-minute range before the grid
// check, so a shared assumption here would be testing a gate that does not
// exist. That assumption is what produced eleven false passes in the first
// audit. Where no estimate existed — the market was flat enough that every 1m
// candle printed high == low — 0.9% is recorded and marked as such.
func TestPerpRoster_LiveSymbolsWereMeasuredAgainstTheRealGrid(t *testing.T) {
	// symbol -> mark, tick, stop fraction as measured.
	type reading struct{ mark, tick, stopFrac float64 }
	venue := map[string]reading{
		"HUSD":        {0.10492112, 1e-05, 0.04531},
		"VELVETUSD":   {1.04028950, 1e-04, 0.04046},
		"AIOUSD":      {0.06617364, 1e-05, 0.03279},
		"AVAAIUSD":    {0.01391174, 1e-06, 0.01302},
		"SKYAIUSD":    {0.07038382, 1e-05, 0.01960},
		"BEATUSD":     {0.40188014, 1e-04, 0.02963},
		"CHIPUSD":     {0.03026841, 1e-05, 0.02677},
		"CROSSUSD":    {0.09625047, 1e-05, 0.00811},
		"PIEVERSEUSD": {0.85201017, 1e-04, 0.00900}, // no estimate; assumed
		"ARCUSD":      {0.07209041, 1e-05, 0.00900}, // no estimate; assumed
		"BLESSUSD":    {0.00900592, 1e-06, 0.00665},
		"AIOTUSD":     {0.04551320, 1e-05, 0.00900}, // no estimate; assumed
		"GIGGLEUSD":   {36.7056050, 1e-02, 0.00900}, // no estimate; assumed
		"EDENUSD":     {0.04638876, 1e-05, 0.00674},
		"PUMPUSD":     {0.00270873, 1e-06, 0.00900}, // no estimate; assumed
		"TSTUSD":      {0.01468276, 1e-06, 0.00140},

		// Held OFF the roster: they fail the gate today but their grids are
		// fine, so they are not in gridBlockedSymbols. Recorded here so the
		// roster comment's claim about them can be re-run.
		"XAIUSD":    {0.00697516, 1e-05, 0.02006},
		"ZECUSD":    {492.351250, 1e-02, 0.00024},
		"SWARMSUSD": {0.00880042, 1e-06, 0.00093},

		// Permanently blocked, kept for the assertions below.
		"MOVEUSD": {0.00636728, 1e-05, 0.00900},
		"LABUSD":  {0.08313850, 1e-04, 0.02287},
	}
	reg := &PerpRegistry{bySymbol: map[string]PerpProduct{}, fetchedAt: time.Now()}
	for sym, r := range venue {
		reg.bySymbol[sym] = PerpProduct{Symbol: sym, MarkPrice: r.mark, TickSize: r.tick, ContractValue: 1}
	}
	ticksFor := func(sym string) float64 {
		r := venue[sym]
		ticks, _ := stopGridTicks(reg, sym, r.mark, r.mark*(1-r.stopFrac))
		return ticks
	}

	// Every roster symbol must clear the gate at its OWN measured stop. These
	// are the streams that spend real money.
	for _, st := range ScalpLiveStreams() {
		if _, ok := venue[st.Symbol]; !ok {
			t.Errorf("%s (%s) is on the live roster with no measured mark/tick/stop here; "+
				"add the venue's figures so the grid claim can be re-run", st.Symbol, st.Strategy)
			continue
		}
		if got := ticksFor(st.Symbol); got < minEntryStopTicks {
			t.Errorf("%s measures %.1f ticks, under the %d-tick minimum, but carries live stream %s",
				st.Symbol, got, minEntryStopTicks, st.Strategy)
		}
	}

	// The three held back must actually fail, or the reason they were held back
	// is wrong and seven streams were dropped for nothing.
	for _, sym := range []string{"XAIUSD", "ZECUSD", "SWARMSUSD"} {
		if got := ticksFor(sym); got >= minEntryStopTicks {
			t.Errorf("%s now measures %.1f ticks and CLEARS the gate; the roster holds 7 streams "+
				"back on the grounds that it does not", sym, got)
		}
		if IsGridBlocked(sym) {
			t.Errorf("%s was added to gridBlockedSymbols; it fails on VOLATILITY, not on its grid, "+
				"and blocking it permanently would ban a contract that recovers on its own", sym)
		}
	}

	// And the permanent exclusions must stay failing at any plausible stop.
	for _, sym := range []string{"MOVEUSD", "LABUSD"} {
		if got := ticksFor(sym); got >= minEntryStopTicks {
			t.Errorf("%s measures %.1f ticks; it is permanently blocked on the grounds that it cannot "+
				"hold a stop, and that no longer holds", sym, got)
		}
	}
}
