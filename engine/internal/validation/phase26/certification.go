package phase26

import "fmt"

// ApplyCertificationGates evaluates gates for a CertificationRecord and
// sets GatesPassed, GatesFailed, and Decision.
// minTrades: required minimum trade count (Tier1=300, Tier2=1000).
func ApplyCertificationGates(rec *CertificationRecord, minTrades int) {
	if rec.TotalTrades < minTrades {
		rec.GatesFailed = append(rec.GatesFailed, fmt.Sprintf("TRADES: have %d, need %d", rec.TotalTrades, minTrades))
		rec.Decision = DecisionMaintain
		rec.Evidence += fmt.Sprintf(" | INSUFFICIENT_EVIDENCE: only %d trades (need %d)", rec.TotalTrades, minTrades)
		return
	}

	check := func(name string, pass bool) {
		if pass {
			rec.GatesPassed = append(rec.GatesPassed, name)
		} else {
			rec.GatesFailed = append(rec.GatesFailed, name)
		}
	}

	check(fmt.Sprintf("PF>=%.2f (have %.3f)", LiveMinPF, rec.ProfitFactor), rec.ProfitFactor >= LiveMinPF)
	check(fmt.Sprintf("Sharpe>=%.2f (have %.3f)", LiveMinSharpe, rec.Sharpe), rec.Sharpe >= LiveMinSharpe)
	check(fmt.Sprintf("Expectancy>%.1f (have %.4f)", LiveMinExpectancy, rec.Expectancy), rec.Expectancy > LiveMinExpectancy)
	check(fmt.Sprintf("RoR<=%.2f (have %.4f)", LiveMaxRoR, rec.RiskOfRuin), rec.RiskOfRuin <= LiveMaxRoR)
	check(fmt.Sprintf("MaxDD<=%.1f%% (have %.2f%%)", LiveMaxDD, rec.MaxDD), rec.MaxDD <= LiveMaxDD)

	// Edge retention >= 80%
	avgRetention := 0.0
	cnt := 0
	if rec.EdgeRetention.HistoricalPF > 0 {
		avgRetention += rec.EdgeRetention.PFRetentionPct
		cnt++
	}
	if rec.EdgeRetention.HistoricalSharpe > 0 {
		avgRetention += rec.EdgeRetention.SharpeRetentionPct
		cnt++
	}
	if cnt > 0 {
		avgRetention /= float64(cnt)
	}
	check(fmt.Sprintf("EdgeRetention>=80%% (have %.1f%%)", avgRetention), avgRetention >= 80.0)

	check("MCStability=STABLE", rec.MCStability == "STABLE")
	check("WFResult=PASS", rec.WFResult == "PASS")

	allPass := len(rec.GatesFailed) == 0

	switch {
	case allPass:
		rec.Decision = DecisionPromote
	case len(rec.GatesFailed) <= 2:
		rec.Decision = DecisionMaintain
	case rec.ProfitFactor < 1.0 || rec.MaxDD > 20.0:
		rec.Decision = DecisionRetire
	case rec.ProfitFactor < LiveMinPF:
		rec.Decision = DecisionDemote
	default:
		rec.Decision = DecisionReduce
	}
}
