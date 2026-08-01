package liveengine

import (
	"math"
	"testing"
)

func TestEligibility_NakedShortIsExcluded(t *testing.T) {
	// A short (non long-premium) type fails gate 1 outright, whatever its record.
	e := EvaluateEligibility(StrategyInput{
		Strategy:   "Intraday_CallSell_X",
		OptionType: "SHORT_CALL",
		RealFills:  500, RealDays: 60, RealExpectancy: 5, RealPF: 2.0, RealFeePct: 0.1,
	})
	if e.Live {
		t.Fatal("a naked short must never be live-eligible")
	}
	if e.Gates[0].Name != "long_premium_only" || e.Gates[0].Pass {
		t.Fatalf("long_premium_only must fail for a short, got %+v", e.Gates[0])
	}
}

func TestEligibility_ZeroRealFillsNotLive(t *testing.T) {
	// Great synthetic numbers, zero real fills → not live, with a clear reason.
	e := EvaluateEligibility(StrategyInput{
		Strategy:        "Intraday_PutBuy_RSIOverboughtExtreme_150m",
		OptionType:      "PUT",
		SyntheticTrades: 500, SyntheticPnL: 2000,
		RealFills: 0, RealDays: 0,
	})
	if e.Live {
		t.Fatal("synthetic performance must not qualify for real money")
	}
	if e.Reason == "" {
		t.Fatal("a non-live strategy must carry an inspectable reason")
	}
}

func TestEligibility_FullRealRecordIsLive(t *testing.T) {
	e := EvaluateEligibility(StrategyInput{
		Strategy:   "Swing_CallBuy_OverextensionFadeDown_600m",
		OptionType: "CALL",
		RealFills:  250, RealDays: 35, RealExpectancy: 0.12, RealPF: 1.4, RealFeePct: 0.3,
	})
	if !e.Live {
		t.Fatalf("a full real record meeting every gate must be live; reason=%s", e.Reason)
	}
}

func TestEligibility_FailsOnEachGate(t *testing.T) {
	base := StrategyInput{
		Strategy: "s", OptionType: "CALL",
		RealFills: 250, RealDays: 35, RealExpectancy: 0.1, RealPF: 1.4, RealFeePct: 0.3,
	}
	// Negative expectancy
	e := base
	e.RealExpectancy = -0.01
	if EvaluateEligibility(e).Live {
		t.Fatal("negative expectancy must fail")
	}
	// PF below 1.1
	e = base
	e.RealPF = 1.0
	if EvaluateEligibility(e).Live {
		t.Fatal("PF < 1.1 must fail")
	}
	// Fees above 0.5%
	e = base
	e.RealFeePct = 0.6
	if EvaluateEligibility(e).Live {
		t.Fatal("fees > 0.5% must fail")
	}
	// Sample too small
	e = base
	e.RealFills = 100
	if EvaluateEligibility(e).Live {
		t.Fatal("< 200 fills must fail go_live_sample")
	}
}

func TestRoundTripCostPctOfPremium(t *testing.T) {
	// $64 notional, 0.03% fee/side, capped at 10% of premium.
	// $1.78 premium: perSide = min(64*0.0003, 1.78*0.10)=min(0.0192,0.178)=0.0192;
	// round-trip = 2*0.0192/1.78*100 ≈ 2.16%.
	got := RoundTripCostPctOfPremium(1.78, 64, 0.0003, 0.10)
	if math.Abs(got-2.157) > 0.05 {
		t.Fatalf("expected ~2.16%%, got %.3f%%", got)
	}
	// $0.19 premium: cap binds — perSide = min(0.0192, 0.019)=0.019;
	// round-trip = 2*0.019/0.19*100 = 20%.
	got = RoundTripCostPctOfPremium(0.19, 64, 0.0003, 0.10)
	if math.Abs(got-20.0) > 0.1 {
		t.Fatalf("expected cap-bound ~20%%, got %.3f%%", got)
	}
}
