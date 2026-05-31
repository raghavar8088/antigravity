package v2

import (
	"math/rand"
	"sort"
)

type MonteCarloConfig struct {
	Paths            int
	Seed             int64
	SlippageShockBps float64
	FundingShockUSD  float64
	SpreadShockUSD   float64
}

type MonteCarloReport struct {
	Paths              int
	WorstCase          float64
	BestCase           float64
	Median             float64
	Percentile5        float64
	Percentile95       float64
	ConfidenceInterval [2]float64
	SurvivalRate       float64
	TerminalPnLs       []float64
}

func RunMonteCarlo(trades []Trade, cfg MonteCarloConfig) MonteCarloReport {
	paths := cfg.Paths
	if paths <= 0 {
		paths = 1000
	}
	rng := rand.New(rand.NewSource(cfg.Seed))
	terminal := make([]float64, 0, paths)
	survivors := 0
	for i := 0; i < paths; i++ {
		cp := append([]Trade(nil), trades...)
		rng.Shuffle(len(cp), func(a, b int) { cp[a], cp[b] = cp[b], cp[a] })
		pnl := 0.0
		for _, tr := range cp {
			execNoise := rng.NormFloat64() * cfg.SlippageShockBps * tr.Size * tr.EntryPrice / 10_000
			fundingNoise := rng.NormFloat64() * cfg.FundingShockUSD
			spreadNoise := rng.NormFloat64() * cfg.SpreadShockUSD
			pnl += tr.NetPnL - abs(execNoise) + fundingNoise - abs(spreadNoise)
		}
		if pnl > 0 {
			survivors++
		}
		terminal = append(terminal, pnl)
	}
	sort.Float64s(terminal)
	return MonteCarloReport{
		Paths:              paths,
		WorstCase:          quantileSorted(terminal, 0),
		BestCase:           quantileSorted(terminal, 1),
		Median:             quantileSorted(terminal, 0.5),
		Percentile5:        quantileSorted(terminal, 0.05),
		Percentile95:       quantileSorted(terminal, 0.95),
		ConfidenceInterval: [2]float64{quantileSorted(terminal, 0.025), quantileSorted(terminal, 0.975)},
		SurvivalRate:       float64(survivors) / float64(paths) * 100,
		TerminalPnLs:       terminal,
	}
}

func quantileSorted(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		return values[0]
	}
	if q >= 1 {
		return values[len(values)-1]
	}
	idx := int(q * float64(len(values)-1))
	return values[idx]
}
