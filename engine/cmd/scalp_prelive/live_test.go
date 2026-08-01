package main

import "testing"

// The desk shipped paper-only — "no order routing, no API keys, no real-money
// code path in this binary". That is now conditional, so the conditions
// themselves are what need pinning.

// Live trading must be OFF unless explicitly switched on. A desk that starts
// routing orders because an env var was forgotten is not a desk anyone should
// run.
func TestScalpLive_DefaultsOff(t *testing.T) {
	t.Setenv("SCALP_LIVE_ENABLED", "")
	if scalpLiveEnabled() {
		t.Fatal("live trading enabled with SCALP_LIVE_ENABLED unset")
	}
	for _, raw := range []string{"", " ", "no", "0", "false", "maybe", "yes"} {
		t.Setenv("SCALP_LIVE_ENABLED", raw)
		if raw == "0" || raw == "false" || raw == "" || raw == " " || raw == "maybe" || raw == "no" || raw == "yes" {
			if scalpLiveEnabled() {
				t.Errorf("SCALP_LIVE_ENABLED=%q enabled live trading; only an explicit true may", raw)
			}
		}
	}
	t.Setenv("SCALP_LIVE_ENABLED", "true")
	if !scalpLiveEnabled() {
		t.Error("an explicit true did not enable live trading")
	}
}

// A nil live desk is the normal paper-only state, and every method must tolerate
// it. If any of these panicked, disabling live trading would crash the desk.
func TestScalpLive_NilDeskIsSafe(t *testing.T) {
	var d *liveDesk
	d.onPaperFill("ANTI_M1_DoubleTop_10bp_Short", "ADAUSD", &position{Dir: "LONG", Entry: 1, SL: 0.9, TP: 1.1})
	d.reportUnknown([]string{"anything"})
	// No assertion beyond "did not panic": the paper desk must be entirely
	// unaffected by live trading being off.
}

// The account defaults to the $100 the owner specified.
func TestScalpLive_EquityDefaultsToOneHundred(t *testing.T) {
	t.Setenv("SCALP_LIVE_EQUITY_USD", "")
	if got := scalpLiveEquityUSD(); got != 100 {
		t.Errorf("equity $%.2f, want $100", got)
	}
	t.Setenv("SCALP_LIVE_EQUITY_USD", "250")
	if got := scalpLiveEquityUSD(); got != 250 {
		t.Errorf("override gave $%.2f", got)
	}
	// A nonsensical value must fall back rather than sizing against garbage.
	for _, raw := range []string{"abc", "-5", "0"} {
		t.Setenv("SCALP_LIVE_EQUITY_USD", raw)
		if got := scalpLiveEquityUSD(); got != 100 {
			t.Errorf("SCALP_LIVE_EQUITY_USD=%q gave $%.2f, want the $100 default", raw, got)
		}
	}
}

// Symbols must default to the two the owner's strategies were leading on, not to
// "any symbol" — the same strategy runs on eight symbols here.
func TestScalpLive_SymbolsDefaultToTheSelectedTwo(t *testing.T) {
	t.Setenv("SCALP_LIVE_SYMBOLS", "")
	got := scalpLiveSymbols()
	if len(got) != 2 {
		t.Fatalf("default symbols = %v, want exactly the two selected", got)
	}
	seen := map[string]bool{got[0]: true, got[1]: true}
	if !seen["ADAUSD"] || !seen["BNBUSD"] {
		t.Errorf("default symbols = %v, want ADAUSD and BNBUSD", got)
	}
}
