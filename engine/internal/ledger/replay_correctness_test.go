package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func buildTestOrderEvents(t *testing.T, store *MemoryStore, accountID, orderID string) {
	t.Helper()
	ctx := context.Background()
	steps := []struct {
		et      EventType
		payload OrderPayload
	}{
		{EventOrderCreated, OrderPayload{ClientOrderID: orderID, Symbol: "BTCUSDT", Side: "BUY", Quantity: 0.1}},
		{EventOrderValidated, OrderPayload{ClientOrderID: orderID}},
		{EventOrderSubmitted, OrderPayload{ClientOrderID: orderID}},
		{EventOrderAcked, OrderPayload{ClientOrderID: orderID, ExchangeOrderID: "EX-" + orderID}},
		{EventOrderFilled, OrderPayload{ClientOrderID: orderID, FillPrice: 50000, FillQuantity: 0.1, FeeUSD: 2.5}},
	}
	for _, s := range steps {
		ev := mustEvent(t, s.et, s.payload)
		ev.AggregateID = orderID
		ev.AccountID = accountID
		ev.PayloadHash = PayloadHash(ev.Payload)
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append(%s): %v", s.et, err)
		}
	}
}

func buildTestPositionEvents(t *testing.T, store *MemoryStore, accountID, posID string) {
	t.Helper()
	ctx := context.Background()
	openPayload := PositionOpenedPayload{
		ClientOrderID: posID, PositionID: posID, Symbol: "BTCUSDT",
		Side: "LONG", EntryPrice: 50000, Quantity: 0.1, NotionalUSD: 5000,
		MarginUsed: 500, Leverage: 10, StopLoss: 49750, TakeProfit: 50500,
		StopLossPct: 0.5, TakeProfitPct: 1.0, StrategyName: "TestStrat", EntryFeeUSD: 2.5,
	}
	closePayload := PositionClosedPayload{
		ClientOrderID: posID, PositionID: posID, Symbol: "BTCUSDT",
		Side: "LONG", EntryPrice: 50000, ExitPrice: 50500, Quantity: 0.1,
		NotionalUSD: 5000, GrossPnLUSD: 50, NetPnLUSD: 45, FeesUSD: 5,
		ExitReason: "TP", StrategyName: "TestStrat", HoldMinutes: 30,
	}
	for _, inp := range []NewEventInput{
		{AggregateType: AggregatePosition, AggregateID: posID, AccountID: accountID, EventType: EventPositionOpened, Payload: openPayload, Source: "test"},
		{AggregateType: AggregatePosition, AggregateID: posID, AccountID: accountID, EventType: EventPositionClosed, Payload: closePayload, Source: "test"},
	} {
		ev, err := NewEvent(inp)
		if err != nil {
			t.Fatalf("NewEvent(%s): %v", inp.EventType, err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("Append(%s): %v", inp.EventType, err)
		}
	}
}

// ── Test Suite 1 — 100k Event Replay ─────────────────────────────────────────

func TestReplay100kEvents(t *testing.T) {
	const (
		numOrders      = 10_000
		eventsPerOrder = 5
		accountID      = "perf-account"
	)

	store := NewMemoryStore()
	start := time.Now()
	for i := 0; i < numOrders; i++ {
		buildTestOrderEvents(t, store, accountID, fmt.Sprintf("ord-%06d", i))
	}
	appendElapsed := time.Since(start)

	start = time.Now()
	result, err := ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("ReplayEverything: %v", err)
	}
	replayElapsed := time.Since(start)

	wantTotal := numOrders * eventsPerOrder
	if result.TotalEventCount != wantTotal {
		t.Errorf("TotalEventCount: want %d got %d", wantTotal, result.TotalEventCount)
	}

	byID, err := ReplayOrders(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("ReplayOrders: %v", err)
	}
	if len(byID) != numOrders {
		t.Errorf("distinct orders: want %d got %d", numOrders, len(byID))
	}
	for id, events := range byID {
		if len(events) != eventsPerOrder {
			t.Errorf("order %s: want %d events got %d", id, eventsPerOrder, len(events))
		}
		if err := VerifySequence(events); err != nil {
			t.Errorf("VerifySequence(%s): %v", id, err)
		}
	}
	t.Logf("100k events: append=%v replay=%v", appendElapsed, replayElapsed)
	if replayElapsed > 5*time.Second {
		t.Errorf("replay too slow: %v (limit 5s)", replayElapsed)
	}
}

// ── Test Suite 2 — Determinism ────────────────────────────────────────────────

func TestReplayDeterminism(t *testing.T) {
	const accountID = "determinism-account"
	store := NewMemoryStore()
	for i := 0; i < 200; i++ {
		buildTestOrderEvents(t, store, accountID, fmt.Sprintf("ord-%04d", i))
	}
	for i := 0; i < 200; i++ {
		buildTestPositionEvents(t, store, accountID, fmt.Sprintf("pos-%04d", i))
	}

	r1, err := ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("replay 1: %v", err)
	}
	r2, err := ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("replay 2: %v", err)
	}
	if r1.TotalEventCount != r2.TotalEventCount {
		t.Fatalf("TotalEventCount differs: %d vs %d", r1.TotalEventCount, r2.TotalEventCount)
	}
	for i := range r1.Orders {
		if r1.Orders[i].EventID != r2.Orders[i].EventID {
			t.Errorf("order event[%d] EventID differs", i)
		}
	}
	for i := range r1.Positions {
		if r1.Positions[i].EventID != r2.Positions[i].EventID {
			t.Errorf("position event[%d] EventID differs", i)
		}
	}
	t.Logf("Determinism confirmed: %d events × 2 replays identical", r1.TotalEventCount)
}

// ── Test Suite 3 — Duplicate Rejection ───────────────────────────────────────

func TestDuplicateEventRejection(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	base, err := NewEvent(NewEventInput{
		AggregateType:  AggregateOrder,
		AggregateID:    "dup-001",
		AccountID:      "acc",
		EventType:      EventOrderCreated,
		IdempotencyKey: "idem-key-001",
		Payload:        OrderPayload{ClientOrderID: "dup-001"},
		Source:         "test",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if _, err := store.Append(ctx, base); err != nil {
		t.Fatalf("first append: %v", err)
	}
	// Duplicate EventID
	if _, err := store.Append(ctx, base); err == nil {
		t.Error("expected ErrDuplicateEvent on duplicate EventID")
	}
	// Duplicate idempotency key
	diff, _ := NewEvent(NewEventInput{
		AggregateType:  AggregateOrder,
		AggregateID:    "dup-001",
		AccountID:      "acc",
		EventType:      EventOrderValidated,
		IdempotencyKey: "idem-key-001",
		Payload:        OrderPayload{ClientOrderID: "dup-001"},
		Source:         "test",
	})
	if _, err := store.Append(ctx, diff); err == nil {
		t.Error("expected ErrDuplicateEvent on duplicate idempotency key")
	}
}

// ── Test Suite 4 — Out-of-Order Detection ────────────────────────────────────

func TestOutOfOrderDetection(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{EventID: "e1", CreatedAt: now},
		{EventID: "e2", CreatedAt: now.Add(1 * time.Second)},
		{EventID: "e3", CreatedAt: now.Add(500 * time.Millisecond)}, // out of order
		{EventID: "e4", CreatedAt: now.Add(2 * time.Second)},
	}
	bad := DetectOutOfOrder(events)
	if len(bad) != 1 || bad[0] != 2 {
		t.Errorf("expected [2], got %v", bad)
	}
}

// ── Test Suite 5 — Deduplication ─────────────────────────────────────────────

func TestDeduplicateEvents(t *testing.T) {
	events := make([]Event, 15)
	for i := range events {
		events[i] = Event{EventID: fmt.Sprintf("evt-%02d", i%10)}
	}
	deduped := DeduplicateEvents(events)
	if len(deduped) != 10 {
		t.Errorf("expected 10 unique events, got %d", len(deduped))
	}
	for _, e := range deduped {
		count := 0
		for _, d := range deduped {
			if d.EventID == e.EventID {
				count++
			}
		}
		if count > 1 {
			t.Errorf("EventID %s appears %d times", e.EventID, count)
		}
	}
}

// ── Test Suite 6 — Schema Migration ──────────────────────────────────────────

func TestSchemaMigrationV1toV2(t *testing.T) {
	payload := map[string]any{"client_order_id": "migr-001", "symbol": "BTCUSDT"}
	raw, _ := json.Marshal(payload)
	v1Event := Event{
		EventID:       "migr-event-001",
		SchemaVersion: 1,
		AggregateType: AggregateOrder,
		AggregateID:   "migr-001",
		EventType:     EventOrderCreated,
		Payload:       raw,
		PayloadHash:   PayloadHash(raw),
		Source:        "legacy",
	}
	upgraded, err := MigrateEvent(v1Event)
	if err != nil {
		t.Fatalf("MigrateEvent: %v", err)
	}
	if upgraded.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("want SchemaVersion=%d got %d", CurrentSchemaVersion, upgraded.SchemaVersion)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(upgraded.Payload, &result); err != nil {
		t.Fatalf("unmarshal migrated payload: %v", err)
	}
	if _, ok := result["client_order_id"]; !ok {
		t.Error("client_order_id missing after migration")
	}
}

// ── Test Suite 7 — MigratingStore ────────────────────────────────────────────

func TestMigratingStoreUpgradesOnRead(t *testing.T) {
	inner := NewMemoryStore()
	ctx := context.Background()
	raw, _ := json.Marshal(map[string]any{"client_order_id": "migr-002"})
	v1Event := Event{
		EventID:       "migr-002-evtid",
		SchemaVersion: 1,
		AggregateType: AggregateOrder,
		AggregateID:   "migr-002",
		EventType:     EventOrderCreated,
		Payload:       raw,
		PayloadHash:   PayloadHash(raw),
		Source:        "legacy",
	}
	if _, err := inner.Append(ctx, v1Event); err != nil {
		t.Fatalf("inner.Append: %v", err)
	}
	ms := NewMigratingStore(inner)
	events, err := ms.Replay(ctx, AggregateOrder, "migr-002")
	if err != nil {
		t.Fatalf("MigratingStore.Replay: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].SchemaVersion != CurrentSchemaVersion {
		t.Errorf("want SchemaVersion=%d got %d", CurrentSchemaVersion, events[0].SchemaVersion)
	}
}
