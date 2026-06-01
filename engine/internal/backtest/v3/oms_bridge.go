package v3

import (
	"context"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// OMSBridge emits OMS v3 events through the ledger for each simulated fill.
// This makes V3 backtest results replay-compatible with the live event store.
type OMSBridge struct {
	store     ledger.Store
	accountID string
	enabled   bool
}

// NewOMSBridge creates a bridge that writes to the given ledger store.
// If store is nil the bridge is disabled and all calls are no-ops.
func NewOMSBridge(store ledger.Store, accountID string) *OMSBridge {
	enabled := store != nil && accountID != ""
	return &OMSBridge{store: store, accountID: accountID, enabled: enabled}
}

// RecordOrderCreated emits EventOrderCreated for a new simulated order.
func (b *OMSBridge) RecordOrderCreated(ctx context.Context, orderID, symbol, side, strategyName string, quantity float64, ts time.Time) error {
	if !b.enabled {
		return nil
	}
	payload := omsv3.OrderCreatedPayload{
		ClientOrderID: orderID,
		Symbol:        symbol,
		Side:          side,
		Quantity:      quantity,
		StrategyName:  strategyName,
		OrderType:     "MARKET",
	}
	return b.emit(ctx, ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    orderID,
		EventType:      ledger.EventOrderCreated,
		AccountID:      b.accountID,
		StrategyID:     strategyName,
		Symbol:         symbol,
		CorrelationID:  orderID,
		IdempotencyKey: orderID + ":created",
		Payload:        payload,
		CreatedAt:      ts,
		Source:         "backtest-v3",
	})
}

// RecordOrderFilled emits EventOrderFilled for a completed simulated fill.
func (b *OMSBridge) RecordOrderFilled(ctx context.Context, orderID, symbol, strategyName string, fillPrice, fillQty, feeUSD, slippageBps float64, ts time.Time) error {
	if !b.enabled {
		return nil
	}
	payload := omsv3.OrderFillPayload{
		ClientOrderID:   orderID,
		ExchangeOrderID: "BT3-" + orderID,
		FillPrice:       fillPrice,
		FillQuantity:    fillQty,
		FeeUSD:          feeUSD,
		SlippageBps:     slippageBps,
	}
	return b.emit(ctx, ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    orderID,
		EventType:      ledger.EventOrderFilled,
		AccountID:      b.accountID,
		StrategyID:     strategyName,
		Symbol:         symbol,
		CorrelationID:  orderID,
		IdempotencyKey: orderID + ":filled",
		Payload:        payload,
		CreatedAt:      ts,
		Source:         "backtest-v3",
	})
}

// RecordOrderPartialFill emits EventOrderPartial for a partial fill event.
func (b *OMSBridge) RecordOrderPartialFill(ctx context.Context, orderID, symbol, strategyName string, event OrderPartialFillEvent) error {
	if !b.enabled {
		return nil
	}
	payload := omsv3.OrderFillPayload{
		ClientOrderID: orderID,
		FillPrice:     event.AverageFillPrice,
		FillQuantity:  event.FilledQuantity,
	}
	return b.emit(ctx, ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    orderID,
		EventType:      ledger.EventOrderPartial,
		AccountID:      b.accountID,
		StrategyID:     strategyName,
		Symbol:         symbol,
		CorrelationID:  orderID,
		IdempotencyKey: fmt.Sprintf("%s:partial:%d", orderID, event.FillNumber),
		Payload:        payload,
		CreatedAt:      event.Timestamp,
		Source:         "backtest-v3",
	})
}

// RecordOrderRejected emits EventOrderRejected for a blocked order (e.g., exchange outage).
func (b *OMSBridge) RecordOrderRejected(ctx context.Context, orderID, symbol, strategyName, reason string, ts time.Time) error {
	if !b.enabled {
		return nil
	}
	payload := omsv3.OrderRejectedPayload{Reason: reason}
	return b.emit(ctx, ledger.NewEventInput{
		AggregateType:  ledger.AggregateOrder,
		AggregateID:    orderID,
		EventType:      ledger.EventOrderRejected,
		AccountID:      b.accountID,
		StrategyID:     strategyName,
		Symbol:         symbol,
		CorrelationID:  orderID,
		IdempotencyKey: orderID + ":rejected",
		Payload:        payload,
		CreatedAt:      ts,
		Source:         "backtest-v3",
	})
}

// RecordPositionOpened emits EventPositionOpened linked to the order.
func (b *OMSBridge) RecordPositionOpened(ctx context.Context, positionID, orderID, symbol, side, strategyName string, entryPrice, qty, slPct, tpPct float64, ts time.Time) error {
	if !b.enabled {
		return nil
	}
	payload := omsv3.PositionOpenedPayload{
		PositionID:    positionID,
		ClientOrderID: orderID,
		Symbol:        symbol,
		Side:          side,
		EntryPrice:    entryPrice,
		Quantity:      qty,
		NotionalUSD:   entryPrice * qty,
		StopLossPct:   slPct,
		TakeProfitPct: tpPct,
		StrategyName:  strategyName,
	}
	return b.emit(ctx, ledger.NewEventInput{
		AggregateType:  ledger.AggregatePosition,
		AggregateID:    positionID,
		EventType:      ledger.EventPositionOpened,
		AccountID:      b.accountID,
		StrategyID:     strategyName,
		Symbol:         symbol,
		CorrelationID:  orderID,
		IdempotencyKey: positionID + ":opened",
		Payload:        payload,
		CreatedAt:      ts,
		Source:         "backtest-v3",
	})
}

// RecordPositionClosed emits EventPositionClosed with full P&L attribution.
func (b *OMSBridge) RecordPositionClosed(ctx context.Context, t V3Trade) error {
	if !b.enabled {
		return nil
	}
	payload := omsv3.PositionClosedPayload{
		PositionID:   t.ID,
		Symbol:       t.Symbol,
		ExitPrice:    t.ExitPrice,
		ExitReason:   t.ExitReason,
		GrossPnLUSD:  t.GrossPnL,
		NetPnLUSD:    t.NetPnL,
		FeesUSD:      t.CommissionUSD,
		HoldMinutes:  t.HoldMinutes,
		StrategyName: t.StrategyName,
	}
	return b.emit(ctx, ledger.NewEventInput{
		AggregateType:  ledger.AggregatePosition,
		AggregateID:    t.ID,
		EventType:      ledger.EventPositionClosed,
		AccountID:      b.accountID,
		StrategyID:     t.StrategyName,
		Symbol:         t.Symbol,
		CorrelationID:  t.ID,
		IdempotencyKey: t.ID + ":closed",
		Payload:        payload,
		CreatedAt:      t.ClosedAt,
		Source:         "backtest-v3",
	})
}

// emit constructs a properly hashed Event via ledger.NewEvent and appends it to the store.
func (b *OMSBridge) emit(ctx context.Context, input ledger.NewEventInput) error {
	ev, err := ledger.NewEvent(input)
	if err != nil {
		return fmt.Errorf("oms_bridge: build event %s: %w", input.EventType, err)
	}
	_, err = b.store.Append(ctx, ev)
	return err
}
