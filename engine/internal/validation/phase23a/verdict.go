package phase23a

import (
	"fmt"
	"strings"
	"time"

	"antigravity-engine/internal/validation/phase22f"
)

// BuildFinalVerdict generates the Phase 14 PHASE23A_FINAL_CERTIFICATION output,
// explicitly answering all 10 institutional deployment questions.
func BuildFinalVerdict(
	ranking []RankedStrategy,
	certs []EdgeCertification,
	alphas []phase22f.AlphaValidationResult,
	elimination []phase22f.EliminationCandidate,
	portfolios []phase22f.PortfolioVariant,
	portfolioMC phase22f.MonteCarloF22,
	plan CapitalDeploymentPlan,
	totalCapital float64,
) FinalVerdict {
	v := FinalVerdict{GeneratedAt: time.Now().UTC()}

	// Q1: strategies with real edge (fully or partially certified)
	for _, c := range certs {
		if c.Certified || c.PartialCredit {
			v.Q1_EdgeStrategies = append(v.Q1_EdgeStrategies, c.StrategyName)
		}
	}

	// Q2: working alpha engines (PF >= 1.20)
	for _, a := range alphas {
		if a.ProfitFactor >= 1.20 && a.Trades >= 10 {
			v.Q2_WorkingAlphas = append(v.Q2_WorkingAlphas, fmt.Sprintf("%s (PF=%.2f)", a.AlphaEngine, a.ProfitFactor))
		}
	}

	// Q3: retire
	for _, e := range elimination {
		if e.Severity == phase22f.EliminateImmediate {
			v.Q3_Retire = append(v.Q3_Retire, e.StrategyName)
		}
	}

	// Q4: paper capital only (partial cert or Paper/Watchlist tier)
	for _, c := range certs {
		if c.PartialCredit && !c.Certified {
			v.Q4_PaperCapital = append(v.Q4_PaperCapital, c.StrategyName)
		}
	}
	for _, rs := range ranking {
		if (rs.CertificationTier == phase22f.TierPaperOnly || rs.CertificationTier == phase22f.TierWatchlist) {
			if !contains23(v.Q4_PaperCapital, rs.StrategyName) {
				v.Q4_PaperCapital = append(v.Q4_PaperCapital, rs.StrategyName)
			}
		}
	}

	// Q5: ready for live capital (full cert AND gating passed)
	for _, de := range plan.Entries {
		if de.AllocationPct > 0 && len(de.GatingFailed) == 0 {
			v.Q5_LiveCapital = append(v.Q5_LiveCapital, fmt.Sprintf("%s (%.0f%%)", de.StrategyName, de.AllocationPct))
		}
	}

	// Q6: portfolio profile (use top10 portfolio if available)
	var topPort *phase22f.PortfolioVariant
	for i := range portfolios {
		if portfolios[i].Name == "Top10" {
			topPort = &portfolios[i]
			break
		}
	}
	if topPort == nil && len(portfolios) > 0 {
		topPort = &portfolios[0]
	}
	if topPort != nil {
		v.Q6_PortfolioProfile = PortfolioProfile{
			ExpectedPF:      topPort.ProfitFactor,
			ExpectedSharpe:  topPort.Sharpe,
			ExpectedMaxDD:   topPort.MaxDrawdown,
			ExpectedMCTier:  topPort.MonteCarlo.Stability,
		}
	}
	// CAGR from top strategy
	if len(ranking) > 0 {
		v.Q6_PortfolioProfile.ExpectedCAGR = ranking[0].CAGR
	}
	// Sortino from top strategy
	if len(ranking) > 0 {
		v.Q6_PortfolioProfile.ExpectedSortino = ranking[0].Sortino
	}

	// Q7: profitable after costs
	v.Q7_ProfitableAfterCosts = portfolioMC.ProbabilityGrow >= 0.60 &&
		v.Q6_PortfolioProfile.ExpectedPF >= 1.10

	// Q8: institutional readiness
	v.Q8_InstitutionalReady = v.Q6_PortfolioProfile.ExpectedPF >= 1.30 &&
		v.Q6_PortfolioProfile.ExpectedSharpe >= 1.50 &&
		(portfolioMC.Stability == phase22f.MCRobust || portfolioMC.Stability == phase22f.MCStable22) &&
		len(v.Q5_LiveCapital) >= 3

	// Q9: top 10 strategies
	v.Q9_Top10 = ranking[:clamp(len(ranking), 0, 10)]

	// Q10: deploy today
	v.Q10_DeployToday = v.Q7_ProfitableAfterCosts && v.Q8_InstitutionalReady && len(v.Q5_LiveCapital) > 0
	if v.Q10_DeployToday {
		v.DeployTodayReason = fmt.Sprintf("System passes all institutional gates: PF=%.2f Sharpe=%.2f MC=%s %d strategies approved for live capital.",
			v.Q6_PortfolioProfile.ExpectedPF, v.Q6_PortfolioProfile.ExpectedSharpe, portfolioMC.Stability, len(v.Q5_LiveCapital))
	} else {
		reasons := []string{}
		if !v.Q7_ProfitableAfterCosts {
			reasons = append(reasons, "system not profitable after costs")
		}
		if !v.Q8_InstitutionalReady {
			reasons = append(reasons, "institutional readiness gates not met")
		}
		if len(v.Q5_LiveCapital) == 0 {
			reasons = append(reasons, "no strategies passed all capital deployment gates")
		}
		v.DeployTodayReason = "NOT READY: " + strings.Join(reasons, "; ")
	}

	v.Narrative = buildFinalNarrative(v)
	return v
}

func buildFinalNarrative(v FinalVerdict) string {
	b := &strings.Builder{}
	deploy := "NOT APPROVED FOR LIVE CAPITAL"
	if v.Q10_DeployToday {
		deploy = "APPROVED FOR PHASED LIVE DEPLOYMENT"
	}

	fmt.Fprintf(b, "PHASE 23A FINAL VERDICT: %s\n\n", deploy)
	fmt.Fprintf(b, "Edge strategies: %d identified. Working alphas: %d. Retire: %d. Paper: %d. Live capital: %d.\n\n",
		len(v.Q1_EdgeStrategies), len(v.Q2_WorkingAlphas), len(v.Q3_Retire), len(v.Q4_PaperCapital), len(v.Q5_LiveCapital))
	fmt.Fprintf(b, "Portfolio profile: PF=%.2f Sharpe=%.2f DD=%.1f%% CAGR=%.1f%% MC=%s\n\n",
		v.Q6_PortfolioProfile.ExpectedPF, v.Q6_PortfolioProfile.ExpectedSharpe,
		v.Q6_PortfolioProfile.ExpectedMaxDD, v.Q6_PortfolioProfile.ExpectedCAGR,
		v.Q6_PortfolioProfile.ExpectedMCTier)
	fmt.Fprintf(b, "Decision basis: %s", v.DeployTodayReason)
	return b.String()
}

func contains23(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
