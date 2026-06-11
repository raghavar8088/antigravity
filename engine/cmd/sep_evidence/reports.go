package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ── CSV ───────────────────────────────────────────────────────────────────────

func writeCSV(path string, metrics []StrategyMetrics) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{
		"strategy_id", "category", "tier", "status",
		"total_trades", "wins", "losses", "win_rate", "loss_rate",
		"gross_profit", "gross_loss", "net_pnl", "profit_factor",
		"avg_win", "avg_loss", "expectancy",
		"sharpe_ratio", "sortino_ratio", "max_drawdown",
		"avg_duration_ms", "median_duration_ms",
		"avg_slippage_bps", "total_fees", "fee_impact",
	})

	for _, m := range metrics {
		_ = w.Write([]string{
			m.StrategyID, m.Category, m.Tier, m.Status,
			fmt.Sprintf("%d", m.TotalTrades),
			fmt.Sprintf("%d", m.Wins),
			fmt.Sprintf("%d", m.Losses),
			fmt.Sprintf("%.4f", m.WinRate),
			fmt.Sprintf("%.4f", m.LossRate),
			fmt.Sprintf("%.2f", m.GrossProfit),
			fmt.Sprintf("%.2f", m.GrossLoss),
			fmt.Sprintf("%.2f", m.NetPnL),
			fmt.Sprintf("%.4f", m.ProfitFactor),
			fmt.Sprintf("%.2f", m.AvgWin),
			fmt.Sprintf("%.2f", m.AvgLoss),
			fmt.Sprintf("%.4f", m.Expectancy),
			fmt.Sprintf("%.4f", m.SharpeRatio),
			fmt.Sprintf("%.4f", m.SortinoRatio),
			fmt.Sprintf("%.4f", m.MaxDrawdown),
			fmt.Sprintf("%d", m.AvgDurationMs),
			fmt.Sprintf("%d", m.MedianDurationMs),
			fmt.Sprintf("%.2f", m.AvgSlippageBPS),
			fmt.Sprintf("%.2f", m.TotalFees),
			fmt.Sprintf("%.4f", m.FeeImpact),
		})
	}
	return nil
}

func writeJSON(path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// ── Phase 1 — Evidence Report ─────────────────────────────────────────────────

func writeEvidenceReport(dir string, metrics []StrategyMetrics, totalTrades int, dateRange [2]time.Time) error {
	path := filepath.Join(dir, "STRATEGY_EVIDENCE_REPORT.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	passed := 0
	insufficient := 0
	failed := 0
	var totalNetPnL, totalFees float64
	for _, m := range metrics {
		switch m.Status {
		case "PASS":
			passed++
		case "INSUFFICIENT_DATA":
			insufficient++
		case "FAIL":
			failed++
		}
		totalNetPnL += m.NetPnL
		totalFees += m.TotalFees
	}

	fmt.Fprintf(f, "# STRATEGY EVIDENCE REPORT\n")
	fmt.Fprintf(f, "## SEP Phase 1 — Forensic Trade Evidence\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Evidence Base:** Live MongoDB paper_trades + SQLite trades\n\n")
	fmt.Fprintf(f, "---\n\n")
	fmt.Fprintf(f, "## SUMMARY\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(f, "| Total trades analysed | %d |\n", totalTrades)
	fmt.Fprintf(f, "| Unique strategies | %d |\n", len(metrics))
	fmt.Fprintf(f, "| Strategies with evidence (≥%d trades) | %d |\n", minTradesForEvidence, passed+failed)
	fmt.Fprintf(f, "| Strategies: PASS | %d |\n", passed)
	fmt.Fprintf(f, "| Strategies: FAIL | %d |\n", failed)
	fmt.Fprintf(f, "| Strategies: INSUFFICIENT_DATA | %d |\n", insufficient)
	fmt.Fprintf(f, "| Date range | %s → %s |\n", dateRange[0].Format("2006-01-02"), dateRange[1].Format("2006-01-02"))
	fmt.Fprintf(f, "| Portfolio net PnL | $%.2f |\n", totalNetPnL)
	fmt.Fprintf(f, "| Total fees paid | $%.2f |\n", totalFees)
	fmt.Fprintf(f, "\n---\n\n")

	fmt.Fprintf(f, "## STRATEGY EVIDENCE TABLE\n\n")
	fmt.Fprintf(f, "| Strategy | Trades | Win%% | PF | Expectancy | Sharpe | MaxDD | Tier | Status |\n")
	fmt.Fprintf(f, "|----------|--------|------|----|-----------|--------|-------|------|--------|\n")

	for _, m := range metrics {
		pf := "—"
		if m.ProfitFactor > 0 {
			pf = fmt.Sprintf("%.2f", m.ProfitFactor)
		}
		exp := "—"
		if m.TotalTrades >= minTradesForEvidence {
			exp = fmt.Sprintf("$%.2f", m.Expectancy)
		}
		sharpe := "—"
		if m.TotalTrades >= minTradesForEvidence {
			sharpe = fmt.Sprintf("%.2f", m.SharpeRatio)
		}
		maxdd := "—"
		if m.TotalTrades >= minTradesForEvidence {
			maxdd = fmt.Sprintf("%.1f%%", m.MaxDrawdown*100)
		}
		fmt.Fprintf(f, "| %-50s | %5d | %5.1f%% | %s | %s | %s | %s | %s | %s |\n",
			truncate(m.StrategyID, 50),
			m.TotalTrades,
			m.WinRate*100,
			pf, exp, sharpe, maxdd,
			m.Tier, m.Status,
		)
	}

	fmt.Fprintf(f, "\n---\n\n")
	fmt.Fprintf(f, "## DATA QUALITY\n\n")
	fmt.Fprintf(f, "- Source: MongoDB Atlas `paper_trades` (authoritative) + SQLite `trades` (supplementary)\n")
	fmt.Fprintf(f, "- Minimum trades for evidence: %d\n", minTradesForEvidence)
	fmt.Fprintf(f, "- Fee model: Entry + Exit combined, from `total_fee` field\n")
	fmt.Fprintf(f, "- Slippage: From `slippage_bps` field (0 if not recorded)\n")
	fmt.Fprintf(f, "- Annualisation factor for Sharpe/Sortino: √(12 × 252) assuming ~12 trades/day\n\n")

	return nil
}

// ── Phase 2 — Expectancy Leaderboard ─────────────────────────────────────────

func writeLeaderboard(dir string, metrics []StrategyMetrics) error {
	// Sort by expectancy descending
	sorted := make([]StrategyMetrics, len(metrics))
	copy(sorted, metrics)
	sort.Slice(sorted, func(i, j int) bool {
		ei, ej := sorted[i].Expectancy, sorted[j].Expectancy
		if sorted[i].Status == "INSUFFICIENT_DATA" {
			ei = -9999
		}
		if sorted[j].Status == "INSUFFICIENT_DATA" {
			ej = -9999
		}
		return ei > ej
	})

	path := filepath.Join(dir, "EXPECTANCY_LEADERBOARD.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tiers := map[string][]StrategyMetrics{
		"A": {}, "B": {}, "C": {}, "D": {}, "F": {},
	}
	for _, m := range sorted {
		tiers[m.Tier] = append(tiers[m.Tier], m)
	}

	fmt.Fprintf(f, "# EXPECTANCY LEADERBOARD\n")
	fmt.Fprintf(f, "## SEP Phase 2 — Institutional Strategy Ranking\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Minimum trades for ranking:** %d\n\n", minTradesForEvidence)

	for _, tier := range []string{"A", "B", "C", "D", "F"} {
		desc := map[string]string{
			"A": "INSTITUTIONAL GRADE — PF ≥ 1.50, Sharpe ≥ 1.0",
			"B": "SELECTIVE EDGE — PF ≥ 1.25, positive expectancy",
			"C": "MARGINAL EDGE — PF ≥ 1.10, positive expectancy",
			"D": "WEAK EDGE — PF ≥ 1.0, positive but fragile",
			"F": "FAIL / INSUFFICIENT DATA",
		}
		fmt.Fprintf(f, "---\n\n### TIER %s — %s\n\n", tier, desc[tier])
		if len(tiers[tier]) == 0 {
			fmt.Fprintf(f, "_No strategies in this tier._\n\n")
			continue
		}
		fmt.Fprintf(f, "| Rank | Strategy | Trades | Win%% | PF | Expectancy | Sharpe | MaxDD |\n")
		fmt.Fprintf(f, "|------|----------|--------|------|----|-----------|--------|-------|\n")
		for i, m := range tiers[tier] {
			expStr := "INSUFFICIENT_DATA"
			sharpeStr := "—"
			pfStr := "—"
			ddStr := "—"
			if m.Status != "INSUFFICIENT_DATA" {
				expStr = fmt.Sprintf("$%.4f", m.Expectancy)
				sharpeStr = fmt.Sprintf("%.2f", m.SharpeRatio)
				pfStr = fmt.Sprintf("%.2f", m.ProfitFactor)
				ddStr = fmt.Sprintf("%.1f%%", m.MaxDrawdown*100)
			} else if m.TotalTrades > 0 {
				expStr = fmt.Sprintf("INSUFFICIENT_DATA (%d trades)", m.TotalTrades)
			}
			fmt.Fprintf(f, "| %d | %s | %d | %.1f%% | %s | %s | %s | %s |\n",
				i+1, truncate(m.StrategyID, 45), m.TotalTrades, m.WinRate*100,
				pfStr, expStr, sharpeStr, ddStr,
			)
		}
		fmt.Fprintf(f, "\n")
	}

	return nil
}

// ── Phase 3 — Loser Retirement Report ────────────────────────────────────────

func writeLoserReport(dir string, metrics []StrategyMetrics) ([]string, error) {
	var losers []StrategyMetrics
	for _, m := range metrics {
		if m.Status == "FAIL" {
			losers = append(losers, m)
		}
	}

	path := filepath.Join(dir, "LOSER_RETIREMENT_REPORT.md")
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fmt.Fprintf(f, "# LOSER RETIREMENT REPORT\n")
	fmt.Fprintf(f, "## SEP Phase 3 — Strategy Termination\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Retirement criteria:**\n")
	fmt.Fprintf(f, "- Profit Factor < 1.0 (losses exceed profits)\n")
	fmt.Fprintf(f, "- Negative expectancy (average trade loses money)\n")
	fmt.Fprintf(f, "- Negative net PnL\n")
	fmt.Fprintf(f, "- Minimum %d trades required to qualify\n\n", minTradesForEvidence)

	fmt.Fprintf(f, "**Strategies retired: %d**\n\n", len(losers))

	if len(losers) == 0 {
		fmt.Fprintf(f, "_No strategies qualify for retirement — either all pass or insufficient data._\n")
		return nil, nil
	}

	fmt.Fprintf(f, "| Strategy | Trades | Win%% | PF | Expectancy | Net PnL | Retirement Reason |\n")
	fmt.Fprintf(f, "|----------|--------|------|----|-----------|---------|------------------|\n")

	retiredIDs := make([]string, 0, len(losers))
	for _, m := range losers {
		reason := retirementReason(m)
		fmt.Fprintf(f, "| %s | %d | %.1f%% | %.2f | $%.4f | $%.2f | %s |\n",
			truncate(m.StrategyID, 40),
			m.TotalTrades, m.WinRate*100, m.ProfitFactor, m.Expectancy, m.NetPnL,
			reason,
		)
		retiredIDs = append(retiredIDs, m.StrategyID)
	}

	fmt.Fprintf(f, "\n---\n\n## TOTAL DAMAGE PREVENTED\n\n")
	var totalDamage float64
	for _, m := range losers {
		if m.NetPnL < 0 {
			totalDamage += math.Abs(m.NetPnL)
		}
	}
	fmt.Fprintf(f, "By retiring these strategies, **$%.2f** in historical losses will not recur.\n\n", totalDamage)

	return retiredIDs, nil
}

func retirementReason(m StrategyMetrics) string {
	reasons := []string{}
	if m.ProfitFactor < 1.0 {
		reasons = append(reasons, fmt.Sprintf("PF=%.2f<1.0", m.ProfitFactor))
	}
	if m.Expectancy < 0 {
		reasons = append(reasons, fmt.Sprintf("Exp=$%.2f<0", m.Expectancy))
	}
	if m.NetPnL < 0 {
		reasons = append(reasons, fmt.Sprintf("PnL=$%.2f", m.NetPnL))
	}
	if len(reasons) == 0 {
		return "FAIL"
	}
	return strings.Join(reasons, "; ")
}

// ── Phase 5 — Top 50 ──────────────────────────────────────────────────────────

func writeTop50(dir string, metrics []StrategyMetrics) ([]StrategyMetrics, error) {
	// Filter to passing strategies, sort by expectancy
	var candidates []StrategyMetrics
	for _, m := range metrics {
		if m.Status == "PASS" {
			candidates = append(candidates, m)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Expectancy > candidates[j].Expectancy
	})
	if len(candidates) > 50 {
		candidates = candidates[:50]
	}

	path := filepath.Join(dir, "TOP_50_REPORT.md")
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fmt.Fprintf(f, "# TOP 50 STRATEGY REPORT\n")
	fmt.Fprintf(f, "## SEP Phase 5 — Universe Reduction\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Selection criteria:** PASS status, ≥%d trades, positive expectancy\n\n", minTradesForEvidence)
	fmt.Fprintf(f, "**Strategies qualifying: %d**\n\n", len(candidates))

	fmt.Fprintf(f, "| Rank | Strategy | Category | Trades | Win%% | PF | Expectancy | Sharpe | Tier |\n")
	fmt.Fprintf(f, "|------|----------|----------|--------|------|----|-----------|--------|------|\n")
	for i, m := range candidates {
		fmt.Fprintf(f, "| %d | %s | %s | %d | %.1f%% | %.2f | $%.4f | %.2f | %s |\n",
			i+1, truncate(m.StrategyID, 40), m.Category,
			m.TotalTrades, m.WinRate*100, m.ProfitFactor,
			m.Expectancy, m.SharpeRatio, m.Tier,
		)
	}

	return candidates, nil
}

// ── Phase 6 — Top 20 Portfolio ────────────────────────────────────────────────

func writeTop20(dir string, candidates []StrategyMetrics, allMetrics []StrategyMetrics) error {
	// From top-50, select top 20 with correlation constraints
	var top20 []StrategyMetrics
	for _, m := range candidates {
		if len(top20) >= 20 {
			break
		}
		// Check correlation vs already-selected strategies
		tooCorrelated := false
		for _, selected := range top20 {
			a, b := AlignPnLSeries(m.PnLSeries, selected.PnLSeries)
			if len(a) >= 10 {
				r := PearsonCorrelation(a, b)
				if r > 0.75 {
					tooCorrelated = true
					break
				}
			}
		}
		if !tooCorrelated {
			top20 = append(top20, m)
		}
	}

	path := filepath.Join(dir, "TOP_20_PORTFOLIO.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# TOP 20 PORTFOLIO\n")
	fmt.Fprintf(f, "## SEP Phase 6 — Institutional Core Portfolio\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(f, "**Correlation cap:** r ≤ 0.75 between any two portfolio members\n\n")
	fmt.Fprintf(f, "**Portfolio size: %d strategies**\n\n", len(top20))

	if len(top20) == 0 {
		fmt.Fprintf(f, "_Insufficient evidence to construct portfolio. Minimum %d trades per strategy required._\n", minTradesForEvidence)
		return nil
	}

	var portPF, portExp, portSharpe, portDD float64
	for _, m := range top20 {
		portPF += m.ProfitFactor
		portExp += m.Expectancy
		portSharpe += m.SharpeRatio
		portDD += m.MaxDrawdown
	}
	n := float64(len(top20))
	fmt.Fprintf(f, "## PORTFOLIO AGGREGATE METRICS\n\n")
	fmt.Fprintf(f, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(f, "| Avg Strategy PF | %.2f |\n", portPF/n)
	fmt.Fprintf(f, "| Avg Expectancy | $%.4f |\n", portExp/n)
	fmt.Fprintf(f, "| Avg Sharpe | %.2f |\n", portSharpe/n)
	fmt.Fprintf(f, "| Avg MaxDrawdown | %.1f%% |\n\n", portDD/n*100)

	fmt.Fprintf(f, "## SELECTED STRATEGIES\n\n")
	fmt.Fprintf(f, "| # | Strategy | Category | Trades | PF | Expectancy | Sharpe | Tier |\n")
	fmt.Fprintf(f, "|---|----------|----------|--------|----|-----------|--------|------|\n")
	for i, m := range top20 {
		fmt.Fprintf(f, "| %d | %s | %s | %d | %.2f | $%.4f | %.2f | %s |\n",
			i+1, truncate(m.StrategyID, 40), m.Category,
			m.TotalTrades, m.ProfitFactor, m.Expectancy, m.SharpeRatio, m.Tier,
		)
	}

	return nil
}

// ── Phase 12 — Alpha Engine Scorecard ─────────────────────────────────────────

func writeAlphaScorecard(dir string, metrics []StrategyMetrics) error {
	alphaStrategies := []string{
		"FundingMeanReversion_Alpha",
		"CVDDivergence_Alpha",
		"DeltaAbsorption_Alpha",
		"LiquiditySweepReversal_Alpha",
		"FVGRetest_Alpha",
		"OrderBlockRetest_Alpha",
		"MSSContinuation_Alpha",
		"POCBounce_Alpha",
		"SessionExpansion_Alpha",
		"LiquidationCascade_Alpha",
		"Phase11LiquiditySweepReversal_Alpha",
		"Phase11FundingMeanReversion_Alpha",
		"Phase11CVDDivergence_Alpha",
		"Phase11LiquidationCascadeReversal_Alpha",
		"Phase11FairValueGap_Alpha",
		"Phase11OrderBlock_Alpha",
		"Phase11MSSCHOCH_Alpha",
	}

	metricsMap := make(map[string]StrategyMetrics)
	for _, m := range metrics {
		metricsMap[m.StrategyID] = m
	}

	path := filepath.Join(dir, "ALPHA_SCORECARD.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# ALPHA ENGINE SCORECARD\n")
	fmt.Fprintf(f, "## SEP Phase 12 — Institutional Alpha Audit\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))

	fmt.Fprintf(f, "| Alpha Engine | Trades | Win%% | PF | Expectancy | Score | Status |\n")
	fmt.Fprintf(f, "|-------------|--------|------|----|-----------|-------|--------|\n")

	for _, name := range alphaStrategies {
		m, ok := metricsMap[name]
		if !ok {
			fmt.Fprintf(f, "| %s | 0 | — | — | — | 0 | NO_TRADES |\n", name)
			continue
		}
		score := computeAlphaScore(m)
		fmt.Fprintf(f, "| %s | %d | %.1f%% | %.2f | $%.4f | %d | %s |\n",
			truncate(name, 45), m.TotalTrades, m.WinRate*100,
			m.ProfitFactor, m.Expectancy, score, m.Status,
		)
	}

	return nil
}

func computeAlphaScore(m StrategyMetrics) int {
	if m.Status == "INSUFFICIENT_DATA" || m.TotalTrades == 0 {
		return 0
	}
	score := 0.0
	score += math.Min(m.WinRate, 1.0) * 20
	score += math.Min(m.ProfitFactor/3.0, 1.0) * 30
	score += math.Max(0, math.Min(m.SharpeRatio/3.0, 1.0)) * 30
	score += (1.0 - math.Min(m.MaxDrawdown, 1.0)) * 20
	return int(score)
}

// ── Phase 18 — Profitability Certification ───────────────────────────────────

func writeProfitabilityCert(dir string, metrics []StrategyMetrics, mc MCResult, wfResults map[string]WFResult) error {
	passing := 0
	for _, m := range metrics {
		if m.Status == "PASS" {
			passing++
		}
	}

	var totalNetPnL float64
	var pfSum, expSum float64
	n := 0
	for _, m := range metrics {
		if m.Status == "PASS" {
			totalNetPnL += m.NetPnL
			pfSum += m.ProfitFactor
			expSum += m.Expectancy
			n++
		}
	}

	portPF := 0.0
	portExp := 0.0
	if n > 0 {
		portPF = pfSum / float64(n)
		portExp = expSum / float64(n)
	}

	path := filepath.Join(dir, "PROFITABILITY_CERTIFICATION.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# PROFITABILITY CERTIFICATION\n")
	fmt.Fprintf(f, "## SEP Phase 18\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))

	questions := []struct {
		q    string
		pass bool
		note string
	}{
		{"Do strategies have edge?", passing > 0, fmt.Sprintf("%d of %d strategies pass", passing, len(metrics))},
		{"Is portfolio PF > 1.3?", portPF > 1.3, fmt.Sprintf("portfolio avg PF = %.2f", portPF)},
		{"Is expectancy positive?", portExp > 0, fmt.Sprintf("avg expectancy = $%.4f", portExp)},
		{"Is portfolio net PnL positive?", totalNetPnL > 0, fmt.Sprintf("net PnL = $%.2f", totalNetPnL)},
		{"Is Monte Carlo risk of ruin < 5%?", mc.RiskOfRuin < 0.05, fmt.Sprintf("risk of ruin = %.1f%%", mc.RiskOfRuin*100)},
	}

	allPass := true
	for _, q := range questions {
		status := "✅ PASS"
		if !q.pass {
			status = "❌ FAIL"
			allPass = false
		}
		fmt.Fprintf(f, "**%s** — %s — _%s_\n\n", q.q, status, q.note)
	}

	fmt.Fprintf(f, "---\n\n")
	if allPass {
		fmt.Fprintf(f, "## VERDICT: PROFITABILITY CERTIFIED\n\n")
		fmt.Fprintf(f, "All profitability criteria met. Portfolio is candidate for VERDICT 2.\n")
	} else {
		fmt.Fprintf(f, "## VERDICT: NOT YET CERTIFIED\n\n")
		fmt.Fprintf(f, "One or more criteria failed. Address failures before advancing verdict.\n")
	}

	return nil
}

// ── Phase 20 — Final Certification ───────────────────────────────────────────

func writeFinalCertification(dir string, metrics []StrategyMetrics, mc MCResult) error {
	passing := 0
	for _, m := range metrics {
		if m.Status == "PASS" {
			passing++
		}
	}

	n := 0
	var pfSum, expSum, sharpeSum, ddSum float64
	for _, m := range metrics {
		if m.Status == "PASS" {
			pfSum += m.ProfitFactor
			expSum += m.Expectancy
			sharpeSum += m.SharpeRatio
			ddSum += m.MaxDrawdown
			n++
		}
	}

	portPF, portExp, portSharpe, portDD := 0.0, 0.0, 0.0, 0.0
	if n > 0 {
		portPF = pfSum / float64(n)
		portExp = expSum / float64(n)
		portSharpe = sharpeSum / float64(n)
		portDD = ddSum / float64(n)
	}

	// Score each dimension 0–10
	alphaScore := math.Min(10, float64(passing)/float64(max(len(metrics), 1))*10)
	pfScore := math.Min(10, (portPF-1.0)/0.5*10)
	sharpeScore := math.Min(10, portSharpe/2.0*10)
	riskScore := math.Min(10, (1.0-mc.RiskOfRuin)*10)
	expScore := 0.0
	if portExp > 0 {
		expScore = math.Min(10, portExp/5.0*10)
	}
	composite := (alphaScore + pfScore + sharpeScore + riskScore + expScore) / 5.0

	verdict := verdictLevel(portPF, portExp, mc.RiskOfRuin, passing, portDD)

	path := filepath.Join(dir, "SEP_FINAL_CERTIFICATION.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# SEP FINAL CERTIFICATION\n")
	fmt.Fprintf(f, "## Phase 20 — Institutional Readiness Assessment\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format("2006-01-02 15:04:05 UTC"))

	fmt.Fprintf(f, "## PORTFOLIO METRICS\n\n")
	fmt.Fprintf(f, "| Metric | Value | Threshold | Status |\n|--------|-------|-----------|--------|\n")
	fmt.Fprintf(f, "| Strategies passing evidence | %d | ≥ 10 | %s |\n", passing, passIcon(passing >= 10))
	fmt.Fprintf(f, "| Portfolio PF | %.2f | ≥ 1.20 | %s |\n", portPF, passIcon(portPF >= 1.20))
	fmt.Fprintf(f, "| Avg Expectancy | $%.4f | > 0 | %s |\n", portExp, passIcon(portExp > 0))
	fmt.Fprintf(f, "| Portfolio Sharpe | %.2f | ≥ 0.5 | %s |\n", portSharpe, passIcon(portSharpe >= 0.5))
	fmt.Fprintf(f, "| Avg MaxDrawdown | %.1f%% | ≤ 20%% | %s |\n", portDD*100, passIcon(portDD <= 0.20))
	fmt.Fprintf(f, "| MC Risk of Ruin | %.1f%% | ≤ 5%% | %s |\n\n", mc.RiskOfRuin*100, passIcon(mc.RiskOfRuin <= 0.05))

	fmt.Fprintf(f, "## SCORES\n\n")
	fmt.Fprintf(f, "| Dimension | Score |\n|-----------|-------|\n")
	fmt.Fprintf(f, "| Alpha Score | %.1f/10 |\n", alphaScore)
	fmt.Fprintf(f, "| PF Score | %.1f/10 |\n", pfScore)
	fmt.Fprintf(f, "| Sharpe Score | %.1f/10 |\n", sharpeScore)
	fmt.Fprintf(f, "| Risk Score | %.1f/10 |\n", riskScore)
	fmt.Fprintf(f, "| Expectancy Score | %.1f/10 |\n", expScore)
	fmt.Fprintf(f, "| **Composite** | **%.1f/10** |\n\n", composite)

	fmt.Fprintf(f, "## FINAL VERDICT\n\n")
	fmt.Fprintf(f, "```\n%s\n```\n\n", verdict)
	fmt.Fprintf(f, "%s\n", verdictDescription(verdict))

	return nil
}

func verdictLevel(portPF, portExp, ror float64, passing int, portDD float64) string {
	if portPF >= 1.5 && portExp > 0 && ror < 0.02 && passing >= 10 && portDD < 0.15 {
		return "VERDICT 1 — CAPITAL READY"
	}
	if portPF >= 1.2 && portExp > 0 && ror < 0.05 && passing >= 5 && portDD < 0.20 {
		return "VERDICT 2 — PAPER CAPITAL READY"
	}
	if portPF >= 1.0 && passing > 0 {
		return "VERDICT 3 — LIMITED EDGE"
	}
	return "VERDICT 4 — NO PROVEN EDGE"
}

func verdictDescription(v string) string {
	switch {
	case strings.Contains(v, "VERDICT 1"):
		return "All institutional criteria met. Ready for live capital deployment with position sizing per CAPITAL_ALLOCATION_REPORT."
	case strings.Contains(v, "VERDICT 2"):
		return "Paper trading criteria met. Deploy on paper capital. Upgrade to VERDICT 1 requires 90-day live paper certification."
	case strings.Contains(v, "VERDICT 3"):
		return "Limited edge detected. Insufficient evidence for capital deployment. Continue evidence accumulation."
	default:
		return "No proven statistical edge. Do not deploy capital. Rebuild strategy selection from scratch."
	}
}

func passIcon(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
