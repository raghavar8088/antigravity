package phase22f

import (
	"math"
	"math/rand"
	"sort"

	"antigravity-engine/internal/validation/phase22e"
)

// RunMonteCarloF22 runs MCSimulations (1000) shuffle permutations of the trade
// returns, capturing terminal P&L and max drawdown for each run.
// Returns a fully classified MonteCarloF22 result.
func RunMonteCarloF22(strategyID string, trades []phase22e.TradeRecord, initialNAV float64, rng *rand.Rand) MonteCarloF22 {
	res := MonteCarloF22{
		StrategyID:  strategyID,
		Simulations: MCSimulations,
		Stability:   MCFailed,
	}
	if len(trades) == 0 || initialNAV <= 0 {
		return res
	}

	returns := make([]float64, len(trades))
	for i, t := range trades {
		returns[i] = t.NetPnLUSD
	}

	terminalPnLs := make([]float64, MCSimulations)
	maxDDs := make([]float64, MCSimulations)
	buf := make([]float64, len(returns))

	for sim := 0; sim < MCSimulations; sim++ {
		copy(buf, returns)
		// Fisher-Yates shuffle
		for i := len(buf) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			buf[i], buf[j] = buf[j], buf[i]
		}
		terminalPnLs[sim] = sumSlice(buf)
		maxDDs[sim] = maxDrawdownPctLocal(buf, initialNAV)
	}

	sort.Float64s(terminalPnLs)
	sort.Float64s(maxDDs)

	n := MCSimulations
	idx := func(pct float64) int {
		i := int(float64(n)*pct) - 1
		if i < 0 {
			i = 0
		}
		if i >= n {
			i = n - 1
		}
		return i
	}

	ruinThreshold := -initialNAV * 0.50
	ruinCount, growCount := 0, 0
	for _, pnl := range terminalPnLs {
		if pnl < ruinThreshold {
			ruinCount++
		}
		if pnl > 0 {
			growCount++
		}
	}

	res.WorstReturn = terminalPnLs[idx(0.05)]
	res.P10Return = terminalPnLs[idx(0.10)]
	res.P25Return = terminalPnLs[idx(0.25)]
	res.ExpectedReturn = terminalPnLs[n/2]
	res.P75Return = terminalPnLs[idx(0.75)]
	res.P90Return = terminalPnLs[idx(0.90)]
	res.BestReturn = terminalPnLs[idx(0.95)]
	res.ExpectedDD = maxDDs[n/2]
	res.WorstDD = maxDDs[idx(0.95)]
	res.ProbabilityRuin = float64(ruinCount) / float64(n)
	res.ProbabilityGrow = float64(growCount) / float64(n)
	res.CapSurvivalRate = 1.0 - res.ProbabilityRuin
	res.TailRisk = math.Abs(cvarAt5Pct(terminalPnLs))
	res.Stability = classifyMCF22(res)
	return res
}

// RunPortfolioMC runs Monte Carlo on the full combined trade list.
func RunPortfolioMC(trades []phase22e.TradeRecord, initialNAV float64, rng *rand.Rand) MonteCarloF22 {
	mc := RunMonteCarloF22("PORTFOLIO", trades, initialNAV, rng)
	return mc
}

func classifyMCF22(r MonteCarloF22) MCStabilityF22 {
	switch {
	case r.ProbabilityRuin > 0.20 || r.WorstDD > 30:
		return MCFailed
	case r.ProbabilityRuin > 0.10 || r.WorstDD > 20:
		return MCUnstable
	case r.ProbabilityRuin > 0.05 || r.ProbabilityGrow < 0.60:
		return MCMarginal
	case r.ProbabilityRuin > 0.01 || r.ProbabilityGrow < 0.80:
		return MCStable22
	default:
		return MCRobust
	}
}

func cvarAt5Pct(sorted []float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	cutoff := int(float64(len(sorted)) * 0.05)
	if cutoff == 0 {
		cutoff = 1
	}
	s := 0.0
	for i := 0; i < cutoff; i++ {
		s += sorted[i]
	}
	return s / float64(cutoff)
}
