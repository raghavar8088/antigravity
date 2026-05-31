package risk

type StressScenario struct {
	Name            string
	BTCMovePct      float64
	VolatilityShock float64
	FundingShockPct float64
	ExchangeOutage  bool
}

type StressResult struct {
	ScenarioName         string
	EquityImpactUSD      float64
	ProjectedEquityUSD   float64
	ProjectedDrawdownPct float64
	TradingHalt          bool
}

func DefaultStressScenarios() []StressScenario {
	return []StressScenario{
		{Name: "BTC -5%", BTCMovePct: -5},
		{Name: "BTC -10%", BTCMovePct: -10},
		{Name: "BTC -20%", BTCMovePct: -20},
		{Name: "Volatility Spike", BTCMovePct: -7, VolatilityShock: 2},
		{Name: "Funding Spike", FundingShockPct: 0.25},
		{Name: "Exchange Outage", ExchangeOutage: true},
	}
}

func RunStressTest(positions map[string]RiskPosition, equityUSD float64, scenario StressScenario) StressResult {
	impact := 0.0
	for _, p := range positions {
		move := p.NotionalUSD * (scenario.BTCMovePct / 100)
		if p.Side == SideShort {
			move = -move
		}
		impact += move
		impact -= p.NotionalUSD * (scenario.FundingShockPct / 100)
		if scenario.VolatilityShock > 0 {
			impact -= PositionRisk(p, equityUSD).DollarRisk * (scenario.VolatilityShock - 1)
		}
	}
	projected := equityUSD + impact
	dd := DrawdownPct(projected, equityUSD)
	return StressResult{
		ScenarioName:         scenario.Name,
		EquityImpactUSD:      impact,
		ProjectedEquityUSD:   projected,
		ProjectedDrawdownPct: dd,
		TradingHalt:          scenario.ExchangeOutage || dd > 10,
	}
}
