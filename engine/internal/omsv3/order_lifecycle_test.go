package omsv3

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func orderEvent(t *testing.T, et ledger.EventType, orderID string) ledger.Event {
	t.Helper()
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   orderID,
		AccountID:     "test-account",
		EventType:     et,
		Payload:       ledger.OrderPayload{ClientOrderID: orderID, Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1},
		Source:        "order-lifecycle-test",
	})
	if err != nil {
		t.Fatalf("NewEvent(%s): %v", et, err)
	}
	return ev
}

// ── Test Suite: Full Happy Path ───────────────────────────────────────────────

// TestOrderLifecycleHappyPath verifies the canonical order lifecycle:
// CREATED → VALIDATED → RISK_APPROVED → SUBMITTED → ACKED → PARTIALLY_FILLED → FILLED
func TestOrderLifecycleHappyPath(t *testing.T) {
	agg := NewOrderAggregate("happy-001")

	steps := []ledger.EventType{
		ledger.EventOrderCreated,
		ledger.EventOrderValidated,
		ledger.EventRiskApproved,
		ledger.EventOrderSubmitted,
		ledger.EventOrderAcked,
		ledger.EventOrderPartial,
		ledger.EventOrderFilled,
	}
	expectedStates := []OrderState{
		StateNew, StateValidated, StateRiskApproved,
		StateSubmitted, StateAcknowledged, StatePartiallyFilled, StateFilled,
	}

	for i, et := range steps {
		ev := orderEvent(t, et, "happy-001")
		if err := agg.ApplyEvent(ev); err != nil {
			t.Fatalf("step %d (%s): %v", i, et, err)
		}
		if agg.State != expectedStates[i] {
			t.Errorf("step %d: want state %s got %s", i, expectedStates[i], agg.State)
		}
	}

	if agg.Version != 7 {
		t.Errorf("Version: want 7 got %d", agg.Version)
	}
	if len(agg.Events) != 7 {
		t.Errorf("Events: want 7 got %d", len(agg.Events))
	}
}

// ── Test Suite: Cancellation Paths ───────────────────────────────────────────

// TestOrderCancelledFromNew verifies an order can be cancelled from NEW state.
func TestOrderCancelledFromNew(t *testing.T) {
	agg := NewOrderAggregate("cancel-new-001")
	for _, et := range []ledger.EventType{ledger.EventOrderCreated, ledger.EventOrderCancelled} {
		if err := agg.ApplyEvent(orderEvent(t, et, "cancel-new-001")); err != nil {
			t.Fatalf("%s: %v", et, err)
		}
	}
	if agg.State != StateCancelled {
		t.Errorf("want CANCELLED got %s", agg.State)
	}
}

// TestOrderCancelledFromPartiallyFilled verifies cancellation mid-fill.
func TestOrderCancelledFromPartiallyFilled(t *testing.T) {
	agg := NewOrderAggregate("cancel-partial-001")
	path := []ledger.EventType{
		ledger.EventOrderCreated, ledger.EventOrderValidated, ledger.EventRiskApproved,
		ledger.EventOrderSubmitted, ledger.EventOrderAcked, ledger.EventOrderPartial,
		ledger.EventOrderCancelled,
	}
	for _, et := range path {
		if err := agg.ApplyEvent(orderEvent(t, et, "cancel-partial-001")); err != nil {
			t.Fatalf("%s: %v", et, err)
		}
	}
	if agg.State != StateCancelled {
		t.Errorf("want CANCELLED got %s", agg.State)
	}
}

// ── Test Suite: Rejection Paths ───────────────────────────────────────────────

// TestOrderRejectedByRisk verifies RISK_BLOCKED moves order to REJECTED.
func TestOrderRejectedByRisk(t *testing.T) {
	agg := NewOrderAggregate("risk-reject-001")
	for _, et := range []ledger.EventType{ledger.EventOrderCreated, ledger.EventOrderValidated, ledger.EventRiskBlocked} {
		if err := agg.ApplyEvent(orderEvent(t, et, "risk-reject-001")); err != nil {
			t.Fatalf("%s: %v", et, err)
		}
	}
	if agg.State != StateRejected {
		t.Errorf("want REJECTED got %s", agg.State)
	}
}

// ── Test Suite: Illegal Transitions ──────────────────────────────────────────

// buildToState walks an OrderAggregate along the canonical happy path until it
// reaches the desired state. Returns the aggregate with the accumulated events.
func buildToState(t *testing.T, target OrderState, id string) *OrderAggregate {
	t.Helper()
	agg := NewOrderAggregate(id)
	// Full canonical path up to FILLED.
	fullPath := []ledger.EventType{
		ledger.EventOrderCreated,
		ledger.EventOrderValidated,
		ledger.EventRiskApproved,
		ledger.EventOrderSubmitted,
		ledger.EventOrderAcked,
		ledger.EventOrderFilled,
	}
	for _, et := range fullPath {
		if err := agg.ApplyEvent(orderEvent(t, et, id)); err != nil {
			t.Fatalf("buildToState(%s) at %s: %v", target, et, err)
		}
		if agg.State == target {
			return agg
		}
	}
	// Check if we need the cancellation branch.
	if target == StateCancelled || target == StateRejected {
		agg = NewOrderAggregate(id + "-branch")
		branch := []ledger.EventType{ledger.EventOrderCreated}
		if target == StateRejected {
			branch = append(branch, ledger.EventOrderRejected)
		} else {
			branch = append(branch, ledger.EventOrderCancelled)
		}
		for _, et := range branch {
			if err := agg.ApplyEvent(orderEvent(t, et, id+"-branch")); err != nil {
				t.Fatalf("buildToState(branch %s) at %s: %v", target, et, err)
			}
		}
		return agg
	}
	t.Fatalf("buildToState: could not reach state %s", target)
	return nil
}

// TestOrderIllegalTransitions verifies that invalid state machine transitions
// are rejected with ErrInvalidTransition and do NOT mutate aggregate state.
func TestOrderIllegalTransitions(t *testing.T) {
	cases := []struct {
		name     string
		target   OrderState
		attempts []ledger.EventType
	}{
		{
			name:     "FILLED cannot transition",
			target:   StateFilled,
			attempts: []ledger.EventType{ledger.EventOrderCancelled, ledger.EventOrderPartial, ledger.EventOrderValidated},
		},
		{
			name:     "CANCELLED is terminal",
			target:   StateCancelled,
			attempts: []ledger.EventType{ledger.EventOrderFilled, ledger.EventOrderValidated},
		},
		{
			name:     "REJECTED is terminal",
			target:   StateRejected,
			attempts: []ledger.EventType{ledger.EventOrderFilled, ledger.EventOrderCancelled},
		},
		{
			name:     "NEW cannot skip to SUBMITTED",
			target:   StateNew,
			attempts: []ledger.EventType{ledger.EventOrderSubmitted, ledger.EventOrderFilled},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agg := buildToState(t, tc.target, "illegal-"+tc.name)
			stateBeforeAttempt := agg.State

			for _, bad := range tc.attempts {
				err := agg.ApplyEvent(orderEvent(t, bad, agg.ID))
				if err == nil {
					t.Errorf("expected ErrInvalidTransition for state %s → event %s, got nil",
						stateBeforeAttempt, bad)
				}
				if agg.State != stateBeforeAttempt {
					t.Errorf("state was mutated from %s after rejected event %s (now %s)",
						stateBeforeAttempt, bad, agg.State)
					// Reset for next attempt
					agg.State = stateBeforeAttempt
				}
			}
		})
	}
}

// stateFromEventType maps event types to the order state they would produce.
func stateFromEventType(et ledger.EventType) OrderState {
	switch et {
	case ledger.EventOrderCreated:
		return StateNew
	case ledger.EventOrderValidated:
		return StateValidated
	case ledger.EventRiskApproved:
		return StateRiskApproved
	case ledger.EventOrderSubmitted:
		return StateSubmitted
	case ledger.EventOrderAcked:
		return StateAcknowledged
	case ledger.EventOrderPartial:
		return StatePartiallyFilled
	case ledger.EventOrderFilled:
		return StateFilled
	case ledger.EventOrderCancelled:
		return StateCancelled
	case ledger.EventOrderRejected, ledger.EventRiskBlocked:
		return StateRejected
	default:
		return StateEmpty
	}
}

// ── Test Suite: Replay ────────────────────────────────────────────────────────

// TestOrderReplayFromStore verifies that an order aggregate rebuilt via Replay()
// is identical to one built event-by-event.
func TestOrderReplayFromStore(t *testing.T) {
	store := ledger.NewMemoryStore()
	ctx := context.Background()
	orderID := "replay-001"
	accountID := "test-account"

	path := []ledger.EventType{
		ledger.EventOrderCreated, ledger.EventOrderValidated, ledger.EventRiskApproved,
		ledger.EventOrderSubmitted, ledger.EventOrderAcked, ledger.EventOrderFilled,
	}
	for _, et := range path {
		ev, err := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateOrder,
			AggregateID:   orderID,
			AccountID:     accountID,
			EventType:     et,
			Payload:       ledger.OrderPayload{ClientOrderID: orderID, Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1},
			Source:        "replay-test",
		})
		if err != nil {
			t.Fatalf("NewEvent(%s): %v", et, err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append(%s): %v", et, err)
		}
	}

	events, err := store.Replay(ctx, ledger.AggregateOrder, orderID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	agg, err := Replay(events)
	if err != nil {
		t.Fatalf("Replay aggregate: %v", err)
	}

	if agg.State != StateFilled {
		t.Errorf("want FILLED got %s", agg.State)
	}
	if agg.Version != int64(len(path)) {
		t.Errorf("Version: want %d got %d", len(path), agg.Version)
	}
}

// ── Test Suite: CommandBus ─────────────────────────────────────────────────────

// TestCommandBusDispatchesCreateOrder verifies the CommandBus correctly creates
// an ORDER_CREATED event, validates ownership, and appends to the ledger.
func TestCommandBusDispatchesCreateOrder(t *testing.T) {
	store := ledger.NewMemoryStore()
	bus := NewCommandBus(store, "test-account")
	ctx := context.Background()

	cmd := &CreateOrderCommand{
		ClientOrderID: "bus-order-001",
		Symbol:        "BTCUSDT",
		Side:          "BUY",
		Quantity:      0.1,
		NotionalUSD:   5000,
		Leverage:      10,
		OrderType:     "MARKET",
		StrategyName:  "TestStrategy",
		StopLossPct:   0.5,
		TakeProfitPct: 1.0,
	}
	result, err := bus.Dispatch(ctx, cmd)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if result.Event.EventType != ledger.EventOrderCreated {
		t.Errorf("want EventOrderCreated got %s", result.Event.EventType)
	}
	if result.Event.AggregateID != "bus-order-001" {
		t.Errorf("AggregateID: want bus-order-001 got %s", result.Event.AggregateID)
	}
	if result.Event.AccountID != "test-account" {
		t.Errorf("AccountID: want test-account got %s", result.Event.AccountID)
	}

	// Verify event is in the ledger.
	events, err := store.Replay(ctx, ledger.AggregateOrder, "bus-order-001")
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event in ledger, got %d", len(events))
	}
}

// TestCommandBusRejectsInvalidCommand verifies that a command with invalid
// parameters is rejected before any ledger write.
func TestCommandBusRejectsInvalidCommand(t *testing.T) {
	store := ledger.NewMemoryStore()
	bus := NewCommandBus(store, "test-account")
	ctx := context.Background()

	// Missing ClientOrderID
	cmd := &CreateOrderCommand{Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1}
	if _, err := bus.Dispatch(ctx, cmd); err == nil {
		t.Error("expected validation error for missing ClientOrderID, got nil")
	}

	// Missing Symbol
	cmd2 := &CreateOrderCommand{ClientOrderID: "bad-002", Side: "BUY", Quantity: 0.1}
	if _, err := bus.Dispatch(ctx, cmd2); err == nil {
		t.Error("expected validation error for missing Symbol, got nil")
	}

	// No events should be in the ledger.
	events, _ := store.ReplayAccount(ctx, "test-account")
	if len(events) != 0 {
		t.Errorf("expected 0 events after rejected commands, got %d", len(events))
	}
}

// TestAggregateOwnershipViolationRejected verifies ValidateEventOwnership rejects
// events emitted under the wrong aggregate type.
func TestAggregateOwnershipViolationRejected(t *testing.T) {
	// POSITION_OPENED under RISK aggregate — ownership violation
	wrongEvent := ledger.Event{
		AggregateType: ledger.AggregateRisk,    // wrong!
		AggregateID:   "pos-001",
		EventType:     ledger.EventPositionOpened, // belongs to POSITION
	}
	if err := ValidateEventOwnership(wrongEvent); err == nil {
		t.Error("expected ErrAggregateOwnershipViolation, got nil")
	}

	// Correct ownership
	rightEvent := ledger.Event{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   "pos-001",
		EventType:     ledger.EventPositionOpened,
	}
	if err := ValidateEventOwnership(rightEvent); err != nil {
		t.Errorf("expected no error for correct ownership, got: %v", err)
	}
}
