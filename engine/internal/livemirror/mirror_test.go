package livemirror

import (
	"testing"

	"antigravity-engine/internal/positions"
)

func newUnconfigured(cfg Config) *Mirror {
	// No DELTA_API_KEY/SECRET in the test env → New returns an unconfigured mirror.
	return New(cfg)
}

func TestContractsForMirrorSizing(t *testing.T) {
	m := newUnconfigured(Config{Symbol: "BTCUSD", MaxContracts: 5, Leverage: 10})

	cases := []struct {
		paperBTC float64
		contract float64
		want     int
	}{
		{0.001, 0.001, 1}, // pre-live default trade size → 1 contract
		{0.0004, 0.001, 1}, // below one contract rounds up to the 1-contract floor
		{0.003, 0.001, 3},
		{0.1, 0.001, 5}, // capped at MaxContracts
		{0.002, 0, 0},   // invalid contract value → no order
	}
	for _, c := range cases {
		if got := m.contractsFor(c.paperBTC, c.contract); got != c.want {
			t.Errorf("contractsFor(%v, %v) = %d, want %d", c.paperBTC, c.contract, got, c.want)
		}
	}
}

func TestContractsForFixedOverride(t *testing.T) {
	m := newUnconfigured(Config{Symbol: "BTCUSD", FixedContracts: 2, MaxContracts: 5})
	if got := m.contractsFor(0.5, 0.001); got != 2 {
		t.Errorf("fixed sizing: got %d, want 2", got)
	}
	m2 := newUnconfigured(Config{Symbol: "BTCUSD", FixedContracts: 9, MaxContracts: 5})
	if got := m2.contractsFor(0.5, 0.001); got != 5 {
		t.Errorf("fixed sizing above cap: got %d, want 5 (capped)", got)
	}
}

func TestEnableRequiresConfiguration(t *testing.T) {
	m := newUnconfigured(Config{Symbol: "BTCUSD", MaxContracts: 5})
	if m.IsConfigured() {
		t.Skip("delta keys present in test environment — skipping unconfigured assertions")
	}
	if m.IsEnabled() {
		t.Fatal("mirror must start disarmed")
	}
	if err := m.SetEnabled(true); err == nil {
		t.Fatal("arming without Delta keys must fail")
	}
	if m.IsEnabled() {
		t.Fatal("failed arm must leave mirror disarmed")
	}
	if err := m.SetEnabled(false); err != nil {
		t.Fatalf("disarming must always succeed: %v", err)
	}
}

func TestEventsIgnoredWhileDisarmed(t *testing.T) {
	m := newUnconfigured(Config{Symbol: "BTCUSD", MaxContracts: 5})
	pos := positions.Position{ID: "POS-1", StrategyName: "s", Side: "BUY", Size: 0.001, EntryPrice: 100000}

	m.OnPaperOpen(pos) // disarmed → must not enqueue
	select {
	case <-m.events:
		t.Fatal("open event enqueued while disarmed")
	default:
	}

	m.OnPaperClose(pos, "TAKE_PROFIT", 101000) // untracked → must not enqueue
	select {
	case <-m.events:
		t.Fatal("close event enqueued for untracked position")
	default:
	}
}
