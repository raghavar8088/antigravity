package reconciliationv2

import (
	"context"
	"time"
)

// AccountState is the complete exchange account snapshot.
// The exchange is the single source of truth; this struct is authoritative.
type AccountState struct {
	Exchange    string             `json:"exchange"`
	AccountID   string             `json:"account_id"`
	Balances    []AssetBalance     `json:"balances"`
	Positions   []ExchangePosition `json:"positions"`
	OpenOrders  []ExchangeOrder    `json:"open_orders"`
	RetrievedAt time.Time          `json:"retrieved_at"`
}

// AssetBalance is a single asset balance as reported by the exchange.
type AssetBalance struct {
	Asset         string  `json:"asset"`
	WalletBalance float64 `json:"wallet_balance"`
	Available     float64 `json:"available"`
	MarginUsed    float64 `json:"margin_used"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	RealizedPnL   float64 `json:"realized_pnl"`
	EquityUSD     float64 `json:"equity_usd"`
}

// ExchangePosition is a position as authoritatively reported by the exchange.
type ExchangePosition struct {
	Symbol           string    `json:"symbol"`
	Side             string    `json:"side"` // LONG|SHORT|BOTH
	Quantity         float64   `json:"quantity"`
	EntryPrice       float64   `json:"entry_price"`
	MarkPrice        float64   `json:"mark_price"`
	LiquidationPrice float64   `json:"liquidation_price"`
	Leverage         float64   `json:"leverage"`
	NotionalUSD      float64   `json:"notional_usd"`
	MarginUsed       float64   `json:"margin_used"`
	UnrealizedPnL    float64   `json:"unrealized_pnl"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ExchangeOrder is an order as reported by the exchange.
type ExchangeOrder struct {
	ExchangeOrderID  string    `json:"exchange_order_id"`
	ClientOrderID    string    `json:"client_order_id"`
	Symbol           string    `json:"symbol"`
	Side             string    `json:"side"`
	Status           string    `json:"status"`
	OrderType        string    `json:"order_type"`
	Quantity         float64   `json:"quantity"`
	FilledQuantity   float64   `json:"filled_quantity"`
	RemainingQty     float64   `json:"remaining_qty"`
	Price            float64   `json:"price"`
	AverageFillPrice float64   `json:"average_fill_price"`
	FeesUSD          float64   `json:"fees_usd"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ExchangeFill is an account fill (execution) as reported by the exchange.
type ExchangeFill struct {
	FillID          string    `json:"fill_id"`
	ExchangeOrderID string    `json:"exchange_order_id"`
	ClientOrderID   string    `json:"client_order_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	Price           float64   `json:"price"`
	Quantity        float64   `json:"quantity"`
	FeeUSD          float64   `json:"fee_usd"`
	FeeAsset        string    `json:"fee_asset"`
	IsMaker         bool      `json:"is_maker"`
	Timestamp       time.Time `json:"timestamp"`
}

// ExchangeTrade is a public market trade (not account-specific).
type ExchangeTrade struct {
	TradeID   string    `json:"trade_id"`
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"`
	Price     float64   `json:"price"`
	Quantity  float64   `json:"quantity"`
	Timestamp time.Time `json:"timestamp"`
}

// FundingPayment is a perpetual contract funding payment.
type FundingPayment struct {
	Symbol    string    `json:"symbol"`
	Rate      float64   `json:"rate"`
	AmountUSD float64   `json:"amount_usd"`
	PaidAt    time.Time `json:"paid_at"`
}

// OMSOrder is an order as tracked by OMS v3 internally.
type OMSOrder struct {
	ClientOrderID   string    `json:"client_order_id"`
	ExchangeOrderID string    `json:"exchange_order_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	State           string    `json:"state"`
	Quantity        float64   `json:"quantity"`
	FilledQuantity  float64   `json:"filled_quantity"`
	AveragePrice    float64   `json:"average_price"`
	FeesUSD         float64   `json:"fees_usd"`
	StrategyName    string    `json:"strategy_name"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// OMSPosition is a position as tracked by OMS v3 internally.
type OMSPosition struct {
	PositionID    string  `json:"position_id"`
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	State         string  `json:"state"`
	EntryPrice    float64 `json:"entry_price"`
	Quantity      float64 `json:"quantity"`
	NotionalUSD   float64 `json:"notional_usd"`
	UnrealPnL     float64 `json:"unreal_pnl_usd"`
	StopLoss      float64 `json:"stop_loss"`
	TakeProfit    float64 `json:"take_profit"`
	StrategyName  string  `json:"strategy_name"`
}

// OMSFill is a fill as recorded in OMS v3.
type OMSFill struct {
	FillID          string    `json:"fill_id"`
	ClientOrderID   string    `json:"client_order_id"`
	ExchangeOrderID string    `json:"exchange_order_id"`
	Symbol          string    `json:"symbol"`
	Side            string    `json:"side"`
	Price           float64   `json:"price"`
	Quantity        float64   `json:"quantity"`
	FeeUSD          float64   `json:"fee_usd"`
	Timestamp       time.Time `json:"timestamp"`
}

// OMSBalanceSnapshot is the balance view from OMS/ledger projections.
type OMSBalanceSnapshot struct {
	EquityUSD     float64 `json:"equity_usd"`
	AvailableUSD  float64 `json:"available_usd"`
	MarginUsedUSD float64 `json:"margin_used_usd"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	RealizedPnL   float64 `json:"realized_pnl"`
}

// OMSSnapshot is the complete internal state used for reconciliation comparisons.
type OMSSnapshot struct {
	AccountID     string             `json:"account_id"`
	Balance       OMSBalanceSnapshot `json:"balance"`
	Positions     []OMSPosition      `json:"positions"`
	OpenOrders    []OMSOrder         `json:"open_orders"`
	RecentFills   []OMSFill          `json:"recent_fills"`
	GrossNotional float64            `json:"gross_notional_usd"`
	NetNotional   float64            `json:"net_notional_usd"`
	SnapshotAt    time.Time          `json:"snapshot_at"`
}

// OMSStateReader provides the internal OMS/ledger state for reconciliation comparison.
// Implemented by the caller, typically wrapping omsv3.Authority.
type OMSStateReader interface {
	GetOMSSnapshot(ctx context.Context, accountID string) (OMSSnapshot, error)
}

// ReconciliationAdapter is the standard interface all exchanges must implement
// to participate in the Phase 15E Exchange Reconciliation Authority.
type ReconciliationAdapter interface {
	// Name returns the canonical exchange identifier (e.g. "binance", "delta", "paper").
	Name() string

	// GetAccountState returns the complete account snapshot for full audits.
	GetAccountState(ctx context.Context) (AccountState, error)

	// GetBalances returns all asset balances.
	GetBalances(ctx context.Context) ([]AssetBalance, error)

	// GetPositions returns all open positions.
	GetPositions(ctx context.Context) ([]ExchangePosition, error)

	// GetOrders returns orders updated since 'since'. Empty symbol = all symbols.
	GetOrders(ctx context.Context, symbol string, since time.Time) ([]ExchangeOrder, error)

	// GetOpenOrders returns all currently open/active orders.
	GetOpenOrders(ctx context.Context, symbol string) ([]ExchangeOrder, error)

	// GetFills returns account fills since 'since'. Empty symbol = all symbols.
	GetFills(ctx context.Context, symbol string, since time.Time) ([]ExchangeFill, error)

	// GetTrades returns public market trades since 'since'.
	GetTrades(ctx context.Context, symbol string, since time.Time) ([]ExchangeTrade, error)

	// GetFunding returns funding rate payments since 'since'.
	GetFunding(ctx context.Context, symbol string, since time.Time) ([]FundingPayment, error)

	// HealthCheck verifies the adapter can reach the exchange.
	HealthCheck(ctx context.Context) error
}
