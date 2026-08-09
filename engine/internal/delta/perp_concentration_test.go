package delta

import (
	"math"
	"testing"
)

// Overshoot must reproduce the three live COOKIEUSD stop-outs.
//
// Those were the evidence that stops were still failing, so the measure is
// only trustworthy if it returns what was actually observed.
func TestPerpStopOvershoot_MatchesTheLiveStopOuts(t *testing.T) {
	cases := []struct {
		name              string
		entry, stop, exit float64
		want              float64
	}{
		// Closed exactly on its stop — the bracket work landing.
		{"exact", 0.01249, 0.01257, 0.01257, 1.00},
		// Filled at the slippage limit: 0.5% of price on a 0.64% stop.
		{"slippage limit", 0.01251, 0.01259, 0.01265, 1.75},
		// No brackets; the 15s monitor closed it late.
		{"monitor fallback", 0.01250, 0.01258, 0.01262, 1.50},
	}
	for _, c := range cases {
		tr := &PerpLiveTrade{
			Side: SideSell, EntryPrice: c.entry, StopPrice: c.stop, ExitReason: "SL",
		}
		got := perpStopOvershoot(tr, c.exit)
		if math.Abs(got-c.want) > 0.01 {
			t.Errorf("%s: overshoot %.2fx, want %.2fx", c.name, got, c.want)
		}
	}
}

// A target or timeout exit has no planned-risk denominator. Reporting 1.0 for
// those would bury real stop overshoot in a crowd of meaningless ones.
func TestPerpStopOvershoot_OnlyMeasuresStops(t *testing.T) {
	for _, reason := range []string{"TP", "TTL", "EXTERNAL", ""} {
		tr := &PerpLiveTrade{
			Side: SideSell, EntryPrice: 0.0125, StopPrice: 0.01258, ExitReason: reason,
		}
		if got := perpStopOvershoot(tr, 0.0122); got != 0 {
			t.Errorf("exit reason %q reported overshoot %.2f; only stops have a planned-risk denominator", reason, got)
		}
	}
}

func TestPerpStopOvershoot_FailsClosedOnBadInput(t *testing.T) {
	if perpStopOvershoot(nil, 1) != 0 {
		t.Error("nil trade")
	}
	// Entry == stop: dividing by zero planned risk must not produce +Inf.
	tr := &PerpLiveTrade{Side: SideSell, EntryPrice: 0.0125, StopPrice: 0.0125, ExitReason: "SL"}
	if got := perpStopOvershoot(tr, 0.0126); got != 0 || math.IsInf(got, 0) {
		t.Errorf("zero planned risk returned %v", got)
	}
}

// The per-symbol cap defaults to one, because the concentration observed was
// same-symbol AND same-direction.
func TestPerpConfig_CapsOnePositionPerSymbolByDefault(t *testing.T) {
	cfg := DefaultPerpRiskConfig(100)
	if cfg.MaxPositionsPerSymbol != 1 {
		t.Errorf("MaxPositionsPerSymbol = %d; three COOKIEUSD shorts in 13 minutes is what anything higher permits",
			cfg.MaxPositionsPerSymbol)
	}
	// It must still be stricter than the overall cap, or it is decorative.
	if cfg.MaxPositionsPerSymbol >= cfg.MaxConcurrentPositions {
		t.Errorf("per-symbol cap %d does not constrain the overall cap %d",
			cfg.MaxPositionsPerSymbol, cfg.MaxConcurrentPositions)
	}
}
