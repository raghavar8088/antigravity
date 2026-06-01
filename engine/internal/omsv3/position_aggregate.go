package omsv3

import (
	"encoding/json"
	"fmt"

	"antigravity-engine/internal/ledger"
)

// PositionState is the lifecycle state of a position aggregate.
type PositionState string

const (
	PositionStateEmpty   PositionState = ""
	PositionStateOpen    PositionState = "OPEN"
	PositionStateReduced PositionState = "REDUCED" // partial close
	PositionStateClosed  PositionState = "CLOSED"
)

var positionTransitions = map[PositionState][]PositionState{
	PositionStateEmpty:   {PositionStateOpen},
	PositionStateOpen:    {PositionStateReduced, PositionStateClosed},
	PositionStateReduced: {PositionStateReduced, PositionStateClosed},
	PositionStateClosed:  {},
}

// PositionAggregate is the OMS v3 authoritative in-process position state.
// All state mutations flow exclusively through ApplyEvent — never via direct
// field assignment. State is reconstructed deterministically by replaying events.
type PositionAggregate struct {
	PositionID    string
	ClientOrderID string
	AccountID     string
	Symbol        string
	Side          string
	EntryPrice    float64
	Quantity      float64
	NotionalUSD   float64
	StopLoss      float64
	TakeProfit    float64
	StopLossPct   float64
	TakeProfitPct float64
	StrategyName  string

	// Set on close
	ExitPrice   float64
	ExitReason  string
	GrossPnLUSD float64
	NetPnLUSD   float64
	FeesUSD     float64
	HoldMinutes float64

	State   PositionState
	Version int64
	Events  []ledger.Event
}

// NewPositionAggregate creates an empty position aggregate for the given position ID.
func NewPositionAggregate(positionID string) *PositionAggregate {
	return &PositionAggregate{PositionID: positionID}
}

// CurrentState returns the aggregate's current state.
func (p *PositionAggregate) CurrentState() PositionState {
	if p == nil {
		return PositionStateEmpty
	}
	return p.State
}

// IsOpen returns true when the position is open or partially reduced.
func (p *PositionAggregate) IsOpen() bool {
	return p.State == PositionStateOpen || p.State == PositionStateReduced
}

// ValidatePositionTransition checks whether a transition to next is allowed.
func (p *PositionAggregate) ValidatePositionTransition(next PositionState) error {
	if p == nil {
		return fmt.Errorf("%w: nil position aggregate", ErrInvalidTransition)
	}
	for _, allowed := range positionTransitions[p.State] {
		if allowed == next {
			return nil
		}
	}
	return fmt.Errorf("%w: position %s -> %s", ErrInvalidTransition, p.State, next)
}

// ApplyEvent applies one ledger event to the position aggregate.
// Only EventPositionOpened, EventPositionChanged, and EventPositionClosed are accepted.
func (p *PositionAggregate) ApplyEvent(event ledger.Event) error {
	if event.AggregateType != ledger.AggregatePosition {
		return fmt.Errorf("omsv3: expected POSITION event, got %s", event.AggregateType)
	}
	next, ok := positionStateFromEvent(event.EventType)
	if !ok {
		return fmt.Errorf("omsv3: unsupported position event %s", event.EventType)
	}
	if err := p.ValidatePositionTransition(next); err != nil {
		return err
	}

	switch event.EventType {
	case ledger.EventPositionOpened:
		var payload PositionOpenedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("omsv3: unmarshal PositionOpenedPayload: %w", err)
		}
		p.ClientOrderID = firstNonEmpty(payload.ClientOrderID, p.ClientOrderID)
		p.PositionID = firstNonEmpty(payload.PositionID, p.PositionID, event.AggregateID)
		p.Symbol = firstNonEmpty(payload.Symbol, event.Symbol)
		p.Side = firstNonEmpty(payload.Side, p.Side)
		p.EntryPrice = firstPositive(payload.EntryPrice, p.EntryPrice)
		p.Quantity = firstPositive(payload.Quantity, p.Quantity)
		p.NotionalUSD = firstPositive(payload.NotionalUSD, p.NotionalUSD)
		p.StopLoss = payload.StopLoss
		p.TakeProfit = payload.TakeProfit
		p.StopLossPct = payload.StopLossPct
		p.TakeProfitPct = payload.TakeProfitPct
		p.StrategyName = firstNonEmpty(payload.StrategyName, event.StrategyID)

	case ledger.EventPositionChanged:
		var payload PositionChangedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("omsv3: unmarshal PositionChangedPayload: %w", err)
		}
		if payload.RemainingQty > 0 {
			p.Quantity = payload.RemainingQty
		}

	case ledger.EventPositionClosed, ledger.EventPositionLiquidated, ledger.EventPositionTransferred:
		var payload PositionClosedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("omsv3: unmarshal PositionClosedPayload: %w", err)
		}
		p.ExitPrice = payload.ExitPrice
		p.ExitReason = payload.ExitReason
		p.GrossPnLUSD = payload.GrossPnLUSD
		p.NetPnLUSD = payload.NetPnLUSD
		p.FeesUSD = payload.FeesUSD
		p.HoldMinutes = payload.HoldMinutes
	}

	p.AccountID = firstNonEmpty(p.AccountID, event.AccountID)
	p.State = next
	p.Version++ // version = count of events applied (independent of ledger SequenceNo)
	p.Events = append(p.Events, event)
	return nil
}

// ReplayPosition rebuilds a PositionAggregate by replaying a slice of events.
// Events must be for the same aggregate ID and in sequence order.
func ReplayPosition(events []ledger.Event) (*PositionAggregate, error) {
	if len(events) == 0 {
		return &PositionAggregate{}, nil
	}
	agg := NewPositionAggregate(events[0].AggregateID)
	for _, event := range events {
		if err := agg.ApplyEvent(event); err != nil {
			return nil, fmt.Errorf("ReplayPosition %s: %w", events[0].AggregateID, err)
		}
	}
	return agg, nil
}

func positionStateFromEvent(eventType ledger.EventType) (PositionState, bool) {
	switch eventType {
	case ledger.EventPositionOpened:
		return PositionStateOpen, true
	case ledger.EventPositionChanged, ledger.EventPositionReduced, ledger.EventPositionScaled:
		return PositionStateReduced, true
	case ledger.EventPositionClosed, ledger.EventPositionLiquidated, ledger.EventPositionTransferred:
		return PositionStateClosed, true
	default:
		return PositionStateEmpty, false
	}
}
