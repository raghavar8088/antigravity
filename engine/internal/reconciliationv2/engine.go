package reconciliationv2

import (
	"context"
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/ledger"
)

// ReconciliationEngine is the core orchestrator for a single exchange.
// It pulls exchange state via ReconciliationAdapter, reads internal state via
// OMSStateReader, runs all detectors, and delegates repairs to RepairEngine.
// One engine instance per exchange.
type ReconciliationEngine struct {
	adapter      ReconciliationAdapter
	omsReader    OMSStateReader
	store        ledger.Store
	repair       *RepairEngine
	audit        *AuditLog
	metrics      *Metrics
	accountID    string
	exchangeName string

	balanceDet  BalanceDriftDetector
	positionDet PositionDriftDetector
	orderDet    OrderDriftDetector
	fillDet     FillDriftDetector
	exposureDet ExposureDriftDetector
}

// NewReconciliationEngine wires together all components for one exchange.
func NewReconciliationEngine(
	adapter ReconciliationAdapter,
	omsReader OMSStateReader,
	store ledger.Store,
	repairTarget RepairTarget,
	metrics *Metrics,
	accountID string,
) *ReconciliationEngine {
	exchange := adapter.Name()
	repairEngine := NewRepairEngine(store, repairTarget, accountID, exchange)
	auditLog := NewAuditLog(store, accountID, exchange, 2000)

	return &ReconciliationEngine{
		adapter:      adapter,
		omsReader:    omsReader,
		store:        store,
		repair:       repairEngine,
		audit:        auditLog,
		metrics:      metrics,
		accountID:    accountID,
		exchangeName: exchange,
	}
}

// RunDomain executes a single reconciliation cycle for the given domain.
// It fetches exchange state, fetches OMS state, runs the relevant detectors,
// attempts repairs, emits events, and records the audit entry.
func (e *ReconciliationEngine) RunDomain(ctx context.Context, domain MismatchDomain) (AuditEntry, error) {
	runID := newRunID()
	start := time.Now()

	// Deliberately not persisted to the ledger: a "cycle started" record has
	// no consumer (nothing replays EventReconciliationStarted) and this
	// scheduler fires every 2-60s forever — persisting it was the single
	// largest contributor to an unbounded ledger growing into the hundreds
	// of thousands of events/day, which leaked memory until the engine got
	// OOM-killed roughly every 24-36 hours (Jun 2026 incident). See AuditLog
	// for the bounded, in-memory cycle history that replaces this.

	oms, err := e.omsReader.GetOMSSnapshot(ctx, e.accountID)
	if err != nil {
		return AuditEntry{}, fmt.Errorf("engine: get oms snapshot: %w", err)
	}

	var mismatches []Mismatch

	switch domain {
	case DomainBalance:
		mismatches, err = e.runBalance(ctx, oms)
	case DomainPosition:
		mismatches, err = e.runPositions(ctx, oms)
	case DomainOrder:
		mismatches, err = e.runOrders(ctx, oms)
	case DomainFill:
		mismatches, err = e.runFills(ctx, oms)
	case DomainExposure:
		mismatches, err = e.runExposure(ctx, oms)
	case DomainFull:
		mismatches, err = e.runFull(ctx, oms)
	default:
		mismatches, err = e.runFull(ctx, oms)
	}

	if err != nil {
		return AuditEntry{}, fmt.Errorf("engine: run domain %s: %w", domain, err)
	}

	// Emit individual mismatch events.
	for _, m := range mismatches {
		e.emitMismatch(ctx, runID, m)
	}

	// Attempt repairs.
	repairResults := e.repair.Repair(ctx, runID, mismatches)

	// Compute cycle metrics.
	durationMs := time.Since(start).Milliseconds()
	driftScore := DriftSeverityScore(mismatches)
	repairCount, escalateCount := 0, 0
	for _, r := range repairResults {
		if r.RepairType == RepairTypeManualIntervention {
			escalateCount++
		} else if r.Success {
			repairCount++
		}
	}

	entry := AuditEntry{
		RunID:         runID,
		Domain:        domain,
		Exchange:      e.exchangeName,
		AccountID:     e.accountID,
		MismatchCount: len(mismatches),
		RepairCount:   repairCount,
		EscalateCount: escalateCount,
		DriftScore:    driftScore,
		Mismatches:    mismatches,
		RepairResults: repairResults,
		StartedAt:     start,
		CompletedAt:   time.Now().UTC(),
		DurationMs:    durationMs,
	}

	e.audit.Record(ctx, entry)
	return entry, nil
}

func (e *ReconciliationEngine) skipPaperBalanceRecon() bool {
	return e.exchangeName == PaperRuntimeExchangeName
}

// runBalance fetches exchange balances and detects drift against OMS.
func (e *ReconciliationEngine) runBalance(ctx context.Context, oms OMSSnapshot) ([]Mismatch, error) {
	if e.skipPaperBalanceRecon() {
		return nil, nil
	}
	balances, err := e.adapter.GetBalances(ctx)
	if err != nil {
		return nil, fmt.Errorf("get balances: %w", err)
	}
	mismatches := e.balanceDet.Detect(balances, oms.Balance, time.Now().UTC())
	if e.metrics != nil {
		for _, m := range mismatches {
			e.metrics.SetBalanceDrift(e.exchangeName, m.Type, m.DriftPct)
		}
	}
	return mismatches, nil
}

// runPositions fetches exchange positions and detects drift.
func (e *ReconciliationEngine) runPositions(ctx context.Context, oms OMSSnapshot) ([]Mismatch, error) {
	positions, err := e.adapter.GetPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get positions: %w", err)
	}
	return e.positionDet.Detect(positions, oms.Positions, time.Now().UTC()), nil
}

// runOrders fetches exchange open orders and detects drift.
func (e *ReconciliationEngine) runOrders(ctx context.Context, oms OMSSnapshot) ([]Mismatch, error) {
	orders, err := e.adapter.GetOpenOrders(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("get open orders: %w", err)
	}
	return e.orderDet.Detect(orders, oms.OpenOrders, time.Now().UTC()), nil
}

// runFills fetches recent fills from exchange and detects drift.
func (e *ReconciliationEngine) runFills(ctx context.Context, oms OMSSnapshot) ([]Mismatch, error) {
	since := time.Now().UTC().Add(-30 * time.Minute)
	fills, err := e.adapter.GetFills(ctx, "", since)
	if err != nil {
		return nil, fmt.Errorf("get fills: %w", err)
	}
	return e.fillDet.Detect(fills, oms.RecentFills, time.Now().UTC()), nil
}

// runExposure computes gross/net notional from exchange positions and compares.
func (e *ReconciliationEngine) runExposure(ctx context.Context, oms OMSSnapshot) ([]Mismatch, error) {
	positions, err := e.adapter.GetPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("get positions for exposure: %w", err)
	}
	// riskEngineGross = 0 unless the caller wired a risk engine snapshot into OMSSnapshot.
	return e.exposureDet.Detect(positions, oms, 0, time.Now().UTC()), nil
}

// runFull runs all domains and aggregates mismatches — used for the 60-second audit.
func (e *ReconciliationEngine) runFull(ctx context.Context, oms OMSSnapshot) ([]Mismatch, error) {
	state, err := e.adapter.GetAccountState(ctx)
	if err != nil {
		return nil, fmt.Errorf("get account state: %w", err)
	}

	since := time.Now().UTC().Add(-30 * time.Minute)
	fills, err := e.adapter.GetFills(ctx, "", since)
	if err != nil {
		log.Printf("[RECON-V2] warn: full audit get fills: %v", err)
	}

	now := time.Now().UTC()
	var all []Mismatch

	if !e.skipPaperBalanceRecon() {
		all = append(all, e.balanceDet.Detect(state.Balances, oms.Balance, now)...)
	}
	all = append(all, e.positionDet.Detect(state.Positions, oms.Positions, now)...)
	all = append(all, e.orderDet.Detect(state.OpenOrders, oms.OpenOrders, now)...)
	if len(fills) > 0 {
		all = append(all, e.fillDet.Detect(fills, oms.RecentFills, now)...)
	}
	all = append(all, e.exposureDet.Detect(state.Positions, oms, 0, now)...)

	// Emit EXCHANGE_AUTHORITY_VERIFIED if no critical mismatches.
	critCount := 0
	for _, m := range all {
		if m.Severity == SeverityCritical {
			critCount++
		}
	}
	if critCount == 0 {
		e.emitAuthorityVerified(ctx, newRunID(), DomainFull)
	}

	return all, nil
}

// ─── Event emission helpers ───────────────────────────────────────────────────

func (e *ReconciliationEngine) emitMismatch(ctx context.Context, runID string, m Mismatch) {
	var eventType ledger.EventType
	switch m.Domain {
	case DomainBalance:
		eventType = EventBalanceMismatchDetected
	case DomainPosition:
		eventType = EventPositionMismatchDetected
	case DomainOrder:
		eventType = EventOrderMismatchDetected
	case DomainFill:
		eventType = EventFillMismatchDetected
	case DomainExposure:
		eventType = EventExposureMismatchDetected
	case DomainPnL:
		eventType = EventPnLMismatchDetected
	default:
		eventType = ledger.EventReconciliationMismatch
	}

	payload := MismatchPayload{
		RunID:         runID,
		Domain:        m.Domain,
		MismatchType:  m.Type,
		Exchange:      e.exchangeName,
		AccountID:     e.accountID,
		Severity:      m.Severity,
		Symbol:        m.Symbol,
		Reference:     m.Reference,
		ExchangeValue: m.ExchangeVal,
		InternalValue: m.InternalVal,
		DriftPct:      m.DriftPct,
		Message:       m.Message,
		DetectedAt:    m.DetectedAt,
	}

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", e.accountID, e.exchangeName),
		EventType:      eventType,
		AccountID:      e.accountID,
		Symbol:         m.Symbol,
		CorrelationID:  runID,
		IdempotencyKey: fmt.Sprintf("mismatch-%s", m.ID),
		Payload:        payload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		log.Printf("[RECON-V2] warn: build mismatch event: %v", err)
		return
	}
	if _, err := e.store.Append(ctx, ev); err != nil {
		log.Printf("[RECON-V2] warn: append mismatch event: %v", err)
	}
}

// emitAuthorityVerified logs (does not persist to the ledger — no consumer
// ever replays EventExchangeAuthorityVerified, and "everything matches" is
// the common case at 60s cadence; logging is sufficient operational signal).
func (e *ReconciliationEngine) emitAuthorityVerified(ctx context.Context, runID string, domain MismatchDomain) {
	_ = ctx
	log.Printf("[RECON-V2] authority verified: exchange=%s account=%s domain=%s run=%s",
		e.exchangeName, e.accountID, domain, runID)
}

// AuditLog returns the audit log for external inspection.
func (e *ReconciliationEngine) AuditLog() *AuditLog {
	return e.audit
}
