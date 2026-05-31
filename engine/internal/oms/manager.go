// Package oms — OMSManager
//
// Central registry for all orders.  Provides thread-safe CRUD, state
// transition helpers, and aggregated portfolio statistics.
// The manager never touches the exchange — that is the Execution Engine's job.
package oms

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ── ID generator ──────────────────────────────────────────────────────────────

var orderSeq atomic.Int64

func nextOrderID() string {
	n := orderSeq.Add(1)
	return fmt.Sprintf("ORD-%d-%d", time.Now().UnixMilli(), n)
}

// ── OMSManager ────────────────────────────────────────────────────────────────

// OMSManager is the single source of truth for all order state.
// Every order in the system MUST be registered here.
type OMSManager struct {
	mu     sync.RWMutex
	orders map[string]*Order // orderID → order

	// Aggregate counters for fast reads
	openCount     atomic.Int64
	filledCount   atomic.Int64
	rejectedCount atomic.Int64
	cancelledCount atomic.Int64

	// Callbacks triggered on state transitions
	onFill   func(*Order)
	onReject func(*Order)
	onClose  func(*Order)
}

// NewOMSManager creates an empty OMS manager.
func NewOMSManager() *OMSManager {
	return &OMSManager{
		orders: make(map[string]*Order, 256),
	}
}

// OnFill registers a callback fired when an order reaches FILLED state.
// Used by ExecutionEngineV2 to open positions.
func (m *OMSManager) OnFill(fn func(*Order)) { m.onFill = fn }

// OnReject registers a callback fired when an order is rejected.
func (m *OMSManager) OnReject(fn func(*Order)) { m.onReject = fn }

// OnClose registers a callback fired when a position is closed.
func (m *OMSManager) OnClose(fn func(*Order)) { m.onClose = fn }

// ── Order lifecycle ────────────────────────────────────────────────────────────

// CreateOrder builds a new order, registers it, and returns it in PENDING state.
func (m *OMSManager) CreateOrder(stratID int, stratName, symbol string,
	side OrderSide, orderType OrderType, qty, notional, leverage float64) *Order {

	id := nextOrderID()
	o := NewOrder(id, id, stratID, stratName, symbol, side, orderType, qty, notional, leverage)

	m.mu.Lock()
	m.orders[id] = o
	m.mu.Unlock()

	m.openCount.Add(1)
	log.Printf("[OMS] CREATE %s strat=%s %s %s qty=%.4f notional=$%.2f",
		id, stratName, side, symbol, qty, notional)
	return o
}

// Submit transitions an order to SUBMITTED, recording the exchange order ID.
func (m *OMSManager) Submit(orderID, exchangeOrderID string) error {
	m.mu.RLock()
	o, ok := m.orders[orderID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("OMS: order %s not found", orderID)
	}
	o.ExchangeOrderID = exchangeOrderID
	return o.Transition(StateSubmitted, "submitted to exchange")
}

// RecordFill records a fill event and fires the OnFill callback when fully filled.
func (m *OMSManager) RecordFill(orderID string, fillPrice, fillQty, feeUSD float64) error {
	m.mu.RLock()
	o, ok := m.orders[orderID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("OMS: order %s not found", orderID)
	}

	if err := o.Fill(fillPrice, fillQty, feeUSD); err != nil {
		return err
	}

	if o.State == StateFilled {
		m.openCount.Add(-1)
		m.filledCount.Add(1)
		log.Printf("[OMS] FILLED %s avg=%.4f fee=$%.4f latency=%dms",
			orderID, o.AvgFillPrice, feeUSD, o.LatencyFillMs())
		if m.onFill != nil {
			go m.onFill(o) // async to avoid blocking OMS
		}
	}
	return nil
}

// Reject marks an order as rejected.
func (m *OMSManager) Reject(orderID, reason string) error {
	m.mu.RLock()
	o, ok := m.orders[orderID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("OMS: order %s not found", orderID)
	}
	if err := o.Transition(StateRejected, reason); err != nil {
		return err
	}
	m.openCount.Add(-1)
	m.rejectedCount.Add(1)
	log.Printf("[OMS] REJECTED %s reason=%s", orderID, reason)
	if m.onReject != nil {
		go m.onReject(o)
	}
	return nil
}

// Cancel cancels a pending or submitted order.
func (m *OMSManager) Cancel(orderID, reason string) error {
	m.mu.RLock()
	o, ok := m.orders[orderID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("OMS: order %s not found", orderID)
	}
	if err := o.Transition(StateCancelled, reason); err != nil {
		return err
	}
	m.openCount.Add(-1)
	m.cancelledCount.Add(1)
	log.Printf("[OMS] CANCELLED %s reason=%s", orderID, reason)
	return nil
}

// Close transitions a filled order to CLOSED (position closed).
func (m *OMSManager) Close(orderID, reason string) error {
	m.mu.RLock()
	o, ok := m.orders[orderID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("OMS: order %s not found", orderID)
	}
	if err := o.Transition(StateClosed, reason); err != nil {
		return err
	}
	log.Printf("[OMS] CLOSED %s reason=%s", orderID, reason)
	if m.onClose != nil {
		go m.onClose(o)
	}
	return nil
}

// ── Queries ────────────────────────────────────────────────────────────────────

// Get returns an order by ID.
func (m *OMSManager) Get(orderID string) (*Order, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[orderID]
	return o, ok
}

// ActiveOrders returns all orders not in a terminal state.
func (m *OMSManager) ActiveOrders() []*Order {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Order
	for _, o := range m.orders {
		if !o.IsTerminal() {
			result = append(result, o)
		}
	}
	return result
}

// OpenFilledOrders returns orders that have been filled and not yet closed.
func (m *OMSManager) OpenFilledOrders() []*Order {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*Order
	for _, o := range m.orders {
		if o.State == StateFilled {
			result = append(result, o)
		}
	}
	return result
}

// Stats returns aggregate counters.
func (m *OMSManager) Stats() (open, filled, rejected, cancelled int64) {
	return m.openCount.Load(), m.filledCount.Load(), m.rejectedCount.Load(), m.cancelledCount.Load()
}

// Snapshot returns a copy of all order states for the dashboard.
type OrderSnapshot struct {
	ID           string
	StrategyName string
	Symbol       string
	Side         string
	State        OrderState
	Quantity     float64
	FilledQty    float64
	AvgFillPrice float64
	Fees         float64
	CreatedAt    time.Time
	LatencyMs    int64
}

func (m *OMSManager) Snapshot() []OrderSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]OrderSnapshot, 0, len(m.orders))
	for _, o := range m.orders {
		result = append(result, OrderSnapshot{
			ID:           o.ID,
			StrategyName: o.StrategyName,
			Symbol:       o.Symbol,
			Side:         string(o.Side),
			State:        o.State,
			Quantity:     o.Quantity,
			FilledQty:    o.FilledQty,
			AvgFillPrice: o.AvgFillPrice,
			Fees:         o.Fees,
			CreatedAt:    o.CreatedAt,
			LatencyMs:    o.LatencyFillMs(),
		})
	}
	return result
}

// Reset clears all orders (used for paper account reset).
func (m *OMSManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders = make(map[string]*Order, 256)
	m.openCount.Store(0)
	m.filledCount.Store(0)
	m.rejectedCount.Store(0)
	m.cancelledCount.Store(0)
	log.Println("[OMS] Full reset")
}
