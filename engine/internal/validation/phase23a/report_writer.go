package phase23a

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/validation/phase22f"
)

// WriteAllReports generates all Phase 23A institutional reports into outDir.
func WriteAllReports(result Phase23AResult, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	type report struct {
		name string
		fn   func(Phase23AResult) string
	}
	reports := []report{
		{"VALIDATION_READINESS_REPORT.md", readinessReport},
		{"DATA_INTEGRITY_REPORT.md", dataIntegrityReport23},
		{"TOP20_SELECTION_REPORT.md", top20Report23},
		{"WALK_FORWARD_REPORT.md", walkForwardReport},
		{"MONTE_CARLO_REPORT.md", monteCarloReport23},
		{"REGIME_ANALYSIS_REPORT.md", regimeAnalysisReport},
		{"EXECUTION_IMPACT_REPORT.md", executionImpactReport},
		{"ALPHA_ENGINE_RANKINGS.md", alphaEngineRankings},
		{"PORTFOLIO_CONSTRUCTION_REPORT.md", portfolioConstructionReport},
		{"ELIMINATION_REPORT.md", eliminationReport23},
		{"EDGE_CERTIFICATION_REPORT.md", edgeCertificationReport},
		{"FINAL_RANKING_REPORT.md", finalRankingReport},
		{"CAPITAL_DEPLOYMENT_PLAN.md", capitalDeploymentPlanReport},
		{"PHASE23A_FINAL_CERTIFICATION.md", finalCertificationReport},
	}

	for _, r := range reports {
		if err := os.WriteFile(filepath.Join(outDir, r.name), []byte(r.fn(result)), 0644); err != nil {
			return fmt.Errorf("write %s: %w", r.name, err)
		}
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func h1(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("# %s\n\n*Generated: %s*\n\n---\n\n", title, time.Now().UTC().Format(time.RFC3339)))
}

func h2(b *strings.Builder, title string) { b.WriteString(fmt.Sprintf("## %s\n\n", title)) }
func h3(b *strings.Builder, title string) { b.WriteString(fmt.Sprintf("### %s\n\n", title)) }

func ok23(v bool) string {
	if v {
		return "PASS"
	}
	return "FAIL"
}

// ── Individual reports ─────────────────────────────────────────────────────────

func readinessReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — VALIDATION READINESS REPORT")
	ra := r.Readiness

	h2(b, "1. Overall Status")
	b.WriteString(fmt.Sprintf("**Status: %s**\n\n", ok23(ra.Passed)))
	if len(ra.Blockers) > 0 {
		h3(b, "Blockers")
		for _, bl := range ra.Blockers {
			b.WriteString(fmt.Sprintf("- %s\n", bl))
		}
		b.WriteString("\n")
	}
	if len(ra.Warnings) > 0 {
		h3(b, "Warnings")
		for _, w := range ra.Warnings {
			b.WriteString(fmt.Sprintf("- %s\n", w))
		}
		b.WriteString("\n")
	}

	h2(b, "2. Component Audit")
	b.WriteString("| Component | Status | Detail |\n|:---|:---:|:---|\n")
	for _, c := range ra.Components {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", c.Name, c.Status, c.Detail))
	}
	b.WriteString("\n")
	return b.String()
}

func dataIntegrityReport23(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — DATA INTEGRITY REPORT")
	ds := r.Dataset

	h2(b, "1. Dataset Summary")
	b.WriteString(fmt.Sprintf("| Metric | Value |\n|:---|:---|\n"))
	b.WriteString(fmt.Sprintf("| Symbol | %s |\n", ds.Symbol))
	b.WriteString(fmt.Sprintf("| Date Range | %s → %s |\n", ds.From.Format("2006-01-02"), ds.To.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("| Candle Count | %d |\n", ds.CandleCount))
	b.WriteString(fmt.Sprintf("| Expected Candles | %d |\n", ds.ExpectedCandles))
	b.WriteString(fmt.Sprintf("| Missing %% | %.2f%% |\n", ds.MissingPct))
	b.WriteString(fmt.Sprintf("| Outlier Count | %d |\n", ds.OutlierCount))
	b.WriteString(fmt.Sprintf("| **Quality Score** | **%.1f/100** |\n", ds.QualityScore))
	b.WriteString(fmt.Sprintf("| Funding Data | %s |\n", ok23(ds.HasFunding)))
	b.WriteString(fmt.Sprintf("| Open Interest | %s |\n", ok23(ds.HasOI)))
	b.WriteString(fmt.Sprintf("| Liquidation Data | %s |\n\n", ok23(ds.HasLiquidations)))

	if len(ds.Issues) > 0 {
		h2(b, "2. Issues")
		for _, iss := range ds.Issues {
			b.WriteString(fmt.Sprintf("- %s\n", iss))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func top20Report23(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — TOP-20 STRATEGY SELECTION")
	b.WriteString("*Source: Phase 22F composite ranking engine applied to walk-forward validation trades.*\n\n")
	b.WriteString("| Rank | Strategy | Family | Score | PF | Sharpe | Expectancy | Trades | Stability |\n")
	b.WriteString("|:---:|:---|:---|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, e := range r.Top20.Entries {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %.1f | %.2f | %.2f | $%.0f | %d | %s |\n",
			e.Rank, e.StrategyName, e.Family, e.Score,
			e.ProfitFactor, e.Sharpe, e.Expectancy, e.TradeCount, e.Stability))
	}
	b.WriteString("\n")
	return b.String()
}

func walkForwardReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — WALK-FORWARD VALIDATION REPORT")

	h2(b, "1. Methodology")
	b.WriteString(fmt.Sprintf("- Train window: %d months\n", DefaultTrainMonths))
	b.WriteString(fmt.Sprintf("- Validate window: %d months\n", DefaultValidMonths))
	b.WriteString(fmt.Sprintf("- Minimum windows: %d\n", MinWFWindows))
	b.WriteString("- Step: roll forward by one validation period (no overlapping validation)\n\n")

	h2(b, "2. Per-Strategy Walk-Forward Summary")
	b.WriteString("| Strategy | Windows | Avg Valid PF | Avg Sharpe | Consistency | Degradation | Consistent? | Degraded? |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, rpt := range r.WalkForward {
		b.WriteString(fmt.Sprintf("| %s | %d | %.2f | %.2f | %.0f%% | %.3f | %s | %s |\n",
			rpt.StrategyName, len(rpt.Windows),
			rpt.AvgValidPF, rpt.AvgValidSharpe,
			rpt.Consistency, rpt.Degradation,
			ok23(rpt.IsConsistent), boolLabel(!rpt.IsDegraded, "NO", "YES")))
	}
	b.WriteString("\n")

	h2(b, "3. Window-Level Detail (Top Strategies)")
	shown := 0
	for _, rpt := range r.WalkForward {
		if shown >= 5 {
			break
		}
		if rpt.AvgValidPF < 1.10 {
			continue
		}
		h3(b, rpt.StrategyName)
		b.WriteString("| Window | Train PF | Valid PF | Train Sharpe | Valid Sharpe | Train n | Valid n |\n")
		b.WriteString("|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
		for _, w := range rpt.Windows {
			tpf, ts, _, _ := sampleMetrics23(w.TrainResult.Trades)
			vpf, vs, _, _ := sampleMetrics23(w.ValidResult.Trades)
			b.WriteString(fmt.Sprintf("| %d | %.2f | %.2f | %.2f | %.2f | %d | %d |\n",
				w.WindowNum, tpf, vpf, ts, vs,
				len(w.TrainResult.Trades), len(w.ValidResult.Trades)))
		}
		b.WriteString("\n")
		shown++
	}
	return b.String()
}

func monteCarloReport23(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — MONTE CARLO STABILITY REPORT (1000 simulations per strategy)")
	mc := r.PortfolioMC

	h2(b, "1. Portfolio Monte Carlo")
	b.WriteString(fmt.Sprintf("- Simulations: %d\n", mc.Simulations))
	b.WriteString(fmt.Sprintf("- Expected Return: $%.0f\n", mc.ExpectedReturn))
	b.WriteString(fmt.Sprintf("- Worst Return (5th pct): $%.0f\n", mc.WorstReturn))
	b.WriteString(fmt.Sprintf("- Best Return (95th pct): $%.0f\n", mc.BestReturn))
	b.WriteString(fmt.Sprintf("- Expected Drawdown: %.1f%%\n", mc.ExpectedDD))
	b.WriteString(fmt.Sprintf("- Worst Drawdown (95th pct): %.1f%%\n", mc.WorstDD))
	b.WriteString(fmt.Sprintf("- P(Ruin): %.1f%%\n", mc.ProbabilityRuin*100))
	b.WriteString(fmt.Sprintf("- P(Growth): %.1f%%\n", mc.ProbabilityGrow*100))
	b.WriteString(fmt.Sprintf("- **Stability: %s**\n\n", mc.Stability))

	h2(b, "2. Per-Strategy Monte Carlo")
	b.WriteString("| Strategy | P(Grow) | P(Ruin) | Expected | Worst | Max DD | Stability |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|\n")

	type row struct {
		name string
		mc   phase22f.MonteCarloF22
	}
	rows := make([]row, 0, len(r.MonteCarlo))
	for id, mc := range r.MonteCarlo {
		name := id
		for _, rpt := range r.WalkForward {
			if rpt.StrategyID == id {
				name = rpt.StrategyName
				break
			}
		}
		rows = append(rows, row{name, mc})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].mc.ProbabilityGrow > rows[j].mc.ProbabilityGrow })
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("| %s | %.0f%% | %.1f%% | $%.0f | $%.0f | %.1f%% | %s |\n",
			row.name, row.mc.ProbabilityGrow*100, row.mc.ProbabilityRuin*100,
			row.mc.ExpectedReturn, row.mc.WorstReturn, row.mc.WorstDD, row.mc.Stability))
	}
	b.WriteString("\n")
	return b.String()
}

func regimeAnalysisReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — REGIME PERFORMANCE ANALYSIS")
	h2(b, "All 10 Regime Performance")
	b.WriteString("| Regime | Trades | WR | PF | Sharpe | Expectancy | Max DD | Net PnL |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	regimes := make([]phase22f.RegimeF22, 0, len(r.RegimePerf))
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

	h2(b, "Regime Classification")
	b.WriteString("Strategies are classified as regime specialists if they show PF ≥ 1.30 in one regime but < 1.00 in others.\n\n")
	return b.String()
}

func executionImpactReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — EXECUTION IMPACT REPORT")
	h2(b, "1. Strategy-Level Execution Cost Analysis")
	b.WriteString("| Strategy | Gross PF | Net PF | Exec Cost (bps) | Slippage ($) | Fees ($) | Missed | Edge Retention |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, ei := range r.ExecutionImpact {
		b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.1f | $%.0f | $%.0f | %d | %.0f%% |\n",
			ei.StrategyName, ei.GrossEdgePF, ei.NetEdgePF,
			ei.ExecutionCostBps, ei.SlippageCostUSD, ei.FeeCostUSD,
			ei.MissedEntries, ei.EdgeRetention*100))
	}
	b.WriteString("\n")

	h2(b, "2. Fee Model")
	b.WriteString(fmt.Sprintf("- Taker fee: %.1f bps per leg (%.1f bps round-trip)\n", DefaultTakerFeeBps, DefaultTakerFeeBps*2))
	b.WriteString(fmt.Sprintf("- Slippage: %.1f bps per leg\n", DefaultSlippageBps))
	b.WriteString(fmt.Sprintf("- Total round-trip cost: ~%.1f bps\n\n", (DefaultTakerFeeBps+DefaultSlippageBps)*2))
	return b.String()
}

func alphaEngineRankings(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — ALPHA ENGINE SHOOTOUT")
	b.WriteString("| Rank | Alpha Engine | Trades | WR | PF | Sharpe | Expectancy | MC | Tier |\n")
	b.WriteString("|:---:|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, av := range r.AlphaRankings {
		b.WriteString(fmt.Sprintf("| %d | %s | %d | %.1f%% | %.2f | %.2f | $%.0f | %s | %s |\n",
			av.Rank, av.AlphaEngine, av.Trades, av.WinRate*100,
			av.ProfitFactor, av.Sharpe, av.Expectancy,
			av.MonteCarlo.Stability, av.Tier))
	}
	b.WriteString("\n")
	h2(b, "Recommendations")
	for _, av := range r.AlphaRankings {
		b.WriteString(fmt.Sprintf("- **%s** (Rank %d): %s\n", av.AlphaEngine, av.Rank, av.Recommendation))
	}
	b.WriteString("\n")
	return b.String()
}

func portfolioConstructionReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — PORTFOLIO CONSTRUCTION REPORT")
	h2(b, "Portfolio Variants")
	b.WriteString("| Portfolio | Strategies | PF | Sharpe | Exp | Max DD | Diversity | MC Stability |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, pv := range r.Portfolios {
		b.WriteString(fmt.Sprintf("| %s | %d | %.2f | %.2f | $%.0f | %.1f%% | %.0f/100 | %s |\n",
			pv.Name, len(pv.Strategies), pv.ProfitFactor, pv.Sharpe,
			pv.Expectancy, pv.MaxDrawdown, pv.DiversScore, pv.MonteCarlo.Stability))
	}
	b.WriteString("\n")
	for _, pv := range r.Portfolios {
		h3(b, pv.Name)
		for i, id := range pv.Strategies {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, id))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func eliminationReport23(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — ELIMINATION REPORT")
	if len(r.Eliminated) == 0 {
		b.WriteString("*No strategies eliminated.*\n\n")
		return b.String()
	}
	b.WriteString(fmt.Sprintf("**%d strategies eliminated** (immediate + recommended).\n\n", len(r.Eliminated)))
	for _, ec := range r.Eliminated {
		h3(b, fmt.Sprintf("%s [%s]", ec.StrategyName, ec.Severity))
		b.WriteString(fmt.Sprintf("- PF=%.3f | Sharpe=%.2f | Exp=$%.2f | DD=%.1f%% | Trades=%d\n", ec.ProfitFactor, ec.Sharpe, ec.Expectancy, ec.MaxDrawdown, ec.TotalTrades))
		for _, reason := range ec.Reasons {
			b.WriteString(fmt.Sprintf("- **REASON**: %s\n", reason))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func edgeCertificationReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — EDGE CERTIFICATION (14-Question Framework)")
	b.WriteString("| Strategy | Pass | Fail | Certified | Partial | Verdict |\n")
	b.WriteString("|:---|:---:|:---:|:---:|:---:|:---|\n")
	for _, c := range r.EdgeCertifications {
		cert := "NO"
		if c.Certified {
			cert = "YES (14/14)"
		} else if c.PartialCredit {
			cert = fmt.Sprintf("PARTIAL (%d/14)", c.PassCount)
		}
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %s | %s | %s |\n",
			c.StrategyName, c.PassCount, c.FailCount,
			ok23(c.Certified), ok23(c.PartialCredit), cert))
	}
	b.WriteString("\n")

	h2(b, "Detailed Question Matrix (Top 5)")
	shown := 0
	for _, c := range r.EdgeCertifications {
		if shown >= 5 {
			break
		}
		if !c.Certified && !c.PartialCredit {
			continue
		}
		h3(b, c.StrategyName)
		b.WriteString(c.Narrative + "\n\n")
		b.WriteString("| # | Question | Answer | Evidence |\n|:---:|:---|:---:|:---|\n")
		for i, a := range c.Answers {
			b.WriteString(fmt.Sprintf("| %d | %s | %s | %s |\n",
				i+1, a.Question, ok23(a.Answer), a.Evidence))
		}
		b.WriteString("\n")
		shown++
	}
	return b.String()
}

func finalRankingReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — FINAL STRATEGY RANKING (TOP 10)")
	h2(b, "Rankings Table")
	b.WriteString("| Rank | Strategy | Trades | WR | PF | Sharpe | Sortino | CAGR | Exp | MaxDD | RoR | MC | Regime | WF% | Capital |\n")
	b.WriteString("|:---:|:---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---|\n")
	limit := 10
	if len(r.FinalRanking) < limit {
		limit = len(r.FinalRanking)
	}
	for _, rs := range r.FinalRanking[:limit] {
		b.WriteString(fmt.Sprintf("| %d | %s | %d | %.0f%% | %.2f | %.2f | %.2f | %.1f%% | $%.0f | %.1f%% | %.1f%% | %s | %s | %.0f%% | %s |\n",
			rs.Rank, rs.StrategyName, rs.TradeCount, rs.WinRate*100,
			rs.ProfitFactor, rs.Sharpe, rs.Sortino, rs.CAGR,
			rs.Expectancy, rs.MaxDD, rs.RiskOfRuin*100,
			rs.MCTier, rs.RegimeStrength, rs.WFConsistency, rs.CapitalAllocation))
	}
	b.WriteString("\n")
	return b.String()
}

func capitalDeploymentPlanReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A — CAPITAL DEPLOYMENT PLAN")
	plan := r.DeploymentPlan
	b.WriteString(fmt.Sprintf("**Total Capital: $%.0f**  \n", plan.TotalCapital))
	b.WriteString(fmt.Sprintf("**Deployed: $%.0f (%.1f%%)**  \n", plan.DeployedTotal, plan.DeployedPct))
	b.WriteString(fmt.Sprintf("**Ready to Deploy: %s**\n\n", ok23(plan.ReadyToDeploy)))

	h2(b, "Minimum Requirements")
	b.WriteString(fmt.Sprintf("- Trades ≥ %d\n", CapMinTrades))
	b.WriteString(fmt.Sprintf("- Profit Factor ≥ %.2f\n", CapMinPF))
	b.WriteString(fmt.Sprintf("- Sharpe ≥ %.2f\n", CapMinSharpe))
	b.WriteString("- Positive Expectancy\n")
	b.WriteString(fmt.Sprintf("- Risk of Ruin ≤ %.0f%%\n", CapMaxRoR*100))
	b.WriteString(fmt.Sprintf("- Max Drawdown ≤ %.0f%%\n", CapMaxDD))
	b.WriteString("- Monte Carlo: STABLE or ROBUST\n\n")

	h2(b, "Allocation Table")
	b.WriteString("| Rank | Strategy | Band | Allocation% | Allocation USD | Gating |\n")
	b.WriteString("|:---:|:---|:---:|:---:|:---:|:---|\n")
	for _, de := range plan.Entries {
		status := "APPROVED"
		if len(de.GatingFailed) > 0 {
			status = fmt.Sprintf("BLOCKED (%d gates)", len(de.GatingFailed))
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %.0f%% | $%.0f | %s |\n",
			de.Rank, de.StrategyName, de.Band,
			de.AllocationPct, de.AllocationUSD, status))
	}
	b.WriteString("\n")

	h2(b, "Blocked Strategies (with reasons)")
	for _, de := range plan.Entries {
		if len(de.GatingFailed) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("**%s**: fails %d gate(s)\n", de.StrategyName, len(de.GatingFailed)))
		for _, f := range de.GatingFailed {
			b.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}
	b.WriteString("\n")
	return b.String()
}

func finalCertificationReport(r Phase23AResult) string {
	b := &strings.Builder{}
	h1(b, "PHASE 23A FINAL CERTIFICATION — Institutional Edge Validation")
	v := r.FinalVerdict

	deployStr := "NO — DO NOT DEPLOY"
	if v.Q10_DeployToday {
		deployStr = "YES — APPROVED FOR PHASED DEPLOYMENT"
	}

	b.WriteString("## EXECUTIVE VERDICT\n\n")
	b.WriteString(fmt.Sprintf("**Deploy Live Capital Today: %s**\n\n", deployStr))
	b.WriteString(v.Narrative + "\n\n---\n\n")

	h2(b, "Q1. Which strategies have real statistical edge?")
	if len(v.Q1_EdgeStrategies) == 0 {
		b.WriteString("*None confirmed.*\n\n")
	} else {
		for i, s := range v.Q1_EdgeStrategies {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	h2(b, "Q2. Which alpha engines actually work?")
	for i, s := range v.Q2_WorkingAlphas {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}
	b.WriteString("\n")

	h2(b, "Q3. Which strategies should be retired?")
	if len(v.Q3_Retire) == 0 {
		b.WriteString("*None — all strategies meet minimum thresholds.*\n\n")
	} else {
		for i, s := range v.Q3_Retire {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	h2(b, "Q4. Which strategies deserve paper capital?")
	for i, s := range v.Q4_PaperCapital {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
	}
	b.WriteString("\n")

	h2(b, "Q5. Which strategies deserve live capital?")
	if len(v.Q5_LiveCapital) == 0 {
		b.WriteString("*None — insufficient evidence for live capital deployment.*\n\n")
	} else {
		for i, s := range v.Q5_LiveCapital {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
		}
		b.WriteString("\n")
	}

	h2(b, "Q6. Expected portfolio profile")
	pp := v.Q6_PortfolioProfile
	b.WriteString(fmt.Sprintf("| Metric | Value |\n|:---|:---|\n"))
	b.WriteString(fmt.Sprintf("| Expected CAGR | %.1f%% |\n", pp.ExpectedCAGR))
	b.WriteString(fmt.Sprintf("| Expected Profit Factor | %.2f |\n", pp.ExpectedPF))
	b.WriteString(fmt.Sprintf("| Expected Sharpe | %.2f |\n", pp.ExpectedSharpe))
	b.WriteString(fmt.Sprintf("| Expected Sortino | %.2f |\n", pp.ExpectedSortino))
	b.WriteString(fmt.Sprintf("| Expected Max Drawdown | %.1f%% |\n", pp.ExpectedMaxDD))
	b.WriteString(fmt.Sprintf("| MC Stability | %s |\n\n", pp.ExpectedMCTier))

	h2(b, "Q7. Is the system profitable after costs?")
	b.WriteString(fmt.Sprintf("**%s** — P(growth) after fees and slippage: Portfolio MC P(grow)=%.0f%%\n\n",
		boolLabel(v.Q7_ProfitableAfterCosts, "YES", "NO"), r.PortfolioMC.ProbabilityGrow*100))

	h2(b, "Q8. Is the system institutionally deployable?")
	b.WriteString(fmt.Sprintf("**%s** — Requires PF≥1.30, Sharpe≥1.50, MC STABLE+, ≥3 live-ready strategies.\n\n",
		boolLabel(v.Q8_InstitutionalReady, "YES", "NO")))

	h2(b, "Q9. Top 5–10 strategies in the entire application")
	b.WriteString("| Rank | Strategy | PF | Sharpe | CAGR | DD | MC | Tier |\n")
	b.WriteString("|:---:|:---|:---:|:---:|:---:|:---:|:---:|:---:|\n")
	for _, rs := range v.Q9_Top10 {
		b.WriteString(fmt.Sprintf("| %d | %s | %.2f | %.2f | %.1f%% | %.1f%% | %s | %s |\n",
			rs.Rank, rs.StrategyName, rs.ProfitFactor, rs.Sharpe,
			rs.CAGR, rs.MaxDD, rs.MCTier, rs.CertificationTier))
	}
	b.WriteString("\n")

	h2(b, "Q10. Would you approve capital deployment today?")
	b.WriteString(fmt.Sprintf("**%s**\n\n%s\n\n", deployStr, v.DeployTodayReason))
	return b.String()
}

func boolLabel(ok bool, t, f string) string {
	if ok {
		return t
	}
	return f
}
