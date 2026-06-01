// Package riskv3 is the Phase 15D Portfolio Risk Engine V3.
//
// It transforms per-trade risk evaluation into portfolio-level risk management,
// thinking like a hedge fund risk desk: Kelly sizing, VaR, CVaR, portfolio heat,
// rolling correlation, concentration detection, and event-sourced audit trail.
//
// Architecture:
//
//	Market Data → Strategies → Risk Engine V3 → OMS v3 → Ledger → Execution
//
// No order reaches OMS v3 unless approved by Risk Engine V3.
// Every decision is a ledger event. All state is replayable.
package riskv3

import "time"

// ─── Institutional hard limits ────────────────────────────────────────────────
// These are the absolute maximum values that must never be exceeded.
// Violations emit RiskViolation events to the ledger and block execution.

const (
	// Position-level limits
	MaxPositionRiskPct = 1.0 // max 1% portfolio risk per single position

	// Strategy-level limits
	MaxStrategyRiskPct     = 5.0  // max 5% total risk allocated to any one strategy
	MaxStrategyLossDayPct  = 3.0  // max 3% daily loss before strategy is suspended

	// Asset concentration limits
	MaxAssetConcentrationPct    = 15.0 // max 15% of portfolio notional in one asset
	MaxDirectionalExposurePct   = 60.0 // max 60% one-directional (all long or all short)

	// Exchange-level limits
	MaxExchangeConcentrationPct = 25.0 // max 25% exposure on any single exchange

	// Portfolio heat limits (sum of position-level dollar risks / equity)
	HeatWarningPct  = 10.0 // emit PortfolioHeatWarning alert
	HeatCriticalPct = 15.0 // emit PortfolioHeatCritical alert, reduce position sizes
	HeatKillPct     = 20.0 // trigger kill switch, no new orders allowed

	// Correlation limits
	MaxCorrelationCoeff         = 0.80 // block correlated positions above this threshold
	MaxCorrelatedExposurePct    = 30.0 // max 30% of equity in correlated cluster

	// Temporal loss limits
	MaxDailyLossPct   = 3.0  // max 3% daily loss → halt trading for the day
	MaxWeeklyLossPct  = 6.0  // max 6% weekly loss → severe position reduction
	MaxDrawdownPct    = 10.0 // max 10% drawdown from HWM → trading halt

	// VaR / CVaR limits
	MaxDailyVaR95Pct = 6.0  // max 6% of equity as daily 95% VaR
	MaxDailyVaR99Pct = 9.0  // max 9% of equity as daily 99% VaR
	MaxCVaR95Pct     = 8.0  // max 8% of equity as 95% CVaR (expected shortfall)
	MaxCVaR99Pct     = 12.0 // max 12% of equity as 99% CVaR

	// Gross exposure limit
	MaxGrossExposurePct = 200.0 // max 2x gross leverage on total portfolio
	MaxNetExposurePct   = 100.0 // max 1x net exposure (long-short netted)

	// Kelly sizing caps
	MaxKellyFractionPct = 5.0 // hard cap: Kelly recommendation never exceeds 5% per trade
	DefaultKellyMode    = "HALF" // full | half | quarter

	// Minimum history for VaR / CVaR computation
	MinVaRHistorySamples = 20 // minimum return samples before VaR is computed

	// Monte Carlo paths for MC-VaR
	MonteCarloVaRPaths = 10_000
)

// ─── Violation types ──────────────────────────────────────────────────────────

type ViolationType string

const (
	ViolationHeatExceeded          ViolationType = "HEAT_EXCEEDED"
	ViolationVaRExceeded           ViolationType = "VAR_EXCEEDED"
	ViolationCVaRExceeded          ViolationType = "CVAR_EXCEEDED"
	ViolationDailyLossExceeded     ViolationType = "DAILY_LOSS_EXCEEDED"
	ViolationWeeklyLossExceeded    ViolationType = "WEEKLY_LOSS_EXCEEDED"
	ViolationDrawdownExceeded      ViolationType = "DRAWDOWN_EXCEEDED"
	ViolationConcentrationExceeded ViolationType = "CONCENTRATION_EXCEEDED"
	ViolationCorrelationExceeded   ViolationType = "CORRELATION_EXCEEDED"
	ViolationExchangeExceeded      ViolationType = "EXCHANGE_EXPOSURE_EXCEEDED"
	ViolationPositionRiskExceeded  ViolationType = "POSITION_RISK_EXCEEDED"
	ViolationGrossExposureExceeded ViolationType = "GROSS_EXPOSURE_EXCEEDED"
	ViolationStrategyRiskExceeded  ViolationType = "STRATEGY_RISK_EXCEEDED"
	ViolationKillSwitchActive      ViolationType = "KILL_SWITCH_ACTIVE"
)

// Violation is a single limit breach detected during pre-trade risk check.
type Violation struct {
	Type        ViolationType `json:"type"`
	Metric      string        `json:"metric"`
	Current     float64       `json:"current"`
	Limit       float64       `json:"limit"`
	Description string        `json:"description"`
}

// ─── Alert severity ───────────────────────────────────────────────────────────

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "INFO"
	AlertSeverityWarning  AlertSeverity = "WARNING"
	AlertSeverityCritical AlertSeverity = "CRITICAL"
	AlertSeverityFatal    AlertSeverity = "FATAL" // triggers kill switch
)

// Alert is a portfolio risk event that may trigger external notifications.
type Alert struct {
	Severity    AlertSeverity `json:"severity"`
	Type        ViolationType `json:"type"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Metric      string        `json:"metric"`
	Current     float64       `json:"current"`
	Limit       float64       `json:"limit"`
	AccountID   string        `json:"account_id"`
	DetectedAt  time.Time     `json:"detected_at"`
}

// ─── Heat levels ──────────────────────────────────────────────────────────────

type HeatLevel string

const (
	HeatNormal   HeatLevel = "NORMAL"    // heat < 10%
	HeatWarning  HeatLevel = "WARNING"   // heat 10–15%: emit warning
	HeatCritical HeatLevel = "CRITICAL"  // heat 15–20%: reduce sizes
	HeatKill     HeatLevel = "KILL"      // heat >= 20%: halt all trading
)

// ClassifyHeat returns the heat level for the given portfolio heat percentage.
func ClassifyHeat(heatPct float64) HeatLevel {
	switch {
	case heatPct >= HeatKillPct:
		return HeatKill
	case heatPct >= HeatCriticalPct:
		return HeatCritical
	case heatPct >= HeatWarningPct:
		return HeatWarning
	default:
		return HeatNormal
	}
}

// ─── Risk decision ────────────────────────────────────────────────────────────

type DecisionStatus string

const (
	DecisionApproved DecisionStatus = "APPROVED"
	DecisionBlocked  DecisionStatus = "BLOCKED"
)

// OrderDecision is the output of a pre-trade risk check by Risk Engine V3.
type OrderDecision struct {
	Status        DecisionStatus `json:"status"`
	Reason        string         `json:"reason"`
	ApprovedSize  float64        `json:"approved_size"`  // may be < requested
	RequestedSize float64        `json:"requested_size"`

	// Risk metrics at decision time
	HeatPct            float64   `json:"heat_pct"`
	HeatLevel          HeatLevel `json:"heat_level"`
	VaR95Pct           float64   `json:"var_95_pct"`
	CVaR95Pct          float64   `json:"cvar_95_pct"`
	DrawdownPct        float64   `json:"drawdown_pct"`
	DailyLossPct       float64   `json:"daily_loss_pct"`
	CorrelationRisk    float64   `json:"correlation_risk"`
	ConcentrationRisk  float64   `json:"concentration_risk"`
	KellyFractionPct   float64   `json:"kelly_fraction_pct"`
	RiskScore          int       `json:"risk_score"` // 0–100; higher = safer

	Violations []Violation `json:"violations,omitempty"`
	CheckedAt  time.Time   `json:"checked_at"`
}

// IsApproved returns true when the order passed all risk checks.
func (d OrderDecision) IsApproved() bool {
	return d.Status == DecisionApproved
}
