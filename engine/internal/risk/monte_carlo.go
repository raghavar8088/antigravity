package risk

import (
	"math"
	"math/rand"
)

type MonteCarloResult struct {
	Paths               int
	Steps               int
	ExpectedReturnUSD   float64
	WorstCaseUSD        float64
	ExpectedDrawdownPct float64
	RiskOfRuinPct       float64
	TerminalReturns     []float64
}

func RunMonteCarlo(returns []float64, equityUSD float64, paths, steps int) MonteCarloResult {
	if paths <= 0 {
		paths = 10000
	}
	if steps <= 0 {
		steps = 30
	}
	if equityUSD <= 0 {
		equityUSD = 1
	}
	if len(returns) == 0 {
		returns = []float64{0}
	}
	rng := rand.New(rand.NewSource(42))
	terminal := make([]float64, 0, paths)
	sumTerminal := 0.0
	worst := math.Inf(1)
	ddSum := 0.0
	ruin := 0
	for i := 0; i < paths; i++ {
		equity := equityUSD
		peak := equityUSD
		for step := 0; step < steps; step++ {
			r := returns[rng.Intn(len(returns))]
			equity *= 1 + r
			if equity > peak {
				peak = equity
			}
		}
		if equity < worst {
			worst = equity
		}
		if equity <= equityUSD*0.50 {
			ruin++
		}
		sumTerminal += equity
		ddSum += DrawdownPct(equity, peak)
		terminal = append(terminal, (equity-equityUSD)/equityUSD)
	}
	return MonteCarloResult{
		Paths:               paths,
		Steps:               steps,
		ExpectedReturnUSD:   sumTerminal/float64(paths) - equityUSD,
		WorstCaseUSD:        worst,
		ExpectedDrawdownPct: ddSum / float64(paths),
		RiskOfRuinPct:       float64(ruin) / float64(paths) * 100,
		TerminalReturns:     terminal,
	}
}
