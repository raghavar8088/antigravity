package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/ledger"
)

// stubRepairTarget is a test double for RepairTarget.
type stubRepairTarget struct {
	rebuildCount    int
	aggregateBuilds int
	shouldFail      bool
}

func (s *stubRepairTarget) RebuildProjections(ctx context.Context, accountID string) error {
	s.rebuildCount++
	if s.shouldFail {
		return context.DeadlineExceeded
	}
	return nil
}

func (s *stubRepairTarget) RebuildAggregate(ctx context.Context, aggregateType, aggregateID string) error {
	s.aggregateBuilds++
	if s.shouldFail {
		return context.DeadlineExceeded
	}
	return nil
}

func newTestRepairEngine(t *testing.T) (*RepairEngine, *stubRepairTarget, ledger.Store) {
	t.Helper()
	store := ledger.NewMemoryStore()
	target := &stubRepairTarget{}
	engine := NewRepairEngine(store, target, "test-account", "test-exchange")
	return engine, target, store
}

func TestRepairEngine_AutoRepair_ProjectionRebuild(t *testing.T) {
	engine, target, _ := newTestRepairEngine(t)

	mismatches := []Mismatch{
		{
			ID:             "m1",
			Domain:         DomainBalance,
			Type:           "equity_drift",
			Severity:       SeverityMedium,
			AutoRepairable: true,
			RepairHint:     RepairTypeProjectionRebuild,
			Reference:      "balance",
			DetectedAt:     time.Now(),
		},
	}

	results := engine.Repair(context.Background(), "run-001", mismatches)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("expected success: %s", results[0].Error)
	}
	if target.rebuildCount != 1 {
		t.Errorf("expected 1 projection rebuild, got %d", target.rebuildCount)
	}
	if results[0].RepairType != RepairTypeProjectionRebuild {
		t.Errorf("expected projection_rebuild, got %s", results[0].RepairType)
	}
}

func TestRepairEngine_AutoRepair_CorrectionEvent(t *testing.T) {
	engine, _, store := newTestRepairEngine(t)

	mismatches := []Mismatch{
		{
			ID:             "m2",
			Domain:         DomainPosition,
			Type:           "wrong_entry_price",
			Severity:       SeverityMedium,
			Symbol:         "BTCUSDT",
			Reference:      "pos-001",
			ExchangeVal:    "50100.00",
			InternalVal:    "50000.00",
			DriftPct:       0.2,
			AutoRepairable: true,
			RepairHint:     RepairTypeCorrectionEvent,
			DetectedAt:     time.Now(),
		},
	}

	results := engine.Repair(context.Background(), "run-002", mismatches)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("expected success: %s", results[0].Error)
	}

	// Verify correction event was appended to the ledger.
	events, _ := store.Replay(context.Background(), ledger.AggregateReconciliation, "recon-test-account-test-exchange")
	foundCorrection := false
	for _, ev := range events {
		if ev.EventType == ledger.EventReconciliationCorrected {
			foundCorrection = true
		}
	}
	if !foundCorrection {
		t.Error("expected RECONCILIATION_CORRECTED event in ledger")
	}
}

func TestRepairEngine_NonAutoRepairable_Escalates(t *testing.T) {
	engine, target, store := newTestRepairEngine(t)

	mismatches := []Mismatch{
		{
			ID:             "m3",
			Domain:         DomainPosition,
			Type:           "ghost_position",
			Severity:       SeverityCritical,
			Symbol:         "BTCUSDT",
			Reference:      "ghost-pos",
			AutoRepairable: false,
			RepairHint:     "manual_investigation",
			DetectedAt:     time.Now(),
		},
	}

	results := engine.Repair(context.Background(), "run-003", mismatches)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Error("ghost position escalation should not be marked success")
	}
	if results[0].RepairType != RepairTypeManualIntervention {
		t.Errorf("expected manual_intervention, got %s", results[0].RepairType)
	}
	if target.rebuildCount != 0 {
		t.Error("should not attempt projection rebuild for ghost position")
	}

	// Verify MANUAL_INTERVENTION_REQUIRED event was emitted.
	events, _ := store.Replay(context.Background(), ledger.AggregateReconciliation, "recon-test-account-test-exchange")
	found := false
	for _, ev := range events {
		if ev.EventType == EventManualInterventionRequired {
			found = true
		}
	}
	if !found {
		t.Error("expected MANUAL_INTERVENTION_REQUIRED event in ledger")
	}
}

func TestRepairEngine_RepairFailure_EmitsFailedEvent(t *testing.T) {
	_, target, store := newTestRepairEngine(t)
	target.shouldFail = true

	engine := NewRepairEngine(store, target, "test-account", "test-exchange")

	mismatches := []Mismatch{
		{
			ID:             "m4",
			Domain:         DomainBalance,
			Type:           "equity_drift",
			Severity:       SeverityHigh,
			AutoRepairable: true,
			RepairHint:     RepairTypeProjectionRebuild,
			DetectedAt:     time.Now(),
		},
	}

	results := engine.Repair(context.Background(), "run-004", mismatches)
	if results[0].Success {
		t.Error("expected repair failure")
	}
	if results[0].Error == "" {
		t.Error("expected error message")
	}

	// Verify REPAIR_FAILED event was emitted.
	events, _ := store.Replay(context.Background(), ledger.AggregateReconciliation, "recon-test-account-test-exchange")
	found := false
	for _, ev := range events {
		if ev.EventType == EventRepairFailed {
			found = true
		}
	}
	if !found {
		t.Error("expected RECONCILIATION_REPAIR_FAILED event in ledger")
	}
}

func TestRepairEngine_MixedMismatches(t *testing.T) {
	engine, target, _ := newTestRepairEngine(t)

	mismatches := []Mismatch{
		{ID: "m1", Domain: DomainBalance, Type: "equity_drift", AutoRepairable: true, RepairHint: RepairTypeProjectionRebuild, DetectedAt: time.Now()},
		{ID: "m2", Domain: DomainPosition, Type: "ghost_position", AutoRepairable: false, RepairHint: "manual_investigation", DetectedAt: time.Now()},
		{ID: "m3", Domain: DomainFill, Type: "missing_fill", AutoRepairable: false, RepairHint: "manual_investigation", DetectedAt: time.Now()},
		{ID: "m4", Domain: DomainOrder, Type: "wrong_status", AutoRepairable: true, RepairHint: RepairTypeCorrectionEvent, DetectedAt: time.Now()},
	}

	results := engine.Repair(context.Background(), "run-005", mismatches)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	autoRepaired := 0
	escalated := 0
	for _, r := range results {
		if r.RepairType == RepairTypeManualIntervention {
			escalated++
		} else if r.Success {
			autoRepaired++
		}
	}
	if autoRepaired != 2 {
		t.Errorf("expected 2 auto-repairs, got %d", autoRepaired)
	}
	if escalated != 2 {
		t.Errorf("expected 2 escalations, got %d", escalated)
	}
	if target.rebuildCount != 1 {
		t.Errorf("expected 1 projection rebuild, got %d", target.rebuildCount)
	}
}
