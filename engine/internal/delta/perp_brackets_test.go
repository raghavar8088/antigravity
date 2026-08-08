package delta

import (
	"math"
	"testing"
)

// Levels must be re-derived from the price actually FILLED.
//
// The plan's stop and target are distances measured from the paper desk's
// entry. A live market order fills wherever the book is, and inheriting the
// plan's absolute prices leaves the distances wrong from the first second —
// wrong asymmetrically, because a fill that slipped against the position moves
// the target closer and the stop further away.
//
// Measured on 2026-08-08: take-profits triggering on a 0.05% move against a
// 0.350% target. Five "wins" that were losses after fees.
func TestPerpLevelsFromFill_AnchorToTheFillNotThePlan(t *testing.T) {
	// Paper entered at 0.2000 with a 0.7% stop. The live order slipped to 0.2010.
	plan := PerpOrderPlan{
		Side:        SideSell,
		LimitPrice:  0.2000,
		StopPrice:   0.2014, // 0.70% above, for a short
		TargetPrice: 0.1993,
	}
	const fill = 0.2010

	stop, target := perpLevelsFromFill(fill, plan)

	risk := math.Abs(fill - stop)
	reward := math.Abs(fill - target)
	wantRisk := math.Abs(plan.LimitPrice - plan.StopPrice)

	if math.Abs(risk-wantRisk) > 1e-9 {
		t.Errorf("risk distance %.6f, want the plan's %.6f — the strategy's own risk must be preserved", risk, wantRisk)
	}
	if rr := reward / risk; math.Abs(rr-PerpRewardRisk) > 1e-9 {
		t.Errorf("R:R = 1:%.4f, want 1:%.1f", rr, PerpRewardRisk)
	}
	// Sides must be right for a SHORT: stop above the fill, target below.
	if stop <= fill {
		t.Errorf("short stop %.6f is not above the fill %.6f", stop, fill)
	}
	if target >= fill {
		t.Errorf("short target %.6f is not below the fill %.6f", target, fill)
	}
	// And the inherited absolute levels must NOT survive.
	if math.Abs(stop-plan.StopPrice) < 1e-9 {
		t.Error("the stop is still the plan's absolute price; it was not re-anchored to the fill")
	}
}

func TestPerpLevelsFromFill_LongSidesAreCorrect(t *testing.T) {
	plan := PerpOrderPlan{Side: SideBuy, LimitPrice: 100, StopPrice: 99, TargetPrice: 103}
	stop, target := perpLevelsFromFill(101, plan)
	if stop >= 101 {
		t.Errorf("long stop %.4f is not below the fill", stop)
	}
	if target <= 101 {
		t.Errorf("long target %.4f is not above the fill", target)
	}
	if rr := (target - 101) / (101 - stop); math.Abs(rr-PerpRewardRisk) > 1e-9 {
		t.Errorf("long R:R = 1:%.4f, want 1:%.1f", rr, PerpRewardRisk)
	}
}

// Degenerate inputs must fall back to the plan rather than inventing levels.
// A zero or negative fill means we do not know where we traded, and guessing
// there is how a position ends up with a stop on the wrong side of the market.
func TestPerpLevelsFromFill_FailsBackNotForward(t *testing.T) {
	plan := PerpOrderPlan{Side: SideBuy, LimitPrice: 100, StopPrice: 99, TargetPrice: 103}
	for _, fill := range []float64{0, -1} {
		s, tg := perpLevelsFromFill(fill, plan)
		if s != plan.StopPrice || tg != plan.TargetPrice {
			t.Errorf("fill %v produced (%v, %v); want the plan's own levels", fill, s, tg)
		}
	}
	// A plan with no risk distance cannot be scaled either.
	flat := PerpOrderPlan{Side: SideBuy, LimitPrice: 100, StopPrice: 100, TargetPrice: 103}
	if s, tg := perpLevelsFromFill(101, flat); s != flat.StopPrice || tg != flat.TargetPrice {
		t.Errorf("a zero-risk plan produced (%v, %v); want its own levels", s, tg)
	}
}

// The house ratio must be the one the paper desk uses, or the two disagree
// about what a strategy is.
func TestPerpRewardRisk_MatchesTheHouseRatio(t *testing.T) {
	if PerpRewardRisk != 3.0 {
		t.Errorf("PerpRewardRisk = %v, want 3.0 — breakeven win rate would be %.1f%%",
			PerpRewardRisk, 100/(1+PerpRewardRisk))
	}
	// At 1:3 a strategy needs 25% to break even before costs. The mirrors'
	// previous 1:0.50 needed 66.7% and delivered 33.3% with real money.
	if be := 100 / (1 + PerpRewardRisk); be > 26 {
		t.Errorf("breakeven win rate is %.1f%%; the ratio is not doing its job", be)
	}
}
