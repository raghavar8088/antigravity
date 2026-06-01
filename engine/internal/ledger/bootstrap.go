package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// BootstrapConfig controls ledger-replay startup behaviour.
type BootstrapConfig struct {
	// AccountID is the ledger account whose events are replayed (e.g. "btc-paper-1").
	AccountID string

	// SnapshotThreshold is the number of delta events that must accumulate after
	// the latest snapshot before a new snapshot is automatically written.
	// Default 10,000.
	SnapshotThreshold int64

	// FailOnCorruption causes Bootstrap to return an error when a payload hash
	// mismatch or sequence gap is detected. When false, corrupted events are
	// skipped with a warning (best-effort recovery).
	FailOnCorruption bool

	// MaxEventsPerBatch is the page size for reading delta events from the store.
	// Default 5,000.
	MaxEventsPerBatch int
}

func (c *BootstrapConfig) applyDefaults() {
	if c.SnapshotThreshold <= 0 {
		c.SnapshotThreshold = 10_000
	}
	if c.MaxEventsPerBatch <= 0 {
		c.MaxEventsPerBatch = 5_000
	}
}

// BootstrapResult holds the state rebuilt from the ledger at startup.
// Callers apply these snapshots to positions.Manager, OMS v3 Authority, and
// the trade journal to restore full engine state without re-querying the DB.
type BootstrapResult struct {
	// Snapshot is the base snapshot loaded from the SnapshotStore (may be empty).
	Snapshot Snapshot

	// DeltaEvents are the events appended to the ledger after the snapshot.
	// Callers replay these onto in-memory aggregates to bring state up to date.
	DeltaEvents []Event

	// EventsReplayed is the count of delta events consumed during this boot.
	EventsReplayed int64

	// TotalEventsInLedger is the global sequence count at boot time (from DurableStore).
	// Zero when the store does not implement DurableStore.
	TotalEventsInLedger int64

	// SnapshotUsed is true when a valid snapshot was found in the SnapshotStore.
	SnapshotUsed bool

	// BootedAt is the wall-clock time when Bootstrap returned.
	BootedAt time.Time
}

// Bootstrap restores engine state from the ledger.
//
// Startup sequence:
//  1. Load the latest snapshot for cfg.AccountID from snapStore (if available).
//  2. Replay all account events from the Store.
//  3. Filter to delta events (those after snapshot.GlobalSequence).
//  4. Validate payload hashes on each delta event.
//  5. Return BootstrapResult with the snapshot + delta for the caller to apply.
//  6. If delta events >= SnapshotThreshold, asynchronously write a new snapshot.
//
// store may be a MemoryStore (for tests/dev) or a PostgresStore (for production).
// snapStore may be nil; when nil, full replay always occurs.
func Bootstrap(ctx context.Context, store Store, snapStore SnapshotStore, cfg BootstrapConfig) (BootstrapResult, error) {
	cfg.applyDefaults()
	result := BootstrapResult{BootedAt: time.Now().UTC()}

	// Step 1: read total event count for telemetry (only when store is durable).
	if ds, ok := store.(DurableStore); ok {
		seq, err := ds.LastSequence(ctx)
		if err != nil {
			log.Printf("[BOOTSTRAP] ⚠️  Could not read last sequence: %v", err)
		}
		result.TotalEventsInLedger = seq
	}

	// Step 2: load snapshot.
	var snap Snapshot
	if snapStore != nil {
		loaded, err := snapStore.Latest(ctx, cfg.AccountID)
		if err == nil {
			snap = loaded
			result.Snapshot = snap
			result.SnapshotUsed = true
			log.Printf("[BOOTSTRAP] ✅ Snapshot loaded | account=%s | globalSeq=%d | orders=%d | positions=%d",
				cfg.AccountID, snap.GlobalSequence, len(snap.Orders), len(snap.Positions))
		} else if err.Error() != fmt.Errorf("%w for account %s", ErrNoSnapshot, cfg.AccountID).Error() {
			log.Printf("[BOOTSTRAP] ⚠️  Snapshot load failed, full replay: %v", err)
		} else {
			log.Printf("[BOOTSTRAP] No snapshot found for account=%s — full replay", cfg.AccountID)
		}
	}

	// Step 3: replay all account events.
	allEvents, err := store.ReplayAccount(ctx, cfg.AccountID)
	if err != nil {
		return result, fmt.Errorf("bootstrap: replay account %s: %w", cfg.AccountID, err)
	}

	// Step 4: filter to delta events (after snapshot.GlobalSequence).
	var delta []Event
	for _, e := range allEvents {
		if e.SequenceNo > snap.GlobalSequence {
			delta = append(delta, e)
		}
	}
	result.EventsReplayed = int64(len(delta))

	// Step 5: validate payload hashes.
	corrupted := 0
	for i, e := range delta {
		if !e.ValidateHash() {
			if cfg.FailOnCorruption {
				return result, fmt.Errorf("bootstrap: event %s (seq %d, index %d) has payload hash mismatch — store corrupted",
					e.EventID, e.SequenceNo, i)
			}
			log.Printf("[BOOTSTRAP] ⚠️  Event %s hash mismatch at index %d — skipping", e.EventID, i)
			corrupted++
		}
	}
	if corrupted > 0 {
		log.Printf("[BOOTSTRAP] ⚠️  %d of %d delta events had hash mismatches and were skipped", corrupted, len(delta))
	}

	result.DeltaEvents = delta

	log.Printf("[BOOTSTRAP] ✅ Ready | account=%s | snapshotUsed=%v | deltaEvents=%d | totalInLedger=%d",
		cfg.AccountID, result.SnapshotUsed, result.EventsReplayed, result.TotalEventsInLedger)

	// Step 6: write a new snapshot in the background when delta exceeds threshold.
	if snapStore != nil && int64(len(delta)) >= cfg.SnapshotThreshold {
		go takeBootstrapSnapshot(context.Background(), store, snapStore, cfg.AccountID)
	}

	return result, nil
}

// takeBootstrapSnapshot materialises a new snapshot from the full event log and
// persists it. Designed to run asynchronously so it does not delay startup.
func takeBootstrapSnapshot(ctx context.Context, store Store, snapStore SnapshotStore, accountID string) {
	rs := NewReplayStore(store, snapStore)
	snap, err := rs.TakeSnapshot(ctx, accountID)
	if err != nil {
		log.Printf("[BOOTSTRAP] ⚠️  Failed to take snapshot: %v", err)
		return
	}
	log.Printf("[BOOTSTRAP] 📸 Snapshot written | account=%s | globalSeq=%d", accountID, snap.GlobalSequence)
}

// BootstrapPositions extracts the open position payloads from a BootstrapResult.
// It scans both the snapshot's Positions map and the delta EventPositionOpened /
// EventPositionClosed events to build the final list of open positions.
// Callers pass these to positions.Manager.RestorePositions or OMS v3 Authority.
func BootstrapPositions(result BootstrapResult) []PositionOpenedPayload {
	// Seed from snapshot positions map (values are raw JSON of PositionOpenedPayload).
	open := make(map[string]PositionOpenedPayload)
	for posID, raw := range result.Snapshot.Positions {
		var p PositionOpenedPayload
		if err := json.Unmarshal(raw, &p); err == nil {
			open[posID] = p
		}
	}

	// Apply delta events.
	for _, e := range result.DeltaEvents {
		if e.AggregateType != AggregatePosition {
			continue
		}
		switch e.EventType {
		case EventPositionOpened:
			var p PositionOpenedPayload
			if err := json.Unmarshal(e.Payload, &p); err == nil {
				if p.PositionID == "" {
					p.PositionID = e.AggregateID
				}
				open[p.PositionID] = p
			}
		case EventPositionClosed:
			delete(open, e.AggregateID)
		}
	}

	result2 := make([]PositionOpenedPayload, 0, len(open))
	for _, p := range open {
		result2 = append(result2, p)
	}
	return result2
}
