package trading

import (
	"context"
	"errors"
	"testing"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/strategy"
)

// The post-gate budget backstop must be un-skippable: an order whose pre-submit
// assertion fails is rejected at the choke point and the broker fill is never
// invoked. The inverse — no failing assertion — must reach the broker. This is
// the enforced home of AssertBuyWithinBudget (Phase C addition #1).
func TestSubmitInstitutionalOrder_FailedAssertBlocksBroker(t *testing.T) {
	o := &Orchestrator{}

	brokerCalled := false
	fillFn := func(context.Context, strategy.Signal, string) (execution.FillResult, error) {
		brokerCalled = true
		return execution.FillResult{}, nil
	}
	appendEvent := func(ledger.EventType, ledger.OrderPayload) (ledger.Event, error) {
		return ledger.Event{}, nil
	}
	sig := strategy.Signal{Symbol: "DELTA-OPT:CALL:64000", Action: strategy.ActionBuy}

	// Failing assertion → order rejected, broker never touched.
	_, err := o.submitInstitutionalOrder(
		context.Background(), sig, "strat", "AG-TEST-1", ledger.OrderPayload{},
		appendEvent, fillFn, 1.0, "test",
		func() error { return errors.New("order cost exceeds risk budget") },
	)
	if err == nil {
		t.Fatal("expected error when the pre-submit assertion fails")
	}
	if brokerCalled {
		t.Fatal("broker fill must NOT be reached when the pre-submit assertion fails")
	}

	// No failing assertion → order proceeds to the broker.
	brokerCalled = false
	if _, err := o.submitInstitutionalOrder(
		context.Background(), sig, "strat", "AG-TEST-2", ledger.OrderPayload{},
		appendEvent, fillFn, 1.0, "test", nil,
	); err != nil {
		t.Fatalf("unexpected error with no assertion: %v", err)
	}
	if !brokerCalled {
		t.Fatal("broker fill must be reached when there is no failing assertion")
	}
}
