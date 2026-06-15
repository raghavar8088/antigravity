package reconciliationv2

import (
	"context"
	"strings"
	"time"

	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/strategy"
)

// PaperRuntimeExchangeName identifies the in-process paper runtime adapter.
// Balance reconciliation is skipped for this exchange — equity is derived from
// two different accounting paths (paper client vs ledger projection) and must
// not halt trading; position/order integrity checks still run.
const PaperRuntimeExchangeName = "engine-runtime"

// PositionManagerExchangeAdapter treats the live in-process position manager and
// paper equity as the authoritative runtime exchange view. Compared against the
// ledger-backed OMS reader, this detects drift between event-sourced projections
// and the execution hot-path without mirroring OMS to itself.
type PositionManagerExchangeAdapter struct {
	posMgr       *positions.Manager
	equityUSD    func() float64
	markPriceUSD func() float64
	accountID    string
}

// NewPositionManagerExchangeAdapter wires runtime position/equity sources.
func NewPositionManagerExchangeAdapter(
	posMgr *positions.Manager,
	equityUSD func() float64,
	accountID string,
	markPriceUSD ...func() float64,
) *PositionManagerExchangeAdapter {
	var markFn func() float64
	if len(markPriceUSD) > 0 {
		markFn = markPriceUSD[0]
	}
	return &PositionManagerExchangeAdapter{
		posMgr:       posMgr,
		equityUSD:    equityUSD,
		markPriceUSD: markFn,
		accountID:    accountID,
	}
}

func (a *PositionManagerExchangeAdapter) Name() string { return PaperRuntimeExchangeName }

func (a *PositionManagerExchangeAdapter) HealthCheck(context.Context) error { return nil }

func (a *PositionManagerExchangeAdapter) GetAccountState(ctx context.Context) (AccountState, error) {
	balances, err := a.GetBalances(ctx)
	if err != nil {
		return AccountState{}, err
	}
	positions, err := a.GetPositions(ctx)
	if err != nil {
		return AccountState{}, err
	}
	return AccountState{
		Exchange:    a.Name(),
		AccountID:   a.accountID,
		Balances:    balances,
		Positions:   positions,
		OpenOrders:  nil,
		RetrievedAt: time.Now().UTC(),
	}, nil
}

func (a *PositionManagerExchangeAdapter) GetBalances(context.Context) ([]AssetBalance, error) {
	equity := 0.0
	if a.equityUSD != nil {
		equity = a.equityUSD()
	}
	unrealized := a.computeRuntimeUnrealizedPnL()
	return []AssetBalance{{
		Asset:         "USD",
		WalletBalance: equity,
		Available:     equity,
		EquityUSD:     equity,
		UnrealizedPnL: unrealized,
	}}, nil
}

func (a *PositionManagerExchangeAdapter) GetPositions(context.Context) ([]ExchangePosition, error) {
	if a.posMgr == nil {
		return nil, nil
	}
	open := a.posMgr.GetOpenPositions()
	result := make([]ExchangePosition, 0, len(open))
	for _, pos := range open {
		side := "LONG"
		if pos.Side == strategy.ActionSell {
			side = "SHORT"
		}
		mark := a.runtimeMarkPrice()
		notional := pos.Size * pos.EntryPrice
		unrealized := runtimePositionUnrealizedPnL(pos, mark)
		result = append(result, ExchangePosition{
			Symbol:        normalizeReconSymbol(pos.Symbol),
			Side:          side,
			Quantity:      pos.Size,
			EntryPrice:    pos.EntryPrice,
			MarkPrice:     mark,
			NotionalUSD:   notional,
			UnrealizedPnL: unrealized,
			UpdatedAt:     pos.OpenedAt,
		})
	}
	return result, nil
}

func (a *PositionManagerExchangeAdapter) runtimeMarkPrice() float64 {
	if a.markPriceUSD == nil {
		return 0
	}
	mark := a.markPriceUSD()
	if mark <= 0 {
		return 0
	}
	return mark
}

func (a *PositionManagerExchangeAdapter) computeRuntimeUnrealizedPnL() float64 {
	if a.posMgr == nil {
		return 0
	}
	mark := a.runtimeMarkPrice()
	if mark <= 0 {
		return 0
	}
	var unrealized float64
	for _, pos := range a.posMgr.GetOpenPositions() {
		unrealized += runtimePositionUnrealizedPnL(pos, mark)
	}
	return unrealized
}

func runtimePositionUnrealizedPnL(pos positions.Position, mark float64) float64 {
	if mark <= 0 || pos.Size <= 0 || pos.EntryPrice <= 0 {
		return 0
	}
	if pos.Side == strategy.ActionSell {
		return (pos.EntryPrice - mark) * pos.Size
	}
	return (mark - pos.EntryPrice) * pos.Size
}

func (a *PositionManagerExchangeAdapter) GetOrders(context.Context, string, time.Time) ([]ExchangeOrder, error) {
	return nil, nil
}

func (a *PositionManagerExchangeAdapter) GetOpenOrders(context.Context, string) ([]ExchangeOrder, error) {
	return nil, nil
}

func (a *PositionManagerExchangeAdapter) GetFills(context.Context, string, time.Time) ([]ExchangeFill, error) {
	return nil, nil
}

func (a *PositionManagerExchangeAdapter) GetTrades(context.Context, string, time.Time) ([]ExchangeTrade, error) {
	return nil, nil
}

func (a *PositionManagerExchangeAdapter) GetFunding(context.Context, string, time.Time) ([]FundingPayment, error) {
	return nil, nil
}

// normalizePositionSide uppercases and maps engine sides to reconciliation sides.
func normalizePositionSide(side string) string {
	s := strings.ToUpper(strings.TrimSpace(side))
	switch s {
	case "BUY", "LONG":
		return "LONG"
	case "SELL", "SHORT":
		return "SHORT"
	default:
		return s
	}
}

// positionSideKey builds a canonical symbol|side lookup key for drift detection.
func positionSideKey(symbol, side string) string {
	return normalizeReconSymbol(symbol) + "|" + normalizePositionSide(side)
}

// normalizeReconSymbol canonicalizes BTC symbol variants for comparison.
func normalizeReconSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	switch s {
	case "BTCUSDT", "BTC-USD", "BTCUSD", "XBTUSD":
		return "BTC-USD"
	default:
		return s
	}
}
