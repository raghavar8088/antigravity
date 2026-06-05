package phase22e

import (
	"math/rand"
	"sort"
)

// MonteCarloResult holds the output of a Monte Carlo simulation run.
type MonteCarloResult struct {
	Simulations     int
	ExpectedReturn  float64 // median terminal P&L across simulations
	BestReturn      float64 // 95th percentile terminal P&L
	WorstReturn     float64 // 5th percentile terminal P&L
	ExpectedDD      float64 // median max drawdown across simulations
	WorstDD         float64 // 95th percentile max drawdown
	ProbabilityRuin float64 // fraction of sims with terminal P&L < -initialNAV*0.50
	ProbabilityGrow float64 // fraction of sims with terminal P&L > 0
	Stability       MCStability
}

// MCStability classifies a strategy's Monte Carlo outcome.
type MCStability string

const (
	MCStable     MCStability = "STABLE"
	MCMarginal   MCStability = "MARGINAL"
	MCUnstable   MCStability = "UNSTABLE"
	MCUntradable MCStability = "UNTRADABLE"
)

// RunMonteCarlo simulates n shuffle permutations of the trade returns,
// computing terminal P&L and max drawdown for each permutation.
func RunMonteCarlo(trades []TradeRecord, initialNAV float64, n int) MonteCarloResult {
	if len(trades) == 0 || n <= 0 {
		return MonteCarloResult{Simulations: 0, Stability: MCUntradable}
	}

	// Extract return series
	returns := make([]float64, len(trades))
	for i, t := range trades {
		returns[i] = t.NetPnLUSD
	}

	rng := rand.New(rand.NewSource(42))
	terminalPnLs := make([]float64, n)
	maxDDs := make([]float64, n)

	buf := make([]float64, len(returns))
	for sim := 0; sim < n; sim++ {
		copy(buf, returns)
		// Fisher-Yates shuffle
		for i := len(buf) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			buf[i], buf[j] = buf[j], buf[i]
		}
		terminalPnLs[sim] = sum(buf)
		maxDDs[sim] = maxDrawdownPct(buf, initialNAV)
	}

	sort.Float64s(terminalPnLs)
	sort.Float64s(maxDDs)

	p5idx := int(float64(n) * 0.05)
	p95idx := int(float64(n) * 0.95)
	if p95idx >= n {
		p95idx = n - 1
	}

	// probability of ruin: terminal loss > 50% of initial NAV
	ruinThreshold := -initialNAV * 0.50
	ruinCount := 0
	growCount := 0
	for _, pnl := range terminalPnLs {
		if pnl < ruinThreshold {
			ruinCount++
		}
		if pnl > 0 {
			growCount++
		}
	}

	res := MonteCarloResult{
		Simulations:     n,
		WorstReturn:     terminalPnLs[p5idx],
		BestReturn:      terminalPnLs[p95idx],
		ExpectedReturn:  terminalPnLs[n/2],
		ExpectedDD:      maxDDs[n/2],
		WorstDD:         maxDDs[p95idx],
		ProbabilityRuin: float64(ruinCount) / float64(n),
		ProbabilityGrow: float64(growCount) / float64(n),
	}
	res.Stability = classifyMCStability(res)
	return res
}

func classifyMCStability(r MonteCarloResult) MCStability {
	switch {
	case r.ProbabilityRuin > 0.20 || r.WorstDD > 30:
		return MCUntradable
	case r.ProbabilityRuin > 0.10 || r.WorstDD > 20:
		return MCUnstable
	case r.ProbabilityRuin > 0.05 || r.ProbabilityGrow < 0.60:
		return MCMarginal
	default:
		return MCStable
	}
}

func sum(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}
