package ledger

import "context"

// DurableStore extends Store with production-grade durability operations.
//
// MemoryStore satisfies Store but NOT DurableStore (it is volatile).
// PostgresStore satisfies both interfaces.
//
// Usage:
//
//	var store ledger.Store = ledger.NewMemoryStore()         // dev / test
//	var store ledger.DurableStore                           // prod (PostgresStore)
type DurableStore interface {
	Store

	// AppendBatch appends multiple events in a single database transaction.
	// All events succeed or all fail atomically.
	AppendBatch(ctx context.Context, events []Event) ([]Event, error)

	// GetEventsAfter returns up to limit events whose global sequence number
	// is strictly greater than afterSequence, ordered by global sequence ascending.
	// Used for snapshot + delta replay on startup.
	GetEventsAfter(ctx context.Context, afterSequence int64, limit int) ([]Event, error)

	// GetEventsByType returns up to limit events matching eventType,
	// ordered by global sequence ascending.
	GetEventsByType(ctx context.Context, eventType EventType, limit int) ([]Event, error)

	// LastSequence returns the highest global sequence number persisted.
	// Returns 0 when the store is empty.
	LastSequence(ctx context.Context) (int64, error)

	// CreateSchema creates all necessary tables and indexes if they do not
	// already exist. Safe to call multiple times (idempotent).
	CreateSchema(ctx context.Context) error
}
