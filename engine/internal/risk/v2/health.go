package v2

type HealthCategory string

const (
	HealthElite      HealthCategory = "ELITE"
	HealthHealthy    HealthCategory = "HEALTHY"
	HealthWatchlist  HealthCategory = "WATCHLIST"
	HealthRestricted HealthCategory = "RESTRICTED"
	HealthDisabled   HealthCategory = "DISABLED"
)

type StrategyHealthRisk struct {
	RiskStability     float64
	RiskContribution  float64
	DrawdownStability float64
	HeatImpact        float64
	CapitalEfficiency float64
	Category          HealthCategory
}

func EvaluateStrategyHealthRisk(metrics StrategyMetrics, decision RiskDecision) StrategyHealthRisk {
	riskStability := clamp(100-decision.Forecast.FutureVaRPct*10, 0, 100)
	drawdownStability := clamp(100-metrics.MaxDrawdownPct*8, 0, 100)
	heatImpact := clamp(100-decision.Heat.CurrentHeatPct*10, 0, 100)
	capitalEfficiency := clamp(metrics.ExpectancyUSD/max(decision.RecommendedRiskUSD, 1)*50, 0, 100)
	contribution := clamp(decision.Attribution.TotalRiskScore, 0, 100)
	score := riskStability*0.25 + drawdownStability*0.25 + heatImpact*0.20 + capitalEfficiency*0.20 + (100-contribution)*0.10
	category := HealthDisabled
	switch {
	case score >= 85:
		category = HealthElite
	case score >= 70:
		category = HealthHealthy
	case score >= 55:
		category = HealthWatchlist
	case score >= 40:
		category = HealthRestricted
	}
	return StrategyHealthRisk{RiskStability: riskStability, RiskContribution: contribution, DrawdownStability: drawdownStability, HeatImpact: heatImpact, CapitalEfficiency: capitalEfficiency, Category: category}
}
