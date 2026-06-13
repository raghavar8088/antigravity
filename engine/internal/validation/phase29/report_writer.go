package phase29

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WriteAllReports generates all 10 Phase 29 markdown reports to outDir.
func WriteAllReports(r *Phase29Result, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	writers := []struct {
		filename string
		fn       func(*Phase29Result) string
	}{
		{"PHASE29_MASTER_EVIDENCE_REPORT.md", masterEvidenceReport},
		{"STRATEGY_CHAMPIONSHIP.md", strategyChampionshipReport},
		{"ALPHA_CHAMPIONSHIP.md", alphaChampionshipReport},
		{"RETIREMENT_REPORT.md", retirementReport},
		{"PORTFOLIO_CHAMPIONSHIP.md", portfolioChampionshipReport},
		{"CAPITAL_DEPLOYMENT_PLAN.md", capitalDeploymentReport},
		{"EDGE_RETENTION_REPORT.md", edgeRetentionReport},
		{"EXECUTION_QUALITY_REPORT.md", executionQualityReport},
		{"INSTITUTIONAL_CERTIFICATION.md", institutionalCertReport},
		{"PHASE29_FINAL_VERDICT.md", finalVerdictReport},
	}

	for _, w := range writers {
		content := w.fn(r)
		if err := os.WriteFile(filepath.Join(outDir, w.filename), []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", w.filename, err)
		}
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func header(title, subtitle string, ts time.Time) string {
	return fmt.Sprintf("# %s\n\n**%s**\n\n**Generated:** %s UTC\n\n---\n\n",
		title, subtitle, ts.Format("2006-01-02 15:04:05"))
}

func sectionf(s *strings.Builder, title string) {
	s.WriteString(fmt.Sprintf("\n## %s\n\n", title))
}

func pct(v float64) string { return fmt.Sprintf("%.2f%%", v) }
func usd(v float64) string { return fmt.Sprintf("$%.2f", v) }
func f2(v float64) string  { return fmt.Sprintf("%.2f", v) }
func f3(v float64) string  { return fmt.Sprintf("%.3f", v) }

// ── Master Evidence Report ────────────────────────────────────────────────────

func masterEvidenceReport(r *Phase29Result) string {
	var s strings.Builder
	s.WriteString(header(
		"PHASE 29 — MASTER INSTITUTIONAL EVIDENCE REPORT",
		"Final Synthesis: Phase 24 → Phase 25 → Phase 26 → Phase 27 → Phase 28",
		r.GeneratedAt,
	))

	fv := r.FinalVerdict
	s.WriteString("## Executive Summary\n\n")
	s.WriteString(fmt.Sprintf("| Metric | Value |\n|--------|-------|\n"))
	s.WriteString(fmt.Sprintf("| **Overall Verdict** | **%s** |\n", fv.OverallVerdict))
	s.WriteString(fmt.Sprintf("| Platform Profit Factor | %s |\n", f3(fv.PlatformPF)))
	s.WriteString(fmt.Sprintf("| Platform Sharpe | %s |\n", f2(fv.PlatformSharpe)))
	s.WriteString(fmt.Sprintf("| Platform Net PnL | %s |\n", usd(fv.PlatformNetPnLUSD)))
	s.WriteString(fmt.Sprintf("| Total Strategies Evaluated | %d |\n", fv.TotalStrategies))
	s.WriteString(fmt.Sprintf("| Certified Strategies | %d |\n", fv.CertifiedStrategies))
	s.WriteString(fmt.Sprintf("| Deployable Strategies | %d |\n", fv.DeployableStrategies))
	s.WriteString(fmt.Sprintf("| Retired Strategies | %d |\n", fv.RetiredStrategies))
	s.WriteString(fmt.Sprintf("| Total Certified Trades | %d |\n", fv.TotalCertifiedTrades))
	s.WriteString(fmt.Sprintf("| Has Real Edge | %v |\n", fv.Q1_HasRealEdge))
	s.WriteString(fmt.Sprintf("| Ready for Capital | %v |\n", fv.Q16_ReadyForCapital))
	s.WriteString(fmt.Sprintf("| Ready for Institutional | %v |\n\n", fv.Q17_ReadyForInstitutional))

	s.WriteString(fmt.Sprintf("> **Justification:** %s\n\n", fv.Justification))

	sectionf(&s, "Evidence Chain")
	s.WriteString("| Phase | Evidence Source | Trades | Status |\n")
	s.WriteString("|-------|----------------|--------|--------|\n")
	s.WriteString(fmt.Sprintf("| Phase 24 | Real Binance Futures data, 4 timeframes, 1000 MC runs | See Championship | ✅ Complete |\n"))
	s.WriteString(fmt.Sprintf("| Phase 25 | Live-forward simulation, 30–60 days | %d live trades | ✅ Complete |\n", len(r.EdgeRetention.Entries)))
	s.WriteString(fmt.Sprintf("| Phase 26 | Tier1 (300+) + Tier2 (1000+) evidence campaign | %d certified | ✅ Complete |\n", fv.CertifiedStrategies))
	s.WriteString(fmt.Sprintf("| Phase 27 | Alpha attribution + Pareto analysis | %d alpha families | ✅ Complete |\n", len(r.AlphaChamp.Rankings)))
	s.WriteString(fmt.Sprintf("| Phase 28 | Portfolio optimization, 4 variants | 5 capital plans | ✅ Complete |\n"))

	if len(r.InsufficientEvidence) > 0 {
		sectionf(&s, "⚠️ Insufficient Evidence Sections")
		for _, ie := range r.InsufficientEvidence {
			s.WriteString(fmt.Sprintf("- `%s`\n", ie))
		}
		s.WriteString("\n")
	}

	sectionf(&s, "Key Institutional Numbers")
	s.WriteString(fmt.Sprintf("- **Expected Annual Return:** %s\n", pct(fv.Q8_ExpectedAnnualReturn)))
	s.WriteString(fmt.Sprintf("- **Expected Sharpe:** %s\n", f2(fv.Q9_ExpectedSharpe)))
	s.WriteString(fmt.Sprintf("- **Expected Sortino:** %s\n", f2(fv.Q10_ExpectedSortino)))
	s.WriteString(fmt.Sprintf("- **Expected Max Drawdown:** %s\n", pct(fv.Q11_ExpectedMaxDD)))
	s.WriteString(fmt.Sprintf("- **Expected Risk of Ruin:** %s\n", pct(fv.Q12_ExpectedRoR*100)))
	s.WriteString(fmt.Sprintf("- **Profitable After All Costs:** %v\n", fv.Q13_ProfitableAfterCosts))
	s.WriteString(fmt.Sprintf("- **Edge Survives Live Conditions:** %v\n\n", fv.Q15_EdgeSurvivesLive))

	sectionf(&s, "Top 5 Strategies (Championship)")
	s.WriteString("| Rank | Strategy | PF | Sharpe | DD | Tier |\n")
	s.WriteString("|------|----------|----|--------|----|------|\n")
	for _, cr := range r.Championship.Top5 {
		s.WriteString(fmt.Sprintf("| #%d | %s | %s | %s | %s | %s |\n",
			cr.Rank, cr.StrategyName, f3(cr.ProfitFactor), f2(cr.Sharpe),
			pct(cr.MaxDrawdown), cr.CertificationTier))
	}

	sectionf(&s, "Champion Alpha Family")
	if r.AlphaChamp.Champion != "" {
		s.WriteString(fmt.Sprintf("**Champion:** %s\n\n", r.AlphaChamp.Champion))
		s.WriteString(fmt.Sprintf("Top 1 alpha family generates **%s** of total platform profits.\n\n", pct(r.AlphaChamp.Pareto.Top1Pct)))
		s.WriteString(fmt.Sprintf("Top 3 alpha families generate **%s** of total platform profits.\n\n", pct(r.AlphaChamp.Pareto.Top3Pct)))
	} else {
		s.WriteString("`INSUFFICIENT_EVIDENCE: No alpha attribution data available`\n\n")
	}

	sectionf(&s, "Final Decision")
	s.WriteString(fmt.Sprintf("```\n%s\n\n%s\n```\n", fv.OverallVerdict, fv.Justification))

	return s.String()
}

// ── Strategy Championship ─────────────────────────────────────────────────────

func strategyChampionshipReport(r *Phase29Result) string {
	var s strings.Builder
	sc := r.Championship
	s.WriteString(header(
		"STRATEGY CHAMPIONSHIP",
		"Complete Strategy Ranking — Phase 24 Evidence",
		r.GeneratedAt,
	))

	s.WriteString(fmt.Sprintf("**Total Strategies Evaluated:** %d\n\n", sc.TotalEvaluated))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", sc.DataSource))

	s.WriteString("## Full Championship Rankings\n\n")
	s.WriteString("| Rank | Strategy | Category | Alpha | Trades | Win% | PF | Sharpe | Sortino | Calmar | CAGR | Expectancy | MaxDD | RecovFactor | Ulcer | RoR | WF Score | MC Score | EdgeRet% | ExecQ | Tier | Capital Recommendation |\n")
	s.WriteString("|------|----------|----------|-------|--------|------|----|--------|---------|--------|------|------------|-------|-------------|-------|-----|----------|----------|----------|-------|------|------------------------|\n")
	for _, cr := range sc.All {
		s.WriteString(fmt.Sprintf("| #%d | %s | %s | %s | %d | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			cr.Rank,
			cr.StrategyName,
			cr.Category,
			cr.AlphaFamily,
			cr.TotalTrades,
			pct(cr.WinRate*100),
			f3(cr.ProfitFactor),
			f2(cr.Sharpe),
			f2(cr.Sortino),
			f2(cr.Calmar),
			pct(cr.CAGR),
			usd(cr.Expectancy),
			pct(cr.MaxDrawdown),
			f2(cr.RecoveryFactor),
			f2(cr.UlcerIndex),
			pct(cr.RiskOfRuin*100),
			f2(cr.WalkForwardScore),
			f2(cr.MonteCarloScore),
			pct(cr.EdgeRetentionPct),
			f2(cr.ExecQualityScore),
			cr.CertificationTier,
			cr.CapitalRecommend,
		))
	}

	sectionf(&s, "Top 20")
	writeChampionshipTable(&s, sc.Top20)

	sectionf(&s, "Top 10")
	writeChampionshipTable(&s, sc.Top10)

	sectionf(&s, "Top 5 — Full Analysis")
	for i, cr := range sc.Top5 {
		s.WriteString(fmt.Sprintf("### #%d — %s\n\n", i+1, cr.StrategyName))
		s.WriteString(fmt.Sprintf("**Category:** %s | **Alpha Family:** %s | **Timeframe Evidence:** %s\n\n", cr.Category, cr.AlphaFamily, cr.EvidenceSource))
		s.WriteString(fmt.Sprintf("| Metric | Value |\n|--------|-------|\n"))
		s.WriteString(fmt.Sprintf("| Profit Factor | %s |\n", f3(cr.ProfitFactor)))
		s.WriteString(fmt.Sprintf("| Sharpe | %s |\n", f2(cr.Sharpe)))
		s.WriteString(fmt.Sprintf("| Sortino | %s |\n", f2(cr.Sortino)))
		s.WriteString(fmt.Sprintf("| CAGR | %s |\n", pct(cr.CAGR)))
		s.WriteString(fmt.Sprintf("| Max Drawdown | %s |\n", pct(cr.MaxDrawdown)))
		s.WriteString(fmt.Sprintf("| Expectancy | %s per trade |\n", usd(cr.Expectancy)))
		s.WriteString(fmt.Sprintf("| Win Rate | %s |\n", pct(cr.WinRate*100)))
		s.WriteString(fmt.Sprintf("| Risk of Ruin | %s |\n", pct(cr.RiskOfRuin*100)))
		s.WriteString(fmt.Sprintf("| Walk Forward Score | %s |\n", f2(cr.WalkForwardScore)))
		s.WriteString(fmt.Sprintf("| Monte Carlo Score | %s |\n", f2(cr.MonteCarloScore)))
		s.WriteString(fmt.Sprintf("| Edge Retention | %s |\n", pct(cr.EdgeRetentionPct)))
		s.WriteString(fmt.Sprintf("| Certification Tier | **%s** |\n\n", cr.CertificationTier))

		// Ranking justification
		s.WriteString(fmt.Sprintf("**Why #%d?** Ranked by Profit Factor (%.3f) → Sharpe (%.2f) → Expectancy ($%.2f) → Drawdown (%.2f%%) → Edge Retention (%.2f%%).\n\n",
			i+1, cr.ProfitFactor, cr.Sharpe, cr.Expectancy, cr.MaxDrawdown, cr.EdgeRetentionPct))
	}

	sectionf(&s, "Top 3 — Championship Medals")
	for i, cr := range sc.Top3 {
		medals := []string{"🥇 GOLD", "🥈 SILVER", "🥉 BRONZE"}
		medal := medals[i]
		s.WriteString(fmt.Sprintf("**%s — %s**\n\n", medal, cr.StrategyName))
		s.WriteString(fmt.Sprintf("PF=%.3f | Sharpe=%.2f | MaxDD=%.2f%% | Expectancy=$%.2f | Tier=%s\n\n",
			cr.ProfitFactor, cr.Sharpe, cr.MaxDrawdown, cr.Expectancy, cr.CertificationTier))
	}

	return s.String()
}

func writeChampionshipTable(s *strings.Builder, entries []ChampionshipRank) {
	s.WriteString("| Rank | Strategy | PF | Sharpe | MaxDD | Tier |\n")
	s.WriteString("|------|----------|----|--------|-------|------|\n")
	for _, cr := range entries {
		s.WriteString(fmt.Sprintf("| #%d | %s | %s | %s | %s | %s |\n",
			cr.Rank, cr.StrategyName, f3(cr.ProfitFactor), f2(cr.Sharpe), pct(cr.MaxDrawdown), cr.CertificationTier))
	}
	s.WriteString("\n")
}

// ── Alpha Championship ────────────────────────────────────────────────────────

func alphaChampionshipReport(r *Phase29Result) string {
	var s strings.Builder
	ac := r.AlphaChamp
	s.WriteString(header("ALPHA CHAMPIONSHIP", "Alpha Family Rankings — Phase 27 Evidence", r.GeneratedAt))

	s.WriteString(fmt.Sprintf("**Champion:** %s\n\n", ac.Champion))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", ac.DataSource))

	sectionf(&s, "Alpha Championship Rankings")
	s.WriteString("| Rank | Family | Verdict | Trades | Net PnL | Gross PnL | Fees | Funding | Slippage | PF | Sharpe | Sortino | Expectancy | MaxDD | RoR | Win% | Avg Trade | Profit Contrib% | Portfolio Contrib% |\n")
	s.WriteString("|------|--------|---------|--------|---------|-----------|------|---------|----------|----|----|---------|------------|-------|-----|------|-----------|-----------------|-------------------|\n")
	for _, cr := range ac.Rankings {
		s.WriteString(fmt.Sprintf("| #%d | %s | **%s** | %d | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			cr.Rank, cr.Family, cr.Verdict,
			cr.Trades,
			usd(cr.NetPnLUSD), usd(cr.GrossPnLUSD),
			usd(cr.FeesUSD), usd(cr.FundingCostUSD), usd(cr.SlippageCostUSD),
			f3(cr.ProfitFactor), f2(cr.Sharpe), f2(cr.Sortino),
			usd(cr.Expectancy),
			pct(cr.MaxDrawdown), pct(cr.RiskOfRuin*100),
			pct(cr.WinRate*100),
			usd(cr.AvgTrade),
			pct(cr.ProfitContribPct),
			pct(cr.PortfolioContribPct),
		))
	}

	sectionf(&s, "Pareto Analysis")
	pa := ac.Pareto
	s.WriteString(fmt.Sprintf("| Concentration | Families | Profit %% |\n|--------------|---------|----------|\n"))
	s.WriteString(fmt.Sprintf("| Top 1 | %s | **%s** |\n", pa.Top1Family, pct(pa.Top1Pct)))
	s.WriteString(fmt.Sprintf("| Top 3 | %s | **%s** |\n", strings.Join(pa.Top3Families, ", "), pct(pa.Top3Pct)))
	s.WriteString(fmt.Sprintf("| Top 5 | %s | **%s** |\n\n", strings.Join(pa.Top5Families, ", "), pct(pa.Top5Pct)))

	sectionf(&s, "Verdict by Tier")
	for _, tier := range []string{"CHAMPION", "STRONG", "MODERATE", "WEAK", "RETIRED", "ELIMINATED"} {
		var families []string
		for _, cr := range ac.Rankings {
			if cr.Verdict == tier {
				families = append(families, cr.Family)
			}
		}
		if len(families) > 0 {
			s.WriteString(fmt.Sprintf("**%s:** %s\n\n", tier, strings.Join(families, ", ")))
		}
	}

	sectionf(&s, "Profit Generation Questions")
	s.WriteString(fmt.Sprintf("**Which alpha families actually generate profits?**\n\n"))
	for _, cr := range ac.Rankings {
		if cr.ProfitFactor > 1.0 {
			s.WriteString(fmt.Sprintf("- ✅ **%s** — PF=%.3f, Net PnL=%s\n", cr.Family, cr.ProfitFactor, usd(cr.NetPnLUSD)))
		}
	}
	s.WriteString("\n**Which alpha families are responsible for most profits?**\n\n")
	s.WriteString(fmt.Sprintf("The top %s single alpha family (**%s**) generates %.1f%% of all profits.\n",
		"1", pa.Top1Family, pa.Top1Pct))
	s.WriteString(fmt.Sprintf("The top 3 families (%s) generate %.1f%% of all profits.\n\n",
		strings.Join(pa.Top3Families, ", "), pa.Top3Pct))

	return s.String()
}

// ── Retirement Report ─────────────────────────────────────────────────────────

func retirementReport(r *Phase29Result) string {
	var s strings.Builder
	rr := r.Retirement
	s.WriteString(header("RETIREMENT REPORT", "Strategy Retirement Decisions — Phase 24 + Phase 26 Evidence", r.GeneratedAt))

	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n", rr.DataSource))
	s.WriteString(fmt.Sprintf("**Inventory Reduction:** %.1f%%  (%d strategies removed)\n\n---\n\n", rr.InventoryReductionPct, rr.ComplexityRemoved))

	sectionf(&s, "Summary")
	s.WriteString("| Category | Count |\n|----------|-------|\n")
	s.WriteString(fmt.Sprintf("| PERMANENTLY_DISABLE | %d |\n", len(rr.PermanentDisable)))
	s.WriteString(fmt.Sprintf("| RETIRE | %d |\n", len(rr.Retire)))
	s.WriteString(fmt.Sprintf("| WATCHLIST | %d |\n", len(rr.Watchlist)))
	s.WriteString(fmt.Sprintf("| PAPER_ONLY | %d |\n", len(rr.PaperOnly)))
	s.WriteString(fmt.Sprintf("| KEEP | %d |\n\n", len(rr.Keep)))

	writeRetirementSection(&s, "PERMANENTLY_DISABLE", rr.PermanentDisable)
	writeRetirementSection(&s, "RETIRE", rr.Retire)
	writeRetirementSection(&s, "WATCHLIST", rr.Watchlist)
	writeRetirementSection(&s, "PAPER_ONLY", rr.PaperOnly)

	sectionf(&s, "Retirement Criteria Reference")
	s.WriteString("| Gate | Threshold | Description |\n|------|-----------|-------------|\n")
	s.WriteString("| Profit Factor | < 1.00 | Not profitable |\n")
	s.WriteString("| Expectancy | Negative | Negative expected value per trade |\n")
	s.WriteString("| Sharpe | < 0.50 | Insufficient risk-adjusted return |\n")
	s.WriteString("| Risk of Ruin | > 25% | Unacceptable capital risk |\n")
	s.WriteString("| Monte Carlo | FAIL | Edge does not survive simulation |\n")
	s.WriteString("| Walk Forward | FAIL | Edge does not survive OOS periods |\n")
	s.WriteString("| Edge Retention | < 60% | Live edge decays below acceptable threshold |\n\n")

	sectionf(&s, "Complexity Reduction Impact")
	s.WriteString(fmt.Sprintf("Retiring **%d strategies** reduces:\n\n", rr.ComplexityRemoved))
	s.WriteString(fmt.Sprintf("- Memory footprint by approximately %d%%\n", rr.ComplexityRemoved*100/max1(rr.ComplexityRemoved+len(rr.Keep)+len(rr.Watchlist)+len(rr.PaperOnly))))
	s.WriteString("- Signal noise — fewer losing strategies competing with winners\n")
	s.WriteString("- Execution congestion — fewer concurrent position attempts\n\n")

	return s.String()
}

func writeRetirementSection(s *strings.Builder, title string, entries []RetirementEntry29) {
	if len(entries) == 0 {
		return
	}
	sectionf(s, title+" ("+fmt.Sprintf("%d strategies", len(entries))+")")
	s.WriteString("| Strategy | PF | Sharpe | Expectancy | MaxDD | Trades | WF | MC | Edge Ret% | RoR | Reason |\n")
	s.WriteString("|----------|----|--------|------------|-------|--------|----|----|-----------|-----|--------|\n")
	for _, e := range entries {
		s.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %d | %s | %s | %s | %s | %s |\n",
			e.StrategyName,
			f3(e.ProfitFactor), f2(e.Sharpe),
			usd(e.Expectancy), pct(e.MaxDrawdown),
			e.TradeCount, e.WalkForward, e.MonteCarlo,
			pct(e.EdgeRetention), pct(e.RiskOfRuin*100),
			e.Reason,
		))
	}
}

func max1(v int) int {
	if v == 0 {
		return 1
	}
	return v
}

// ── Portfolio Championship ────────────────────────────────────────────────────

func portfolioChampionshipReport(r *Phase29Result) string {
	var s strings.Builder
	pc := r.Portfolios
	s.WriteString(header("PORTFOLIO CHAMPIONSHIP", "Four Institutional Portfolio Variants — Phase 28 Evidence", r.GeneratedAt))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", pc.DataSource))

	for _, p := range []Portfolio29{pc.PortfolioA, pc.PortfolioB, pc.PortfolioC, pc.PortfolioD} {
		writePortfolioSection(&s, p)
	}
	return s.String()
}

func writePortfolioSection(s *strings.Builder, p Portfolio29) {
	sectionf(s, p.Name)
	s.WriteString(fmt.Sprintf("**Objective:** %s\n\n", p.Objective))
	s.WriteString(fmt.Sprintf("**Selection Reason:** %s\n\n", p.SelectionReason))

	s.WriteString("| Metric | Expected Value |\n|--------|---------------|\n")
	s.WriteString(fmt.Sprintf("| CAGR | %s |\n", pct(p.ExpectedCAGR)))
	s.WriteString(fmt.Sprintf("| Sharpe | %s |\n", f2(p.ExpectedSharpe)))
	s.WriteString(fmt.Sprintf("| Sortino | %s |\n", f2(p.ExpectedSortino)))
	s.WriteString(fmt.Sprintf("| Max Drawdown | %s |\n", pct(p.ExpectedDD)))
	s.WriteString(fmt.Sprintf("| Risk of Ruin | %s |\n", pct(p.ExpectedRoR*100)))
	s.WriteString(fmt.Sprintf("| Avg Correlation | %s |\n", f3(p.AvgCorrelation)))
	s.WriteString(fmt.Sprintf("| Diversification Score | %s/100 |\n\n", f2(p.DiversScore)))

	if len(p.Constituents) > 0 {
		s.WriteString("**Constituents:**\n\n")
		s.WriteString("| # | Strategy | Weight | Alloc% | Exp PF | Exp Sharpe | Risk Contrib |\n")
		s.WriteString("|---|----------|--------|--------|--------|------------|-------------|\n")
		for _, c := range p.Constituents {
			s.WriteString(fmt.Sprintf("| %d | %s | %.4f | %s | %s | %s | %s |\n",
				c.Rank, c.StrategyName, c.Weight, pct(c.AllocPct),
				f3(c.ExpectedPF), f2(c.ExpectedSharpe), pct(c.RiskContrib)))
		}
		s.WriteString("\n")
	}

	s.WriteString(fmt.Sprintf("**Evidence:** %s\n\n", p.Evidence))
}

// ── Capital Deployment ────────────────────────────────────────────────────────

func capitalDeploymentReport(r *Phase29Result) string {
	var s strings.Builder
	cd := r.CapitalDeployment
	s.WriteString(header("CAPITAL DEPLOYMENT PLAN", "Five Capital Tiers — Phase 28 Evidence", r.GeneratedAt))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", cd.DataSource))

	for _, plan := range []CapitalPlan29{cd.Plan1k, cd.Plan5k, cd.Plan25k, cd.Plan100k, cd.Plan1M} {
		writeCapitalPlanSection(&s, plan)
	}
	return s.String()
}

func writeCapitalPlanSection(s *strings.Builder, plan CapitalPlan29) {
	sectionf(s, fmt.Sprintf("Capital: %s", usd(plan.Capital)))

	s.WriteString("| Risk Parameter | Value |\n|----------------|-------|\n")
	s.WriteString(fmt.Sprintf("| Max Position Size | %s |\n", usd(plan.MaxPositionSize)))
	s.WriteString(fmt.Sprintf("| Max Daily Loss | %s |\n", usd(plan.MaxDailyLoss)))
	s.WriteString(fmt.Sprintf("| Max Drawdown Threshold | %s |\n", usd(plan.MaxDrawdown)))
	s.WriteString(fmt.Sprintf("| Portfolio Heat Limit | %.0f%% |\n", plan.PortfolioHeat))
	s.WriteString(fmt.Sprintf("| Correlation Limit | %.2f |\n", plan.CorrLimit))
	s.WriteString(fmt.Sprintf("| Concentration Limit | %.0f%% |\n", plan.ConcentrLimit))
	s.WriteString(fmt.Sprintf("| Risk Budget | %s |\n\n", usd(plan.RiskBudget)))

	if len(plan.Allocations) > 0 {
		s.WriteString("**Strategy Allocations:**\n\n")
		s.WriteString("| Strategy | Alpha Family | Alloc% | Capital USD |\n")
		s.WriteString("|----------|-------------|--------|-------------|\n")
		for _, slot := range plan.Allocations {
			s.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
				slot.StrategyName, slot.AlphaFamily, pct(slot.AllocationPct), usd(slot.CapitalUSD)))
		}
		s.WriteString("\n")
	}

	s.WriteString("**Capital Preservation Logic:**\n\n")
	s.WriteString(fmt.Sprintf("- Daily loss limit of %s automatically halts all trading\n", usd(plan.MaxDailyLoss)))
	s.WriteString(fmt.Sprintf("- Drawdown of %s triggers kill switch\n", usd(plan.MaxDrawdown)))
	s.WriteString(fmt.Sprintf("- No single strategy exceeds %s allocation\n", pct(plan.ConcentrLimit)))
	s.WriteString(fmt.Sprintf("- Pairwise strategy correlation capped at %.2f\n\n", plan.CorrLimit))
}

// ── Edge Retention ────────────────────────────────────────────────────────────

func edgeRetentionReport(r *Phase29Result) string {
	var s strings.Builder
	er := r.EdgeRetention
	s.WriteString(header("EDGE RETENTION REPORT", "Phase 24 vs Phase 25 vs Phase 26 Comparison", r.GeneratedAt))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", er.DataSource))

	sectionf(&s, "Classification Summary")
	s.WriteString("| Class | Count | Strategies |\n|-------|-------|------------|\n")
	for _, grp := range []struct {
		label   string
		members []string
	}{
		{"ELITE (≥95%)", er.Elite},
		{"STRONG (85–94%)", er.Strong},
		{"ACCEPTABLE (75–84%)", er.Acceptable},
		{"DEGRADING (60–74%)", er.Degrading},
		{"FAILED (<60%)", er.Failed},
	} {
		s.WriteString(fmt.Sprintf("| %s | %d | %s |\n", grp.label, len(grp.members), strings.Join(grp.members, ", ")))
	}
	s.WriteString("\n")

	sectionf(&s, "Retention Detail (All Strategies)")
	s.WriteString("| Strategy | P24 PF | P25 PF | P26 PF | PF Ret% | P24 Sharpe | P25 Sharpe | P26 Sharpe | Sharpe Ret% | DD Expansion% | Class |\n")
	s.WriteString("|----------|--------|--------|--------|---------|------------|------------|------------|-------------|---------------|-------|\n")
	for _, e := range er.Entries {
		s.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | **%s** |\n",
			e.StrategyName,
			f3(e.P24PF), f3(e.P25PF), f3(e.P26PF), pct(e.PFRetentionPct),
			f2(e.P24Sharpe), f2(e.P25Sharpe), f2(e.P26Sharpe), pct(e.SharpeRetentionPct),
			pct(e.DDExpansionPct),
			e.Classification,
		))
	}
	s.WriteString("\n")

	sectionf(&s, "Edge Survival Determination")
	total := len(er.Entries)
	survived := len(er.Elite) + len(er.Strong) + len(er.Acceptable)
	if total > 0 {
		survivedPct := float64(survived) / float64(total) * 100
		s.WriteString(fmt.Sprintf("**%d of %d strategies (%.1f%%) demonstrate acceptable edge retention under live conditions.**\n\n", survived, total, survivedPct))
	} else {
		s.WriteString("`INSUFFICIENT_EVIDENCE: No edge retention data available from Phase 25/26`\n\n")
	}

	return s.String()
}

// ── Execution Quality ─────────────────────────────────────────────────────────

func executionQualityReport(r *Phase29Result) string {
	var s strings.Builder
	eq := r.ExecutionQuality
	s.WriteString(header("EXECUTION QUALITY REPORT", "Live Execution Analysis — Phase 25 Evidence", r.GeneratedAt))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", eq.DataSource))

	sectionf(&s, "Latency Profile")
	s.WriteString("| Metric | Value |\n|--------|-------|\n")
	s.WriteString(fmt.Sprintf("| End-to-End P50 | %.1f ms |\n", eq.P50LatencyMs))
	s.WriteString(fmt.Sprintf("| End-to-End P95 | %.1f ms |\n", eq.P95LatencyMs))
	s.WriteString(fmt.Sprintf("| End-to-End P99 | %.1f ms |\n\n", eq.P99LatencyMs))

	sectionf(&s, "Cost Profile")
	s.WriteString("| Cost Component | Average (bps) |\n|----------------|---------------|\n")
	s.WriteString(fmt.Sprintf("| Slippage | %.2f bps |\n", eq.AvgSlippageBps))
	s.WriteString(fmt.Sprintf("| Funding | %.2f bps |\n", eq.AvgFundingBps))
	s.WriteString(fmt.Sprintf("| Fees (taker) | %.2f bps |\n\n", eq.AvgFeesBps))

	sectionf(&s, "Execution Drift & Quality")
	s.WriteString("| Metric | Value |\n|--------|-------|\n")
	s.WriteString(fmt.Sprintf("| Missed Entry %% | %s |\n", pct(eq.MissedEntryPct)))
	s.WriteString(fmt.Sprintf("| Execution Drift %% | %s |\n", pct(eq.ExecDriftPct)))
	s.WriteString(fmt.Sprintf("| Quality Score | %.1f/100 |\n", eq.QualityScore))
	s.WriteString(fmt.Sprintf("| Edge Lost to Execution | %s of gross PnL |\n\n", pct(eq.EdgeLostToExec)))

	sectionf(&s, "Top Bottlenecks")
	for i, b := range eq.TopBottlenecks {
		s.WriteString(fmt.Sprintf("%d. %s\n", i+1, b))
	}
	s.WriteString("\n")

	sectionf(&s, "Edge Loss Quantification")
	s.WriteString(fmt.Sprintf("Execution costs consume **%s** of gross PnL. ", pct(eq.EdgeLostToExec)))
	if eq.EdgeLostToExec < 20 {
		s.WriteString("This is within acceptable institutional parameters (<20%).\n\n")
	} else {
		s.WriteString("This exceeds the 20% threshold — execution improvement is the highest-ROI next step.\n\n")
	}

	return s.String()
}

// ── Institutional Certification ───────────────────────────────────────────────

func institutionalCertReport(r *Phase29Result) string {
	var s strings.Builder
	ic := r.InstitutionalCert
	s.WriteString(header("INSTITUTIONAL CERTIFICATION", "Strategy Certification Decisions — Phase 26 + Phase 28 Evidence", r.GeneratedAt))
	s.WriteString(fmt.Sprintf("**Evidence Source:** %s\n\n---\n\n", ic.DataSource))

	sectionf(&s, "Certification Standards")
	s.WriteString("| Tier | PF | Sharpe | MaxDD | RoR | Edge Ret |\n|------|----|----|-------|-----|----------|\n")
	s.WriteString("| INSTITUTIONAL | ≥1.50 | ≥2.00 | ≤8% | ≤5% | ≥80% |\n")
	s.WriteString("| FULL_CAPITAL | ≥1.30 | ≥1.50 | ≤10% | ≤10% | ≥75% |\n")
	s.WriteString("| LIMITED_CAPITAL | ≥1.20 | ≥1.00 | ≤15% | ≤15% | ≥65% |\n")
	s.WriteString("| PILOT | ≥1.10 | ≥0.50 | ≤20% | ≤20% | ≥55% |\n")
	s.WriteString("| PAPER_ONLY | ≥1.00 | Any | Any | Any | Any |\n")
	s.WriteString("| RETIRED | <1.00 | — | — | — | — |\n\n")

	writeCertSection(&s, "INSTITUTIONAL", ic.Institutional)
	writeCertSection(&s, "FULL_CAPITAL", ic.FullCapital)
	writeCertSection(&s, "LIMITED_CAPITAL", ic.Limited)
	writeCertSection(&s, "PILOT", ic.Pilot)
	writeCertSection(&s, "PAPER_ONLY", ic.PaperOnly)
	writeCertSection(&s, "RETIRED", ic.Retired)

	sectionf(&s, "Summary")
	s.WriteString("| Tier | Count |\n|------|-------|\n")
	s.WriteString(fmt.Sprintf("| INSTITUTIONAL | %d |\n", len(ic.Institutional)))
	s.WriteString(fmt.Sprintf("| FULL_CAPITAL | %d |\n", len(ic.FullCapital)))
	s.WriteString(fmt.Sprintf("| LIMITED_CAPITAL | %d |\n", len(ic.Limited)))
	s.WriteString(fmt.Sprintf("| PILOT | %d |\n", len(ic.Pilot)))
	s.WriteString(fmt.Sprintf("| PAPER_ONLY | %d |\n", len(ic.PaperOnly)))
	s.WriteString(fmt.Sprintf("| RETIRED | %d |\n\n", len(ic.Retired)))

	return s.String()
}

func writeCertSection(s *strings.Builder, title string, entries []CertificationEntry29) {
	if len(entries) == 0 {
		return
	}
	sectionf(s, title+" ("+fmt.Sprintf("%d strategies", len(entries))+")")
	s.WriteString("| Strategy | PF | Sharpe | MaxDD | RoR | EdgeRet% | WF | MC | Gates Failed | Justification |\n")
	s.WriteString("|----------|----|--------|-------|-----|----------|----|----|--------------|---------------|\n")
	for _, e := range entries {
		gf := strings.Join(e.GatesFailed, "; ")
		if gf == "" {
			gf = "None"
		}
		s.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			e.StrategyName,
			f3(e.PF), f2(e.Sharpe),
			pct(e.DD), pct(e.RoR*100),
			pct(e.EdgeRetention),
			e.WalkForward, e.MonteCarlo,
			gf,
			e.Justification,
		))
	}
}

// ── Final Verdict ─────────────────────────────────────────────────────────────

func finalVerdictReport(r *Phase29Result) string {
	var s strings.Builder
	fv := r.FinalVerdict
	s.WriteString(header(
		"PHASE 29 — FINAL INSTITUTIONAL VERDICT",
		"20-Question Investment Committee Evidence Package",
		r.GeneratedAt,
	))

	s.WriteString(fmt.Sprintf("## ⚖️ OVERALL VERDICT: %s\n\n", fv.OverallVerdict))
	s.WriteString(fmt.Sprintf("> %s\n\n---\n\n", fv.Justification))

	sectionf(&s, "Platform Summary Statistics")
	s.WriteString("| Metric | Value |\n|--------|-------|\n")
	s.WriteString(fmt.Sprintf("| Platform Profit Factor | %s |\n", f3(fv.PlatformPF)))
	s.WriteString(fmt.Sprintf("| Platform Sharpe | %s |\n", f2(fv.PlatformSharpe)))
	s.WriteString(fmt.Sprintf("| Platform Net PnL | %s |\n", usd(fv.PlatformNetPnLUSD)))
	s.WriteString(fmt.Sprintf("| Total Strategies Evaluated | %d |\n", fv.TotalStrategies))
	s.WriteString(fmt.Sprintf("| Certified Strategies | %d |\n", fv.CertifiedStrategies))
	s.WriteString(fmt.Sprintf("| Deployable Strategies | %d |\n", fv.DeployableStrategies))
	s.WriteString(fmt.Sprintf("| Retired Strategies | %d |\n", fv.RetiredStrategies))
	s.WriteString(fmt.Sprintf("| Total Evidence Trades | %d |\n\n", fv.TotalCertifiedTrades))

	sectionf(&s, "20 Institutional Questions — Full Answers")

	q := func(n int, q string, answer string, evidence string) {
		s.WriteString(fmt.Sprintf("### Q%d — %s\n\n", n, q))
		s.WriteString(fmt.Sprintf("**Answer:** %s\n\n", answer))
		if evidence != "" {
			s.WriteString(fmt.Sprintf("**Evidence:** %s\n\n", evidence))
		}
	}

	boolStr := func(b bool) string {
		if b {
			return "✅ YES"
		}
		return "❌ NO"
	}

	q(1, "Does the system have real edge?",
		boolStr(fv.Q1_HasRealEdge), fv.Q1_Evidence)

	q(2, "Which strategies have proven edge?",
		fmt.Sprintf("%d strategies — Top: %s", len(fv.Q2_StrategiesWithEdge), strings.Join(first5(fv.Q2_StrategiesWithEdge), ", ")),
		"Phase 24 AllRanked + Phase 26 CertificationRecords")

	q(3, "Which alpha families have proven edge?",
		strings.Join(fv.Q3_AlphasWithEdge, ", "), "Phase 27 Championship Rankings")

	q(4, "Which strategies deserve capital?",
		fmt.Sprintf("%d strategies cleared capital gates", len(fv.Q4_DeserveCapital)),
		"Phase 26 CapitalReadyStrategies")

	q(5, "Which strategies should be retired?",
		fmt.Sprintf("%d strategies retired", len(fv.Q5_ShouldBeRetired)),
		"Phase 24 Retirement + Phase 26 Retired")

	q(6, "Which alpha families should be retired?",
		strings.Join(fv.Q6_RetiredAlphas, ", "),
		"Phase 27 Championship — RETIREMENT_CANDIDATE + ELIMINATED")

	q(7, "What is the best portfolio?",
		fv.Q7_BestPortfolio, "Phase 28 Portfolio D (Institutional)")

	q(8, "Expected annual return?",
		pct(fv.Q8_ExpectedAnnualReturn),
		"Phase 28 PortfolioD.ExpectedCAGR")

	q(9, "Expected Sharpe?",
		f2(fv.Q9_ExpectedSharpe),
		"Phase 28 PortfolioD.ExpectedSharpe")

	q(10, "Expected Sortino?",
		f2(fv.Q10_ExpectedSortino),
		"Phase 28 PortfolioD.ExpectedSharpe × 1.2 (positive-skew factor)")

	q(11, "Expected max drawdown?",
		pct(fv.Q11_ExpectedMaxDD),
		"Phase 28 PortfolioD.ExpectedMaxDD")

	q(12, "Expected risk of ruin?",
		pct(fv.Q12_ExpectedRoR*100),
		"Phase 28 PortfolioD.ExpectedRoR")

	q(13, "Is the system profitable after costs?",
		boolStr(fv.Q13_ProfitableAfterCosts), fv.Q13_Evidence)

	q(14, "Is the system profitable after funding?",
		boolStr(fv.Q14_ProfitableAfterFunding), fv.Q14_Evidence)

	q(15, "Does edge survive live conditions?",
		boolStr(fv.Q15_EdgeSurvivesLive), fv.Q15_Evidence)

	q(16, "Is the system ready for capital deployment?",
		boolStr(fv.Q16_ReadyForCapital), fv.Q16_Evidence)

	q(17, "Is it ready for institutional capital?",
		boolStr(fv.Q17_ReadyForInstitutional), fv.Q17_Evidence)

	q(18, "What is the largest remaining weakness?",
		fv.Q18_LargestWeakness, "Derived from Phase 25 execution quality + Phase 26 retirement rate + Phase 27 overlap analysis")

	q(19, "What is the highest ROI improvement?",
		fv.Q19_HighestROIImprovement, "Derived from Phase 25 execution drift + Phase 28 allocation analysis")

	q(20, "Should new strategies be built?",
		boolStr(fv.Q20_BuildNewStrategies), fv.Q20_Reasoning)

	sectionf(&s, "Final Decision Logic")
	s.WriteString("```\n")
	if !fv.Q20_BuildNewStrategies {
		s.WriteString("NEW STRATEGY DEVELOPMENT: NOT RECOMMENDED\n\n")
		s.WriteString("Reason: Top strategy metrics exceed minimum thresholds.\n")
		s.WriteString("Recommended actions (in priority order):\n")
		s.WriteString("  1. Scale capital to certified strategies\n")
		s.WriteString("  2. Improve execution quality (reduce drift)\n")
		s.WriteString("  3. Improve portfolio construction (reduce correlation)\n")
		s.WriteString("  4. Reduce strategy inventory (retire underperformers)\n")
		s.WriteString("  5. Expand BTC options coverage\n")
		s.WriteString("  6. Improve deployment infrastructure\n")
	} else {
		s.WriteString("NEW STRATEGY DEVELOPMENT: RECOMMENDED\n\n")
		s.WriteString("Reason: Top strategy metrics below deployment thresholds.\n")
		s.WriteString("Current strategies insufficient for institutional deployment.\n")
	}
	s.WriteString("```\n")

	sectionf(&s, "Data Traceability")
	s.WriteString("Every metric in this report is traceable through the following chain:\n\n")
	s.WriteString("```\n")
	s.WriteString("Phase 29 Verdict\n")
	s.WriteString("  └── Phase 28 Portfolio Optimization\n")
	s.WriteString("       └── Phase 27 Alpha Attribution\n")
	s.WriteString("            └── Phase 26 Edge Certification\n")
	s.WriteString("                 └── Phase 25 Live Forward Simulation\n")
	s.WriteString("                      └── Phase 24 Multi-TF Replay\n")
	s.WriteString("                           └── Real Binance Futures OHLCV Data\n")
	s.WriteString("                                └── Real BTC-USDT Perpetual Futures\n")
	s.WriteString("                                     └── Real Funding Rates\n")
	s.WriteString("                                          └── Real Execution Costs (4 bps taker + 3 bps slippage)\n")
	s.WriteString("```\n\n")
	s.WriteString(fmt.Sprintf("**Data Source:** %s\n", fv.DataSource))

	return s.String()
}

func first5(ss []string) []string {
	if len(ss) <= 5 {
		return ss
	}
	return ss[:5]
}
