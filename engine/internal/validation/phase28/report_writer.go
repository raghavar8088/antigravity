package phase28

import (
	"fmt"
	"strings"
)

// BuildReports generates all Phase 28 markdown reports.
func BuildReports(r *Phase28Result) map[string]string {
	reports := make(map[string]string)
	reports["correlation"] = buildCorrReport(r)
	reports["portfolio_a"] = buildPortfolioReport(r, "A — Max Sharpe", r.PortfolioA)
	reports["portfolio_b"] = buildPortfolioReport(r, "B — Max CAGR", r.PortfolioB)
	reports["portfolio_c"] = buildPortfolioReport(r, "C — Min Drawdown", r.PortfolioC)
	reports["portfolio_d"] = buildPortfolioReport(r, "D — Institutional", r.PortfolioD)
	reports["allocations"] = buildAllocationsReport(r)
	reports["failure_analysis"] = buildFailureReport(r)
	reports["verdict"] = buildVerdictReport28(r)
	return reports
}

func buildCorrReport(r *Phase28Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 28 — Correlation Matrix\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Strategies: %s\n\n", strings.Join(r.CorrelationMatrix.Strategies, ", ")))
	for _, sA := range r.CorrelationMatrix.Strategies {
		for _, sB := range r.CorrelationMatrix.Strategies {
			if sA >= sB {
				continue
			}
			val := r.CorrelationMatrix.Matrix[sA][sB]
			sb.WriteString(fmt.Sprintf("- %s vs %s: r=%.4f\n", sA, sB, val))
		}
	}
	return sb.String()
}

func buildPortfolioReport(r *Phase28Result, name string, p Portfolio28) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Phase 28 — Portfolio %s\n\n", name))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- ExpectedSharpe: %.3f\n", p.ExpectedSharpe))
	sb.WriteString(fmt.Sprintf("- ExpectedPF: %.3f\n", p.ExpectedPF))
	sb.WriteString(fmt.Sprintf("- ExpectedMaxDD: %.2f%%\n", p.ExpectedMaxDD))
	sb.WriteString(fmt.Sprintf("- ExpectedRoR: %.4f\n", p.ExpectedRoR))
	sb.WriteString(fmt.Sprintf("- AverageCorrelation: %.4f\n", p.AverageCorrelation))
	sb.WriteString(fmt.Sprintf("- DiversScore: %.1f\n\n", p.DiversScore))
	sb.WriteString(fmt.Sprintf("Evidence: %s\n\n", p.Evidence))
	sb.WriteString("| Strategy | Weight | Alloc% | AllocUSD1k | AllocUSD1M |\n")
	sb.WriteString("|----------|--------|--------|------------|------------|\n")
	for _, pw := range p.Weights {
		sb.WriteString(fmt.Sprintf("| %s | %.4f | %.2f%% | $%.0f | $%.0f |\n",
			pw.StrategyName, pw.Weight, pw.AllocationPct, pw.AllocUSD1k, pw.AllocUSD1M))
	}
	return sb.String()
}

func buildAllocationsReport(r *Phase28Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 28 — Strategy Allocations\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString("| Strategy | CompositeScore | AllocationPct | PFScore | SharpeScore |\n")
	sb.WriteString("|----------|----------------|---------------|---------|-------------|\n")
	for _, a := range r.StrategyAllocations {
		sb.WriteString(fmt.Sprintf("| %s | %.2f | %.2f%% | %.1f | %.1f |\n",
			a.StrategyName, a.CompositeScore, a.AllocationPct, a.PFScore, a.SharpeScore))
	}
	return sb.String()
}

func buildFailureReport(r *Phase28Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 28 — Failure Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	for _, f := range r.FailureAnalysis {
		sb.WriteString(fmt.Sprintf("## %s — %s\n", f.StrategyName, f.Recommendation))
		sb.WriteString(fmt.Sprintf("- ReducesQuality: %v\n", f.ReducesPortfolioQuality))
		sb.WriteString(fmt.Sprintf("- IncreasesDD: %v | DecreasesSharpe: %v\n", f.IncreasesDrawdown, f.DecreasesSharpe))
		sb.WriteString(fmt.Sprintf("- UnstableEdge: %v | PoorRetention: %v\n", f.UnstableEdge, f.PoorRetention))
		sb.WriteString(fmt.Sprintf("- Evidence: %s\n\n", f.Evidence))
	}
	return sb.String()
}

func buildVerdictReport28(r *Phase28Result) string {
	v := r.FinalVerdict
	var sb strings.Builder
	sb.WriteString("# Phase 28 — Final 17-Question Verdict\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", v.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Q1** HasVerifiedEdge: %v — %s\n\n", v.Q1_HasVerifiedEdge, v.Q1_Evidence))
	sb.WriteString(fmt.Sprintf("**Q2** StrategiesWithEdge: %s\n\n", strings.Join(v.Q2_StrategiesWithEdge, ", ")))
	sb.WriteString(fmt.Sprintf("**Q3** CapitalStrategies: %s\n\n", strings.Join(v.Q3_CapitalStrategies, ", ")))
	sb.WriteString(fmt.Sprintf("**Q4** RetiredStrategies: %s\n\n", strings.Join(v.Q4_RetiredStrategies, ", ")))
	sb.WriteString(fmt.Sprintf("**Q5** TopAlphaFamily: %s\n\n", v.Q5_TopAlphaFamily))
	sb.WriteString(fmt.Sprintf("**Q6** AlphaToRemove: %s\n\n", v.Q6_AlphaToRemove))
	sb.WriteString(fmt.Sprintf("**Q7** ParetoConcentration — Top1: %.1f%% Top3: %.1f%% Top5: %.1f%%\n\n", v.Q7_Top1AlphaPct, v.Q7_Top3AlphaPct, v.Q7_Top5AlphaPct))
	sb.WriteString(fmt.Sprintf("**Q11** ExpectedAnnualReturn: %.2f%%\n\n", v.Q11_ExpectedAnnualReturn))
	sb.WriteString(fmt.Sprintf("**Q12** ExpectedSharpe: %.3f\n\n", v.Q12_ExpectedSharpe))
	sb.WriteString(fmt.Sprintf("**Q13** ExpectedMaxDD: %.2f%%\n\n", v.Q13_ExpectedMaxDD))
	sb.WriteString(fmt.Sprintf("**Q14** ExpectedRoR: %.4f\n\n", v.Q14_ExpectedRoR))
	sb.WriteString(fmt.Sprintf("**Q15** InstitutionalJustified: %v — %s\n\n", v.Q15_InstitutionalJustified, v.Q15_Evidence))
	sb.WriteString(fmt.Sprintf("**Q16** ScaleCapital: %v — %s\n\n", v.Q16_ScaleCapitalJustified, v.Q16_Evidence))
	sb.WriteString(fmt.Sprintf("**Q17** CreateNewStrategies: %v — %s\n\n", v.Q17_CreateNewStrategies, v.Q17_Reasoning))
	sb.WriteString(fmt.Sprintf("---\n**Overall: %s**\n%s\n", v.OverallVerdict, v.Justification))
	return sb.String()
}
