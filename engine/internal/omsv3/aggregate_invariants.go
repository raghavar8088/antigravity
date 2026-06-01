package omsv3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// ── Command pattern ───────────────────────────────────────────────────────────
//
// All state changes in the event-sourced system MUST flow through this path:
//
//   Command → CommandBus.Dispatch() → Aggregate.Apply() → ledger.Append()
//
// Services that bypass this path and mutate structs directly will produce state
// that cannot be replayed, breaking crash recovery and auditability.

// Command is the interface every write intent must implement before state
// transitions are allowed. Implementing Command forces the caller to declare:
// - which aggregate they intend to mutate
// - what validation preconditions must hold before mutation
type Command interface {
	// AggregateType identifies the owning aggregate (ORDER, POSITION, etc.).
	AggregateType() ledger.AggregateType
	// AggregateID identifies the specific aggregate instance.
	AggregateID() string
	// Validate returns an error if the command is internally inconsistent
	// (e.g., negative quantity, missing required fields).
	Validate() error
	// EventType returns the ledger event type this command produces when accepted.
	EventType() ledger.EventType
	// Payload returns the event payload (any JSON-serialisable value).
	Payload() any
}

// AggregateOwnershipMap declares which aggregate type owns which event types.
// Attempting to emit an event for an aggregate that doesn't own it is rejected.
var AggregateOwnershipMap = map[ledger.EventType]ledger.AggregateType{
	// ORDER aggregate
	ledger.EventOrderCreated:          ledger.AggregateOrder,
	ledger.EventOrderValidated:        ledger.AggregateOrder,
	ledger.EventOrderAccepted:         ledger.AggregateOrder,
	ledger.EventOrderSubmitted:        ledger.AggregateOrder,
	ledger.EventOrderAcked:            ledger.AggregateOrder,
	ledger.EventOrderPartial:          ledger.AggregateOrder,
	ledger.EventOrderFilled:           ledger.AggregateOrder,
	ledger.EventOrderCancelled:        ledger.AggregateOrder,
	ledger.EventOrderRejected:         ledger.AggregateOrder,
	ledger.EventOrderExpired:          ledger.AggregateOrder,
	ledger.EventOrderReplaceRequested: ledger.AggregateOrder,
	ledger.EventOrderReplaced:         ledger.AggregateOrder,
	ledger.EventRiskApproved:          ledger.AggregateOrder, // risk approval moves order to RISK_APPROVED
	ledger.EventRiskBlocked:           ledger.AggregateOrder,

	// POSITION aggregate
	ledger.EventPositionOpened:          ledger.AggregatePosition,
	ledger.EventPositionScaled:          ledger.AggregatePosition,
	ledger.EventPositionChanged:         ledger.AggregatePosition,
	ledger.EventPositionReduced:         ledger.AggregatePosition,
	ledger.EventPositionClosed:          ledger.AggregatePosition,
	ledger.EventPositionLiquidated:      ledger.AggregatePosition,
	ledger.EventPositionTransferred:     ledger.AggregatePosition,
	ledger.EventPositionSLMoved:         ledger.AggregatePosition,
	ledger.EventPositionBreakevenActivated: ledger.AggregatePosition,
	ledger.EventPositionTrailingActivated:  ledger.AggregatePosition,
	ledger.EventPositionTPMoved:         ledger.AggregatePosition,
	ledger.EventPositionRiskAdjusted:    ledger.AggregatePosition,

	// RISK aggregate
	ledger.EventRiskCheckStarted:           ledger.AggregateRisk,
	ledger.EventRiskViolation:              ledger.AggregateRisk,
	ledger.EventRiskTriggered:              ledger.AggregateRisk,
	ledger.EventExposureLimitExceeded:      ledger.AggregateRisk,
	ledger.EventPortfolioHeatExceeded:      ledger.AggregateRisk,
	ledger.EventVaRBreach:                  ledger.AggregateRisk,
	ledger.EventCVaRBreach:                 ledger.AggregateRisk,
	ledger.EventMaxDrawdownBreached:        ledger.AggregateRisk,
	ledger.EventFundingExposureExceeded:    ledger.AggregateRisk,
	ledger.EventRiskDailyLossLimitExceeded: ledger.AggregateRisk,
	ledger.EventRiskMarginViolation:        ledger.AggregateRisk,
	ledger.EventRiskLeverageViolation:      ledger.AggregateRisk,
	ledger.EventRiskConcentrationViolation: ledger.AggregateRisk,
	ledger.EventRiskCorrelationViolation:   ledger.AggregateRisk,
	ledger.EventKillSwitchTriggered:        ledger.AggregateRisk,
	ledger.EventKillSwitchReleased:         ledger.AggregateRisk,

	// STRATEGY aggregate
	ledger.EventStrategyRegistered:        ledger.AggregateStrategy,
	ledger.EventStrategyEnabled:           ledger.AggregateStrategy,
	ledger.EventStrategyDisabled:          ledger.AggregateStrategy,
	ledger.EventStrategyPaused:            ledger.AggregateStrategy,
	ledger.EventStrategyResumed:           ledger.AggregateStrategy,
	ledger.EventStrategyPromoted:          ledger.AggregateStrategy,
	ledger.EventStrategyDemoted:           ledger.AggregateStrategy,
	ledger.EventStrategyAllocationChanged: ledger.AggregateStrategy,

	// EXCHANGE aggregate
	ledger.EventExchangeConnected:       ledger.AggregateExchange,
	ledger.EventExchangeDisconnected:    ledger.AggregateExchange,
	ledger.EventExchangeReconnected:     ledger.AggregateExchange,
	ledger.EventExchangeOrderRejected:   ledger.AggregateExchange,
	ledger.EventExchangeLatencySpike:    ledger.AggregateExchange,
	ledger.EventExchangeRateLimitHit:    ledger.AggregateExchange,
	ledger.EventExchangeDataGapDetected: ledger.AggregateExchange,
	ledger.EventExchangeOutage:          ledger.AggregateExchange,
	ledger.EventMarketDataStale:         ledger.AggregateExchange,
	ledger.EventMarketDataRecovered:     ledger.AggregateExchange,

	// SYSTEM aggregate
	ledger.EventEngineStarting:     ledger.AggregateSystem,
	ledger.EventEngineStarted:      ledger.AggregateSystem,
	ledger.EventEngineStopping:     ledger.AggregateSystem,
	ledger.EventEngineStopped:      ledger.AggregateSystem,
	ledger.EventReplayStarted:      ledger.AggregateSystem,
	ledger.EventReplayCompleted:    ledger.AggregateSystem,
	ledger.EventProjectionRebuilt:  ledger.AggregateSystem,
	ledger.EventSnapshotCreated:    ledger.AggregateSystem,
	ledger.EventSnapshotRestored:   ledger.AggregateSystem,

	// RECONCILIATION aggregate
	ledger.EventReconciliationStarted:  ledger.AggregateReconciliation,
	ledger.EventReconciliationMismatch: ledger.AggregateReconciliation,
	ledger.EventReconciliationAlert:    ledger.AggregateReconciliation,
	ledger.EventReconciliationCorrected: ledger.AggregateReconciliation,
	ledger.EventReconciliationResolved: ledger.AggregateReconciliation,
}

// ErrAggregateOwnershipViolation is returned when an event type is emitted
// under the wrong aggregate type.
var ErrAggregateOwnershipViolation = errors.New("omsv3: aggregate ownership violation")

// ValidateEventOwnership checks that event.EventType is owned by event.AggregateType.
// Returns ErrAggregateOwnershipViolation if the mapping is incorrect.
func ValidateEventOwnership(event ledger.Event) error {
	expected, known := AggregateOwnershipMap[event.EventType]
	if !known {
		// Unknown event types are not validated (forward-compatibility).
		return nil
	}
	if expected != event.AggregateType {
		return fmt.Errorf("%w: event %s belongs to %s aggregate, got %s",
			ErrAggregateOwnershipViolation, event.EventType, expected, event.AggregateType)
	}
	return nil
}

// ── CommandBus ────────────────────────────────────────────────────────────────

// CommandBus is the canonical write path for all aggregate state changes.
// It enforces:
//  1. Aggregate ownership (event type must belong to declared aggregate)
//  2. Command validation (Validate() must pass before append)
//  3. Immutable append (events are written to ledger before in-memory Apply)
//
// Usage:
//
//	bus := NewCommandBus(store, "btc-paper-1")
//	result, err := bus.Dispatch(ctx, &CreateOrderCommand{...})
type CommandBus struct {
	store     ledger.Store
	accountID string
}

// NewCommandBus creates a CommandBus backed by the given store.
func NewCommandBus(store ledger.Store, accountID string) *CommandBus {
	return &CommandBus{store: store, accountID: accountID}
}

// DispatchResult holds the ledger event produced by a dispatched command.
type DispatchResult struct {
	Event ledger.Event
}

// Dispatch validates the command, enforces aggregate ownership, appends the
// resulting event to the ledger, and returns it. The caller is responsible for
// calling the aggregate's ApplyEvent() with the returned event.
func (b *CommandBus) Dispatch(ctx context.Context, cmd Command) (DispatchResult, error) {
	if err := cmd.Validate(); err != nil {
		return DispatchResult{}, fmt.Errorf("omsv3.CommandBus.Dispatch: validation: %w", err)
	}

	// Build a candidate event before ownership check so the error message is rich.
	candidate := ledger.Event{
		AggregateType: cmd.AggregateType(),
		AggregateID:   cmd.AggregateID(),
		EventType:     cmd.EventType(),
		AccountID:     b.accountID,
	}
	if err := ValidateEventOwnership(candidate); err != nil {
		return DispatchResult{}, fmt.Errorf("omsv3.CommandBus.Dispatch: %w", err)
	}

	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: cmd.AggregateType(),
		AggregateID:   cmd.AggregateID(),
		EventType:     cmd.EventType(),
		AccountID:     b.accountID,
		Payload:       cmd.Payload(),
		Source:        "command-bus",
		CreatedAt:     time.Now().UTC(),
	})
	if err != nil {
		return DispatchResult{}, fmt.Errorf("omsv3.CommandBus.Dispatch: build event: %w", err)
	}

	stored, err := b.store.Append(ctx, event)
	if err != nil {
		return DispatchResult{}, fmt.Errorf("omsv3.CommandBus.Dispatch: append: %w", err)
	}
	return DispatchResult{Event: stored}, nil
}

// ── Canonical command implementations ────────────────────────────────────────

// CreateOrderCommand represents the intent to create a new order.
type CreateOrderCommand struct {
	ClientOrderID string
	AccountID     string
	Symbol        string
	Side          string  // BUY | SELL
	Quantity      float64
	NotionalUSD   float64
	Leverage      float64
	OrderType     string
	StrategyName  string
	StopLossPct   float64
	TakeProfitPct float64
}

func (c *CreateOrderCommand) AggregateType() ledger.AggregateType { return ledger.AggregateOrder }
func (c *CreateOrderCommand) AggregateID() string                  { return c.ClientOrderID }
func (c *CreateOrderCommand) EventType() ledger.EventType          { return ledger.EventOrderCreated }
func (c *CreateOrderCommand) Payload() any {
	return map[string]any{
		"client_order_id": c.ClientOrderID,
		"symbol":          c.Symbol,
		"side":            c.Side,
		"quantity":        c.Quantity,
		"notional_usd":    c.NotionalUSD,
		"leverage":        c.Leverage,
		"order_type":      c.OrderType,
		"strategy_name":   c.StrategyName,
		"stop_loss_pct":   c.StopLossPct,
		"take_profit_pct": c.TakeProfitPct,
	}
}
func (c *CreateOrderCommand) Validate() error {
	if c.ClientOrderID == "" {
		return errors.New("CreateOrderCommand: ClientOrderID is required")
	}
	if c.Symbol == "" {
		return errors.New("CreateOrderCommand: Symbol is required")
	}
	if c.Side != "BUY" && c.Side != "SELL" && c.Side != "LONG" && c.Side != "SHORT" {
		return fmt.Errorf("CreateOrderCommand: invalid side %q", c.Side)
	}
	if c.Quantity <= 0 && c.NotionalUSD <= 0 {
		return errors.New("CreateOrderCommand: Quantity or NotionalUSD must be > 0")
	}
	return nil
}

// FillOrderCommand records a partial or full fill event.
type FillOrderCommand struct {
	ClientOrderID   string
	ExchangeOrderID string
	FillPrice       float64
	FillQuantity    float64
	FeeUSD          float64
	SlippageBps     float64
	IsFinal         bool // true → EventOrderFilled, false → EventOrderPartial
}

func (c *FillOrderCommand) AggregateType() ledger.AggregateType { return ledger.AggregateOrder }
func (c *FillOrderCommand) AggregateID() string                  { return c.ClientOrderID }
func (c *FillOrderCommand) EventType() ledger.EventType {
	if c.IsFinal {
		return ledger.EventOrderFilled
	}
	return ledger.EventOrderPartial
}
func (c *FillOrderCommand) Payload() any {
	return map[string]any{
		"client_order_id":   c.ClientOrderID,
		"exchange_order_id": c.ExchangeOrderID,
		"fill_price":        c.FillPrice,
		"fill_quantity":     c.FillQuantity,
		"fee_usd":           c.FeeUSD,
		"slippage_bps":      c.SlippageBps,
	}
}
func (c *FillOrderCommand) Validate() error {
	if c.ClientOrderID == "" {
		return errors.New("FillOrderCommand: ClientOrderID is required")
	}
	if c.FillPrice <= 0 {
		return fmt.Errorf("FillOrderCommand: FillPrice must be > 0, got %.6f", c.FillPrice)
	}
	if c.FillQuantity <= 0 {
		return fmt.Errorf("FillOrderCommand: FillQuantity must be > 0, got %.6f", c.FillQuantity)
	}
	return nil
}

// CancelOrderCommand cancels an open order.
type CancelOrderCommand struct {
	ClientOrderID string
	Reason        string
}

func (c *CancelOrderCommand) AggregateType() ledger.AggregateType { return ledger.AggregateOrder }
func (c *CancelOrderCommand) AggregateID() string                  { return c.ClientOrderID }
func (c *CancelOrderCommand) EventType() ledger.EventType          { return ledger.EventOrderCancelled }
func (c *CancelOrderCommand) Payload() any {
	return map[string]any{"client_order_id": c.ClientOrderID, "reason": c.Reason}
}
func (c *CancelOrderCommand) Validate() error {
	if c.ClientOrderID == "" {
		return errors.New("CancelOrderCommand: ClientOrderID is required")
	}
	return nil
}

// OpenPositionCommand records that a position has been opened from a fill.
type OpenPositionCommand struct {
	ClientOrderID string
	PositionID    string
	Symbol        string
	Side          string
	EntryPrice    float64
	Quantity      float64
	NotionalUSD   float64
	MarginUsed    float64
	Leverage      float64
	StopLoss      float64
	TakeProfit    float64
	StopLossPct   float64
	TakeProfitPct float64
	LiqPrice      float64
	StrategyName  string
	EntryFeeUSD   float64
	SlippageBps   float64
}

func (c *OpenPositionCommand) AggregateType() ledger.AggregateType { return ledger.AggregatePosition }
func (c *OpenPositionCommand) AggregateID() string                  { return c.PositionID }
func (c *OpenPositionCommand) EventType() ledger.EventType          { return ledger.EventPositionOpened }
func (c *OpenPositionCommand) Payload() any {
	return ledger.PositionOpenedPayload{
		ClientOrderID: c.ClientOrderID,
		PositionID:    c.PositionID,
		Symbol:        c.Symbol,
		Side:          c.Side,
		EntryPrice:    c.EntryPrice,
		Quantity:      c.Quantity,
		NotionalUSD:   c.NotionalUSD,
		MarginUsed:    c.MarginUsed,
		Leverage:      c.Leverage,
		StopLoss:      c.StopLoss,
		TakeProfit:    c.TakeProfit,
		StopLossPct:   c.StopLossPct,
		TakeProfitPct: c.TakeProfitPct,
		LiqPrice:      c.LiqPrice,
		StrategyName:  c.StrategyName,
		EntryFeeUSD:   c.EntryFeeUSD,
		SlippageBps:   c.SlippageBps,
	}
}
func (c *OpenPositionCommand) Validate() error {
	if c.PositionID == "" {
		return errors.New("OpenPositionCommand: PositionID is required")
	}
	if c.Symbol == "" {
		return errors.New("OpenPositionCommand: Symbol is required")
	}
	if c.EntryPrice <= 0 {
		return fmt.Errorf("OpenPositionCommand: EntryPrice must be > 0, got %.6f", c.EntryPrice)
	}
	if c.Quantity <= 0 {
		return fmt.Errorf("OpenPositionCommand: Quantity must be > 0, got %.6f", c.Quantity)
	}
	return nil
}

// ClosePositionCommand records that a position has been closed.
type ClosePositionCommand struct {
	ClientOrderID string
	PositionID    string
	Symbol        string
	Side          string
	EntryPrice    float64
	ExitPrice     float64
	Quantity      float64
	NotionalUSD   float64
	GrossPnLUSD   float64
	NetPnLUSD     float64
	FeesUSD       float64
	ExitReason    string
	StrategyName  string
	HoldMinutes   float64
	IsLiquidation bool
}

func (c *ClosePositionCommand) AggregateType() ledger.AggregateType { return ledger.AggregatePosition }
func (c *ClosePositionCommand) AggregateID() string                  { return c.PositionID }
func (c *ClosePositionCommand) EventType() ledger.EventType {
	if c.IsLiquidation {
		return ledger.EventPositionLiquidated
	}
	return ledger.EventPositionClosed
}
func (c *ClosePositionCommand) Payload() any {
	return ledger.PositionClosedPayload{
		ClientOrderID: c.ClientOrderID,
		PositionID:    c.PositionID,
		Symbol:        c.Symbol,
		Side:          c.Side,
		EntryPrice:    c.EntryPrice,
		ExitPrice:     c.ExitPrice,
		Quantity:      c.Quantity,
		NotionalUSD:   c.NotionalUSD,
		GrossPnLUSD:   c.GrossPnLUSD,
		NetPnLUSD:     c.NetPnLUSD,
		FeesUSD:       c.FeesUSD,
		ExitReason:    c.ExitReason,
		StrategyName:  c.StrategyName,
		HoldMinutes:   c.HoldMinutes,
	}
}
func (c *ClosePositionCommand) Validate() error {
	if c.PositionID == "" {
		return errors.New("ClosePositionCommand: PositionID is required")
	}
	if c.ExitPrice <= 0 {
		return fmt.Errorf("ClosePositionCommand: ExitPrice must be > 0, got %.6f", c.ExitPrice)
	}
	if c.ExitReason == "" {
		return errors.New("ClosePositionCommand: ExitReason is required")
	}
	return nil
}
