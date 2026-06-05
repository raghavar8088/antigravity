package phase23a

import (
	"fmt"
	"math"
	"strings"

	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

// CertifyEdge answers all 14 institutional edge questions for each strategy.
func CertifyEdge(
	reports []WalkForwardReport,
	mcResults map[string]phase22f.MonteCarloF22,
	execImpact []ExecutionImpactResult,
	tiers []phase22f.TierClassification,
	totalCapital float64,
) []EdgeCertification {
	// Build lookup maps
	mcMap := mcResults
	execMap := make(map[string]ExecutionImpactResult, len(execImpact))
	for _, ei := range execImpact {
		execMap[ei.StrategyID] = ei
	}
	tierMap := make(map[string]phase22f.TierClassification, len(tiers))
	for _, tc := range tiers {
		tierMap[tc.StrategyID] = tc
	}

	certs := make([]EdgeCertification, 0, len(reports))
	for _, rpt := range reports {
		// gather all validation trades
		var validTrades []phase22e.TradeRecord
		for _, w := range rpt.Windows {
			validTrades = append(validTrades, w.ValidResult.Trades...)
		}

		pf, sharpe, exp, wr := sampleMetrics23(validTrades)
		pnls := make([]float64, len(validTrades))
		for i, t := range validTrades {
			pnls[i] = t.NetPnLUSD
		}
		dd := maxDD23(pnls, totalCapital/20)
		ror := estimateRoR23(pnls, totalCapital/20)
		mc := mcMap[rpt.StrategyID]
		ei := execMap[rpt.StrategyID]
		tc := tierMap[rpt.StrategyID]

		cert := certifyStrategy(rpt, validTrades, pf, sharpe, exp, wr, dd, ror, mc, ei, tc)
		certs = append(certs, cert)
	}
	return certs
}

func certifyStrategy(
	rpt WalkForwardReport,
	trades []phase22e.TradeRecord,
	pf, sharpe, exp, wr, dd, ror float64,
	mc phase22f.MonteCarloF22,
	ei ExecutionImpactResult,
	tc phase22f.TierClassification,
) EdgeCertification {
	cert := EdgeCertification{
		StrategyID:   rpt.StrategyID,
		StrategyName: rpt.StrategyName,
	}

	answers := []EdgeAnswer{
		{
			Question: Q1_StatSig,
			Answer:   pf >= 1.20 && len(trades) >= 30 && isStatSig(trades),
			Evidence: fmt.Sprintf("PF=%.2f n=%d stat_sig=%v", pf, len(trades), isStatSig(trades)),
		},
		{
			Question: Q2_Repeatable,
			Answer:   rpt.IsConsistent,
			Evidence: fmt.Sprintf("WF consistency=%.0f%% (≥60%% required), avg_valid_PF=%.2f", rpt.Consistency, rpt.AvgValidPF),
		},
		{
			Question: Q3_RegimeRobust,
			Answer:   regimeRobustness(trades) >= 3,
			Evidence: fmt.Sprintf("positive PF in %d of 4 basic regimes", regimeRobustness(trades)),
		},
		{
			Question: Q4_WFRobust,
			Answer:   rpt.AvgValidPF >= 1.10 && !rpt.IsDegraded,
			Evidence: fmt.Sprintf("avg_valid_PF=%.2f degradation=%.2f (>-0.20 ok)", rpt.AvgValidPF, rpt.Degradation),
		},
		{
			Question: Q5_MCRobust,
			Answer:   mc.Simulations > 0 && (mc.Stability == phase22f.MCRobust || mc.Stability == phase22f.MCStable22),
			Evidence: fmt.Sprintf("MC stability=%s P(grow)=%.0f%% P(ruin)=%.1f%%", mc.Stability, mc.ProbabilityGrow*100, mc.ProbabilityRuin*100),
		},
		{
			Question: Q6_PostExec,
			Answer:   ei.NetEdgePF >= 1.10 && ei.EdgeRetention >= 0.80,
			Evidence: fmt.Sprintf("gross_PF=%.2f net_PF=%.2f edge_retention=%.0f%%", ei.GrossEdgePF, ei.NetEdgePF, ei.EdgeRetention*100),
		},
		{
			Question: Q7_Drawdown,
			Answer:   dd < 20.0,
			Evidence: fmt.Sprintf("max_drawdown=%.1f%% (<20%% required)", dd),
		},
		{
			Question: Q8_Scale,
			Answer:   pf >= 1.20 && sharpe >= 1.0 && wr >= 0.50,
			Evidence: fmt.Sprintf("PF=%.2f Sharpe=%.2f WR=%.0f%% — scalability determined by risk-adjusted metrics", pf, sharpe, wr*100),
		},
		{
			Question: Q9_Funding,
			Answer:   ei.EdgeRetention >= 0.75,
			Evidence: fmt.Sprintf("total_fees_pct_of_profit=%.1f%%", feesPctOfProfit(trades)),
		},
		{
			Question: Q10_Volatility,
			Answer:   volatileRegimePF(trades) >= 1.0,
			Evidence: fmt.Sprintf("volatile_regime_PF=%.2f", volatileRegimePF(trades)),
		},
		{
			Question: Q11_SampleSize,
			Answer:   len(trades) >= 200,
			Evidence: fmt.Sprintf("n=%d (≥200 required for certification; 1000 for institutional)", len(trades)),
		},
		{
			Question: Q12_RoR,
			Answer:   ror < CapMaxRoR,
			Evidence: fmt.Sprintf("risk_of_ruin=%.1f%% (<%.0f%% required)", ror*100, CapMaxRoR*100),
		},
		{
			Question: Q13_HedgeFund,
			Answer:   pf >= 1.30 && sharpe >= 1.50 && dd < 15 && len(trades) >= 500,
			Evidence: fmt.Sprintf("PF=%.2f Sharpe=%.2f DD=%.1f%% n=%d — institutional threshold: PF≥1.30 Sharpe≥1.50 DD<15%% n≥500", pf, sharpe, dd, len(trades)),
		},
		{
			Question: Q14_Capital,
			Answer:   tc.Tier != phase22f.TierFailed && tc.Tier != phase22f.TierWatchlist && tc.MaxCapitalPct > 0,
			Evidence: fmt.Sprintf("certification_tier=%s max_capital=%.0f%%", tc.Tier, tc.MaxCapitalPct),
		},
	}

	cert.Answers = answers
	for _, a := range answers {
		if a.Answer {
			cert.PassCount++
		} else {
			cert.FailCount++
		}
	}
	cert.Certified = cert.PassCount == 14
	cert.PartialCredit = cert.PassCount >= 10
	cert.Narrative = buildCertNarrative(cert, pf, sharpe, exp, dd)
	return cert
}

// ── helpers ───────────────────────────────────────────────────────────────────

func isStatSig(trades []phase22e.TradeRecord) bool {
	if len(trades) < 30 {
		return false
	}
	wins := 0
	for _, t := range trades {
		if t.NetPnLUSD >= 0 {
			wins++
		}
	}
	n := float64(len(trades))
	p := float64(wins) / n
	se := math.Sqrt(0.5 * 0.5 / n)
	if se == 0 {
		return false
	}
	z := math.Abs((p - 0.5) / se)
	return z > 1.96
}

func regimeRobustness(trades []phase22e.TradeRecord) int {
	type rdata struct{ gw, gl float64 }
	regimes := make(map[phase22e.Regime]*rdata)
	for _, t := range trades {
		d, ok := regimes[t.Regime]
		if !ok {
			d = &rdata{}
			regimes[t.Regime] = d
		}
		if t.NetPnLUSD >= 0 {
			d.gw += t.NetPnLUSD
		} else {
			d.gl += -t.NetPnLUSD
		}
	}
	robust := 0
	for _, d := range regimes {
		if d.gl > 0 && d.gw/d.gl >= 1.0 {
			robust++
		}
	}
	return robust
}

func volatileRegimePF(trades []phase22e.TradeRecord) float64 {
	gw, gl := 0.0, 0.0
	for _, t := range trades {
		if t.Regime == phase22e.RegimeVolatile {
			if t.NetPnLUSD >= 0 {
				gw += t.NetPnLUSD
			} else {
				gl += -t.NetPnLUSD
			}
		}
	}
	if gl == 0 {
		return 1.0 // no volatile trades — assume neutral
	}
	return gw / gl
}

func feesPctOfProfit(trades []phase22e.TradeRecord) float64 {
	totalFees, totalProfit := 0.0, 0.0
	for _, t := range trades {
		totalFees += t.FeesUSD
		if t.GrossPnLUSD > 0 {
			totalProfit += t.GrossPnLUSD
		}
	}
	if totalProfit == 0 {
		return 100
	}
	return totalFees / totalProfit * 100
}

func maxDD23(pnls []float64, nav float64) float64 {
	if len(pnls) == 0 || nav <= 0 {
		return 0
	}
	peak, cum, maxDD := nav, nav, 0.0
	for _, p := range pnls {
		cum += p
		if cum > peak {
			peak = cum
		}
		if peak > 0 {
			dd := (peak - cum) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return maxDD
}

func estimateRoR23(pnls []float64, nav float64) float64 {
	if len(pnls) < 10 || nav <= 0 {
		return 1.0
	}
	wins, losses := 0, 0
	gw, gl := 0.0, 0.0
	for _, p := range pnls {
		if p > 0 {
			wins++
			gw += p
		} else if p < 0 {
			losses++
			gl += -p
		}
	}
	if wins == 0 || losses == 0 {
		return 0
	}
	wr := float64(wins) / float64(len(pnls))
	lr := float64(losses) / float64(len(pnls))
	avgW := gw / float64(wins)
	avgL := gl / float64(losses)
	if avgW == 0 {
		return 1.0
	}
	ratio := (lr * avgL) / (wr * avgW)
	if ratio >= 1 {
		return 1.0
	}
	ruinUnits := nav / avgL
	ror := math.Pow(ratio, ruinUnits)
	if math.IsNaN(ror) || math.IsInf(ror, 0) {
		return 0
	}
	return math.Min(1.0, math.Max(0.0, ror))
}

func buildCertNarrative(cert EdgeCertification, pf, sharpe, exp, dd float64) string {
	b := &strings.Builder{}
	switch {
	case cert.Certified:
		fmt.Fprintf(b, "FULLY CERTIFIED (%d/14): %s demonstrates institutional-grade edge. ", cert.PassCount, cert.StrategyName)
		fmt.Fprintf(b, "PF=%.2f Sharpe=%.2f Exp=$%.0f DD=%.1f%%. All 14 institutional criteria satisfied.", pf, sharpe, exp, dd)
	case cert.PartialCredit:
		fmt.Fprintf(b, "PARTIALLY CERTIFIED (%d/14): %s shows evidence of edge but fails %d criteria. ", cert.PassCount, cert.StrategyName, cert.FailCount)
		failed := make([]string, 0)
		for _, a := range cert.Answers {
			if !a.Answer {
				failed = append(failed, string(a.Question))
			}
		}
		fmt.Fprintf(b, "Failing: %s.", strings.Join(failed, "; "))
	default:
		fmt.Fprintf(b, "NOT CERTIFIED (%d/14): %s lacks sufficient evidence of edge. ", cert.PassCount, cert.StrategyName)
		fmt.Fprintf(b, "PF=%.2f Sharpe=%.2f — does not meet institutional deployment standards.", pf, sharpe)
	}
	return b.String()
}
