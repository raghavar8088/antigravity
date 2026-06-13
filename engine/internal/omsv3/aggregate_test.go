package omsv3

import (
	"errors"
	"testing"

	"antigravity-engine/internal/ledger"
)

func TestOrderAggregateReplaysValidStateMachine(t *testing.T) {
	events := []ledger.Event{
		mustOrderEvent(t, ledger.EventOrderCreated, ledger.OrderPayload{ClientOrderID: "cl-1", Symbol: "BTCUSDT", Side: "BUY", Quantity: 1}),
		mustOrderEvent(t, ledger.EventOrderValidated, ledger.OrderPayload{ClientOrderID: "cl-1"}),
		mustOrderEvent(t, ledger.EventRiskApproved, ledger.OrderPayload{ClientOrderID: "cl-1"}),
		mustOrderEvent(t, ledger.EventOrderSubmitted, ledger.OrderPayload{ClientOrderID: "cl-1"}),
		mustOrderEvent(t, ledger.EventOrderAcked, ledger.OrderPayload{ClientOrderID: "cl-1", ExchangeOrderID: "ex-1"}),
		mustOrderEvent(t, ledger.EventOrderPartial, ledger.OrderPayload{ClientOrderID: "cl-1", FillQuantity: 0.25, FillPrice: 100}),
		mustOrderEvent(t, ledger.EventOrderFilled, ledger.OrderPayload{ClientOrderID: "cl-1", FillQuantity: 0.75, FillPrice: 110}),
	}
	for i := range events {
		events[i].SequenceNo = int64(i + 1)
	}

	aggregate, err := Replay(events)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if aggregate.CurrentState() != StateFilled {
		t.Fatalf("expected FILLED, got %s", aggregate.CurrentState())
	}
	if aggregate.ExchangeOrderID != "ex-1" {
		t.Fatalf("expected exchange id ex-1, got %q", aggregate.ExchangeOrderID)
	}
	if aggregate.FilledQuantity != 1 {
		t.Fatalf("expected full quantity, got %.2f", aggregate.FilledQuantity)
	}
}

func TestOrderAggregateRejectsFilledToNew(t *testing.T) {
	aggregate := NewOrderAggregate("cl-1")
	for _, eventType := range []ledger.EventType{
		ledger.EventOrderCreated,
		ledger.EventOrderValidated,
		ledger.EventRiskApproved,
		ledger.EventOrderSubmitted,
		ledger.EventOrderAcked,
		ledger.EventOrderFilled,
	} {
		if err := aggregate.ApplyEvent(mustOrderEvent(t, eventType, ledger.OrderPayload{ClientOrderID: "cl-1"})); err != nil {
			t.Fatalf("apply %s: %v", eventType, err)
		}
	}

	err := aggregate.ApplyEvent(mustOrderEvent(t, ledger.EventOrderCreated, ledger.OrderPayload{ClientOrderID: "cl-1"}))
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestOrderAggregateRejectsRejectedToPartiallyFilled(t *testing.T) {
	aggregate := NewOrderAggregate("cl-1")
	if err := aggregate.ApplyEvent(mustOrderEvent(t, ledger.EventOrderCreated, ledger.OrderPayload{ClientOrderID: "cl-1"})); err != nil {
		t.Fatalf("apply create: %v", err)
	}
	if err := aggregate.ApplyEvent(mustOrderEvent(t, ledger.EventOrderRejected, ledger.OrderPayload{ClientOrderID: "cl-1"})); err != nil {
		t.Fatalf("apply reject: %v", err)
	}

	err := aggregate.ApplyEvent(mustOrderEvent(t, ledger.EventOrderPartial, ledger.OrderPayload{ClientOrderID: "cl-1", FillQuantity: 0.1, FillPrice: 100}))
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

// TestOrderAcceptedTransition verifies SUBMITTED → ACCEPTED via ORDER_ACCEPTED event.
func TestOrderAcceptedTransition(t *testing.T) {
	events := []ledger.Event{
		mustOrderEvent(t, ledger.EventOrderCreated, ledger.OrderPayload{ClientOrderID: "cl-acc-1", Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1}),
		mustOrderEvent(t, ledger.EventOrderValidated, ledger.OrderPayload{ClientOrderID: "cl-acc-1"}),
		mustOrderEvent(t, ledger.EventRiskApproved, ledger.OrderPayload{ClientOrderID: "cl-acc-1"}),
		mustOrderEvent(t, ledger.EventOrderSubmitted, ledger.OrderPayload{ClientOrderID: "cl-acc-1"}),
		mustOrderEvent(t, ledger.EventOrderAccepted, ledger.OrderPayload{ClientOrderID: "cl-acc-1"}),
	}
	for i := range events {
		events[i].SequenceNo = int64(i + 1)
	}
	agg, err := Replay(events)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if agg.CurrentState() != StateAccepted {
		t.Fatalf("expected ACCEPTED, got %s", agg.CurrentState())
	}
}

// TestOrderAcceptedToFilled verifies ACCEPTED → FILLED is valid.
func TestOrderAcceptedToFilled(t *testing.T) {
	events := []ledger.Event{
		mustOrderEvent(t, ledger.EventOrderCreated, ledger.OrderPayload{ClientOrderID: "cl-acc-2", Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1}),
		mustOrderEvent(t, ledger.EventOrderValidated, ledger.OrderPayload{ClientOrderID: "cl-acc-2"}),
		mustOrderEvent(t, ledger.EventRiskApproved, ledger.OrderPayload{ClientOrderID: "cl-acc-2"}),
		mustOrderEvent(t, ledger.EventOrderSubmitted, ledger.OrderPayload{ClientOrderID: "cl-acc-2"}),
		mustOrderEvent(t, ledger.EventOrderAccepted, ledger.OrderPayload{ClientOrderID: "cl-acc-2"}),
		mustOrderEvent(t, ledger.EventOrderFilled, ledger.OrderPayload{ClientOrderID: "cl-acc-2", FillPrice: 62000, FillQuantity: 0.1}),
	}
	for i := range events {
		events[i].SequenceNo = int64(i + 1)
	}
	agg, err := Replay(events)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if agg.CurrentState() != StateFilled {
		t.Fatalf("expected FILLED, got %s", agg.CurrentState())
	}
}

// TestDirectValidatedToFilledRejected verifies that VALIDATED → FILLED (direct jump) is rejected.
func TestDirectValidatedToFilledRejected(t *testing.T) {
	events := []ledger.Event{
		mustOrderEvent(t, ledger.EventOrderCreated, ledger.OrderPayload{ClientOrderID: "cl-acc-3", Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1}),
		mustOrderEvent(t, ledger.EventOrderValidated, ledger.OrderPayload{ClientOrderID: "cl-acc-3"}),
		mustOrderEvent(t, ledger.EventOrderFilled, ledger.OrderPayload{ClientOrderID: "cl-acc-3", FillPrice: 62000, FillQuantity: 0.1}),
	}
	for i := range events {
		events[i].SequenceNo = int64(i + 1)
	}
	_, err := Replay(events)
	if err == nil {
		t.Fatal("expected error for direct VALIDATED → FILLED jump, got nil")
	}
}

func mustOrderEvent(t *testing.T, eventType ledger.EventType, payload ledger.OrderPayload) ledger.Event {
	t.Helper()
	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateOrder,
		AggregateID:   "cl-1",
		EventType:     eventType,
		AccountID:     "acct-1",
		Symbol:        "BTCUSDT",
		Payload:       payload,
		Source:        "omsv3-test",
	})
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	return event
}
