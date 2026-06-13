package eventstore

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"antigravity-engine/internal/events"
)

const maxReplayEvents = 10000

// EventReader replays events from the TimescaleDB event_store table.
type EventReader struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewEventReader creates a reader backed by pool.
func NewEventReader(pool *pgxpool.Pool, logger *slog.Logger) *EventReader {
	return &EventReader{pool: pool, logger: logger}
}

// ReadSince returns up to 10 000 events after since filtered by eventTypes.
// Pass nil for eventTypes to retrieve all event types.
func (r *EventReader) ReadSince(ctx context.Context, since time.Time, eventTypes []string) ([]RawEvent, error) {
	var rows []RawEvent

	query := `SELECT event_id, event_type, source, payload, occurred_at
	          FROM event_store
	          WHERE occurred_at > $1
	            AND ($2::text[] IS NULL OR event_type = ANY($2))
	          ORDER BY occurred_at ASC
	          LIMIT $3`

	var typesArg interface{} = nil
	if len(eventTypes) > 0 {
		typesArg = eventTypes
	}

	pgRows, err := r.pool.Query(ctx, query, since, typesArg, maxReplayEvents)
	if err != nil {
		return nil, fmt.Errorf("eventstore ReadSince: %w", err)
	}
	defer pgRows.Close()

	for pgRows.Next() {
		var ev RawEvent
		if err := pgRows.Scan(&ev.EventID, &ev.EventType, &ev.Source, &ev.Payload, &ev.OccurredAt); err != nil {
			r.logger.Warn("[eventstore] scan error", "error", err)
			continue
		}
		rows = append(rows, ev)
	}
	return rows, pgRows.Err()
}

// ReplayToState applies events chronologically to rebuild engine state.
// Unknown event types are skipped to ensure forward compatibility.
func (r *EventReader) ReplayToState(ctx context.Context, since time.Time) (*ReplayedState, error) {
	rawEvents, err := r.ReadSince(ctx, since, nil)
	if err != nil {
		return nil, fmt.Errorf("eventstore ReplayToState: %w", err)
	}

	state := &ReplayedState{
		OpenPositions: make(map[string]interface{}),
		Since:         since,
	}

	for _, raw := range rawEvents {
		state.EventsReplayed++
		switch raw.EventType {
		case "PositionOpenedEvent":
			var ev events.PositionOpenedEvent
			if err := json.Unmarshal(raw.Payload, &ev); err != nil {
				r.logger.Debug("[eventstore] replay: unmarshal PositionOpenedEvent", "error", err)
				continue
			}
			state.OpenPositions[ev.PositionID] = ev

		case "PositionClosedEvent":
			var ev events.PositionClosedEvent
			if err := json.Unmarshal(raw.Payload, &ev); err != nil {
				r.logger.Debug("[eventstore] replay: unmarshal PositionClosedEvent", "error", err)
				continue
			}
			delete(state.OpenPositions, ev.PositionID)
			state.ClosedTrades = append(state.ClosedTrades, ev)

		case "LedgerUpdatedEvent":
			var ev events.LedgerUpdatedEvent
			if err := json.Unmarshal(raw.Payload, &ev); err != nil {
				r.logger.Debug("[eventstore] replay: unmarshal LedgerUpdatedEvent", "error", err)
				continue
			}
			// LedgerUpdatedEvent signals a ledger write; balance is authoritative
			// from MongoDB. We track it here for validation purposes only.

		default:
			r.logger.Debug("[eventstore] replay: unknown event type — skipping", "type", raw.EventType)
		}
	}

	return state, nil
}
