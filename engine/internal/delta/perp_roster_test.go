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
// The live roster is an OWNER DECISION, pinned exactly.
//
// Not "at least these" — exactly these. The previous version asserted only that
// the original eight survived, so the list could grow without anyone deciding
// to and the test would still pass. A roster that can drift upward silently is
// the one control here that spends money.
//
// Replaced 2026-08-02: the earlier selection was made on the desk's old basis
// ($3,000 notional, maker fees). These are the streams that survive restatement
// onto a real $100 account with taker fees.
func TestScalpLiveStrategies_MatchTheOwnerSelectionExactly(t *testing.T) {
	want := []string{
		"ANTI_D20_VWAP_Reversion",
		"ANTI_M1_Break_D30_T20_Long",
		"ANTI_M1_Break_D60_T50_Long",
		"ANTI_M1_DoubleBottom_10bp_Long",
		"ANTI_M1_DoubleTop_10bp_Short",
		"ANTI_M1_DoubleTop_20bp_Short",
		"ANTI_M1_HMA21_Flip_Long",
		"ANTI_M1_HMA21_Flip_Short",
		"ANTI_M1_InsideBar_V20_Long",
		"ANTI_M1_VWAP_Doji_Short",
	}
	got := append([]string(nil), ScalpLiveStrategies()...)
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("live scalp roster has %d strategies, the owner selected %d: got %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("roster[%d] = %q, want %q — the live allow-list drifted from the owner's selection", i, got[i], want[i])
		}
	}

	// The strategies dropped in this revision must stay dropped. Each was
	// selected under the old basis and is not on the restated list.
	for _, gone := range []string{
		"ANTI_D20_EMA_Cross_9_21",
		"Historical_Vol_Percentile_Breakout",
		"ANTI_M1X_VWAP_TrendPull_Long",
		"M1X_Squeeze_Break_Short",
		"ANTI_M1_BB_Rev_CMF5_Long",
		"ANTI_M1_RSI2_5_95_T50_Short",
		"ANTI_M1_NR7_Expand_T50_Long",
	} {
		for _, g := range got {
			if g == gone {
				t.Errorf("%q was removed from the live roster but is back on it", gone)
			}
		}
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
		if len(ScalpLiveStrategies()) != len(defaultScalpLiveStrategies) {
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
