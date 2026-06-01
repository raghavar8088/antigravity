package omsv3

import (
	"errors"
	"fmt"

	"antigravity-engine/internal/ledger"
)

type OrderState string

const (
	StateEmpty           OrderState = ""
	StateNew             OrderState = "NEW"
	StateValidated       OrderState = "VALIDATED"
	StateRiskApproved    OrderState = "RISK_APPROVED"
	StateSubmitted       OrderState = "SUBMITTED"
	StateAcknowledged    OrderState = "ACKNOWLEDGED"
	StatePartiallyFilled OrderState = "PARTIALLY_FILLED"
	StateFilled          OrderState = "FILLED"
	StateCancelled       OrderState = "CANCELLED"
	StateRejected        OrderState = "REJECTED"
)

var ErrInvalidTransition = errors.New("omsv3: invalid order transition")

var transitions = map[OrderState][]OrderState{
	StateEmpty:           {StateNew},
	StateNew:             {StateValidated, StateCancelled, StateRejected},
	StateValidated:       {StateRiskApproved, StateCancelled, StateRejected},
	StateRiskApproved:    {StateSubmitted, StateCancelled, StateRejected},
	StateSubmitted:       {StateAcknowledged, StateCancelled, StateRejected},
	StateAcknowledged:    {StatePartiallyFilled, StateFilled, StateCancelled, StateRejected},
	StatePartiallyFilled: {StatePartiallyFilled, StateFilled, StateCancelled, StateRejected},
	StateFilled:          {},
	StateCancelled:       {},
	StateRejected:        {},
}

type OrderAggregate struct {
	ID               string
	AccountID        string
	Symbol           string
	Side             string
	Quantity         float64
	FilledQuantity   float64
	AverageFillPrice float64
	ExchangeOrderID  string
	State            OrderState
	Version          int64
	Events           []ledger.Event
}

func NewOrderAggregate(id string) *OrderAggregate {
	return &OrderAggregate{ID: id}
}

func (o *OrderAggregate) CurrentState() OrderState {
	if o == nil {
		return StateEmpty
	}
	return o.State
}

func (o *OrderAggregate) ValidateTransition(next OrderState) error {
	if o == nil {
		return fmt.Errorf("%w: nil aggregate", ErrInvalidTransition)
	}
	for _, allowed := range transitions[o.State] {
		if allowed == next {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, o.State, next)
}

func (o *OrderAggregate) ApplyEvent(event ledger.Event) error {
	if event.AggregateType != ledger.AggregateOrder {
		return fmt.Errorf("omsv3: expected ORDER event, got %s", event.AggregateType)
	}
	next, ok := stateFromEvent(event.EventType)
	if !ok {
		return fmt.Errorf("omsv3: unsupported order event %s", event.EventType)
	}
	if err := o.ValidateTransition(next); err != nil {
		return err
	}
	o.State = next
	o.Version++ // version = count of events applied (independent of ledger SequenceNo)
	o.Events = append(o.Events, event)

	projection, err := ledger.ProjectOrder(o.Events)
	if err != nil {
		return err
	}
	o.ID = firstNonEmpty(o.ID, projection.ClientOrderID, event.AggregateID)
	o.AccountID = firstNonEmpty(o.AccountID, event.AccountID)
	o.Symbol = firstNonEmpty(projection.Symbol, o.Symbol, event.Symbol)
	o.Side = firstNonEmpty(projection.Side, o.Side)
	o.Quantity = firstPositive(projection.Quantity, o.Quantity)
	o.FilledQuantity = projection.FilledQuantity
	o.AverageFillPrice = projection.AveragePrice
	o.ExchangeOrderID = firstNonEmpty(projection.ExchangeOrderID, o.ExchangeOrderID)
	return nil
}

func Replay(events []ledger.Event) (*OrderAggregate, error) {
	if len(events) == 0 {
		return &OrderAggregate{}, nil
	}
	aggregate := NewOrderAggregate(events[0].AggregateID)
	for _, event := range events {
		if err := aggregate.ApplyEvent(event); err != nil {
			return nil, err
		}
	}
	return aggregate, nil
}

func stateFromEvent(eventType ledger.EventType) (OrderState, bool) {
	switch eventType {
	case ledger.EventOrderCreated:
		return StateNew, true
	case ledger.EventOrderValidated:
		return StateValidated, true
	case ledger.EventRiskApproved:
		return StateRiskApproved, true
	case ledger.EventOrderSubmitted:
		return StateSubmitted, true
	case ledger.EventOrderAcked:
		return StateAcknowledged, true
	case ledger.EventOrderPartial:
		return StatePartiallyFilled, true
	case ledger.EventOrderFilled:
		return StateFilled, true
	case ledger.EventOrderCancelled:
		return StateCancelled, true
	case ledger.EventOrderRejected, ledger.EventRiskBlocked:
		return StateRejected, true
	default:
		return StateEmpty, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
