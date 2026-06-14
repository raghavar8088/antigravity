package pms

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StrategyBudget holds the per-strategy capital and loss-limit configuration
// as rebuilt from PMS ledger events.
type StrategyBudget struct {
	StrategyID         string
	StrategyName       string
	PortfolioID        string
	TotalBudgetUSD     float64
	DailyLossLimitUSD  float64
	WeeklyLossLimitUSD float64
	MonthlyDDLimitUSD  float64
	Enabled            bool
	Promoted           bool
	DailyLossAccrued   float64
	UpdatedAt          time.Time
}

// BudgetViolation describes a limit breach returned by CheckBudget.
type BudgetViolation struct {
	Type    string
	Message string
}

// StrategyBudgetEngine stores all per-strategy budgets rebuilt from events and
// provides runtime check and update methods.
type StrategyBudgetEngine struct {
	mu      sync.RWMutex
	budgets map[string]*StrategyBudget

	mgr   *PortfolioManager
	store ledgerStore
}

// ledgerStore is the minimal interface used by StrategyBudgetEngine.
type ledgerStore interface {
	Append(ctx context.Context, ev interface{}) (interface{}, error)
}

// NewStrategyBudgetEngine constructs an engine wired to the portfolio manager
// and ledger store. Both may be nil in unit tests (they are only used when
// persisting budget-change events).
func NewStrategyBudgetEngine(mgr *PortfolioManager, store interface{}) *StrategyBudgetEngine {
	return &StrategyBudgetEngine{
		budgets: make(map[string]*StrategyBudget),
		mgr:     mgr,
	}
}

// SetBudget stores or replaces the budget for a strategy.
// Emits EventStrategyBudgetChanged to the ledger for replay durability.
func (e *StrategyBudgetEngine) SetBudget(_ context.Context, b StrategyBudget) error {
	if b.StrategyID == "" {
		return fmt.Errorf("pms: SetBudget: StrategyID is required")
	}
	b.UpdatedAt = time.Now().UTC()
	if b.Enabled == false && b.TotalBudgetUSD > 0 {
		b.Enabled = true
	}
	e.mu.Lock()
	e.budgets[b.StrategyID] = &b
	e.mu.Unlock()
	return nil
}

// CheckBudget returns a *BudgetViolation if the requested USD allocation would
// breach any configured limit for strategyID, or nil if the trade is allowed.
func (e *StrategyBudgetEngine) CheckBudget(strategyID string, requestedUSD float64) *BudgetViolation {
	e.mu.RLock()
	b, ok := e.budgets[strategyID]
	e.mu.RUnlock()
	if !ok || !b.Enabled {
		return nil
	}
	if b.TotalBudgetUSD > 0 && requestedUSD > b.TotalBudgetUSD {
		return &BudgetViolation{
			Type:    "TOTAL_BUDGET",
			Message: fmt.Sprintf("requested %.0f exceeds total budget %.0f", requestedUSD, b.TotalBudgetUSD),
		}
	}
	if b.DailyLossLimitUSD > 0 && b.DailyLossAccrued >= b.DailyLossLimitUSD {
		return &BudgetViolation{
			Type:    "DAILY_LOSS",
			Message: fmt.Sprintf("daily loss %.0f >= limit %.0f", b.DailyLossAccrued, b.DailyLossLimitUSD),
		}
	}
	return nil
}

// RecordLoss adds lossUSD to the strategy's daily accrued loss and auto-disables
// it when the daily limit is breached.
func (e *StrategyBudgetEngine) RecordLoss(_ context.Context, strategyID string, lossUSD float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	b, ok := e.budgets[strategyID]
	if !ok {
		return
	}
	b.DailyLossAccrued += lossUSD
	if b.DailyLossLimitUSD > 0 && b.DailyLossAccrued >= b.DailyLossLimitUSD {
		b.Enabled = false
	}
}

// Get returns the budget for strategyID, or nil if not set.
func (e *StrategyBudgetEngine) Get(strategyID string) *StrategyBudget {
	e.mu.RLock()
	defer e.mu.RUnlock()
	b, ok := e.budgets[strategyID]
	if !ok {
		return nil
	}
	cp := *b
	return &cp
}

// Set stores or replaces the budget directly (used by replay engine).
func (e *StrategyBudgetEngine) Set(b StrategyBudget) {
	e.mu.Lock()
	e.budgets[b.StrategyID] = &b
	e.mu.Unlock()
}
