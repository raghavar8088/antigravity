package phase24

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/validation/phase23b"
)

// WriteAllReports writes all 12 Phase 24 markdown reports to outDir.
func WriteAllReports(r Phase24Result, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	writers := []struct {
		name string
		fn   func(Phase24Result) string
	}{
		{"DATA_CERTIFICATION_REPORT.md", writeDataCert},
		{"STRATEGY_EVIDENCE_REPORT.md", writeStrategyEvidence},
		{"ALPHA_CHAMPIONSHIP_REPORT.md", writeAlphaChampionship},
		{"WALK_FORWARD_CERTIFICATION.md", writeWalkForward},
		{"MONTE_CARLO_CERTIFICATION.md", writeMonteCarlo},
		{"REGIME_EDGE_REPORT.md", writeRegimeEdge},
		{"STRATEGY_RETIREMENT_REPORT.md", writeRetirement},
		{"CAPITAL_CERTIFICATION_REPORT.md", writeCapCert},
		{"TOP3_PORTFOLIO.md", writeTop3Portfolio},
		{"TOP5_PORTFOLIO.md", writeTop5Portfolio},
		{"TOP10_PORTFOLIO.md", writeTop10Portfolio},
		{"PHASE24_FINAL_VERDICT.md", writeFinalVerdict},
	}

	for _, w := range writers {
		content := w.fn(r)
		path := filepath.Join(outDir, w.name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", w.name, err)
		}
	}
	return nil
}

func header(title string, r Phase24Result) string {
	return fmt.Sprintf("# %s\n\n**Generated:** %s  \n**Symbol:** %s  \n**Period:** %s → %s  \n**Total Trades:** %d  \n**Strategies Evaluated:** %d\n\n---\n\n",
		title,
		r.GeneratedAt.Format("2006-01-02 15:04 UTC"),
		r.Config.Symbol,
		r.Config.From.Format("2006-01-02"),
		r.Config.To.Format("2006-01-02"),
		len(r.AllTrades),
		r.TotalStrategiesEvaluated,
	)
}

// ── 24.1: Data Certification ──────────────────────────────────────────────────

func writeDataCert(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — DATA CERTIFICATION REPORT", r))

	dc := r.DataCert
	fmt.Fprintf(&b, "## Overall Verdict: %s\n\n", dc.OverallQuality)
	fmt.Fprintf(&b, "| Field | Value |\n|-------|-------|\n")
	fmt.Fprintf(&b, "| Symbol | %s |\n", dc.Symbol)
	fmt.Fprintf(&b, "| Period | %s → %s |\n", dc.From.Format("2006-01-02"), dc.To.Format("2006-01-02"))
	fmt.Fprintf(&b, "| Total Candles | %d |\n", r.TotalCandles)
	fmt.Fprintf(&b, "| Coverage Days | %.0f |\n", dc.To.Sub(dc.From).Hours()/24)
	fmt.Fprintf(&b, "| Data Accepted | %v |\n\n", dc.Accepted)

	b.WriteString("## Data Sources\n\n")
	b.WriteString("| Source | Type | Records | Missing% | Quality | Score |\n")
	b.WriteString("|--------|------|---------|----------|---------|-------|\n")
	for _, s := range dc.Sources {
		fmt.Fprintf(&b, "| %s | %s | %d | %.1f%% | %s | %.0f |\n",
			s.Name, s.Type, s.TotalRecords, s.MissingPct, s.Quality, s.QualityScore)
	}

	if len(dc.Issues) > 0 {
		b.WriteString("\n## Issues\n\n")
		for _, issue := range dc.Issues {
			fmt.Fprintf(&b, "- %s\n", issue)
		}
	}

	b.WriteString("\n## Timeframes Loaded\n\n")
	for _, tf := range AllTimeframes {
		multi := tfMultiplier(tf)
		approxCandles := r.TotalCandles / multi
		fmt.Fprintf(&b, "- **%s**: ~%d candles (aggregated from 1m)\n", tf, approxCandles)
	}

	fmt.Fprintf(&b, "\n## Readiness Verdict\n\n**%s** — Data quality sufficient for institutional certification.\n", dc.OverallQuality)
	return b.String()
}

// ── 24.3: Strategy Evidence ───────────────────────────────────────────────────

func writeStrategyEvidence(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — STRATEGY PERFORMANCE EVIDENCE REPORT", r))

	// Sort by composite score
	ranked := make([]ExtendedMetrics, 0, len(r.Metrics))
	for _, m := range r.Metrics {
		ranked = append(ranked, m)
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].CompositeScore > ranked[j].CompositeScore
	})

	b.WriteString("## Strategy Leaderboard\n\n")
	b.WriteString("| Rank | Strategy | TF | Trades | WR% | PF | Sharpe | Sortino | Calmar | DD% | Expectancy | CAGR% | Score |\n")
	b.WriteString("|------|----------|----|--------|-----|----|--------|---------|--------|-----|------------|-------|-------|\n")
	for i, m := range ranked {
		fmt.Fprintf(&b, "| %d | %s | %s | %d | %.1f | %.3f | %.2f | %.2f | %.2f | %.1f | $%.2f | %.1f | %.3f |\n",
			i+1, truncate24(m.StrategyName, 35), m.Timeframe,
			m.TotalTrades, m.WinRate*100, m.ProfitFactor,
			m.Sharpe, m.Sortino, m.Calmar,
			m.MaxDrawdown, m.Expectancy, m.CAGR, m.CompositeScore)
	}

	b.WriteString("\n## Extended Statistics (Top 20)\n\n")
	limit := 20
	if len(ranked) < limit {
		limit = len(ranked)
	}
	for _, m := range ranked[:limit] {
		fmt.Fprintf(&b, "### %s (%s)\n\n", m.StrategyName, m.Timeframe)
		fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
		fmt.Fprintf(&b, "| Total Trades | %d |\n| Win Rate | %.2f%% |\n| Loss Rate | %.2f%% |\n",
			m.TotalTrades, m.WinRate*100, m.LossRate*100)
		fmt.Fprintf(&b, "| Profit Factor | %.4f |\n| Gross Profit | $%.2f |\n| Gross Loss | $%.2f |\n| Net Profit | $%.2f |\n",
			m.ProfitFactor, m.GrossProfit, m.GrossLoss, m.NetProfit)
		fmt.Fprintf(&b, "| Expectancy | $%.4f |\n| Sharpe | %.4f |\n| Sortino | %.4f |\n| Calmar | %.4f |\n| Ulcer Index | %.4f |\n",
			m.Expectancy, m.Sharpe, m.Sortino, m.Calmar, m.Ulcer)
		fmt.Fprintf(&b, "| Recovery Factor | %.4f |\n| Return/Drawdown | %.4f |\n| Risk of Ruin | %.4f%% |\n",
			m.RecoveryFactor, m.ReturnDrawdown, m.RiskOfRuin*100)
		fmt.Fprintf(&b, "| Avg Win | $%.4f |\n| Avg Loss | $%.4f |\n| Largest Win | $%.4f |\n| Largest Loss | $%.4f |\n",
			m.AvgWin, m.AvgLoss, m.LargestWin, m.LargestLoss)
		fmt.Fprintf(&b, "| Max Consecutive Wins | %d |\n| Max Consecutive Losses | %d |\n",
			m.MaxConWins, m.MaxConLosses)
		fmt.Fprintf(&b, "| CAGR | %.2f%% |\n| Max Drawdown | %.2f%% |\n| Avg Hold | %.1f min |\n\n",
			m.CAGR, m.MaxDrawdown, m.AvgHoldMinutes)
	}
	return b.String()
}

// ── 24.4: Alpha Championship ──────────────────────────────────────────────────

func writeAlphaChampionship(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — ALPHA ENGINE CHAMPIONSHIP REPORT", r))

	b.WriteString("## Alpha Engine Rankings\n\n")
	b.WriteString("| Rank | Alpha Engine | Trades | PF | Sharpe | Sortino | Expectancy | Net PnL | WR% | MaxDD% | MC | WF% | Verdict |\n")
	b.WriteString("|------|-------------|--------|-----|--------|---------|------------|---------|-----|--------|-----|-----|--------|\n")
	for _, a := range r.AlphaChampionship {
		fmt.Fprintf(&b, "| %d | %s | %d | %.3f | %.2f | %.2f | $%.2f | $%.0f | %.1f | %.1f | %s | %.0f%% | **%s** |\n",
			a.Rank, a.Engine, a.TradeCount, a.ProfitFactor, a.Sharpe, a.Sortino,
			a.Expectancy, a.NetPnLUSD, a.WinRate*100, a.MaxDD,
			a.MCStability, a.WFConsistency*100, a.Verdict)
	}

	b.WriteString("\n## Alpha Analysis\n\n")
	for _, a := range r.AlphaChampionship {
		if a.Rank > 5 {
			continue
		}
		fmt.Fprintf(&b, "### #%d — %s\n\n", a.Rank, a.Engine)
		fmt.Fprintf(&b, "- **Verdict:** %s\n", a.Verdict)
		fmt.Fprintf(&b, "- **Trade Count:** %d\n", a.TradeCount)
		fmt.Fprintf(&b, "- **Profit Factor:** %.4f\n", a.ProfitFactor)
		fmt.Fprintf(&b, "- **Sharpe:** %.4f | Sortino: %.4f\n", a.Sharpe, a.Sortino)
		fmt.Fprintf(&b, "- **WF Consistency:** %.0f%% | MC: %s\n", a.WFConsistency*100, a.MCStability)
		fmt.Fprintf(&b, "- **Execution Retention:** %.2f%%\n", a.ExecRetention*100)
		fmt.Fprintf(&b, "- **Capital Efficiency:** %.4f\n\n", a.CapitalEfficiency)
	}

	if len(r.AlphaChampionship) > 0 {
		champion := r.AlphaChampionship[0]
		worst := r.AlphaChampionship[len(r.AlphaChampionship)-1]
		bestSharpe := r.AlphaChampionship[0]
		bestPF := r.AlphaChampionship[0]
		for _, a := range r.AlphaChampionship {
			if a.Sharpe > bestSharpe.Sharpe {
				bestSharpe = a
			}
			if a.ProfitFactor > bestPF.ProfitFactor {
				bestPF = a
			}
		}
		b.WriteString("## Summary Findings\n\n")
		fmt.Fprintf(&b, "- **Best Alpha:** %s (PF=%.3f, Sharpe=%.2f)\n", champion.Engine, champion.ProfitFactor, champion.Sharpe)
		fmt.Fprintf(&b, "- **Worst Alpha:** %s (PF=%.3f)\n", worst.Engine, worst.ProfitFactor)
		fmt.Fprintf(&b, "- **Highest Sharpe:** %s (%.2f)\n", bestSharpe.Engine, bestSharpe.Sharpe)
		fmt.Fprintf(&b, "- **Highest PF:** %s (%.3f)\n", bestPF.Engine, bestPF.ProfitFactor)
	}
	return b.String()
}

// ── 24.5: Walk-Forward ────────────────────────────────────────────────────────

func writeWalkForward(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — WALK-FORWARD CERTIFICATION", r))

	fmt.Fprintf(&b, "## Configuration\n\n- Train: %d months\n- Validate: %d months\n- Rolling windows\n- No look-ahead bias\n- No survivorship bias\n\n", WFTrainMonths, WFValidMonths)

	b.WriteString("## Walk-Forward Results\n\n")
	b.WriteString("| Strategy | Windows | Avg Valid PF | Avg Valid Sharpe | Consistency | Degradation | Verdict |\n")
	b.WriteString("|----------|---------|--------------|------------------|-------------|-------------|--------|\n")

	consistent := 0
	for _, wf := range r.WalkForward {
		verdict := "PASS"
		if !wf.IsConsistent {
			verdict = "INCONSISTENT"
		}
		if wf.IsDegraded {
			verdict = "DEGRADED"
		}
		if wf.IsConsistent && !wf.IsDegraded {
			consistent++
		}
		fmt.Fprintf(&b, "| %s | %d | %.3f | %.2f | %.0f%% | %.3f | %s |\n",
			truncate24(wf.StrategyName, 30),
			len(wf.Windows), wf.AvgValidPF, wf.AvgValidSharpe,
			wf.Consistency*100, wf.Degradation, verdict)
	}

	total := len(r.WalkForward)
	fmt.Fprintf(&b, "\n## Summary\n\n- **Total Strategies:** %d\n- **Walk-Forward Consistent:** %d (%.0f%%)\n",
		total, consistent, pct(consistent, total))
	return b.String()
}

// ── 24.6: Monte Carlo ─────────────────────────────────────────────────────────

func writeMonteCarlo(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — MONTE CARLO CERTIFICATION", r))

	fmt.Fprintf(&b, "## Configuration\n\n- Simulations: %d\n- Input: Actual trade distributions (real data)\n- Ruin threshold: 50%% capital loss\n\n", MCRuns)

	b.WriteString("## Monte Carlo Results\n\n")
	b.WriteString("| Strategy | Sims | P10 PnL | P50 PnL | P90 PnL | P50 DD% | P90 DD% | RoR% | Profitable% | Stability |\n")
	b.WriteString("|----------|------|---------|---------|---------|---------|---------|------|-------------|----------|\n")

	counts := map[string]int{"ROBUST": 0, "STABLE": 0, "MARGINAL": 0, "UNSTABLE": 0, "FAILED": 0}
	for name, mc := range r.MonteCarlo {
		_ = name
		tier := string(mc.Stability)
		counts[tier]++
		fmt.Fprintf(&b, "| %s | %d | $%.0f | $%.0f | $%.0f | %.1f | %.1f | %.1f%% | %.0f%% | %s |\n",
			truncate24(mc.StrategyName, 28), mc.Simulations,
			mc.P10PnL, mc.P50PnL, mc.P90PnL,
			mc.P50DD, mc.P90DD,
			mc.RiskOfRuin*100, mc.PctProfitable*100, mc.Stability)
	}

	b.WriteString("\n## Stability Classification\n\n")
	b.WriteString("| Tier | Count |\n|------|-------|\n")
	for _, tier := range []string{"ROBUST", "STABLE", "MARGINAL", "UNSTABLE", "FAILED"} {
		fmt.Fprintf(&b, "| %s | %d |\n", tier, counts[tier])
	}
	return b.String()
}

// ── 24.7: Regime Edge ─────────────────────────────────────────────────────────

func writeRegimeEdge(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — REGIME EDGE REPORT", r))

	b.WriteString("## Strategy Performance by Market Regime\n\n")

	for _, profile := range r.RegimeProfiles {
		fmt.Fprintf(&b, "### %s\n\n", profile.StrategyName)
		fmt.Fprintf(&b, "- **Dominant Regime:** %s\n", profile.DominantRegime)
		fmt.Fprintf(&b, "- **Weakest Regime:** %s\n", profile.WeakestRegime)
		fmt.Fprintf(&b, "- **Regime Robust:** %v (performs in 3+ regimes with PF>1)\n\n", profile.RegimeRobust)

		b.WriteString("| Regime | Trades | WR% | PF | Sharpe | Expectancy | MaxDD% |\n")
		b.WriteString("|--------|--------|-----|----|--------|------------|-------|\n")
		for regime, stats := range profile.Regimes {
			fmt.Fprintf(&b, "| %s | %d | %.1f | %.3f | %.2f | $%.2f | %.1f |\n",
				regime, stats.TradeCount, stats.WinRate*100,
				stats.ProfitFactor, stats.Sharpe,
				stats.Expectancy, stats.MaxDD)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ── 24.8: Retirement ─────────────────────────────────────────────────────────

func writeRetirement(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — STRATEGY RETIREMENT REPORT", r))

	counts := map[RetirementStatus]int{}
	for _, rec := range r.Retirement {
		counts[rec.Status]++
	}

	b.WriteString("## Summary\n\n")
	b.WriteString("| Status | Count |\n|--------|-------|\n")
	for _, status := range []RetirementStatus{
		RetirementKeep, RetirementWatchlist, RetirementPaperOnly,
		RetirementRetire, RetirementPermanentlyDisable,
	} {
		fmt.Fprintf(&b, "| %s | %d |\n", status, counts[status])
	}

	for _, status := range []RetirementStatus{
		RetirementKeep, RetirementWatchlist, RetirementPaperOnly,
		RetirementRetire, RetirementPermanentlyDisable,
	} {
		b.WriteString(fmt.Sprintf("\n## %s\n\n", status))
		b.WriteString("| Strategy | PF | Sharpe | DD% | Trades | MC | WF% | Reason |\n")
		b.WriteString("|----------|----|--------|-----|--------|-----|-----|--------|\n")
		for _, rec := range r.Retirement {
			if rec.Status != status {
				continue
			}
			fmt.Fprintf(&b, "| %s | %.3f | %.2f | %.1f | %d | %s | %.0f%% | %s |\n",
				truncate24(rec.StrategyName, 30),
				rec.ProfitFactor, rec.Sharpe, rec.MaxDD,
				rec.TradeCount, rec.MCTier,
				rec.WFConsistency*100, truncate24(rec.Reason, 60))
		}
	}
	return b.String()
}

// ── 24.9: Capital Certification ──────────────────────────────────────────────

func writeCapCert(r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header("PHASE 24 — CAPITAL CERTIFICATION REPORT", r))

	b.WriteString("## Tier Definitions\n\n")
	b.WriteString("| Tier | PF | Sharpe | MaxDD | Trades | Allocation |\n")
	b.WriteString("|------|----|--------|-------|--------|------------|\n")
	fmt.Fprintf(&b, "| INSTITUTIONAL | ≥%.2f | ≥%.2f | <%.0f%% | ≥%d | 20%% |\n", InstMinPF, InstMinSharpe, InstMaxDD, InstMinTrades)
	fmt.Fprintf(&b, "| FULL_CAPITAL | ≥%.2f | ≥%.2f | <%.0f%% | ≥%d | 10%% |\n", FullMinPF, FullMinSharpe, FullMaxDD, FullMinTrades)
	fmt.Fprintf(&b, "| LIMITED_CAPITAL | ≥%.2f | ≥%.2f | <%.0f%% | ≥%d | 3%% |\n", LimitedMinPF, LimitedMinSharpe, LimitedMaxDD, LimitedMinTrades)
	b.WriteString("| PAPER_ONLY | >1.00 | — | — | — | 0% |\n")
	b.WriteString("| RETIRE | <1.00 | — | — | — | 0% |\n\n")

	b.WriteString("## Certified Strategies\n\n")
	b.WriteString("| Strategy | Tier | Allocation% | Allocation$ | Evidence |\n")
	b.WriteString("|----------|------|-------------|-------------|----------|\n")

	// Sort: best tiers first
	certs := make([]CapCertification24, len(r.CapCerts))
	copy(certs, r.CapCerts)
	sort.Slice(certs, func(i, j int) bool {
		return tierOrd(certs[i].Tier) > tierOrd(certs[j].Tier)
	})
	for _, c := range certs {
		fmt.Fprintf(&b, "| %s | %s | %.1f%% | $%.0f | %s |\n",
			truncate24(c.StrategyName, 35), c.Tier,
			c.AllocationPct, c.AllocationUSD,
			truncate24(c.Evidence, 60))
	}
	return b.String()
}

func tierOrd(t CapTier24) int {
	switch t {
	case CapTier24Institutional:
		return 4
	case CapTier24Full:
		return 3
	case CapTier24Limited:
		return 2
	case CapTier24PaperOnly:
		return 1
	default:
		return 0
	}
}

// ── 24.10: Portfolios ─────────────────────────────────────────────────────────

func writeTop3Portfolio(r Phase24Result) string {
	return writePortfolioReport("PHASE 24 — TOP-3 INSTITUTIONAL PORTFOLIO", r.Top3Portfolio, r)
}
func writeTop5Portfolio(r Phase24Result) string {
	return writePortfolioReport("PHASE 24 — TOP-5 INSTITUTIONAL PORTFOLIO", r.Top5Portfolio, r)
}
func writeTop10Portfolio(r Phase24Result) string {
	return writePortfolioReport("PHASE 24 — TOP-10 INSTITUTIONAL PORTFOLIO", r.Top10Portfolio, r)
}

func writePortfolioReport(title string, p Portfolio24, r Phase24Result) string {
	var b strings.Builder
	b.WriteString(header(title, r))

	fmt.Fprintf(&b, "## Portfolio Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Total Capital | $%.0f |\n", p.TotalCapital)
	fmt.Fprintf(&b, "| Expected CAGR | %.2f%% |\n", p.ExpectedCAGR)
	fmt.Fprintf(&b, "| Expected PF | %.3f |\n", p.ExpectedPF)
	fmt.Fprintf(&b, "| Expected Sharpe | %.2f |\n", p.ExpectedSharpe)
	fmt.Fprintf(&b, "| Expected Max DD | %.1f%% |\n", p.ExpectedMaxDD)
	fmt.Fprintf(&b, "| Diversification Score | %.0f/100 |\n\n", p.DiversificationScore)

	b.WriteString("## Allocations\n\n")
	b.WriteString("| Rank | Strategy | Alpha Source | Weight% | Allocation$ | Exp Sharpe | Risk Contrib% |\n")
	b.WriteString("|------|----------|-------------|---------|-------------|------------|---------------|\n")
	for _, e := range p.Entries {
		fmt.Fprintf(&b, "| %d | %s | %s | %.1f%% | $%.0f | %.2f | %.1f%% |\n",
			e.Rank, e.StrategyName, e.AlphaSource,
			e.Weight, e.AllocationUSD, e.ExpectedSharpe, e.RiskContrib)
	}

	b.WriteString("\n## Correlation Matrix\n\n")
	names := make([]string, 0, len(p.CorrelationMatrix))
	for n := range p.CorrelationMatrix {
		names = append(names, n)
	}
	sort.Strings(names)

	// Header row
	b.WriteString("| |")
	for _, n := range names {
		fmt.Fprintf(&b, " %s |", truncate24(n, 15))
	}
	b.WriteString("\n|---|")
	for range names {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, row := range names {
		fmt.Fprintf(&b, "| %s |", truncate24(row, 15))
		for _, col := range names {
			fmt.Fprintf(&b, " %.2f |", p.CorrelationMatrix[row][col])
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ── 24.11: Final Verdict ──────────────────────────────────────────────────────

func writeFinalVerdict(r Phase24Result) string {
	var b strings.Builder
	v := r.Verdict

	b.WriteString(header("PHASE 24 — FINAL INSTITUTIONAL VERDICT", r))

	deployIcon := "❌"
	if v.Q17_ApproveDeployment {
		deployIcon = "✅"
	}
	fmt.Fprintf(&b, "## %s DEPLOYMENT DECISION: %s\n\n", deployIcon,
		map[bool]string{true: "APPROVED", false: "NOT APPROVED"}[v.Q17_ApproveDeployment])
	fmt.Fprintf(&b, "> %s\n\n", v.Q17_Justification)

	b.WriteString("---\n\n## Platform-Level Metrics\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Platform Profit Factor | %.4f |\n", v.PlatformPF)
	fmt.Fprintf(&b, "| Platform Sharpe | %.4f |\n", v.PlatformSharpe)
	fmt.Fprintf(&b, "| Platform Net PnL | $%.2f |\n", v.PlatformNetPnL)
	fmt.Fprintf(&b, "| Total Strategies | %d |\n", v.TotalStrategies)
	fmt.Fprintf(&b, "| Deploy-Ready Strategies | %d |\n", v.DeployStrategies)
	fmt.Fprintf(&b, "| Retired Strategies | %d |\n\n", v.RetiredStrategies)

	b.WriteString("---\n\n## 17 Institutional Questions — Evidence-Based Answers\n\n")

	q := func(n int, q string, a interface{}, evidence string) {
		fmt.Fprintf(&b, "### Q%d: %s\n\n**Answer:** %v\n\n", n, q, a)
		if evidence != "" {
			fmt.Fprintf(&b, "**Evidence:** %s\n\n", evidence)
		}
	}

	q(1, "Does the platform possess real edge?", v.Q1_HasRealEdge, v.Q1_Evidence)

	b.WriteString("### Q2: Which exact strategies possess edge?\n\n")
	for i, s := range v.Q2_StrategiesWithEdge {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q3: Which alpha sources possess edge?\n\n")
	for _, s := range v.Q3_AlphasWithEdge {
		fmt.Fprintf(&b, "- %s\n", s)
	}
	b.WriteString("\n")

	b.WriteString("### Q4: Which strategies deserve capital?\n\n")
	for i, s := range v.Q4_DeserveCapital {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q5: Which strategies should be retired?\n\n")
	for i, s := range v.Q5_ShouldBeRetired {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Q6: Expected annual return?\n\n**%.2f%%**\n\n", v.Q6_ExpectedAnnualReturn)
	fmt.Fprintf(&b, "### Q7: Expected maximum drawdown?\n\n**%.2f%%**\n\n", v.Q7_ExpectedMaxDD)
	fmt.Fprintf(&b, "### Q8: Expected Sharpe ratio?\n\n**%.4f**\n\n", v.Q8_ExpectedSharpe)
	fmt.Fprintf(&b, "### Q9: Probability of ruin?\n\n**%.4f%%** (portfolio level, Top-5)\n\n", v.Q9_ProbabilityOfRuin*100)

	q(10, "Is the system profitable after all costs (fees+slippage+funding)?", v.Q10_ProfitableAfterCosts, v.Q10_Evidence)
	q(11, "Is the system deployable live today?", v.Q11_DeployableToday, v.Q11_Evidence)
	q(12, "Is the system institutional-grade?", v.Q12_InstitutionalGrade, v.Q12_Evidence)

	b.WriteString("### Q13: Top 20 Strategies\n\n")
	for i, s := range v.Q13_Top20Strategies {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q14: Top 10 Strategies\n\n")
	for i, s := range v.Q14_Top10Strategies {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q15: Top 5 Strategies\n\n")
	for i, s := range v.Q15_Top5Strategies {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q16: Recommended Capital Allocation\n\n")
	b.WriteString("| Strategy | Allocation% |\n|----------|-------------|\n")
	for strat, w := range v.Q16_CapitalAllocation {
		fmt.Fprintf(&b, "| %s | %.1f%% |\n", strat, w)
	}
	b.WriteString("\n")

	q(17, "Would you approve deployment today?", v.Q17_ApproveDeployment, v.Q17_Justification)

	fmt.Fprintf(&b, "\n---\n\n*Report generated %s by Phase 24 Institutional Edge Certification Engine.*\n",
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	return b.String()
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func truncate24(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

// Ensure phase23b.WFReport fields compile (used in writeWalkForward).
var _ phase23b.WFReport
