// Package oms — institutional Order Management System.
//
// All orders flow through a strict state machine.  Invalid transitions
// are rejected at compile-time through the transition table.  Every state
// change is logged with a timestamp for full audit trail.
package oms

import (
	"errors"
	"fmt"
	"time"
)

// ── Order states ──────────────────────────────────────────────────────────────

type OrderState string

const (
	StatePending   OrderState = "PENDING"   // created, not yet sent to exchange
	StateSubmitted OrderState = "SUBMITTED" // sent, awaiting acknowledgement
	StatePartial   OrderState = "PARTIAL"   // partially filled
	StateFilled    OrderState = "FILLED"    // fully filled
	StateRejected  OrderState = "REJECTED"  // exchange rejected
	StateCancelled OrderState = "CANCELLED" // cancelled before fill
	StateClosed    OrderState = "CLOSED"    // position closed (terminal)
)

// ── Order types ────────────────────────────────────────────────────────────────

type OrderSide string

const (
	SideBuy  OrderSide = "BUY"
	SideSell OrderSide = "SELL"
)

type OrderType string

const (
	TypeMarket  OrderType = "MARKET"
	TypeLimit   OrderType = "LIMIT"
	TypeStopMkt OrderType = "STOP_MARKET"
)

// ── State transition table ────────────────────────────────────────────────────

// validTransitions defines all legal state machine transitions.
// Any transition not in this map is rejected with an error.
var validTransitions = map[OrderState][]OrderState{
	StatePending:   {StateSubmitted, StateCancelled},
	StateSubmitted: {StateFilled, StatePartial, StateRejected, StateCancelled},
	StatePartial:   {StateFilled, StateCancelled},
	StateFilled:    {StateClosed},
	StateRejected:  {}, // terminal
	StateCancelled: {}, // terminal
	StateClosed:    {}, // terminal
}

// ── StateEvent ─────────────────────────────────────────────────────────────────

// StateEvent records one state transition for the audit log.
type StateEvent struct {
	From      OrderState
	To        OrderState
	Timestamp time.Time
	Reason    string
}

// ── Order ─────────────────────────────────────────────────────────────────────

// Order is a single instruction to the exchange with full lifecycle tracking.
type Order struct {
	// Identity
	ID           string
	ClientID     string // our internal reference
	StrategyID   int
	StrategyName string
	Symbol       string

	// Parameters
	Side       OrderSide
	Type       OrderType
	Quantity   float64 // base asset (BTC)
	LimitPrice float64 // only for LIMIT / STOP_MARKET orders
	Notional   float64 // USD notional at order creation
	Leverage   float64

	// Risk parameters
	SLPrice float64 // stop-loss absolute price
	TPPrice float64 // take-profit absolute price

	// Fill details
	FilledQty   float64
	AvgFillPrice float64
	Fees        float64

	// State machine
	State      OrderState
	StateLog   []StateEvent
	CreatedAt  time.Time
	UpdatedAt  time.Time
	SubmittedAt time.Time
	FilledAt   time.Time
	ClosedAt   time.Time

	// Exchange response
	ExchangeOrderID string
	RejectReason    string

	// Metadata
	SignalScore int
	SignalID    string
}

// NewOrder creates an order in PENDING state.
func NewOrder(id, clientID string, stratID int, stratName, symbol string,
	side OrderSide, orderType OrderType, qty, notional, leverage float64) *Order {

	now := time.Now()
	o := &Order{
		ID:           id,
		ClientID:     clientID,
		StrategyID:   stratID,
		StrategyName: stratName,
		Symbol:       symbol,
		Side:         side,
		Type:         orderType,
		Quantity:     qty,
		Notional:     notional,
		Leverage:     leverage,
		State:        StatePending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	o.StateLog = append(o.StateLog, StateEvent{
		From:      "",
		To:        StatePending,
		Timestamp: now,
		Reason:    "created",
	})
	return o
}

// Transition moves the order to the next state, enforcing the transition table.
// Returns an error if the transition is not allowed.
func (o *Order) Transition(next OrderState, reason string) error {
	allowed, ok := validTransitions[o.State]
	if !ok {
		return fmt.Errorf("OMS: order %s is in terminal state %s", o.ID, o.State)
	}
	for _, s := range allowed {
		if s == next {
			ev := StateEvent{
				From:      o.State,
				To:        next,
				Timestamp: time.Now(),
				Reason:    reason,
			}
			o.State = next
			o.UpdatedAt = ev.Timestamp
			o.StateLog = append(o.StateLog, ev)

			switch next {
			case StateSubmitted:
				o.SubmittedAt = ev.Timestamp
			case StateFilled:
				o.FilledAt = ev.Timestamp
			case StateClosed:
				o.ClosedAt = ev.Timestamp
			case StateRejected:
				o.RejectReason = reason
			}
			return nil
		}
	}
	return fmt.Errorf("OMS: invalid transition %s → %s for order %s",
		o.State, next, o.ID)
}

// IsTerminal returns true if no further transitions are possible.
func (o *Order) IsTerminal() bool {
	return o.State == StateRejected || o.State == StateCancelled || o.State == StateClosed
}

// IsFilled returns true if the order has been fully or partially filled.
func (o *Order) IsFilled() bool {
	return o.State == StateFilled || o.State == StatePartial
}

// Fill records a (partial or full) fill.
func (o *Order) Fill(fillPrice, fillQty, feeUSD float64) error {
	if o.State != StateSubmitted && o.State != StatePartial {
		return errors.New("OMS: fill on order not in SUBMITTED or PARTIAL state")
	}

	o.FilledQty += fillQty
	// Update average fill price
	prevNotional := (o.FilledQty - fillQty) * o.AvgFillPrice
	newNotional := fillQty * fillPrice
	o.AvgFillPrice = (prevNotional + newNotional) / o.FilledQty
	o.Fees += feeUSD

	if o.FilledQty >= o.Quantity-1e-8 {
		return o.Transition(StateFilled, fmt.Sprintf("full fill @ %.4f", fillPrice))
	}
	return o.Transition(StatePartial, fmt.Sprintf("partial fill %.4f/%.4f @ %.4f",
		o.FilledQty, o.Quantity, fillPrice))
}

// LatencySubmitMs returns the milliseconds from creation to submission.
func (o *Order) LatencySubmitMs() int64 {
	if o.SubmittedAt.IsZero() {
		return -1
	}
	return o.SubmittedAt.Sub(o.CreatedAt).Milliseconds()
}

// LatencyFillMs returns the milliseconds from submission to fill.
func (o *Order) LatencyFillMs() int64 {
	if o.FilledAt.IsZero() || o.SubmittedAt.IsZero() {
		return -1
	}
	return o.FilledAt.Sub(o.SubmittedAt).Milliseconds()
}
