package phase22e

import "fmt"

// Certify evaluates a ValidationResult and issues the go-live certification.
// It populates Result.Status, .Passed, .Failed, .Warnings, and .Narrative.
func Certify(result *ValidationResult) {
	pm := result.Portfolio
	strategies := result.Strategies

	passed := []string{}
	failed := []string{}
	warnings := []string{}

	// ── Hard criteria ─────────────────────────────────────────────────────────

	checkHard := func(condition bool, pass, fail string) {
		if condition {
			passed = append(passed, pass)
		} else {
			failed = append(failed, fail)
		}
	}

	checkHard(pm.Checkpoint1000,
		fmt.Sprintf("Total trades %d ≥ %d", pm.TotalTrades, MinTrades1000),
		fmt.Sprintf("Total trades %d < %d required", pm.TotalTrades, MinTrades1000))

	checkHard(pm.ProfitFactor >= MinProfitFactor,
		fmt.Sprintf("Profit Factor %.2f ≥ %.2f", pm.ProfitFactor, MinProfitFactor),
		fmt.Sprintf("Profit Factor %.2f < %.2f required", pm.ProfitFactor, MinProfitFactor))

	checkHard(pm.WinRate >= MinWinRate,
		fmt.Sprintf("Win Rate %.1f%% ≥ %.0f%%", pm.WinRate*100, MinWinRate*100),
		fmt.Sprintf("Win Rate %.1f%% < %.0f%% required", pm.WinRate*100, MinWinRate*100))

	checkHard(pm.Sharpe >= MinSharpe,
		fmt.Sprintf("Sharpe Ratio %.2f ≥ %.2f", pm.Sharpe, MinSharpe),
		fmt.Sprintf("Sharpe Ratio %.2f < %.2f required", pm.Sharpe, MinSharpe))

	checkHard(pm.MaxDrawdown < MaxDrawdownPct,
		fmt.Sprintf("Max Drawdown %.1f%% < %.0f%%", pm.MaxDrawdown, MaxDrawdownPct),
		fmt.Sprintf("Max Drawdown %.1f%% ≥ %.0f%% limit", pm.MaxDrawdown, MaxDrawdownPct))

	checkHard(pm.ConsecPositiveMonths >= MinConsecPositiveMon,
		fmt.Sprintf("Consecutive Positive Months %d ≥ %d", pm.ConsecPositiveMonths, MinConsecPositiveMon),
		fmt.Sprintf("Consecutive Positive Months %d < %d required", pm.ConsecPositiveMonths, MinConsecPositiveMon))

	checkHard(pm.StatSig,
		"Statistical significance confirmed (binomial z > 1.96)",
		"Statistical significance not confirmed (need z > 1.96)")

	// ── Soft criteria → warnings ──────────────────────────────────────────────

	if pm.Checkpoint500 && !pm.Checkpoint1000 {
		warnings = append(warnings, fmt.Sprintf("500-trade checkpoint passed but 1000-trade checkpoint not yet reached (%d/%d)", pm.TotalTrades, MinTrades1000))
	}

	// Check all four regimes have data
	for _, r := range []Regime{RegimeBull, RegimeBear, RegimeRange, RegimeVolatile} {
		rp, ok := pm.RegimePerf[r]
		if !ok || rp.Trades < 20 {
			warnings = append(warnings, fmt.Sprintf("Regime %s has limited data (%d trades) — cross-regime stability unconfirmed", r, func() int {
				if ok {
					return rp.Trades
				}
				return 0
			}()))
		} else if rp.ProfitFactor < 1.0 {
			warnings = append(warnings, fmt.Sprintf("Regime %s shows negative edge (PF=%.2f) — exposure during this regime should be reduced", r, rp.ProfitFactor))
		}
	}

	// Check for outlier dependency
	if pm.OutlierWinPct > 50 {
		warnings = append(warnings, fmt.Sprintf("%.0f%% of gross profit comes from top 5%% of trades — results may be fragile to tail events", pm.OutlierWinPct))
	}

	// Live vs backtest degradation
	approvedCount := 0
	degradedCount := 0
	for _, s := range strategies {
		if s.Approved {
			approvedCount++
			if !s.LiveVsBacktestOK && s.LivePF > 0 {
				degradedCount++
			}
		}
	}
	if degradedCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d/%d approved strategies show live-vs-backtest degradation > %.0f%% — investigate execution quality", degradedCount, approvedCount, MaxLiveVsBacktestDeg*100))
	}

	if pm.ReturnSkew < -1.0 {
		warnings = append(warnings, fmt.Sprintf("Return distribution has negative skew (%.2f) — left-tail risk present", pm.ReturnSkew))
	}

	// ── Determine status ──────────────────────────────────────────────────────

	hardFailCount := len(failed)
	var status CertificationStatus

	switch {
	case hardFailCount == 0 && len(warnings) == 0:
		status = StatusApprovedScaling
	case hardFailCount == 0:
		status = StatusApproved
	case hardFailCount <= 2:
		status = StatusConditional
	default:
		status = StatusNotApproved
	}

	// ── Narrative ─────────────────────────────────────────────────────────────

	narrative := buildNarrative(status, pm, approvedCount, len(strategies), passed, failed, warnings)

	result.Status = status
	result.Passed = passed
	result.Failed = failed
	result.Warnings = warnings
	result.Narrative = narrative
}

func buildNarrative(status CertificationStatus, pm PortfolioMetrics, approvedStrategies, totalStrategies int, passed, failed, warnings []string) string {
	switch status {
	case StatusApprovedScaling:
		return fmt.Sprintf(
			"The trading system has passed all Phase 22E go-live criteria without exception. "+
				"Across %d validated trades, the portfolio achieved a Profit Factor of %.2f, Win Rate of %.1f%%, "+
				"Sharpe of %.2f, and Maximum Drawdown of %.1f%%. "+
				"%d consecutive profitable months confirm temporal stability. "+
				"%d of %d strategies are approved for deployment. "+
				"The system is certified for capital scaling — incremental capital expansion may proceed in validated tranches.",
			pm.TotalTrades, pm.ProfitFactor, pm.WinRate*100, pm.Sharpe, pm.MaxDrawdown,
			pm.ConsecPositiveMonths, approvedStrategies, totalStrategies)

	case StatusApproved:
		return fmt.Sprintf(
			"The trading system has met all mandatory Phase 22E go-live criteria. "+
				"Portfolio metrics: PF=%.2f, WR=%.1f%%, Sharpe=%.2f, MaxDD=%.1f%% across %d trades. "+
				"%d of %d strategies are approved. %d advisory warnings require monitoring before capital expansion is considered. "+
				"The system is approved for limited live capital deployment at prescribed sizes.",
			pm.ProfitFactor, pm.WinRate*100, pm.Sharpe, pm.MaxDrawdown, pm.TotalTrades,
			approvedStrategies, totalStrategies, len(warnings))

	case StatusConditional:
		return fmt.Sprintf(
			"The trading system partially meets Phase 22E criteria. "+
				"%d of %d hard criteria passed; %d failed. "+
				"Conditional approval is granted for paper trading continuation and targeted remediation. "+
				"Live capital deployment is NOT authorised until all failed criteria are resolved. "+
				"Key failures: %v.",
			len(passed), len(passed)+len(failed), len(failed), failed)

	default:
		return fmt.Sprintf(
			"The trading system does NOT meet Phase 22E go-live criteria. "+
				"%d of %d mandatory criteria failed. "+
				"Live capital deployment is DENIED. Continue paper trading, resolve all failures, "+
				"and re-run Phase 22E validation before re-applying for certification.",
			len(failed), len(passed)+len(failed))
	}
}

// ScalingThresholds returns the capital expansion gates after initial approval.
func ScalingThresholds() []ScalingGate {
	return []ScalingGate{
		{CapitalUSD: 10_000, MinTrades: 100, MinPF: 1.30, MinSharpe: 1.50, Label: "Tranche 1 — Seed Capital"},
		{CapitalUSD: 50_000, MinTrades: 300, MinPF: 1.35, MinSharpe: 1.60, Label: "Tranche 2 — Early Capital"},
		{CapitalUSD: 200_000, MinTrades: 1000, MinPF: 1.40, MinSharpe: 1.70, Label: "Tranche 3 — Growth Capital"},
		{CapitalUSD: 1_000_000, MinTrades: 2000, MinPF: 1.45, MinSharpe: 1.80, Label: "Tranche 4 — Institutional Scale"},
	}
}

// ScalingGate defines minimum metrics required before releasing more capital.
type ScalingGate struct {
	CapitalUSD float64
	MinTrades  int
	MinPF      float64
	MinSharpe  float64
	Label      string
}
