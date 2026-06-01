package ledger

import "time"

// EventEnvelope is the read-only view of any ledger event.
// Every Event struct satisfies this interface; it is used by replay and
// projection code that must be polymorphic over event sources.
type EventEnvelope interface {
	GetEventID() string
	GetAggregateID() string
	GetAggregateType() AggregateType
	GetEventType() EventType
	GetTimestamp() time.Time
	GetVersion() int64
}

// Implement EventEnvelope on the concrete Event struct.

func (e Event) GetEventID() string              { return e.EventID }
func (e Event) GetAggregateID() string          { return e.AggregateID }
func (e Event) GetAggregateType() AggregateType { return e.AggregateType }
func (e Event) GetEventType() EventType         { return e.EventType }
func (e Event) GetTimestamp() time.Time         { return e.CreatedAt }
func (e Event) GetVersion() int64               { return e.SequenceNo }

// ── Position payload types ─────────────────────────────────────────────────────

// PositionOpenedPayload is the canonical payload for EventPositionOpened.
// Also used by PaperOMS to log paper fills into the ledger.
type PositionOpenedPayload struct {
	ClientOrderID string  `json:"client_order_id"`
	PositionID    string  `json:"position_id"`
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	EntryPrice    float64 `json:"entry_price"`
	Quantity      float64 `json:"quantity"`
	NotionalUSD   float64 `json:"notional_usd"`
	MarginUsed    float64 `json:"margin_used"`
	Leverage      float64 `json:"leverage"`
	StopLoss      float64 `json:"stop_loss"`
	TakeProfit    float64 `json:"take_profit"`
	StopLossPct   float64 `json:"stop_loss_pct"`
	TakeProfitPct float64 `json:"take_profit_pct"`
	LiqPrice      float64 `json:"liq_price,omitempty"`
	StrategyName  string  `json:"strategy_name"`
	StrategyID    string  `json:"strategy_id,omitempty"`
	EntryFeeUSD   float64 `json:"entry_fee_usd"`
	SlippageBps   float64 `json:"slippage_bps"`
}

// PositionClosedPayload is the canonical payload for EventPositionClosed and
// EventPositionLiquidated. Contains the full round-trip P&L breakdown so the
// ledger is the single source of truth for historical performance.
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
	ExitReason    string  `json:"exit_reason"`
	StrategyName  string  `json:"strategy_name"`
	HoldMinutes   float64 `json:"hold_minutes"`
}

// PositionSLMovedPayload is attached to EventPositionSLMoved,
// EventPositionBreakevenActivated, and EventPositionTrailingActivated.
type PositionSLMovedPayload struct {
	PositionID  string  `json:"position_id"`
	Symbol      string  `json:"symbol"`
	PreviousSL  float64 `json:"previous_sl"`
	NewSL       float64 `json:"new_sl"`
	MarkPrice   float64 `json:"mark_price"`
	Reason      string  `json:"reason"` // "BREAKEVEN" | "TRAILING" | "MANUAL"
}

// ── Risk payload types ─────────────────────────────────────────────────────────

// RiskCheckPayload is attached to EventRiskApproved, EventRiskBlocked, and
// EventRiskViolation. Contains enough context to reconstruct the decision.
type RiskCheckPayload struct {
	Symbol              string  `json:"symbol"`
	Side                string  `json:"side"`
	RequestedSizeBTC    float64 `json:"requested_size_btc"`
	CurrentExposureBTC  float64 `json:"current_exposure_btc"`
	ProposedExposureBTC float64 `json:"proposed_exposure_btc"`
	CurrentPriceUSD     float64 `json:"current_price_usd"`
	ProposedNotionalUSD float64 `json:"proposed_notional_usd"`
	DailyPnLUSD         float64 `json:"daily_pnl_usd"`
	Confidence          float64 `json:"confidence,omitempty"`
	PolicyName          string  `json:"policy_name,omitempty"`
	ViolationReason     string  `json:"violation_reason,omitempty"`
}

// ── Strategy payload types ────────────────────────────────────────────────────

// StrategyLifecyclePayload is attached to all EventStrategy* events.
type StrategyLifecyclePayload struct {
	StrategyID     string  `json:"strategy_id"`
	StrategyName   string  `json:"strategy_name"`
	Family         string  `json:"family"`
	Module         string  `json:"module,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	PreviousState  string  `json:"previous_state,omitempty"`
	NewAllocation  float64 `json:"new_allocation,omitempty"`
	WinRate        float64 `json:"win_rate,omitempty"`
	ProfitFactor   float64 `json:"profit_factor,omitempty"`
}

// ── Exchange payload types ────────────────────────────────────────────────────

// ExchangeConnectivityPayload is attached to EventExchange* connectivity events.
type ExchangeConnectivityPayload struct {
	Exchange    string  `json:"exchange"`
	Symbol      string  `json:"symbol,omitempty"`
	Reason      string  `json:"reason,omitempty"`
	LatencyMs   float64 `json:"latency_ms,omitempty"`
	ErrorCode   string  `json:"error_code,omitempty"`
	GapStartSeq int64   `json:"gap_start_seq,omitempty"`
	GapEndSeq   int64   `json:"gap_end_seq,omitempty"`
}

// ── System payload types ──────────────────────────────────────────────────────

// SystemLifecyclePayload is attached to EventEngineStarted/Stopped,
// EventReplayStarted/Completed, EventSnapshotCreated/Restored,
// and EventProjectionRebuilt.
type SystemLifecyclePayload struct {
	Component      string `json:"component"`
	SnapshotID     string `json:"snapshot_id,omitempty"`
	EventCount     int64  `json:"event_count,omitempty"`
	AggregateCount int    `json:"aggregate_count,omitempty"`
	DurationMs     int64  `json:"duration_ms,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Version        string `json:"version,omitempty"`
}

// KillSwitchReleasedPayload is attached to EventKillSwitchReleased.
type KillSwitchReleasedPayload struct {
	OriginalTrigger string `json:"original_trigger"`
	ReleasedBy      string `json:"released_by"`
	Reason          string `json:"reason"`
}

// ReconciliationResolvedPayload is attached to EventReconciliationResolved.
// Records what was wrong, what the repair action was, and its result.
type ReconciliationResolvedPayload struct {
	AlertReference  string `json:"alert_reference"`
	AlertType       string `json:"alert_type"`
	ExpectedState   string `json:"expected_state"`
	ActualState     string `json:"actual_state"`
	RepairAction    string `json:"repair_action"`
	RepairResult    string `json:"repair_result"`
	ResolvedBy      string `json:"resolved_by"`
}
