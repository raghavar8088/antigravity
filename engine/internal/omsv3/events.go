package omsv3

// OrderCreatedPayload is attached to EventOrderCreated.
// Carries all fields needed to reconstruct the order intent from the event log alone.
type OrderCreatedPayload struct {
	ClientOrderID string  `json:"client_order_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Quantity      float64 `json:"quantity"`
	NotionalUSD   float64 `json:"notional_usd"`
	Leverage      float64 `json:"leverage"`
	OrderType     string  `json:"order_type"`
	StrategyName  string  `json:"strategy_name"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
}

// OrderFillPayload is attached to EventOrderFilled and EventOrderPartial.
type OrderFillPayload struct {
	ClientOrderID   string  `json:"client_order_id"`
	ExchangeOrderID string  `json:"exchange_order_id"`
	FillPrice       float64 `json:"fill_price"`
	FillQuantity    float64 `json:"fill_quantity"`
	FeeUSD          float64 `json:"fee_usd"`
	SlippageBps     float64 `json:"slippage_bps"`
}

// OrderRejectedPayload is attached to EventOrderRejected and EventRiskBlocked.
type OrderRejectedPayload struct {
	Reason     string `json:"reason"`
	PolicyName string `json:"policy_name,omitempty"`
}

// PositionOpenedPayload is attached to EventPositionOpened.
// Emitted by the trading orchestrator after a successful order fill opens a
// tracked position. Links the position to its originating ClientOrderID so
// the two aggregates (order + position) can be correlated during replay.
type PositionOpenedPayload struct {
	ClientOrderID string  `json:"client_order_id"`
	PositionID    string  `json:"position_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"` // BUY | SELL | LONG | SHORT
	EntryPrice    float64 `json:"entry_price"`
	Quantity      float64 `json:"quantity"`
	NotionalUSD   float64 `json:"notional_usd"`
	MarginUsed    float64 `json:"margin_used"`
	Leverage      float64 `json:"leverage"`
	StopLoss      float64 `json:"stop_loss"`
	TakeProfit    float64 `json:"take_profit"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
	StrategyName  string  `json:"strategy_name"`
	EntryFeeUSD   float64 `json:"entry_fee_usd"`
	SlippageBps   float64 `json:"slippage_bps"`
}

// PositionClosedPayload is attached to EventPositionClosed.
// Contains the full P&L breakdown so the ledger is the only source of truth
// for historical performance — no need to query a separate analytics store.
type PositionClosedPayload struct {
	ClientOrderID string  `json:"client_order_id"`
	PositionID    string  `json:"position_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	EntryPrice    float64 `json:"entry_price"`
	ExitPrice     float64 `json:"exit_price"`
	Quantity      float64 `json:"quantity"`
	NotionalUSD   float64 `json:"notional_usd"`
	GrossPnLUSD   float64 `json:"gross_pnl_usd"`
	NetPnLUSD     float64 `json:"net_pnl_usd"`
	FeesUSD       float64 `json:"fees_usd"`
	// ExitReason: TP | SL | TIME | TRAIL | BREAKEVEN | MANUAL | KILL_SWITCH
	ExitReason   string  `json:"exit_reason"`
	StrategyName string  `json:"strategy_name"`
	HoldMinutes  float64 `json:"hold_minutes"`
}

// PositionChangedPayload is attached to EventPositionChanged (partial close).
type PositionChangedPayload struct {
	PositionID       string  `json:"position_id"`
	ClientOrderID    string  `json:"client_order_id"`
	PartialExitPrice float64 `json:"partial_exit_price"`
	PartialQuantity  float64 `json:"partial_quantity"`
	RemainingQty     float64 `json:"remaining_quantity"`
	PartialPnLUSD    float64 `json:"partial_pnl_usd"`
}
