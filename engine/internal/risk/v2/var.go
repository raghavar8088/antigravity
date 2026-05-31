package v2

import "math"

type VaRReport struct {
	DailyVaR95USD   float64
	DailyVaR99USD   float64
	WeeklyVaR95USD  float64
	MonthlyVaR95USD float64
	DailyVaRPct     float64
	Method          string
	Breached        bool
	Escalating      bool
}

func CalculateVaR(returns []float64, equityUSD float64) VaRReport {
	if equityUSD <= 0 {
		equityUSD = 100_000
	}
	h95 := HistoricalVaR(returns, equityUSD, 0.95)
	h99 := HistoricalVaR(returns, equityUSD, 0.99)
	param := ParametricVaR(returns, equityUSD, 0.95)
	mc := MonteCarloVaR(returns, equityUSD, 0.95, 1000)
	daily := max(h95, max(param, mc))
	return VaRReport{
		DailyVaR95USD:   daily,
		DailyVaR99USD:   h99,
		WeeklyVaR95USD:  daily * math.Sqrt(5),
		MonthlyVaR95USD: daily * math.Sqrt(21),
		DailyVaRPct:     daily / equityUSD * 100,
		Method:          "max(historical,parametric,monte_carlo)",
		Breached:        daily/equityUSD*100 > 6,
		Escalating:      len(returns) >= 5 && mean(tail(returns, 5)) < 0,
	}
}

func HistoricalVaR(returns []float64, equityUSD, confidence float64) float64 {
	if len(returns) == 0 || equityUSD <= 0 {
		return 0
	}
	q := percentile(returns, 1-confidence)
	if q >= 0 {
		return 0
	}
	return abs(q) * equityUSD
}

func ParametricVaR(returns []float64, equityUSD, confidence float64) float64 {
	if len(returns) < 2 || equityUSD <= 0 {
		return 0
	}
	z := 1.65
	if confidence >= 0.99 {
		z = 2.33
	}
	loss := -(mean(returns) - z*stddev(returns))
	if loss <= 0 {
		return 0
	}
	return loss * equityUSD
}

func MonteCarloVaR(returns []float64, equityUSD, confidence float64, paths int) float64 {
	if len(returns) == 0 {
		return 0
	}
	if paths <= 0 {
		paths = 1000
	}
	sim := make([]float64, 0, paths)
	for i := 0; i < paths; i++ {
		idx := (i*37 + 17) % len(returns)
		sim = append(sim, returns[idx])
	}
	return HistoricalVaR(sim, equityUSD, confidence)
}
