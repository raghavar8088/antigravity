package phase22f

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WriteAllReports generates all Phase 22F certification reports into outDir.
func WriteAllReports(result Phase22FResult, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	type report struct {
		name string
		fn   func(Phase22FResult) string
	}
	reports := []report{
		{"DATA_INTEGRITY_CERTIFICATION.md", dataIntegrityReport},
		{"TOP20_SELECTION_REPORT.md", top20SelectionReport},
		{"STRATEGY_VALIDATION_DATASET.md", strategyValidationDataset},
		{"STATISTICAL_VALIDATION_REPORT.md", statisticalValidationReport},
		{"CONFIDENCE_ANALYSIS_REPORT.md", confidenceAnalysisReport},
		{"MONTE_CARLO_CERTIFICATION.md", monteCarloReport},
		{"REGIME_CERTIFICATION.md", regimeCertificationReport},
		{"ALPHA_EDGE_REPORT.md", alphaEdgeReport},
		{"EXECUTION_CORRELATION_REPORT.md", execCorrelationReport},
		{"PORTFOLIO_OPTIMIZATION_REPORT.md", portfolioOptimizationReport},
		{"CAPITAL_DEPLOYMENT_CERTIFICATION.md", capitalDeploymentReport},
		{"STRATEGY_RETIREMENT_REPORT.md", strategyRetirementReport},
		{"INSTITUTIONAL_CERTIFICATION_REPORT.md", institutionalCertificationReport},
		{"EDGE_VERDICT.md", edgeVerdictReport},
		{"AUTOMATED_PIPELINE_REPORT.md", automatedPipelineReport},
		{"PRODUCTION_READINESS_REPORT.md", productionReadinessReport},
		{"PHASE_22F_IMPLEMENTATION_REPORT.md", implementationReport22F},
	}

	for _, r := range reports {
		content := r.fn(result)
		path := filepath.Join(outDir, r.name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", r.name, err)
		}
	}
	return nil
}

// ── Report helpers ─────────────────────────────────────────────────────────

func hdr(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("# %s\n\n", title))
	b.WriteString(fmt.Sprintf("*Generated: %s*\n\n---\n\n", time.Now().UTC().Format(time.RFC3339)))
}

func sec(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("## %s\n\n", title))
}

func subsec(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("### %s\n\n", title))
}

func pass(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func tierEmoji(t InstitutionalTier) string {
	switch t {
	case TierInstitutional:
		return "INSTITUTIONAL"
	case TierFull:
		return "FULL DEPLOYMENT"
	case TierLimited:
		return "LIMITED CAPITAL"
	case TierPilot:
		return "PILOT"
	case TierPaperOnly:
		return "PAPER ONLY"
	case TierWatchlist:
		return "WATCHLIST"
	default:
		return "FAILED"
	}
}

// ── Individual report functions ────────────────────────────────────────────

func dataIntegrityReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — DATA INTEGRITY CERTIFICATION")
	di := r.DataIntegrity

	sec(b, "1. Certification Summary")
	b.WriteString(fmt.Sprintf("| Metric | Value |\n|:---|:---|\n"))
	b.WriteString(fmt.Sprintf("| Certification Status | **%s** |\n", pass(di.Passed)))
	b.WriteString(fmt.Sprintf("| Certified Trades | %d |\n", di.CertifiedTrades))
	b.WriteString(fmt.Sprintf("| Certified Fills | %d |\n", di.CertifiedFills))
	b.WriteString(fmt.Sprintf("| Certified Strategies | %d |\n\n", di.CertifiedStrategies))

	sec(b, "2. Data Source Audit")
	b.WriteString("| Source | Available | Total | Valid | Dups | Corrupt | Survivorship | Look-Ahead |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, s := range di.Sources {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d | %d | %s | %s |\n",
			s.Source, pass(s.Available), s.TotalRecords, s.ValidRecords,
			s.Duplicates, s.Corrupted,
			pass(!s.SurvivshipBias), pass(!s.LookAheadBias)))
	}
	b.WriteString("\n")

	if len(di.Issues) > 0 {
		sec(b, "3. Issues & Warnings")
		for _, iss := range di.Issues {
			b.WriteString(fmt.Sprintf("- %s\n", iss))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func top20SelectionReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — TOP-20 STRATEGY SELECTION REPORT")
	sec(b, "Methodology")
	b.WriteString(r.Top20.Methodology + "\n\n")
	sec(b, "Rankings")
	b.WriteString("| Rank | Strategy | Family | Score | PF | Sharpe | Expectancy | MaxDD | Trades | Stability | Justification |\n")
	b.WriteString("|:---:|:---|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---|\n")
	for _, e := range r.Top20.Entries {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %.1f | %.2f | %.2f | $%.0f | %.1f%% | %d | %s | %s |\n",
			e.Rank, e.StrategyName, e.Family, e.Score,
			e.ProfitFactor, e.Sharpe, e.Expectancy, e.MaxDrawdown,
			e.TradeCount, e.Stability, e.Justification))
	}
	b.WriteString("\n")
	return b.String()
}

func strategyValidationDataset(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — 1000-TRADE VALIDATION CAMPAIGN")
	sec(b, "Campaign Summary")
	completed, active, invalidated, stalled := 0, 0, 0, 0
	for _, ce := range r.Campaign {
		switch ce.Status {
		case CampaignCompleted:
			completed++
		case CampaignActive:
			active++
		case CampaignInvalidated:
			invalidated++
		case CampaignStalled:
			stalled++
		}
	}
	b.WriteString(fmt.Sprintf("- Completed (1000+ trades): **%d**\n", completed))
	b.WriteString(fmt.Sprintf("- Active (in progress): **%d**\n", active))
	b.WriteString(fmt.Sprintf("- Invalidated: **%d**\n", invalidated))
	b.WriteString(fmt.Sprintf("- Stalled (insufficient data): **%d**\n\n", stalled))

	sec(b, "Per-Strategy Campaign Status")
	b.WriteString("| Strategy | Trades | Status | Reason | PF@1000 | WR@1000 | DD@1000 |\n")
	b.WriteString("|:---|:---:|:---:|:---|:---:|:---:|:---:|\n")
	for _, ce := range r.Campaign {
		pfFinal, wrFinal, ddFinal := 0.0, 0.0, 0.0
		if ce.FinalMetrics != nil {
			pfFinal = ce.FinalMetrics.Base.ProfitFactor
			wrFinal = ce.FinalMetrics.Base.WinRate
			ddFinal = ce.FinalMetrics.Base.MaxDrawdown
		}
		b.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %.2f | %.1f%% | %.1f%% |\n",
			ce.StrategyName, ce.TotalTrades, ce.Status, ce.Reason,
			pfFinal, wrFinal*100, ddFinal))
	}
	b.WriteString("\n")

	sec(b, "Milestone Checkpoints (Top Strategies)")
	for _, ce := range r.Campaign {
		if len(ce.Checkpoints) == 0 {
			continue
		}
		subsec(b, ce.StrategyName)
		b.WriteString("| At Trade | PF | WR | Sharpe | DD | Expectancy |\n")
		b.WriteString("|:---:|:---:|:---:|:---:|:---:|:---:|\n")
		for _, cp := range ce.Checkpoints {
			b.WriteString(fmt.Sprintf("| %d | %.2f | %.1f%% | %.2f | %.1f%% | $%.1f |\n",
				cp.AtTrade, cp.ProfitFactor, cp.WinRate*100, cp.Sharpe, cp.MaxDrawdown, cp.Expectancy))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func statisticalValidationReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — COMPLETE STATISTICAL ANALYSIS")
	sec(b, "Per-Strategy Extended Statistics")
	b.WriteString("| Strategy | Trades | WR | PF | Sharpe | Sortino | Calmar | UlcerIdx | MaxDD | Expectancy | RoR | MaxConsecW | MaxConsecL |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, es := range r.ExtendedStats {
		b.WriteString(fmt.Sprintf("| %s | %d | %.1f%% | %.2f | %.2f | %.2f | %.2f | %.2f | %.1f%% | $%.1f | %.1f%% | %d | %d |\n",
			es.Base.StrategyName, es.Base.TotalTrades,
			es.Base.WinRate*100, es.Base.ProfitFactor, es.Base.Sharpe,
			es.SortinoRatio, es.CalmarRatio, es.UlcerIndex, es.Base.MaxDrawdown,
			es.Base.Expectancy, es.RiskOfRuin*100,
			es.MaxConsecWins, es.MaxConsecLosses))
	}
	b.WriteString("\n")
	return b.String()
}

func confidenceAnalysisReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — CONFIDENCE INTERVAL ANALYSIS")
	sec(b, "Portfolio-Level Confidence Intervals")
	writeCI(b, r.Confidence.Portfolio)
	sec(b, "Per-Strategy Confidence Intervals")
	for _, sci := range r.Confidence.Strategies {
		subsec(b, fmt.Sprintf("%s (n=%d)", sci.StrategyName, sci.TradeCount))
		writeCI(b, sci)
	}
	return b.String()
}

func writeCI(b *strings.Builder, sci StrategyCI) {
	b.WriteString("| Metric | Point | 90% CI | 95% CI | 99% CI | Reliable |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|\n")
	for _, ci := range []ConfidenceInterval{sci.ProfitFactor, sci.Sharpe, sci.Expectancy, sci.WinRate} {
		b.WriteString(fmt.Sprintf("| %s | %.3f | [%.3f, %.3f] | [%.3f, %.3f] | [%.3f, %.3f] | %s |\n",
			ci.Metric, ci.Point,
			ci.CI90Low, ci.CI90High,
			ci.CI95Low, ci.CI95High,
			ci.CI99Low, ci.CI99High,
			pass(ci.Reliable)))
	}
	b.WriteString("\n")
}

func monteCarloReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — MONTE CARLO CERTIFICATION (1000 simulations per strategy)")
	sec(b, "Portfolio Monte Carlo")
	mc := r.PortfolioMC
	b.WriteString(fmt.Sprintf("- Simulations: %d\n", mc.Simulations))
	b.WriteString(fmt.Sprintf("- Expected Return (median): $%.0f\n", mc.ExpectedReturn))
	b.WriteString(fmt.Sprintf("- Worst Return (5th pct): $%.0f\n", mc.WorstReturn))
	b.WriteString(fmt.Sprintf("- Best Return (95th pct): $%.0f\n", mc.BestReturn))
	b.WriteString(fmt.Sprintf("- Expected Drawdown: %.1f%%\n", mc.ExpectedDD))
	b.WriteString(fmt.Sprintf("- Worst Drawdown (95th pct): %.1f%%\n", mc.WorstDD))
	b.WriteString(fmt.Sprintf("- P(Ruin): %.1f%%\n", mc.ProbabilityRuin*100))
	b.WriteString(fmt.Sprintf("- P(Growth): %.1f%%\n", mc.ProbabilityGrow*100))
	b.WriteString(fmt.Sprintf("- Capital Survival Rate: %.1f%%\n", mc.CapSurvivalRate*100))
	b.WriteString(fmt.Sprintf("- **Stability: %s**\n\n", mc.Stability))

	sec(b, "Per-Strategy Monte Carlo Summary")
	b.WriteString("| Strategy | Sims | Expected | Worst | P(Grow) | P(Ruin) | MaxDD | Stability |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")

	// sort by stability (best first)
	type row struct {
		name string
		mc   MonteCarloF22
	}
	rows := make([]row, 0, len(r.MonteCarlo))
	for id, mc := range r.MonteCarlo {
		name := id
		for _, es := range r.ExtendedStats {
			if es.Base.StrategyID == id {
				name = es.Base.StrategyName
				break
			}
		}
		rows = append(rows, row{name, mc})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].mc.ProbabilityGrow > rows[j].mc.ProbabilityGrow
	})
	for _, row := range rows {
		mc := row.mc
		b.WriteString(fmt.Sprintf("| %s | %d | $%.0f | $%.0f | %.0f%% | %.1f%% | %.1f%% | %s |\n",
			row.name, mc.Simulations, mc.ExpectedReturn, mc.WorstReturn,
			mc.ProbabilityGrow*100, mc.ProbabilityRuin*100, mc.WorstDD, mc.Stability))
	}
	b.WriteString("\n")
	return b.String()
}

func regimeCertificationReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — REGIME PERFORMANCE CERTIFICATION")
	sec(b, "All 10 Regime Performance")
	b.WriteString("| Regime | Trades | WinRate | PF | Sharpe | Expectancy | MaxDD | NetPnL |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")

	// sort regimes alphabetically
	regimes := make([]RegimeF22, 0, len(r.RegimePerf))
	for reg := range r.RegimePerf {
		regimes = append(regimes, reg)
	}
	sort.Slice(regimes, func(i, j int) bool { return string(regimes[i]) < string(regimes[j]) })

	for _, reg := range regimes {
		rp := r.RegimePerf[reg]
		b.WriteString(fmt.Sprintf("| %s | %d | %.1f%% | %.2f | %.2f | $%.0f | %.1f%% | $%.0f |\n",
			rp.Regime, rp.Trades, rp.WinRate*100, rp.ProfitFactor, rp.Sharpe,
			rp.Expectancy, rp.MaxDrawdown, rp.NetPnLUSD))
	}
	b.WriteString("\n")
	return b.String()
}

func alphaEdgeReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — ALPHA ENGINE VALIDATION REPORT")
	sec(b, "Alpha Engine Rankings")
	b.WriteString("| Rank | Alpha Engine | Trades | WR | PF | Sharpe | Expectancy | MaxDD | ExecQ | MC | Tier |\n")
	b.WriteString("|:---:|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, av := range r.AlphaValidation {
		b.WriteString(fmt.Sprintf("| %d | %s | %d | %.1f%% | %.2f | %.2f | $%.0f | %.1f%% | %.0f | %s | %s |\n",
			av.Rank, av.AlphaEngine, av.Trades, av.WinRate*100,
			av.ProfitFactor, av.Sharpe, av.Expectancy, av.MaxDrawdown,
			av.ExecQuality, av.MonteCarlo.Stability, tierEmoji(av.Tier)))
	}
	b.WriteString("\n")
	sec(b, "Alpha Engine Recommendations")
	for _, av := range r.AlphaValidation {
		b.WriteString(fmt.Sprintf("- **%s**: %s\n", av.AlphaEngine, av.Recommendation))
	}
	b.WriteString("\n")
	return b.String()
}

func execCorrelationReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — EXECUTION QUALITY CORRELATION REPORT")
	ec := r.ExecCorrelation
	sec(b, "Summary")
	b.WriteString(ec.Summary + "\n\n")
	sec(b, "Top Impact Correlations")
	b.WriteString("| Exec Metric | Perf Metric | Correlation | Impact | Significance |\n")
	b.WriteString("|:---|:---|:---:|:---:|:---:|\n")
	for _, e := range ec.TopImpact {
		b.WriteString(fmt.Sprintf("| %s | %s | %.3f | %s | %s |\n",
			e.ExecMetric, e.PerfMetric, e.Correlation, e.Impact, e.Significance))
	}
	b.WriteString("\n")
	sec(b, "Full Correlation Matrix")
	b.WriteString("| Exec Metric | Perf Metric | Correlation | Impact | Significance |\n")
	b.WriteString("|:---|:---|:---:|:---:|:---:|\n")
	for _, e := range ec.Entries {
		b.WriteString(fmt.Sprintf("| %s | %s | %.3f | %s | %s |\n",
			e.ExecMetric, e.PerfMetric, e.Correlation, e.Impact, e.Significance))
	}
	b.WriteString("\n")
	return b.String()
}

func portfolioOptimizationReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — PORTFOLIO OPTIMIZATION REPORT")
	sec(b, "Portfolio Variants")
	b.WriteString("| Portfolio | Strategies | PF | Sharpe | Expectancy | MaxDD | Diversity | Capital% | MC Stability |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, pv := range r.Portfolios {
		b.WriteString(fmt.Sprintf("| %s | %d | %.2f | %.2f | $%.0f | %.1f%% | %.0f/100 | %.0f%% | %s |\n",
			pv.Name, len(pv.Strategies), pv.ProfitFactor, pv.Sharpe,
			pv.Expectancy, pv.MaxDrawdown, pv.DiversScore,
			pv.TotalCapitalPct, pv.MonteCarlo.Stability))
	}
	b.WriteString("\n")
	for _, pv := range r.Portfolios {
		subsec(b, pv.Name+" — Strategy List")
		for i, id := range pv.Strategies {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, id))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func capitalDeploymentReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — CAPITAL DEPLOYMENT CERTIFICATION")
	sec(b, "Allocation Table")
	b.WriteString("| Strategy | Score | Band | Allocation% | Allocation USD | Rationale |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---|\n")
	total := 0.0
	for _, ca := range r.CapitalAllocation {
		b.WriteString(fmt.Sprintf("| %s | %.1f | %s | %.0f%% | $%.0f | %s |\n",
			ca.StrategyName, ca.WeightedScore, ca.Band,
			ca.AllocationPct, ca.AllocationUSD,
			strings.Join(ca.Rationale[:min2(2, len(ca.Rationale))], "; ")))
		total += ca.AllocationUSD
	}
	b.WriteString(fmt.Sprintf("\n**Total Deployed Capital: $%.0f**\n\n", total))
	return b.String()
}

func strategyRetirementReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — STRATEGY RETIREMENT REPORT")
	if len(r.Elimination) == 0 {
		b.WriteString("*No strategies flagged for retirement.*\n\n")
		return b.String()
	}
	immediate := filterBySeverity(r.Elimination, EliminateImmediate)
	recommended := filterBySeverity(r.Elimination, EliminateRecommended)
	conditional := filterBySeverity(r.Elimination, EliminateConditional)

	sec(b, fmt.Sprintf("IMMEDIATE RETIREMENT (%d strategies)", len(immediate)))
	for _, ec := range immediate {
		b.WriteString(fmt.Sprintf("### %s\n", ec.StrategyName))
		b.WriteString(fmt.Sprintf("- PF=%.3f | Sharpe=%.2f | Expectancy=$%.2f | MaxDD=%.1f%%\n", ec.ProfitFactor, ec.Sharpe, ec.Expectancy, ec.MaxDrawdown))
		for _, r := range ec.Reasons {
			b.WriteString(fmt.Sprintf("- **REASON**: %s\n", r))
		}
		b.WriteString("\n")
	}
	sec(b, fmt.Sprintf("RECOMMENDED RETIREMENT (%d strategies)", len(recommended)))
	for _, ec := range recommended {
		b.WriteString(fmt.Sprintf("### %s\n- PF=%.3f | %s\n\n", ec.StrategyName, ec.ProfitFactor, strings.Join(ec.Reasons, " | ")))
	}
	sec(b, fmt.Sprintf("CONDITIONAL MONITORING (%d strategies)", len(conditional)))
	for _, ec := range conditional {
		b.WriteString(fmt.Sprintf("- %s: %s\n", ec.StrategyName, strings.Join(ec.Reasons, " | ")))
	}
	b.WriteString("\n")
	return b.String()
}

func institutionalCertificationReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — INSTITUTIONAL CERTIFICATION TIERS")
	counts := TierCounts22(r.CertificationTiers)
	sec(b, "Tier Distribution")
	tiers := []InstitutionalTier{TierInstitutional, TierFull, TierLimited, TierPilot, TierPaperOnly, TierWatchlist, TierFailed}
	b.WriteString("| Tier | Count | Max Capital% |\n|:---|:---:|:---:|\n")
	for _, t := range tiers {
		b.WriteString(fmt.Sprintf("| %s | %d | %.0f%% |\n", tierEmoji(t), counts[t], tierMaxCapital(t)))
	}
	b.WriteString("\n")
	sec(b, "Per-Strategy Classification")
	b.WriteString("| Strategy | Family | Tier | Max Capital% | Evidence |\n")
	b.WriteString("|:---|:---|:---:|:---:|:---|\n")
	for _, tc := range r.CertificationTiers {
		e := ""
		if len(tc.Evidence) > 0 {
			e = tc.Evidence[len(tc.Evidence)-1]
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f%% | %s |\n",
			tc.StrategyName, tc.Family, tierEmoji(tc.Tier), tc.MaxCapitalPct, e))
	}
	b.WriteString("\n")
	return b.String()
}

func edgeVerdictReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — EDGE VERDICT")
	v := r.EdgeVerdict
	sec(b, "Final Determination")
	edgeStr := "NO EDGE CONFIRMED"
	if v.SystemHasEdge {
		edgeStr = "EDGE CONFIRMED"
	}
	b.WriteString(fmt.Sprintf("## %s (%s CONFIDENCE)\n\n", edgeStr, v.Confidence))
	b.WriteString(v.Narrative + "\n\n")

	sec(b, "Key Questions Answered")
	b.WriteString(fmt.Sprintf("1. **Does the system have edge?** %s\n", boolStr(v.SystemHasEdge, "YES", "NO")))
	b.WriteString(fmt.Sprintf("2. **Strongest strategy:** %s\n", v.StrongestStrategy))
	b.WriteString(fmt.Sprintf("3. **Strongest alpha engine:** %s\n", v.StrongestAlpha))
	b.WriteString(fmt.Sprintf("4. **Expected portfolio PF:** %.3f\n", v.ExpectedPortfolioPF))
	b.WriteString(fmt.Sprintf("5. **Expected portfolio Sharpe:** %.2f\n", v.ExpectedSharpe))
	b.WriteString(fmt.Sprintf("6. **Expected drawdown:** %.1f%%\n", v.ExpectedDrawdown))
	b.WriteString(fmt.Sprintf("7. **Strategies passed:** %d\n", v.StrategiesPassed))
	b.WriteString(fmt.Sprintf("8. **Strategies failed:** %d\n", v.StrategiesFailed))
	b.WriteString(fmt.Sprintf("9. **%% deserve capital:** %.1f%%\n", v.PctDeserveCapital))
	b.WriteString(fmt.Sprintf("10. **%% should retire:** %.1f%%\n\n", v.PctShouldRetire))

	sec(b, "Supporting Evidence")
	for i, ev := range v.SupportingEvidence {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, ev))
	}
	b.WriteString("\n")
	return b.String()
}

func automatedPipelineReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — AUTOMATED VALIDATION PIPELINE")
	sec(b, "Pipeline Requirements")
	b.WriteString("Every new strategy **must** pass the following gates before capital deployment:\n\n")
	b.WriteString("1. Minimum 30 trades for certification entry\n")
	b.WriteString("2. Minimum 500 trades for limited capital approval\n")
	b.WriteString("3. Minimum 1000 trades for institutional grade\n")
	b.WriteString("4. Profit Factor ≥ 1.20 at every 200-trade checkpoint\n")
	b.WriteString("5. Monte Carlo simulation: not FAILED\n")
	b.WriteString("6. Regime analysis: performs in ≥3 market regimes\n")
	b.WriteString("7. Execution quality: slippage within acceptable bands\n\n")
	sec(b, "Current Pipeline Status")
	b.WriteString(fmt.Sprintf("- Total strategies evaluated: %d\n", r.TotalStrategies))
	b.WriteString(fmt.Sprintf("- Passed pipeline: %d\n", r.PassedStrategies))
	b.WriteString(fmt.Sprintf("- Failed pipeline: %d\n\n", r.FailedStrategies))
	return b.String()
}

func productionReadinessReport(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F — PRODUCTION READINESS ASSESSMENT")
	v := r.EdgeVerdict
	mc := r.PortfolioMC

	sec(b, "Go/No-Go Decision Matrix")
	b.WriteString("| Criterion | Required | Actual | Status |\n|:---|:---:|:---:|:---:|\n")
	checks := []struct {
		name string
		req  string
		act  string
		ok   bool
	}{
		{"System Edge", "CONFIRMED", boolStr(v.SystemHasEdge, "CONFIRMED", "ABSENT"), v.SystemHasEdge},
		{"Portfolio PF", "≥1.20", fmt.Sprintf("%.2f", v.ExpectedPortfolioPF), v.ExpectedPortfolioPF >= 1.20},
		{"Portfolio Sharpe", "≥1.00", fmt.Sprintf("%.2f", v.ExpectedSharpe), v.ExpectedSharpe >= 1.00},
		{"Drawdown", "≤15%", fmt.Sprintf("%.1f%%", v.ExpectedDrawdown), v.ExpectedDrawdown <= 15},
		{"MC Stability", "≥STABLE", string(mc.Stability), mc.Stability == MCRobust || mc.Stability == MCStable22},
		{"P(Ruin)", "<5%", fmt.Sprintf("%.1f%%", mc.ProbabilityRuin*100), mc.ProbabilityRuin < 0.05},
		{"Strategies Passed", "≥3", fmt.Sprintf("%d", v.StrategiesPassed), v.StrategiesPassed >= 3},
		{"Data Integrity", "PASS", pass(r.DataIntegrity.Passed), r.DataIntegrity.Passed},
	}
	allPass := true
	for _, c := range checks {
		if !c.ok {
			allPass = false
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", c.name, c.req, c.act, pass(c.ok)))
	}
	b.WriteString(fmt.Sprintf("\n**Overall: %s**\n\n", pass(allPass)))

	sec(b, "Recommendation")
	if allPass {
		b.WriteString("**PROCEED TO PHASED LIVE DEPLOYMENT.**\n\n")
		b.WriteString("Begin with PILOT capital, escalate on 90-day performance reviews.\n")
	} else {
		b.WriteString("**DO NOT DEPLOY LIVE CAPITAL.**\n\n")
		b.WriteString("Address failing criteria, collect additional paper trade evidence, then re-run certification.\n")
	}
	return b.String()
}

func implementationReport22F(r Phase22FResult) string {
	b := &strings.Builder{}
	hdr(b, "PHASE 22F IMPLEMENTATION REPORT — Institutional 1000-Trade Validation")

	sections := []struct {
		title string
		fn    func(Phase22FResult) string
	}{
		{"1. Data Integrity Certification", dataIntegrityReport},
		{"2. Top-20 Strategy Selection", top20SelectionReport},
		{"3. 1000-Trade Validation Results", strategyValidationDataset},
		{"4. Statistical Analysis", statisticalValidationReport},
		{"5. Confidence Interval Analysis", confidenceAnalysisReport},
		{"6. Monte Carlo Results", monteCarloReport},
		{"7. Regime Analysis", regimeCertificationReport},
		{"8. Alpha Rankings", alphaEdgeReport},
		{"9. Execution Correlation Findings", execCorrelationReport},
		{"10. Portfolio Construction", portfolioOptimizationReport},
		{"11. Capital Deployment Recommendations", capitalDeploymentReport},
		{"12. Retirement Candidates", strategyRetirementReport},
		{"13. Certification Tiers", institutionalCertificationReport},
		{"14. Edge Verdict", edgeVerdictReport},
		{"15. Production Readiness", productionReadinessReport},
	}

	for _, s := range sections {
		b.WriteString(fmt.Sprintf("---\n\n# %s\n\n", s.title))
		// embed the relevant section text without the header
		body := s.fn(r)
		// strip leading title/generated-at lines
		lines := strings.SplitN(body, "\n", 5)
		if len(lines) > 4 {
			b.WriteString(strings.Join(lines[4:], "\n"))
		}
	}
	return b.String()
}

// ── utility helpers ─────────────────────────────────────────────────────────

func filterBySeverity(candidates []EliminationCandidate, sev EliminationSeverity) []EliminationCandidate {
	var out []EliminationCandidate
	for _, c := range candidates {
		if c.Severity == sev {
			out = append(out, c)
		}
	}
	return out
}

func boolStr(ok bool, t, f string) string {
	if ok {
		return t
	}
	return f
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
