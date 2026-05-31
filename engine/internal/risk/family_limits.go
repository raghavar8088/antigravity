package risk

type FamilyLimitBook map[string]float64

func DefaultFamilyLimitBook() FamilyLimitBook {
	return FamilyLimitBook{
		"Trend":          30,
		"Order Flow":     20,
		"Funding":        20,
		"Breakout":       25,
		"Mean Reversion": 25,
	}
}

func (e *PortfolioRiskEngine) checkFamilyLimitLocked(order RiskOrder, metrics PortfolioMetrics) string {
	if metrics.EquityUSD <= 0 || metrics.GrossExposureUSD/metrics.EquityUSD*100 < 20 {
		return ""
	}
	family := order.StrategyFamily
	if family == "" {
		return ""
	}
	limit := e.limits.MaxFamilyAllocationPct
	if v, ok := e.familyLimits[family]; ok && v > 0 {
		limit = v
	}
	if metrics.FamilyConcentration[family] > limit {
		return "strategy family allocation limit breached"
	}
	return ""
}
