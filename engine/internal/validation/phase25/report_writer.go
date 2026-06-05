package phase25

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WriteAllReports writes all 14 Phase 25 markdown reports to outDir.
func WriteAllReports(r Phase25Result, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	writers := []struct {
		name string
		fn   func(Phase25Result) string
	}{
		{"PHASE25_STRATEGY_ELIGIBILITY_REPORT.md", writeEligibility},
		{"LIVE_FORWARD_VALIDATION_REPORT.md", writeLiveForward},
		{"PAPER_TRADING_CAMPAIGN_REPORT.md", writePaperTrading},
		{"EDGE_DRIFT_REPORT.md", writeEdgeDrift},
		{"ALPHA_SURVIVAL_REPORT.md", writeAlphaSurvival},
		{"CAPITAL_ESCALATION_REPORT.md", writeCapEscalation},
		{"AUTO_DEMOTION_REPORT.md", writeAutoDemotion},
		{"PORTFOLIO_HEAT_REPORT.md", writePortfolioHeat},
		{"EXECUTION_QUALITY_REPORT.md", writeExecQuality},
		{"MONTHLY_STRATEGY_REPORT.md", writeMonthlyStrategy},
		{"MONTHLY_ALPHA_REPORT.md", writeMonthlyAlpha},
		{"MONTHLY_PORTFOLIO_REPORT.md", writeMonthlyPortfolio},
		{"MONTHLY_CAPITAL_REPORT.md", writeMonthlyCapital},
		{"PHASE25_FINAL_LIVE_VERDICT.md", writeFinalLiveVerdict},
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

func hdr25(title string, r Phase25Result) string {
	return fmt.Sprintf("# %s\n\n**Generated:** %s  \n**Symbol:** %s  \n**Live Window:** %d days  \n**Live Trades:** %d  \n**Eligible Strategies:** %d\n\n---\n\n",
		title,
		r.GeneratedAt.Format("2006-01-02 15:04 UTC"),
		r.Config.Symbol,
		r.Config.LiveDays,
		len(r.LiveTrades),
		r.TotalEligible,
	)
}

// ── 25.0: Eligibility ─────────────────────────────────────────────────────────

func writeEligibility(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — STRATEGY ELIGIBILITY REPORT", r))

	approved, excluded := 0, 0
	for _, e := range r.Eligibility {
		if e.ApprovedForLive {
			approved++
		} else {
			excluded++
		}
	}

	fmt.Fprintf(&b, "## Summary\n\n- **Total Phase 24 Certified:** %d\n- **Approved For Live:** %d\n- **Excluded:** %d\n\n", len(r.Eligibility), approved, excluded)

	b.WriteString("## Eligibility Table\n\n")
	b.WriteString("| Strategy | Phase24 Tier | Hist PF | Hist Sharpe | Hist DD% | Hist Expectancy | Approved |\n")
	b.WriteString("|----------|--------------|---------|-------------|---------|-----------------|----------|\n")

	sorted := make([]EligibilityRecord, len(r.Eligibility))
	copy(sorted, r.Eligibility)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].ApprovedForLive != sorted[j].ApprovedForLive {
			return sorted[i].ApprovedForLive
		}
		return sorted[i].HistoricalPF > sorted[j].HistoricalPF
	})

	for _, e := range sorted {
		approved := "YES"
		if !e.ApprovedForLive {
			approved = fmt.Sprintf("NO — %s", e.ExclusionReason)
		}
		fmt.Fprintf(&b, "| %s | %s | %.3f | %.2f | %.1f%% | $%.2f | %s |\n",
			trunc25(e.StrategyName, 35), e.Phase24Tier,
			e.HistoricalPF, e.HistoricalSharpe, e.HistoricalDD, e.HistoricalExpectancy,
			approved)
	}

	b.WriteString("\n## Excluded Strategies\n\n")
	for _, e := range sorted {
		if !e.ApprovedForLive {
			fmt.Fprintf(&b, "- **%s** — %s\n", e.StrategyName, e.ExclusionReason)
		}
	}
	return b.String()
}

// ── 25.1: Live Forward Validation ────────────────────────────────────────────

func writeLiveForward(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — LIVE FORWARD VALIDATION REPORT", r))

	fmt.Fprintf(&b, "## Campaign Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Platform Live PF | %.4f |\n", r.PlatformLivePF)
	fmt.Fprintf(&b, "| Platform Live Sharpe | %.4f |\n", r.PlatformLiveSharpe)
	fmt.Fprintf(&b, "| Platform Live Net PnL | $%.2f |\n", r.PlatformLiveNetPnLUSD)
	fmt.Fprintf(&b, "| Total Live Trades | %d |\n", len(r.LiveTrades))
	fmt.Fprintf(&b, "| Total Candles Processed | %d |\n", r.TotalCandles)
	fmt.Fprintf(&b, "| Live-Certified Strategies | %d |\n", r.TotalLiveCertified)
	fmt.Fprintf(&b, "| Auto-Demoted | %d |\n", r.TotalDemoted)
	fmt.Fprintf(&b, "| Retired | %d |\n\n", r.TotalRetired)

	b.WriteString("## Per-Strategy Live Performance\n\n")
	b.WriteString("| Strategy | Trades | WR% | PF | Sharpe | Sortino | DD% | Expectancy | RoR% | Net PnL |\n")
	b.WriteString("|----------|--------|-----|----|--------|---------|-----|------------|------|--------|\n")

	ranked := make([]LiveMetrics, 0, len(r.LiveMetrics))
	for _, m := range r.LiveMetrics {
		ranked = append(ranked, m)
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].ProfitFactor > ranked[j].ProfitFactor })

	for _, m := range ranked {
		fmt.Fprintf(&b, "| %s | %d | %.1f | %.3f | %.2f | %.2f | %.1f | $%.2f | %.2f | $%.0f |\n",
			trunc25(m.StrategyName, 35), m.TotalTrades, m.WinRate*100,
			m.ProfitFactor, m.Sharpe, m.Sortino, m.MaxDrawdown,
			m.Expectancy, m.RiskOfRuin*100, m.NetPnLUSD)
	}
	return b.String()
}

// ── 25.2: Paper Trading Campaign ─────────────────────────────────────────────

func writePaperTrading(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — PAPER TRADING CAMPAIGN REPORT", r))

	b.WriteString("## Campaign Configuration\n\n")
	fmt.Fprintf(&b, "- **Mode:** Paper-only (zero real capital at risk)\n")
	fmt.Fprintf(&b, "- **Symbol:** %s\n", r.Config.Symbol)
	fmt.Fprintf(&b, "- **Live Window:** %d days\n", r.Config.LiveDays)
	fmt.Fprintf(&b, "- **Initial Paper Capital:** $%.0f\n", r.Config.InitialCapital)
	fmt.Fprintf(&b, "- **Fee Model:** Taker=%.2fbps Slippage=%.2fbps\n\n", r.Config.TakerFeeBps, r.Config.SlippageBps)

	b.WriteString("## Audit Trail Summary\n\n")
	totalFees := 0.0
	totalSlip := 0.0
	totalFunding := 0.0
	for _, t := range r.LiveTrades {
		totalFees += t.FeeUSD
		totalSlip += t.SlippageUSD
		totalFunding += t.FundingUSD
	}
	fmt.Fprintf(&b, "| Cost Component | Total USD |\n|----------------|----------|\n")
	fmt.Fprintf(&b, "| Execution Fees | $%.2f |\n", totalFees)
	fmt.Fprintf(&b, "| Slippage | $%.2f |\n", totalSlip)
	fmt.Fprintf(&b, "| Funding Costs | $%.2f |\n", totalFunding)
	fmt.Fprintf(&b, "| Total Costs | $%.2f |\n\n", totalFees+totalSlip+totalFunding)

	// Sample trade log (first 20)
	b.WriteString("## Sample Trade Log (First 20 Trades)\n\n")
	b.WriteString("| TradeID | Strategy | Direction | Entry | Exit | Size | NetPnL | Fees | Slippage |\n")
	b.WriteString("|---------|----------|-----------|-------|------|------|--------|------|----------|\n")
	limit := 20
	if len(r.LiveTrades) < limit {
		limit = len(r.LiveTrades)
	}
	for _, t := range r.LiveTrades[:limit] {
		fmt.Fprintf(&b, "| %s | %s | %s | %.2f | %.2f | %.4f | $%.2f | $%.2f | $%.2f |\n",
			trunc25(t.TradeID, 20), trunc25(t.StrategyName, 25), t.Direction,
			t.EntryPrice, t.ExitPrice, t.Size, t.NetPnLUSD, t.FeeUSD, t.SlippageUSD)
	}
	return b.String()
}

// ── 25.3: Edge Drift ──────────────────────────────────────────────────────────

func writeEdgeDrift(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — EDGE DRIFT DETECTION REPORT", r))

	summary := DriftSummary(r.EdgeDrift)
	b.WriteString("## Drift Summary\n\n")
	b.WriteString("| Status | Count |\n|--------|-------|\n")
	for _, s := range []EdgeStatus{EdgeImproving, EdgeStable, EdgeDecaying, EdgeFailed} {
		fmt.Fprintf(&b, "| %s | %d |\n", s, summary[s])
	}

	b.WriteString("\n## Per-Strategy Edge Drift\n\n")
	b.WriteString("| Strategy | Hist PF | Live PF | PF Drift% | Hist Sharpe | Live Sharpe | Sharpe Drift% | Hist DD% | Live DD% | Status |\n")
	b.WriteString("|----------|---------|---------|-----------|-------------|-------------|---------------|----------|----------|--------|\n")

	sorted := make([]EdgeDriftRecord, len(r.EdgeDrift))
	copy(sorted, r.EdgeDrift)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].PFDriftPct > sorted[j].PFDriftPct })

	for _, d := range sorted {
		fmt.Fprintf(&b, "| %s | %.3f | %.3f | %+.1f%% | %.2f | %.2f | %+.1f%% | %.1f | %.1f | **%s** |\n",
			trunc25(d.StrategyName, 30),
			d.HistoricalPF, d.LivePF, d.PFDriftPct,
			d.HistoricalSharpe, d.LiveSharpe, d.SharpeDriftPct,
			d.HistoricalDD, d.LiveDD,
			d.EdgeStatus)
	}

	b.WriteString("\n## Evidence\n\n")
	for _, d := range sorted {
		fmt.Fprintf(&b, "- **%s**: %s\n", trunc25(d.StrategyName, 40), d.Evidence)
	}
	return b.String()
}

// ── 25.4: Alpha Survival ──────────────────────────────────────────────────────

func writeAlphaSurvival(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — ALPHA SURVIVAL ANALYSIS REPORT", r))

	if len(r.AlphaSurvival) > 0 {
		champ := r.AlphaSurvival[0]
		weak := r.AlphaSurvival[len(r.AlphaSurvival)-1]
		fmt.Fprintf(&b, "## Champion Alpha: **%s** (PF=%.3f, Sharpe=%.2f, Verdict=%s)\n\n", champ.Family, champ.ProfitFactor, champ.Sharpe, champ.Verdict)
		fmt.Fprintf(&b, "## Weakest Alpha: **%s** (PF=%.3f, Sharpe=%.2f, Verdict=%s)\n\n", weak.Family, weak.ProfitFactor, weak.Sharpe, weak.Verdict)
	}

	b.WriteString("## Alpha Family Rankings\n\n")
	b.WriteString("| Rank | Family | Trades | WR% | PF | Sharpe | Sortino | Expectancy | MaxDD% | RoR% | CapEfficiency | ExecQ | Verdict |\n")
	b.WriteString("|------|--------|--------|-----|----|--------|---------|------------|-------|------|--------------|-------|--------|\n")

	for _, a := range r.AlphaSurvival {
		fmt.Fprintf(&b, "| %d | %s | %d | %.1f | %.3f | %.2f | %.2f | $%.2f | %.1f | %.1f | %.2f | %.0f | **%s** |\n",
			a.Rank, a.Family, a.Trades, a.WinRate*100,
			a.ProfitFactor, a.Sharpe, a.Sortino,
			a.Expectancy, a.MaxDD, a.RiskOfRuin*100,
			a.CapitalEfficiency, a.ExecQualityScore, a.Verdict)
	}

	b.WriteString("\n## Retirement Candidates\n\n")
	for _, a := range r.AlphaSurvival {
		if a.Verdict == "RETIREMENT_CANDIDATE" || a.Verdict == "NO_TRADES" {
			fmt.Fprintf(&b, "- **%s** — Verdict: %s | PF=%.3f | Trades=%d\n", a.Family, a.Verdict, a.ProfitFactor, a.Trades)
		}
	}
	return b.String()
}

// ── 25.5: Capital Escalation ──────────────────────────────────────────────────

func writeCapEscalation(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — CAPITAL ESCALATION REPORT", r))

	b.WriteString("## Tier Ladder\n\n")
	b.WriteString("| Tier | Capital |\n|------|---------|\n")
	for _, tier := range []CapLadderTier{TierPaper, TierPilot, TierLimited, TierFull, TierInstitutional} {
		fmt.Fprintf(&b, "| %s | $%.0f |\n", tier, TierCapitalUSD[tier])
	}

	totalDeployed := TotalCapitalDeployed(r.CapEscalation)
	fmt.Fprintf(&b, "\n**Total Recommended Deployment:** $%.0f\n\n", totalDeployed)

	b.WriteString("## Strategy Escalation Decisions\n\n")
	b.WriteString("| Strategy | Current Tier | Recommended Tier | Action | Current $ | Recommended $ | Gates Passed |\n")
	b.WriteString("|----------|--------------|-----------------|--------|-----------|--------------|-------------|\n")

	sorted := make([]CapEscalationRecord, len(r.CapEscalation))
	copy(sorted, r.CapEscalation)
	sort.Slice(sorted, func(i, j int) bool { return tierOrd25(sorted[i].RecommendedTier) > tierOrd25(sorted[j].RecommendedTier) })

	for _, e := range sorted {
		fmt.Fprintf(&b, "| %s | %s | %s | **%s** | $%.0f | $%.0f | %d/%d |\n",
			trunc25(e.StrategyName, 30), e.CurrentTier, e.RecommendedTier, e.PromotionStatus,
			e.CurrentCapitalUSD, e.RecommendedCapUSD,
			len(e.GatesPassed), len(e.GatesPassed)+len(e.GatesFailed))
	}

	b.WriteString("\n## Reasoning\n\n")
	for _, e := range sorted {
		fmt.Fprintf(&b, "- **%s**: %s\n", trunc25(e.StrategyName, 35), e.Reasoning)
	}
	return b.String()
}

// ── 25.6: Auto Demotion ──────────────────────────────────────────────────────

func writeAutoDemotion(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — AUTO-DEMOTION ENGINE REPORT", r))

	if len(r.Demotions) == 0 {
		b.WriteString("## Result\n\nNo automatic demotions triggered. All eligible strategies remain within acceptable live performance bounds.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "## Summary: %d Demotions Triggered\n\n", len(r.Demotions))

	actionCounts := make(map[string]int)
	for _, d := range r.Demotions {
		actionCounts[d.Action]++
	}
	b.WriteString("| Action | Count |\n|--------|-------|\n")
	for _, a := range []string{"REDUCE_ALLOCATION", "MOVE_WATCHLIST", "PAPER_ONLY", "RETIRE"} {
		fmt.Fprintf(&b, "| %s | %d |\n", a, actionCounts[a])
	}

	b.WriteString("\n## Demotion Records\n\n")
	b.WriteString("| Strategy | Previous Tier | New Tier | Action | Triggers |\n")
	b.WriteString("|----------|---------------|---------|--------|----------|\n")
	for _, d := range r.Demotions {
		fmt.Fprintf(&b, "| %s | %s | %s | **%s** | %s |\n",
			trunc25(d.StrategyName, 30), d.PreviousTier, d.NewTier, d.Action,
			strings.Join(d.TriggeredBy, "; "))
	}

	b.WriteString("\n## Evidence\n\n")
	for _, d := range r.Demotions {
		fmt.Fprintf(&b, "- **%s**: %s\n", trunc25(d.StrategyName, 35), d.Evidence)
	}
	return b.String()
}

// ── 25.7: Portfolio Heat ──────────────────────────────────────────────────────

func writePortfolioHeat(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — PORTFOLIO HEAT MANAGEMENT REPORT", r))
	h := r.PortfolioHeat

	fmt.Fprintf(&b, "## Heat Score: **%.1f / 100**\n\n", h.TotalHeat)
	if h.TotalHeat < 30 {
		b.WriteString("Status: **LOW HEAT** — Portfolio risk within acceptable bounds.\n\n")
	} else if h.TotalHeat < 60 {
		b.WriteString("Status: **MEDIUM HEAT** — Monitor concentration and correlation.\n\n")
	} else {
		b.WriteString("Status: **HIGH HEAT** — Immediate risk mitigation required.\n\n")
	}

	fmt.Fprintf(&b, "## Concentration Metrics\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Top-1 Capital Concentration | %.1f%% |\n", h.CapConcentrationTop1)
	fmt.Fprintf(&b, "| Top-3 Capital Concentration | %.1f%% |\n", h.CapConcentrationTop3)
	fmt.Fprintf(&b, "| Direction Long | %.1f%% |\n", h.DirectionLong)
	fmt.Fprintf(&b, "| Direction Short | %.1f%% |\n\n", h.DirectionShort)

	b.WriteString("## Alpha Family Concentration\n\n")
	b.WriteString("| Family | Capital% |\n|--------|----------|\n")
	for f, pct := range h.AlphaConcentration {
		fmt.Fprintf(&b, "| %s | %.1f%% |\n", f, pct)
	}

	if len(h.StrategyCorrelations) > 0 {
		b.WriteString("\n## Top Correlated Strategy Pairs\n\n")
		b.WriteString("| Strategy A | Strategy B | Correlation |\n|------------|------------|-------------|\n")
		for _, p := range h.StrategyCorrelations {
			fmt.Fprintf(&b, "| %s | %s | %.3f |\n", trunc25(p.A, 30), trunc25(p.B, 30), p.Correlation)
		}
	}

	if len(h.Violations) > 0 {
		b.WriteString("\n## Heat Violations\n\n")
		b.WriteString("| Type | Severity | Description |\n|------|----------|-------------|\n")
		for _, v := range h.Violations {
			fmt.Fprintf(&b, "| %s | **%s** | %s |\n", v.Type, v.Severity, v.Description)
		}
	}

	b.WriteString("\n## Mitigation Actions\n\n")
	for _, a := range h.MitigationActions {
		fmt.Fprintf(&b, "- %s\n", a)
	}
	return b.String()
}

// ── 25.8: Execution Quality ───────────────────────────────────────────────────

func writeExecQuality(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — LIVE EXECUTION QUALITY REPORT", r))
	eq := r.ExecQuality

	fmt.Fprintf(&b, "## Quality Score: **%.1f / 100**\n\n", eq.QualityScore)

	writeLatencyTable := func(title string, ls LatencyStats) {
		fmt.Fprintf(&b, "### %s\n\n", title)
		fmt.Fprintf(&b, "| P50 | P95 | P99 | Worst | Average |\n|-----|-----|-----|-------|--------|\n")
		fmt.Fprintf(&b, "| %.2fms | %.2fms | %.2fms | %.2fms | %.2fms |\n\n",
			ls.P50, ls.P95, ls.P99, ls.Worst, ls.Average)
	}

	b.WriteString("## Latency Breakdown\n\n")
	writeLatencyTable("Signal → Submit", eq.SignalToSubmit)
	writeLatencyTable("Submit → Ack", eq.SubmitToAck)
	writeLatencyTable("Ack → Fill", eq.AckToFill)
	writeLatencyTable("Fill → Position", eq.FillToPosition)
	writeLatencyTable("Position → Exit", eq.PositionToExit)
	writeLatencyTable("End-to-End", eq.EndToEnd)

	b.WriteString("## Cost Analysis\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Avg Slippage | %.3f bps |\n", eq.AvgSlippageBps)
	fmt.Fprintf(&b, "| Avg Funding Cost | %.3f bps |\n", eq.AvgFundingCostBps)
	fmt.Fprintf(&b, "| Avg Fees | %.3f bps |\n", eq.AvgFeesBps)
	fmt.Fprintf(&b, "| Execution Drift vs Backtest | %+.1f%% |\n", eq.ExecDriftPct)
	fmt.Fprintf(&b, "| Missed Entries | %d |\n", eq.TotalMissedEntries)
	fmt.Fprintf(&b, "| Expired Signals | %d |\n\n", eq.TotalExpiredSignals)
	return b.String()
}

// ── 25.9: Monthly Reports ─────────────────────────────────────────────────────

func writeMonthlyStrategy(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — MONTHLY STRATEGY CERTIFICATION REPORT", r))
	b.WriteString("| Strategy | Month | Trades | WR% | PF | Sharpe | Sortino | Expectancy | DD% | RoR% |\n")
	b.WriteString("|----------|-------|--------|-----|----|--------|---------|------------|-----|------|\n")
	for _, m := range r.MonthlyStrategies {
		fmt.Fprintf(&b, "| %s | %s | %d | %.1f | %.3f | %.2f | %.2f | $%.2f | %.1f | %.2f |\n",
			trunc25(m.StrategyName, 30), m.Month, m.Trades, m.WinRate*100,
			m.ProfitFactor, m.Sharpe, m.Sortino, m.Expectancy, m.MaxDD, m.RiskOfRuin*100)
	}
	return b.String()
}

func writeMonthlyAlpha(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — MONTHLY ALPHA ENGINE REPORT", r))

	if len(r.MonthlyAlphas) > 0 {
		best := r.MonthlyAlphas[0]
		worst := r.MonthlyAlphas[len(r.MonthlyAlphas)-1]
		fmt.Fprintf(&b, "## Best Alpha: **%s** (PF=%.3f, Verdict=%s)\n", best.Family, best.PF, best.Verdict)
		fmt.Fprintf(&b, "## Worst Alpha: **%s** (PF=%.3f, Verdict=%s)\n\n", worst.Family, worst.PF, worst.Verdict)
	}

	b.WriteString("| Rank | Family | Month | Trades | PF | Sharpe | Net PnL | Verdict |\n")
	b.WriteString("|------|--------|-------|--------|-----|--------|---------|--------|\n")
	for _, a := range r.MonthlyAlphas {
		fmt.Fprintf(&b, "| %d | %s | %s | %d | %.3f | %.2f | $%.0f | %s |\n",
			a.Rank, a.Family, a.Month, a.Trades, a.PF, a.Sharpe, a.NetPnL, a.Verdict)
	}
	return b.String()
}

func writeMonthlyPortfolio(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — MONTHLY PORTFOLIO REPORT", r))
	b.WriteString("| Month | Return% | Volatility | Sharpe | MaxDD% | RoR% | NetPnL |\n")
	b.WriteString("|-------|---------|------------|--------|--------|------|--------|\n")
	for _, p := range r.MonthlyPortfolios {
		fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.2f | %.1f | %.2f | $%.0f |\n",
			p.Month, p.Return, p.Volatility, p.Sharpe, p.MaxDD, p.RiskOfRuin*100, p.NetPnLUSD)
	}
	return b.String()
}

func writeMonthlyCapital(r Phase25Result) string {
	var b strings.Builder
	b.WriteString(hdr25("PHASE 25 — MONTHLY CAPITAL ACTION REPORT", r))
	b.WriteString("| Strategy | Action | Old Tier | New Tier | Reasoning |\n")
	b.WriteString("|----------|--------|----------|---------|----------|\n")
	for _, a := range r.MonthlyCapital {
		fmt.Fprintf(&b, "| %s | **%s** | %s | %s | %s |\n",
			trunc25(a.StrategyName, 30), a.Action, a.OldTier, a.NewTier,
			trunc25(a.Reasoning, 80))
	}
	return b.String()
}

// ── 25.10: Final Live Verdict ─────────────────────────────────────────────────

func writeFinalLiveVerdict(r Phase25Result) string {
	var b strings.Builder
	v := r.Verdict

	b.WriteString(hdr25("PHASE 25 — FINAL LIVE DEPLOYMENT VERDICT", r))

	icon := map[string]string{"DEPLOY": "✅", "PAPER_ONLY": "⚠️", "HALT": "❌"}[v.OverallVerdict]
	fmt.Fprintf(&b, "## %s VERDICT: %s\n\n", icon, v.OverallVerdict)
	fmt.Fprintf(&b, "> %s\n\n", v.Justification)

	b.WriteString("---\n\n## Platform Live Summary\n\n")
	fmt.Fprintf(&b, "| Metric | Value |\n|--------|-------|\n")
	fmt.Fprintf(&b, "| Live Platform PF | %.4f |\n", v.PlatformLivePF)
	fmt.Fprintf(&b, "| Live Platform Sharpe | %.4f |\n", v.PlatformLiveSharpe)
	fmt.Fprintf(&b, "| Live Net PnL | $%.2f |\n", v.PlatformLiveNetPnL)
	fmt.Fprintf(&b, "| Total Live Trades | %d |\n", v.TotalLiveTrades)
	fmt.Fprintf(&b, "| Live-Certified Strategies | %d |\n\n", v.LiveCertifiedStrategies)

	b.WriteString("---\n\n## 15 Institutional Questions — Evidence-Based Answers\n\n")

	qa := func(n int, q, evidence string) {
		fmt.Fprintf(&b, "### Q%d: %s\n\n**Evidence:** %s\n\n", n, q, evidence)
	}

	qa(1, "Do the certified strategies still have edge?", v.Q1_Evidence)

	b.WriteString("### Q2: Which strategies improved?\n\n")
	for i, s := range v.Q2_ImprovedStrategies {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q3: Which strategies degraded?\n\n")
	for i, s := range v.Q3_DegradedStrategies {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Q4: Which alpha family is strongest?\n\n**%s** — %s\n\n", v.Q4_StrongestAlpha, v.Q4_Evidence)
	fmt.Fprintf(&b, "### Q5: Which alpha family is weakest?\n\n**%s** — %s\n\n", v.Q5_WeakestAlpha, v.Q5_Evidence)

	b.WriteString("### Q6: Which strategies deserve promotion?\n\n")
	for i, s := range v.Q6_PromotionCandidates {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q7: Which strategies require demotion?\n\n")
	for i, s := range v.Q7_DemotionCandidates {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	b.WriteString("### Q8: Which strategies should be retired?\n\n")
	for i, s := range v.Q8_RetirementList {
		fmt.Fprintf(&b, "%d. %s\n", i+1, s)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "### Q9: Expected annual return?\n\n**%.2f%%** (adjusted for live drift)\n\n", v.Q9_ExpectedAnnualReturn)
	fmt.Fprintf(&b, "### Q10: Expected maximum drawdown?\n\n**%.2f%%** (adjusted for live drift)\n\n", v.Q10_ExpectedMaxDD)
	fmt.Fprintf(&b, "### Q11: Expected Sharpe ratio?\n\n**%.4f** (live-forward observed)\n\n", v.Q11_ExpectedSharpe)
	fmt.Fprintf(&b, "### Q12: Probability of ruin?\n\n**%.4f%%** (portfolio-level, live data)\n\n", v.Q12_ProbabilityOfRuin*100)

	qa(13, "Is live deployment justified?", v.Q13_Evidence)
	qa(14, "Is institutional capital justified?", v.Q14_Evidence)

	b.WriteString("### Q15: Recommended capital allocation model\n\n")
	b.WriteString("| Strategy | Recommended Tier | Capital |\n|----------|-----------------|--------|\n")
	strats := make([]string, 0, len(v.Q15_RecommendedCapAllocation))
	for s := range v.Q15_RecommendedCapAllocation {
		strats = append(strats, s)
	}
	sort.Strings(strats)
	for _, s := range strats {
		tier := v.Q15_RecommendedCapAllocation[s]
		fmt.Fprintf(&b, "| %s | %s | $%.0f |\n", trunc25(s, 35), tier, TierCapitalUSD[tier])
	}

	fmt.Fprintf(&b, "\n---\n\n*Report generated %s by Phase 25 Live Edge Verification Engine.*\n",
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
	return b.String()
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func trunc25(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
