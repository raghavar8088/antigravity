package main

import (
	"testing"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// scalpTestCandle is a minimal bar for exercising the fill path.
func scalpTestCandle() scalers.Candle { return scalers.Candle{} }

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

// Six of the eight strategies selected for live trading are ANTI_ mirrors. The
// only live hook was on the ORIGINAL's fill path, so those six were
// structurally unable to place an order — the bridge armed, the allow-list
// resolved, the desk traded, and nothing errored. This pins that a mirror's
// fill reaches the live path too.
func TestScalpLive_MirrorFillsReachTheLivePath(t *testing.T) {
	var offered []string
	observeFill = func(strategy, symbol string, pos *position) { offered = append(offered, strategy) }
	t.Cleanup(func() { observeFill = nil })

	d := &desk{combos: map[string]*comboState{}}
	pos := &position{Dir: "LONG", Entry: 0.17, SL: 0.169, TP: 0.172, Profile: "scalp"}
	d.combos[comboKey("TEST_Strat", "ADAUSD")] = &comboState{Eq: 1, Peak: 1, Pos: pos}

	d.openMirror("TEST_Strat", "ADAUSD", pos, 1, scalpTestCandle())

	found := false
	for _, n := range offered {
		if n == "ANTI_TEST_Strat" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the mirror's fill never reached the live path (offered: %v) — every ANTI_ strategy on the "+
			"live allow-list would be unable to trade, silently", offered)
	}
}

// The live bridge must inherit the SAME holding-period cap the paper desk uses
// for that strategy's profile, or the two records diverge on the exit that
// accounts for 91% of them.
func TestScalpLive_ProfileTTLMatchesTheDesk(t *testing.T) {
	for name, cfg := range profiles {
		want := time.Duration(cfg.TTLBars) * time.Minute
		if got := profileTTL(name); got != want {
			t.Errorf("profile %q TTL = %s, desk uses %d bars (%s)", name, got, cfg.TTLBars, want)
		}
	}
}

// An unknown profile must not mean "hold forever". Falling back to no cap would
// leave a live position open indefinitely on a desk whose median trade is
// closed by the clock.
func TestScalpLive_UnknownProfileStillGetsATimeStop(t *testing.T) {
	got := profileTTL("no-such-profile")
	if got <= 0 {
		t.Fatal("an unknown profile got no time stop; the position would hold indefinitely")
	}
	longest := time.Duration(0)
	for _, c := range profiles {
		if d := time.Duration(c.TTLBars) * time.Minute; d > longest {
			longest = d
		}
	}
	if got != longest {
		t.Errorf("unknown profile TTL = %s, want the longest configured cap %s", got, longest)
	}
}
