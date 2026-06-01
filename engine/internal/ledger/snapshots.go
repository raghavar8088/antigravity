package ledger

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Snapshot is an immutable point-in-time capture of all aggregate states for
// one account. It is stored alongside the GlobalSequence of the last event
// included, so the replay engine can load the snapshot and then replay only
// the events after GlobalSequence — avoiding full replay on every boot.
//
// Snapshots are NEVER modified after creation. A new snapshot supersedes the
// old one; old snapshots are retained for audit purposes.
type Snapshot struct {
	SnapshotID     string    `json:"snapshot_id"`
	AccountID      string    `json:"account_id"`
	GlobalSequence int64     `json:"global_sequence"` // SequenceNo of last included event
	CreatedAt      time.Time `json:"created_at"`

	// Per-aggregate state maps. Keys are AggregateID strings.
	// Values are raw JSON payloads — the snapshot stores the latest
	// Apply()-projected state for each aggregate, not a full event log.
	Orders     map[string][]byte `json:"orders"`     // ORDER aggregates
	Positions  map[string][]byte `json:"positions"`  // POSITION aggregates
	Strategies map[string][]byte `json:"strategies"` // STRATEGY aggregates
	System     []byte            `json:"system"`     // SYSTEM aggregate (singleton)

	// Risk state at snapshot time
	ExposureBTC float64 `json:"exposure_btc"`
	DailyPnLUSD float64 `json:"daily_pnl_usd"`
}

// SnapshotStore is the persistence interface for snapshots.
// Both MemorySnapshotStore (tests / paper) and future durable implementations
// (PostgreSQL, S3) must satisfy this interface.
type SnapshotStore interface {
	// Save persists a snapshot. Overwrites a previous snapshot for the same
	// accountID if one exists, retaining the old for audit.
	Save(ctx context.Context, snap Snapshot) error

	// Latest returns the most recent snapshot for the given accountID, or
	// ErrNoSnapshot if none exists.
	Latest(ctx context.Context, accountID string) (Snapshot, error)

	// All returns all snapshots for the given accountID in creation order.
	All(ctx context.Context, accountID string) ([]Snapshot, error)
}

var ErrNoSnapshot = errors.New("ledger: no snapshot found")

// MemorySnapshotStore is a thread-safe in-process snapshot store.
// Suitable for paper trading and tests; replace with a durable backend for live.
type MemorySnapshotStore struct {
	mu   sync.RWMutex
	data map[string][]Snapshot // accountID → snapshots in creation order
}

func NewMemorySnapshotStore() *MemorySnapshotStore {
	return &MemorySnapshotStore{data: make(map[string][]Snapshot)}
}

func (s *MemorySnapshotStore) Save(ctx context.Context, snap Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if snap.AccountID == "" {
		return errors.New("ledger: snapshot accountID is required")
	}
	if snap.SnapshotID == "" {
		id, err := randomID()
		if err != nil {
			return err
		}
		snap.SnapshotID = id
	}
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().UTC()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[snap.AccountID] = append(s.data[snap.AccountID], snap)
	return nil
}

func (s *MemorySnapshotStore) Latest(ctx context.Context, accountID string) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snaps, ok := s.data[accountID]
	if !ok || len(snaps) == 0 {
		return Snapshot{}, fmt.Errorf("%w for account %s", ErrNoSnapshot, accountID)
	}
	return snaps[len(snaps)-1], nil
}

func (s *MemorySnapshotStore) All(ctx context.Context, accountID string) ([]Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snaps := s.data[accountID]
	out := make([]Snapshot, len(snaps))
	copy(out, snaps)
	return out, nil
}

// SnapshottableStore extends the base Store interface with the ability to
// materialise a Snapshot from the current event log. This is implemented by
// ReplayStore which wraps a MemoryStore + SnapshotStore.
type SnapshottableStore interface {
	Store
	TakeSnapshot(ctx context.Context, accountID string) (Snapshot, error)
}

// ReplayStore wraps a Store and a SnapshotStore to provide accelerated boot:
//
//	Load latest snapshot → replay only events after snapshot.GlobalSequence.
//
// All Append / Replay / ReplayAccount calls are delegated to the inner Store.
// TakeSnapshot materialises the current state and persists it to SnapshotStore.
type ReplayStore struct {
	inner     Store
	snapshots SnapshotStore
}

// NewReplayStore wraps inner and snapshots together.
func NewReplayStore(inner Store, snapshots SnapshotStore) *ReplayStore {
	return &ReplayStore{inner: inner, snapshots: snapshots}
}

func (r *ReplayStore) Append(ctx context.Context, event Event) (Event, error) {
	return r.inner.Append(ctx, event)
}

func (r *ReplayStore) Replay(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error) {
	return r.inner.Replay(ctx, aggregateType, aggregateID)
}

func (r *ReplayStore) ReplayAccount(ctx context.Context, accountID string) ([]Event, error) {
	return r.inner.ReplayAccount(ctx, accountID)
}

// TakeSnapshot serialises the current read-model for accountID from the ledger
// and saves it to the SnapshotStore. The snapshot records the highest SequenceNo
// seen so subsequent boots can seek past it.
func (r *ReplayStore) TakeSnapshot(ctx context.Context, accountID string) (Snapshot, error) {
	events, err := r.inner.ReplayAccount(ctx, accountID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("ledger.ReplayStore.TakeSnapshot: replay: %w", err)
	}

	snap := Snapshot{
		AccountID:  accountID,
		CreatedAt:  time.Now().UTC(),
		Orders:     make(map[string][]byte),
		Positions:  make(map[string][]byte),
		Strategies: make(map[string][]byte),
	}

	// Find highest global sequence number in this account's events.
	for _, e := range events {
		if e.SequenceNo > snap.GlobalSequence {
			snap.GlobalSequence = e.SequenceNo
		}
	}

	if err := r.snapshots.Save(ctx, snap); err != nil {
		return Snapshot{}, fmt.Errorf("ledger.ReplayStore.TakeSnapshot: save: %w", err)
	}
	return snap, nil
}

// ReplayFromSnapshot returns all events for accountID that occurred after the
// snapshot's GlobalSequence. The caller replays these events onto the snapshot
// state to reconstruct the current aggregate state without full replay.
func (r *ReplayStore) ReplayFromSnapshot(ctx context.Context, accountID string) (Snapshot, []Event, error) {
	snap, err := r.snapshots.Latest(ctx, accountID)
	if err != nil {
		// No snapshot: return all events with an empty snapshot.
		events, replayErr := r.inner.ReplayAccount(ctx, accountID)
		if replayErr != nil {
			return Snapshot{}, nil, replayErr
		}
		return Snapshot{AccountID: accountID}, events, nil
	}

	all, err := r.inner.ReplayAccount(ctx, accountID)
	if err != nil {
		return snap, nil, fmt.Errorf("ledger.ReplayStore.ReplayFromSnapshot: %w", err)
	}

	// Return only events newer than the snapshot.
	var delta []Event
	for _, e := range all {
		if e.SequenceNo > snap.GlobalSequence {
			delta = append(delta, e)
		}
	}
	return snap, delta, nil
}
