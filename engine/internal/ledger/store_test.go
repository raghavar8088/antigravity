package ledger

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreAppendsEventsWithAggregateSequence(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	created := mustEvent(t, EventOrderCreated, OrderPayload{ClientOrderID: "cl-1", Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1})
	submitted := mustEvent(t, EventOrderSubmitted, OrderPayload{ClientOrderID: "cl-1"})

	first, err := store.Append(ctx, created)
	if err != nil {
		t.Fatalf("append created: %v", err)
	}
	second, err := store.Append(ctx, submitted)
	if err != nil {
		t.Fatalf("append submitted: %v", err)
	}
	if first.SequenceNo != 1 || second.SequenceNo != 2 {
		t.Fatalf("expected aggregate sequence 1,2 got %d,%d", first.SequenceNo, second.SequenceNo)
	}

	events, err := store.Replay(ctx, AggregateOrder, "cl-1")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 replayed events, got %d", len(events))
	}
}

func TestMemoryStoreRejectsDuplicateIdempotencyKey(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	first := mustEvent(t, EventOrderCreated, OrderPayload{ClientOrderID: "cl-1"})
	first.IdempotencyKey = "same-signal"
	second := mustEvent(t, EventOrderCreated, OrderPayload{ClientOrderID: "cl-1"})
	second.IdempotencyKey = "same-signal"

	if _, err := store.Append(ctx, first); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if _, err := store.Append(ctx, second); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestProjectOrderRebuildsOrderStateFromLedger(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	events := []Event{
		mustEvent(t, EventOrderCreated, OrderPayload{ClientOrderID: "cl-1", Symbol: "BTCUSDT", Side: "BUY", Quantity: 1}),
		mustEvent(t, EventOrderSubmitted, OrderPayload{ClientOrderID: "cl-1"}),
		mustEvent(t, EventOrderAcked, OrderPayload{ClientOrderID: "cl-1", ExchangeOrderID: "ex-1"}),
		mustEvent(t, EventOrderPartial, OrderPayload{ClientOrderID: "cl-1", FillQuantity: 0.4, FillPrice: 100, FeeUSD: 0.02}),
		mustEvent(t, EventOrderFilled, OrderPayload{ClientOrderID: "cl-1", FillQuantity: 0.6, FillPrice: 110, FeeUSD: 0.03}),
	}
	for _, event := range events {
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("append %s: %v", event.EventType, err)
		}
	}
	replayed, err := store.Replay(ctx, AggregateOrder, "cl-1")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	projection, err := ProjectOrder(replayed)
	if err != nil {
		t.Fatalf("project: %v", err)
	}

	if projection.State != OrderStateFilled {
		t.Fatalf("expected filled state, got %s", projection.State)
	}
	if projection.ExchangeOrderID != "ex-1" {
		t.Fatalf("expected exchange id ex-1, got %q", projection.ExchangeOrderID)
	}
	if projection.FilledQuantity != 1 {
		t.Fatalf("expected filled qty 1, got %.2f", projection.FilledQuantity)
	}
	if projection.AveragePrice != 106 {
		t.Fatalf("expected avg price 106, got %.2f", projection.AveragePrice)
	}
	if projection.FeesUSD != 0.05 {
		t.Fatalf("expected fees 0.05, got %.2f", projection.FeesUSD)
	}
}

func mustEvent(t *testing.T, eventType EventType, payload OrderPayload) Event {
	t.Helper()
	event, err := NewEvent(NewEventInput{
		AggregateType: AggregateOrder,
		AggregateID:   "cl-1",
		EventType:     eventType,
		AccountID:     "acct-1",
		Symbol:        "BTCUSDT",
		Payload:       payload,
		Source:        "ledger-test",
	})
	if err != nil {
		t.Fatalf("new event: %v", err)
	}
	return event
}
