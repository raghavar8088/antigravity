package pilot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"antigravity-engine/internal/ledger"
)

// CertificationEngine evaluates live metrics against Phase 22E / Phase 23 go-live requirements
// and generates structured certification reports.
type CertificationEngine struct {
	scaler *CapitalScaler
	ranker *StrategyRanker

	// Go-live minimum requirements (Phase 22E)
	MinTrades         int
	MinProfitFactor   float64
	MinWinRate        float64
	MinSharpe         float64
	MaxDrawdown       float64
	MinPositiveMonths int
}

func NewCertificationEngine() *CertificationEngine {
	return &CertificationEngine{
		scaler:            NewCapitalScaler(),
		ranker:            NewStrategyRanker(),
		MinTrades:         1000,
		MinProfitFactor:   1.30,
		MinWinRate:        0.45,
		MinSharpe:         1.50,
		MaxDrawdown:       0.10,
		MinPositiveMonths: 3,
	}
}

// Certify evaluates a metrics summary and returns the appropriate certification status
// plus the list of reasons (failing conditions or qualifying statements).
func (ce *CertificationEngine) Certify(m LiveMetricsSummary) (CertificationStatus, []string) {
	fails := ce.failingConditions(m)
	if len(fails) > 0 {
		return CertNotReady, fails
	}
	// Graduated thresholds for higher certification tiers
	if m.TotalTrades >= 2000 && m.ProfitFactor >= 1.50 && m.SharpeRatio >= 2.0 && m.PositiveMonths >= 6 {
		return CertReadyForInstitutionalScaling, []string{"all institutional scaling thresholds met"}
	}
	if m.TotalTrades >= 1500 && m.SharpeRatio >= 1.75 && m.PositiveMonths >= 5 {
		return CertReadyForLimitedScaling, []string{"extended track record qualifies for limited scaling"}
	}
	return CertReadyForPilot, []string{"all pilot go-live thresholds met"}
}

// Evaluate triggers a certification update on the aggregate if the status has changed.
func (ce *CertificationEngine) Evaluate(ctx context.Context, agg *PilotAggregate, m *LiveMetrics) {
	summary := m.Summary()
	status, reasons := ce.Certify(summary)
	if status != agg.Certification {
		agg.UpdateCertification(ctx, status, reasons, summary)
	}
}

// EvaluateScaling checks auto scale-up/down rules per strategy and emits the appropriate events.
func (ce *CertificationEngine) EvaluateScaling(ctx context.Context, agg *PilotAggregate, m *LiveMetrics) {
	for strategyID, sm := range m.StrategyMetrics {
		summary := LiveMetricsSummary{
			TotalTrades:    sm.Trades,
			ProfitFactor:   sm.ProfitFactor,
			WinRate:        sm.WinRate,
			SharpeRatio:    sm.Sharpe,
			MaxDrawdown:    sm.MaxDrawdown,
			RiskViolations: m.RiskViolations,
		}
		decision := ce.scaler.Evaluate(summary)
		switch decision.Direction {
		case "DOWN":
			EmitAutoDownscale(ctx, agg.store, agg.AccountID, AutoDownscalePayload{
				PilotID:     agg.PilotID,
				StrategyID:  strategyID,
				Trigger:     decision.Trigger,
				MetricValue: decision.Value,
				Threshold:   decision.Threshold,
				TriggeredAt: time.Now().UTC(),
			})
		case "UP":
			emitPilotEvent(ctx, agg.store, ledger.NewEventInput{
				AggregateType: AggregatePilot,
				AggregateID:   agg.PilotID,
				EventType:     EventPilotAutoUpscaleApproved,
				AccountID:     agg.AccountID,
				StrategyID:    strategyID,
				Payload: map[string]any{
					"pilot_id":    agg.PilotID,
					"strategy_id": strategyID,
					"trigger":     decision.Trigger,
					"reason":      decision.Reason,
					"approved_at": time.Now().UTC(),
				},
				Source: "pilot.certification",
			})
		}
	}
}

// GenerateCertificationReport produces a markdown certification report for LIVE_EXECUTION_CERTIFICATION.md.
func (ce *CertificationEngine) GenerateCertificationReport(agg *PilotAggregate, m *LiveMetrics, ranked []RankedStrategy) string {
	status, reasons := ce.Certify(m.Summary())
	now := time.Now().UTC()
	var sb strings.Builder

	sb.WriteString("# Live Execution Certification Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s  \n", now.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("Pilot ID: `%s`  \nAccount: `%s`\n\n", agg.PilotID, agg.AccountID))

	sb.WriteString("## Certification Verdict\n\n")
	sb.WriteString(fmt.Sprintf("**%s**\n\n", status.String()))

	sb.WriteString("## Go-Live Requirements\n\n")
	sb.WriteString("| Metric | Value | Requirement | Status |\n")
	sb.WriteString("|--------|-------|-------------|--------|\n")
	sb.WriteString(certRow("Total Trades", fmt.Sprintf("%d", m.TotalTrades), fmt.Sprintf("≥ %d", ce.MinTrades), m.TotalTrades >= ce.MinTrades))
	sb.WriteString(certRow("Profit Factor", fmt.Sprintf("%.2f", m.ProfitFactor), fmt.Sprintf("≥ %.2f", ce.MinProfitFactor), m.ProfitFactor >= ce.MinProfitFactor))
	sb.WriteString(certRow("Win Rate", fmt.Sprintf("%.1f%%", m.WinRate*100), fmt.Sprintf("≥ %.0f%%", ce.MinWinRate*100), m.WinRate >= ce.MinWinRate))
	sb.WriteString(certRow("Sharpe Ratio", fmt.Sprintf("%.2f", m.SharpeRatio), fmt.Sprintf("≥ %.2f", ce.MinSharpe), m.SharpeRatio >= ce.MinSharpe))
	sb.WriteString(certRow("Max Drawdown", fmt.Sprintf("%.1f%%", m.MaxDrawdown*100), fmt.Sprintf("< %.0f%%", ce.MaxDrawdown*100), m.MaxDrawdown < ce.MaxDrawdown))
	sb.WriteString(certRow("Positive Months", fmt.Sprintf("%d", m.PositiveMonths), fmt.Sprintf("≥ %d consecutive", ce.MinPositiveMonths), m.PositiveMonths >= ce.MinPositiveMonths))
	sb.WriteString(certRow("Risk Violations", fmt.Sprintf("%d", m.RiskViolations), "0", m.RiskViolations == 0))

	if len(reasons) > 0 {
		label := "Outstanding Requirements"
		if status != CertNotReady {
			label = "Qualifying Conditions"
		}
		sb.WriteString(fmt.Sprintf("\n## %s\n\n", label))
		for _, r := range reasons {
			sb.WriteString(fmt.Sprintf("- %s\n", r))
		}
	}

	sb.WriteString("\n## Execution Quality\n\n")
	sb.WriteString(fmt.Sprintf("- Average Slippage: %.2f bps\n", m.AvgSlippageBps))
	sb.WriteString(fmt.Sprintf("- Average Latency: %.0f ms\n", m.AvgLatencyMs))
	sb.WriteString(fmt.Sprintf("- Total Fees: $%.2f\n", m.TotalFees))
	sb.WriteString(fmt.Sprintf("- Net PnL: $%.2f\n", m.TotalNetPnL))
	sb.WriteString(fmt.Sprintf("- Expectancy: $%.2f per trade\n", m.Expectancy))

	if len(ranked) > 0 {
		sb.WriteString("\n## Strategy Deployment Ranking\n\n")
		sb.WriteString("| Rank | Strategy | Sharpe | PF | Win Rate | Drawdown | Score |\n")
		sb.WriteString("|------|----------|--------|----|----------|----------|-------|\n")
		for _, rs := range ranked {
			sb.WriteString(fmt.Sprintf("| %d | %s | %.2f | %.2f | %.1f%% | %.1f%% | %.3f |\n",
				rs.Rank, rs.StrategyName,
				rs.Metrics.Sharpe, rs.Metrics.ProfitFactor,
				rs.Metrics.WinRate*100, rs.Metrics.MaxDrawdown*100,
				rs.Score))
		}
	}

	sb.WriteString(fmt.Sprintf("\n## Deployment State\n\n"))
	sb.WriteString(fmt.Sprintf("- Current Stage: **%s**\n", agg.Stage.String()))
	sb.WriteString(fmt.Sprintf("- Deployed Strategies: %d\n", len(agg.DeployedStrategies)))
	sb.WriteString(fmt.Sprintf("- Total Capital: $%.0f\n", agg.TotalCapital))
	sb.WriteString(fmt.Sprintf("- Halted: %v\n", agg.Halted))

	return sb.String()
}

// GenerateCapitalScalingFramework produces the CAPITAL_SCALING_FRAMEWORK.md document.
func (ce *CertificationEngine) GenerateCapitalScalingFramework() string {
	up := ce.scaler.Up
	dn := ce.scaler.Down
	var sb strings.Builder

	sb.WriteString("# Capital Scaling Framework — Phase 23\n\n")
	sb.WriteString("## Scale-Up Requirements (ALL must be met)\n\n")
	sb.WriteString("| Metric | Threshold |\n")
	sb.WriteString("|--------|-----------|\n")
	sb.WriteString(fmt.Sprintf("| Live Trades | ≥ %d |\n", up.MinTrades))
	sb.WriteString(fmt.Sprintf("| Profit Factor | ≥ %.2f |\n", up.MinProfitFactor))
	sb.WriteString(fmt.Sprintf("| Win Rate | ≥ %.0f%% |\n", up.MinWinRate*100))
	sb.WriteString(fmt.Sprintf("| Sharpe Ratio | ≥ %.2f |\n", up.MinSharpe))
	sb.WriteString(fmt.Sprintf("| Max Drawdown | < %.0f%% |\n", up.MaxDrawdownPct*100))
	sb.WriteString(fmt.Sprintf("| Risk Violations | = %d |\n", up.MaxRiskViolations))

	sb.WriteString("\n## Automatic Downscale Triggers (ANY single breach)\n\n")
	sb.WriteString("| Condition | Threshold |\n")
	sb.WriteString("|-----------|----------|\n")
	sb.WriteString(fmt.Sprintf("| Profit Factor | < %.2f |\n", dn.MinProfitFactor))
	sb.WriteString(fmt.Sprintf("| Sharpe Ratio | < %.2f |\n", dn.MinSharpe))
	sb.WriteString(fmt.Sprintf("| Max Drawdown | > %.0f%% |\n", dn.MaxDrawdownPct*100))
	sb.WriteString(fmt.Sprintf("| Consecutive Losing Weeks | ≥ %d |\n", dn.MaxConsecutiveLosses))
	sb.WriteString("| Risk Violation Detected | any |\n")

	sb.WriteString("\n## Deployment Stages\n\n")
	sb.WriteString("| Stage | Strategies | Capital per Strategy | Duration |\n")
	sb.WriteString("|-------|-----------|----------------------|----------|\n")
	sb.WriteString("| Stage 1 | Top 3 | 0.25–0.50% | 30 days |\n")
	sb.WriteString("| Stage 2 | Top 5 | 0.50–1.00% | 30 days |\n")
	sb.WriteString("| Stage 3 | Top 10 | 1.00–2.00% | 60 days |\n")
	sb.WriteString("| Stage 4 | Full portfolio | 2.00–5.00% | Open-ended |\n")

	sb.WriteString("\n## Final Certification Tiers\n\n")
	sb.WriteString("| Status | Requirements |\n")
	sb.WriteString("|--------|--------------|\n")
	sb.WriteString("| NOT READY FOR LIVE CAPITAL | Any go-live threshold not met |\n")
	sb.WriteString("| READY FOR PILOT DEPLOYMENT | All go-live thresholds met (1000+ trades) |\n")
	sb.WriteString("| READY FOR LIMITED SCALING | 1500+ trades, Sharpe ≥ 1.75, 5+ positive months |\n")
	sb.WriteString("| READY FOR INSTITUTIONAL SCALING | 2000+ trades, PF ≥ 1.50, Sharpe ≥ 2.0, 6+ positive months |\n")

	return sb.String()
}

func (ce *CertificationEngine) failingConditions(m LiveMetricsSummary) []string {
	var fails []string
	if m.TotalTrades < ce.MinTrades {
		fails = append(fails, fmt.Sprintf("trades %d < %d required", m.TotalTrades, ce.MinTrades))
	}
	if m.ProfitFactor < ce.MinProfitFactor {
		fails = append(fails, fmt.Sprintf("profit factor %.2f < %.2f", m.ProfitFactor, ce.MinProfitFactor))
	}
	if m.WinRate < ce.MinWinRate {
		fails = append(fails, fmt.Sprintf("win rate %.1f%% < %.1f%%", m.WinRate*100, ce.MinWinRate*100))
	}
	if m.SharpeRatio < ce.MinSharpe {
		fails = append(fails, fmt.Sprintf("Sharpe %.2f < %.2f", m.SharpeRatio, ce.MinSharpe))
	}
	if m.MaxDrawdown > ce.MaxDrawdown {
		fails = append(fails, fmt.Sprintf("max drawdown %.1f%% > %.1f%%", m.MaxDrawdown*100, ce.MaxDrawdown*100))
	}
	if m.PositiveMonths < ce.MinPositiveMonths {
		fails = append(fails, fmt.Sprintf("positive months %d < %d required", m.PositiveMonths, ce.MinPositiveMonths))
	}
	if m.RiskViolations > 0 {
		fails = append(fails, fmt.Sprintf("%d risk violation(s) recorded", m.RiskViolations))
	}
	return fails
}

func certRow(label, value, req string, pass bool) string {
	status := "FAIL"
	if pass {
		status = "PASS"
	}
	return fmt.Sprintf("| %s | %s | %s | %s |\n", label, value, req, status)
}
