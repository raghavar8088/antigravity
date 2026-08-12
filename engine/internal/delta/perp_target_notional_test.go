package delta

import (
	"errors"
	"math"
	"testing"
)

// Positions must land near the target regardless of what a contract costs.
//
// Per-contract cost spans 744x on the live roster, so "1 contract" carried
// wildly unequal risk: a stop-out cost a fraction of a rupee on the cheap end
// and twenty on the dear end, and the desk's P&L was decided by whichever
// expensive coin happened to trade.
func TestPlanPerpOrder_TargetNotionalSizesToTheTarget(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(10)
	cfg.TargetNotionalUSD = 3

	// ADAUSD at ~0.1726 with contract value 1 — about $0.17 a contract.
	entry := 0.17258828
	plan, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*0.98, entry*1.06, 0, 0)
	if err != nil {
		t.Fatalf("target-notional plan failed: %v", err)
	}
	if math.Abs(plan.NotionalUSD-3) > 3*0.2 {
		t.Errorf("notional $%.4f is not within 20%% of the $3 target", plan.NotionalUSD)
	}
	if plan.Contracts < 2 {
		t.Errorf("contracts = %d; a $0.17 contract needs many to reach $3", plan.Contracts)
	}
}

// A contract dearer than the target cannot be split — Delta has no fractional
// contracts — so it must still trade at one rather than being dropped.
func TestPlanPerpOrder_TargetNotionalFloorsAtOneContract(t *testing.T) {
	reg := riskTestRegistry(t)
	// A book with room for it: BEATUSD costs $10.40 a contract against a $30
	// ceiling, so the live case fits. $100 of equity here keeps the fixture's
	// pricier BTCUSD in the same position.
	cfg := DefaultPerpRiskConfig(100)
	cfg.TargetNotionalUSD = 3

	// BTCUSD: 0.001 BTC a contract at ~$63k is ~$63, far over the $3 target.
	entry := 63083.98
	plan, err := PlanPerpOrder(reg, cfg, "BTCUSD", true, entry, entry*0.98, entry*1.06, 0, 0)
	if err != nil {
		t.Fatalf("a contract dearer than the target was refused: %v", err)
	}
	if plan.Contracts != 1 {
		t.Errorf("contracts = %d, want exactly 1 — rounding down would send zero and silently drop the symbol", plan.Contracts)
	}
}

// A symbol whose single contract exceeds the WHOLE book ceiling can never
// trade, and must be refused rather than opened over the limit.
//
// Nothing on the live roster is in this state — BEATUSD is the dearest at
// $10.40 against a $30 ceiling — but it is one price move away, and the failure
// would otherwise be a position larger than the book the desk is allowed.
func TestPlanPerpOrder_TargetNotionalRefusesWhatCannotFitAtAll(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(10) // $30 ceiling
	cfg.TargetNotionalUSD = 3

	entry := 63083.98 // ~$63 a contract, over the entire ceiling
	if _, err := PlanPerpOrder(reg, cfg, "BTCUSD", true, entry, entry*0.98, entry*1.06, 0, 0); !errors.Is(err, ErrAggregateExposureReached) {
		t.Fatalf("a $63 contract was allowed onto a $30 book: %v", err)
	}
}

// The book ceiling still binds, and a position that does not fit is REFUSED
// rather than shrunk — shrinking reintroduces the dust the target prevents.
func TestPlanPerpOrder_TargetNotionalRespectsTheBookCeiling(t *testing.T) {
	reg := riskTestRegistry(t)
	cfg := DefaultPerpRiskConfig(10) // $30 ceiling at 3x
	cfg.TargetNotionalUSD = 3

	entry := 0.17258828
	// $29 already open leaves $1 — under a $3 position.
	_, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*0.98, entry*1.06, 1, 29)
	if !errors.Is(err, ErrAggregateExposureReached) {
		t.Fatalf("a $3 position was allowed against $1 of room: %v", err)
	}

	// With the book empty it must go through.
	if _, err := PlanPerpOrder(reg, cfg, "ADAUSD", true, entry, entry*0.98, entry*1.06, 0, 0); err != nil {
		t.Fatalf("an empty book refused a target-sized position: %v", err)
	}
}

// Ten concurrent $3 positions must fit the $30 ceiling exactly — the sizing and
// the concurrency cap are chosen together, and a change to either that breaks
// the relationship should fail here rather than in production.
func TestPlanPerpOrder_TargetTimesConcurrencyFitsTheBook(t *testing.T) {
	cfg := DefaultPerpRiskConfig(10)
	cfg.TargetNotionalUSD = 3
	cfg.MaxConcurrentPositions = 10

	ceiling := cfg.EquityUSD * cfg.MaxAggregateLeverage
	needed := float64(cfg.MaxConcurrentPositions) * cfg.TargetNotionalUSD
	if needed > ceiling {
		t.Errorf("%d positions of $%.2f need $%.2f but the ceiling is $%.2f — the last positions would always be refused",
			cfg.MaxConcurrentPositions, cfg.TargetNotionalUSD, needed, ceiling)
	}
}
