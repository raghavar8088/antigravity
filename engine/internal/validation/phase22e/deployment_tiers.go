package phase22e

import "fmt"

// DeploymentTier classifies the capital deployment authorisation level.
type DeploymentTier string

const (
	TierNotCertified   DeploymentTier = "NOT CERTIFIED"
	TierPaperOnly      DeploymentTier = "PAPER ONLY"
	TierPilotCapital   DeploymentTier = "PILOT CAPITAL"
	TierLimitedCapital DeploymentTier = "LIMITED CAPITAL"
	TierFullDeployment DeploymentTier = "FULL DEPLOYMENT"
	TierInstitutional  DeploymentTier = "INSTITUTIONAL GRADE"
)

// DeploymentClassification holds the tier assignment and evidence for one strategy.
type DeploymentClassification struct {
	StrategyID   string
	StrategyName string
	Family       string
	Tier         DeploymentTier
	Evidence     []string // criteria that drove this tier
	MaxCapitalPct float64  // maximum % of total capital at this tier
}

// ClassifyDeploymentTier assigns an institutional deployment tier to a strategy.
//
// Tiers (ascending):
//
//	NOT CERTIFIED    — PF < 1.00
//	PAPER ONLY       — PF ≥ 1.10
//	PILOT CAPITAL    — PF ≥ 1.20, Sharpe ≥ 1.00
//	LIMITED CAPITAL  — PF ≥ 1.30, Sharpe ≥ 1.25
//	FULL DEPLOYMENT  — PF ≥ 1.40, Sharpe ≥ 1.50
//	INSTITUTIONAL    — PF ≥ 1.50, Sharpe ≥ 2.00, trades ≥ 1000, MaxDD < 10%
func ClassifyDeploymentTier(s StrategyMetrics) DeploymentClassification {
	dc := DeploymentClassification{
		StrategyID:   s.StrategyID,
		StrategyName: s.StrategyName,
		Family:       s.Family,
	}

	switch {
	case s.ProfitFactor >= 1.50 && s.Sharpe >= 2.00 && s.TotalTrades >= 1000 && s.MaxDrawdown < 10.0:
		dc.Tier = TierInstitutional
		dc.MaxCapitalPct = 20.0
		dc.Evidence = []string{
			fmt.Sprintf("PF %.2f ≥ 1.50", s.ProfitFactor),
			fmt.Sprintf("Sharpe %.2f ≥ 2.00", s.Sharpe),
			fmt.Sprintf("%d trades ≥ 1000", s.TotalTrades),
			fmt.Sprintf("MaxDD %.1f%% < 10%%", s.MaxDrawdown),
		}
	case s.ProfitFactor >= 1.40 && s.Sharpe >= 1.50:
		dc.Tier = TierFullDeployment
		dc.MaxCapitalPct = 15.0
		dc.Evidence = []string{
			fmt.Sprintf("PF %.2f ≥ 1.40", s.ProfitFactor),
			fmt.Sprintf("Sharpe %.2f ≥ 1.50", s.Sharpe),
		}
	case s.ProfitFactor >= 1.30 && s.Sharpe >= 1.25:
		dc.Tier = TierLimitedCapital
		dc.MaxCapitalPct = 10.0
		dc.Evidence = []string{
			fmt.Sprintf("PF %.2f ≥ 1.30", s.ProfitFactor),
			fmt.Sprintf("Sharpe %.2f ≥ 1.25", s.Sharpe),
		}
	case s.ProfitFactor >= 1.20 && s.Sharpe >= 1.00:
		dc.Tier = TierPilotCapital
		dc.MaxCapitalPct = 5.0
		dc.Evidence = []string{
			fmt.Sprintf("PF %.2f ≥ 1.20", s.ProfitFactor),
			fmt.Sprintf("Sharpe %.2f ≥ 1.00", s.Sharpe),
		}
	case s.ProfitFactor >= 1.10:
		dc.Tier = TierPaperOnly
		dc.MaxCapitalPct = 0.0
		dc.Evidence = []string{
			fmt.Sprintf("PF %.2f ≥ 1.10 (below live threshold)", s.ProfitFactor),
			"Continue paper trading to build statistical confidence",
		}
	default:
		dc.Tier = TierNotCertified
		dc.MaxCapitalPct = 0.0
		dc.Evidence = []string{
			fmt.Sprintf("PF %.2f < 1.10 — no positive edge demonstrated", s.ProfitFactor),
		}
	}

	return dc
}

// ClassifyAllStrategies applies deployment tier classification to every strategy.
func ClassifyAllStrategies(strategies []StrategyMetrics) []DeploymentClassification {
	out := make([]DeploymentClassification, len(strategies))
	for i, s := range strategies {
		out[i] = ClassifyDeploymentTier(s)
	}
	return out
}

// TierCounts returns a summary count per tier.
func TierCounts(classifications []DeploymentClassification) map[DeploymentTier]int {
	counts := make(map[DeploymentTier]int)
	for _, dc := range classifications {
		counts[dc.Tier]++
	}
	return counts
}
