package reconciliationv2

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// AuditEntry records a single reconciliation cycle outcome for the audit trail.
type AuditEntry struct {
	RunID         string
	Domain        MismatchDomain
	Exchange      string
	AccountID     string
	MismatchCount int
	RepairCount   int
	EscalateCount int
	DriftScore    float64
	Mismatches    []Mismatch
	RepairResults []RepairResult
	StartedAt     time.Time
	CompletedAt   time.Time
	DurationMs    int64
}

// AuditLog maintains an in-memory audit trail of reconciliation cycles and
// persists each cycle as a completed event in the ledger for replay compatibility.
type AuditLog struct {
	mu        sync.RWMutex
	entries   []AuditEntry
	maxLen    int
	store     ledger.Store
	accountID string
	exchange  string
}

// NewAuditLog creates an AuditLog that retains the last maxEntries cycles.
func NewAuditLog(store ledger.Store, accountID, exchange string, maxEntries int) *AuditLog {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &AuditLog{
		store:     store,
		accountID: accountID,
		exchange:  exchange,
		maxLen:    maxEntries,
	}
}

// Record appends an audit entry to the bounded in-memory history (always —
// this is what Recent/TotalCycles/LastDriftScore/CriticalCount read from)
// and persists a RECONCILIATION_COMPLETED event to the durable ledger only
// for cycles that found something worth a permanent record. The overwhelming
// majority of cycles are "all clear," and persisting every single one to the
// shared ledger (every 2-60s, forever) was the dominant contributor to an
// unbounded event store that leaked memory until the engine got OOM-killed
// roughly every 24-36 hours (Jun 2026 incident).
func (a *AuditLog) Record(ctx context.Context, entry AuditEntry) {
	a.mu.Lock()
	a.entries = append(a.entries, entry)
	if len(a.entries) > a.maxLen {
		a.entries = a.entries[len(a.entries)-a.maxLen:]
	}
	a.mu.Unlock()

	if entry.MismatchCount > 0 || entry.RepairCount > 0 || entry.EscalateCount > 0 {
		a.persistCompletedEvent(ctx, entry)
	}
}

// Recent returns the most recent n audit entries (newest first).
func (a *AuditLog) Recent(n int) []AuditEntry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	total := len(a.entries)
	if n > total {
		n = total
	}
	out := make([]AuditEntry, n)
	for i := 0; i < n; i++ {
		out[i] = a.entries[total-n+i]
	}
	return out
}

// TotalCycles returns the total number of reconciliation cycles recorded.
func (a *AuditLog) TotalCycles() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.entries)
}

// LastDriftScore returns the drift score from the most recent cycle, or 0.
func (a *AuditLog) LastDriftScore() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.entries) == 0 {
		return 0
	}
	return a.entries[len(a.entries)-1].DriftScore
}

// CriticalCount returns the number of CRITICAL mismatches in the most recent cycle.
func (a *AuditLog) CriticalCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if len(a.entries) == 0 {
		return 0
	}
	last := a.entries[len(a.entries)-1]
	count := 0
	for _, m := range last.Mismatches {
		if m.Severity == SeverityCritical {
			count++
		}
	}
	return count
}

func (a *AuditLog) persistCompletedEvent(ctx context.Context, entry AuditEntry) {
	payload := ReconciliationCompletedPayload{
		RunID:         entry.RunID,
		Domain:        entry.Domain,
		Exchange:      entry.Exchange,
		AccountID:     entry.AccountID,
		MismatchCount: entry.MismatchCount,
		RepairCount:   entry.RepairCount,
		DriftScore:    entry.DriftScore,
		DurationMs:    entry.DurationMs,
		CompletedAt:   entry.CompletedAt,
	}

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType:  ledger.AggregateReconciliation,
		AggregateID:    fmt.Sprintf("recon-%s-%s", a.accountID, a.exchange),
		EventType:      EventReconciliationCompleted,
		AccountID:      a.accountID,
		CorrelationID:  entry.RunID,
		IdempotencyKey: fmt.Sprintf("recon-completed-%s", entry.RunID),
		Payload:        payload,
		Source:         "reconciliation-authority-v2",
	})
	if err != nil {
		log.Printf("[RECON-V2] audit: build completed event: %v", err)
		return
	}
	if _, err := a.store.Append(ctx, ev); err != nil {
		log.Printf("[RECON-V2] audit: append completed event: %v", err)
	}
}
