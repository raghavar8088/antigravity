package omsv3

import (
	"encoding/json"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// SystemState tracks the engine's operational lifecycle.
type SystemState string

const (
	SystemStateUnknown  SystemState = ""
	SystemStateStarting SystemState = "STARTING"
	SystemStateRunning  SystemState = "RUNNING"
	SystemStateReplaying SystemState = "REPLAYING"
	SystemStateStopped  SystemState = "STOPPED"
)

var systemTransitions = map[SystemState][]SystemState{
	SystemStateUnknown:   {SystemStateStarting, SystemStateRunning},
	SystemStateStarting:  {SystemStateRunning, SystemStateReplaying, SystemStateStopped},
	SystemStateRunning:   {SystemStateReplaying, SystemStateStopped},
	SystemStateReplaying: {SystemStateRunning, SystemStateStopped},
	SystemStateStopped:   {SystemStateStarting},
}

// SystemAggregate is a singleton per account that owns the engine lifecycle.
// It records engine boots, stops, replays, snapshot creation/restore, and
// projection rebuilds so the audit trail includes every operational event.
type SystemAggregate struct {
	AccountID   string
	Version     string // engine version tag
	StartedAt   time.Time
	StoppedAt   time.Time
	LastReplayAt time.Time
	LastSnapshotID string
	TotalReplays   int
	TotalSnapshots int

	State   SystemState
	SeqNo   int64
	Events  []ledger.Event
}

// NewSystemAggregate creates a system aggregate for the given account.
func NewSystemAggregate(accountID string) *SystemAggregate {
	return &SystemAggregate{AccountID: accountID}
}

// ValidateSystemTransition checks whether transitioning to next is allowed.
func (s *SystemAggregate) ValidateSystemTransition(next SystemState) error {
	for _, allowed := range systemTransitions[s.State] {
		if allowed == next {
			return nil
		}
	}
	return fmt.Errorf("%w: system %s -> %s", ErrInvalidTransition, s.State, next)
}

// ApplyEvent applies one SYSTEM event to the system aggregate.
func (s *SystemAggregate) ApplyEvent(event ledger.Event) error {
	if event.AggregateType != ledger.AggregateSystem {
		return fmt.Errorf("omsv3: expected SYSTEM event, got %s", event.AggregateType)
	}

	var payload ledger.SystemLifecyclePayload
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}

	switch event.EventType {
	case ledger.EventEngineStarted:
		if err := s.ValidateSystemTransition(SystemStateRunning); err != nil {
			// Tolerate duplicate starts (idempotent boot).
			if s.State != SystemStateRunning {
				return err
			}
		}
		s.State = SystemStateRunning
		s.StartedAt = event.CreatedAt
		if payload.Version != "" {
			s.Version = payload.Version
		}

	case ledger.EventEngineStopped:
		if err := s.ValidateSystemTransition(SystemStateStopped); err != nil {
			return err
		}
		s.State = SystemStateStopped
		s.StoppedAt = event.CreatedAt

	case ledger.EventReplayStarted:
		if err := s.ValidateSystemTransition(SystemStateReplaying); err != nil {
			return err
		}
		s.State = SystemStateReplaying
		s.LastReplayAt = event.CreatedAt
		s.TotalReplays++

	case ledger.EventReplayCompleted:
		if err := s.ValidateSystemTransition(SystemStateRunning); err != nil {
			return err
		}
		s.State = SystemStateRunning

	case ledger.EventSnapshotCreated:
		s.TotalSnapshots++
		if payload.SnapshotID != "" {
			s.LastSnapshotID = payload.SnapshotID
		}

	case ledger.EventSnapshotRestored:
		if payload.SnapshotID != "" {
			s.LastSnapshotID = payload.SnapshotID
		}

	case ledger.EventProjectionRebuilt:
		// No state change; purely informational.

	default:
		return fmt.Errorf("omsv3: unsupported system event %s", event.EventType)
	}

	s.SeqNo = event.SequenceNo
	s.Events = append(s.Events, event)
	return nil
}

// ReplaySystemAggregate rebuilds the system aggregate from a slice of events.
func ReplaySystemAggregate(events []ledger.Event) (*SystemAggregate, error) {
	if len(events) == 0 {
		return &SystemAggregate{}, nil
	}
	agg := NewSystemAggregate(events[0].AccountID)
	for _, event := range events {
		if err := agg.ApplyEvent(event); err != nil {
			return nil, fmt.Errorf("ReplaySystemAggregate: %w", err)
		}
	}
	return agg, nil
}
