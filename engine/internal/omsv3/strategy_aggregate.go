package omsv3

import (
	"encoding/json"
	"fmt"

	"antigravity-engine/internal/ledger"
)

// StrategyState is the operational lifecycle of a single strategy.
type StrategyState string

const (
	StrategyStateEmpty    StrategyState = ""
	StrategyStateEnabled  StrategyState = "ENABLED"
	StrategyStateDisabled StrategyState = "DISABLED"
	StrategyStatePaused   StrategyState = "PAUSED"
	StrategyStateResumed  StrategyState = "RESUMED"   // alias for ENABLED after pause
	StrategyStatePromoted StrategyState = "PROMOTED"  // moved to live / higher allocation
	StrategyStateDemoted  StrategyState = "DEMOTED"   // reduced allocation / watchlist
)

var strategyTransitions = map[StrategyState][]StrategyState{
	StrategyStateEmpty:    {StrategyStateEnabled, StrategyStateDisabled},
	StrategyStateEnabled:  {StrategyStatePaused, StrategyStateDisabled, StrategyStatePromoted, StrategyStateDemoted},
	StrategyStateResumed:  {StrategyStatePaused, StrategyStateDisabled, StrategyStatePromoted, StrategyStateDemoted},
	StrategyStatePaused:   {StrategyStateResumed, StrategyStateDisabled},
	StrategyStatePromoted: {StrategyStatePaused, StrategyStateDisabled, StrategyStateDemoted},
	StrategyStateDemoted:  {StrategyStateEnabled, StrategyStateResumed, StrategyStateDisabled},
	StrategyStateDisabled: {StrategyStateEnabled}, // can be re-enabled after investigation
}

// StrategyAggregate owns the lifecycle of a single strategy.
// All state mutations flow exclusively through ApplyEvent; no direct field
// assignment is permitted outside of Apply.
type StrategyAggregate struct {
	StrategyID   string
	StrategyName string
	Family       string
	Module       string
	Allocation   float64 // USD notional or % of equity
	WinRate      float64
	ProfitFactor float64
	DisableReason string

	State   StrategyState
	Version int64
	Events  []ledger.Event
}

// NewStrategyAggregate creates an empty strategy aggregate for the given strategy ID.
func NewStrategyAggregate(strategyID string) *StrategyAggregate {
	return &StrategyAggregate{StrategyID: strategyID}
}

// CurrentState returns the strategy's current lifecycle state.
func (s *StrategyAggregate) CurrentState() StrategyState {
	if s == nil {
		return StrategyStateEmpty
	}
	return s.State
}

// IsActive returns true when the strategy should generate and execute signals.
func (s *StrategyAggregate) IsActive() bool {
	return s.State == StrategyStateEnabled ||
		s.State == StrategyStateResumed ||
		s.State == StrategyStatePromoted
}

// ValidateStrategyTransition checks whether transitioning to next is allowed.
func (s *StrategyAggregate) ValidateStrategyTransition(next StrategyState) error {
	if s == nil {
		return fmt.Errorf("%w: nil strategy aggregate", ErrInvalidTransition)
	}
	for _, allowed := range strategyTransitions[s.State] {
		if allowed == next {
			return nil
		}
	}
	return fmt.Errorf("%w: strategy %s %s -> %s", ErrInvalidTransition, s.StrategyID, s.State, next)
}

// ApplyEvent applies one ledger STRATEGY event to the aggregate.
func (s *StrategyAggregate) ApplyEvent(event ledger.Event) error {
	if event.AggregateType != ledger.AggregateStrategy {
		return fmt.Errorf("omsv3: expected STRATEGY event, got %s", event.AggregateType)
	}
	next, ok := strategyStateFromEvent(event.EventType)
	if !ok {
		return fmt.Errorf("omsv3: unsupported strategy event %s", event.EventType)
	}
	if err := s.ValidateStrategyTransition(next); err != nil {
		return err
	}

	var payload ledger.StrategyLifecyclePayload
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}

	s.StrategyName = firstNonEmpty(payload.StrategyName, s.StrategyName)
	s.Family = firstNonEmpty(payload.Family, s.Family)
	s.Module = firstNonEmpty(payload.Module, s.Module)
	if payload.NewAllocation > 0 {
		s.Allocation = payload.NewAllocation
	}
	if payload.WinRate > 0 {
		s.WinRate = payload.WinRate
	}
	if payload.ProfitFactor > 0 {
		s.ProfitFactor = payload.ProfitFactor
	}
	if next == StrategyStateDisabled && payload.Reason != "" {
		s.DisableReason = payload.Reason
	}

	s.State = next
	s.Version = event.SequenceNo
	s.Events = append(s.Events, event)
	return nil
}

// ReplayStrategy rebuilds a StrategyAggregate by replaying events.
func ReplayStrategy(events []ledger.Event) (*StrategyAggregate, error) {
	if len(events) == 0 {
		return &StrategyAggregate{}, nil
	}
	agg := NewStrategyAggregate(events[0].AggregateID)
	for _, event := range events {
		if err := agg.ApplyEvent(event); err != nil {
			return nil, fmt.Errorf("ReplayStrategy %s: %w", events[0].AggregateID, err)
		}
	}
	return agg, nil
}

func strategyStateFromEvent(et ledger.EventType) (StrategyState, bool) {
	switch et {
	case ledger.EventStrategyEnabled:
		return StrategyStateEnabled, true
	case ledger.EventStrategyDisabled:
		return StrategyStateDisabled, true
	case ledger.EventStrategyPaused:
		return StrategyStatePaused, true
	case ledger.EventStrategyResumed:
		return StrategyStateResumed, true
	case ledger.EventStrategyPromoted:
		return StrategyStatePromoted, true
	case ledger.EventStrategyDemoted:
		return StrategyStateDemoted, true
	case ledger.EventStrategyAllocationChanged:
		// Allocation change doesn't change state; we reuse current state.
		// Return "" to signal a non-transitioning event.
		return StrategyStateEmpty, false
	default:
		return StrategyStateEmpty, false
	}
}

// ApplyAllocationChange updates strategy allocation without a state transition.
// Emits EventStrategyAllocationChanged (caller must append it to the ledger).
func (s *StrategyAggregate) ApplyAllocationChange(event ledger.Event) error {
	if event.EventType != ledger.EventStrategyAllocationChanged {
		return fmt.Errorf("omsv3: expected STRATEGY_ALLOCATION_CHANGED, got %s", event.EventType)
	}
	var payload ledger.StrategyLifecyclePayload
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}
	if payload.NewAllocation > 0 {
		s.Allocation = payload.NewAllocation
	}
	s.Version = event.SequenceNo
	s.Events = append(s.Events, event)
	return nil
}
