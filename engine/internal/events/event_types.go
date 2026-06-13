package events

import "time"

// event_types.go defines the full set of strongly-typed event structs used
// throughout the engine pipeline. Each type corresponds to one EventType
// constant in bus.go. Consumers type-assert Event.Payload against these types.

// ── Signal events ─────────────────────────────────────────────────────────────

// SignalGeneratedEvent is emitted by strategy evaluation before any gating.
type SignalGeneratedEvent struct {
	EventID      string
	EventType    string
	Timestamp    time.Time
	Source       string
	StrategyName string
	Symbol       string
	Direction    string
	Confidence   float64
	Regime       string
	Payload      interface{}
}

// RiskApprovedEvent is emitted by the risk gate when a signal passes all checks.
type RiskApprovedEvent struct {
	EventID      string
	EventType    string
	Timestamp    time.Time
	Source       string
	StrategyName string
	Symbol       string
	Direction    string
	Confidence   float64
	PositionSize float64
}

// RiskRejectedEvent is emitted by the risk gate when a signal fails risk checks.
type RiskRejectedEvent struct {
	EventID      string
	EventType    string
	Timestamp    time.Time
	Source       string
	StrategyName string
	Symbol       string
	Direction    string
	Reason       string
}

// ── Order events ──────────────────────────────────────────────────────────────

// OrderCreatedEvent is emitted by OMS v3 when an order aggregate is first created.
type OrderCreatedEvent struct {
	EventID       string
	EventType     string
	Timestamp     time.Time
	Source        string
	ClientOrderID string
	AccountID     string
	Symbol        string
	Side          string
	Quantity      float64
	Price         float64
	OrderType     string
}

// OrderSubmittedEvent is emitted when an order is sent to the broker.
type OrderSubmittedEvent struct {
	EventID       string
	EventType     string
	Timestamp     time.Time
	Source        string
	ClientOrderID string
	BrokerOrderID string
	AccountID     string
}

// OrderFilledEvent is emitted when a fill is received.
type OrderFilledEvent struct {
	EventID       string
	EventType     string
	Timestamp     time.Time
	Source        string
	ClientOrderID string
	BrokerOrderID string
	AccountID     string
	FillPrice     float64
	FillQuantity  float64
	Commission    float64
}

// OrderCancelledEvent is emitted when an order is cancelled (by user or system).
type OrderCancelledEvent struct {
	EventID       string
	EventType     string
	Timestamp     time.Time
	Source        string
	ClientOrderID string
	Reason        string
	AccountID     string
}

// ── Position events ───────────────────────────────────────────────────────────

// PositionOpenedEvent is emitted when a new position is created.
type PositionOpenedEvent struct {
	EventID      string
	EventType    string
	Timestamp    time.Time
	Source       string
	PositionID   string
	AccountID    string
	Symbol       string
	Direction    string
	EntryPrice   float64
	Size         float64
	StopLoss     float64
	TakeProfit1  float64
	TakeProfit2  float64
	StrategyName string
}

// PositionClosedEvent is emitted when a position is fully exited.
type PositionClosedEvent struct {
	EventID      string
	EventType    string
	Timestamp    time.Time
	Source       string
	PositionID   string
	AccountID    string
	Symbol       string
	ExitPrice    float64
	RealizedPnL  float64
	ExitReason   string
	StrategyName string
}

// ── Ledger / reconciliation events ───────────────────────────────────────────

// LedgerUpdatedEvent is emitted by the ledger after every event store write.
type LedgerUpdatedEvent struct {
	EventID   string
	EventType string
	Timestamp time.Time
	Source    string
	AccountID string
	EventKind string // the inner ledger event type (e.g. "ORDER_CREATED")
	Sequence  int64
}

// ReconciliationCompleteEvent is emitted after each reconciliation cycle.
type ReconciliationCompleteEvent struct {
	EventID          string
	EventType        string
	Timestamp        time.Time
	Source           string
	AccountID        string
	MismatchCount    int
	DriftScore       float64
	DurationMs       int64
	DiscrepancyCount int
}

// ── System events ─────────────────────────────────────────────────────────────

// KillSwitchActivatedEvent is emitted by the kill switch when trading is halted.
type KillSwitchActivatedEvent struct {
	EventID    string
	EventType  string
	Timestamp  time.Time
	Source     string
	AccountID  string
	Trigger    string
	Reason     string
	ActivatedBy string
}

// KillSwitchResumedEvent is emitted when the kill switch is released.
type KillSwitchResumedEvent struct {
	EventID    string
	EventType  string
	Timestamp  time.Time
	Source     string
	AccountID  string
	ReleasedBy string
	Reason     string
}

// RegimeChangedEvent is emitted when the market regime transitions.
type RegimeChangedEvent struct {
	EventID      string
	EventType    string
	Timestamp    time.Time
	Source       string
	PrevRegime   string
	NewRegime    string
	Confidence   float64
	SizeMult     float64
	AllowEntries bool
}

// DailyLossLimitHitEvent is emitted when the daily loss limit is breached.
type DailyLossLimitHitEvent struct {
	EventID        string
	EventType      string
	Timestamp      time.Time
	Source         string
	AccountID      string
	DailyLossUSD   float64
	LimitUSD       float64
	LimitPct       float64
}
