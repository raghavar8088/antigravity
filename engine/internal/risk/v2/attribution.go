package v2

type RiskAttribution struct {
	ByStrategy      map[string]float64
	ByFamily        map[StrategyFamily]float64
	FundingRisk     float64
	VolatilityRisk  float64
	ExecutionRisk   float64
	CorrelationRisk float64
	RegimeRisk      float64
	TotalRiskScore  float64
}

func AttributeRisk(portfolio Portfolio, req TradeRequest, heat HeatSnapshot, vr VaRReport, cv CVaRReport, corr CorrelationReport, exposure ExposureReport) RiskAttribution {
	byStrategy := make(map[string]float64)
	byFamily := make(map[StrategyFamily]float64)
	equity := max(portfolio.Account.EquityUSD, 1)
	for _, pos := range portfolio.Positions {
		r := PositionRiskFromPosition(pos) / equity * 100
		byStrategy[pos.Strategy] += r
		byFamily[pos.Family] += r
	}
	reqRisk := PositionRiskUSD(req, req.RequestedSizeBTC) / equity * 100
	byStrategy[req.Strategy] += reqRisk
	byFamily[req.Family] += reqRisk
	total := heat.CurrentHeatPct + vr.DailyVaRPct + cv.CVaR95Pct + corr.MaxCorrelation*10 + exposure.GrossExposurePct/10
	return RiskAttribution{
		ByStrategy:      byStrategy,
		ByFamily:        byFamily,
		FundingRisk:     abs(reqRisk * 0.15),
		VolatilityRisk:  vr.DailyVaRPct,
		ExecutionRisk:   heat.CurrentHeatPct * 0.10,
		CorrelationRisk: corr.MaxCorrelation * 100,
		RegimeRisk:      exposure.GrossExposurePct * 0.05,
		TotalRiskScore:  total,
	}
}
