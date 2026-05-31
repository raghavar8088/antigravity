package v2

type RiskDashboard struct {
	PortfolioHeatGauge  HeatSnapshot
	VaRDashboard        []VaRReport
	CVaRDashboard       []CVaRReport
	CorrelationHeatmap  map[string]map[string]float64
	CapitalAllocation   map[string]float64
	RiskBudgetDashboard map[StrategyFamily]float64
	DrawdownDashboard   DrawdownDecision
	LeverageDashboard   float64
	TailRiskMonitor     TailRiskDecision
	RiskAttributionView RiskAttribution
	ExposureMonitor     ExposureReport
	ForecastDashboard   RiskForecast
	FamilyRiskView      map[StrategyFamily]FamilyRiskDecision
	KellySizingWidget   KellyDecision
	Alerts              []RiskAlert
	LastDecision        RiskDecision
}

func BuildRiskDashboard(portfolio Portfolio, heatHistory []HeatSnapshot, varHistory []VaRReport, cvarHistory []CVaRReport, decisions []RiskDecision) RiskDashboard {
	var last RiskDecision
	if len(decisions) > 0 {
		last = decisions[len(decisions)-1]
	}
	heat := HeatSnapshot{}
	if len(heatHistory) > 0 {
		heat = heatHistory[len(heatHistory)-1]
	}
	families := make(map[StrategyFamily]FamilyRiskDecision)
	for _, pos := range portfolio.Positions {
		if _, ok := families[pos.Family]; !ok {
			families[pos.Family] = EvaluateFamilyRisk(portfolio, TradeRequest{Family: pos.Family}, 30)
		}
	}
	return RiskDashboard{
		PortfolioHeatGauge:  heat,
		VaRDashboard:        append([]VaRReport(nil), varHistory...),
		CVaRDashboard:       append([]CVaRReport(nil), cvarHistory...),
		CorrelationHeatmap:  last.Correlation.Matrix,
		CapitalAllocation:   portfolio.Allocations,
		RiskBudgetDashboard: DefaultRiskBudgets(),
		DrawdownDashboard:   EvaluateDrawdown(portfolio.Account),
		LeverageDashboard:   last.RecommendedLeverage,
		TailRiskMonitor:     last.TailRisk,
		RiskAttributionView: last.Attribution,
		ExposureMonitor:     last.Exposure,
		ForecastDashboard:   last.Forecast,
		FamilyRiskView:      families,
		KellySizingWidget:   last.Kelly,
		Alerts:              last.Alerts,
		LastDecision:        last,
	}
}
