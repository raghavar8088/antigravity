package v2

type OptimizationMethod string

const (
	OptimizeEqualWeight        OptimizationMethod = "EQUAL_WEIGHT"
	OptimizeRiskParity         OptimizationMethod = "RISK_PARITY"
	OptimizeSharpe             OptimizationMethod = "SHARPE_OPTIMIZATION"
	OptimizeMinimumVariance    OptimizationMethod = "MINIMUM_VARIANCE"
	OptimizeMaxDiversification OptimizationMethod = "MAXIMUM_DIVERSIFICATION"
	OptimizeKellyPortfolio     OptimizationMethod = "KELLY_PORTFOLIO"
)

type PortfolioSnapshot struct {
	Method            OptimizationMethod
	Weights           map[string]float64
	ExpectedReturn    float64
	ExpectedRisk      float64
	Sharpe            float64
	EfficientFrontier []FrontierPoint
}

type FrontierPoint struct {
	Return float64
	Risk   float64
	Sharpe float64
}

func OptimizePortfolio(metrics map[string]StrategyMetrics, method OptimizationMethod) PortfolioSnapshot {
	weights := make(map[string]float64)
	if len(metrics) == 0 {
		return PortfolioSnapshot{Method: method, Weights: weights}
	}
	totalScore := 0.0
	for name, m := range metrics {
		score := 1.0
		switch method {
		case OptimizeSharpe:
			score = max(m.Sharpe, 0.01)
		case OptimizeMinimumVariance:
			score = 1 / max(stddev(m.Returns), 0.001)
		case OptimizeRiskParity:
			score = 1 / max(abs(m.MaxDrawdownPct), 1)
		case OptimizeKellyPortfolio:
			score = clamp((m.ProfitFactor-1)*m.WinRate, 0.01, 1)
		case OptimizeMaxDiversification:
			score = max(m.Sortino, 0.01)
		default:
			score = 1
		}
		weights[name] = score
		totalScore += score
	}
	for name, w := range weights {
		weights[name] = w / totalScore * 100
	}
	ret, risk := 0.0, 0.0
	for name, w := range weights {
		m := metrics[name]
		ret += w / 100 * m.ExpectancyUSD
		risk += w / 100 * max(stddev(m.Returns), m.MaxDrawdownPct/100)
	}
	return PortfolioSnapshot{
		Method:            method,
		Weights:           weights,
		ExpectedReturn:    ret,
		ExpectedRisk:      risk,
		Sharpe:            ret / max(risk, 0.001),
		EfficientFrontier: []FrontierPoint{{Return: ret * 0.75, Risk: risk * 0.75, Sharpe: ret / max(risk, 0.001)}, {Return: ret, Risk: risk, Sharpe: ret / max(risk, 0.001)}, {Return: ret * 1.15, Risk: risk * 1.35, Sharpe: ret * 1.15 / max(risk*1.35, 0.001)}},
	}
}
