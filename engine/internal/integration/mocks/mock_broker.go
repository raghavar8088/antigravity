package mocks

import (
	"fmt"
	"sync"
)

// Order is a simplified order record for testing.
type Order struct {
	StrategyName string
	Bias         string
	EntryPrice   float64
	StopLoss     float64
	TakeProfit1  float64
	TakeProfit2  float64
	PositionUSD  float64
}

// Fill is a simplified fill record for testing.
type Fill struct {
	FillPrice float64
	Order     Order
}

// MockBroker records all submitted orders and returns configurable fills.
type MockBroker struct {
	FillPrice    float64 // price to fill at (defaults to order entry price)
	RejectOrders bool    // if true, all orders are rejected

	mu     sync.Mutex
	orders []Order
}

// NewMockBroker creates a MockBroker that fills at fillPrice.
func NewMockBroker(fillPrice float64) *MockBroker {
	return &MockBroker{FillPrice: fillPrice}
}

// Submit records an order and returns a fill. Returns error if RejectOrders is true.
func (b *MockBroker) Submit(order Order) (Fill, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.RejectOrders {
		return Fill{}, fmt.Errorf("order rejected by mock broker")
	}
	b.orders = append(b.orders, order)
	fp := b.FillPrice
	if fp == 0 {
		fp = order.EntryPrice
	}
	return Fill{FillPrice: fp, Order: order}, nil
}

// GetSubmittedOrders returns a copy of all submitted orders.
func (b *MockBroker) GetSubmittedOrders() []Order {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Order, len(b.orders))
	copy(out, b.orders)
	return out
}

// Reset clears all recorded orders.
func (b *MockBroker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.orders = b.orders[:0]
}
