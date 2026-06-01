package reconciliationv2

import (
	"context"
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/ledger"
)

// RepairType classifies the mechanism used to correct internal state.
const (
	RepairTypeProjectionRebuild  = "projection_rebuild"
	RepairTypeAggregateRebuild   = "aggregate_rebuild"
	RepairTypeReplay             = "replay_repair"
	RepairTypeStateSync          = "state_sync"
	RepairTypeCorrectionEvent    = "correction_event"
	RepairTypeManualIntervention = "manual_intervention"
)

// RepairTarget is the interface the repair engine uses to trigger rebuilds.
// Implemented by the caller (wrapping omsv3.ReplayAll etc.) to keep reconciliationv2
// decoupled from the omsv3 package.
type RepairTarget interface {
	// RebuildProjections replays all account events and rebuilds all CQRS projections.
	RebuildProjections(ctx context.Context, accountID string) error

	// RebuildAggregate replays a single aggregate and updates its in-memory state.
	RebuildAggregate(ctx context.Context, aggregateType, aggregateID string) error
}

// RepairResult records the outcome of a single repair attempt.
type RepairResult struct {
	RepairID    string
	MismatchID  string
	RepairType  string
	Success     bool
	Description string
	Error       string
	Duration    time.Duration
	AppliedAt   time.Time
}

// RepairEngine performs automatic correction of detected drift.
// It NEVER modifies existing ledger events — it only appends corrective events.
type RepairEngine struct {
	store     ledger.Store
	target    RepairTarget
	accountID string
	exchange  string
}

// NewRepairEngine creates a RepairEngine.
func NewRepairEngine(store ledger.Store, target RepairTarget, accountID, exchange string) *RepairEngine {
	return &RepairEngine{
		store:     store,
		target:    target,
		accountID: accountID,
		exchange:  exchange,
	}
}

// Repair attempts to automatically fix all auto-repairable mismatches.
// Mismatches that are not auto-repairable are escalated to ManualInterventionRequired events.
// Returns the list of repair results, including escalations.
func (e *RepairEngine) Repair(ctx context.Context, runID string, mismatches []Mismatch) []RepairResult {
	results := make([]RepairResult, 0, len(mismatches))

	for _, m := range mismatches {
		if !m.AutoRepairable {
			results = append(results, e.escalate(ctx, runID, m))
			continue
		}

		start := time.Now()
		var repairErr error

		switch m.RepairHint {
		case RepairTypeProjectionRebuild:
			repairErr = e.repairProjectionRebuild(ctx, runID, m)
		case RepairTypeCorrectionEvent:
			repairErr = e.repairCorrectionEvent(ctx, runID, m)
		default:
			repairErr = e.repairProjectionRebuild(ctx, runID, m)
		}

		dur := time.Since(start)
		res := RepairResult{
			RepairID:   newMismatchID(),
			MismatchID: m.ID,
			RepairType: m.RepairHint,
			Duration:   dur,
			AppliedAt:  time.Now().UTC(),
		}

		if repairErr != nil {
			res.Success = false
			res.Error = repairErr.Error()
			res.Description = fmt.Sprintf("repair failed for %s/%s: %v", m.Domain, m.Type, repairErr)
			e.emitRepairFailed(ctx, runID, res, m)
			log.Printf("[RECON-V2] repair FAILED domain=%s type=%s err=%v", m.Domain, m.Type, repairErr)
		} else {
			res.Success = true
			res.Description = fmt.Sprintf("repaired %s/%s mismatch for %s", m.Domain, m.Type, m.Reference)
			e.emitRepairCompleted(ctx, runID, res, m)
			log.Printf("[RECON-V2] repair OK domain=%s type=%s ref=%s", m.Domain, m.Type, m.Reference)
		}

		results = append(results, res)
	}

	return results
}

// repairProjectionRebuild triggers a full projection rebuild from the ledger.
func (e *RepairEngine) repairProjectionRebuild(ctx context.Context, runID string, m Mismatch) error {
	if e.target == nil {
		return nil // no-op when no target is wired (e.g. in tests)
	}
	if err := e.emitRepairStarted(ctx, runID, m, RepairTypeProjectionRebuild); err != nil {
		log.Printf("[RECON-V2] warn: emit repair-started: %v", err)
	}
	return e.target.RebuildProjections(ctx, e.accountID)
}

// repairCorrectionEvent appends an immutable corrective event to the ledger.
// The corrective event records what the exchange says the true value is.
// Downstream projections rebuild their state by incorporating this event.
func (e *RepairEngine) repairCorrectionEvent(ctx context.Context, runID string, m Mismatch) error {
	if err := e.emitRepairStarted(ctx, runID, m, RepairTypeCorrectionEvent); err != nil {
		log.Printf("[RECON-V2] warn: emit repair-started: %v", err)
	}

	correctionPayload := map[string]interface{}{
		"run_id":         runID,
		"mismatch_id":    m.ID,
		"domain":         string(m.Domain),
		"mismatch_type":  m.Type,
		"exchange_value": m.ExchangeVal,
		"internal_value": m.InternalVal,
		"drift_pct":      m.DriftPct,
		"symbol":         m.Symbol,
		"reference":      m.Reference,
		"corrected_at":   time.Now().UTC(),
		"exchange":       e.exchange,
		"authority":      "exchange",
	}

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", e.accountID, e.exchange),
		EventType:      ledger.EventReconciliationCorrected,
		AccountID:      e.accountID,
		Symbol:         m.Symbol,
		CorrelationID:  runID,
		IdempotencyKey: fmt.Sprintf("repair-correction-%s", m.ID),
		Payload:        correctionPayload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		return fmt.Errorf("repair: build correction event: %w", err)
	}

	if _, err := e.store.Append(ctx, ev); err != nil {
		return fmt.Errorf("repair: append correction event: %w", err)
	}
	return nil
}

// escalate emits a ManualInterventionRequired event for mismatches that cannot
// be safely auto-repaired (ghost positions, missing fills, duplicate positions).
func (e *RepairEngine) escalate(ctx context.Context, runID string, m Mismatch) RepairResult {
	payload := ManualInterventionPayload{
		RunID:      runID,
		Domain:     m.Domain,
		Exchange:   e.exchange,
		AccountID:  e.accountID,
		Severity:   m.Severity,
		Reference:  m.Reference,
		Message:    m.Message,
		Reason:     fmt.Sprintf("auto-repair not safe for type=%s", m.Type),
		RequiredAt: time.Now().UTC(),
	}

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", e.accountID, e.exchange),
		EventType:      EventManualInterventionRequired,
		AccountID:      e.accountID,
		Symbol:         m.Symbol,
		CorrelationID:  runID,
		IdempotencyKey: fmt.Sprintf("manual-intervention-%s", m.ID),
		Payload:        payload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		log.Printf("[RECON-V2] warn: build manual-intervention event: %v", err)
	} else if _, appErr := e.store.Append(ctx, ev); appErr != nil {
		log.Printf("[RECON-V2] warn: append manual-intervention event: %v", appErr)
	}

	log.Printf("[RECON-V2] MANUAL INTERVENTION REQUIRED domain=%s type=%s severity=%s ref=%s",
		m.Domain, m.Type, m.Severity, m.Reference)

	return RepairResult{
		RepairID:    newMismatchID(),
		MismatchID:  m.ID,
		RepairType:  RepairTypeManualIntervention,
		Success:     false,
		Description: fmt.Sprintf("manual intervention required: %s", m.Message),
		AppliedAt:   time.Now().UTC(),
	}
}

func (e *RepairEngine) emitRepairStarted(ctx context.Context, runID string, m Mismatch, repairType string) error {
	repairID := newMismatchID()
	payload := RepairPayload{
		RunID:       runID,
		RepairID:    repairID,
		RepairType:  repairType,
		Domain:      m.Domain,
		Exchange:    e.exchange,
		AccountID:   e.accountID,
		MismatchRef: m.ID,
		Description: fmt.Sprintf("repairing %s/%s for %s", m.Domain, m.Type, m.Reference),
		Automatic:   true,
		InitiatedAt: time.Now().UTC(),
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", e.accountID, e.exchange),
		EventType:      EventRepairStarted,
		AccountID:      e.accountID,
		CorrelationID:  runID,
		IdempotencyKey: fmt.Sprintf("repair-start-%s", repairID),
		Payload:        payload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		return err
	}
	_, err = e.store.Append(ctx, ev)
	return err
}

func (e *RepairEngine) emitRepairCompleted(ctx context.Context, runID string, res RepairResult, m Mismatch) {
	completedAt := time.Now().UTC()
	payload := RepairPayload{
		RunID:       runID,
		RepairID:    res.RepairID,
		RepairType:  res.RepairType,
		Domain:      m.Domain,
		Exchange:    e.exchange,
		AccountID:   e.accountID,
		MismatchRef: m.ID,
		Description: res.Description,
		Automatic:   true,
		InitiatedAt: res.AppliedAt,
		CompletedAt: completedAt,
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", e.accountID, e.exchange),
		EventType:      EventRepairCompleted,
		AccountID:      e.accountID,
		CorrelationID:  runID,
		IdempotencyKey: fmt.Sprintf("repair-complete-%s", res.RepairID),
		Payload:        payload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		log.Printf("[RECON-V2] warn: build repair-completed event: %v", err)
		return
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		log.Printf("[RECON-V2] warn: append repair-completed event: %v", err)
	}
}

func (e *RepairEngine) emitRepairFailed(ctx context.Context, runID string, res RepairResult, m Mismatch) {
	completedAt := time.Now().UTC()
	payload := RepairPayload{
		RunID:       runID,
		RepairID:    res.RepairID,
		RepairType:  res.RepairType,
		Domain:      m.Domain,
		Exchange:    e.exchange,
		AccountID:   e.accountID,
		MismatchRef: m.ID,
		Description: res.Description,
		Automatic:   true,
		Error:       res.Error,
		InitiatedAt: res.AppliedAt,
		CompletedAt: completedAt,
	}
	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", e.accountID, e.exchange),
		EventType:      EventRepairFailed,
		AccountID:      e.accountID,
		CorrelationID:  runID,
		IdempotencyKey: fmt.Sprintf("repair-fail-%s", res.RepairID),
		Payload:        payload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		log.Printf("[RECON-V2] warn: build repair-failed event: %v", err)
		return
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		log.Printf("[RECON-V2] warn: append repair-failed event: %v", err)
	}
}
