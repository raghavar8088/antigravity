package v2

import "math"

type RiskForecast struct {
	FutureVaRPct      float64
	FutureDrawdownPct float64
	FutureHeatPct     float64
	FutureExposurePct float64
	EWMAVolatility    float64
	MonteCarloLossPct float64
	RiskForecastScore float64
}

func ForecastRisk(portfolio Portfolio, market MarketState) RiskForecast {
	returns := tail(portfolio.Returns, 250)
	ewma := EWMAVolatility(returns, 0.94)
	varReport := CalculateVaR(returns, portfolio.Account.EquityUSD)
	dd := EvaluateDrawdown(portfolio.Account)
	exposure := CalculateExposure(portfolio, TradeRequest{EntryPrice: 0}, 0)
	futureHeat := 0.0
	for _, pos := range portfolio.Positions {
		futureHeat += PositionRiskFromPosition(pos)
	}
	futureHeat = futureHeat / max(portfolio.Account.EquityUSD, 1) * 100
	mcLoss := MonteCarloVaR(returns, portfolio.Account.EquityUSD, 0.99, 2000) / max(portfolio.Account.EquityUSD, 1) * 100
	score := 100 - clamp(varReport.DailyVaRPct*6+dd.DrawdownPct*4+futureHeat*4+ewma*100+mcLoss*3+market.VolatilityPct*2, 0, 100)
	return RiskForecast{
		FutureVaRPct:      varReport.DailyVaRPct,
		FutureDrawdownPct: dd.DrawdownPct + max(-mean(tail(returns, 10))*100, 0),
		FutureHeatPct:     futureHeat,
		FutureExposurePct: exposure.GrossExposurePct,
		EWMAVolatility:    ewma,
		MonteCarloLossPct: mcLoss,
		RiskForecastScore: score,
	}
}

func EWMAVolatility(returns []float64, lambda float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	if lambda <= 0 || lambda >= 1 {
		lambda = 0.94
	}
	var variance float64
	for _, r := range returns {
		variance = lambda*variance + (1-lambda)*r*r
	}
	return math.Sqrt(variance)
}
