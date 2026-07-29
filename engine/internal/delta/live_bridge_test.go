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
	b := &Bridge{}

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
