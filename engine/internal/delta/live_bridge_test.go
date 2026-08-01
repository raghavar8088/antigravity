package delta

import (
	"reflect"
	"testing"
)

// The Live Engine is buying-only from the buying engine (native long signals).
// Buying the wrong option type = the wrong trade, so pin the direction logic.
func TestResolveLiveOptionSide(t *testing.T) {
	cases := []struct {
		name               string
		sigType            string
		buying, nativeBuy  bool
		wantType, wantSide string
	}{
		// Native buy source (the option BUYING engine): buy the exact type.
		{"native buy CALL", "CALL", true, true, "CALL", "buy"},
		{"native buy PUT", "PUT", true, true, "PUT", "buy"},
		// Legacy mirror of the selling engine in buy mode: invert.
		{"legacy buy inverts PUT→CALL", "PUT", true, false, "CALL", "buy"},
		{"legacy buy inverts CALL→PUT", "CALL", true, false, "PUT", "buy"},
		// Sell mode: sell the exact type.
		{"sell CALL", "CALL", false, false, "CALL", "sell"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotType, gotSide := resolveLiveOptionSide(c.sigType, c.buying, c.nativeBuy)
			if gotType != c.wantType || gotSide != c.wantSide {
				t.Fatalf("got (%s,%s), want (%s,%s)", gotType, gotSide, c.wantType, c.wantSide)
			}
		})
	}
}

func TestLiveAllowList_RestrictsAndRoundTrips(t *testing.T) {
	// This test is about the ALLOW-LIST, so the master options switch is turned
	// on to isolate it. With the switch off nothing is allowed regardless of the
	// list, which is asserted separately below.
	b := &Bridge{optionsTradingEnabled: true}

	// nil allow-list (never set) = allow all (legacy behavior).
	if !b.strategyAllowedLocked("AnyStrategy") {
		t.Fatal("nil allow-list must allow all")
	}

	b.SetLiveAllowList([]string{"Swing_CallBuy_X", "Intraday_PutBuy_Y", ""})
	if got := b.LiveAllowList(); !reflect.DeepEqual(got, []string{"Intraday_PutBuy_Y", "Swing_CallBuy_X"}) {
		t.Fatalf("allow-list round-trip (sorted, empty dropped): got %v", got)
	}
	if !b.strategyAllowedLocked("Swing_CallBuy_X") {
		t.Fatal("listed strategy must be allowed")
	}
	if b.strategyAllowedLocked("Intraday_CallSell_Z") {
		t.Fatal("unlisted strategy must be blocked from live")
	}

	// Empty (but set) allow-list blocks everything — fail closed.
	b.SetLiveAllowList([]string{})
	if b.strategyAllowedLocked("Swing_CallBuy_X") {
		t.Fatal("empty allow-list must block all live orders")
	}
}

// The master options switch: off by default, and it outranks everything else.
//
// The failure it prevents: the Delta Engine toggle was armed for 112 minutes
// while the operator was watching the PERPETUAL desk. That toggle is shared —
// it backs account reads, the kill switch and the reconciler — so arming it is
// not consent to buy options. Before this gate, it silently re-permitted nine
// option strategies to spend real money.
func TestOptionsMasterSwitch_DefaultsOffAndOutranksTheAllowList(t *testing.T) {
	// A freshly constructed bridge trades nothing. The zero value must be the
	// safe one: a Bridge that is never configured cannot place an order.
	if (&Bridge{}).strategyAllowedLocked("Swing_CallBuy_X") {
		t.Fatal("a zero-value Bridge permitted an option order")
	}
	if NewBridge().OptionsTradingEnabled() {
		t.Fatal("NewBridge starts with option trading enabled")
	}

	// The switch outranks the allow-list in BOTH directions. This is the part
	// that matters: the roster API can edit the allow-list at runtime, so if the
	// list alone decided, adding a strategy there would resume live option
	// trading with nobody deciding to.
	b := &Bridge{}
	b.SetLiveAllowList([]string{"Swing_CallBuy_X"})
	if b.strategyAllowedLocked("Swing_CallBuy_X") {
		t.Error("an allow-listed strategy traded while the master switch was off")
	}

	// And with the switch on, the allow-list still governs — the switch permits,
	// it does not promote.
	b.SetOptionsTradingEnabled(true)
	if !b.strategyAllowedLocked("Swing_CallBuy_X") {
		t.Error("allow-listed strategy blocked after the master switch was turned on")
	}
	if b.strategyAllowedLocked("Some_Unlisted_Strategy") {
		t.Error("the master switch overrode the allow-list; it must only permit, never widen")
	}

	// Reversible, and reported honestly for the status surfaces.
	b.SetOptionsTradingEnabled(false)
	if b.OptionsTradingEnabled() || b.strategyAllowedLocked("Swing_CallBuy_X") {
		t.Error("turning the master switch back off did not stop option orders")
	}
}

// Arming the engine must not, on its own, permit an option order.
func TestOptionsMasterSwitch_ArmingTheEngineIsNotConsent(t *testing.T) {
	b := &Bridge{openByPaperID: map[string]string{}, configured: true, buyingMode: true}
	b.SetLiveAllowList([]string{"Swing_CallBuy_X"})

	b.SetEnabled(true) // the Delta Engine toggle
	if !b.enabled {
		t.Fatal("fixture: engine did not enable")
	}
	if b.strategyAllowedLocked("Swing_CallBuy_X") {
		t.Fatal("arming the Delta Engine re-permitted option trading by itself")
	}
}
