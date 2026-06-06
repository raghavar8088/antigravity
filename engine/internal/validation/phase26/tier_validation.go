package phase26

import (
	"fmt"

	"antigravity-engine/internal/validation/phase24"
	"antigravity-engine/internal/validation/phase25"
)

// BuildTierCertification builds a full CertificationRecord for a strategy
// using actual trade records from Phase 25 and metrics from Phase 24/25.
func BuildTierCertification(
	strat EligibleStrategy,
	p24 *phase24.Phase24Result,
	p25 *phase25.Phase25Result,
) CertificationRecord {
	name := strat.StrategyName
	lm, hasLM := p25.LiveMetrics[name]
	p24m := p24.Metrics[name]

	rec := CertificationRecord{
		StrategyName: name,
		MCStability:  "INSUFFICIENT_DATA",
		WFResult:     "INSUFFICIENT_DATA",
	}

	if !hasLM || lm.TotalTrades == 0 {
		rec.Evidence = "INSUFFICIENT_EVIDENCE: no live trades recorded for this strategy"
		rec.Decision = DecisionRetire
		return rec
	}

	// Compute from actual LiveTrade records
	trades := filterTrades(p25.LiveTrades, name)
	var pnls []float64
	for _, t := range trades {
		pnls = append(pnls, t.NetPnLUSD)
	}

	wr, pf, sh, so, exp, mdd, ror, net, fee, fund, slip := computeMetricsFromTrades(trades)

	rec.TotalTrades = len(trades)
	rec.WinRate = wr
	rec.ProfitFactor = pf
	rec.Sharpe = sh
	rec.Sortino = so
	rec.Expectancy = exp
	rec.MaxDD = mdd
	rec.RiskOfRuin = ror
	rec.NetPnLUSD = net
	rec.TotalFeeUSD = fee
	rec.TotalFundingUSD = fund
	rec.TotalSlippageUSD = slip
	rec.UlcerIndex = computeUlcerIndex(pnls)

	if mdd > 0 {
		rec.RecoveryFactor = net / (mdd / 100 * net)
	}

	// Edge retention
	rec.EdgeRetention = BuildEdgeRetention(name, p24m, lm)

	// Drift detection
	rec.DriftRecord = BuildDriftRecord(name, p25.LiveTrades)

	// MC stability from Phase 24 MonteCarlo
	if mc, ok := p24.MonteCarlo[name]; ok {
		if mc.PctProfitable >= 0.60 {
			rec.MCStability = "STABLE"
		} else {
			rec.MCStability = "UNSTABLE"
		}
	}

	// WF result from Phase 24 WalkForward
	for _, wf := range p24.WalkForward {
		if wf.StrategyName == name {
			if wf.IsConsistent {
				rec.WFResult = "PASS"
			} else {
				rec.WFResult = "FAIL"
			}
			break
		}
	}

	rec.Evidence = fmt.Sprintf(
		"Strategy=%s | LiveTrades=%d | Phase24Trades=%d | PF=%.3f | Sharpe=%.3f | MaxDD=%.2f%% | MC=%s | WF=%s",
		name, len(trades), p24m.TotalTrades, pf, sh, mdd, rec.MCStability, rec.WFResult,
	)

	return rec
}

// BuildMilestones builds MilestoneReport at 100/300/500/750/1000 trade marks.
func BuildMilestones(stratName string, trades []phase25.LiveTrade) []MilestoneReport {
	milestones := []int{100, 300, 500, 750, 1000}
	var reports []MilestoneReport
	for _, m := range milestones {
		if len(trades) >= m {
			reports = append(reports, computeMilestoneAt(stratName, trades, m))
		}
	}
	return reports
}
