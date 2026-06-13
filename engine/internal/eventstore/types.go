// Package eventstore provides dual-write event persistence to PostgreSQL/TimescaleDB
// for audit trails and crash-recovery replay. All writes are non-blocking — trading
// continues even when the event store is unavailable.
package eventstore

import "time"

// RawEvent is one persisted event row in the event_store table.
type RawEvent struct {
	EventID    string    `db:"event_id"`
	EventType  string    `db:"event_type"`
	Source     string    `db:"source"`
	Payload    []byte    `db:"payload"` // JSONB
	OccurredAt time.Time `db:"occurred_at"`
}

// ReplayedState is the engine state rebuilt from replaying all events since a point in time.
type ReplayedState struct {
	OpenPositions    map[string]interface{}
	PortfolioBalance float64
	ClosedTrades     []interface{}
	EventsReplayed   int
	Since            time.Time
}

// ValidationReport compares live MongoDB state against the replayed event-store state.
type ValidationReport struct {
	Matches               bool
	LivePositionCount     int
	ReplayedPositionCount int
	Discrepancies         []string
	LiveBalance           float64
	ReplayedBalance       float64
	ValidationDuration    time.Duration
}
