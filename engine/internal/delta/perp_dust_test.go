package delta

import (
	"errors"
	"testing"
)

// A book at its ceiling must refuse, not shrink to dust.
//
// Measured live on 2026-08-09: SKYAIUSD opened at $299.1 of a $300 ceiling, and
// the next two signals were sized into the remainder — COOKIEUSD at $0.44 and
// MUBARAKUSD at $0.25 against an intended $300. Both paid full round-trip fees,
// each held one of three concurrency slots, and each wrote a fill worth
// -$0.002 into its strategy's record: results that look like evidence and carry
// none.
func TestPlanPerpOrder_RefusesDustWhenTheBookIsNearlyFull(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100) // $300 ceiling at 3x

	// The live shape: $299.10 open against a $300 ceiling leaves $0.90 — 0.3% of
	// the intended notional. MUBARAKUSD is not in the test fixture, so ADAUSD
	// stands in; the symbol is incidental, the remaining-room ratio is the point.
	_, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, 0.17258828, 0.17198, 0.17380, 1, 299.10)
	if !errors.Is(err, ErrAggregateExposureReached) {
		t.Fatalf("a $0.90 remainder produced err=%v; it must be refused rather than opened as dust", err)
	}
}

// A genuinely partial position is still a position — the aggregate cap doing
// its job — and must not be refused along with the dust.
func TestPlanPerpOrder_StillAllowsARealPartial(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)

	// $150 open leaves $150 of a $300 ceiling: half the book, plainly tradable.
	plan, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, 0.17258828, 0.17198, 0.17380, 1, 150)
	if err != nil {
		t.Fatalf("half the book free was refused: %v", err)
	}
	if plan.Contracts <= 0 {
		t.Fatal("partial plan produced no contracts")
	}
}

// The existing full-ceiling refusal must keep working.
func TestPlanPerpOrder_RefusesAtTheCeiling(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)
	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, 0.17258828, 0.17198, 0.17380, 1, 300); !errors.Is(err, ErrAggregateExposureReached) {
		t.Fatalf("a full book returned err=%v", err)
	}
}
