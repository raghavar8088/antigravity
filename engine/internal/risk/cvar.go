package risk

import "sort"

type CVaRMetrics struct {
	CVaR95USD float64
	CVaR99USD float64
}

func HistoricalCVaR(returns []float64, equityUSD, confidence float64) float64 {
	if len(returns) == 0 || equityUSD <= 0 {
		return 0
	}
	cp := append([]float64(nil), returns...)
	sort.Float64s(cp)
	cut := int((1 - confidence) * float64(len(cp)))
	if cut < 0 {
		cut = 0
	}
	if cut >= len(cp) {
		cut = len(cp) - 1
	}
	lossSum := 0.0
	count := 0
	for i := 0; i <= cut; i++ {
		loss := -cp[i] * equityUSD
		if loss > 0 {
			lossSum += loss
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return lossSum / float64(count)
}

func PortfolioCVaR(returns []float64, equityUSD float64) CVaRMetrics {
	return CVaRMetrics{
		CVaR95USD: HistoricalCVaR(returns, equityUSD, 0.95),
		CVaR99USD: HistoricalCVaR(returns, equityUSD, 0.99),
	}
}
