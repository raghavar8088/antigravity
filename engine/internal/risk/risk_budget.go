package risk

type RiskBudgetBook map[string]float64

func DefaultRiskBudgetBook() RiskBudgetBook {
	return RiskBudgetBook{
		"Trend":          30,
		"Mean Reversion": 20,
		"Breakout":       20,
		"Order Flow":     15,
		"Funding":        15,
	}
}

func (e *PortfolioRiskEngine) checkRiskBudgetLocked(order RiskOrder, metrics PortfolioMetrics) string {
	family := order.StrategyFamily
	if family == "" {
		return ""
	}
	budget, ok := e.riskBudgets[family]
	if !ok {
		return ""
	}
	if metrics.RiskBudgetUsage[family] > budget {
		return "risk budget breached for strategy family"
	}
	return ""
}

func RiskBudgetUsage(positions map[string]RiskPosition, equityUSD float64) map[string]float64 {
	out := make(map[string]float64)
	if equityUSD <= 0 {
		return out
	}
	for _, p := range positions {
		key := p.StrategyFamily
		if key == "" {
			key = "Unknown"
		}
		out[key] += PositionRisk(p, equityUSD).RiskPct
	}
	return out
}
