package phase23a

import (
	"fmt"
	"time"

	"antigravity-engine/internal/validation/phase22f"
)

// BuildCapitalDeploymentPlan creates the Phase 13 capital allocation plan.
// Only strategies meeting ALL hard gates receive capital.
func BuildCapitalDeploymentPlan(
	ranking []RankedStrategy,
	certs []EdgeCertification,
	totalCapital float64,
) CapitalDeploymentPlan {
	plan := CapitalDeploymentPlan{
		GeneratedAt:  time.Now().UTC(),
		TotalCapital: totalCapital,
	}

	certMap := make(map[string]EdgeCertification, len(certs))
	for _, c := range certs {
		certMap[c.StrategyID] = c
	}

	for _, rs := range ranking {
		cert := certMap[rs.StrategyID]
		passed, failed := gatingCheck(rs)

		var band phase22f.CapitalAllocationBand
		allocPct := 0.0

		if len(failed) == 0 {
			band, allocPct = selectBand(rs)
		} else {
			band = phase22f.Band0
		}

		allocUSD := totalCapital * allocPct / 100

		justification := make([]string, 0, 3)
		for i, a := range cert.Answers {
			if i >= 3 {
				break
			}
			justification = append(justification, fmt.Sprintf("%s — %s", a.Question, a.Evidence))
		}

		plan.Entries = append(plan.Entries, DeploymentEntry{
			Rank:          rs.Rank,
			StrategyID:    rs.StrategyID,
			StrategyName:  rs.StrategyName,
			Band:          band,
			AllocationPct: allocPct,
			AllocationUSD: allocUSD,
			JustifiedBy:   justification,
			GatingPassed:  passed,
			GatingFailed:  failed,
		})
		plan.DeployedTotal += allocUSD
	}

	plan.DeployedPct = plan.DeployedTotal / totalCapital * 100
	plan.UndeployedPct = 100 - plan.DeployedPct
	plan.ReadyToDeploy = plan.DeployedPct >= 20 // at least 20% capital deployed
	return plan
}

func gatingCheck(rs RankedStrategy) (passed, failed []string) {
	checks := []struct {
		name string
		ok   bool
	}{
		{fmt.Sprintf("Trades ≥ %d (actual: %d)", CapMinTrades, rs.TradeCount), rs.TradeCount >= CapMinTrades},
		{fmt.Sprintf("PF ≥ %.2f (actual: %.2f)", CapMinPF, rs.ProfitFactor), rs.ProfitFactor >= CapMinPF},
		{fmt.Sprintf("Sharpe ≥ %.2f (actual: %.2f)", CapMinSharpe, rs.Sharpe), rs.Sharpe >= CapMinSharpe},
		{"Positive Expectancy", rs.Expectancy > 0},
		{fmt.Sprintf("RoR ≤ %.0f%% (actual: %.1f%%)", CapMaxRoR*100, rs.RiskOfRuin*100), rs.RiskOfRuin <= CapMaxRoR},
		{fmt.Sprintf("MaxDD ≤ %.0f%% (actual: %.1f%%)", CapMaxDD, rs.MaxDD), rs.MaxDD <= CapMaxDD},
		{fmt.Sprintf("MC tier STABLE+ (actual: %s)", rs.MCTier), rs.MCTier == phase22f.MCRobust || rs.MCTier == phase22f.MCStable22},
	}
	for _, c := range checks {
		if c.ok {
			passed = append(passed, c.name)
		} else {
			failed = append(failed, c.name)
		}
	}
	return passed, failed
}

func selectBand(rs RankedStrategy) (phase22f.CapitalAllocationBand, float64) {
	switch {
	case rs.ProfitFactor >= 1.50 && rs.Sharpe >= 2.00 && rs.TradeCount >= 1000 && rs.MaxDD < 10:
		return phase22f.Band25, 25
	case rs.ProfitFactor >= 1.40 && rs.Sharpe >= 1.75:
		return phase22f.Band20, 20
	case rs.ProfitFactor >= 1.35 && rs.Sharpe >= 1.60:
		return phase22f.Band15, 15
	case rs.ProfitFactor >= 1.30 && rs.Sharpe >= 1.50:
		return phase22f.Band10, 10
	default:
		return phase22f.Band5, 5
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
