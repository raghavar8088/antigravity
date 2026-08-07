package delta

import (
	"sort"
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
	want := []PerpStream{
		{Strategy: "ANTI_M1_Break_D30_T20_Long", Symbol: "AVAXUSD"},
		{Strategy: "ANTI_M1_Break_D60_T50_Long", Symbol: "AVAXUSD"},
		{Strategy: "ANTI_M1_DoubleBottom_10bp_Long", Symbol: "ADAUSD"},
	}
	got := append([]PerpStream(nil), ScalpLiveStreams()...)
	sort.Slice(got, func(i, j int) bool {
		if got[i].Strategy != got[j].Strategy {
			return got[i].Strategy < got[j].Strategy
		}
		return got[i].Symbol < got[j].Symbol
	})
	if len(got) != len(want) {
		t.Fatalf("live roster has %d streams, the owner selected %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("stream[%d] = %v, want %v — the live allow-list drifted from the selection", i, got[i], want[i])
		}
	}

	// Every previously live strategy must be gone. Each was selected under an
	// earlier basis and is not on this list.
	live := map[string]bool{}
	for _, st := range got {
		live[st.Strategy] = true
	}
	for _, gone := range []string{
		"ANTI_D20_VWAP_Reversion", "ANTI_M1_DoubleTop_10bp_Short", "ANTI_M1_DoubleTop_20bp_Short",
		"ANTI_M1_HMA21_Flip_Long", "ANTI_M1_HMA21_Flip_Short", "ANTI_M1_InsideBar_V20_Long",
		"ANTI_M1_VWAP_Doji_Short", "ANTI_D20_EMA_Cross_9_21", "Historical_Vol_Percentile_Breakout",
		"M1X_Squeeze_Break_Short", "ANTI_M1_RSI2_5_95_T50_Short", "ANTI_M1_NR7_Expand_T50_Long",
		"ANTI_M1_BB_Rev_CMF5_Long", "ANTI_M1X_VWAP_TrendPull_Long",
	} {
		if live[gone] {
			t.Errorf("%q was removed from the live roster but is back on it", gone)
		}
	}
}

// The cross product is the bug this replaces. Three selected rows must enable
// three streams — not the six that strategies x symbols produces.
func TestPerpAllowList_PairsDoNotCrossProduct(t *testing.T) {
	a := NewPerpAllowList()
	a.SetPairs(ScalpLiveStreams())

	for _, ok := range []PerpStream{
		{"ANTI_M1_DoubleBottom_10bp_Long", "ADAUSD"},
		{"ANTI_M1_Break_D30_T20_Long", "AVAXUSD"},
		{"ANTI_M1_Break_D60_T50_Long", "AVAXUSD"},
	} {
		if !a.Allowed(ok.Strategy, ok.Symbol) {
			t.Errorf("selected stream %v was blocked", ok)
		}
	}
	// The pairings the cross product would have invented. Each is a real,
	// tradeable stream on the paper desk — which is exactly why permitting one
	// unasked would have gone unnoticed.
	for _, bad := range []PerpStream{
		{"ANTI_M1_DoubleBottom_10bp_Long", "AVAXUSD"},
		{"ANTI_M1_Break_D30_T20_Long", "ADAUSD"},
		{"ANTI_M1_Break_D60_T50_Long", "ADAUSD"},
	} {
		if a.Allowed(bad.Strategy, bad.Symbol) {
			t.Errorf("unselected pairing %v reached the venue — the cross product is back", bad)
		}
	}
	// Symbol matching stays case-insensitive.
	if !a.Allowed("ANTI_M1_Break_D30_T20_Long", "avaxusd") {
		t.Error("lowercase symbol was rejected")
	}
	// And an unknown strategy is still denied.
	if a.Allowed("Some_Other_Strategy", "ADAUSD") {
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
		if len(ScalpLiveStrategies()) != len(defaultScalpLiveStreams) {
			t.Errorf("SCALP_LIVE_STRATEGIES=%q did not fall back to the default list", raw)
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
