package reconciliationv2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// ExchangeReconciliationAuthority is the top-level Phase 15E component.
//
// Authority hierarchy:
//   1. Exchange            ← single source of truth
//   2. ReconciliationAuthority  ← enforces exchange authority
//   3. OMS v3             ← must match exchange
//   4. Ledger             ← append-only record
//   5. Projections        ← derived from ledger
//   6. UI                 ← derived from projections
//
// One Authority manages one exchange's reconciliation. Create multiple Authority
// instances for multi-exchange deployments and run them concurrently.
type ExchangeReconciliationAuthority struct {
	engine    *ReconciliationEngine
	scheduler *ReconciliationScheduler
	metrics   *Metrics
	adapter   ReconciliationAdapter
	store     ledger.Store
	accountID string
	exchange  string

	mu      sync.RWMutex
	started bool
	stopFn  context.CancelFunc
}

// NewExchangeReconciliationAuthority constructs and wires the full Phase 15E stack.
//
//   adapter     — exchange-specific ReconciliationAdapter
//   omsReader   — OMS/ledger state reader (wraps omsv3.Authority in production)
//   store       — ledger event store
//   repairTarget — projection/aggregate rebuilder (wraps omsv3.ReplayAll in production)
//   accountID   — trading account identifier
//   cfg         — scheduler intervals (nil = DefaultScheduleConfig)
func NewExchangeReconciliationAuthority(
	adapter ReconciliationAdapter,
	omsReader OMSStateReader,
	store ledger.Store,
	repairTarget RepairTarget,
	accountID string,
	cfg *ScheduleConfig,
) *ExchangeReconciliationAuthority {
	if cfg == nil {
		def := DefaultScheduleConfig()
		cfg = &def
	}
	exchange := adapter.Name()
	metrics := NewMetrics(accountID)
	engine := NewReconciliationEngine(adapter, omsReader, store, repairTarget, metrics, accountID)
	scheduler := NewReconciliationScheduler(engine, metrics, *cfg)

	return &ExchangeReconciliationAuthority{
		engine:    engine,
		scheduler: scheduler,
		metrics:   metrics,
		adapter:   adapter,
		store:     store,
		accountID: accountID,
		exchange:  exchange,
	}
}

// Start launches the reconciliation authority and all scheduled cycles.
// Returns immediately; reconciliation runs in background goroutines.
// Call Stop() or cancel the parent context to shut down.
func (a *ExchangeReconciliationAuthority) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.started {
		return fmt.Errorf("reconciliation authority for %s already started", a.exchange)
	}

	// Health-check the adapter before starting.
	if err := a.adapter.HealthCheck(ctx); err != nil {
		log.Printf("[RECON-V2] WARNING: exchange %s health check failed: %v — starting anyway", a.exchange, err)
		a.metrics.SetExchangeHealth(a.exchange, false)
	} else {
		a.metrics.SetExchangeHealth(a.exchange, true)
		log.Printf("[RECON-V2] exchange %s health check OK", a.exchange)
	}

	authCtx, cancel := context.WithCancel(ctx)
	a.stopFn = cancel
	a.started = true

	go func() {
		log.Printf("[RECON-V2] starting exchange reconciliation authority for %s account=%s", a.exchange, a.accountID)
		a.scheduler.Start(authCtx)
	}()

	return nil
}

// Stop gracefully shuts down all reconciliation goroutines.
func (a *ExchangeReconciliationAuthority) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopFn != nil {
		a.stopFn()
		a.started = false
		log.Printf("[RECON-V2] authority stopped for %s", a.exchange)
	}
}

// RunNow triggers an immediate full-domain reconciliation cycle outside the scheduler.
// Useful for post-crash validation and on-demand checks.
func (a *ExchangeReconciliationAuthority) RunNow(ctx context.Context) (AuditEntry, error) {
	return a.engine.RunDomain(ctx, DomainFull)
}

// RunDomain triggers an immediate reconciliation cycle for a specific domain.
func (a *ExchangeReconciliationAuthority) RunDomain(ctx context.Context, domain MismatchDomain) (AuditEntry, error) {
	return a.engine.RunDomain(ctx, domain)
}

// SetCycleHook registers a post-cycle callback on the background scheduler.
func (a *ExchangeReconciliationAuthority) SetCycleHook(hook CycleHook) {
	if a.scheduler != nil {
		a.scheduler.SetCycleHook(hook)
	}
}

// Status returns the current operational status of the authority.
func (a *ExchangeReconciliationAuthority) Status() AuthorityStatus {
	a.mu.RLock()
	running := a.started
	a.mu.RUnlock()

	audit := a.engine.AuditLog()

	return AuthorityStatus{
		Exchange:      a.exchange,
		AccountID:     a.accountID,
		Running:       running,
		TotalCycles:   audit.TotalCycles(),
		LastDriftScore: audit.LastDriftScore(),
		CriticalCount: audit.CriticalCount(),
		RecentCycles:  audit.Recent(5),
	}
}

// AuthorityStatus is a snapshot of reconciliation authority health.
type AuthorityStatus struct {
	Exchange       string
	AccountID      string
	Running        bool
	TotalCycles    int
	LastDriftScore float64
	CriticalCount  int
	RecentCycles   []AuditEntry
}

// ValidateCrashRecovery verifies that after a crash and replay, exchange state
// matches internal state. Returns all detected mismatches.
func (a *ExchangeReconciliationAuthority) ValidateCrashRecovery(ctx context.Context) ([]Mismatch, error) {
	log.Printf("[RECON-V2] crash recovery validation started for %s", a.exchange)
	entry, err := a.engine.RunDomain(ctx, DomainFull)
	if err != nil {
		return nil, fmt.Errorf("crash recovery validation: %w", err)
	}
	log.Printf("[RECON-V2] crash recovery validation complete: mismatches=%d drift=%.2f", entry.MismatchCount, entry.DriftScore)
	return entry.Mismatches, nil
}

// ─── Package-level helpers ────────────────────────────────────────────────────

// newRunID generates a unique reconciliation run identifier.
func newRunID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return "run-" + hex.EncodeToString(b[:])
}

// newMismatchID generates a unique mismatch identifier.
func newMismatchID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("mismatch-%d", time.Now().UnixNano())
	}
	return "mm-" + hex.EncodeToString(b[:])
}
