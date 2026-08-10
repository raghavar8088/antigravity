package delta

import (
	"errors"
	"testing"
)

// Fixed size means exactly that: the requested count, whatever risk sizing
// would have produced.
func TestPlanPerpOrder_FixedContractsIgnoresRiskSizing(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)
	cfg.FixedContracts = 1

	entry := 0.17258828
	plan, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*0.9936, entry*1.019, 0, 0)
	if err != nil {
		t.Fatalf("fixed-size plan failed: %v", err)
	}
	if plan.Contracts != 1 {
		t.Errorf("contracts = %d, want exactly 1", plan.Contracts)
	}
	// Risk sizing on a $100 account would have produced hundreds of contracts;
	// if it did, the override is not being honoured.
	if plan.NotionalUSD > 5 {
		t.Errorf("notional $%.2f — a one-contract order cannot be this large", plan.NotionalUSD)
	}
}

// The notional caps exist to stop a position being too LARGE. A minimum-size
// order cannot be, so they must not refuse it — the dust guard especially,
// which compares against an intended notional this mode never computes.
func TestPlanPerpOrder_FixedContractsSurvivesAFullBook(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)
	cfg.FixedContracts = 1

	entry := 0.17258828
	// $299.99 of a $300 ceiling: risk sizing would refuse this as dust.
	plan, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*0.9936, entry*1.019, 1, 299.99)
	if err != nil {
		t.Fatalf("a one-contract order was refused against a near-full book: %v", err)
	}
	if plan.Contracts != 1 {
		t.Errorf("contracts = %d, want 1", plan.Contracts)
	}
}

// Caps about COUNT still apply — a minimum-size position can still occupy a
// slot it should not.
func TestPlanPerpOrder_FixedContractsStillRespectsConcurrency(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)
	cfg.FixedContracts = 1

	entry := 0.17258828
	_, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*0.9936, entry*1.019, cfg.MaxConcurrentPositions, 0)
	if !errors.Is(err, ErrTooManyPositions) {
		t.Errorf("fixed size bypassed the concurrency cap: %v", err)
	}
}

// An inverted stop must still be rejected: fixed size skips the sizing maths,
// not the sanity checks that keep a "risk" from secretly being a gain.
func TestPlanPerpOrder_FixedContractsStillValidatesTheStop(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(100)
	cfg.FixedContracts = 1

	entry := 0.17258828
	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*1.01, entry*1.02, 0, 0); err == nil {
		t.Error("a long with its stop ABOVE entry was accepted in fixed-size mode")
	}
}
