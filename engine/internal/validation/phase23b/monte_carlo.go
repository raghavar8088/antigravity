package phase23b

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"antigravity-engine/internal/validation/phase22f"
)

// RunMonteCarlo runs 1000-simulation Monte Carlo for every strategy
// using the actual trade return distribution (no assumptions, no GBM).
func RunMonteCarlo(replays []StrategyReplayResult, initialCapital float64) map[string]RealMCResult {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	results := make(map[string]RealMCResult, len(replays))
	for _, r := range replays {
		if len(r.Trades) < 30 {
			results[r.StrategyName] = RealMCResult{
				StrategyName: r.StrategyName,
				Simulations:  MCRuns,
				InputTrades:  len(r.Trades),
				Stability:    phase22f.MCFailed,
			}
			continue
		}
		results[r.StrategyName] = runMC(r.StrategyName, r.Trades, initialCapital, rng)
	}
	return results
}

func runMC(name string, trades []CertifiedTrade, initialCapital float64, rng *rand.Rand) RealMCResult {
	returns := make([]float64, len(trades))
	for i, t := range trades {
		returns[i] = t.NetPnLUSD
	}

	terminalPnLs := make([]float64, MCRuns)
	maxDDs := make([]float64, MCRuns)
	buf := make([]float64, len(returns))

	ruinThreshold := -initialCapital * 0.50

	for sim := 0; sim < MCRuns; sim++ {
		copy(buf, returns)
		// Fisher-Yates shuffle
		for i := len(buf) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			buf[i], buf[j] = buf[j], buf[i]
		}
		terminalPnLs[sim] = sum(buf)
		maxDDs[sim] = maxDrawdownFromPnLs(buf, initialCapital)
	}

	sort.Float64s(terminalPnLs)
	sort.Float64s(maxDDs)

	n := MCRuns
	pctile := func(pct float64) float64 {
		idx := int(float64(n)*pct) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return terminalPnLs[idx]
	}
	pctileDD := func(pct float64) float64 {
		idx := int(float64(n)*pct) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		return maxDDs[idx]
	}

	ruinCount, growCount := 0, 0
	for _, p := range terminalPnLs {
		if p < ruinThreshold {
			ruinCount++
		}
		if p > 0 {
			growCount++
		}
	}

	res := RealMCResult{
		StrategyName:  name,
		Simulations:   MCRuns,
		InputTrades:   len(trades),
		P10PnL:        pctile(0.10),
		P25PnL:        pctile(0.25),
		P50PnL:        pctile(0.50),
		P75PnL:        pctile(0.75),
		P90PnL:        pctile(0.90),
		P10DD:         pctileDD(0.10),
		P50DD:         pctileDD(0.50),
		P90DD:         pctileDD(0.90),
		MaxDD:         pctileDD(1.00),
		RiskOfRuin:    float64(ruinCount) / float64(n),
		PctProfitable: float64(growCount) / float64(n),
	}
	res.Stability = classifyMCStability(res)
	return res
}

func classifyMCStability(r RealMCResult) phase22f.MCStabilityF22 {
	switch {
	case r.RiskOfRuin > 0.25:
		return phase22f.MCFailed
	case r.P10PnL < 0 && r.P25PnL < 0:
		return phase22f.MCUnstable
	case r.P25PnL < 0:
		return phase22f.MCMarginal
	case r.PctProfitable >= 0.75 && r.RiskOfRuin < 0.05:
		return phase22f.MCRobust
	case r.PctProfitable >= 0.60 && r.RiskOfRuin < 0.10:
		return phase22f.MCStable22
	default:
		return phase22f.MCMarginal
	}
}

func sum(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s
}

func maxDrawdownFromPnLs(pnls []float64, initialCapital float64) float64 {
	equity := initialCapital
	peak := equity
	maxDD := 0.0
	for _, p := range pnls {
		equity += p
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			dd := (peak - equity) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	return math.Max(0, maxDD)
}
