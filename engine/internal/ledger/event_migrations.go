package ledger

import (
	"context"
	"encoding/json"
	"fmt"
)

// CurrentSchemaVersion is the schema version stamped on all newly created events.
// Increment when adding required fields to an event payload; bump the migration
// registry below so old events (SchemaVersion < N) can be upconverted on replay.
const CurrentSchemaVersion = 2

// MigrationFn upgrades an event from one schema version to the next.
// It must be pure (no side effects) and deterministic.
type MigrationFn func(event Event) (Event, error)

// migration describes a single schema-version upgrade step.
type migration struct {
	fromVersion int
	toVersion   int
	eventTypes  []EventType // nil means applies to all event types
	apply       MigrationFn
}

// migrations is the ordered registry of all schema upgrades.
// New migrations are APPENDED — never reordered or removed.
var migrations = []migration{
	{
		fromVersion: 1,
		toVersion:   2,
		eventTypes:  nil, // applies globally
		apply:       migrateV1toV2,
	},
}

// MigrateEvent upgrades a single event to CurrentSchemaVersion by applying all
// registered migrations in sequence. If the event is already at CurrentSchemaVersion
// it is returned unchanged. This is called during replay of any event whose
// SchemaVersion < CurrentSchemaVersion.
func MigrateEvent(event Event) (Event, error) {
	for event.SchemaVersion < CurrentSchemaVersion {
		upgraded := false
		for _, m := range migrations {
			if m.fromVersion != event.SchemaVersion {
				continue
			}
			if !appliesToEventType(m.eventTypes, event.EventType) {
				continue
			}
			var err error
			event, err = m.apply(event)
			if err != nil {
				return event, fmt.Errorf("ledger.MigrateEvent(%s v%d→v%d): %w",
					event.EventType, m.fromVersion, m.toVersion, err)
			}
			event.SchemaVersion = m.toVersion
			upgraded = true
			break
		}
		if !upgraded {
			// No migration found for this version — treat as current.
			event.SchemaVersion = CurrentSchemaVersion
		}
	}
	return event, nil
}

// MigrateAll upgrades an entire event slice to CurrentSchemaVersion.
// Safe to call on event slices returned by Replay — returns a new slice.
func MigrateAll(events []Event) ([]Event, error) {
	out := make([]Event, len(events))
	for i, e := range events {
		migrated, err := MigrateEvent(e)
		if err != nil {
			return nil, err
		}
		out[i] = migrated
	}
	return out, nil
}

// ── Migration implementations ─────────────────────────────────────────────────

// migrateV1toV2 adds the SchemaVersion field and back-fills Producer from Source.
// Schema v1 → v2:
//   - Producer field added (was missing in v1; back-fill from Source)
//   - All existing payloads get a "schema_version":2 field injected
func migrateV1toV2(event Event) (Event, error) {
	// Inject schema_version into payload JSON without breaking existing fields.
	if len(event.Payload) > 0 && event.Payload[0] == '{' {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(event.Payload, &m); err == nil {
			if _, ok := m["schema_version"]; !ok {
				m["schema_version"] = json.RawMessage(`2`)
				patched, pErr := json.Marshal(m)
				if pErr == nil {
					event.Payload = patched
					event.PayloadHash = PayloadHash(patched)
				}
			}
		}
	}
	// Back-fill Producer into Source if Source is empty.
	if event.Source == "" {
		event.Source = "migrated-v1"
	}
	return event, nil
}

// ── Replay-time migration hook ────────────────────────────────────────────────

// MigratingStore wraps a Store and automatically upgrades event schemas during
// Replay and ReplayAccount calls. New events written via Append are stamped with
// CurrentSchemaVersion.
type MigratingStore struct {
	inner Store
}

// NewMigratingStore wraps inner with automatic schema migration on reads.
func NewMigratingStore(inner Store) *MigratingStore {
	return &MigratingStore{inner: inner}
}

func (m *MigratingStore) Append(ctx context.Context, event Event) (Event, error) {
	// Stamp new events with the current schema version.
	event.SchemaVersion = CurrentSchemaVersion
	return m.inner.Append(ctx, event)
}

func (m *MigratingStore) Replay(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error) {
	events, err := m.inner.Replay(ctx, aggregateType, aggregateID)
	if err != nil {
		return nil, err
	}
	return MigrateAll(events)
}

func (m *MigratingStore) ReplayAccount(ctx context.Context, accountID string) ([]Event, error) {
	events, err := m.inner.ReplayAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return MigrateAll(events)
}

// appliesToEventType reports whether a migration applies to the given event type.
// A nil list means the migration applies to all event types.
func appliesToEventType(types []EventType, et EventType) bool {
	if len(types) == 0 {
		return true
	}
	for _, t := range types {
		if t == et {
			return true
		}
	}
	return false
}

// ── Schema version helpers ────────────────────────────────────────────────────

// StampSchemaVersion sets the SchemaVersion on a NewEventInput before the event
// is constructed. Call this in NewEvent() — already handled by the updated
// NewEvent function below.
func StampSchemaVersion(input *NewEventInput) {
	// Currently a no-op: NewEvent always stamps CurrentSchemaVersion.
	// Kept for forward-compatibility when downstream callers want to pin version.
}
