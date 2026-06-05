package phase22f

import (
	"math"
	"math/rand"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

const bootstrapIterations = 2000

// ComputeConfidenceIntervals runs bootstrap resampling to compute
// 90/95/99% confidence intervals for PF, Sharpe, Expectancy, and WinRate
// at both the portfolio level and per-strategy.
func ComputeConfidenceIntervals(trades []phase22e.TradeRecord, rng *rand.Rand) ConfidenceAnalysis {
	ca := ConfidenceAnalysis{GeneratedAt: time.Now().UTC()}

	if len(trades) < 30 {
		return ca
	}

	// portfolio-level CI
	ca.Portfolio = bootstrapCI("PORTFOLIO", trades, rng)

	// per-strategy CI
	byStrat := GroupTradesByStrategy(trades)
	for _, id := range SortedStrategyIDs(byStrat) {
		st := byStrat[id]
		if len(st) < 30 {
			continue
		}
		sci := bootstrapCI(id, st, rng)
		if len(st) > 0 {
			sci.StrategyName = st[0].StrategyName
		}
		ca.Strategies = append(ca.Strategies, sci)
	}
	return ca
}

// bootstrapCI computes confidence intervals for one trade series.
func bootstrapCI(strategyID string, trades []phase22e.TradeRecord, rng *rand.Rand) StrategyCI {
	sci := StrategyCI{
		StrategyID: strategyID,
		TradeCount: len(trades),
	}
	if len(trades) == 0 {
		return sci
	}

	n := len(trades)
	pfSamples := make([]float64, 0, bootstrapIterations)
	sharpeSamples := make([]float64, 0, bootstrapIterations)
	expSamples := make([]float64, 0, bootstrapIterations)
	wrSamples := make([]float64, 0, bootstrapIterations)

	buf := make([]phase22e.TradeRecord, n)
	for iter := 0; iter < bootstrapIterations; iter++ {
		// resample with replacement
		for i := 0; i < n; i++ {
			buf[i] = trades[rng.Intn(n)]
		}
		pf, sharpe, exp, wr := sampleMetrics(buf)
		pfSamples = append(pfSamples, pf)
		sharpeSamples = append(sharpeSamples, sharpe)
		expSamples = append(expSamples, exp)
		wrSamples = append(wrSamples, wr)
	}

	// point estimates from original data
	origPF, origSharpe, origExp, origWR := sampleMetrics(trades)

	sci.ProfitFactor = buildCI("ProfitFactor", origPF, pfSamples)
	sci.Sharpe = buildCI("Sharpe", origSharpe, sharpeSamples)
	sci.Expectancy = buildCI("Expectancy", origExp, expSamples)
	sci.WinRate = buildCI("WinRate", origWR, wrSamples)
	return sci
}

func buildCI(metric string, point float64, samples []float64) ConfidenceInterval {
	ci := ConfidenceInterval{Metric: metric, Point: point}
	if len(samples) == 0 {
		return ci
	}
	sortedSamples := make([]float64, len(samples))
	copy(sortedSamples, samples)
	sortFloat64s(sortedSamples)

	n := len(sortedSamples)
	ci.CI90Low = sortedSamples[int(float64(n)*0.05)]
	ci.CI90High = sortedSamples[int(float64(n)*0.95)-1]
	ci.CI95Low = sortedSamples[int(float64(n)*0.025)]
	ci.CI95High = sortedSamples[int(float64(n)*0.975)-1]
	ci.CI99Low = sortedSamples[int(float64(n)*0.005)]
	ci.CI99High = sortedSamples[int(float64(n)*0.995)-1]
	ci.Reliable = point >= ci.CI90Low && point <= ci.CI90High
	return ci
}

func sampleMetrics(trades []phase22e.TradeRecord) (pf, sharpe, exp, wr float64) {
	if len(trades) == 0 {
		return
	}
	wins, losses := 0, 0
	gw, gl := 0.0, 0.0
	pnls := make([]float64, len(trades))
	for i, t := range trades {
		pnls[i] = t.NetPnLUSD
		if t.NetPnLUSD >= 0 {
			wins++
			gw += t.NetPnLUSD
		} else {
			losses++
			gl += -t.NetPnLUSD
		}
	}
	n := float64(len(trades))
	wr = float64(wins) / n
	exp = (gw - gl) / n
	if gl > 0 {
		pf = gw / gl
	}
	m := meanLocal(pnls)
	s := stddevLocal(pnls)
	if s > 0 {
		sharpe = (m / s) * math.Sqrt(n)
	}
	return
}

func sortFloat64s(xs []float64) {
	// insertion sort for small n, otherwise use stdlib sort indirectly
	n := len(xs)
	for i := 1; i < n; i++ {
		key := xs[i]
		j := i - 1
		for j >= 0 && xs[j] > key {
			xs[j+1] = xs[j]
			j--
		}
		xs[j+1] = key
	}
}
