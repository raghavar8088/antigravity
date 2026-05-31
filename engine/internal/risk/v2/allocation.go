package v2

type AllocationDecision struct {
	StrategyAllocationPct float64
	FamilyAllocationPct   float64
	AlphaSourcePct        float64
	RecommendedCapitalPct float64
	RebalanceCadence      string
	Allowed               bool
	Reason                string
	Scores                map[string]float64
}

func AllocateCapital(portfolio Portfolio, req TradeRequest, metrics StrategyMetrics) AllocationDecision {
	sharpeScore := clamp(metrics.Sharpe/2.5, 0, 1)
	sortinoScore := clamp(metrics.Sortino/3, 0, 1)
	pfScore := clamp((metrics.ProfitFactor-1)/1.5, 0, 1)
	expScore := 0.5
	if metrics.ExpectancyUSD > 0 {
		expScore = clamp(metrics.ExpectancyUSD/100, 0, 1)
	}
	ddScore := clamp(1-metrics.MaxDrawdownPct/15, 0, 1)
	oosScore := 0.0
	if metrics.OOSProfitFactor > 1 && metrics.OOSExpectancyUSD > 0 {
		oosScore = clamp((metrics.OOSProfitFactor-1)/1.5, 0, 1)
	}
	healthScore := clamp(metrics.HealthScore/100, 0, 1)
	composite := sharpeScore*0.20 + sortinoScore*0.10 + pfScore*0.20 + expScore*0.15 + ddScore*0.15 + oosScore*0.10 + healthScore*0.10
	recommended := 5 + composite*20
	if metrics.TotalTrades < 30 {
		recommended *= 0.50
	}
	familyPct := currentFamilyAllocation(portfolio, req.Family)
	stratPct := currentStrategyAllocation(portfolio, req.Strategy)
	return AllocationDecision{
		StrategyAllocationPct: stratPct,
		FamilyAllocationPct:   familyPct,
		AlphaSourcePct:        recommended,
		RecommendedCapitalPct: recommended,
		RebalanceCadence:      "daily / weekly / monthly",
		Allowed:               stratPct <= 20 && familyPct <= 30,
		Reason:                "allocation within limits",
		Scores: map[string]float64{
			"sharpe": sharpeScore, "sortino": sortinoScore, "profitFactor": pfScore, "expectancy": expScore,
			"drawdown": ddScore, "oos": oosScore, "health": healthScore,
		},
	}
}

func currentFamilyAllocation(portfolio Portfolio, family StrategyFamily) float64 {
	equity := max(portfolio.Account.EquityUSD, 1)
	total := 0.0
	for _, pos := range portfolio.Positions {
		if pos.Family == family {
			total += PositionNotionalUSD(pos)
		}
	}
	return total / equity * 100
}

func currentStrategyAllocation(portfolio Portfolio, strategy string) float64 {
	equity := max(portfolio.Account.EquityUSD, 1)
	total := 0.0
	for _, pos := range portfolio.Positions {
		if pos.Strategy == strategy {
			total += PositionNotionalUSD(pos)
		}
	}
	return total / equity * 100
}
