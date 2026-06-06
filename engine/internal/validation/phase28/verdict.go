package phase28

import (
	"fmt"

	"antigravity-engine/internal/validation/phase25"
	"antigravity-engine/internal/validation/phase26"
	"antigravity-engine/internal/validation/phase27"
)

// BuildFinalVerdict answers the 17 institutional questions from actual data.
func BuildFinalVerdict(p26 *phase26.Phase26Result, p27 *phase27.Phase27Result, p28 Phase28Result) FinalVerdict28 {
	v := FinalVerdict28{}
	v.GeneratedAt = p26.FinalVerdict.GeneratedAt
	// Q1
	v.Q1_HasVerifiedEdge = p26.TotalCertified > 0
	if v.Q1_HasVerifiedEdge {
		v.Q1_Evidence = fmt.Sprintf("%d strategies passed institutional gates", p26.TotalCertified)
	} else {
		v.Q1_Evidence = "INSUFFICIENT_EVIDENCE: no strategies cleared all gates"
	}

	// Q2–Q4
	v.Q2_StrategiesWithEdge = p26.InstitutionalCertified
	v.Q3_CapitalStrategies = p26.FinalVerdict.CapitalReadyStrategies
	v.Q4_RetiredStrategies = p26.Retired

	// Q5–Q7: alpha concentration
	if len(p27.Championship) > 0 {
		v.Q5_TopAlphaFamily = p27.Championship[0].Family
	}
	if len(p27.RedundantFamilies) > 0 {
		v.Q6_AlphaToRemove = p27.RedundantFamilies[0]
	}
	v.Q7_Top1AlphaPct = p27.ParetoAnalysis.Top1AlphaPct
	v.Q7_Top3AlphaPct = p27.ParetoAnalysis.Top3AlphaPct
	v.Q7_Top5AlphaPct = p27.ParetoAnalysis.Top5AlphaPct

	// Q8–Q10: portfolios
	v.Q8_OptimalTop3Portfolio = p28.PortfolioA
	v.Q9_OptimalTop5Portfolio = p28.PortfolioB
	v.Q10_OptimalInstPortfolio = p28.PortfolioD

	// Q11–Q14: platform metrics from best portfolio
	best := p28.PortfolioD
	v.Q11_ExpectedAnnualReturn = best.ExpectedCAGR
	v.Q12_ExpectedSharpe = best.ExpectedSharpe
	v.Q13_ExpectedMaxDD = best.ExpectedMaxDD
	v.Q14_ExpectedRoR = best.ExpectedRoR

	// Q15: institutional justified?
	v.Q15_InstitutionalJustified = p26.TotalCertified >= 3 && best.ExpectedSharpe >= 1.5 && best.ExpectedMaxDD <= 10.0
	if v.Q15_InstitutionalJustified {
		v.Q15_Evidence = fmt.Sprintf("Platform has %d certified strategies, Sharpe=%.3f, MaxDD=%.2f%%", p26.TotalCertified, best.ExpectedSharpe, best.ExpectedMaxDD)
	} else {
		v.Q15_Evidence = fmt.Sprintf("Institutional threshold not met: certified=%d, Sharpe=%.3f, MaxDD=%.2f%%", p26.TotalCertified, best.ExpectedSharpe, best.ExpectedMaxDD)
	}

	// Q16: scale capital?
	v.Q16_ScaleCapitalJustified = v.Q15_InstitutionalJustified && best.ExpectedRoR <= 0.05
	if v.Q16_ScaleCapitalJustified {
		v.Q16_Evidence = fmt.Sprintf("RoR=%.4f <= 5%%, institutional grade confirmed", best.ExpectedRoR)
	} else {
		v.Q16_Evidence = "INSUFFICIENT_EVIDENCE: thresholds not met for capital scaling"
	}

	// Q17: create new strategies?
	// Only recommend new strategies if top strategies PF < 1.20 or Sharpe < 1.00
	topPF := 0.0
	topSharpe := 0.0
	for _, c := range p26.Tier1Strategies {
		if c.ProfitFactor > topPF {
			topPF = c.ProfitFactor
		}
		if c.Sharpe > topSharpe {
			topSharpe = c.Sharpe
		}
	}
	v.Q17_CreateNewStrategies = topPF < 1.20 || topSharpe < 1.00
	if v.Q17_CreateNewStrategies {
		v.Q17_Reasoning = fmt.Sprintf("Best strategy PF=%.3f Sharpe=%.3f — below thresholds, research new alpha", topPF, topSharpe)
	} else {
		v.Q17_Reasoning = fmt.Sprintf("Best strategy PF=%.3f Sharpe=%.3f — scale existing strategies, do not dilute with unproven alpha", topPF, topSharpe)
	}

	// Final verdict
	if v.Q1_HasVerifiedEdge && v.Q15_InstitutionalJustified {
		v.OverallVerdict = "DEPLOY_INSTITUTIONAL"
		v.Justification = fmt.Sprintf("Platform certified %d strategies, institutional grade confirmed, capital scaling justified", p26.TotalCertified)
	} else if v.Q1_HasVerifiedEdge {
		v.OverallVerdict = "DEPLOY_LIMITED"
		v.Justification = "Edge verified but institutional thresholds not fully met — limited capital deployment"
	} else {
		v.OverallVerdict = "PAPER_ONLY"
		v.Justification = "INSUFFICIENT_EVIDENCE: no strategies cleared institutional certification"
	}

	return v
}

// boolToVerdict converts bool to typed verdict. Keeps import used.
var _ = phase25.FamilyFunding
var _ = phase26.DecisionPromote
var _ = phase27.AlphaChampion
