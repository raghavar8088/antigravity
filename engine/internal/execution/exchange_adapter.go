package execution

import (
	"context"
	"time"
)

// ExchangeAdapter is the boundary between OMS v3 and a physical (or simulated)
// exchange. Adapters own exchange communication ONLY. They must never own order
// state, position state, or account balance. All state ownership belongs to
// OMS v3 and the ledger.
//
// Adapters return plain value types; callers (OMS v3) are responsible for
// converting results into ledger events.
type ExchangeAdapter interface {
	// PlaceOrder submits an order to the exchange and returns the exchange
	// acknowledgement. The adapter must not retry internally; retry logic is
	// the responsibility of the OMS layer.
	PlaceOrder(ctx context.Context, req OrderRequest) (OrderAck, error)

	// CancelOrder requests cancellation of an active exchange order.
	// Returns nil when the cancellation is accepted (not necessarily processed).
	CancelOrder(ctx context.Context, exchangeOrderID string, symbol string) error

	// GetOrder fetches the current status of a single order from the exchange.
	// Used by reconciliation to detect fills the OMS has not yet seen.
	GetOrder(ctx context.Context, exchangeOrderID string, symbol string) (ExchangeOrderStatus, error)

	// GetPosition returns the current signed net position for a symbol.
	// For long positions the quantity is positive; shorts are negative.
	GetPosition(ctx context.Context, symbol string) (ExchangePositionStatus, error)

	// GetBalance returns the current account balance from the exchange.
	GetBalance(ctx context.Context) (ExchangeBalanceStatus, error)

	// Name returns a human-readable adapter name for logging and routing.
	Name() string
}

// ─── Request / response types ─────────────────────────────────────────────────

// OrderRequest carries the parameters for placing one order.
type OrderRequest struct {
	ClientOrderID string
	Symbol        string
	Side          string    // BUY | SELL
	Quantity      float64
	OrderType     string    // MARKET | LIMIT | POST_ONLY | IOC
	LimitPrice    float64   // 0 for market orders
	TimeInForce   string    // GTC | IOC | FOK (empty defaults to exchange default)
	RequestedAt   time.Time
}

// OrderAck is the exchange's acknowledgement of a submitted order.
type OrderAck struct {
	ExchangeOrderID string
	ClientOrderID   string
	Status          string    // PENDING | ACCEPTED | REJECTED
	RejectReason    string    // non-empty when Status == REJECTED
	AcknowledgedAt  time.Time
}

// ExchangeOrderStatus is the real-time status of one order at the exchange.
type ExchangeOrderStatus struct {
	ExchangeOrderID  string
	ClientOrderID    string
	Symbol           string
	Side             string
	Status           string  // NEW | PARTIALLY_FILLED | FILLED | CANCELLED | REJECTED | EXPIRED
	OrderedQuantity  float64
	FilledQuantity   float64
	AverageFillPrice float64
	RemainingQty     float64
	FeesUSD          float64
	UpdatedAt        time.Time
}

// ExchangePositionStatus is the net position reported by the exchange.
type ExchangePositionStatus struct {
	Symbol          string
	NetQuantity     float64 // positive = long, negative = short
	AveragePrice    float64
	UnrealizedPnL   float64
	LiquidationPrice float64
	UpdatedAt       time.Time
}

// ExchangeBalanceStatus is the account balance reported by the exchange.
type ExchangeBalanceStatus struct {
	EquityUSD      float64
	AvailableUSD   float64
	MarginUsedUSD  float64
	UnrealizedPnL  float64
	UpdatedAt      time.Time
}

// ─── NoOpAdapter ──────────────────────────────────────────────────────────────

// NoOpAdapter is a safe default that rejects all orders.
// Use in tests and when no real adapter is configured.
type NoOpAdapter struct{}

func (NoOpAdapter) Name() string { return "noop" }

func (NoOpAdapter) PlaceOrder(_ context.Context, _ OrderRequest) (OrderAck, error) {
	return OrderAck{Status: "REJECTED", RejectReason: "no adapter configured"}, nil
}

func (NoOpAdapter) CancelOrder(_ context.Context, _, _ string) error { return nil }

func (NoOpAdapter) GetOrder(_ context.Context, _, _ string) (ExchangeOrderStatus, error) {
	return ExchangeOrderStatus{Status: "UNKNOWN"}, nil
}

func (NoOpAdapter) GetPosition(_ context.Context, symbol string) (ExchangePositionStatus, error) {
	return ExchangePositionStatus{Symbol: symbol}, nil
}

func (NoOpAdapter) GetBalance(_ context.Context) (ExchangeBalanceStatus, error) {
	return ExchangeBalanceStatus{}, nil
}
