package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// TestProjections_BuildFromEvents verifies that all projections rebuild
// deterministically from ledger events — crash recovery compatibility.
func TestProjections_BuildFromEvents_Empty(t *testing.T) {
	proj := BuildReconciliationProjection(nil, "acc-1", "binance")
	if proj.TotalCycles != 0 {
		t.Errorf("expected 0 cycles, got %d", proj.TotalCycles)
	}
	if proj.LastDriftScore != 0 {
		t.Errorf("expected 0 drift score")
	}
}

func TestProjections_ReconciliationProjection_FromEvents(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "acc-1"
	exchange := "binance"
	aggID := "recon-acc-1-binance"

	// Append a ReconciliationCompleted event.
	payload := ReconciliationCompletedPayload{
		RunID:         "run-001",
		Domain:        DomainFull,
		Exchange:      exchange,
		AccountID:     accountID,
		MismatchCount: 2,
		RepairCount:   1,
		DriftScore:    15.5,
		DurationMs:    250,
		CompletedAt:   time.Now().UTC(),
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateReconciliation,
		AggregateID:   aggID,
		EventType:     EventReconciliationCompleted,
		AccountID:     accountID,
		Payload:       payload,
		Source:        "test",
	})
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	if _, err := store.Append(ctx, ev); err != nil {
		t.Fatalf("append event: %v", err)
	}

	// Replay and rebuild projection.
	events, err := store.Replay(ctx, ledger.AggregateReconciliation, aggID)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	proj := BuildReconciliationProjection(events, accountID, exchange)
	if proj.TotalCycles != 1 {
		t.Errorf("expected 1 cycle, got %d", proj.TotalCycles)
	}
	if proj.TotalMismatches != 2 {
		t.Errorf("expected 2 mismatches, got %d", proj.TotalMismatches)
	}
	if proj.TotalRepairs != 1 {
		t.Errorf("expected 1 repair, got %d", proj.TotalRepairs)
	}
	if proj.LastDriftScore != 15.5 {
		t.Errorf("expected drift=15.5, got %.2f", proj.LastDriftScore)
	}
	if proj.LastRunID != "run-001" {
		t.Errorf("expected run-001, got %s", proj.LastRunID)
	}
}

func TestProjections_DriftProjection_TracksHistory(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "acc-1"
	exchange := "binance"
	aggID := "recon-acc-1-binance"

	scores := []float64{5.0, 10.0, 0.0, 25.0, 0.0}
	for i, score := range scores {
		payload := ReconciliationCompletedPayload{
			RunID:       "run-" + string(rune('A'+i)),
			Domain:      DomainFull,
			Exchange:    exchange,
			AccountID:   accountID,
			DriftScore:  score,
			CompletedAt: time.Now().UTC().Add(time.Duration(i) * time.Second),
		}
		ev, _ := ledger.NewEvent(ledger.NewEventInput{
			AggregateType: ledger.AggregateReconciliation,
			AggregateID:   aggID,
			EventType:     EventReconciliationCompleted,
			AccountID:     accountID,
			Payload:       payload,
			Source:        "test",
		})
		store.Append(ctx, ev)
	}

	events, _ := store.Replay(ctx, ledger.AggregateReconciliation, aggID)
	proj := BuildDriftProjection(events, accountID, exchange)

	if len(proj.History) != 5 {
		t.Errorf("expected 5 history points, got %d", len(proj.History))
	}
	if proj.PeakScore != 25.0 {
		t.Errorf("expected peak=25.0, got %.2f", proj.PeakScore)
	}
	if proj.CurrentScore != 0.0 {
		t.Errorf("expected current=0.0, got %.2f", proj.CurrentScore)
	}
}

func TestProjections_RepairProjection_FromEvents(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "acc-1"
	exchange := "binance"
	aggID := "recon-acc-1-binance"

	// Append repair completed.
	repairPayload := RepairPayload{
		RunID:       "run-001",
		RepairID:    "repair-001",
		RepairType:  RepairTypeProjectionRebuild,
		Domain:      DomainBalance,
		Exchange:    exchange,
		AccountID:   accountID,
		Description: "rebuilt balance projection",
		Automatic:   true,
		CompletedAt: time.Now().UTC(),
	}
	ev1, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateReconciliation,
		AggregateID:   aggID,
		EventType:     EventRepairCompleted,
		AccountID:     accountID,
		Payload:       repairPayload,
		Source:        "test",
	})
	store.Append(ctx, ev1)

	// Append repair failed.
	failPayload := RepairPayload{
		RunID:       "run-002",
		RepairID:    "repair-002",
		RepairType:  RepairTypeCorrectionEvent,
		Domain:      DomainPosition,
		Exchange:    exchange,
		AccountID:   accountID,
		Error:       "timeout",
		CompletedAt: time.Now().UTC(),
	}
	ev2, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    aggID,
		EventType:      EventRepairFailed,
		AccountID:      accountID,
		IdempotencyKey: "repair-fail-002",
		Payload:        failPayload,
		Source:         "test",
	})
	store.Append(ctx, ev2)

	// Append manual intervention.
	interventionPayload := ManualInterventionPayload{
		RunID:      "run-003",
		Domain:     DomainFill,
		Exchange:   exchange,
		AccountID:  accountID,
		Severity:   SeverityCritical,
		RequiredAt: time.Now().UTC(),
	}
	ev3, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    aggID,
		EventType:      EventManualInterventionRequired,
		AccountID:      accountID,
		IdempotencyKey: "intervention-003",
		Payload:        interventionPayload,
		Source:         "test",
	})
	store.Append(ctx, ev3)

	events, _ := store.Replay(ctx, ledger.AggregateReconciliation, aggID)
	proj := BuildRepairProjection(events, accountID, exchange)

	if proj.TotalRepairs != 2 {
		t.Errorf("expected 2 repairs, got %d", proj.TotalRepairs)
	}
	if proj.SuccessfulRepairs != 1 {
		t.Errorf("expected 1 successful repair, got %d", proj.SuccessfulRepairs)
	}
	if proj.FailedRepairs != 1 {
		t.Errorf("expected 1 failed repair, got %d", proj.FailedRepairs)
	}
	if proj.TotalEscalations != 1 {
		t.Errorf("expected 1 escalation, got %d", proj.TotalEscalations)
	}
	if len(proj.RecentRepairs) != 3 {
		t.Errorf("expected 3 recent repair records, got %d", len(proj.RecentRepairs))
	}
}

func TestProjections_AccountProjection_DriftFreeAfterCleanAudit(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "acc-1"
	exchange := "binance"
	aggID := "recon-acc-1-binance"

	// First a mismatch.
	mismatch := MismatchPayload{
		Domain:    DomainPosition,
		Exchange:  exchange,
		AccountID: accountID,
		Severity:  SeverityHigh,
	}
	ev1, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateReconciliation,
		AggregateID:   aggID,
		EventType:     EventPositionMismatchDetected,
		AccountID:     accountID,
		Payload:       mismatch,
		Source:        "test",
	})
	store.Append(ctx, ev1)

	// Then a clean full audit.
	completed := ReconciliationCompletedPayload{
		Domain:        DomainFull,
		Exchange:      exchange,
		AccountID:     accountID,
		MismatchCount: 0,
		CompletedAt:   time.Now().UTC(),
	}
	ev2, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    aggID,
		EventType:      EventReconciliationCompleted,
		AccountID:      accountID,
		IdempotencyKey: "completed-clean",
		Payload:        completed,
		Source:         "test",
	})
	store.Append(ctx, ev2)

	events, _ := store.Replay(ctx, ledger.AggregateReconciliation, aggID)
	proj := BuildAccountReconciliationProjection(events, accountID, exchange)

	if !proj.IsDriftFree {
		t.Error("expected IsDriftFree=true after clean full audit")
	}
	if proj.PositionDrift != 0 {
		t.Errorf("expected 0 position drift, got %d", proj.PositionDrift)
	}
}

// TestReplayCompatibility verifies that the entire reconciliation event stream
// can be replayed from scratch to rebuild all projections deterministically.
func TestReplayCompatibility_AllProjections(t *testing.T) {
	ctx := context.Background()
	store := ledger.NewMemoryStore()
	accountID := "acc-1"
	exchange := "binance"
	aggID := "recon-acc-1-binance"

	// Simulate a full reconciliation run by appending all event types.
	eventsToAppend := []struct {
		evType  ledger.EventType
		payload interface{}
	}{
		{ledger.EventReconciliationStarted, ReconciliationStartedPayload{
			RunID: "run-1", Domain: DomainFull, Exchange: exchange, AccountID: accountID,
		}},
		{EventPositionMismatchDetected, MismatchPayload{
			RunID: "run-1", Domain: DomainPosition, Exchange: exchange, AccountID: accountID, Severity: SeverityCritical,
		}},
		{EventManualInterventionRequired, ManualInterventionPayload{
			RunID: "run-1", Domain: DomainPosition, Exchange: exchange, AccountID: accountID, Severity: SeverityCritical,
		}},
		{EventReconciliationCompleted, ReconciliationCompletedPayload{
			RunID: "run-1", Domain: DomainFull, Exchange: exchange, AccountID: accountID,
			MismatchCount: 1, DriftScore: 50.0, CompletedAt: time.Now().UTC(),
		}},
		{EventRepairStarted, RepairPayload{
			RunID: "run-1", RepairID: "r-1", RepairType: RepairTypeProjectionRebuild, Exchange: exchange, AccountID: accountID,
		}},
		{EventRepairCompleted, RepairPayload{
			RunID: "run-1", RepairID: "r-1", RepairType: RepairTypeProjectionRebuild, Exchange: exchange, AccountID: accountID,
		}},
	}

	for i, e := range eventsToAppend {
		ev, err := ledger.NewEvent(ledger.NewEventInput{
			AggregateType:  ledger.AggregateReconciliation,
			AggregateID:    aggID,
			EventType:      e.evType,
			AccountID:      accountID,
			IdempotencyKey: "test-" + string(rune('A'+i)),
			Payload:        e.payload,
			Source:         "test",
		})
		if err != nil {
			t.Fatalf("build event %d: %v", i, err)
		}
		if _, err := store.Append(ctx, ev); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
	}

	// Replay all events and rebuild each projection type independently.
	events, err := store.Replay(ctx, ledger.AggregateReconciliation, aggID)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(events) != len(eventsToAppend) {
		t.Errorf("expected %d events, got %d", len(eventsToAppend), len(events))
	}

	reconProj := BuildReconciliationProjection(events, accountID, exchange)
	if reconProj.TotalCycles != 1 {
		t.Errorf("expected 1 cycle, got %d", reconProj.TotalCycles)
	}
	if reconProj.TotalEscalations != 1 {
		t.Errorf("expected 1 escalation, got %d", reconProj.TotalEscalations)
	}

	driftProj := BuildDriftProjection(events, accountID, exchange)
	if len(driftProj.History) != 1 {
		t.Errorf("expected 1 drift history point, got %d", len(driftProj.History))
	}

	repairProj := BuildRepairProjection(events, accountID, exchange)
	if repairProj.TotalRepairs != 1 {
		t.Errorf("expected 1 repair, got %d", repairProj.TotalRepairs)
	}

	acctProj := BuildAccountReconciliationProjection(events, accountID, exchange)
	if acctProj.IsDriftFree {
		t.Error("expected drift detected (no clean full audit)")
	}
}
