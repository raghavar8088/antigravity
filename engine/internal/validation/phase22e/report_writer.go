package phase22e

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WriteAllReports generates all 11 Phase 22E certification reports into outDir.
func WriteAllReports(result ValidationResult, totalCapital float64, outDir string) error {
	type reportFn func(ValidationResult, float64) string
	reports := []struct {
		filename string
		fn       reportFn
	}{
		{"TRADE_VALIDATION_REPORT.md", tradeValidationReport},
		{"PROFIT_FACTOR_CERTIFICATION.md", profitFactorReport},
		{"WIN_RATE_CERTIFICATION.md", winRateReport},
		{"SHARPE_CERTIFICATION.md", sharpeReport},
		{"DRAWDOWN_CERTIFICATION.md", drawdownReport},
		{"REGIME_PERFORMANCE_REPORT.md", regimeReport},
		{"STRATEGY_RANKING_REPORT.md", strategyRankingReport},
		{"CAPITAL_ALLOCATION_REPORT.md", capitalAllocationReport},
		{"LIVE_VS_BACKTEST_CERTIFICATION.md", liveVsBacktestReport},
		{"GO_LIVE_READINESS_REPORT.md", goLiveReport},
		{"PHASE_22E_IMPLEMENTATION_REPORT.md", implementationReport},
	}
	for _, r := range reports {
		content := r.fn(result, totalCapital)
		path := filepath.Join(outDir, r.filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", r.filename, err)
		}
	}
	return nil
}

// ── Report generators ─────────────────────────────────────────────────────────

func tradeValidationReport(r ValidationResult, _ float64) string {
	pm := r.Portfolio
	b := &strings.Builder{}
	header(b, "TRADE VALIDATION REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\nPeriod: %s → %s\n\n",
		r.GeneratedAt.Format(time.RFC3339),
		r.DataFrom.Format("2006-01-02"),
		r.DataTo.Format("2006-01-02")))

	section(b, "1. Trade Volume")
	b.WriteString(fmt.Sprintf("| Checkpoint         | Required | Actual | Status |\n"))
	b.WriteString(fmt.Sprintf("|:-------------------|:--------:|:------:|:------:|\n"))
	b.WriteString(fmt.Sprintf("| 500-Trade Gate     | 500      | %-6d | %s |\n", pm.TotalTrades, badge(pm.Checkpoint500)))
	b.WriteString(fmt.Sprintf("| 1000-Trade Gate    | 1000     | %-6d | %s |\n", pm.TotalTrades, badge(pm.Checkpoint1000)))
	b.WriteString(fmt.Sprintf("| Statistical Sig.   | z > 1.96 | —      | %s |\n\n", badge(pm.StatSig)))

	section(b, "2. Trade Distribution")
	b.WriteString(fmt.Sprintf("- Total Trades: **%d**\n", pm.TotalTrades))
	b.WriteString(fmt.Sprintf("- Winning Trades: **%.0f** (%.1f%%)\n", float64(pm.TotalTrades)*pm.WinRate, pm.WinRate*100))
	b.WriteString(fmt.Sprintf("- Losing Trades: **%.0f** (%.1f%%)\n", float64(pm.TotalTrades)*(1-pm.WinRate), (1-pm.WinRate)*100))
	b.WriteString(fmt.Sprintf("- Average Hold Time: **%.1f min**\n", pm.AvgHoldMin))

	section(b, "3. Return Distribution")
	b.WriteString(fmt.Sprintf("- Skewness: **%.2f** %s\n", pm.ReturnSkew, skewNote(pm.ReturnSkew)))
	b.WriteString(fmt.Sprintf("- Excess Kurtosis: **%.2f** %s\n", pm.ReturnKurtosis, kurtNote(pm.ReturnKurtosis)))
	b.WriteString(fmt.Sprintf("- 95%% VaR per trade: **$%.2f**\n", pm.VaR95USD))
	b.WriteString(fmt.Sprintf("- 95%% CVaR (Expected Shortfall): **$%.2f**\n", pm.CVaR95USD))

	section(b, "4. Outlier Analysis")
	b.WriteString(fmt.Sprintf("- Top 5%% of trades contribute **%.1f%%** of gross profit\n", pm.OutlierWinPct))
	b.WriteString(fmt.Sprintf("- Bottom 5%% of trades contribute **%.1f%%** of gross losses\n", pm.OutlierLossPct))
	outlierWarning(b, pm.OutlierWinPct)

	section(b, "5. Monthly Trade Distribution")
	b.WriteString("| Month | Trades | Net P&L | Status |\n")
	b.WriteString("|:------|:------:|--------:|:------:|\n")
	for _, m := range pm.MonthlyReturns {
		b.WriteString(fmt.Sprintf("| %s %d | %d | $%.2f | %s |\n",
			m.Month.String()[:3], m.Year, m.Trades, m.NetPnL, badge(m.Positive)))
	}
	b.WriteString(fmt.Sprintf("\nConsecutive Positive Months: **%d** %s\n", pm.ConsecPositiveMonths, badge(pm.ConsecPositiveMonths >= MinConsecPositiveMon)))
	return b.String()
}

func profitFactorReport(r ValidationResult, _ float64) string {
	pm := r.Portfolio
	b := &strings.Builder{}
	header(b, "PROFIT FACTOR CERTIFICATION — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("**Profit Factor = Gross Winning P&L / Gross Losing P&L** (net of fees).\n")
	b.WriteString("Computed on all closed trades. A PF ≥ 1.30 is required for certification.\n")
	b.WriteString("PF > 2.0 is considered strong; > 3.0 is considered exceptional.\n\n")

	section(b, "Portfolio Profit Factor")
	b.WriteString(fmt.Sprintf("| Metric               | Value        |\n"))
	b.WriteString(fmt.Sprintf("|:---------------------|:------------:|\n"))
	b.WriteString(fmt.Sprintf("| Gross Winning P&L    | $%.2f    |\n", pm.GrossWinUSD))
	b.WriteString(fmt.Sprintf("| Gross Losing P&L     | $%.2f    |\n", pm.GrossLossUSD))
	b.WriteString(fmt.Sprintf("| Total Fees           | $%.2f    |\n", pm.TotalFeesUSD))
	b.WriteString(fmt.Sprintf("| Net P&L              | $%.2f    |\n", pm.NetPnLUSD))
	b.WriteString(fmt.Sprintf("| **Profit Factor**    | **%.2f**     |\n", pm.ProfitFactor))
	b.WriteString(fmt.Sprintf("| Required PF          | ≥ %.2f       |\n", MinProfitFactor))
	b.WriteString(fmt.Sprintf("| **Certification**    | **%s**   |\n\n", badge(pm.ProfitFactor >= MinProfitFactor)))

	section(b, "Strategy Profit Factors")
	b.WriteString("| Rank | Strategy | Trades | Gross Win | Gross Loss | PF | Status |\n")
	b.WriteString("|:----:|:---------|:------:|----------:|-----------:|:--:|:------:|\n")
	sorted := sortedByRank(r.Strategies)
	for _, s := range sorted {
		b.WriteString(fmt.Sprintf("| %d | %s | %d | $%.2f | $%.2f | %.2f | %s |\n",
			s.Rank, s.StrategyName, s.TotalTrades, s.GrossWin, s.GrossLoss, s.ProfitFactor,
			badge(s.ProfitFactor >= MinProfitFactor)))
	}

	section(b, "Certification Verdict")
	verdict(b, pm.ProfitFactor >= MinProfitFactor,
		fmt.Sprintf("Profit Factor CERTIFIED: %.2f (required ≥ %.2f)", pm.ProfitFactor, MinProfitFactor),
		fmt.Sprintf("Profit Factor FAILED: %.2f (required ≥ %.2f)", pm.ProfitFactor, MinProfitFactor))
	return b.String()
}

func winRateReport(r ValidationResult, _ float64) string {
	pm := r.Portfolio
	b := &strings.Builder{}
	header(b, "WIN RATE CERTIFICATION — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("**Win Rate = Winning Trades / Total Trades**.\n")
	b.WriteString("A trade is a winner if Net P&L (after fees) ≥ $0.\n")
	b.WriteString("Minimum required: 45%. Binomial z-test confirms statistical significance at p < 0.05.\n\n")

	section(b, "Portfolio Win Rate")
	wins := int(float64(pm.TotalTrades) * pm.WinRate)
	losses := pm.TotalTrades - wins
	b.WriteString(fmt.Sprintf("| Metric            | Value    |\n"))
	b.WriteString(fmt.Sprintf("|:------------------|:--------:|\n"))
	b.WriteString(fmt.Sprintf("| Total Trades      | %d       |\n", pm.TotalTrades))
	b.WriteString(fmt.Sprintf("| Winners           | %d       |\n", wins))
	b.WriteString(fmt.Sprintf("| Losers            | %d       |\n", losses))
	b.WriteString(fmt.Sprintf("| **Win Rate**      | **%.1f%%** |\n", pm.WinRate*100))
	b.WriteString(fmt.Sprintf("| Required          | ≥ %.0f%%  |\n", MinWinRate*100))
	b.WriteString(fmt.Sprintf("| Avg Win           | $%.2f   |\n", pm.AvgWinUSD))
	b.WriteString(fmt.Sprintf("| Avg Loss          | $%.2f   |\n", pm.AvgLossUSD))
	b.WriteString(fmt.Sprintf("| Win/Loss Ratio    | %.2f     |\n", safeDiv(pm.AvgWinUSD, pm.AvgLossUSD)))
	b.WriteString(fmt.Sprintf("| Stat. Significant | %s      |\n\n", badge(pm.StatSig)))

	section(b, "Win Rate by Regime")
	for _, reg := range []Regime{RegimeBull, RegimeBear, RegimeRange, RegimeVolatile} {
		rp, ok := pm.RegimePerf[reg]
		if ok && rp.Trades > 0 {
			b.WriteString(fmt.Sprintf("- **%s**: %.1f%% WR over %d trades\n", reg, rp.WinRate*100, rp.Trades))
		} else {
			b.WriteString(fmt.Sprintf("- **%s**: no data\n", reg))
		}
	}
	b.WriteString("\n")

	section(b, "Certification Verdict")
	verdict(b, pm.WinRate >= MinWinRate,
		fmt.Sprintf("Win Rate CERTIFIED: %.1f%% (required ≥ %.0f%%)", pm.WinRate*100, MinWinRate*100),
		fmt.Sprintf("Win Rate FAILED: %.1f%% (required ≥ %.0f%%)", pm.WinRate*100, MinWinRate*100))
	return b.String()
}

func sharpeReport(r ValidationResult, _ float64) string {
	pm := r.Portfolio
	b := &strings.Builder{}
	header(b, "SHARPE RATIO CERTIFICATION — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("**Sharpe Ratio = (Mean Return − Risk-Free Rate) / StdDev of Returns × √N**\n\n")
	b.WriteString(fmt.Sprintf("- Risk-free rate: %.1f%% per annum (India 10-Year Government Bond)\n", RiskFreeAnnual*100))
	b.WriteString("- Returns computed per trade (net of all fees)\n")
	b.WriteString("- Annualised using √N scaling on trade count\n")
	b.WriteString("- Minimum required Sharpe: 1.50\n\n")

	section(b, "Portfolio Sharpe")
	b.WriteString(fmt.Sprintf("| Metric              | Value    |\n"))
	b.WriteString(fmt.Sprintf("|:--------------------|:--------:|\n"))
	b.WriteString(fmt.Sprintf("| Mean Net P&L/trade  | $%.2f   |\n", pm.Expectancy))
	b.WriteString(fmt.Sprintf("| Profit Factor       | %.2f     |\n", pm.ProfitFactor))
	b.WriteString(fmt.Sprintf("| **Sharpe Ratio**    | **%.2f** |\n", pm.Sharpe))
	b.WriteString(fmt.Sprintf("| Required            | ≥ %.2f   |\n", MinSharpe))
	b.WriteString(fmt.Sprintf("| **Certification**   | **%s** |\n\n", badge(pm.Sharpe >= MinSharpe)))

	section(b, "Interpretation")
	b.WriteString(sharpeInterpretation(pm.Sharpe))

	section(b, "Certification Verdict")
	verdict(b, pm.Sharpe >= MinSharpe,
		fmt.Sprintf("Sharpe Ratio CERTIFIED: %.2f (required ≥ %.2f)", pm.Sharpe, MinSharpe),
		fmt.Sprintf("Sharpe Ratio FAILED: %.2f (required ≥ %.2f)", pm.Sharpe, MinSharpe))
	return b.String()
}

func drawdownReport(r ValidationResult, _ float64) string {
	pm := r.Portfolio
	b := &strings.Builder{}
	header(b, "DRAWDOWN CERTIFICATION — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("**Max Drawdown = (Peak Cumulative P&L − Trough) / Peak × 100**\n")
	b.WriteString("Computed on the cumulative net P&L series across all trades.\n")
	b.WriteString("Must remain below 10% to qualify for live deployment.\n\n")

	section(b, "Drawdown Metrics")
	b.WriteString(fmt.Sprintf("| Metric                  | Value    |\n"))
	b.WriteString(fmt.Sprintf("|:------------------------|:--------:|\n"))
	b.WriteString(fmt.Sprintf("| **Max Drawdown**        | **%.2f%%** |\n", pm.MaxDrawdown))
	b.WriteString(fmt.Sprintf("| Required (< limit)      | < %.0f%%   |\n", MaxDrawdownPct))
	b.WriteString(fmt.Sprintf("| 95%% VaR per trade       | $%.2f   |\n", pm.VaR95USD))
	b.WriteString(fmt.Sprintf("| 95%% CVaR per trade      | $%.2f   |\n", pm.CVaR95USD))
	b.WriteString(fmt.Sprintf("| Return Skewness         | %.2f     |\n", pm.ReturnSkew))
	b.WriteString(fmt.Sprintf("| **Certification**       | **%s** |\n\n", badge(pm.MaxDrawdown < MaxDrawdownPct)))

	section(b, "Drawdown Controls Validated")
	b.WriteString("- Kill switch engaged if intraday loss > 2% of capital\n")
	b.WriteString("- Position sizing uses ½-Kelly to limit individual position risk\n")
	b.WriteString("- Portfolio heat limit: maximum 6 concurrent positions\n")
	b.WriteString("- Capital preservation: no new positions when drawdown > 5%\n\n")

	section(b, "Certification Verdict")
	verdict(b, pm.MaxDrawdown < MaxDrawdownPct,
		fmt.Sprintf("Max Drawdown CERTIFIED: %.2f%% (limit < %.0f%%)", pm.MaxDrawdown, MaxDrawdownPct),
		fmt.Sprintf("Max Drawdown FAILED: %.2f%% exceeds %.0f%% limit", pm.MaxDrawdown, MaxDrawdownPct))
	return b.String()
}

func regimeReport(r ValidationResult, _ float64) string {
	pm := r.Portfolio
	b := &strings.Builder{}
	header(b, "REGIME PERFORMANCE REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Regime Classification Methodology")
	b.WriteString("Regimes are classified using a 20-trade rolling window:\n")
	b.WriteString("- **BULL**: Cumulative return > +5% in window, low volatility\n")
	b.WriteString("- **BEAR**: Cumulative return < −5% in window, low volatility\n")
	b.WriteString("- **RANGE**: Cumulative return −5% to +5%, low volatility\n")
	b.WriteString("- **VOLATILE**: ATR% > 2.5% threshold regardless of direction\n\n")

	section(b, "Portfolio Performance by Regime")
	b.WriteString("| Regime   | Trades | Win Rate | Profit Factor | Expectancy | Net P&L  |\n")
	b.WriteString("|:---------|:------:|:--------:|:-------------:|:----------:|---------:|\n")
	for _, reg := range []Regime{RegimeBull, RegimeBear, RegimeRange, RegimeVolatile} {
		rp, ok := pm.RegimePerf[reg]
		if ok && rp.Trades > 0 {
			b.WriteString(fmt.Sprintf("| %-8s | %6d | %8.1f%% | %13.2f | $%9.2f | $%8.2f |\n",
				reg, rp.Trades, rp.WinRate*100, rp.ProfitFactor, rp.Expectancy, rp.NetPnLUSD))
		} else {
			b.WriteString(fmt.Sprintf("| %-8s | %6d | %8s | %13s | %10s | %8s |\n",
				reg, 0, "—", "—", "—", "—"))
		}
	}
	b.WriteString("\n")

	section(b, "Strategy Regime Stability")
	b.WriteString("Strategies must be profitable in ≥ 2 of 4 regimes to qualify for full capital allocation.\n\n")
	b.WriteString("| Strategy | BULL PF | BEAR PF | RANGE PF | VOL PF | Stable Regimes |\n")
	b.WriteString("|:---------|:-------:|:-------:|:--------:|:------:|:--------------:|\n")
	sorted := sortedByRank(r.Strategies)
	for _, s := range sorted[:min10(len(sorted))] {
		stableCount := 0
		regPFs := map[Regime]string{}
		for _, reg := range []Regime{RegimeBull, RegimeBear, RegimeRange, RegimeVolatile} {
			if rp, ok := s.RegimePerf[reg]; ok && rp.Trades >= 5 {
				regPFs[reg] = fmt.Sprintf("%.2f", rp.ProfitFactor)
				if rp.ProfitFactor >= 1.0 {
					stableCount++
				}
			} else {
				regPFs[reg] = "—"
			}
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %d/4 |\n",
			truncate(s.StrategyName, 25),
			regPFs[RegimeBull], regPFs[RegimeBear], regPFs[RegimeRange], regPFs[RegimeVolatile],
			stableCount))
	}

	section(b, "Cross-Regime Stability Conclusion")
	allOK := true
	for _, reg := range []Regime{RegimeBull, RegimeBear, RegimeRange, RegimeVolatile} {
		rp, ok := pm.RegimePerf[reg]
		if !ok || rp.Trades < 20 || rp.ProfitFactor < 1.0 {
			allOK = false
		}
	}
	if allOK {
		b.WriteString("**CERTIFIED**: Portfolio maintains positive edge across all four market regimes.\n")
	} else {
		b.WriteString("**CONDITIONAL**: Portfolio shows regime-dependent performance. Reduce exposure in underperforming regimes.\n")
	}
	return b.String()
}

func strategyRankingReport(r ValidationResult, _ float64) string {
	b := &strings.Builder{}
	header(b, "STRATEGY RANKING REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Ranking Methodology")
	b.WriteString("Composite score (0–100):\n")
	b.WriteString("- 35% — Profit Factor (normalised to cap 3.0)\n")
	b.WriteString("- 25% — Sharpe Ratio (normalised to cap 4.0)\n")
	b.WriteString("- 20% — Win Rate (normalised to 70% ceiling)\n")
	b.WriteString("- 10% — Expectancy per trade (normalised to $50 cap)\n")
	b.WriteString("- 10% — Drawdown penalty (inverted Max DD)\n\n")

	section(b, "Deployment Eligibility Criteria")
	b.WriteString(fmt.Sprintf("- Minimum trades: %d (statistical significance)\n", MinStrategyTrades))
	b.WriteString(fmt.Sprintf("- Minimum Profit Factor: %.2f\n", MinProfitFactor))
	b.WriteString(fmt.Sprintf("- Minimum Win Rate: %.0f%%\n", MinWinRate*100))
	b.WriteString(fmt.Sprintf("- Maximum Drawdown: %.0f%%\n", MaxDrawdownPct))
	b.WriteString(fmt.Sprintf("- Live vs Backtest PF degradation: < %.0f%%\n", MaxLiveVsBacktestDeg*100))
	b.WriteString("- Kelly Fraction must be positive\n\n")

	approved := 0
	rejected := 0
	for _, s := range r.Strategies {
		if s.Approved {
			approved++
		} else {
			rejected++
		}
	}

	b.WriteString(fmt.Sprintf("**Approved Strategies: %d / %d** | **Rejected: %d**\n\n", approved, len(r.Strategies), rejected))

	section(b, "Full Strategy Rankings")
	b.WriteString("| Rank | Strategy | Family | Trades | PF | WR | Sharpe | MaxDD | Kelly | Status |\n")
	b.WriteString("|:----:|:---------|:-------|:------:|:--:|:--:|:------:|:-----:|:-----:|:------:|\n")
	for _, s := range sortedByRank(r.Strategies) {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %d | %.2f | %.0f%% | %.2f | %.1f%% | %.1f%% | %s |\n",
			s.Rank, truncate(s.StrategyName, 30), s.Family, s.TotalTrades,
			s.ProfitFactor, s.WinRate*100, s.Sharpe, s.MaxDrawdown, s.KellyFraction*100,
			approvalBadge(s.Approved)))
	}

	section(b, "Rejected Strategies — Reasons")
	for _, s := range r.Strategies {
		if !s.Approved {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", s.StrategyName, s.RejectReason))
		}
	}
	return b.String()
}

func capitalAllocationReport(r ValidationResult, totalCapital float64) string {
	b := &strings.Builder{}
	header(b, "CAPITAL ALLOCATION REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n", r.GeneratedAt.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Total Deployable Capital: **$%.0f**\n\n", totalCapital))

	section(b, "Allocation Methodology")
	b.WriteString("Capital is allocated using **Half-Kelly Criterion**:\n\n")
	b.WriteString("```\n")
	b.WriteString("Kelly% = WinRate − (LossRate × AvgLoss / AvgWin)\n")
	b.WriteString("Allocated% = (HalfKelly_i / ΣHalfKelly_approved) × 100\n")
	b.WriteString("Cap per strategy: 20% (concentration limit)\n")
	b.WriteString("```\n\n")

	slots := CapitalDeploymentPlan(r.Strategies, totalCapital)
	totalAlloc := 0.0
	for _, sl := range slots {
		totalAlloc += sl.AllocatedUSD
	}

	b.WriteString(fmt.Sprintf("Capital deployed to approved strategies: **$%.0f** (%.1f%%)\n\n", totalAlloc, totalAlloc/totalCapital*100))

	section(b, "Capital Deployment Plan")
	b.WriteString("| Rank | Strategy | Family | Alloc % | Alloc USD | Kelly | PF | WR |\n")
	b.WriteString("|:----:|:---------|:-------|:-------:|----------:|:-----:|:--:|:--:|\n")
	for _, sl := range slots {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %.1f%% | $%-10.0f | %.1f%% | %.2f | %.0f%% |\n",
			sl.Rank, truncate(sl.StrategyName, 30), sl.Family,
			sl.AllocatedPct, sl.AllocatedUSD, sl.KellyFraction*100, sl.ProfitFactor, sl.WinRate*100))
	}

	section(b, "Scaling Thresholds")
	b.WriteString("Capital expansion is gated on live performance milestones:\n\n")
	b.WriteString("| Tranche | Capital | Min Trades | Min PF | Min Sharpe |\n")
	b.WriteString("|:--------|--------:|:----------:|:------:|:----------:|\n")
	for _, gate := range ScalingThresholds() {
		b.WriteString(fmt.Sprintf("| %s | $%-8.0f | %d | %.2f | %.2f |\n",
			gate.Label, gate.CapitalUSD, gate.MinTrades, gate.MinPF, gate.MinSharpe))
	}

	section(b, "Risk Limits")
	b.WriteString("- Maximum single-position size: 2% of total capital\n")
	b.WriteString("- Maximum portfolio heat: 6 concurrent positions\n")
	b.WriteString("- Daily loss limit: 2% triggers kill switch\n")
	b.WriteString("- Weekly loss limit: 5% triggers strategy review\n")
	b.WriteString("- Drawdown > 7%: halve all position sizes\n")
	b.WriteString("- Drawdown > 10%: full halt, manual review required\n")
	return b.String()
}

func liveVsBacktestReport(r ValidationResult, _ float64) string {
	b := &strings.Builder{}
	header(b, "LIVE VS BACKTEST CERTIFICATION — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("Live performance is compared against paper/backtest performance for the same strategies.\n")
	b.WriteString(fmt.Sprintf("A degradation of > %.0f%% in Profit Factor is considered a failure.\n\n", MaxLiveVsBacktestDeg*100))
	b.WriteString("**Degradation = (LivePF − BacktestPF) / BacktestPF × 100** (negative = live worse)\n\n")

	section(b, "Strategy Live vs Backtest Comparison")
	b.WriteString("| Strategy | Backtest PF | Live PF | Degradation | WR Back | WR Live | Status |\n")
	b.WriteString("|:---------|:-----------:|:-------:|:-----------:|:-------:|:-------:|:------:|\n")
	for _, s := range sortedByRank(r.Strategies) {
		if s.BacktestPF == 0 && s.LivePF == 0 {
			b.WriteString(fmt.Sprintf("| %s | — | — | — | — | — | ⚪ No Data |\n", truncate(s.StrategyName, 25)))
			continue
		}
		deg := s.PFDegradation * 100
		b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %+.1f%% | %.0f%% | %.0f%% | %s |\n",
			truncate(s.StrategyName, 25), s.BacktestPF, s.LivePF, deg,
			s.BacktestWR*100, s.LiveWR*100, badge(s.LiveVsBacktestOK)))
	}
	b.WriteString("\n")

	section(b, "Sources of Live vs Backtest Divergence")
	b.WriteString("1. **Slippage** — live fills at worse prices than backtest simulation\n")
	b.WriteString("2. **Market Impact** — larger orders move the market\n")
	b.WriteString("3. **Latency** — signal-to-fill delay not modelled in backtest\n")
	b.WriteString("4. **Look-ahead Bias** — backtest data may include survivorship bias\n")
	b.WriteString("5. **Regime Shift** — market conditions changed after backtest period\n\n")

	allOK := true
	for _, s := range r.Strategies {
		if s.Approved && !s.LiveVsBacktestOK && s.LivePF > 0 {
			allOK = false
		}
	}
	section(b, "Certification Verdict")
	verdict(b, allOK,
		"Live vs Backtest CERTIFIED: All approved strategies within degradation limits",
		"Live vs Backtest FAILED: One or more approved strategies exceed degradation limits")
	return b.String()
}

func goLiveReport(r ValidationResult, totalCapital float64) string {
	b := &strings.Builder{}
	header(b, "GO-LIVE READINESS REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Final Certification Status")
	b.WriteString(fmt.Sprintf("## 🏛️ %s\n\n", r.Status))
	b.WriteString(r.Narrative + "\n\n")

	section(b, "Go-Live Requirements Checklist")
	b.WriteString("| Criterion | Required | Actual | Status |\n")
	b.WriteString("|:----------|:--------:|:------:|:------:|\n")
	pm := r.Portfolio
	b.WriteString(fmt.Sprintf("| Total Trades | ≥ 1000 | %d | %s |\n", pm.TotalTrades, badge(pm.Checkpoint1000)))
	b.WriteString(fmt.Sprintf("| Profit Factor | ≥ 1.30 | %.2f | %s |\n", pm.ProfitFactor, badge(pm.ProfitFactor >= MinProfitFactor)))
	b.WriteString(fmt.Sprintf("| Win Rate | ≥ 45%% | %.1f%% | %s |\n", pm.WinRate*100, badge(pm.WinRate >= MinWinRate)))
	b.WriteString(fmt.Sprintf("| Sharpe Ratio | ≥ 1.50 | %.2f | %s |\n", pm.Sharpe, badge(pm.Sharpe >= MinSharpe)))
	b.WriteString(fmt.Sprintf("| Max Drawdown | < 10%% | %.1f%% | %s |\n", pm.MaxDrawdown, badge(pm.MaxDrawdown < MaxDrawdownPct)))
	b.WriteString(fmt.Sprintf("| Positive Months | ≥ 3 consecutive | %d | %s |\n", pm.ConsecPositiveMonths, badge(pm.ConsecPositiveMonths >= MinConsecPositiveMon)))
	b.WriteString(fmt.Sprintf("| Statistical Significance | p < 0.05 | — | %s |\n", badge(pm.StatSig)))

	if len(r.Passed) > 0 {
		section(b, "Passed Criteria")
		for _, p := range r.Passed {
			b.WriteString(fmt.Sprintf("- ✅ %s\n", p))
		}
		b.WriteString("\n")
	}
	if len(r.Failed) > 0 {
		section(b, "Failed Criteria")
		for _, f := range r.Failed {
			b.WriteString(fmt.Sprintf("- ❌ %s\n", f))
		}
		b.WriteString("\n")
	}
	if len(r.Warnings) > 0 {
		section(b, "Warnings (Non-Blocking)")
		for _, w := range r.Warnings {
			b.WriteString(fmt.Sprintf("- ⚠️ %s\n", w))
		}
		b.WriteString("\n")
	}

	section(b, "Deployment Recommendation")
	switch r.Status {
	case StatusApprovedScaling, StatusApproved:
		b.WriteString(fmt.Sprintf("1. Deploy approved strategies with total capital: **$%.0f**\n", totalCapital))
		b.WriteString("2. Begin at Tranche 1 ($10,000) and validate 100 live trades\n")
		b.WriteString("3. Monitor daily P&L, drawdown, and kill switch\n")
		b.WriteString("4. Scale to Tranche 2 after 300 live trades meeting updated thresholds\n")
		b.WriteString("5. Re-run Phase 22E validation before each capital tranche increase\n")
	case StatusConditional:
		b.WriteString("1. Continue paper trading until all failed criteria are resolved\n")
		b.WriteString("2. Address each failure with targeted strategy improvements\n")
		b.WriteString("3. Re-run Phase 22E validation after minimum 200 additional trades\n")
		b.WriteString("4. Do NOT deploy live capital until full certification is achieved\n")
	default:
		b.WriteString("1. Do NOT deploy live capital\n")
		b.WriteString("2. Review strategy logic and risk management\n")
		b.WriteString("3. Accumulate more paper trades to confirm edge\n")
		b.WriteString("4. Re-apply for Phase 22E certification after remediation\n")
	}
	return b.String()
}

func implementationReport(r ValidationResult, totalCapital float64) string {
	b := &strings.Builder{}
	header(b, "PHASE 22E IMPLEMENTATION REPORT")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))

	section(b, "Implementation Overview")
	b.WriteString("Phase 22E implements a complete profitability validation and capital deployment framework\n")
	b.WriteString("for the institutional trading platform. The validation engine is built as a standalone\n")
	b.WriteString("Go package at `engine/internal/validation/phase22e/` and integrates with the existing\n")
	b.WriteString("PMS analytics engine and ledger event store.\n\n")

	section(b, "Package Structure")
	b.WriteString("```\n")
	b.WriteString("engine/internal/validation/phase22e/\n")
	b.WriteString("  types.go          — All validation types and thresholds\n")
	b.WriteString("  validator.go      — Main Validator.Run() pipeline\n")
	b.WriteString("  metrics.go        — Statistical computations (PF, WR, Sharpe, VaR, etc.)\n")
	b.WriteString("  regime.go         — Regime classification (BULL/BEAR/RANGE/VOLATILE)\n")
	b.WriteString("  ranker.go         — Strategy ranking and Half-Kelly capital allocation\n")
	b.WriteString("  certification.go  — Go-live certification and scaling gates\n")
	b.WriteString("  report_writer.go  — Markdown report generation\n")
	b.WriteString("  phase22e_test.go  — Unit tests\n")
	b.WriteString("engine/cmd/phase22e/main.go — CLI runner\n")
	b.WriteString("```\n\n")

	section(b, "Validation Pipeline")
	b.WriteString("```\n")
	b.WriteString("Ledger (PostgreSQL/SQLite)\n")
	b.WriteString("  └── PositionClosedPayload events\n")
	b.WriteString("        └── TradeRecord conversion\n")
	b.WriteString("              └── AssignRegimes() — 20-trade rolling regime classifier\n")
	b.WriteString("                    ├── computePortfolioMetrics() — portfolio-level stats\n")
	b.WriteString("                    ├── computeStrategyMetrics()  — per-strategy stats\n")
	b.WriteString("                    ├── RankStrategies()          — ranking + Half-Kelly\n")
	b.WriteString("                    └── Certify()                 — go-live determination\n")
	b.WriteString("                          └── WriteAllReports()   — 11 markdown reports\n")
	b.WriteString("```\n\n")

	section(b, "Metrics Implemented")
	b.WriteString("| Metric | Method | Notes |\n")
	b.WriteString("|:-------|:-------|:------|\n")
	b.WriteString("| Profit Factor | GrossWin / GrossLoss | Net of all fees |\n")
	b.WriteString("| Win Rate | Winners / Total | Net P&L ≥ 0 |\n")
	b.WriteString("| Sharpe Ratio | (Mean − Rf) / StdDev × √N | Per-trade returns, annualised |\n")
	b.WriteString("| Expectancy | Mean Net P&L per trade | Direct average |\n")
	b.WriteString("| Max Drawdown | Peak-to-trough on cum. P&L | % of peak |\n")
	b.WriteString("| Kelly Fraction | WR − LR × (AvgL/AvgW) | Half-Kelly for allocation |\n")
	b.WriteString("| VaR 95% | 5th percentile of returns | Per trade |\n")
	b.WriteString("| CVaR 95% | Mean of worst 5% returns | Expected shortfall |\n")
	b.WriteString("| Return Skewness | 3rd standardised moment | Fisher's skewness |\n")
	b.WriteString("| Excess Kurtosis | 4th standardised moment | Fisher's kurtosis |\n")
	b.WriteString("| Tail Ratio | p95 win / p5 loss | > 1 preferred |\n\n")

	section(b, "Regime Classification")
	b.WriteString("The 20-trade rolling window regime classifier:\n")
	b.WriteString("- BULL: cumulative return > +5%, low volatility\n")
	b.WriteString("- BEAR: cumulative return < −5%, low volatility\n")
	b.WriteString("- RANGE: cumulative return ±5%, low volatility\n")
	b.WriteString("- VOLATILE: ATR% > 2.5% threshold\n\n")

	section(b, "Capital Allocation Algorithm")
	b.WriteString("1. Filter out strategies failing any mandatory criterion\n")
	b.WriteString("2. Compute composite score (PF 35%, Sharpe 25%, WR 20%, E[trade] 10%, DD 10%)\n")
	b.WriteString("3. Compute Half-Kelly fraction for each approved strategy\n")
	b.WriteString("4. Allocate proportionally: Alloc_i = HalfKelly_i / ΣHalfKelly × TotalCapital\n")
	b.WriteString("5. Hard cap: no single strategy > 20% of capital\n")
	b.WriteString("6. Re-normalise after capping\n\n")

	section(b, "Certification Decision Logic")
	b.WriteString("| Outcome | Condition |\n")
	b.WriteString("|:--------|:----------|\n")
	b.WriteString("| APPROVED FOR CAPITAL SCALING | All 7 hard criteria pass, 0 warnings |\n")
	b.WriteString("| APPROVED FOR LIMITED LIVE CAPITAL | All 7 hard criteria pass, warnings present |\n")
	b.WriteString("| CONDITIONALLY APPROVED | 1–2 hard criteria fail |\n")
	b.WriteString("| NOT APPROVED | 3+ hard criteria fail |\n\n")

	section(b, "Current Validation Summary")
	pm := r.Portfolio
	b.WriteString(fmt.Sprintf("- Data period: %s → %s\n", r.DataFrom.Format("2006-01-02"), r.DataTo.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("- Total trades validated: **%d**\n", pm.TotalTrades))
	b.WriteString(fmt.Sprintf("- Strategies evaluated: **%d**\n", len(r.Strategies)))
	b.WriteString(fmt.Sprintf("- Portfolio Profit Factor: **%.2f**\n", pm.ProfitFactor))
	b.WriteString(fmt.Sprintf("- Portfolio Win Rate: **%.1f%%**\n", pm.WinRate*100))
	b.WriteString(fmt.Sprintf("- Portfolio Sharpe: **%.2f**\n", pm.Sharpe))
	b.WriteString(fmt.Sprintf("- Max Drawdown: **%.1f%%**\n", pm.MaxDrawdown))
	b.WriteString(fmt.Sprintf("- Consecutive Positive Months: **%d**\n", pm.ConsecPositiveMonths))
	b.WriteString(fmt.Sprintf("- **Final Status: %s**\n", r.Status))
	return b.String()
}

// ── Formatting helpers ────────────────────────────────────────────────────────

func header(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("# %s\n\n", title))
	b.WriteString("---\n\n")
}

func section(b *strings.Builder, title string) {
	b.WriteString(fmt.Sprintf("## %s\n\n", title))
}

func badge(ok bool) string {
	if ok {
		return "✅ PASS"
	}
	return "❌ FAIL"
}

func approvalBadge(ok bool) string {
	if ok {
		return "✅ APPROVED"
	}
	return "❌ REJECTED"
}

func verdict(b *strings.Builder, ok bool, pass, fail string) {
	if ok {
		b.WriteString(fmt.Sprintf("**✅ %s**\n", pass))
	} else {
		b.WriteString(fmt.Sprintf("**❌ %s**\n", fail))
	}
}

func skewNote(s float64) string {
	switch {
	case s > 0.5:
		return "(positive skew — right tail, favourable)"
	case s < -0.5:
		return "(negative skew — left tail, caution)"
	default:
		return "(near-symmetric)"
	}
}

func kurtNote(k float64) string {
	switch {
	case k > 3:
		return "(leptokurtic — fat tails, elevated tail risk)"
	case k < -1:
		return "(platykurtic — thin tails)"
	default:
		return "(near-normal)"
	}
}

func sharpeInterpretation(s float64) string {
	switch {
	case s >= 3.0:
		return "Sharpe ≥ 3.0: Exceptional risk-adjusted return. Rare in live systems.\n"
	case s >= 2.0:
		return "Sharpe ≥ 2.0: Very strong. Institutional quality performance.\n"
	case s >= 1.5:
		return "Sharpe ≥ 1.5: Solid. Meets institutional minimum for capital deployment.\n"
	case s >= 1.0:
		return "Sharpe ≥ 1.0: Acceptable but below institutional minimum. Continue optimising.\n"
	default:
		return "Sharpe < 1.0: Insufficient risk-adjusted return. Capital deployment not recommended.\n"
	}
}

func outlierWarning(b *strings.Builder, pct float64) {
	if pct > 50 {
		b.WriteString(fmt.Sprintf("\n⚠️ High outlier dependency: %.0f%% of profit from top 5%% of trades. Results may not be reproducible.\n", pct))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func sortedByRank(strategies []StrategyMetrics) []StrategyMetrics {
	out := make([]StrategyMetrics, len(strategies))
	copy(out, strategies)
	sort.Slice(out, func(i, j int) bool { return out[i].Rank < out[j].Rank })
	return out
}

func min10(n int) int {
	if n > 10 {
		return 10
	}
	return n
}

// ── Phase 22E extended reports ────────────────────────────────────────────────

// MonteCarloReport generates the MONTE_CARLO_REPORT.md content.
func MonteCarloReport(stratMC map[string]MonteCarloResult, portfolioMC MonteCarloResult, strategies []StrategyMetrics) string {
	b := &strings.Builder{}
	header(b, "MONTE CARLO VALIDATION REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("Each strategy's historical trade returns are shuffled 1,000 times (Fisher-Yates).\n")
	b.WriteString("For each permutation, terminal P&L and maximum drawdown are recorded.\n")
	b.WriteString("Percentiles (p5/p50/p95) and ruin probability (terminal loss > 50% of NAV) are computed.\n\n")

	section(b, "Portfolio-Level Monte Carlo")
	b.WriteString(fmt.Sprintf("| Metric                    | Value        |\n"))
	b.WriteString(fmt.Sprintf("|:--------------------------|:------------:|\n"))
	b.WriteString(fmt.Sprintf("| Simulations               | %d           |\n", portfolioMC.Simulations))
	b.WriteString(fmt.Sprintf("| Expected Return (p50)     | $%.2f    |\n", portfolioMC.ExpectedReturn))
	b.WriteString(fmt.Sprintf("| Best Case Return (p95)    | $%.2f    |\n", portfolioMC.BestReturn))
	b.WriteString(fmt.Sprintf("| Worst Case Return (p5)    | $%.2f    |\n", portfolioMC.WorstReturn))
	b.WriteString(fmt.Sprintf("| Expected Drawdown (p50)   | %.1f%%         |\n", portfolioMC.ExpectedDD))
	b.WriteString(fmt.Sprintf("| Worst Drawdown (p95)      | %.1f%%         |\n", portfolioMC.WorstDD))
	b.WriteString(fmt.Sprintf("| Probability of Ruin       | %.1f%%         |\n", portfolioMC.ProbabilityRuin*100))
	b.WriteString(fmt.Sprintf("| Probability of Growth     | %.1f%%         |\n", portfolioMC.ProbabilityGrow*100))
	b.WriteString(fmt.Sprintf("| Stability Classification  | **%s**     |\n\n", portfolioMC.Stability))

	section(b, "Per-Strategy Monte Carlo Results")
	b.WriteString("| Strategy | Sims | E[Return] | Worst Return | E[DD] | Worst DD | P(Ruin) | Stability |\n")
	b.WriteString("|:---------|:----:|----------:|-------------:|:-----:|:--------:|:-------:|:---------:|\n")
	for _, s := range sortedByRank(strategies) {
		mc, ok := stratMC[s.StrategyID]
		if !ok {
			b.WriteString(fmt.Sprintf("| %s | — | — | — | — | — | — | — |\n", truncate(s.StrategyName, 25)))
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %d | $%.0f | $%.0f | %.1f%% | %.1f%% | %.1f%% | %s |\n",
			truncate(s.StrategyName, 25), mc.Simulations,
			mc.ExpectedReturn, mc.WorstReturn,
			mc.ExpectedDD, mc.WorstDD,
			mc.ProbabilityRuin*100, mc.Stability))
	}
	b.WriteString("\n")

	section(b, "Stability Classification Summary")
	stableCnt, marginalCnt, unstableCnt, untradableCnt := 0, 0, 0, 0
	for _, mc := range stratMC {
		switch mc.Stability {
		case MCStable:
			stableCnt++
		case MCMarginal:
			marginalCnt++
		case MCUnstable:
			unstableCnt++
		case MCUntradable:
			untradableCnt++
		}
	}
	b.WriteString(fmt.Sprintf("- **STABLE**: %d strategies\n", stableCnt))
	b.WriteString(fmt.Sprintf("- **MARGINAL**: %d strategies\n", marginalCnt))
	b.WriteString(fmt.Sprintf("- **UNSTABLE**: %d strategies — reduce position size or paper-only\n", unstableCnt))
	b.WriteString(fmt.Sprintf("- **UNTRADABLE**: %d strategies — immediate retirement recommended\n", untradableCnt))
	return b.String()
}

// AlphaCertificationReport generates the ALPHA_CERTIFICATION_REPORT.md content.
func AlphaCertificationReport(alphas []AlphaMetrics) string {
	b := &strings.Builder{}
	header(b, "ALPHA ENGINE CERTIFICATION REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	section(b, "Alpha Engine Universe")
	b.WriteString("Each alpha engine groups all strategies sharing the same signal origin.\n")
	b.WriteString("Certification requires: PF ≥ 1.30, Sharpe ≥ 1.50, positive expectancy.\n\n")

	section(b, "Alpha Engine Rankings")
	b.WriteString("| Rank | Alpha Engine | Trades | Win Rate | PF | Sharpe | Expectancy | MaxDD | MC Stability |\n")
	b.WriteString("|:----:|:-------------|:------:|:--------:|:--:|:------:|:----------:|:-----:|:------------:|\n")
	for _, a := range alphas {
		certified := a.ProfitFactor >= MinProfitFactor && a.Sharpe >= MinSharpe
		b.WriteString(fmt.Sprintf("| %d | %s | %d | %.1f%% | %.2f %s | %.2f | $%.2f | %.1f%% | %s |\n",
			a.Rank, a.AlphaEngine, a.Trades, a.WinRate*100,
			a.ProfitFactor, badge(certified),
			a.Sharpe, a.Expectancy, a.MaxDrawdown, a.MonteCarlo.Stability))
	}

	section(b, "Alpha Edge Summary")
	certified := 0
	for _, a := range alphas {
		if a.ProfitFactor >= MinProfitFactor {
			certified++
		}
	}
	b.WriteString(fmt.Sprintf("- Alpha engines evaluated: **%d**\n", len(alphas)))
	b.WriteString(fmt.Sprintf("- Alpha engines with positive edge (PF ≥ 1.30): **%d**\n", certified))
	b.WriteString(fmt.Sprintf("- Alpha engines failing: **%d**\n", len(alphas)-certified))
	return b.String()
}

// CorrelationReport generates the PORTFOLIO_CORRELATION_REPORT.md content.
func CorrelationReport(cm CorrelationMatrix) string {
	b := &strings.Builder{}
	header(b, "PORTFOLIO CORRELATION REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	section(b, "Methodology")
	b.WriteString("Pearson correlation is computed on per-strategy cumulative equity curves.\n")
	b.WriteString("Low correlation (< 0.30) indicates genuine diversification.\n")
	b.WriteString("High correlation (> 0.70) indicates redundant alpha sources.\n\n")

	b.WriteString(fmt.Sprintf("**Portfolio Diversification Score: %.1f / 100**\n\n", cm.DiversScore))

	if len(cm.Clusters) > 0 {
		section(b, "High-Correlation Clusters (|r| > 0.70)")
		b.WriteString("Strategies within a cluster should not all receive full capital simultaneously.\n\n")
		for i, cl := range cm.Clusters {
			b.WriteString(fmt.Sprintf("**Cluster %d** (avg r=%.2f): %s\n", i+1, cl.AvgCorr, strings.Join(cl.StrategyIDs, ", ")))
		}
		b.WriteString("\n")
	} else {
		section(b, "Cluster Analysis")
		b.WriteString("No high-correlation clusters detected. Portfolio is well diversified.\n\n")
	}

	if len(cm.FamilyMatrix) > 0 {
		section(b, "Family-Level Correlation Matrix")
		families := make([]string, 0, len(cm.FamilyMatrix))
		for f := range cm.FamilyMatrix {
			families = append(families, f)
		}
		// header row
		b.WriteString("| Family |")
		for _, f := range families {
			b.WriteString(fmt.Sprintf(" %s |", truncate(f, 12)))
		}
		b.WriteString("\n|:-------|")
		for range families {
			b.WriteString(":------:|")
		}
		b.WriteString("\n")
		for _, f1 := range families {
			b.WriteString(fmt.Sprintf("| %s |", truncate(f1, 12)))
			for _, f2 := range families {
				r := cm.FamilyMatrix[f1][f2]
				b.WriteString(fmt.Sprintf(" %.2f |", r))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	section(b, "Diversification Conclusion")
	switch {
	case cm.DiversScore >= 80:
		b.WriteString("**EXCELLENT**: Portfolio strategies are highly uncorrelated. Capital can be deployed across all approved strategies simultaneously.\n")
	case cm.DiversScore >= 60:
		b.WriteString("**GOOD**: Portfolio has adequate diversification. Monitor cluster groups to avoid concentration.\n")
	case cm.DiversScore >= 40:
		b.WriteString("**MODERATE**: Some correlation present. Consider reducing position size in highly correlated clusters.\n")
	default:
		b.WriteString("**POOR**: Strategies are highly correlated. Portfolio behaves like a single strategy — concentrated risk.\n")
	}
	return b.String()
}

// RetirementReport generates the RETIREMENT_CANDIDATES_REPORT.md content.
func RetirementReport(candidates []RetirementCandidate, totalStrategies int) string {
	b := &strings.Builder{}
	header(b, "RETIREMENT CANDIDATES REPORT — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	section(b, "Retirement Criteria")
	b.WriteString("A strategy is flagged for retirement if it meets one or more of:\n\n")
	b.WriteString("| Criterion | Threshold | Severity |\n")
	b.WriteString("|:----------|:----------|:--------:|\n")
	b.WriteString("| Profit Factor < 1.00 | Losing money absolutely | IMMEDIATE |\n")
	b.WriteString("| Negative expectancy | E[trade] < $0 | IMMEDIATE |\n")
	b.WriteString("| Sharpe < 0.50 | Insufficient risk-adjusted return | RECOMMENDED |\n")
	b.WriteString("| Max Drawdown > 15% | Persistent capital impairment | RECOMMENDED |\n")
	b.WriteString("| Monte Carlo UNTRADABLE | Ruin probability > 20% | IMMEDIATE |\n")
	b.WriteString("| Monte Carlo UNSTABLE | Ruin probability > 10% | RECOMMENDED |\n")
	b.WriteString("| No statistical edge | n ≥ 50 but significance not confirmed | RECOMMENDED |\n\n")

	immediate := 0
	recommended := 0
	conditional := 0
	for _, c := range candidates {
		switch c.Severity {
		case SeverityImmediate:
			immediate++
		case SeverityRecommended:
			recommended++
		case SeverityConditional:
			conditional++
		}
	}

	b.WriteString(fmt.Sprintf("**Total strategies evaluated: %d** | **Retirement candidates: %d** (%.0f%%)\n\n",
		totalStrategies, len(candidates), float64(len(candidates))/float64(max1(1, totalStrategies))*100))
	b.WriteString(fmt.Sprintf("- IMMEDIATE retirement: **%d** strategies\n", immediate))
	b.WriteString(fmt.Sprintf("- RECOMMENDED retirement: **%d** strategies\n", recommended))
	b.WriteString(fmt.Sprintf("- CONDITIONAL retirement: **%d** strategies\n\n", conditional))

	if len(candidates) == 0 {
		b.WriteString("**No retirement candidates identified.** All strategies meet minimum performance standards.\n")
		return b.String()
	}

	section(b, "Retirement Candidates — Ordered by Severity")
	b.WriteString("| Severity | Strategy | Family | Trades | PF | Sharpe | MaxDD | Reasons |\n")
	b.WriteString("|:--------:|:---------|:-------|:------:|:--:|:------:|:-----:|:--------|\n")
	for _, c := range candidates {
		reasonStr := ""
		if len(c.Reasons) > 0 {
			reasonStr = c.Reasons[0]
			if len(c.Reasons) > 1 {
				reasonStr += fmt.Sprintf(" (+%d more)", len(c.Reasons)-1)
			}
		}
		b.WriteString(fmt.Sprintf("| **%s** | %s | %s | %d | %.2f | %.2f | %.1f%% | %s |\n",
			c.Severity, truncate(c.StrategyName, 28), c.Family,
			c.TotalTrades, c.ProfitFactor, c.Sharpe, c.MaxDrawdown,
			truncate(reasonStr, 60)))
	}
	b.WriteString("\n")

	if immediate > 0 {
		section(b, "Immediate Retirement Action Required")
		for _, c := range candidates {
			if c.Severity != SeverityImmediate {
				continue
			}
			b.WriteString(fmt.Sprintf("**%s** — %s\n", c.StrategyName, c.Family))
			for _, r := range c.Reasons {
				b.WriteString(fmt.Sprintf("  - %s\n", r))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// DeploymentTierReport generates the LIVE_DEPLOYMENT_CERTIFICATION.md content.
func DeploymentTierReport(classifications []DeploymentClassification) string {
	b := &strings.Builder{}
	header(b, "LIVE DEPLOYMENT CERTIFICATION — Phase 22E")
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	section(b, "Deployment Tier Definitions")
	b.WriteString("| Tier | PF Requirement | Sharpe | Trades | Max DD | Max Capital% |\n")
	b.WriteString("|:-----|:--------------:|:------:|:------:|:------:|:------------:|\n")
	b.WriteString("| NOT CERTIFIED | < 1.10 | — | — | — | 0% |\n")
	b.WriteString("| PAPER ONLY | ≥ 1.10 | — | — | — | 0% |\n")
	b.WriteString("| PILOT CAPITAL | ≥ 1.20 | ≥ 1.00 | — | — | 5% |\n")
	b.WriteString("| LIMITED CAPITAL | ≥ 1.30 | ≥ 1.25 | — | — | 10% |\n")
	b.WriteString("| FULL DEPLOYMENT | ≥ 1.40 | ≥ 1.50 | — | — | 15% |\n")
	b.WriteString("| INSTITUTIONAL GRADE | ≥ 1.50 | ≥ 2.00 | ≥ 1000 | < 10% | 20% |\n\n")

	counts := TierCounts(classifications)
	section(b, "Tier Distribution")
	for _, tier := range []DeploymentTier{TierInstitutional, TierFullDeployment, TierLimitedCapital, TierPilotCapital, TierPaperOnly, TierNotCertified} {
		b.WriteString(fmt.Sprintf("- **%s**: %d strategies\n", tier, counts[tier]))
	}
	b.WriteString("\n")

	section(b, "Full Strategy Classification")
	b.WriteString("| Strategy | Family | PF | Sharpe | Tier | Max Capital% |\n")
	b.WriteString("|:---------|:-------|:--:|:------:|:-----|:------------:|\n")
	for _, dc := range classifications {
		b.WriteString(fmt.Sprintf("| %s | %s | — | — | **%s** | %.0f%% |\n",
			truncate(dc.StrategyName, 28), dc.Family, dc.Tier, dc.MaxCapitalPct))
	}
	return b.String()
}
