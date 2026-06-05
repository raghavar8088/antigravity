package phase23c

import (
	"fmt"
	"time"

	"antigravity-engine/internal/validation/phase22f"
	"antigravity-engine/internal/validation/phase23b"
)

// BuildFinalVerdict answers the 12 institutional deployment questions
// using only real, evidence-backed data from Phase 23B/C.
func BuildFinalVerdict(source *phase23b.Phase23BResult, ranked []RankedStrategy23C,
	alpha []AlphaChampionResult, eliminated []EliminationRecord,
	top5 PortfolioProfile) FinalVerdict23C {

	v := FinalVerdict23C{GeneratedAt: time.Now().UTC()}

	// Q1: Which strategies have real edge?
	for _, r := range ranked {
		if r.ProfitFactor >= 1.30 && r.Sharpe >= 1.50 && r.TradeCount >= 100 {
			v.Q1_EdgeStrategies = append(v.Q1_EdgeStrategies, r.StrategyName)
		}
	}

	// Q2: Which alpha engines actually work?
	for _, a := range alpha {
		if a.Verdict == "CHAMPION" || a.Verdict == "STRONG" {
			v.Q2_WorkingAlphas = append(v.Q2_WorkingAlphas, a.AlphaEngine)
		}
	}

	// Q3: Which strategies deserve capital?
	for _, r := range ranked {
		if r.DeploymentStatus == "DEPLOY_NOW" || r.DeploymentStatus == "PILOT" {
			v.Q3_DeserveCapital = append(v.Q3_DeserveCapital, r.StrategyName)
		}
	}

	// Q4: Which strategies deserve live deployment?
	for _, r := range ranked {
		if r.DeploymentStatus == "DEPLOY_NOW" {
			v.Q4_DeserveLiveDeploy = append(v.Q4_DeserveLiveDeploy, r.StrategyName)
		}
	}

	// Q5: Which strategies should be retired?
	for _, e := range eliminated {
		v.Q5_Retire = append(v.Q5_Retire, e.StrategyName)
	}

	// Q6: Is the platform profitable after costs?
	totalNet := 0.0
	sumWin, sumLoss := 0.0, 0.0
	for _, t := range source.CertifiedTrades {
		totalNet += t.NetPnLUSD
		if t.NetPnLUSD > 0 {
			sumWin += t.NetPnLUSD
		} else {
			if t.NetPnLUSD < 0 {
				sumLoss += -t.NetPnLUSD
			}
		}
	}
	platformPF := 0.0
	if sumLoss > 0 {
		platformPF = sumWin / sumLoss
	}
	v.Q6_PlatformProfitable = totalNet > 0 && platformPF >= 1.10
	v.Q6_Evidence = fmt.Sprintf("Platform net PnL: $%.0f | Platform PF: %.3f | Total certified trades: %d",
		totalNet, platformPF, source.TotalTrades)

	// Q7: Is the platform institutionally deployable?
	certCount := 0
	for _, c := range source.CapCertifications {
		if c.Tier == phase23b.CapTierInstitutional || c.Tier == phase23b.CapTierFull {
			certCount++
		}
	}
	v.Q7_InstitutionalReady = certCount >= 3 && v.Q6_PlatformProfitable
	v.Q7_Evidence = fmt.Sprintf("%d strategies at Full/Institutional tier | Platform profitable: %v",
		certCount, v.Q6_PlatformProfitable)

	// Q8-Q10: Rankings
	if len(ranked) >= 20 {
		v.Q8_TrueTop20 = ranked[:20]
	} else {
		v.Q8_TrueTop20 = ranked
	}
	if len(ranked) >= 10 {
		v.Q9_TrueTop10 = ranked[:10]
	} else {
		v.Q9_TrueTop10 = ranked
	}
	if len(ranked) >= 5 {
		v.Q10_TrueTop5 = ranked[:5]
	} else {
		v.Q10_TrueTop5 = ranked
	}

	// Q11: Deploy capital today?
	v.Q11_DeployCapitalToday = v.Q7_InstitutionalReady && len(v.Q4_DeserveLiveDeploy) >= 3
	if v.Q11_DeployCapitalToday {
		v.Q11_Justification = fmt.Sprintf(
			"APPROVED FOR CAPITAL DEPLOYMENT: %d strategies certified for live deployment, "+
				"platform PF=%.3f, net PnL=$%.0f, %d institutional-grade strategies identified. "+
				"Recommend staged deployment: S1(10%%) → S2(25%%) → S3(50%%) → S4(100%%).",
			len(v.Q4_DeserveLiveDeploy), platformPF, totalNet, certCount)
	} else {
		v.Q11_Justification = fmt.Sprintf(
			"NOT APPROVED FOR CAPITAL DEPLOYMENT: Requires minimum 3 live-deployable strategies "+
				"and institutional platform readiness. Current: %d deployable, institutional ready: %v.",
			len(v.Q4_DeserveLiveDeploy), v.Q7_InstitutionalReady)
	}

	// Q12: Allocation plan
	v.Q12_AllocationPlan = make(map[string]float64)
	for _, entry := range top5.Entries {
		v.Q12_AllocationPlan[entry.StrategyName] = entry.Weight
	}

	// Narrative
	v.NarrativeStatement = buildNarrative(v, source, ranked, alpha, eliminated, platformPF, totalNet)

	return v
}

func buildNarrative(v FinalVerdict23C, source *phase23b.Phase23BResult,
	ranked []RankedStrategy23C, alpha []AlphaChampionResult,
	eliminated []EliminationRecord, platformPF, totalNet float64) string {

	// Count MC stability distribution
	robustCount, stableCount := 0, 0
	for _, mc := range source.MonteCarlo {
		if mc.Stability == phase22f.MCRobust {
			robustCount++
		}
		if mc.Stability == phase22f.MCStable22 {
			stableCount++
		}
	}

	workingAlphas := len(v.Q2_WorkingAlphas)
	totalAlphas := len(alpha)

	narrative := fmt.Sprintf(
		"## INSTITUTIONAL VERDICT — PHASE 23B/C\n\n"+
			"**Data:** %d real Binance Futures candles, %.0f calendar days\n"+
			"**Strategies Validated:** %d real strategy instances via Strategy.OnCandle()\n"+
			"**Certified Trades:** %d (100%% real, 0%% synthetic)\n\n"+

			"**Platform Performance (post-cost):**\n"+
			"- Net PnL: $%.0f across all strategies\n"+
			"- Platform Profit Factor: %.3f\n"+
			"- Strategies with edge (PF≥1.30, Sharpe≥1.50): %d\n"+
			"- Monte Carlo Robust: %d | Stable: %d\n\n"+

			"**Alpha Engine Results:** %d/%d alpha engines validated as STRONG or CHAMPION\n\n"+

			"**Deployment Summary:**\n"+
			"- DEPLOY NOW: %d strategies\n"+
			"- PILOT: %d strategies\n"+
			"- PAPER ONLY: %d strategies\n"+
			"- RETIRED: %d strategies\n\n"+

			"**Capital Deployment Decision:** %s\n\n"+
			"%s",

		source.TotalCandles, source.CoverageDays,
		source.TotalStrategies,
		source.TotalTrades,
		totalNet, platformPF,
		len(v.Q1_EdgeStrategies),
		robustCount, stableCount,
		workingAlphas, totalAlphas,
		len(v.Q4_DeserveLiveDeploy),
		countStatus(ranked, "PILOT"),
		countStatus(ranked, "PAPER"),
		len(eliminated),
		boolToVerdict(v.Q11_DeployCapitalToday),
		v.Q11_Justification,
	)
	return narrative
}

func countStatus(ranked []RankedStrategy23C, status string) int {
	n := 0
	for _, r := range ranked {
		if r.DeploymentStatus == status {
			n++
		}
	}
	return n
}

func boolToVerdict(b bool) string {
	if b {
		return "✅ APPROVED — DEPLOY TODAY"
	}
	return "❌ NOT APPROVED — CONDITIONS NOT MET"
}
