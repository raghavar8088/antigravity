package pilot

import "fmt"

// ScaleUpThresholds defines minimum performance requirements that must ALL be met
// before capital may be scaled up to the next deployment stage.
type ScaleUpThresholds struct {
	MinTrades         int     // minimum live trade count
	MinProfitFactor   float64 // profit factor floor
	MinWinRate        float64 // win rate floor (0–1)
	MinSharpe         float64 // annualized Sharpe floor
	MaxDrawdownPct    float64 // maximum acceptable drawdown (0–1)
	MaxRiskViolations int     // zero tolerance
}

// DefaultScaleUpThresholds returns the Phase 23 certified scale-up thresholds.
func DefaultScaleUpThresholds() ScaleUpThresholds {
	return ScaleUpThresholds{
		MinTrades:         500,
		MinProfitFactor:   1.30,
		MinWinRate:        0.45,
		MinSharpe:         1.50,
		MaxDrawdownPct:    0.10,
		MaxRiskViolations: 0,
	}
}

// ScaleDownThresholds defines breach conditions that trigger AUTOMATIC capital reduction.
// Any single breach triggers downscaling immediately.
type ScaleDownThresholds struct {
	MinProfitFactor      float64 // PF below this → downscale
	MinSharpe            float64 // Sharpe below this → downscale
	MaxDrawdownPct       float64 // DD above this → downscale
	MaxConsecutiveLosses int     // consecutive losing weeks → downscale
}

// DefaultScaleDownThresholds returns the Phase 23 automatic downscale triggers.
func DefaultScaleDownThresholds() ScaleDownThresholds {
	return ScaleDownThresholds{
		MinProfitFactor:      1.10,
		MinSharpe:            1.00,
		MaxDrawdownPct:       0.10,
		MaxConsecutiveLosses: 3,
	}
}

// ScaleDecision is the output of a scaling evaluation.
type ScaleDecision struct {
	Direction string  // UP | DOWN | HOLD
	Trigger   string  // which threshold triggered the decision
	Value     float64 // the current metric value
	Threshold float64 // the threshold that was breached or met
	Reason    string  // human-readable explanation
}

// CapitalScaler evaluates live metrics against Phase 23 scaling rules.
type CapitalScaler struct {
	Up   ScaleUpThresholds
	Down ScaleDownThresholds
}

func NewCapitalScaler() *CapitalScaler {
	return &CapitalScaler{
		Up:   DefaultScaleUpThresholds(),
		Down: DefaultScaleDownThresholds(),
	}
}

// Evaluate checks current metrics against both threshold sets and returns the
// highest-priority scaling decision. Downscale conditions take priority over scale-up.
func (cs *CapitalScaler) Evaluate(m LiveMetricsSummary) ScaleDecision {
	// Downscale: any single breach triggers immediately
	if m.RiskViolations > 0 {
		return ScaleDecision{
			Direction: "DOWN", Trigger: "RISK_VIOLATION",
			Value: float64(m.RiskViolations), Threshold: 0,
			Reason: fmt.Sprintf("%d risk violation(s) detected", m.RiskViolations),
		}
	}
	if m.ProfitFactor < cs.Down.MinProfitFactor {
		return ScaleDecision{
			Direction: "DOWN", Trigger: "PF_BREACH",
			Value: m.ProfitFactor, Threshold: cs.Down.MinProfitFactor,
			Reason: fmt.Sprintf("PF %.2f below floor %.2f", m.ProfitFactor, cs.Down.MinProfitFactor),
		}
	}
	if m.SharpeRatio < cs.Down.MinSharpe {
		return ScaleDecision{
			Direction: "DOWN", Trigger: "SHARPE_BREACH",
			Value: m.SharpeRatio, Threshold: cs.Down.MinSharpe,
			Reason: fmt.Sprintf("Sharpe %.2f below floor %.2f", m.SharpeRatio, cs.Down.MinSharpe),
		}
	}
	if m.MaxDrawdown > cs.Down.MaxDrawdownPct {
		return ScaleDecision{
			Direction: "DOWN", Trigger: "DD_BREACH",
			Value: m.MaxDrawdown, Threshold: cs.Down.MaxDrawdownPct,
			Reason: fmt.Sprintf("drawdown %.1f%% exceeds limit %.1f%%", m.MaxDrawdown*100, cs.Down.MaxDrawdownPct*100),
		}
	}
	if m.ConsecutiveLosses >= cs.Down.MaxConsecutiveLosses {
		return ScaleDecision{
			Direction: "DOWN", Trigger: "LOSING_WEEKS",
			Value: float64(m.ConsecutiveLosses), Threshold: float64(cs.Down.MaxConsecutiveLosses),
			Reason: fmt.Sprintf("%d consecutive losing weeks", m.ConsecutiveLosses),
		}
	}

	// Scale-up: all conditions must be met simultaneously
	if cs.allScaleUpMet(m) {
		return ScaleDecision{
			Direction: "UP", Trigger: "ALL_THRESHOLDS_MET",
			Reason: "all scale-up thresholds satisfied",
		}
	}

	return ScaleDecision{
		Direction: "HOLD",
		Reason:    "within acceptable range; scale-up thresholds not yet met",
	}
}

// FailingConditions returns a list of unmet scale-up conditions for diagnostics and reporting.
func (cs *CapitalScaler) FailingConditions(m LiveMetricsSummary) []string {
	var fails []string
	if m.TotalTrades < cs.Up.MinTrades {
		fails = append(fails, fmt.Sprintf("trades %d < %d required", m.TotalTrades, cs.Up.MinTrades))
	}
	if m.ProfitFactor < cs.Up.MinProfitFactor {
		fails = append(fails, fmt.Sprintf("profit factor %.2f < %.2f", m.ProfitFactor, cs.Up.MinProfitFactor))
	}
	if m.WinRate < cs.Up.MinWinRate {
		fails = append(fails, fmt.Sprintf("win rate %.1f%% < %.1f%%", m.WinRate*100, cs.Up.MinWinRate*100))
	}
	if m.SharpeRatio < cs.Up.MinSharpe {
		fails = append(fails, fmt.Sprintf("Sharpe %.2f < %.2f", m.SharpeRatio, cs.Up.MinSharpe))
	}
	if m.MaxDrawdown > cs.Up.MaxDrawdownPct {
		fails = append(fails, fmt.Sprintf("max drawdown %.1f%% > %.1f%%", m.MaxDrawdown*100, cs.Up.MaxDrawdownPct*100))
	}
	if m.RiskViolations > cs.Up.MaxRiskViolations {
		fails = append(fails, fmt.Sprintf("risk violations %d > %d allowed", m.RiskViolations, cs.Up.MaxRiskViolations))
	}
	return fails
}

func (cs *CapitalScaler) allScaleUpMet(m LiveMetricsSummary) bool {
	return m.TotalTrades >= cs.Up.MinTrades &&
		m.ProfitFactor >= cs.Up.MinProfitFactor &&
		m.WinRate >= cs.Up.MinWinRate &&
		m.SharpeRatio >= cs.Up.MinSharpe &&
		m.MaxDrawdown <= cs.Up.MaxDrawdownPct &&
		m.RiskViolations <= cs.Up.MaxRiskViolations
}
