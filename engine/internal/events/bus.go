// Package events provides a zero-dependency, in-process pub/sub event bus.
//
// Design principles:
//   - All execution-critical components (OMS, Risk, Execution Engine) publish
//     events and return immediately. They never wait for subscribers.
//   - Analytics, AI research, logging, and reporting subscribe asynchronously.
//   - The bus never blocks the publisher. If a subscriber's channel is full,
//     the event is dropped and a DroppedEvents counter increments.
//   - Each subscription runs in its own goroutine, isolated from other subscribers.
//   - The bus is safe for concurrent use from multiple goroutines.
package events

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ── Event Types ──────────────────────────────────────────────────────────────

type EventType string

const (
	EventSignalCreated    EventType = "SIGNAL_CREATED"
	EventSignalRejected   EventType = "SIGNAL_REJECTED"
	EventSignalApproved   EventType = "SIGNAL_APPROVED"
	EventOrderSubmitted   EventType = "ORDER_SUBMITTED"
	EventOrderFilled      EventType = "ORDER_FILLED"
	EventOrderRejected    EventType = "ORDER_REJECTED"
	EventOrderCancelled   EventType = "ORDER_CANCELLED"
	EventPositionOpened   EventType = "POSITION_OPENED"
	EventPositionUpdated  EventType = "POSITION_UPDATED"
	EventPositionClosed   EventType = "POSITION_CLOSED"
	EventRiskViolation    EventType = "RISK_VIOLATION"
	EventRegimeChanged    EventType = "REGIME_CHANGED"
	EventDailyReset       EventType = "DAILY_RESET"
	EventCircuitBreaker   EventType = "CIRCUIT_BREAKER"
	EventFundingAccrued   EventType = "FUNDING_ACCRUED"
	EventLatencyRecorded  EventType = "LATENCY_RECORDED"
	EventResearchComplete EventType = "RESEARCH_COMPLETE"
	EventAnomalyDetected  EventType = "ANOMALY_DETECTED"
)

// ── Event Payload ─────────────────────────────────────────────────────────────

// Event is the envelope carried on the bus. Payload is typed via interface{}.
// Subscribers type-assert the payload based on the EventType they subscribed to.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Payload   interface{}
}

// ── Payload structs ───────────────────────────────────────────────────────────

type SignalCreatedPayload struct {
	StrategyID   int
	StrategyName string
	Symbol       string
	Direction    string // "BUY" | "SELL"
	Confidence   float64
	Score        int
	SignalID     string
}

type SignalRejectedPayload struct {
	StrategyID   int
	StrategyName string
	Reason       string
	Score        int
}

type OrderSubmittedPayload struct {
	OrderID      string
	StrategyID   int
	Symbol       string
	Side         string
	Notional     float64
	Price        float64
	SubmittedAt  time.Time
}

type OrderFilledPayload struct {
	OrderID    string
	StrategyID int
	Symbol     string
	Side       string
	FillPrice  float64
	Notional   float64
	Fee        float64
	FilledAt   time.Time
	LatencyMs  int64
}

type PositionOpenedPayload struct {
	PositionID   string
	StrategyID   int
	StrategyName string
	Symbol       string
	Side         string
	EntryPrice   float64
	Notional     float64
	SLPrice      float64
	TPPrice      float64
	OpenedAt     time.Time
}

type PositionClosedPayload struct {
	PositionID   string
	StrategyID   int
	StrategyName string
	Symbol       string
	Side         string
	EntryPrice   float64
	ExitPrice    float64
	NetPnL       float64
	Fees         float64
	ExitReason   string
	HoldMinutes  float64
	OpenedAt     time.Time
	ClosedAt     time.Time
}

type RiskViolationPayload struct {
	StrategyID int
	Symbol     string
	Reason     string
	Signal     string
}

type RegimeChangedPayload struct {
	Previous string
	Current  string
	ADX      float64
	ATR      float64
	EMASlope float64
}

type CircuitBreakerPayload struct {
	Reason     string
	DailyPnL   float64
	DailyLimit float64
}

type LatencyRecordedPayload struct {
	SignalToApproveMs int64
	ApproveToOrderMs  int64
	OrderToFillMs     int64
	TotalMs           int64
	StrategyID        int
}

// ── Bus ───────────────────────────────────────────────────────────────────────

const subscriberBufferSize = 256

// subscription holds a subscriber's channel and label.
type subscription struct {
	ch    chan Event
	label string
}

// Bus is a thread-safe in-process publish/subscribe event bus.
type Bus struct {
	mu            sync.RWMutex
	subscribers   map[EventType][]subscription
	droppedEvents atomic.Int64
	totalEvents   atomic.Int64
}

// New creates an empty event bus.
func New() *Bus {
	return &Bus{
		subscribers: make(map[EventType][]subscription),
	}
}

// Subscribe registers a handler function for the given event types.
// The handler runs in a dedicated goroutine and must be safe for concurrent use.
// label is used for logging dropped events.
func (b *Bus) Subscribe(label string, handler func(Event), types ...EventType) {
	ch := make(chan Event, subscriberBufferSize)

	b.mu.Lock()
	for _, t := range types {
		b.subscribers[t] = append(b.subscribers[t], subscription{ch: ch, label: label})
	}
	b.mu.Unlock()

	go func() {
		for ev := range ch {
			safeHandle(label, handler, ev)
		}
	}()
}

// Publish emits an event to all subscribers of its type.
// This call is non-blocking: if a subscriber's buffer is full the event is dropped.
func (b *Bus) Publish(evType EventType, payload interface{}) {
	ev := Event{
		Type:      evType,
		Timestamp: time.Now(),
		Payload:   payload,
	}

	b.totalEvents.Add(1)

	b.mu.RLock()
	subs := b.subscribers[evType]
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub.ch <- ev:
		default:
			b.droppedEvents.Add(1)
			log.Printf("[EVENTS] DROP %s → subscriber=%s (buffer full)", evType, sub.label)
		}
	}
}

// Stats returns operational counters.
func (b *Bus) Stats() (total int64, dropped int64) {
	return b.totalEvents.Load(), b.droppedEvents.Load()
}

func safeHandle(label string, fn func(Event), ev Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[EVENTS] PANIC in subscriber=%s type=%s: %v", label, ev.Type, r)
		}
	}()
	fn(ev)
}
