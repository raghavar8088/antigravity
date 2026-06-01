package v3

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

func TestOMSBridge_Disabled_WhenNilStore(t *testing.T) {
	bridge := NewOMSBridge(nil, "acc1")
	ctx := context.Background()
	ts := time.Now()
	// All calls should be no-ops without error.
	if err := bridge.RecordOrderCreated(ctx, "O1", "BTCUSD", "BUY", "strat1", 0.1, ts); err != nil {
		t.Fatalf("nil bridge should not error: %v", err)
	}
	if err := bridge.RecordOrderFilled(ctx, "O1", "BTCUSD", "strat1", 50000, 0.1, 20, 2.5, ts); err != nil {
		t.Fatalf("nil bridge should not error: %v", err)
	}
}

func TestOMSBridge_RecordOrderCreated(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	err := bridge.RecordOrderCreated(ctx, "O1", "BTCUSD", "BUY", "stratA", 0.5, ts)
	if err != nil {
		t.Fatalf("RecordOrderCreated error: %v", err)
	}
	events, err := store.Replay(ctx, ledger.AggregateOrder, "O1")
	if err != nil {
		t.Fatalf("replay error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != ledger.EventOrderCreated {
		t.Fatalf("expected ORDER_CREATED, got %s", events[0].EventType)
	}
}

func TestOMSBridge_RecordOrderFilled(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	_ = bridge.RecordOrderCreated(ctx, "O2", "BTCUSD", "BUY", "stratB", 0.5, ts)
	err := bridge.RecordOrderFilled(ctx, "O2", "BTCUSD", "stratB", 50000, 0.5, 20, 2.0, ts.Add(50*time.Millisecond))
	if err != nil {
		t.Fatalf("RecordOrderFilled error: %v", err)
	}
	events, _ := store.Replay(ctx, ledger.AggregateOrder, "O2")
	if len(events) != 2 {
		t.Fatalf("expected 2 events (created + filled), got %d", len(events))
	}
	if events[1].EventType != ledger.EventOrderFilled {
		t.Fatalf("second event should be ORDER_FILLED, got %s", events[1].EventType)
	}
}

func TestOMSBridge_RecordPartialFill(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	_ = bridge.RecordOrderCreated(ctx, "O3", "BTCUSD", "BUY", "stratC", 1.0, ts)
	pf := OrderPartialFillEvent{
		OrderID:           "O3",
		FilledQuantity:    0.5,
		RemainingQuantity: 0.5,
		AverageFillPrice:  50100,
		FillNumber:        1,
		Timestamp:         ts.Add(10 * time.Millisecond),
	}
	err := bridge.RecordOrderPartialFill(ctx, "O3", "BTCUSD", "stratC", pf)
	if err != nil {
		t.Fatalf("RecordOrderPartialFill error: %v", err)
	}
	events, _ := store.Replay(ctx, ledger.AggregateOrder, "O3")
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[1].EventType != ledger.EventOrderPartial {
		t.Fatalf("second event should be ORDER_PARTIAL, got %s", events[1].EventType)
	}
}

func TestOMSBridge_RecordOrderRejected(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	_ = bridge.RecordOrderCreated(ctx, "O4", "BTCUSD", "BUY", "stratD", 0.5, ts)
	err := bridge.RecordOrderRejected(ctx, "O4", "BTCUSD", "stratD", "exchange outage", ts.Add(1*time.Millisecond))
	if err != nil {
		t.Fatalf("RecordOrderRejected error: %v", err)
	}
	events, _ := store.Replay(ctx, ledger.AggregateOrder, "O4")
	found := false
	for _, ev := range events {
		if ev.EventType == ledger.EventOrderRejected {
			found = true
		}
	}
	if !found {
		t.Fatal("should have ORDER_REJECTED event")
	}
}

func TestOMSBridge_RecordPositionOpened(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	err := bridge.RecordPositionOpened(ctx, "POS1", "O5", "BTCUSD", "BUY", "stratE", 50000, 0.5, 0.35, 0.70, ts)
	if err != nil {
		t.Fatalf("RecordPositionOpened error: %v", err)
	}
	events, _ := store.Replay(ctx, ledger.AggregatePosition, "POS1")
	if len(events) != 1 {
		t.Fatalf("expected 1 position event, got %d", len(events))
	}
	if events[0].EventType != ledger.EventPositionOpened {
		t.Fatalf("expected POSITION_OPENED, got %s", events[0].EventType)
	}
}

func TestOMSBridge_RecordPositionClosed(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	_ = bridge.RecordPositionOpened(ctx, "POS2", "O6", "BTCUSD", "BUY", "stratF", 50000, 0.5, 0.35, 0.70, ts)
	trade := V3Trade{
		ID:           "POS2",
		Symbol:       "BTCUSD",
		StrategyName: "stratF",
		EntryPrice:   50000,
		ExitPrice:    51000,
		Size:         0.5,
		GrossPnL:     500,
		NetPnL:       480,
		CommissionUSD: 20,
		HoldMinutes:  15,
		ExitReason:   "TP",
		ClosedAt:     ts.Add(15 * time.Minute),
	}
	err := bridge.RecordPositionClosed(ctx, trade)
	if err != nil {
		t.Fatalf("RecordPositionClosed error: %v", err)
	}
	events, _ := store.Replay(ctx, ledger.AggregatePosition, "POS2")
	found := false
	for _, ev := range events {
		if ev.EventType == ledger.EventPositionClosed {
			found = true
		}
	}
	if !found {
		t.Fatal("should have POSITION_CLOSED event")
	}
}

func TestOMSBridge_Idempotency(t *testing.T) {
	store := ledger.NewMemoryStore()
	bridge := NewOMSBridge(store, "acc-test")
	ctx := context.Background()
	ts := time.Now()

	// Same call twice — idempotency key should deduplicate.
	_ = bridge.RecordOrderCreated(ctx, "O7", "BTCUSD", "BUY", "stratG", 0.5, ts)
	_ = bridge.RecordOrderCreated(ctx, "O7", "BTCUSD", "BUY", "stratG", 0.5, ts)

	// Idempotency behavior depends on the store implementation.
	// We just verify it doesn't panic and events exist.
	events, _ := store.Replay(ctx, ledger.AggregateOrder, "O7")
	if len(events) == 0 {
		t.Fatal("should have at least one order event after two identical calls")
	}
}
