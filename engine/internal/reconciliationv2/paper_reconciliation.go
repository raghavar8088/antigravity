package reconciliationv2

import (
	"context"
	"time"
)

// PaperReconciliationAdapter is the ReconciliationAdapter for the paper trading account.
// It returns the OMS snapshot as the "exchange" truth — for paper trading, the OMS
// is both the exchange simulation and the internal state, so no drift is expected.
// Used for testing and paper-only deployments.
type PaperReconciliationAdapter struct {
	reader    OMSStateReader
	accountID string
}

// NewPaperReconciliationAdapter creates a paper adapter that mirrors OMS as exchange.
func NewPaperReconciliationAdapter(reader OMSStateReader, accountID string) *PaperReconciliationAdapter {
	return &PaperReconciliationAdapter{reader: reader, accountID: accountID}
}

func (a *PaperReconciliationAdapter) Name() string { return "paper" }

func (a *PaperReconciliationAdapter) HealthCheck(ctx context.Context) error { return nil }

func (a *PaperReconciliationAdapter) GetAccountState(ctx context.Context) (AccountState, error) {
	snap, err := a.reader.GetOMSSnapshot(ctx, a.accountID)
	if err != nil {
		return AccountState{}, err
	}

	balances := []AssetBalance{{
		Asset:         "USD",
		WalletBalance: snap.Balance.EquityUSD,
		Available:     snap.Balance.AvailableUSD,
		MarginUsed:    snap.Balance.MarginUsedUSD,
		UnrealizedPnL: snap.Balance.UnrealizedPnL,
		EquityUSD:     snap.Balance.EquityUSD,
	}}

	positions := make([]ExchangePosition, 0, len(snap.Positions))
	for _, p := range snap.Positions {
		if p.State != "OPEN" && p.State != "REDUCED" {
			continue
		}
		positions = append(positions, ExchangePosition{
			Symbol:      p.Symbol,
			Side:        p.Side,
			Quantity:    p.Quantity,
			EntryPrice:  p.EntryPrice,
			NotionalUSD: p.NotionalUSD,
			UpdatedAt:   snap.SnapshotAt,
		})
	}

	orders := make([]ExchangeOrder, 0, len(snap.OpenOrders))
	for _, o := range snap.OpenOrders {
		orders = append(orders, ExchangeOrder{
			ClientOrderID:  o.ClientOrderID,
			ExchangeOrderID: o.ExchangeOrderID,
			Symbol:         o.Symbol,
			Side:           o.Side,
			Status:         o.State,
			Quantity:       o.Quantity,
			FilledQuantity: o.FilledQuantity,
			AverageFillPrice: o.AveragePrice,
			UpdatedAt:      o.UpdatedAt,
		})
	}

	return AccountState{
		Exchange:    a.Name(),
		AccountID:   a.accountID,
		Balances:    balances,
		Positions:   positions,
		OpenOrders:  orders,
		RetrievedAt: time.Now().UTC(),
	}, nil
}

func (a *PaperReconciliationAdapter) GetBalances(ctx context.Context) ([]AssetBalance, error) {
	state, err := a.GetAccountState(ctx)
	if err != nil {
		return nil, err
	}
	return state.Balances, nil
}

func (a *PaperReconciliationAdapter) GetPositions(ctx context.Context) ([]ExchangePosition, error) {
	state, err := a.GetAccountState(ctx)
	if err != nil {
		return nil, err
	}
	return state.Positions, nil
}

func (a *PaperReconciliationAdapter) GetOrders(ctx context.Context, symbol string, since time.Time) ([]ExchangeOrder, error) {
	state, err := a.GetAccountState(ctx)
	if err != nil {
		return nil, err
	}
	return state.OpenOrders, nil
}

func (a *PaperReconciliationAdapter) GetOpenOrders(ctx context.Context, symbol string) ([]ExchangeOrder, error) {
	return a.GetOrders(ctx, symbol, time.Time{})
}

func (a *PaperReconciliationAdapter) GetFills(ctx context.Context, symbol string, since time.Time) ([]ExchangeFill, error) {
	snap, err := a.reader.GetOMSSnapshot(ctx, a.accountID)
	if err != nil {
		return nil, err
	}
	result := make([]ExchangeFill, 0, len(snap.RecentFills))
	for _, f := range snap.RecentFills {
		if !since.IsZero() && f.Timestamp.Before(since) {
			continue
		}
		if symbol != "" && f.Symbol != symbol {
			continue
		}
		result = append(result, ExchangeFill{
			FillID:          f.FillID,
			ExchangeOrderID: f.ExchangeOrderID,
			ClientOrderID:   f.ClientOrderID,
			Symbol:          f.Symbol,
			Side:            f.Side,
			Price:           f.Price,
			Quantity:        f.Quantity,
			FeeUSD:          f.FeeUSD,
			Timestamp:       f.Timestamp,
		})
	}
	return result, nil
}

func (a *PaperReconciliationAdapter) GetTrades(ctx context.Context, symbol string, since time.Time) ([]ExchangeTrade, error) {
	return nil, nil
}

func (a *PaperReconciliationAdapter) GetFunding(ctx context.Context, symbol string, since time.Time) ([]FundingPayment, error) {
	return nil, nil
}
