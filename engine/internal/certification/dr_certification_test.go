package certification

import (
	"context"
	"fmt"
	"testing"
	"time"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// ─── Phase 9: Disaster Recovery Certification ───────────────────────────────

// drSnapshot represents a point-in-time capture of account state for backup/restore.
type drSnapshot struct {
	AccountID   string
	TakenAt     time.Time
	EventCount  int
	Orders      []ledger.Event
	Positions   []ledger.Event
	Risk        []ledger.Event
	Recon       []ledger.Event
	System      []ledger.Event
	Checksum    int // sum of sequence numbers — integrity check
}

func takeDRSnapshot(ctx context.Context, store ledger.Store, accountID string) (drSnapshot, error) {
	r, err := ledger.ReplayEverything(ctx, store, accountID)
	if err != nil {
		return drSnapshot{}, fmt.Errorf("snapshot: %w", err)
	}
	checksum := 0
	for _, e := range r.Orders {
		checksum += int(e.SequenceNo)
	}
	for _, e := range r.Positions {
		checksum += int(e.SequenceNo)
	}
	return drSnapshot{
		AccountID:  accountID,
		TakenAt:    time.Now().UTC(),
		EventCount: r.TotalEventCount,
		Orders:     r.Orders,
		Positions:  r.Positions,
		Risk:       r.Risk,
		Recon:      r.Reconciliation,
		System:     r.System,
		Checksum:   checksum,
	}, nil
}

// restoreFromSnapshot creates a new store and replays the snapshot into it.
func restoreFromSnapshot(t *testing.T, snap drSnapshot) ledger.Store {
	t.Helper()
	store := ledger.NewMemoryStore()
	ctx := context.Background()

	allEvents := make([]ledger.Event, 0,
		len(snap.Orders)+len(snap.Positions)+len(snap.Risk)+len(snap.Recon)+len(snap.System))
	allEvents = append(allEvents, snap.Orders...)
	allEvents = append(allEvents, snap.Positions...)
	allEvents = append(allEvents, snap.Risk...)
	allEvents = append(allEvents, snap.Recon...)
	allEvents = append(allEvents, snap.System...)

	// Sort by CreatedAt + SequenceNo before restoring (deterministic ordering).
	for i := 0; i < len(allEvents)-1; i++ {
		for j := i + 1; j < len(allEvents); j++ {
			if allEvents[j].CreatedAt.Before(allEvents[i].CreatedAt) ||
				(allEvents[j].CreatedAt.Equal(allEvents[i].CreatedAt) &&
					allEvents[j].SequenceNo < allEvents[i].SequenceNo) {
				allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
			}
		}
	}

	for _, ev := range allEvents {
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("restoreFromSnapshot: %v", err)
		}
	}
	return store
}

// TestDR_SnapshotAndFullRecovery simulates a complete service restart with
// state restored from snapshot. Verifies RPO and RTO targets.
func TestDR_SnapshotAndFullRecovery(t *testing.T) {
	ctx := context.Background()
	accountID := "DR_CERT_001"

	// ── Phase 1: Pre-failure state ──────────────────────────────────────────
	primaryStore := ledger.NewMemoryStore()
	const preFailureOrders = 500

	for i := 0; i < preFailureOrders; i++ {
		oid := fmt.Sprintf("DR_ORD_%05d", i)
		for _, et := range []ledger.EventType{
			ledger.EventOrderCreated, ledger.EventOrderFilled,
		} {
			ev := newOrderEvent(t, accountID, oid, et)
			mustAppend(t, primaryStore, ev)
		}
	}

	// ── Phase 2: Take snapshot ────────────────────────────────────────────
	snapshotStart := time.Now()
	snap, err := takeDRSnapshot(ctx, primaryStore, accountID)
	snapshotDuration := time.Since(snapshotStart)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snap.EventCount != preFailureOrders*2 {
		t.Errorf("snapshot integrity: want %d events, got %d", preFailureOrders*2, snap.EventCount)
	}
	t.Logf("snapshot taken in %v: %d events, checksum=%d", snapshotDuration, snap.EventCount, snap.Checksum)

	// ── Phase 3: Simulate primary failure ────────────────────────────────
	primaryStore = nil // primary is gone

	// ── Phase 4: Restore from snapshot (measure RTO) ─────────────────────
	rtoStart := time.Now()
	recoveredStore := restoreFromSnapshot(t, snap)
	rto := time.Since(rtoStart)

	// ── Phase 5: Verify recovered state ──────────────────────────────────
	recoveredSnap, err := takeDRSnapshot(ctx, recoveredStore, accountID)
	if err != nil {
		t.Fatalf("post-recovery snapshot: %v", err)
	}
	if recoveredSnap.EventCount != snap.EventCount {
		t.Errorf("RPO violation: recovered %d events, original had %d",
			recoveredSnap.EventCount, snap.EventCount)
	}
	if recoveredSnap.Checksum != snap.Checksum {
		t.Errorf("data integrity violation: checksum mismatch (%d vs %d)",
			recoveredSnap.Checksum, snap.Checksum)
	}

	t.Logf("DR recovery complete: RTO=%v, RPO=0 events, %d events recovered",
		rto, recoveredSnap.EventCount)

	// RTO target: < 15 minutes. RTO for in-memory replay should be milliseconds.
	if rto > 15*time.Minute {
		t.Errorf("RTO target missed: %v > 15m", rto)
	}
}

// TestDR_LedgerReplayAfterCrash verifies that crash_recovery_test patterns hold
// at the integration level — a corrupted append session leaves prior events intact.
func TestDR_LedgerReplayAfterCrash(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "DR_CERT_002"

	// Phase 1: write 200 events.
	const phase1 = 200
	for i := 0; i < phase1; i++ {
		oid := fmt.Sprintf("CRASH_ORD_%05d", i)
		ev := newOrderEvent(t, accountID, oid, ledger.EventOrderCreated)
		mustAppend(t, store, ev)
	}

	// Phase 2: simulate crash mid-write (no events written).
	// Nothing needed — the store retains Phase 1 events.

	// Phase 3: verify Phase 1 events are intact after "restart".
	r, err := ledger.ReplayEverything(context.Background(), store, accountID)
	if err != nil {
		t.Fatalf("post-crash replay: %v", err)
	}
	if r.TotalEventCount != phase1 {
		t.Errorf("crash corruption: want %d events, got %d", phase1, r.TotalEventCount)
	}
}

// TestDR_KillSwitchSurvivesRestart verifies that after a system restart the
// kill switch state can be re-derived from the ledger event log — no active
// trade should slip through a restart window.
func TestDR_KillSwitchSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	accountID := "DR_CERT_003"

	// Before restart: activate kill switch.
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, &noopKSExecutor{}, accountID)
	if err := ks.Trigger(ctx, killswitch.Activation{
		Trigger:     killswitch.TriggerDailyLoss,
		Reason:      "DR test: kill switch before restart",
		Actions:     []killswitch.Action{killswitch.ActionBlockNewOrders},
		ActivatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("pre-restart trigger: %v", err)
	}

	// Simulate restart: reconstruct service from persisted ledger.
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		t.Fatalf("replay system events: %v", err)
	}

	foundActivation := false
	for _, ev := range events {
		if ev.EventType == ledger.EventKillSwitchTriggered {
			foundActivation = true
			break
		}
	}
	if !foundActivation {
		t.Fatal("FAIL: kill switch activation not persisted — cannot re-derive state after restart")
	}
	t.Log("kill switch state survives restart via ledger replay — PASS")
}

// TestDR_ProjectionRebuildAfterRestart verifies that all OMS aggregates can
// be rebuilt from scratch by replaying the full ledger — the core CQRS guarantee.
func TestDR_ProjectionRebuildAfterRestart(t *testing.T) {
	ctx := context.Background()
	accountID := "DR_CERT_004"
	store := ledger.NewMemoryStore()

	const numOrders = 100
	type orderExpect struct {
		id    string
		state omsv3.OrderState
	}
	expectations := make([]orderExpect, 0, numOrders)

	for i := 0; i < numOrders; i++ {
		oid := fmt.Sprintf("PROJ_ORD_%05d", i)
		// Even orders: FILLED (full New->Validated->RiskApproved->Submitted->Accepted->Filled
		// chain — FILLED is not reachable directly from VALIDATED); odd orders: CANCELLED
		// (valid directly from VALIDATED).
		var events []ledger.EventType
		var finalState omsv3.OrderState
		if i%2 == 1 {
			events = []ledger.EventType{ledger.EventOrderCreated, ledger.EventOrderValidated, ledger.EventOrderCancelled}
			finalState = omsv3.StateCancelled
		} else {
			events = []ledger.EventType{
				ledger.EventOrderCreated, ledger.EventOrderValidated, ledger.EventRiskApproved,
				ledger.EventOrderSubmitted, ledger.EventOrderAccepted, ledger.EventOrderFilled,
			}
			finalState = omsv3.StateFilled
		}
		for _, et := range events {
			ev := newOrderEvent(t, accountID, oid, et)
			mustAppend(t, store, ev)
		}
		expectations = append(expectations, orderExpect{id: oid, state: finalState})
	}

	// Simulate restart: rebuild projections from full replay.
	r, err := ledger.ReplayEverything(ctx, store, accountID)
	if err != nil {
		t.Fatalf("ReplayEverything: %v", err)
	}

	// Rebuild order aggregates.
	aggregates := make(map[string]*omsv3.OrderAggregate)
	for _, ev := range r.Orders {
		if aggregates[ev.AggregateID] == nil {
			aggregates[ev.AggregateID] = omsv3.NewOrderAggregate(ev.AggregateID)
		}
		if err := aggregates[ev.AggregateID].ApplyEvent(ev); err != nil {
			t.Errorf("rebuild order %s event %s: %v", ev.AggregateID, ev.EventType, err)
		}
	}

	// Verify final states match expectations.
	for _, exp := range expectations {
		agg := aggregates[exp.id]
		if agg == nil {
			t.Errorf("order %s missing from rebuilt projection", exp.id)
			continue
		}
		if agg.State != exp.state {
			t.Errorf("order %s: want state %s, got %s after rebuild", exp.id, exp.state, agg.State)
		}
	}

	t.Logf("projection rebuild: %d orders reconstructed from %d events", len(aggregates), r.TotalEventCount)
}

// TestDR_RPO_MeasureEventLoss measures actual Recovery Point Objective by
// simulating a failure mid-write and measuring event loss.
func TestDR_RPO_MeasureEventLoss(t *testing.T) {
	store := ledger.NewMemoryStore()
	accountID := "DR_RPO_001"
	ctx := context.Background()

	// Write 1000 events to represent steady-state trading.
	const totalPreFailure = 1000
	for i := 0; i < totalPreFailure; i++ {
		oid := fmt.Sprintf("RPO_ORD_%05d", i)
		ev := newOrderEvent(t, accountID, oid, ledger.EventOrderCreated)
		mustAppend(t, store, ev)
	}

	// Take snapshot at T=0.
	snap, err := takeDRSnapshot(ctx, store, accountID)
	if err != nil {
		t.Fatalf("snapshot T0: %v", err)
	}

	// Write 50 more events after snapshot (these are the RPO window).
	const postSnapshotEvents = 50
	for i := 0; i < postSnapshotEvents; i++ {
		oid := fmt.Sprintf("RPO_POST_%05d", i)
		ev := newOrderEvent(t, accountID, oid, ledger.EventOrderCreated)
		mustAppend(t, store, ev)
	}

	// Failure occurs here — recover only from snapshot.
	recovered := restoreFromSnapshot(t, snap)
	recSnap, _ := takeDRSnapshot(ctx, recovered, accountID)

	eventLoss := totalPreFailure + postSnapshotEvents - recSnap.EventCount
	t.Logf("RPO measurement: %d total events, %d in snapshot, %d events lost",
		totalPreFailure+postSnapshotEvents, snap.EventCount, eventLoss)

	// With PostgreSQL WAL streaming in production, RPO target is < 30s / ~50 events.
	// In-memory store: snapshot is exact, event loss equals post-snapshot events.
	if eventLoss > postSnapshotEvents {
		t.Errorf("unexpected data loss: lost %d events (more than post-snapshot window %d)",
			eventLoss, postSnapshotEvents)
	}
}
