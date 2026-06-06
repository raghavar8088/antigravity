package phase27

import (
	"fmt"
	"strings"
)

// BuildReports generates all Phase 27 markdown reports as a map keyed by name.
func BuildReports(r *Phase27Result) map[string]string {
	reports := make(map[string]string)
	reports["attribution"] = buildAttributionReport(r)
	reports["alpha_performance"] = buildAlphaPerfReport(r)
	reports["pareto"] = buildParetoReport(r)
	reports["championship"] = buildChampionshipReport(r)
	reports["overlap"] = buildOverlapReport(r)
	reports["executive_summary"] = buildSummary27(r)
	return reports
}

func buildAttributionReport(r *Phase27Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 27 — Alpha Attribution\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Total Records: %d\n\n", len(r.AttributionRecords)))
	sb.WriteString("| TradeID | Strategy | Family | NetPnL | Win |\n")
	sb.WriteString("|---------|----------|--------|--------|-----|\n")
	for _, rec := range r.AttributionRecords {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | $%.2f | %v |\n",
			rec.TradeID, rec.StrategyName, rec.AlphaFamily, rec.NetPnLUSD, rec.IsWin))
	}
	return sb.String()
}

func buildAlphaPerfReport(r *Phase27Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 27 — Alpha Family Performance\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Total Platform PnL: $%.2f\n\n", r.TotalPlatformPnL))
	sb.WriteString("| Family | Trades | WinRate | PF | Sharpe | Expectancy | NetPnL | Contribution% |\n")
	sb.WriteString("|--------|--------|---------|-----|--------|-----------|--------|---------------|\n")
	for _, p := range r.AlphaPerformance {
		sb.WriteString(fmt.Sprintf("| %s | %d | %.2f | %.3f | %.3f | $%.2f | $%.2f | %.1f%% |\n",
			p.Family, p.Trades, p.WinRate, p.ProfitFactor, p.Sharpe, p.Expectancy, p.NetPnLUSD, p.ProfitContributionPct))
	}
	if len(r.InsufficientEvidence) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Insufficient Evidence (<30 trades):** %s\n", strings.Join(r.InsufficientEvidence, ", ")))
	}
	return sb.String()
}

func buildParetoReport(r *Phase27Result) string {
	pa := r.ParetoAnalysis
	var sb strings.Builder
	sb.WriteString("# Phase 27 — Pareto Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- Top 1 Alpha (%s): %.1f%% of profit\n", pa.Top1Family, pa.Top1AlphaPct))
	sb.WriteString(fmt.Sprintf("- Top 3 Alphas: %.1f%% of profit\n", pa.Top3AlphaPct))
	sb.WriteString(fmt.Sprintf("- Top 5 Alphas: %.1f%% of profit\n", pa.Top5AlphaPct))
	return sb.String()
}

func buildChampionshipReport(r *Phase27Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 27 — Alpha Championship\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	for _, c := range r.Championship {
		sb.WriteString(fmt.Sprintf("**#%d %s** — %s\n", c.Rank, c.Family, c.Verdict))
		sb.WriteString(fmt.Sprintf("  %s\n\n", c.Evidence))
	}
	return sb.String()
}

func buildOverlapReport(r *Phase27Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 27 — Overlap Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString("| FamilyA | FamilyB | TradeOverlap% | ReturnCorr | Redundant |\n")
	sb.WriteString("|---------|---------|----------------|------------|----------|\n")
	for _, ov := range r.OverlapAnalysis {
		sb.WriteString(fmt.Sprintf("| %s | %s | %.1f | %.4f | %v |\n",
			ov.FamilyA, ov.FamilyB, ov.TradeCorrelation, ov.ReturnCorrelation, ov.Redundant))
	}
	if len(r.RedundantFamilies) > 0 {
		fams := make([]string, len(r.RedundantFamilies))
		for i, f := range r.RedundantFamilies {
			fams[i] = string(f)
		}
		sb.WriteString(fmt.Sprintf("\n**Redundant Families:** %s\n", strings.Join(fams, ", ")))
	}
	return sb.String()
}

func buildSummary27(r *Phase27Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 27 — Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- Total Trades Attributed: %d\n", len(r.AttributionRecords)))
	sb.WriteString(fmt.Sprintf("- Alpha Families Analyzed: %d\n", len(r.AlphaPerformance)))
	sb.WriteString(fmt.Sprintf("- Total Platform PnL: $%.2f\n", r.TotalPlatformPnL))
	sb.WriteString(fmt.Sprintf("- Redundant Families: %d\n", len(r.RedundantFamilies)))
	sb.WriteString(fmt.Sprintf("- Insufficient Evidence Families: %d\n", len(r.InsufficientEvidence)))
	if len(r.Championship) > 0 {
		sb.WriteString(fmt.Sprintf("- Champion Alpha: %s (%s)\n", r.Championship[0].Family, r.Championship[0].Verdict))
	}
	return sb.String()
}
