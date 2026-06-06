package phase26

import (
	"fmt"
	"strings"
)

// BuildReports generates all Phase 26 markdown reports as strings.
func BuildReports(r *Phase26Result) map[string]string {
	reports := make(map[string]string)
	reports["eligibility"] = buildEligibilityReport(r)
	reports["tier1_validation"] = buildTierReport(r, "Tier 1", r.Tier1Strategies)
	reports["tier2_validation"] = buildTierReport(r, "Tier 2", r.Tier2Strategies)
	reports["edge_retention"] = buildEdgeRetentionReport(r)
	reports["drift_detection"] = buildDriftReport(r)
	reports["certification"] = buildCertificationReport(r)
	reports["verdict"] = buildVerdictReport(r)
	reports["executive_summary"] = buildExecutiveSummary(r)
	return reports
}

func buildEligibilityReport(r *Phase26Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 26 — Strategy Eligibility\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("Total Eligible: %d\n\n", r.TotalEligible))
	sb.WriteString("| Strategy | Phase24Tier | Phase25Drift | EligibleTier | ExclusionReason |\n")
	sb.WriteString("|----------|-------------|--------------|--------------|------------------|\n")
	for _, e := range r.EligibleStrategies {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			e.StrategyName, e.Phase24Tier, e.Phase25Drift, e.EligibleTier, e.ExclusionReason))
	}
	return sb.String()
}

func buildTierReport(r *Phase26Result, tierName string, certs []CertificationRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Phase 26 — %s Certification\n\n", tierName))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	for _, c := range certs {
		sb.WriteString(fmt.Sprintf("## %s\n", c.StrategyName))
		sb.WriteString(fmt.Sprintf("- Trades: %d\n- PF: %.3f\n- Sharpe: %.3f\n- MaxDD: %.2f%%\n- Decision: %s\n", c.TotalTrades, c.ProfitFactor, c.Sharpe, c.MaxDD, c.Decision))
		sb.WriteString(fmt.Sprintf("- Gates Passed: %s\n", strings.Join(c.GatesPassed, ", ")))
		sb.WriteString(fmt.Sprintf("- Gates Failed: %s\n\n", strings.Join(c.GatesFailed, ", ")))
	}
	if len(certs) == 0 {
		sb.WriteString("INSUFFICIENT_EVIDENCE: no strategies met criteria\n")
	}
	return sb.String()
}

func buildEdgeRetentionReport(r *Phase26Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 26 — Edge Retention Analysis\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString("| Strategy | PFRetention% | SharpeRetention% | Class |\n")
	sb.WriteString("|----------|-------------|------------------|-------|\n")
	for _, er := range r.EdgeRetention {
		sb.WriteString(fmt.Sprintf("| %s | %.1f | %.1f | %s |\n", er.StrategyName, er.PFRetentionPct, er.SharpeRetentionPct, er.RetentionClass))
	}
	return sb.String()
}

func buildDriftReport(r *Phase26Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 26 — Drift Detection\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	for _, d := range r.DriftRecords {
		sb.WriteString(fmt.Sprintf("## %s\n", d.StrategyName))
		sb.WriteString(fmt.Sprintf("- DriftClass: %s\n- Trend: %s\n- Evidence: %s\n\n", d.DriftClass, d.Trend, d.Evidence))
	}
	return sb.String()
}

func buildCertificationReport(r *Phase26Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 26 — Certification Decisions\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- Certified (PROMOTE): %s\n", strings.Join(r.InstitutionalCertified, ", ")))
	sb.WriteString(fmt.Sprintf("- Maintained: %s\n", strings.Join(r.Maintained, ", ")))
	sb.WriteString(fmt.Sprintf("- Reduced: %s\n", strings.Join(r.Reduced, ", ")))
	sb.WriteString(fmt.Sprintf("- Demoted: %s\n", strings.Join(r.Demoted, ", ")))
	sb.WriteString(fmt.Sprintf("- Retired: %s\n", strings.Join(r.Retired, ", ")))
	sb.WriteString(fmt.Sprintf("- Insufficient Evidence: %s\n", strings.Join(r.InsufficientEvidenceStrategies, ", ")))
	return sb.String()
}

func buildVerdictReport(r *Phase26Result) string {
	v := r.FinalVerdict
	var sb strings.Builder
	sb.WriteString("# Phase 26 — Final Verdict\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", v.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- HasVerifiedEdge: %v\n", v.HasVerifiedEdge))
	sb.WriteString(fmt.Sprintf("- Evidence: %s\n", v.VerifiedEdgeEvidence))
	sb.WriteString(fmt.Sprintf("- OverallDecision: %s\n", v.OverallDecision))
	sb.WriteString(fmt.Sprintf("- Justification: %s\n", v.Justification))
	sb.WriteString(fmt.Sprintf("- PlatformPF: %.3f\n", v.PlatformPF))
	sb.WriteString(fmt.Sprintf("- PlatformSharpe: %.3f\n", v.PlatformSharpe))
	sb.WriteString(fmt.Sprintf("- PlatformNetPnL: $%.2f\n", v.PlatformNetPnL))
	return sb.String()
}

func buildExecutiveSummary(r *Phase26Result) string {
	var sb strings.Builder
	sb.WriteString("# Phase 26 — Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("- Total Eligible Strategies: %d\n", r.TotalEligible))
	sb.WriteString(fmt.Sprintf("- Total Certified (PROMOTE): %d\n", r.TotalCertified))
	sb.WriteString(fmt.Sprintf("- Tier 1 Strategies: %d\n", len(r.Tier1Strategies)))
	sb.WriteString(fmt.Sprintf("- Tier 2 Strategies: %d\n", len(r.Tier2Strategies)))
	sb.WriteString(fmt.Sprintf("- Retired: %d\n", len(r.Retired)))
	sb.WriteString(fmt.Sprintf("- Insufficient Evidence: %d\n", len(r.InsufficientEvidenceStrategies)))
	sb.WriteString(fmt.Sprintf("- Overall Decision: %s\n", r.FinalVerdict.OverallDecision))
	return sb.String()
}
