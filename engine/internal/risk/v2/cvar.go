package v2

type CVaRReport struct {
	CVaR95USD float64
	CVaR99USD float64
	CVaR95Pct float64
	CVaR99Pct float64
	Method    string
}

func CalculateCVaR(returns []float64, equityUSD float64) CVaRReport {
	if equityUSD <= 0 {
		equityUSD = 100_000
	}
	c95 := HistoricalCVaR(returns, equityUSD, 0.95)
	c99 := HistoricalCVaR(returns, equityUSD, 0.99)
	return CVaRReport{
		CVaR95USD: c95,
		CVaR99USD: c99,
		CVaR95Pct: c95 / equityUSD * 100,
		CVaR99Pct: c99 / equityUSD * 100,
		Method:    "historical expected shortfall",
	}
}

func HistoricalCVaR(returns []float64, equityUSD, confidence float64) float64 {
	if len(returns) == 0 || equityUSD <= 0 {
		return 0
	}
	threshold := percentile(returns, 1-confidence)
	losses := make([]float64, 0)
	for _, r := range returns {
		if r <= threshold && r < 0 {
			losses = append(losses, abs(r)*equityUSD)
		}
	}
	if len(losses) == 0 {
		return HistoricalVaR(returns, equityUSD, confidence)
	}
	return mean(losses)
}
