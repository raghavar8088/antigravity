package risk

import (
	"math"
	"sort"
)

type VaRMetrics struct {
	Daily95USD   float64
	Daily99USD   float64
	Weekly95USD  float64
	Weekly99USD  float64
	Monthly95USD float64
	Monthly99USD float64
}

func HistoricalVaR(returns []float64, equityUSD, confidence float64) float64 {
	if len(returns) == 0 || equityUSD <= 0 {
		return 0
	}
	cp := append([]float64(nil), returns...)
	sort.Float64s(cp)
	idx := int(math.Floor((1 - confidence) * float64(len(cp))))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	loss := -cp[idx] * equityUSD
	if loss < 0 {
		return 0
	}
	return loss
}

func ParametricVaR(returns []float64, equityUSD, confidence float64) float64 {
	if len(returns) < 2 || equityUSD <= 0 {
		return 0
	}
	mean, std := meanStd(returns)
	z := 1.645
	if confidence >= 0.99 {
		z = 2.326
	}
	loss := -(mean - z*std) * equityUSD
	if loss < 0 {
		return 0
	}
	return loss
}

func MonteCarloVaR(returns []float64, equityUSD, confidence float64, paths int) float64 {
	result := RunMonteCarlo(returns, equityUSD, paths, 1)
	return HistoricalVaR(result.TerminalReturns, equityUSD, confidence)
}

func PortfolioVaR(returns []float64, equityUSD float64) VaRMetrics {
	d95 := HistoricalVaR(returns, equityUSD, 0.95)
	d99 := HistoricalVaR(returns, equityUSD, 0.99)
	return VaRMetrics{
		Daily95USD:   d95,
		Daily99USD:   d99,
		Weekly95USD:  d95 * math.Sqrt(5),
		Weekly99USD:  d99 * math.Sqrt(5),
		Monthly95USD: d95 * math.Sqrt(21),
		Monthly99USD: d99 * math.Sqrt(21),
	}
}

func meanStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	variance := 0.0
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	if len(values) > 1 {
		variance /= float64(len(values) - 1)
	}
	return mean, math.Sqrt(variance)
}
