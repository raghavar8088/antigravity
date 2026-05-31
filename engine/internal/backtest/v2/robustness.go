package v2

type RobustnessScenario struct {
	Name               string
	FeeMultiplier      float64
	SpreadMultiplier   float64
	SlippageMultiplier float64
	FundingShockUSD    float64
	MissingDataPct     float64
}

type RobustnessResult struct {
	Scenario     string
	NetPnL       float64
	ProfitFactor float64
	Expectancy   float64
	Survived     bool
}

type RobustnessReport struct {
	Results         []RobustnessResult
	RobustnessScore float64
	Profitable      bool
}

func DefaultRobustnessScenarios() []RobustnessScenario {
	return []RobustnessScenario{
		{Name: "HIGHER_FEES", FeeMultiplier: 2},
		{Name: "HIGHER_SPREAD", SpreadMultiplier: 2},
		{Name: "HIGHER_SLIPPAGE", SlippageMultiplier: 2},
		{Name: "FUNDING_SHOCK", FundingShockUSD: 5},
		{Name: "VOLATILITY_SHOCK", SlippageMultiplier: 3, SpreadMultiplier: 1.5},
		{Name: "LATENCY_SHOCK", SlippageMultiplier: 1.3},
		{Name: "RANDOM_MISSING_DATA", MissingDataPct: 5},
	}
}

func RunRobustness(trades []Trade, initialCapital float64, scenarios []RobustnessScenario) RobustnessReport {
	if len(scenarios) == 0 {
		scenarios = DefaultRobustnessScenarios()
	}
	report := RobustnessReport{Profitable: true}
	survived := 0
	for _, sc := range scenarios {
		stressed := make([]Trade, 0, len(trades))
		for i, tr := range trades {
			if sc.MissingDataPct > 0 && float64(i%100) < sc.MissingDataPct {
				continue
			}
			feeMul := nonZero(sc.FeeMultiplier, 1)
			spreadMul := nonZero(sc.SpreadMultiplier, 1)
			slipMul := nonZero(sc.SlippageMultiplier, 1)
			tr.NetPnL -= tr.Fees*(feeMul-1) + tr.SpreadCost*(spreadMul-1) + tr.SlippageCost*(slipMul-1) + abs(sc.FundingShockUSD)
			stressed = append(stressed, tr)
		}
		m := CalculateMetrics(stressed, initialCapital)
		ok := m.NetPnL > 0 && m.ProfitFactor > 1
		if ok {
			survived++
		} else {
			report.Profitable = false
		}
		report.Results = append(report.Results, RobustnessResult{Scenario: sc.Name, NetPnL: m.NetPnL, ProfitFactor: m.ProfitFactor, Expectancy: m.Expectancy, Survived: ok})
	}
	report.RobustnessScore = float64(survived) / float64(len(scenarios)) * 100
	return report
}

func nonZero(v, fallback float64) float64 {
	if v == 0 {
		return fallback
	}
	return v
}
