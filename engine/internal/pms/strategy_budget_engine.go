package pms

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// StrategyBudget defines the capital and risk boundaries for a single strategy.
type StrategyBudget struct {
	StrategyID   string
	StrategyName string
	PortfolioID  string

	// Capital budget
	TotalBudgetUSD float64
	UsedBudgetUSD  float64

	// Loss budgets
	DailyLossLimitUSD  float64
	WeeklyLossLimitUSD float64
	MonthlyDDLimitUSD  float64

	// Tracking
	DailyLossUSD  float64
	WeeklyLossUSD float64
	MonthlyDDUSD  float64

	// Performance (rolling)
	SharpeRatio  float64
	WinRate      float64
	ProfitFactor float64
	DrawdownPct  float64

	// Lifecycle
	Enabled   bool
	Promoted  bool
	UpdatedAt time.Time
}

// BudgetViolation describes a budget limit breach.
type BudgetViolation struct {
	StrategyID string
	Type       string // DAILY_LOSS | WEEKLY_LOSS | MONTHLY_DD | TOTAL_BUDGET
	Current    float64
	Limit      float64
}

// StrategyBudgetEngine manages per-strategy capital and loss budgets.
// It auto-scales, promotes, and disables strategies based on performance.
type StrategyBudgetEngine struct {
	mu      sync.RWMutex
	budgets map[string]*StrategyBudget // keyed by strategyID
	store   ledger.Store
	manager *PortfolioManager

	// Thresholds for auto-management
	AutoPromoteSharpe    float64 // promote if Sharpe exceeds this (default 1.5)
	AutoDisableSharpe    float64 // disable if Sharpe falls below this (default 0.3)
	AutoScaleUpPct       float64 // scale-up increment (default 5%)
	AutoScaleDownPct     float64 // scale-down decrement (default 5%)
	MaxAllocPct          float64 // hard cap per strategy (default 25%)
	MinAllocPct          float64 // floor per strategy (default 1%)
}

// NewStrategyBudgetEngine constructs a budget engine with institutional defaults.
func NewStrategyBudgetEngine(manager *PortfolioManager, store ledger.Store) *StrategyBudgetEngine {
	return &StrategyBudgetEngine{
		budgets:           make(map[string]*StrategyBudget),
		store:             store,
		manager:           manager,
		AutoPromoteSharpe: 1.5,
		AutoDisableSharpe: 0.3,
		AutoScaleUpPct:    5.0,
		AutoScaleDownPct:  5.0,
		MaxAllocPct:       25.0,
		MinAllocPct:       1.0,
	}
}

// SetBudget initialises or updates a strategy budget, emitting an event.
func (e *StrategyBudgetEngine) SetBudget(ctx context.Context, b StrategyBudget) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	b.UpdatedAt = time.Now().UTC()
	b.Enabled = true

	payload := StrategyBudgetChangedPayload{
		StrategyID:      b.StrategyID,
		StrategyName:    b.StrategyName,
		PortfolioID:     b.PortfolioID,
		NewBudget:       b.TotalBudgetUSD,
		DailyLossLimit:  b.DailyLossLimitUSD,
		WeeklyLossLimit: b.WeeklyLossLimitUSD,
		MonthlyDDLimit:  b.MonthlyDDLimitUSD,
		Reason:          "initial budget set",
	}
	if existing, ok := e.budgets[b.StrategyID]; ok {
		payload.PreviousBudget = existing.TotalBudgetUSD
		payload.Reason = "budget updated"
	}

	ev, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateBudget,
		AggregateID:   fmt.Sprintf("%s:%s", b.PortfolioID, b.StrategyID),
		EventType:     EventStrategyBudgetChanged,
		AccountID:     b.PortfolioID,
		StrategyID:    b.StrategyID,
		Payload:       payload,
		Source:        "pms.budget",
	})
	if err != nil {
		return err
	}
	if e.store != nil {
		e.store.Append(ctx, ev) //nolint:errcheck
	}
	e.budgets[b.StrategyID] = &b
	return nil
}

// CheckBudget validates whether a strategy can open a new position of the given USD size.
// Returns nil if allowed, or a BudgetViolation if not.
func (e *StrategyBudgetEngine) CheckBudget(strategyID string, proposedUSD float64) *BudgetViolation {
	e.mu.RLock()
	defer e.mu.RUnlock()
	b, ok := e.budgets[strategyID]
	if !ok {
		return &BudgetViolation{StrategyID: strategyID, Type: "UNKNOWN", Limit: 0}
	}
	if !b.Enabled {
		return &BudgetViolation{StrategyID: strategyID, Type: "DISABLED", Current: 0, Limit: 0}
	}
	if b.UsedBudgetUSD+proposedUSD > b.TotalBudgetUSD {
		return &BudgetViolation{
			StrategyID: strategyID,
			Type:       "TOTAL_BUDGET",
			Current:    b.UsedBudgetUSD + proposedUSD,
			Limit:      b.TotalBudgetUSD,
		}
	}
	return nil
}

// RecordLoss registers a realised loss for a strategy and checks limits.
// If a limit is breached the strategy is auto-disabled.
func (e *StrategyBudgetEngine) RecordLoss(ctx context.Context, strategyID string, lossUSD float64) {
	e.mu.Lock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.Unlock()
		return
	}
	b.DailyLossUSD += lossUSD
	b.WeeklyLossUSD += lossUSD
	b.MonthlyDDUSD += lossUSD
	b.UpdatedAt = time.Now().UTC()
	e.mu.Unlock()

	e.checkLossLimits(ctx, strategyID)
}

// RecordWin registers a realised gain for budget tracking.
func (e *StrategyBudgetEngine) RecordWin(strategyID string, gainUSD float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	b, ok := e.budgets[strategyID]
	if !ok {
		return
	}
	b.UsedBudgetUSD = max64(0, b.UsedBudgetUSD-gainUSD)
	b.UpdatedAt = time.Now().UTC()
}

// UpdatePerformance refreshes performance metrics and triggers auto-management.
func (e *StrategyBudgetEngine) UpdatePerformance(ctx context.Context, strategyID string, m StrategyMetrics) {
	e.mu.Lock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.Unlock()
		return
	}
	b.SharpeRatio = m.SharpeRatio
	b.WinRate = m.WinRate
	b.ProfitFactor = m.ProfitFactor
	b.DrawdownPct = m.CurrentDDPct
	prevSharpe := b.SharpeRatio
	e.mu.Unlock()

	e.autoManage(ctx, strategyID, prevSharpe, m)
}

// ResetDailyBudgets clears daily loss counters. Call at market open each day.
func (e *StrategyBudgetEngine) ResetDailyBudgets() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.budgets {
		b.DailyLossUSD = 0
	}
}

// ResetWeeklyBudgets clears weekly loss counters. Call on Monday market open.
func (e *StrategyBudgetEngine) ResetWeeklyBudgets() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.budgets {
		b.WeeklyLossUSD = 0
	}
}

// BudgetSnapshot returns a read-only view of all budgets.
func (e *StrategyBudgetEngine) BudgetSnapshot() []StrategyBudget {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]StrategyBudget, 0, len(e.budgets))
	for _, b := range e.budgets {
		out = append(out, *b)
	}
	return out
}

// ── Auto-management ───────────────────────────────────────────────────────────

func (e *StrategyBudgetEngine) checkLossLimits(ctx context.Context, strategyID string) {
	e.mu.RLock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.RUnlock()
		return
	}
	dailyBreached := b.DailyLossLimitUSD > 0 && b.DailyLossUSD >= b.DailyLossLimitUSD
	weeklyBreached := b.WeeklyLossLimitUSD > 0 && b.WeeklyLossUSD >= b.WeeklyLossLimitUSD
	monthlyBreached := b.MonthlyDDLimitUSD > 0 && b.MonthlyDDUSD >= b.MonthlyDDLimitUSD
	portfolioID := b.PortfolioID
	name := b.StrategyName
	e.mu.RUnlock()

	if dailyBreached || weeklyBreached || monthlyBreached {
		reason := "daily_loss_limit"
		if weeklyBreached {
			reason = "weekly_loss_limit"
		}
		if monthlyBreached {
			reason = "monthly_dd_limit"
		}
		e.disableStrategy(ctx, strategyID, portfolioID, name, reason)
	}
}

func (e *StrategyBudgetEngine) autoManage(ctx context.Context, strategyID string, prevSharpe float64, m StrategyMetrics) {
	e.mu.RLock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.RUnlock()
		return
	}
	portfolioID := b.PortfolioID
	enabled := b.Enabled
	e.mu.RUnlock()

	if !enabled {
		return
	}

	if m.SharpeRatio < e.AutoDisableSharpe && m.TotalTrades >= 20 {
		e.disableStrategy(ctx, strategyID, portfolioID, m.StrategyName, "sharpe_below_threshold")
		return
	}
	if m.SharpeRatio >= e.AutoPromoteSharpe && prevSharpe < e.AutoPromoteSharpe {
		e.promoteStrategy(ctx, strategyID, portfolioID, m)
		return
	}
	// Drawdown-triggered scale-down
	if m.CurrentDDPct >= m.MaxDrawdownPct*0.8 {
		e.scaleDown(ctx, strategyID, portfolioID, m, "approaching_max_drawdown")
		return
	}
	// Performance-triggered scale-up
	if m.SharpeRatio >= 1.0 && m.WinRate >= 0.55 && m.ProfitFactor >= 1.5 {
		e.scaleUp(ctx, strategyID, portfolioID, m, "strong_performance")
	}
}

func (e *StrategyBudgetEngine) disableStrategy(ctx context.Context, strategyID, portfolioID, name, reason string) {
	e.mu.Lock()
	b, ok := e.budgets[strategyID]
	if !ok || !b.Enabled {
		e.mu.Unlock()
		return
	}
	b.Enabled = false
	b.UpdatedAt = time.Now().UTC()
	e.mu.Unlock()

	log.Printf("[PMS BUDGET] AUTO-DISABLED strategy=%s reason=%s portfolio=%s", name, reason, portfolioID)
	payload := StrategyBudgetChangedPayload{
		StrategyID:   strategyID,
		StrategyName: name,
		PortfolioID:  portfolioID,
		Reason:       "auto_disabled: " + reason,
	}
	ev, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateBudget,
		AggregateID:   fmt.Sprintf("%s:%s", portfolioID, strategyID),
		EventType:     EventStrategyAutoDisabled,
		AccountID:     portfolioID,
		StrategyID:    strategyID,
		Payload:       payload,
		Source:        "pms.budget.auto",
	})
	if e.store != nil {
		e.store.Append(ctx, ev) //nolint:errcheck
	}
}

func (e *StrategyBudgetEngine) promoteStrategy(ctx context.Context, strategyID, portfolioID string, m StrategyMetrics) {
	e.mu.Lock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.Unlock()
		return
	}
	b.Promoted = true
	b.UpdatedAt = time.Now().UTC()
	e.mu.Unlock()

	log.Printf("[PMS BUDGET] AUTO-PROMOTED strategy=%s sharpe=%.2f portfolio=%s", m.StrategyName, m.SharpeRatio, portfolioID)
	payload := StrategyAutoscaledPayload{
		StrategyID:   strategyID,
		StrategyName: m.StrategyName,
		PortfolioID:  portfolioID,
		Direction:    "PROMOTE",
		Trigger:      "SHARPE",
		MetricValue:  m.SharpeRatio,
	}
	ev, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateBudget,
		AggregateID:   fmt.Sprintf("%s:%s", portfolioID, strategyID),
		EventType:     EventStrategyAutoPromoted,
		AccountID:     portfolioID,
		StrategyID:    strategyID,
		Payload:       payload,
		Source:        "pms.budget.auto",
	})
	if e.store != nil {
		e.store.Append(ctx, ev) //nolint:errcheck
	}
}

func (e *StrategyBudgetEngine) scaleUp(ctx context.Context, strategyID, portfolioID string, m StrategyMetrics, trigger string) {
	e.mu.Lock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.Unlock()
		return
	}
	prevPct := m.CurrentAllocPct
	newPct := min64(prevPct+e.AutoScaleUpPct, e.MaxAllocPct)
	newUSD := m.TotalEquityUSD * newPct / 100.0
	b.TotalBudgetUSD = newUSD
	b.UpdatedAt = time.Now().UTC()
	e.mu.Unlock()

	payload := StrategyAutoscaledPayload{
		StrategyID:   strategyID,
		StrategyName: m.StrategyName,
		PortfolioID:  portfolioID,
		Direction:    "UP",
		PreviousPct:  prevPct,
		NewPct:       newPct,
		Trigger:      trigger,
		MetricValue:  m.SharpeRatio,
	}
	ev, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateBudget,
		AggregateID:   fmt.Sprintf("%s:%s", portfolioID, strategyID),
		EventType:     EventStrategyAutoscaled,
		AccountID:     portfolioID,
		StrategyID:    strategyID,
		Payload:       payload,
		Source:        "pms.budget.auto",
	})
	if e.store != nil {
		e.store.Append(ctx, ev) //nolint:errcheck
	}
}

func (e *StrategyBudgetEngine) scaleDown(ctx context.Context, strategyID, portfolioID string, m StrategyMetrics, trigger string) {
	e.mu.Lock()
	b, ok := e.budgets[strategyID]
	if !ok {
		e.mu.Unlock()
		return
	}
	prevPct := m.CurrentAllocPct
	newPct := max64(prevPct-e.AutoScaleDownPct, e.MinAllocPct)
	newUSD := m.TotalEquityUSD * newPct / 100.0
	b.TotalBudgetUSD = newUSD
	b.UpdatedAt = time.Now().UTC()
	e.mu.Unlock()

	payload := StrategyAutoscaledPayload{
		StrategyID:   strategyID,
		StrategyName: m.StrategyName,
		PortfolioID:  portfolioID,
		Direction:    "DOWN",
		PreviousPct:  prevPct,
		NewPct:       newPct,
		Trigger:      trigger,
		MetricValue:  m.CurrentDDPct,
	}
	ev, _ := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: AggregateBudget,
		AggregateID:   fmt.Sprintf("%s:%s", portfolioID, strategyID),
		EventType:     EventStrategyAutoscaled,
		AccountID:     portfolioID,
		StrategyID:    strategyID,
		Payload:       payload,
		Source:        "pms.budget.auto",
	})
	if e.store != nil {
		e.store.Append(ctx, ev) //nolint:errcheck
	}
}

func max64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
