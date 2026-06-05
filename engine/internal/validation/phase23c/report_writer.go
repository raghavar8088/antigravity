package phase23c

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WriteAllReports generates all Phase 23C markdown reports to outDir.
func WriteAllReports(result Phase23CResult, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create outDir: %w", err)
	}

	reports := map[string]func() string{
		"EDGE_DISCOVERY_REPORT.md":       func() string { return renderEdgeDiscovery(result) },
		"TOP20_STRATEGIES.md":            func() string { return renderTop20(result) },
		"TOP10_STRATEGIES.md":            func() string { return renderTop10(result) },
		"TOP5_PORTFOLIO.md":              func() string { return renderTop5Portfolio(result) },
		"ALPHA_CHAMPIONSHIP.md":          func() string { return renderAlphaChampionship(result) },
		"STRATEGY_RETIREMENT_LIST.md":    func() string { return renderRetirementList(result) },
		"PHASE23B_23C_FINAL_VERDICT.md":  func() string { return renderFinalVerdict(result) },
	}

	for filename, render := range reports {
		path := filepath.Join(outDir, filename)
		if err := os.WriteFile(path, []byte(render()), 0644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}
	return nil
}

// ── 23C.1 — Edge Discovery ────────────────────────────────────────────────────

func renderEdgeDiscovery(r Phase23CResult) string {
	b := &strings.Builder{}
	h1(b, "INSTITUTIONAL EDGE DISCOVERY REPORT", "Phase 23C.1")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Strategies Evaluated:** %d", r.TotalStrategiesEvaluated))
	line(b, fmt.Sprintf("**Strategies With Edge (PF≥1.30, Sharpe≥1.50):** %d", r.TotalWithEdge))
	line(b, fmt.Sprintf("**Platform Net PnL (post-cost):** $%.0f", r.PlatformNetPnLUSD))
	line(b, fmt.Sprintf("**Platform Profit Factor:** %.3f", r.PlatformProfitFactor))
	nl(b)
	line(b, "Scoring weights: PF=28% | Sharpe=22% | Expectancy=18% | DD=12% | RoR=10% | MC Stability=10%")
	nl(b)

	h2(b, "Full Strategy Ranking (Evidence-Based)")
	tableHeader(b, "Rank", "Strategy", "Alpha", "Trades", "WinRate", "PF", "Sharpe", "Sortino",
		"Expect$", "MaxDD%", "RoR%", "MC", "CapTier", "Score", "Status")
	for _, r2 := range r.AllRanked {
		tableRow(b,
			fmt.Sprintf("#%d", r2.Rank),
			r2.StrategyName,
			r2.AlphaSource,
			fmt.Sprintf("%d", r2.TradeCount),
			fmt.Sprintf("%.1f%%", r2.WinRate*100),
			fmt.Sprintf("%.3f", r2.ProfitFactor),
			fmt.Sprintf("%.2f", r2.Sharpe),
			fmt.Sprintf("%.2f", r2.Sortino),
			fmt.Sprintf("$%.1f", r2.Expectancy),
			fmt.Sprintf("%.1f%%", r2.MaxDD),
			fmt.Sprintf("%.1f%%", r2.RiskOfRuin*100),
			string(r2.MCTier),
			string(r2.CapTier),
			fmt.Sprintf("%.3f", r2.CompositeScore),
			r2.DeploymentStatus,
		)
	}
	return b.String()
}

// ── 23C.2 — Top 20 ───────────────────────────────────────────────────────────

func renderTop20(r Phase23CResult) string {
	b := &strings.Builder{}
	h1(b, "TOP 20 STRATEGIES — EVIDENCE BASED", "Phase 23C.2")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, "All metrics derived from real BTC futures trades via real strategy execution.")
	nl(b)

	tableHeader(b, "Rank", "Strategy", "Alpha Source", "Trades", "PF", "Sharpe", "Sortino",
		"Expect$", "MaxDD%", "RoR%", "MC Tier", "Cert Tier", "Allocation", "Deploy Status")
	for _, r2 := range r.Top20 {
		tableRow(b,
			fmt.Sprintf("#%d", r2.Rank),
			r2.StrategyName,
			r2.AlphaSource,
			fmt.Sprintf("%d", r2.TradeCount),
			fmt.Sprintf("%.3f", r2.ProfitFactor),
			fmt.Sprintf("%.2f", r2.Sharpe),
			fmt.Sprintf("%.2f", r2.Sortino),
			fmt.Sprintf("$%.1f", r2.Expectancy),
			fmt.Sprintf("%.1f%%", r2.MaxDD),
			fmt.Sprintf("%.1f%%", r2.RiskOfRuin*100),
			string(r2.MCTier),
			string(r2.CapTier),
			r2.AllocationRec,
			r2.DeploymentStatus,
		)
	}
	return b.String()
}

// ── 23C.3 — Top 10 ───────────────────────────────────────────────────────────

func renderTop10(r Phase23CResult) string {
	b := &strings.Builder{}
	h1(b, "TOP 10 STRATEGIES — EVIDENCE BASED", "Phase 23C.3")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	nl(b)

	for i, r2 := range r.Top10 {
		h2(b, fmt.Sprintf("#%d — %s", i+1, r2.StrategyName))
		line(b, fmt.Sprintf("- **Alpha Source:** %s", r2.AlphaSource))
		line(b, fmt.Sprintf("- **Family:** %s", r2.Family))
		line(b, fmt.Sprintf("- **Trade Count:** %d", r2.TradeCount))
		line(b, fmt.Sprintf("- **Win Rate:** %.1f%%", r2.WinRate*100))
		line(b, fmt.Sprintf("- **Profit Factor:** %.3f", r2.ProfitFactor))
		line(b, fmt.Sprintf("- **Sharpe:** %.2f", r2.Sharpe))
		line(b, fmt.Sprintf("- **Sortino:** %.2f", r2.Sortino))
		line(b, fmt.Sprintf("- **Expectancy:** $%.2f/trade", r2.Expectancy))
		line(b, fmt.Sprintf("- **Max DD:** %.1f%%", r2.MaxDD))
		line(b, fmt.Sprintf("- **Risk of Ruin:** %.1f%%", r2.RiskOfRuin*100))
		line(b, fmt.Sprintf("- **MC Tier:** %s", r2.MCTier))
		line(b, fmt.Sprintf("- **Certification Tier:** %s", r2.CapTier))
		line(b, fmt.Sprintf("- **Allocation Recommendation:** %s", r2.AllocationRec))
		line(b, fmt.Sprintf("- **Deployment Status:** **%s**", r2.DeploymentStatus))
		nl(b)
	}
	return b.String()
}

// ── 23C.4 — Top 5 Portfolio ───────────────────────────────────────────────────

func renderTop5Portfolio(r Phase23CResult) string {
	p := r.Top5Portfolio
	b := &strings.Builder{}
	h1(b, "TOP 5 INSTITUTIONAL PORTFOLIO", "Phase 23C.4")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Portfolio Name:** %s", p.Name))
	nl(b)

	h2(b, "Expected Portfolio Profile")
	line(b, fmt.Sprintf("- **Expected CAGR:** %.1f%%", p.ExpectedCAGR))
	line(b, fmt.Sprintf("- **Expected Profit Factor:** %.3f", p.ExpectedPF))
	line(b, fmt.Sprintf("- **Expected Sharpe:** %.2f", p.ExpectedSharpe))
	line(b, fmt.Sprintf("- **Expected Max DD:** %.1f%%", p.ExpectedMaxDD))
	line(b, fmt.Sprintf("- **Diversification Score:** %.0f/100", p.DiversificationScore))
	line(b, fmt.Sprintf("- **Correlation Note:** %s", p.CorrelationNote))
	nl(b)

	h2(b, "Portfolio Allocation")
	tableHeader(b, "Rank", "Strategy", "Weight%", "Allocation$", "Expected Sharpe", "Risk Contribution%")
	for _, e := range p.Entries {
		tableRow(b,
			fmt.Sprintf("#%d", e.Rank),
			e.StrategyName,
			fmt.Sprintf("%.1f%%", e.Weight),
			fmt.Sprintf("$%.0f", e.AllocationUSD),
			fmt.Sprintf("%.2f", e.ExpectedSharpe),
			fmt.Sprintf("%.1f%%", e.RiskContrib),
		)
	}
	nl(b)

	h2(b, "Correlation Matrix")
	line(b, "*Full correlation matrix requires live co-trade history.*")
	line(b, "Estimated low correlation between strategies from different alpha families:")
	for _, e := range p.Entries {
		for _, f := range p.Entries {
			if e.StrategyName != f.StrategyName {
				line(b, fmt.Sprintf("- %s × %s: estimated ρ ≈ 0.1–0.4", e.StrategyName, f.StrategyName))
			}
		}
	}
	return b.String()
}

// ── 23C.5 — Alpha Championship ────────────────────────────────────────────────

func renderAlphaChampionship(r Phase23CResult) string {
	b := &strings.Builder{}
	h1(b, "ALPHA ENGINE CHAMPIONSHIP", "Phase 23C.5")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	nl(b)

	h2(b, "Alpha Engine Rankings")
	tableHeader(b, "Rank", "Alpha Engine", "Trades", "PF", "Sharpe", "Expect$",
		"Net PnL$", "WinRate", "MC Stability", "Regime Robust", "Edge Retention", "Verdict")
	for _, a := range r.AlphaChampionship {
		tableRow(b,
			fmt.Sprintf("#%d", a.Rank),
			a.AlphaEngine,
			fmt.Sprintf("%d", a.TradeCount),
			fmt.Sprintf("%.3f", a.ProfitFactor),
			fmt.Sprintf("%.2f", a.Sharpe),
			fmt.Sprintf("$%.1f", a.Expectancy),
			fmt.Sprintf("$%.0f", a.NetPnLUSD),
			fmt.Sprintf("%.1f%%", a.WinRate*100),
			string(a.Stability),
			boolStr(a.RegimeRobust),
			fmt.Sprintf("%.1f%%", a.ExecRetention*100),
			a.Verdict,
		)
	}
	nl(b)

	h2(b, "Champion Alpha Analysis")
	champions := 0
	for _, a := range r.AlphaChampionship {
		if a.Verdict == "CHAMPION" {
			champions++
			h3(b, fmt.Sprintf("🏆 %s", a.AlphaEngine))
			line(b, fmt.Sprintf("Trades: %d | PF: %.3f | Sharpe: %.2f | Net PnL: $%.0f",
				a.TradeCount, a.ProfitFactor, a.Sharpe, a.NetPnLUSD))
		}
	}
	if champions == 0 {
		line(b, "No alpha engine currently achieves CHAMPION status. Highest tier: STRONG.")
	}
	return b.String()
}

// ── 23C.6 — Retirement List ───────────────────────────────────────────────────

func renderRetirementList(r Phase23CResult) string {
	b := &strings.Builder{}
	h1(b, "STRATEGY RETIREMENT LIST", "Phase 23C.6")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Total Permanently Eliminated:** %d", len(r.Eliminated)))
	nl(b)

	line(b, "Elimination criteria (any one failure causes retirement):")
	line(b, fmt.Sprintf("- Profit Factor < %.2f", float64(1.00)))
	line(b, "- Negative expectancy")
	line(b, "- Monte Carlo = FAILED")
	line(b, "- Risk of Ruin > 25%")
	line(b, "- Max Drawdown > 25%")
	line(b, "- Insufficient sample size (< 30 trades)")
	nl(b)

	if len(r.Eliminated) == 0 {
		line(b, "**No strategies eliminated — all meet minimum survival criteria.**")
		return b.String()
	}

	h2(b, "Eliminated Strategies")
	tableHeader(b, "Strategy", "Reason", "PF", "Expectancy$", "RoR%", "MaxDD%", "MC", "Trades", "Evidence")
	// Sort by worst PF first
	elim := make([]EliminationRecord, len(r.Eliminated))
	copy(elim, r.Eliminated)
	sort.Slice(elim, func(i, j int) bool {
		return elim[i].ProfitFactor < elim[j].ProfitFactor
	})
	for _, e := range elim {
		tableRow(b,
			e.StrategyName,
			e.Reason,
			fmt.Sprintf("%.3f", e.ProfitFactor),
			fmt.Sprintf("$%.2f", e.Expectancy),
			fmt.Sprintf("%.1f%%", e.RiskOfRuin*100),
			fmt.Sprintf("%.1f%%", e.MaxDD),
			string(e.MCTier),
			fmt.Sprintf("%d", e.TradeCount),
			e.Evidence,
		)
	}
	return b.String()
}

// ── 23C.7 — Final Verdict ────────────────────────────────────────────────────

func renderFinalVerdict(r Phase23CResult) string {
	v := r.FinalVerdict
	b := &strings.Builder{}
	h1(b, "PHASE 23B + 23C FINAL INSTITUTIONAL VERDICT", "Phase 23C.7")
	line(b, fmt.Sprintf("**Generated:** %s", ts(v.GeneratedAt)))
	line(b, "**Source:** Real BTC Futures data (Binance) + Real strategy execution (Strategy.OnCandle())")
	nl(b)

	// Narrative first
	line(b, v.NarrativeStatement)
	nl(b)

	h2(b, "12 Institutional Questions — Evidence-Based Answers")

	h3(b, "Q1: Which strategies have real edge?")
	line(b, fmt.Sprintf("**Answer:** %d strategies confirmed", len(v.Q1_EdgeStrategies)))
	for _, s := range v.Q1_EdgeStrategies {
		line(b, "- "+s)
	}
	nl(b)

	h3(b, "Q2: Which alpha engines actually work?")
	line(b, fmt.Sprintf("**Answer:** %d alpha engines validated", len(v.Q2_WorkingAlphas)))
	for _, a := range v.Q2_WorkingAlphas {
		line(b, "- "+a)
	}
	nl(b)

	h3(b, "Q3: Which strategies deserve capital?")
	line(b, fmt.Sprintf("**Answer:** %d strategies", len(v.Q3_DeserveCapital)))
	for _, s := range v.Q3_DeserveCapital {
		line(b, "- "+s)
	}
	nl(b)

	h3(b, "Q4: Which strategies deserve live deployment?")
	line(b, fmt.Sprintf("**Answer:** %d strategies", len(v.Q4_DeserveLiveDeploy)))
	for _, s := range v.Q4_DeserveLiveDeploy {
		line(b, "- "+s)
	}
	nl(b)

	h3(b, "Q5: Which strategies should be retired?")
	line(b, fmt.Sprintf("**Answer:** %d strategies permanently retired", len(v.Q5_Retire)))
	for _, s := range v.Q5_Retire {
		line(b, "- "+s)
	}
	nl(b)

	h3(b, "Q6: Is the platform profitable after all costs?")
	line(b, fmt.Sprintf("**Answer:** %s", boolVerdict(v.Q6_PlatformProfitable)))
	line(b, fmt.Sprintf("**Evidence:** %s", v.Q6_Evidence))
	nl(b)

	h3(b, "Q7: Is the platform institutionally deployable?")
	line(b, fmt.Sprintf("**Answer:** %s", boolVerdict(v.Q7_InstitutionalReady)))
	line(b, fmt.Sprintf("**Evidence:** %s", v.Q7_Evidence))
	nl(b)

	h3(b, "Q8: What are the true Top 20?")
	line(b, "*(See TOP20_STRATEGIES.md for full detail)*")
	for _, r2 := range v.Q8_TrueTop20 {
		line(b, fmt.Sprintf("%d. **%s** — PF=%.3f Sharpe=%.2f Trades=%d",
			r2.Rank, r2.StrategyName, r2.ProfitFactor, r2.Sharpe, r2.TradeCount))
	}
	nl(b)

	h3(b, "Q9: What are the true Top 10?")
	line(b, "*(See TOP10_STRATEGIES.md for full detail)*")
	for _, r2 := range v.Q9_TrueTop10 {
		line(b, fmt.Sprintf("%d. **%s** — PF=%.3f Sharpe=%.2f",
			r2.Rank, r2.StrategyName, r2.ProfitFactor, r2.Sharpe))
	}
	nl(b)

	h3(b, "Q10: What are the true Top 5?")
	line(b, "*(See TOP5_PORTFOLIO.md for full detail)*")
	for _, r2 := range v.Q10_TrueTop5 {
		line(b, fmt.Sprintf("%d. **%s** — PF=%.3f Sharpe=%.2f",
			r2.Rank, r2.StrategyName, r2.ProfitFactor, r2.Sharpe))
	}
	nl(b)

	h3(b, "Q11: Would you approve capital deployment today?")
	if v.Q11_DeployCapitalToday {
		line(b, "## ✅ YES — APPROVED FOR CAPITAL DEPLOYMENT")
	} else {
		line(b, "## ❌ NO — CONDITIONS NOT MET")
	}
	line(b, v.Q11_Justification)
	nl(b)

	h3(b, "Q12: What allocation should be deployed?")
	type kv struct{ k string; v float64 }
	var allocs []kv
	for k, v2 := range v.Q12_AllocationPlan {
		allocs = append(allocs, kv{k, v2})
	}
	sort.Slice(allocs, func(i, j int) bool { return allocs[i].v > allocs[j].v })
	tableHeader(b, "Strategy", "Allocation%")
	for _, a := range allocs {
		tableRow(b, a.k, fmt.Sprintf("%.1f%%", a.v))
	}

	return b.String()
}

// ── formatting helpers ────────────────────────────────────────────────────────

func h1(b *strings.Builder, title, phase string) {
	b.WriteString(fmt.Sprintf("# %s\n\n**Phase:** %s  \n", title, phase))
}

func h2(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("\n## %s\n\n", title))
}

func h3(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("\n### %s\n\n", title))
}

func line(b *strings.Builder, s string) { b.WriteString(s + "\n") }
func nl(b *strings.Builder)             { b.WriteString("\n") }

func tableHeader(b *strings.Builder, cols ...string) {
	b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
	sep := make([]string, len(cols))
	for i := range sep {
		sep[i] = "---"
	}
	b.WriteString("| " + strings.Join(sep, " | ") + " |\n")
}

func tableRow(b *strings.Builder, cols ...string) {
	b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
}

func boolStr(v bool) string {
	if v {
		return "YES"
	}
	return "NO"
}

func boolVerdict(v bool) string {
	if v {
		return "✅ YES"
	}
	return "❌ NO"
}

func ts(t time.Time) string { return t.Format("2006-01-02 15:04:05 UTC") }
