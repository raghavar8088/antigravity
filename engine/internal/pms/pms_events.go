// Package pms implements the Institutional Portfolio Management System (Phase 17).
// It is the capital allocation authority above strategies, risk, OMS, and execution.
package pms

import (
	"antigravity-engine/internal/ledger"
	"context"
	"fmt"
	"time"
)

// ── PMS Aggregate Types ───────────────────────────────────────────────────────

const (
	AggregatePortfolio ledger.AggregateType = "PORTFOLIO"
	AggregateAccount   ledger.AggregateType = "PMS_ACCOUNT"
	AggregateBudget    ledger.AggregateType = "PMS_BUDGET"
)

// ── PMS Event Types ───────────────────────────────────────────────────────────

const (
	// Portfolio lifecycle
	EventPortfolioCreated ledger.EventType = "PORTFOLIO_CREATED"
	EventPortfolioUpdated ledger.EventType = "PORTFOLIO_UPDATED"
	EventPortfolioDeleted ledger.EventType = "PORTFOLIO_DELETED"
	EventPortfolioNAVUpdated ledger.EventType = "PORTFOLIO_NAV_UPDATED"

	// Allocation
	EventAllocationCreated ledger.EventType = "ALLOCATION_CREATED"
	EventAllocationChanged ledger.EventType = "ALLOCATION_CHANGED"
	EventAllocationRemoved ledger.EventType = "ALLOCATION_REMOVED"
	EventMasterAllocationChanged ledger.EventType = "MASTER_ALLOCATION_CHANGED"

	// Risk budgets
	EventRiskBudgetCreated ledger.EventType = "RISK_BUDGET_CREATED"
	EventRiskBudgetChanged ledger.EventType = "RISK_BUDGET_CHANGED"
	EventPortfolioHeatExceeded ledger.EventType = "PMS_PORTFOLIO_HEAT_EXCEEDED"
	EventExposureThresholdExceeded ledger.EventType = "EXPOSURE_THRESHOLD_EXCEEDED"
	EventRiskBudgetViolation ledger.EventType = "RISK_BUDGET_VIOLATION"

	// Strategy budgets
	EventStrategyBudgetChanged  ledger.EventType = "STRATEGY_BUDGET_CHANGED"
	EventStrategyAutoscaled     ledger.EventType = "STRATEGY_AUTOSCALED"
	EventStrategyAutoDisabled   ledger.EventType = "STRATEGY_AUTO_DISABLED"
	EventStrategyAutoPromoted   ledger.EventType = "STRATEGY_AUTO_PROMOTED"

	// Accounts
	EventAccountCreated ledger.EventType = "PMS_ACCOUNT_CREATED"
	EventAccountClosed  ledger.EventType = "PMS_ACCOUNT_CLOSED"
	EventAccountUpdated ledger.EventType = "PMS_ACCOUNT_UPDATED"

	// Optimizer
	EventOptimizerRecommendationCreated ledger.EventType = "OPTIMIZER_RECOMMENDATION_CREATED"
)

// ── Portfolio Payloads ────────────────────────────────────────────────────────

type PortfolioCreatedPayload struct {
	PortfolioID   string    `json:"portfolio_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"` // MASTER | SUB | MANAGED | PROP | PAPER
	ParentID      string    `json:"parent_id,omitempty"`
	InitialNAV    float64   `json:"initial_nav_usd"`
	BaseCurrency  string    `json:"base_currency"`
	Description   string    `json:"description,omitempty"`
	CreatedBy     string    `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

type PortfolioUpdatedPayload struct {
	PortfolioID string            `json:"portfolio_id"`
	Changes     map[string]string `json:"changes"`
	UpdatedBy   string            `json:"updated_by"`
}

type PortfolioNAVUpdatedPayload struct {
	PortfolioID    string    `json:"portfolio_id"`
	PreviousNAV    float64   `json:"previous_nav_usd"`
	CurrentNAV     float64   `json:"current_nav_usd"`
	DeltaUSD       float64   `json:"delta_usd"`
	DeltaPct       float64   `json:"delta_pct"`
	ComputedAt     time.Time `json:"computed_at"`
}

// ── Allocation Payloads ───────────────────────────────────────────────────────

type AllocationCreatedPayload struct {
	AllocationID  string  `json:"allocation_id"`
	PortfolioID   string  `json:"portfolio_id"`
	StrategyID    string  `json:"strategy_id"`
	StrategyName  string  `json:"strategy_name"`
	Method        string  `json:"method"` // FIXED|DYNAMIC|SHARPE_WEIGHTED|VOL_ADJUSTED|DRAWDOWN_ADJUSTED|KELLY|PERF_WEIGHTED
	AllocPct      float64 `json:"alloc_pct"`
	AllocUSD      float64 `json:"alloc_usd"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	MaxPositionPct float64 `json:"max_position_pct"`
}

type AllocationChangedPayload struct {
	AllocationID  string  `json:"allocation_id"`
	PortfolioID   string  `json:"portfolio_id"`
	StrategyID    string  `json:"strategy_id"`
	PreviousPct   float64 `json:"previous_pct"`
	NewPct        float64 `json:"new_pct"`
	PreviousUSD   float64 `json:"previous_usd"`
	NewUSD        float64 `json:"new_usd"`
	Reason        string  `json:"reason"`
}

type AllocationRemovedPayload struct {
	AllocationID string  `json:"allocation_id"`
	PortfolioID  string  `json:"portfolio_id"`
	StrategyID   string  `json:"strategy_id"`
	FinalNAV     float64 `json:"final_nav_usd"`
	Reason       string  `json:"reason"`
}

type MasterAllocationChangedPayload struct {
	MasterPortfolioID string                  `json:"master_portfolio_id"`
	SubAllocations    []SubPortfolioAllocation `json:"sub_allocations"`
	TotalAllocPct     float64                 `json:"total_alloc_pct"`
	Reason            string                  `json:"reason"`
}

type SubPortfolioAllocation struct {
	SubPortfolioID string  `json:"sub_portfolio_id"`
	AllocPct       float64 `json:"alloc_pct"`
	AllocUSD       float64 `json:"alloc_usd"`
}

// ── Risk Budget Payloads ──────────────────────────────────────────────────────

type RiskBudgetCreatedPayload struct {
	PortfolioID       string  `json:"portfolio_id"`
	MaxHeatPct        float64 `json:"max_heat_pct"`
	MaxVaR95Pct       float64 `json:"max_var95_pct"`
	MaxCVaR95Pct      float64 `json:"max_cvar95_pct"`
	MaxDrawdownPct    float64 `json:"max_drawdown_pct"`
	MaxDailyLossPct   float64 `json:"max_daily_loss_pct"`
	MaxWeeklyLossPct  float64 `json:"max_weekly_loss_pct"`
	MaxMonthlyLossPct float64 `json:"max_monthly_loss_pct"`
	MaxGrossExpPct    float64 `json:"max_gross_exp_pct"`
	MaxNetExpPct      float64 `json:"max_net_exp_pct"`
}

type RiskBudgetChangedPayload struct {
	PortfolioID string            `json:"portfolio_id"`
	Changes     map[string]string `json:"changes"`
	Reason      string            `json:"reason"`
}

type PortfolioHeatExceededPayload struct {
	PortfolioID    string  `json:"portfolio_id"`
	CurrentHeatPct float64 `json:"current_heat_pct"`
	LimitPct       float64 `json:"limit_pct"`
	TriggerSource  string  `json:"trigger_source"`
}

type ExposureThresholdExceededPayload struct {
	PortfolioID    string  `json:"portfolio_id"`
	ExposureType   string  `json:"exposure_type"` // GROSS|NET|SYMBOL|EXCHANGE|STRATEGY
	Dimension      string  `json:"dimension"`
	CurrentPct     float64 `json:"current_pct"`
	LimitPct       float64 `json:"limit_pct"`
}

// ── Strategy Budget Payloads ──────────────────────────────────────────────────

type StrategyBudgetChangedPayload struct {
	StrategyID      string  `json:"strategy_id"`
	StrategyName    string  `json:"strategy_name"`
	PortfolioID     string  `json:"portfolio_id"`
	PreviousBudget  float64 `json:"previous_budget_usd"`
	NewBudget       float64 `json:"new_budget_usd"`
	DailyLossLimit  float64 `json:"daily_loss_limit_usd"`
	WeeklyLossLimit float64 `json:"weekly_loss_limit_usd"`
	MonthlyDDLimit  float64 `json:"monthly_dd_limit_usd"`
	Reason          string  `json:"reason"`
}

type StrategyAutoscaledPayload struct {
	StrategyID   string  `json:"strategy_id"`
	StrategyName string  `json:"strategy_name"`
	PortfolioID  string  `json:"portfolio_id"`
	Direction    string  `json:"direction"` // UP | DOWN
	PreviousPct  float64 `json:"previous_pct"`
	NewPct       float64 `json:"new_pct"`
	Trigger      string  `json:"trigger"` // SHARPE | DRAWDOWN | WINRATE | PROFIT_FACTOR
	MetricValue  float64 `json:"metric_value"`
}

// ── Account Payloads ──────────────────────────────────────────────────────────

type AccountCreatedPayload struct {
	AccountID   string  `json:"account_id"`
	Name        string  `json:"name"`
	Type        string  `json:"type"` // MASTER|SUB|PROP|CLIENT|PAPER
	PortfolioID string  `json:"portfolio_id"`
	InitialNAV  float64 `json:"initial_nav_usd"`
	Currency    string  `json:"currency"`
	CreatedBy   string  `json:"created_by"`
}

type AccountClosedPayload struct {
	AccountID   string  `json:"account_id"`
	PortfolioID string  `json:"portfolio_id"`
	FinalNAV    float64 `json:"final_nav_usd"`
	Reason      string  `json:"reason"`
}

// ── Optimizer Payloads ────────────────────────────────────────────────────────

type OptimizerRecommendationPayload struct {
	RecommendationID string               `json:"recommendation_id"`
	PortfolioID      string               `json:"portfolio_id"`
	Method           string               `json:"method"` // MVO|RISK_PARITY|MAX_DIV|VOL_TARGET
	CurrentWeights   map[string]float64   `json:"current_weights"`
	OptimalWeights   map[string]float64   `json:"optimal_weights"`
	ExpectedReturn   float64              `json:"expected_return_pct"`
	ExpectedVol      float64              `json:"expected_vol_pct"`
	ExpectedSharpe   float64              `json:"expected_sharpe"`
	DiversificationRatio float64          `json:"diversification_ratio"`
	Rationale        string               `json:"rationale"`
	Changes          []WeightChange       `json:"changes"`
	GeneratedAt      time.Time            `json:"generated_at"`
}

type WeightChange struct {
	StrategyID   string  `json:"strategy_id"`
	StrategyName string  `json:"strategy_name"`
	CurrentPct   float64 `json:"current_pct"`
	OptimalPct   float64 `json:"optimal_pct"`
	DeltaPct     float64 `json:"delta_pct"`
	Rationale    string  `json:"rationale"`
}

// ── Event emission helpers ────────────────────────────────────────────────────

func emitPMSEvent(ctx context.Context, store ledger.Store, input ledger.NewEventInput) {
	if store == nil {
		return
	}
	ev, err := ledger.NewEvent(input)
	if err != nil {
		return
	}
	store.Append(ctx, ev) //nolint:errcheck
}

func EmitPortfolioCreated(ctx context.Context, store ledger.Store, p PortfolioCreatedPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   p.PortfolioID,
		EventType:     EventPortfolioCreated,
		AccountID:     p.PortfolioID,
		Payload:       p,
		Source:        "pms",
	})
}

func EmitAllocationChanged(ctx context.Context, store ledger.Store, p AllocationChangedPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   p.PortfolioID,
		EventType:     EventAllocationChanged,
		AccountID:     p.PortfolioID,
		StrategyID:    p.StrategyID,
		Payload:       p,
		Source:        "pms.allocation",
	})
}

func EmitStrategyBudgetChanged(ctx context.Context, store ledger.Store, p StrategyBudgetChangedPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregateBudget,
		AggregateID:   fmt.Sprintf("%s:%s", p.PortfolioID, p.StrategyID),
		EventType:     EventStrategyBudgetChanged,
		AccountID:     p.PortfolioID,
		StrategyID:    p.StrategyID,
		Payload:       p,
		Source:        "pms.budget",
	})
}

func EmitRiskBudgetCreated(ctx context.Context, store ledger.Store, p RiskBudgetCreatedPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   p.PortfolioID,
		EventType:     EventRiskBudgetCreated,
		AccountID:     p.PortfolioID,
		Payload:       p,
		Source:        "pms.risk",
	})
}

func EmitPortfolioHeatExceeded(ctx context.Context, store ledger.Store, p PortfolioHeatExceededPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   p.PortfolioID,
		EventType:     EventPortfolioHeatExceeded,
		AccountID:     p.PortfolioID,
		Payload:       p,
		Source:        "pms.risk",
	})
}

func EmitExposureThresholdExceeded(ctx context.Context, store ledger.Store, p ExposureThresholdExceededPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   p.PortfolioID,
		EventType:     EventExposureThresholdExceeded,
		AccountID:     p.PortfolioID,
		Payload:       p,
		Source:        "pms.exposure",
	})
}

func EmitAccountCreated(ctx context.Context, store ledger.Store, p AccountCreatedPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregateAccount,
		AggregateID:   p.AccountID,
		EventType:     EventAccountCreated,
		AccountID:     p.AccountID,
		Payload:       p,
		Source:        "pms.accounts",
	})
}

func EmitOptimizerRecommendation(ctx context.Context, store ledger.Store, p OptimizerRecommendationPayload) {
	emitPMSEvent(ctx, store, ledger.NewEventInput{
		AggregateType: AggregatePortfolio,
		AggregateID:   p.PortfolioID,
		EventType:     EventOptimizerRecommendationCreated,
		AccountID:     p.PortfolioID,
		Payload:       p,
		Source:        "pms.optimizer",
	})
}
