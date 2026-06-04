package pms

import (
	"context"
	"fmt"
	"sync"
	"time"

	"antigravity-engine/internal/ledger"
)

// RiskContribution tracks a single dimension's share of total portfolio risk.
type RiskContribution struct {
	Dimension   string  // strategy name, exchange name, or asset symbol
	DollarRisk  float64 // USD at risk from this dimension
	HeatPct     float64 // this dimension's share of total heat %
	VaRContrib  float64 // marginal VaR contribution in USD
	NotionalUSD float64 // gross notional exposure
	Weight      float64 // portfolio weight 0–1
}

// PortfolioRiskState holds the current real-time risk budget utilisation.
type PortfolioRiskState struct {
	PortfolioID string
	UpdatedAt   time.Time

	// Aggregate heat
	TotalHeatPct float64 // sum of all position dollar risks / equity * 100
	HeatLimitPct float64

	// VaR / CVaR
	VaR95Pct  float64
	CVaR95Pct float64

	// Drawdown
	DrawdownPct    float64
	MaxDrawdownPct float64

	// Daily / weekly / monthly losses
	DailyLossUSD   float64
	WeeklyLossUSD  float64
	MonthlyLossUSD float64

	// Exposure
	GrossExpPct float64
	NetExpPct   float64

	// Risk contributions
	ByStrategy []RiskContribution
	ByExchange []RiskContribution
	ByAsset    []RiskContribution

	// Budget utilisation (0–1)
	HeatUtilisation    float64
	VaRUtilisation     float64
	DrawdownUtilisation float64
	DailyLossUtilisation float64
}

// RiskBudgetViolation describes a portfolio-level risk limit breach.
type RiskBudgetViolation struct {
	Type    string  // HEAT | VAR | CVAR | DRAWDOWN | DAILY_LOSS | EXPOSURE
	Current float64
	Limit   float64
	Message string
}

// PortfolioRiskBudget extends the riskv3 engine with portfolio-level limits
// that override per-strategy limits. Portfolio-level limits are the final gate.
type PortfolioRiskBudget struct {
	mu     sync.RWMutex
	states map[string]*PortfolioRiskState // keyed by portfolioID
	store  ledger.Store
}

// NewPortfolioRiskBudget constructs a risk budget manager.
func NewPortfolioRiskBudget(store ledger.Store) *PortfolioRiskBudget {
	return &PortfolioRiskBudget{
		states: make(map[string]*PortfolioRiskState),
		store:  store,
	}
}

// InitPortfolio registers a portfolio's risk budget from its RiskBudget config.
func (rb *PortfolioRiskBudget) InitPortfolio(portfolioID string, budget RiskBudget) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.states[portfolioID] = &PortfolioRiskState{
		PortfolioID:    portfolioID,
		HeatLimitPct:   budget.MaxHeatPct,
		VaR95Pct:       0,
		CVaR95Pct:      0,
		MaxDrawdownPct: budget.MaxDrawdownPct,
		UpdatedAt:      time.Now().UTC(),
	}
}

// CheckPortfolioRisk is the portfolio-level pre-trade risk gate.
// It runs AFTER the strategy-level riskv3 check. Both must pass.
// Returns nil when the trade may proceed; returns a violation slice otherwise.
func (rb *PortfolioRiskBudget) CheckPortfolioRisk(
	ctx context.Context,
	portfolioID string,
	budget RiskBudget,
	proposedDollarRisk float64,
	equityUSD float64,
) []RiskBudgetViolation {
	rb.mu.RLock()
	state, ok := rb.states[portfolioID]
	rb.mu.RUnlock()

	if !ok || equityUSD <= 0 {
		return nil
	}

	var violations []RiskBudgetViolation

	// Heat check (proposed additional risk)
	proposedHeat := state.TotalHeatPct + (proposedDollarRisk/equityUSD)*100
	if proposedHeat > budget.MaxHeatPct {
		violations = append(violations, RiskBudgetViolation{
			Type:    "HEAT",
			Current: proposedHeat,
			Limit:   budget.MaxHeatPct,
			Message: fmt.Sprintf("proposed heat %.1f%% exceeds portfolio limit %.1f%%", proposedHeat, budget.MaxHeatPct),
		})
	}

	// VaR check
	if state.VaR95Pct > budget.MaxVaR95Pct {
		violations = append(violations, RiskBudgetViolation{
			Type:    "VAR",
			Current: state.VaR95Pct,
			Limit:   budget.MaxVaR95Pct,
			Message: fmt.Sprintf("VaR95 %.2f%% exceeds portfolio limit %.2f%%", state.VaR95Pct, budget.MaxVaR95Pct),
		})
	}

	// CVaR check
	if state.CVaR95Pct > budget.MaxCVaR95Pct {
		violations = append(violations, RiskBudgetViolation{
			Type:    "CVAR",
			Current: state.CVaR95Pct,
			Limit:   budget.MaxCVaR95Pct,
			Message: fmt.Sprintf("CVaR95 %.2f%% exceeds portfolio limit %.2f%%", state.CVaR95Pct, budget.MaxCVaR95Pct),
		})
	}

	// Drawdown check
	if state.DrawdownPct >= budget.MaxDrawdownPct {
		violations = append(violations, RiskBudgetViolation{
			Type:    "DRAWDOWN",
			Current: state.DrawdownPct,
			Limit:   budget.MaxDrawdownPct,
			Message: fmt.Sprintf("portfolio drawdown %.1f%% >= limit %.1f%% — trading halted", state.DrawdownPct, budget.MaxDrawdownPct),
		})
	}

	// Daily loss check
	dailyLossPct := state.DailyLossUSD / equityUSD * 100
	if dailyLossPct >= budget.MaxDailyLossPct {
		violations = append(violations, RiskBudgetViolation{
			Type:    "DAILY_LOSS",
			Current: dailyLossPct,
			Limit:   budget.MaxDailyLossPct,
			Message: fmt.Sprintf("daily loss %.2f%% >= limit %.2f%%", dailyLossPct, budget.MaxDailyLossPct),
		})
	}

	// Gross exposure check
	if state.GrossExpPct > budget.MaxGrossExpPct {
		violations = append(violations, RiskBudgetViolation{
			Type:    "GROSS_EXPOSURE",
			Current: state.GrossExpPct,
			Limit:   budget.MaxGrossExpPct,
			Message: fmt.Sprintf("gross exposure %.1f%% exceeds limit %.1f%%", state.GrossExpPct, budget.MaxGrossExpPct),
		})
	}

	// Emit events for each violation
	for _, v := range violations {
		if v.Type == "HEAT" {
			EmitPortfolioHeatExceeded(ctx, rb.store, PortfolioHeatExceededPayload{
				PortfolioID:    portfolioID,
				CurrentHeatPct: proposedHeat,
				LimitPct:       budget.MaxHeatPct,
				TriggerSource:  "pre_trade_check",
			})
		} else {
			EmitExposureThresholdExceeded(ctx, rb.store, ExposureThresholdExceededPayload{
				PortfolioID:  portfolioID,
				ExposureType: v.Type,
				CurrentPct:   v.Current,
				LimitPct:     v.Limit,
			})
		}
	}

	return violations
}

// UpdateState refreshes the portfolio risk state after a position change.
func (rb *PortfolioRiskBudget) UpdateState(
	portfolioID string,
	heatPct, var95Pct, cvar95Pct, drawdownPct float64,
	dailyLossUSD, weeklyLossUSD, monthlyLossUSD float64,
	grossExpPct, netExpPct float64,
	byStrategy, byExchange, byAsset []RiskContribution,
) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	s, ok := rb.states[portfolioID]
	if !ok {
		s = &PortfolioRiskState{PortfolioID: portfolioID}
		rb.states[portfolioID] = s
	}
	s.TotalHeatPct = heatPct
	s.VaR95Pct = var95Pct
	s.CVaR95Pct = cvar95Pct
	s.DrawdownPct = drawdownPct
	s.DailyLossUSD = dailyLossUSD
	s.WeeklyLossUSD = weeklyLossUSD
	s.MonthlyLossUSD = monthlyLossUSD
	s.GrossExpPct = grossExpPct
	s.NetExpPct = netExpPct
	s.ByStrategy = byStrategy
	s.ByExchange = byExchange
	s.ByAsset = byAsset
	s.UpdatedAt = time.Now().UTC()

	// Compute utilisation ratios
	if s.HeatLimitPct > 0 {
		s.HeatUtilisation = heatPct / s.HeatLimitPct
	}
	if s.MaxDrawdownPct > 0 {
		s.DrawdownUtilisation = drawdownPct / s.MaxDrawdownPct
	}
}

// Snapshot returns a read-only copy of a portfolio's risk state.
func (rb *PortfolioRiskBudget) Snapshot(portfolioID string) (PortfolioRiskState, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	s, ok := rb.states[portfolioID]
	if !ok {
		return PortfolioRiskState{}, false
	}
	// Deep copy contributions slices
	snap := *s
	snap.ByStrategy = append([]RiskContribution(nil), s.ByStrategy...)
	snap.ByExchange = append([]RiskContribution(nil), s.ByExchange...)
	snap.ByAsset = append([]RiskContribution(nil), s.ByAsset...)
	return snap, true
}

// BudgetUtilisationReport returns a human-readable summary of budget utilisation.
func (rb *PortfolioRiskBudget) BudgetUtilisationReport(portfolioID string) string {
	s, ok := rb.Snapshot(portfolioID)
	if !ok {
		return fmt.Sprintf("portfolio %s: no risk state", portfolioID)
	}
	return fmt.Sprintf(
		"portfolio=%s heat=%.1f%% var95=%.2f%% cvar95=%.2f%% dd=%.1f%% daily_loss=$%.0f gross_exp=%.1f%%",
		portfolioID, s.TotalHeatPct, s.VaR95Pct, s.CVaR95Pct,
		s.DrawdownPct, s.DailyLossUSD, s.GrossExpPct,
	)
}
