package riskv3

import "time"

// PortfolioMetrics is the complete real-time risk snapshot for the portfolio.
// Computed on demand from PortfolioSnapshot by the portfolio engine.
// This is the single source of truth for all risk dashboards and alerts.
type PortfolioMetrics struct {
	// ── Account ──────────────────────────────────────────────────────────────
	AccountID    string    `json:"account_id"`
	EquityUSD    float64   `json:"equity_usd"`
	HWMUSD       float64   `json:"hwm_usd"`
	OpenPositions int      `json:"open_positions"`

	// ── Loss tracking ────────────────────────────────────────────────────────
	DailyPnLUSD   float64 `json:"daily_pnl_usd"`
	WeeklyPnLUSD  float64 `json:"weekly_pnl_usd"`
	DailyLossPct  float64 `json:"daily_loss_pct"`   // positive = loss
	WeeklyLossPct float64 `json:"weekly_loss_pct"`

	// ── Drawdown ─────────────────────────────────────────────────────────────
	DrawdownPct float64 `json:"drawdown_pct"` // current DD from HWM

	// ── Portfolio heat ───────────────────────────────────────────────────────
	HeatPct      float64   `json:"heat_pct"`
	HeatLevel    HeatLevel `json:"heat_level"`
	HeatUSD      float64   `json:"heat_usd"`   // total dollar risk at risk

	// ── Exposure ─────────────────────────────────────────────────────────────
	GrossNotionalUSD    float64 `json:"gross_notional_usd"`
	NetNotionalUSD      float64 `json:"net_notional_usd"`
	GrossExposurePct    float64 `json:"gross_exposure_pct"` // gross / equity * 100
	NetExposurePct      float64 `json:"net_exposure_pct"`   // net / equity * 100

	// ── VaR (Value at Risk) ───────────────────────────────────────────────────
	DailyVaR95USD float64 `json:"daily_var_95_usd"`
	DailyVaR99USD float64 `json:"daily_var_99_usd"`
	DailyVaR95Pct float64 `json:"daily_var_95_pct"`
	DailyVaR99Pct float64 `json:"daily_var_99_pct"`
	WeeklyVaR95USD float64 `json:"weekly_var_95_usd"`

	// ── CVaR / Expected Shortfall ─────────────────────────────────────────────
	CVaR95USD float64 `json:"cvar_95_usd"`
	CVaR99USD float64 `json:"cvar_99_usd"`
	CVaR95Pct float64 `json:"cvar_95_pct"`
	CVaR99Pct float64 `json:"cvar_99_pct"`

	// ── Correlation ───────────────────────────────────────────────────────────
	MaxPairwiseCorr      float64 `json:"max_pairwise_corr"`       // highest |ρ| between any two positions
	CorrelatedClusterPct float64 `json:"correlated_cluster_pct"`  // % equity in correlated cluster

	// ── Concentration ─────────────────────────────────────────────────────────
	MaxSymbolConcentrationPct   float64 `json:"max_symbol_concentration_pct"`
	MaxStrategyConcentrationPct float64 `json:"max_strategy_concentration_pct"`
	MaxExchangeConcentrationPct float64 `json:"max_exchange_concentration_pct"`
	ConcentrationScore          float64 `json:"concentration_score"` // 0–100; higher = more concentrated

	// ── Kelly ─────────────────────────────────────────────────────────────────
	KellyFractionPct  float64 `json:"kelly_fraction_pct"`  // recommended as % of equity
	KellyEdge         float64 `json:"kelly_edge"`

	// ── Composite risk score ─────────────────────────────────────────────────
	// RiskScore is 0–100. Higher = safer. Below 70 blocks new orders.
	RiskScore   int    `json:"risk_score"`
	RiskSummary string `json:"risk_summary"` // human-readable explanation

	// ── Violations ────────────────────────────────────────────────────────────
	Violations []Violation `json:"violations,omitempty"`

	// ── Metadata ─────────────────────────────────────────────────────────────
	ComputedAt time.Time `json:"computed_at"`
}

// HasViolations returns true when any hard limit is breached.
func (m PortfolioMetrics) HasViolations() bool {
	return len(m.Violations) > 0
}

// IsSafeToTrade returns true when RiskScore >= 70 and no KILL-level heat.
func (m PortfolioMetrics) IsSafeToTrade() bool {
	return m.RiskScore >= 70 && m.HeatLevel != HeatKill
}

// ─── Strategy risk metrics ────────────────────────────────────────────────────

// StrategyRiskMetrics holds per-strategy risk contribution.
type StrategyRiskMetrics struct {
	StrategyName    string  `json:"strategy_name"`
	DollarRisk      float64 `json:"dollar_risk_usd"`
	RiskPct         float64 `json:"risk_pct"`    // as % of equity
	NotionalUSD     float64 `json:"notional_usd"`
	PositionCount   int     `json:"position_count"`
}

// ─── Composite risk score ─────────────────────────────────────────────────────

// ComputeRiskScore calculates a composite risk score 0–100.
// Higher is safer. The score is penalised by heat, VaR, CVaR, drawdown,
// concentration, and correlation. 70 is the institutional trading threshold.
//
//   score = 100
//         – min(heat_pct * 4,  40)    max 40 points from heat
//         – min(var95_pct * 3, 25)    max 25 points from VaR
//         – min(cvar95_pct * 2, 20)   max 20 points from CVaR
//         – min(dd_pct * 3,   30)     max 30 points from drawdown
//         – min(corr * 15,    15)     max 15 points from max correlation
//         – min(conc_score * 0.1, 10) max 10 points from concentration
func ComputeRiskScore(metrics PortfolioMetrics) int {
	score := 100.0
	score -= clamp(metrics.HeatPct*4, 0, 40)
	score -= clamp(metrics.DailyVaR95Pct*3, 0, 25)
	score -= clamp(metrics.CVaR95Pct*2, 0, 20)
	score -= clamp(metrics.DrawdownPct*3, 0, 30)
	score -= clamp(metrics.MaxPairwiseCorr*15, 0, 15)
	score -= clamp(metrics.ConcentrationScore*0.10, 0, 10)

	if score < 0 {
		score = 0
	}
	return int(score)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
