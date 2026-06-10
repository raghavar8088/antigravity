package reconciliationv2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// LedgerOMSStateReader builds reconciliation OMSSnapshot views by replaying the
// durable event ledger. This is the internal OMS truth used when comparing against
// exchange REST snapshots or the live position manager runtime.
type LedgerOMSStateReader struct {
	store     ledger.Store
	accountID string
}

// NewLedgerOMSStateReader creates a reader backed by the shared ledger store.
func NewLedgerOMSStateReader(store ledger.Store, accountID string) *LedgerOMSStateReader {
	if accountID == "" {
		accountID = "btc-paper-1"
	}
	return &LedgerOMSStateReader{store: store, accountID: accountID}
}

// GetOMSSnapshot implements OMSStateReader.
func (r *LedgerOMSStateReader) GetOMSSnapshot(ctx context.Context, accountID string) (OMSSnapshot, error) {
	if r.store == nil {
		return OMSSnapshot{}, fmt.Errorf("ledger oms reader: store is nil")
	}
	if accountID == "" {
		accountID = r.accountID
	}

	events, err := r.store.ReplayAccount(ctx, accountID)
	if err != nil {
		return OMSSnapshot{}, fmt.Errorf("ledger oms reader: replay account: %w", err)
	}

	openPos := omsv3.BuildOpenPositionProjections(events)
	orderProjs := omsv3.BuildOrderProjections(events)
	exposure := omsv3.BuildExposureProjection(events)
	pnl := omsv3.BuildPnLProjection(events)

	positions := make([]OMSPosition, 0, len(openPos))
	for _, pos := range openPos {
		positions = append(positions, OMSPosition{
			PositionID:    pos.PositionID,
			ClientOrderID: pos.ClientOrderID,
			Symbol:        pos.Symbol,
			Side:          pos.Side,
			State:         pos.State,
			EntryPrice:    pos.EntryPrice,
			Quantity:      pos.Quantity,
			NotionalUSD:   pos.NotionalUSD,
			UnrealPnL:     0,
			StopLoss:      pos.StopLoss,
			TakeProfit:    pos.TakeProfit,
			StrategyName:  pos.StrategyName,
		})
	}

	openOrders := make([]OMSOrder, 0)
	for _, order := range orderProjs {
		if !isLiveOMSOrderState(order.State) {
			continue
		}
		openOrders = append(openOrders, OMSOrder{
			ClientOrderID:   order.ClientOrderID,
			ExchangeOrderID: order.ExchangeOrderID,
			Symbol:          order.Symbol,
			Side:            order.Side,
			State:           order.State,
			Quantity:        order.Quantity,
			FilledQuantity:  order.FilledQuantity,
			AveragePrice:    order.AveragePrice,
			FeesUSD:         order.FeesUSD,
			StrategyName:    order.StrategyName,
			UpdatedAt:       time.Now().UTC(),
		})
	}

	now := time.Now().UTC()
	recentFills := buildRecentFillsFromEvents(events, now.Add(-30*time.Minute))

	grossNotional := exposure.TotalNotionalUSD
	netNotional := 0.0
	for _, qty := range exposure.NetExposure {
		netNotional += qty
	}
	if netNotional < 0 {
		netNotional = -netNotional
	}

	return OMSSnapshot{
		AccountID: accountID,
		Balance: OMSBalanceSnapshot{
			EquityUSD:     pnl.TotalPnLUSD,
			AvailableUSD:  pnl.TotalPnLUSD,
			RealizedPnL:   pnl.TotalPnLUSD,
		},
		Positions:     positions,
		OpenOrders:    openOrders,
		RecentFills:   recentFills,
		GrossNotional: grossNotional,
		NetNotional:   netNotional,
		SnapshotAt:    now,
	}, nil
}

func isLiveOMSOrderState(state string) bool {
	switch state {
	case string(ledger.OrderStateNew),
		string(ledger.OrderStateValidated),
		string(ledger.OrderStateRiskApproved),
		string(ledger.OrderStateSubmitted),
		string(ledger.OrderStateAcknowledged),
		string(ledger.OrderStatePartialFill):
		return true
	default:
		return false
	}
}

func buildRecentFillsFromEvents(events []ledger.Event, since time.Time) []OMSFill {
	fills := make([]OMSFill, 0)
	for _, ev := range events {
		if ev.AggregateType != ledger.AggregateOrder {
			continue
		}
		if ev.EventType != ledger.EventOrderFilled && ev.EventType != ledger.EventOrderPartial {
			continue
		}
		if !since.IsZero() && ev.CreatedAt.Before(since) {
			continue
		}
		var payload ledger.OrderPayload
		if len(ev.Payload) > 0 {
			_ = json.Unmarshal(ev.Payload, &payload)
		}
		fillQty := payload.FillQuantity
		if fillQty <= 0 {
			continue
		}
		fills = append(fills, OMSFill{
			FillID:          fmt.Sprintf("%s:%s", ev.AggregateID, ev.EventType),
			ClientOrderID:   payload.ClientOrderID,
			ExchangeOrderID: payload.ExchangeOrderID,
			Symbol:          firstNonEmpty(payload.Symbol, ev.Symbol),
			Side:            payload.Side,
			Price:           payload.FillPrice,
			Quantity:        fillQty,
			Timestamp:       ev.CreatedAt,
		})
	}
	return fills
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
