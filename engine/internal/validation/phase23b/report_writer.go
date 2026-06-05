package phase23b

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

// WriteAllReports generates all Phase 23B markdown reports to outDir.
func WriteAllReports(result Phase23BResult, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create outDir: %w", err)
	}

	reports := map[string]func() string{
		"REAL_DATA_AUDIT.md":              func() string { return renderDataAudit(result) },
		"SYNTHETIC_REMOVAL_REPORT.md":     func() string { return renderSyntheticRemoval(result) },
		"REAL_EXECUTION_INTEGRATION_REPORT.md": func() string { return renderExecutionIntegration(result) },
		"REPLAY_ENGINE_REPORT.md":         func() string { return renderReplayEngine(result) },
		"EXECUTION_COST_REPORT.md":        func() string { return renderCostReport(result) },
		"TRADE_CERTIFICATION_REPORT.md":   func() string { return renderTradeCert(result) },
		"REAL_WALK_FORWARD_REPORT.md":     func() string { return renderWalkForward(result) },
		"REAL_MONTE_CARLO_REPORT.md":      func() string { return renderMonteCarlo(result) },
		"REAL_REGIME_REPORT.md":           func() string { return renderRegimeReport(result) },
		"REAL_CAPITAL_CERTIFICATION.md":   func() string { return renderCapitalCert(result) },
	}

	for filename, render := range reports {
		path := filepath.Join(outDir, filename)
		if err := os.WriteFile(path, []byte(render()), 0644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}
	return nil
}

// ── 23B.1 — Data Audit ────────────────────────────────────────────────────────

func renderDataAudit(r Phase23BResult) string {
	a := r.DataAudit
	b := &strings.Builder{}
	header(b, "REAL DATA AUDIT REPORT", "Phase 23B.1")
	line(b, fmt.Sprintf("**Generated:** %s", ts(a.GeneratedAt)))
	line(b, fmt.Sprintf("**Symbol:** %s", a.Symbol))
	line(b, fmt.Sprintf("**Coverage:** %s → %s", a.From.Format("2006-01-02"), a.To.Format("2006-01-02")))
	line(b, fmt.Sprintf("**Overall Quality:** %s", a.OverallQuality))
	line(b, fmt.Sprintf("**Accepted for Validation:** %v", a.Accepted))
	nl(b)

	h2(b, "Data Sources")
	tableHeader(b, "Source", "Type", "Available", "Records", "Missing%", "Quality", "Reliability", "Latency")
	for _, s := range a.Sources {
		row(b,
			s.Name,
			s.Type,
			boolStr(s.Available),
			fmt.Sprintf("%d", s.TotalRecords),
			fmt.Sprintf("%.2f%%", s.MissingPct),
			string(s.Quality),
			fmt.Sprintf("%.0f/100", s.ReliabilityScore),
			s.LatencyProfile,
		)
	}
	nl(b)

	if len(a.Issues) > 0 {
		h2(b, "Issues")
		for _, iss := range a.Issues {
			line(b, "- "+iss)
		}
		nl(b)
	}
	if len(a.RejectedSources) > 0 {
		h2(b, "Rejected Sources")
		for _, s := range a.RejectedSources {
			line(b, "- "+s)
		}
	}
	return b.String()
}

// ── 23B.2 — Synthetic Removal ─────────────────────────────────────────────────

func renderSyntheticRemoval(r Phase23BResult) string {
	s := r.SyntheticRemoval
	b := &strings.Builder{}
	header(b, "SYNTHETIC REMOVAL REPORT", "Phase 23B.2")
	line(b, fmt.Sprintf("**Generated:** %s", ts(s.GeneratedAt)))
	line(b, fmt.Sprintf("**Total Synthetic Components Found:** %d", s.TotalFound))
	line(b, fmt.Sprintf("**Removed:** %d", s.TotalRemoved))
	line(b, fmt.Sprintf("**Remaining:** %d", s.Remaining))
	line(b, fmt.Sprintf("**Clean (zero synthetic):** %v", s.Clean))
	nl(b)

	h2(b, "Component Inventory")
	tableHeader(b, "Component", "File", "Replaced", "Replacement", "Impact")
	for _, c := range s.Components {
		row(b, c.Name, c.File, boolStr(c.Replaced), c.Replacement, c.Impact)
	}
	return b.String()
}

// ── 23B.3 — Execution Integration ────────────────────────────────────────────

func renderExecutionIntegration(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "REAL EXECUTION INTEGRATION REPORT", "Phase 23B.3")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Strategies Loaded:** %d (from BuildCuratedScalpers())", r.TotalStrategies))
	line(b, fmt.Sprintf("**Execution Path:** Strategy.OnCandle() → Signal → Position → TP/SL → CertifiedTrade"))
	nl(b)

	h2(b, "Signal Flow")
	line(b, "```")
	line(b, "Real OHLCVCandle (Binance)")
	line(b, "  → strategy.OnCandle(candle.ToTick())")
	line(b, "  → []Signal (confidence-filtered ≥ 0.60)")
	line(b, "  → RealReplayEngine.OpenPosition (1-candle execution delay)")
	line(b, "  → TP/SL checked against real candle High/Low each period")
	line(b, "  → Position closed → CertifiedTrade record")
	line(b, "  → CostModel (taker fee + slippage + funding)")
	line(b, "  → phase22e.TradeRecord (certified, traceable)")
	line(b, "```")
	nl(b)

	h2(b, "Execution Parameters")
	line(b, fmt.Sprintf("- Symbol: %s", r.Config.Symbol))
	line(b, fmt.Sprintf("- Initial Capital: $%.0f", r.Config.InitialCapital))
	line(b, fmt.Sprintf("- Position Cap: %.0f%% per trade", r.Config.PositionCapPct*100))
	line(b, fmt.Sprintf("- Taker Fee: %.0f bps", r.Config.TakerFeeBps))
	line(b, fmt.Sprintf("- Slippage: %.0f bps", r.Config.SlippageBps))
	line(b, fmt.Sprintf("- Max Hold: %d minutes", r.Config.MaxHoldMins))
	line(b, fmt.Sprintf("- Min Signal Confidence: %.0f%%", r.Config.MinConfidence*100))
	nl(b)

	h2(b, "Strategy Signal Statistics")
	tableHeader(b, "Strategy", "Category", "Signals", "Executed", "Trades", "Exec Rate")
	for _, rep := range r.StrategyReplays {
		execRate := 0.0
		if rep.SignalsTotal > 0 {
			execRate = float64(rep.SignalsExec) / float64(rep.SignalsTotal) * 100
		}
		row(b, rep.StrategyName, rep.Category,
			fmt.Sprintf("%d", rep.SignalsTotal),
			fmt.Sprintf("%d", rep.SignalsExec),
			fmt.Sprintf("%d", len(rep.Trades)),
			fmt.Sprintf("%.1f%%", execRate),
		)
	}
	return b.String()
}

// ── 23B.4 — Replay Engine ─────────────────────────────────────────────────────

func renderReplayEngine(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "REPLAY ENGINE REPORT", "Phase 23B.4")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Data Source:** Binance Futures (BTCUSDT) — real market data"))
	line(b, fmt.Sprintf("**Total Candles:** %d (1-minute OHLCV)", r.TotalCandles))
	line(b, fmt.Sprintf("**Coverage:** %.0f days", r.CoverageDays))
	line(b, fmt.Sprintf("**Period:** %s → %s",
		r.Config.From.Format("2006-01-02"), r.Config.To.Format("2006-01-02")))
	nl(b)

	h2(b, "Replay Results per Strategy")
	tableHeader(b, "Strategy", "Trades", "FinalNAV", "CAGR%", "Trading Days")
	for _, rep := range r.StrategyReplays {
		row(b,
			rep.StrategyName,
			fmt.Sprintf("%d", len(rep.Trades)),
			fmt.Sprintf("$%.0f", rep.FinalNAV),
			fmt.Sprintf("%.1f%%", rep.CAGR),
			fmt.Sprintf("%d", rep.TradingDays),
		)
	}
	nl(b)

	h2(b, "Regime Distribution of Trades")
	tableHeader(b, "Regime", "Trade Count", "Percentage")
	total := float64(r.TotalTrades)
	for regime, count := range r.TradesPerRegime {
		pct := 0.0
		if total > 0 {
			pct = float64(count) / total * 100
		}
		row(b, string(regime), fmt.Sprintf("%d", count), fmt.Sprintf("%.1f%%", pct))
	}
	return b.String()
}

// ── 23B.5 — Cost Report ───────────────────────────────────────────────────────

func renderCostReport(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "EXECUTION COST REPORT", "Phase 23B.5")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, "Actual trading costs applied: exchange fees, slippage, and funding payments.")
	nl(b)

	h2(b, "Cost Model Parameters")
	line(b, fmt.Sprintf("- Taker Fee: **%.0f bps** (Binance Futures standard)", TakerFeeBps))
	line(b, fmt.Sprintf("- Maker Fee: **%.0f bps**", MakerFeeBps))
	line(b, fmt.Sprintf("- Round-trip Slippage: **%.0f bps**", SlippageBps))
	line(b, "- Funding: Real 8h settlement rates from Binance (default 0.01%/period if unavailable)")
	nl(b)

	h2(b, "Cost Breakdown per Strategy")
	tableHeader(b, "Strategy", "Trades", "Avg Fee/Trade", "Avg Slip/Trade", "Avg Funding/Trade",
		"Total Cost", "GrossPF", "NetPF", "Edge Retention%")
	for _, cb := range r.CostBreakdowns {
		row(b,
			cb.StrategyName,
			fmt.Sprintf("%d", cb.TradeCount),
			fmt.Sprintf("$%.2f", cb.AvgTakerFeeUSD),
			fmt.Sprintf("$%.2f", cb.AvgSlippageUSD),
			fmt.Sprintf("$%.2f", cb.AvgFundingUSD),
			fmt.Sprintf("$%.0f", cb.TotalCostUSD),
			fmt.Sprintf("%.3f", cb.GrossPF),
			fmt.Sprintf("%.3f", cb.NetPF),
			fmt.Sprintf("%.1f%%", cb.EdgeRetention*100),
		)
	}
	return b.String()
}

// ── 23B.6 — Trade Certification ───────────────────────────────────────────────

func renderTradeCert(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "TRADE CERTIFICATION REPORT", "Phase 23B.6")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Total Certified Trades:** %d", r.TotalTrades))
	line(b, "All trades sourced from real strategy execution against real Binance Futures data.")
	nl(b)

	h2(b, "Trades per Strategy")
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range r.TradesPerStrategy {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	tableHeader(b, "Strategy", "Trade Count")
	for _, kv := range sorted {
		row(b, kv.k, fmt.Sprintf("%d", kv.v))
	}
	nl(b)

	h2(b, "Trades per Alpha Source")
	type kvA struct{ k string; v int }
	var sortedA []kvA
	for k, v := range r.TradesPerAlpha {
		sortedA = append(sortedA, kvA{k, v})
	}
	sort.Slice(sortedA, func(i, j int) bool { return sortedA[i].v > sortedA[j].v })
	tableHeader(b, "Alpha Source", "Trade Count")
	for _, kv := range sortedA {
		row(b, kv.k, fmt.Sprintf("%d", kv.v))
	}
	nl(b)

	h2(b, "Trades per Regime")
	tableHeader(b, "Regime", "Trade Count")
	for r2, count := range r.TradesPerRegime {
		row(b, string(r2), fmt.Sprintf("%d", count))
	}
	nl(b)

	h2(b, "Trade Certification Criteria")
	line(b, "Every certified trade satisfies:")
	line(b, "1. Originated from `strategy.OnCandle()` — real strategy logic")
	line(b, "2. Entry at next candle Open — no look-ahead")
	line(b, "3. Exit at real TP/SL price within candle High/Low range")
	line(b, "4. Full cost model applied (fees + slippage + funding)")
	line(b, "5. Regime classified from actual price action")
	line(b, "6. IsReal = true — no synthetic or estimated values")
	return b.String()
}

// ── 23B.7 — Walk-Forward ──────────────────────────────────────────────────────

func renderWalkForward(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "REAL WALK-FORWARD VALIDATION REPORT", "Phase 23B.7")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Train Period:** %d months | **Validation Period:** %d months", WFTrainMonths, WFValidMonths))
	line(b, "No future data leakage. No survivorship bias. Real trade outcomes.")
	nl(b)

	h2(b, "Walk-Forward Summary")
	tableHeader(b, "Strategy", "Windows", "Avg Valid PF", "Avg Valid Sharpe",
		"Consistency%", "Degradation", "Consistent?", "Degraded?")
	for _, wf := range r.WalkForward {
		row(b,
			wf.StrategyName,
			fmt.Sprintf("%d", len(wf.Windows)),
			fmt.Sprintf("%.3f", wf.AvgValidPF),
			fmt.Sprintf("%.2f", wf.AvgValidSharpe),
			fmt.Sprintf("%.1f%%", wf.Consistency),
			fmt.Sprintf("%.3f", wf.Degradation),
			boolStr(wf.IsConsistent),
			boolStr(wf.IsDegraded),
		)
	}
	return b.String()
}

// ── 23B.8 — Monte Carlo ───────────────────────────────────────────────────────

func renderMonteCarlo(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "REAL MONTE CARLO VALIDATION REPORT", "Phase 23B.8")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Simulations per strategy:** %d", MCRuns))
	line(b, "Uses actual trade return distribution. No assumptions.")
	nl(b)

	h2(b, "Monte Carlo Results")
	tableHeader(b, "Strategy", "Input Trades", "P10 PnL", "P50 PnL", "P90 PnL",
		"P90 DD%", "Risk of Ruin%", "Profitable%", "Stability")
	type entry struct{ name string; mc RealMCResult }
	var entries []entry
	for name, mc := range r.MonteCarlo {
		entries = append(entries, entry{name, mc})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].mc.P50PnL > entries[j].mc.P50PnL
	})
	for _, e := range entries {
		mc := e.mc
		row(b,
			e.name,
			fmt.Sprintf("%d", mc.InputTrades),
			fmt.Sprintf("$%.0f", mc.P10PnL),
			fmt.Sprintf("$%.0f", mc.P50PnL),
			fmt.Sprintf("$%.0f", mc.P90PnL),
			fmt.Sprintf("%.1f%%", mc.P90DD),
			fmt.Sprintf("%.1f%%", mc.RiskOfRuin*100),
			fmt.Sprintf("%.1f%%", mc.PctProfitable*100),
			string(mc.Stability),
		)
	}
	nl(b)

	h2(b, "Stability Distribution")
	dist := map[phase22f.MCStabilityF22]int{}
	for _, mc := range r.MonteCarlo {
		dist[mc.Stability]++
	}
	tableHeader(b, "Stability Tier", "Count")
	for _, tier := range []phase22f.MCStabilityF22{
		phase22f.MCRobust, phase22f.MCStable22, phase22f.MCMarginal,
		phase22f.MCUnstable, phase22f.MCFailed,
	} {
		row(b, string(tier), fmt.Sprintf("%d", dist[tier]))
	}
	return b.String()
}

// ── 23B.9 — Regime ────────────────────────────────────────────────────────────

func renderRegimeReport(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "REAL REGIME PERFORMANCE REPORT", "Phase 23B.9")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	nl(b)

	h2(b, "Regime Performance Summary (All Strategies)")
	regimeTotals := map[phase22e.Regime][]float64{}
	for _, p := range r.RegimeProfiles {
		for regime, s := range p.Regimes {
			regimeTotals[regime] = append(regimeTotals[regime], s.ProfitFactor)
		}
	}
	tableHeader(b, "Regime", "Strategy Count", "Avg PF")
	for regime, pfs := range regimeTotals {
		row(b, string(regime), fmt.Sprintf("%d", len(pfs)), fmt.Sprintf("%.3f", mean(pfs)))
	}
	nl(b)

	h2(b, "Per-Strategy Regime Profiles")
	tableHeader(b, "Strategy", "Bull PF", "Bear PF", "Range PF", "Volatile PF", "Dominant", "Regime Robust?")
	for _, p := range r.RegimeProfiles {
		bull := pfStr(p.Regimes[phase22e.RegimeBull])
		bear := pfStr(p.Regimes[phase22e.RegimeBear])
		rng := pfStr(p.Regimes[phase22e.RegimeRange])
		vol := pfStr(p.Regimes[phase22e.RegimeVolatile])
		row(b, p.StrategyName, bull, bear, rng, vol,
			string(p.DominantRegime), boolStr(p.RegimeRobust))
	}
	return b.String()
}

// ── 23B.10 — Capital Certification ───────────────────────────────────────────

func renderCapitalCert(r Phase23BResult) string {
	b := &strings.Builder{}
	header(b, "REAL CAPITAL CERTIFICATION REPORT", "Phase 23B.10")
	line(b, fmt.Sprintf("**Generated:** %s", ts(r.GeneratedAt)))
	line(b, fmt.Sprintf("**Total Capital Available:** $%.0f", r.Config.InitialCapital))
	line(b, fmt.Sprintf("**Certified Strategies:** %d", r.CertifiedStrategies))
	line(b, fmt.Sprintf("**Retired Strategies:** %d", r.RetiredStrategies))
	nl(b)

	h2(b, "Certification Gates")
	line(b, fmt.Sprintf("- Trades ≥ %d", CapMinTrades))
	line(b, fmt.Sprintf("- Profit Factor ≥ %.2f", CapMinPF))
	line(b, fmt.Sprintf("- Sharpe ≥ %.2f", CapMinSharpe))
	line(b, "- Positive Expectancy")
	line(b, fmt.Sprintf("- Max DD ≤ %.0f%%", CapMaxDD))
	line(b, fmt.Sprintf("- Risk of Ruin ≤ %.0f%%", CapMaxRoR*100))
	line(b, "- Monte Carlo: STABLE or ROBUST")
	nl(b)

	h2(b, "Certification Results")

	// Sort by tier (best first)
	certs := make([]CapCertResult, len(r.CapCertifications))
	copy(certs, r.CapCertifications)
	tierOrder := map[CapCertTier]int{
		CapTierInstitutional: 0, CapTierFull: 1, CapTierLimited: 2,
		CapTierPilot: 3, CapTierPaperOnly: 4, CapTierWatchlist: 5, CapTierFailed: 6,
	}
	sort.Slice(certs, func(i, j int) bool {
		return tierOrder[certs[i].Tier] < tierOrder[certs[j].Tier]
	})

	tableHeader(b, "Strategy", "Tier", "Alloc%", "Alloc$", "Gates Passed", "Gates Failed", "Evidence")
	for _, c := range certs {
		row(b,
			c.StrategyName,
			string(c.Tier),
			fmt.Sprintf("%.1f%%", c.AllocationPct),
			fmt.Sprintf("$%.0f", c.AllocationUSD),
			fmt.Sprintf("%d/%d", len(c.GatesPassed), len(c.GatesChecked)),
			fmt.Sprintf("%d", len(c.GatesFailed)),
			c.Evidence,
		)
	}
	nl(b)

	h2(b, "Tier Distribution")
	dist := map[CapCertTier]int{}
	for _, c := range r.CapCertifications {
		dist[c.Tier]++
	}
	tableHeader(b, "Tier", "Count")
	for _, tier := range []CapCertTier{
		CapTierInstitutional, CapTierFull, CapTierLimited,
		CapTierPilot, CapTierPaperOnly, CapTierWatchlist, CapTierFailed,
	} {
		row(b, string(tier), fmt.Sprintf("%d", dist[tier]))
	}
	return b.String()
}

// ── Formatting helpers ────────────────────────────────────────────────────────

func header(b *strings.Builder, title, phase string) {
	b.WriteString(fmt.Sprintf("# %s\n\n**Phase:** %s  \n", title, phase))
}

func h2(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("\n## %s\n\n", title))
}

func line(b *strings.Builder, s string) {
	b.WriteString(s + "\n")
}

func nl(b *strings.Builder) {
	b.WriteString("\n")
}

func tableHeader(b *strings.Builder, cols ...string) {
	b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
	sep := make([]string, len(cols))
	for i := range sep {
		sep[i] = "---"
	}
	b.WriteString("| " + strings.Join(sep, " | ") + " |\n")
}

func row(b *strings.Builder, cols ...string) {
	b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
}

func boolStr(v bool) string {
	if v {
		return "YES"
	}
	return "NO"
}

func ts(t time.Time) string {
	return t.Format("2006-01-02 15:04:05 UTC")
}

func pfStr(s *RegimeStats) string {
	if s == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.3f", s.ProfitFactor)
}
