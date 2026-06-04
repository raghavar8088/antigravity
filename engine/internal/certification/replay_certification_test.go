package certification

import (
	"context"
	"fmt"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// ─── Phase 2: Massive Replay Validation ─────────────────────────────────────

// seedAccount populates a MemoryStore with n complete order lifecycle event
// triples (CREATED → VALIDATED → FILLED) for the given accountID.
func seedAccount(t *testing.T, store ledger.Store, accountID string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		orderID := fmt.Sprintf("%s_ORD_%07d", accountID, i)
		for _, et := range []ledger.EventType{
			ledger.EventOrderCreated,
			ledger.EventOrderValidated,
			ledger.EventOrderFilled,
		} {
			ev := ledger.NewEvent(ledger.AggregateOrder, orderID, et, nil)
			ev.AccountID = accountID
			if _, err := store.Append(ctx, ev); err != nil {
				t.Fatalf("seedAccount[%d][%s]: %v", i, et, err)
			}
		}
	}
}

// seedMixedAccount adds orders, position events, and risk events to simulate
// a real multi-aggregate account session.
func seedMixedAccount(t *testing.T, store ledger.Store, accountID string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		orderID := fmt.Sprintf("%s_ORD_%07d", accountID, i)
		posID := fmt.Sprintf("%s_POS_%07d", accountID, i)

		// Order events
		for _, et := range []ledger.EventType{
			ledger.EventOrderCreated, ledger.EventOrderFilled,
		} {
			ev := ledger.NewEvent(ledger.AggregateOrder, orderID, et, nil)
			ev.AccountID = accountID
			if _, err := store.Append(ctx, ev); err != nil {
				t.Fatalf("seedMixed order[%d][%s]: %v", i, et, err)
			}
		}

		// Position events
		for _, et := range []ledger.EventType{
			ledger.EventPositionOpened, ledger.EventPositionClosed,
		} {
			ev := ledger.NewEvent(ledger.AggregatePosition, posID, et, nil)
			ev.AccountID = accountID
			if _, err := store.Append(ctx, ev); err != nil {
				t.Fatalf("seedMixed position[%d][%s]: %v", i, et, err)
			}
		}

		// Risk events
		riskID := fmt.Sprintf("%s_RISK_%07d", accountID, i)
		ev := ledger.NewEvent(ledger.AggregateRisk, riskID, ledger.EventRiskApproved, nil)
		ev.AccountID = accountID
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("seedMixed risk[%d]: %v", i, err)
		}
	}
}

// TestReplay_100K validates replay correctness and determinism at 100K events.
func TestReplay_100K(t *testing.T) {
	const n = 33334 // 3 events each → ~100K
	store := newCertStore(t)
	accountID := "REPLAY_100K"

	seedAccount(t, store, accountID, n)

	ctx := context.Background()
	r1, err := ledger.ReplayEverything(ctx, store, accountID)
	if err != nil {
		t.Fatalf("first replay: %v", err)
	}
	r2, err := ledger.ReplayEverything(ctx, store, accountID)
	if err != nil {
		t.Fatalf("second replay: %v", err)
	}

	if r1.TotalEventCount != n*3 {
		t.Errorf("want %d events, got %d", n*3, r1.TotalEventCount)
	}
	if r1.TotalEventCount != r2.TotalEventCount {
		t.Errorf("non-determinism: %d vs %d", r1.TotalEventCount, r2.TotalEventCount)
	}
	if len(r1.Orders) != len(r2.Orders) {
		t.Errorf("order slice non-determinism: %d vs %d", len(r1.Orders), len(r2.Orders))
	}
}

// TestReplay_1M validates replay at 1M events.
func TestReplay_1M(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1M replay in short mode")
	}
	const n = 333334
	store := newCertStore(t)
	accountID := "REPLAY_1M"

	seedAccount(t, store, accountID, n)

	start := time.Now()
	r, err := ledger.ReplayEverything(context.Background(), store, accountID)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("replay 1M: %v", err)
	}
	if r.TotalEventCount != n*3 {
		t.Errorf("want %d events, got %d", n*3, r.TotalEventCount)
	}
	t.Logf("1M event replay: %v (%.0f events/sec)", elapsed,
		float64(r.TotalEventCount)/elapsed.Seconds())
}

// TestReplay_5M validates replay at 5M events.
func TestReplay_5M(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 5M replay in short mode")
	}
	const n = 1666667
	store := newCertStore(t)
	accountID := "REPLAY_5M"

	seedAccount(t, store, accountID, n)

	start := time.Now()
	r, err := ledger.ReplayEverything(context.Background(), store, accountID)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("replay 5M: %v", err)
	}
	if r.TotalEventCount != n*3 {
		t.Errorf("want %d events, got %d", n*3, r.TotalEventCount)
	}
	t.Logf("5M event replay: %v (%.0f events/sec)", elapsed,
		float64(r.TotalEventCount)/elapsed.Seconds())
}

// TestReplay_MixedAggregates verifies each partition is correctly isolated
// during replay — orders go to Orders, positions to Positions, etc.
func TestReplay_MixedAggregates(t *testing.T) {
	const n = 1000
	store := newCertStore(t)
	accountID := "REPLAY_MIXED_001"

	seedMixedAccount(t, store, accountID, n)

	r, err := ledger.ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("ReplayEverything: %v", err)
	}

	// Expect 2 order events + 2 position events + 1 risk event per iteration.
	wantOrders := n * 2
	wantPositions := n * 2
	wantRisk := n

	if len(r.Orders) != wantOrders {
		t.Errorf("orders: want %d, got %d", wantOrders, len(r.Orders))
	}
	if len(r.Positions) != wantPositions {
		t.Errorf("positions: want %d, got %d", wantPositions, len(r.Positions))
	}
	if len(r.Risk) != wantRisk {
		t.Errorf("risk: want %d, got %d", wantRisk, len(r.Risk))
	}

	// Verify every order event in the Orders partition actually has type ORDER.
	for i, ev := range r.Orders {
		if ev.AggregateType != ledger.AggregateOrder {
			t.Errorf("Orders[%d] has wrong aggregate type: %s", i, ev.AggregateType)
		}
	}
}

// TestReplay_SequenceIntegrity verifies that sequence numbers are monotonically
// increasing within each aggregate — a precondition for correct state rebuild.
func TestReplay_SequenceIntegrity(t *testing.T) {
	store := newCertStore(t)
	accountID := "REPLAY_SEQ_001"
	orderID := "ORD_SEQ_001"
	ctx := context.Background()

	eventTypes := []ledger.EventType{
		ledger.EventOrderCreated,
		ledger.EventOrderValidated,
		ledger.EventOrderAccepted,
		ledger.EventOrderSubmitted,
		ledger.EventOrderAcked,
		ledger.EventOrderFilled,
	}

	for _, et := range eventTypes {
		ev := ledger.NewEvent(ledger.AggregateOrder, orderID, et, nil)
		ev.AccountID = accountID
		mustEvent(t, store, ev)
	}

	events, err := store.Replay(ctx, ledger.AggregateOrder, orderID)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(events) != len(eventTypes) {
		t.Fatalf("want %d events, got %d", len(eventTypes), len(events))
	}

	for i := 1; i < len(events); i++ {
		if events[i].SequenceNo <= events[i-1].SequenceNo {
			t.Errorf("sequence not monotonic at index %d: seq[%d]=%d, seq[%d]=%d",
				i, i-1, events[i-1].SequenceNo, i, events[i].SequenceNo)
		}
	}
}

// BenchmarkReplay_100K measures replay throughput for the 100K event dataset.
func BenchmarkReplay_100K(b *testing.B) {
	const n = 33334
	store := ledger.NewMemoryStore()
	accountID := "BENCH_100K"
	ctx := context.Background()
	for i := 0; i < n; i++ {
		oid := fmt.Sprintf("%s_ORD_%07d", accountID, i)
		for _, et := range []ledger.EventType{
			ledger.EventOrderCreated, ledger.EventOrderValidated, ledger.EventOrderFilled,
		} {
			ev := ledger.NewEvent(ledger.AggregateOrder, oid, et, nil)
			ev.AccountID = accountID
			if _, err := store.Append(ctx, ev); err != nil {
				b.Fatalf("seed: %v", err)
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ledger.ReplayEverything(ctx, store, accountID); err != nil {
			b.Fatalf("replay: %v", err)
		}
	}
}

// BenchmarkReplay_1M measures replay throughput for the 1M event dataset.
func BenchmarkReplay_1M(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping in short mode")
	}
	const n = 333334
	store := ledger.NewMemoryStore()
	accountID := "BENCH_1M"
	ctx := context.Background()
	for i := 0; i < n; i++ {
		oid := fmt.Sprintf("%s_ORD_%07d", accountID, i)
		for _, et := range []ledger.EventType{
			ledger.EventOrderCreated, ledger.EventOrderFilled,
		} {
			ev := ledger.NewEvent(ledger.AggregateOrder, oid, et, nil)
			ev.AccountID = accountID
			if _, err := store.Append(ctx, ev); err != nil {
				b.Fatalf("seed: %v", err)
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := ledger.ReplayEverything(ctx, store, accountID); err != nil {
			b.Fatalf("replay: %v", err)
		}
	}
}
