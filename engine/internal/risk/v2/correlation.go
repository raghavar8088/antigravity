package v2

import "math"

type CorrelationReport struct {
	StrategyCorrelation  map[string]float64
	PositionCorrelation  map[string]float64
	PortfolioCorrelation float64
	MaxCorrelation       float64
	ClusterAllocationPct float64
	HiddenConcentration  bool
	Matrix               map[string]map[string]float64
}

func CalculateCorrelationReport(portfolio Portfolio, req TradeRequest) CorrelationReport {
	matrix := make(map[string]map[string]float64)
	strategyCorr := make(map[string]float64)
	maxCorr := 0.0
	clusterNotional := 0.0
	for name, stats := range portfolio.StrategyMetrics {
		corr := Pearson(tail(stats.Returns, 100), tail(portfolio.StrategyMetrics[req.Strategy].Returns, 100))
		if name == req.Strategy {
			corr = 1
		}
		strategyCorr[name] = corr
		if abs(corr) > maxCorr && name != req.Strategy {
			maxCorr = abs(corr)
		}
		matrix[name] = map[string]float64{req.Strategy: corr}
	}
	for _, pos := range portfolio.Positions {
		sameSide := pos.Side == req.Side
		sameFamily := pos.Family == req.Family
		if sameSide && (sameFamily || pos.Symbol == req.Symbol) {
			clusterNotional += PositionNotionalUSD(pos)
		}
	}
	equity := max(portfolio.Account.EquityUSD, 1)
	clusterPct := clusterNotional / equity * 100
	return CorrelationReport{
		StrategyCorrelation:  strategyCorr,
		PositionCorrelation:  map[string]float64{req.Symbol: maxCorr},
		PortfolioCorrelation: maxCorr,
		MaxCorrelation:       maxCorr,
		ClusterAllocationPct: clusterPct,
		HiddenConcentration:  clusterPct > 35 || maxCorr > 0.80,
		Matrix:               matrix,
	}
}

func Pearson(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 2 {
		return 0
	}
	a = a[len(a)-n:]
	b = b[len(b)-n:]
	ma, mb := mean(a), mean(b)
	var num, da2, db2 float64
	for i := 0; i < n; i++ {
		da := a[i] - ma
		db := b[i] - mb
		num += da * db
		da2 += da * da
		db2 += db * db
	}
	den := da2 * db2
	if den <= 0 {
		return 0
	}
	return num / math.Sqrt(den)
}

func RollingCorrelation(a, b []float64, window int) float64 {
	return Pearson(tail(a, window), tail(b, window))
}
