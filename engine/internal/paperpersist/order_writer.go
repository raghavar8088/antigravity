package paperpersist

// order_writer.go — OMS lifecycle transition persistence.
//
// Every time the OMS v3 state machine transitions an order through its states
// (NEW → VALIDATED → RISK_APPROVED → SUBMITTED → ACKNOWLEDGED →
//  SIMULATED_FILL → POSITION_OPENED → POSITION_CLOSED | REJECTED | CANCELLED)
// the caller records the transition via OrderWriter.RecordTransition().
//
// Each transition is an independent append-only document in paper_orders.
// Uniqueness is enforced on (order_id, transition_to) so replays are safe.
//
// Security: account_key is always ownerAccountKey. Never client input.

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── OMS state constants ───────────────────────────────────────────────────────

// These mirror omsv3.OrderState but are redeclared here to keep paperpersist
// free from circular imports.
type OMSState string

const (
	OMSNew            OMSState = "NEW"
	OMSRiskChecked    OMSState = "RISK_CHECKED"
	OMSAccepted       OMSState = "ACCEPTED"
	OMSSimulatedFill  OMSState = "SIMULATED_FILL"
	OMSPositionOpened OMSState = "POSITION_OPENED"
	OMSPositionClosed OMSState = "POSITION_CLOSED"
	OMSRejected       OMSState = "REJECTED"
	OMSCancelled      OMSState = "CANCELLED"
)

// ── OrderTransition ───────────────────────────────────────────────────────────

// OrderTransition captures a single OMS state-machine step.
type OrderTransition struct {
	// Order identity
	OrderID       string `json:"order_id"`        // omsv3 ClientOrderID
	StrategyID    string `json:"strategy_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	RequestedSize float64 `json:"requested_size"` // BTC

	// Transition
	TransitionFrom OMSState `json:"transition_from"`
	TransitionTo   OMSState `json:"transition_to"`
	TransitionAt   time.Time `json:"transition_at"`

	// Rejection / cancellation reason (empty on success paths)
	Reason string `json:"reason,omitempty"`

	// Fill details (populated on SIMULATED_FILL and POSITION_OPENED)
	FillPrice  float64 `json:"fill_price,omitempty"`
	FillSize   float64 `json:"fill_size,omitempty"`
	SlippagePct float64 `json:"slippage_pct,omitempty"`

	// Risk gate outcome (populated on RISK_CHECKED)
	RiskApproved   bool    `json:"risk_approved"`
	KellyFraction  float64 `json:"kelly_fraction,omitempty"`
	ApprovedSizeBTC float64 `json:"approved_size_btc,omitempty"`

	// Position linkage (populated on POSITION_OPENED / POSITION_CLOSED)
	PositionID string `json:"position_id,omitempty"`
	NetPnL     float64 `json:"net_pnl,omitempty"`
}

// ── OrderWriter ───────────────────────────────────────────────────────────────

// OrderWriter appends OMS lifecycle transitions to paper_orders.
type OrderWriter struct {
	mgr *MongoManager
}

// NewOrderWriter returns a ready OrderWriter.
func NewOrderWriter(mgr *MongoManager) *OrderWriter {
	return &OrderWriter{mgr: mgr}
}

// RecordTransition persists a single OMS state transition.
// It is idempotent: re-recording the same (order_id, transition_to) is a no-op.
func (ow *OrderWriter) RecordTransition(ctx context.Context, t OrderTransition) error {
	if t.OrderID == "" {
		return fmt.Errorf("paperpersist/order_writer: OrderID required")
	}
	if t.TransitionAt.IsZero() {
		t.TransitionAt = time.Now()
	}

	col := ow.mgr.Col(ColPaperOrders)
	if col == nil {
		return fmt.Errorf("paperpersist/order_writer: not connected")
	}

	now := time.Now()
	doc := baseDoc(now)

	doc["order_id"] = t.OrderID
	doc["strategy_id"] = t.StrategyID
	doc["symbol"] = t.Symbol
	doc["side"] = t.Side
	doc["requested_size"] = t.RequestedSize
	doc["transition_from"] = string(t.TransitionFrom)
	doc["transition_to"] = string(t.TransitionTo)
	doc["transition_at"] = t.TransitionAt
	doc["recorded_at"] = now
	doc["risk_approved"] = t.RiskApproved

	if t.Reason != "" {
		doc["reason"] = t.Reason
	}
	if t.FillPrice > 0 {
		doc["fill_price"] = t.FillPrice
		doc["fill_size"] = t.FillSize
		doc["slippage_pct"] = t.SlippagePct
	}
	if t.KellyFraction > 0 {
		doc["kelly_fraction"] = t.KellyFraction
		doc["approved_size_btc"] = t.ApprovedSizeBTC
	}
	if t.PositionID != "" {
		doc["position_id"] = t.PositionID
	}
	if t.NetPnL != 0 {
		doc["net_pnl"] = t.NetPnL
	}
	doc["checksum"] = checksum(t)

	// Unique on (order_id, transition_to) so that replaying events is safe.
	filter := bson.M{
		"order_id":      t.OrderID,
		"transition_to": string(t.TransitionTo),
	}

	if err := upsertOne(ctx, col, filter, doc); err != nil {
		log.Printf("[paperpersist/order_writer] transition %s→%s for order %s: %v",
			t.TransitionFrom, t.TransitionTo, t.OrderID, err)
		return err
	}
	return nil
}

// RecordBatch persists multiple transitions in sequence. First failure stops
// the batch but does not roll back earlier writes.
func (ow *OrderWriter) RecordBatch(ctx context.Context, transitions []OrderTransition) error {
	for i, t := range transitions {
		if err := ow.RecordTransition(ctx, t); err != nil {
			return fmt.Errorf("paperpersist/order_writer: batch[%d] %s: %w", i, t.OrderID, err)
		}
	}
	return nil
}

// PositionDoc persists an open position to paper_positions.
// Called when the OMS transitions to POSITION_OPENED.
func (ow *OrderWriter) PersistOpenPosition(ctx context.Context, p OpenPosition) error {
	col := ow.mgr.Col(ColPaperPositions)
	if col == nil {
		return fmt.Errorf("paperpersist/order_writer: not connected")
	}

	now := time.Now()
	doc := baseDoc(now)

	doc["position_id"] = p.PositionID
	doc["order_id"] = p.OrderID
	doc["strategy_id"] = p.StrategyID
	doc["symbol"] = p.Symbol
	doc["side"] = p.Side
	doc["entry_price"] = p.EntryPrice
	doc["size"] = p.Size
	doc["stop_loss"] = p.StopLoss
	doc["take_profit"] = p.TakeProfit
	doc["status"] = "OPEN"
	doc["opened_at"] = p.OpenedAt
	doc["checksum"] = checksum(p)

	filter := bson.M{"position_id": p.PositionID}
	return upsertOne(ctx, col, filter, doc)
}

// ClosePosition updates the position document to CLOSED state.
// Called when the OMS transitions to POSITION_CLOSED.
func (ow *OrderWriter) ClosePosition(ctx context.Context, positionID string, exitPrice, netPnL float64, closedAt time.Time, reason string) error {
	col := ow.mgr.Col(ColPaperPositions)
	if col == nil {
		return fmt.Errorf("paperpersist/order_writer: not connected")
	}

	now := time.Now()
	filter := bson.M{"position_id": positionID}
	update := bson.M{
		"$set": bson.M{
			"status":     "CLOSED",
			"exit_price": exitPrice,
			"net_pnl":    netPnL,
			"closed_at":  closedAt,
			"exit_reason": reason,
			"updated_at": now,
			"account_key": ownerAccountKey, // paranoia: enforce on update too
		},
	}
	if col == nil {
		return fmt.Errorf("paperpersist/order_writer: not connected")
	}
	_, err := col.UpdateOne(ctx, filter, update)
	return err
}

// OpenPosition is the shape passed to PersistOpenPosition.
type OpenPosition struct {
	PositionID string
	OrderID    string
	StrategyID string
	Symbol     string
	Side       string
	EntryPrice float64
	Size       float64
	StopLoss   float64
	TakeProfit float64
	OpenedAt   time.Time
}
