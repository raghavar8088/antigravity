package omsv3

import (
	"context"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/reconciliation"
)

// LedgerSnapshotProvider implements reconciliation.SnapshotProvider by replaying
// all ledger events for an account and converting the resulting projections into
// the reconciliation.Snapshot format.
//
// For paper trading, ExchangePositions and ExchangeOrders mirror the OMS
// projections (there is no real exchange to query). When wired to a live
// exchange adapter the Snapshot method should be overridden to call the real
// exchange REST API.
type LedgerSnapshotProvider struct {
	store     ledger.Store
	accountID string
}

// NewLedgerSnapshotProvider creates a snapshot provider backed by the given
// ledger store for the given account.
func NewLedgerSnapshotProvider(store ledger.Store, accountID string) *LedgerSnapshotProvider {
	if accountID == "" {
		accountID = "default"
	}
	return &LedgerSnapshotProvider{store: store, accountID: accountID}
}

// Snapshot implements reconciliation.SnapshotProvider.
// It replays all ledger events for the account, builds CQRS projections,
// and returns a reconciliation.Snapshot.
func (p *LedgerSnapshotProvider) Snapshot(ctx context.Context) (reconciliation.Snapshot, error) {
	if p.store == nil {
		return reconciliation.Snapshot{}, fmt.Errorf("omsv3.LedgerSnapshotProvider: store is nil")
	}

	events, err := p.store.ReplayAccount(ctx, p.accountID)
	if err != nil {
		return reconciliation.Snapshot{}, fmt.Errorf("omsv3.LedgerSnapshotProvider.Snapshot: %w", err)
	}

	// Build projections from events
	openPos := BuildOpenPositionProjections(events)
	orderProjs := BuildOrderProjections(events)

	// Convert OMS position projections to reconciliation types
	omsPositions := make([]reconciliation.OMSPosition, 0, len(openPos))
	exPositions := make([]reconciliation.ExchangePosition, 0, len(openPos))
	for _, pos := range openPos {
		omsPositions = append(omsPositions, reconciliation.OMSPosition{
			Symbol:   pos.Symbol,
			Side:     pos.Side,
			Quantity: pos.Quantity,
		})
		// For paper trading, exchange mirrors OMS (no real exchange to query).
		exPositions = append(exPositions, reconciliation.ExchangePosition{
			Symbol:   pos.Symbol,
			Side:     pos.Side,
			Quantity: pos.Quantity,
		})
	}

	// Convert live OMS orders to reconciliation types
	omsOrders := make([]reconciliation.OMSOrder, 0, len(orderProjs))
	for _, order := range orderProjs {
		if !isLiveOrderState(order.State) {
			continue
		}
		omsOrders = append(omsOrders, reconciliation.OMSOrder{
			ClientOrderID:   order.ClientOrderID,
			ExchangeOrderID: order.ExchangeOrderID,
			Symbol:          order.Symbol,
			Side:            order.Side,
			State:           reconciliation.OrderState(order.State),
			Quantity:        order.Quantity,
			FilledQuantity:  order.FilledQuantity,
			UpdatedAt:       time.Now().UTC(), // MemoryStore has no UpdatedAt; use now as approximation
		})
	}

	// Build PnL projection for balance snapshot
	pnl := BuildPnLProjection(events)
	omsBalance := reconciliation.BalanceSnapshot{
		EquityUSD: pnl.TotalPnLUSD,
		CashUSD:   pnl.TotalPnLUSD,
	}

	return reconciliation.Snapshot{
		OMSOrders:         omsOrders,
		ExchangeOrders:    []reconciliation.ExchangeOrder{}, // paper: exchange = OMS
		OMSPositions:      omsPositions,
		ExchangePositions: exPositions,
		OMSBalance:        omsBalance,
		ExchangeBalance:   omsBalance, // paper: exchange balance = OMS balance
		AccountID:         p.accountID,
	}, nil
}

func isLiveOrderState(state string) bool {
	return state == string(ledger.OrderStateSubmitted) ||
		state == string(ledger.OrderStateAcknowledged) ||
		state == string(ledger.OrderStatePartialFill)
}
