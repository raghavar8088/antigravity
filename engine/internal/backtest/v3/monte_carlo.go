package v3

import (
	"math"
	"sort"
)

// MonteCarloV3Config configures the V3 Monte Carlo simulation.
type MonteCarloV3Config struct {
	Paths                    int
	Seed                     int64
	UseStudentT              bool    // fat-tail distribution vs Gaussian
	DegreesOfFreedom         float64 // Student-t degrees of freedom (3–6 typical for crypto)
	BootstrapWithReplacement bool    // true = bootstrap, false = shuffle
	SlippageShockBps         float64
	FundingShockUSD          float64
}

// MonteCarloV3Report is the output of the V3 Monte Carlo engine.
type MonteCarloV3Report struct {
	Paths         int
	WorstCase     float64
	BestCase      float64
	Median        float64
	Mean          float64
	StdDev        float64
	Percentile1   float64
	Percentile5   float64
	Percentile25  float64
	Percentile75  float64
	Percentile95  float64
	Percentile99  float64
	SurvivalRate  float64 // % paths with positive terminal PnL
	RiskOfRuin    float64 // % paths with drawdown > 50%
	ExpectedSharpe [2]float64 // [5th percentile, 95th percentile] of per-path Sharpe
	ExpectedCAGR   [2]float64 // [5th, 95th] of annualized CAGR
	DrawdownDist   DrawdownDistribution
	TerminalPnLs   []float64
}

// DrawdownDistribution summarizes drawdown outcomes across paths.
type DrawdownDistribution struct {
	Mean    float64
	Median  float64
	P95     float64
	P99     float64
	MaxEver float64
}

// RunMonteCarloV3 simulates Paths paths using bootstrap resampling with fat-tail noise.
// It produces PnL distributions, drawdown distributions, risk of ruin, and Sharpe/CAGR ranges.
func RunMonteCarloV3(trades []V3Trade, cfg MonteCarloV3Config) MonteCarloV3Report {
	paths := cfg.Paths
	if paths <= 0 {
		paths = 5000
	}
	if cfg.DegreesOfFreedom <= 0 {
		cfg.DegreesOfFreedom = 4
	}

	rng := newSimpleRNG(cfg.Seed)
	n := len(trades)
	if n == 0 {
		return MonteCarloV3Report{Paths: paths}
	}

	// Extract return series from trades.
	returns := make([]float64, n)
	initialCapital := 1_000_000.0 // normalized base
	for i, t := range trades {
		returns[i] = t.NetPnL / initialCapital
	}

	terminalPnLs := make([]float64, 0, paths)
	survivals := 0
	ruins := 0
	perPathSharpes := make([]float64, 0, paths)
	perPathCAGRs := make([]float64, 0, paths)
	drawdowns := make([]float64, 0, paths)

	for p := 0; p < paths; p++ {
		// Sample with replacement (bootstrap) or shuffle.
		sampled := make([]float64, n)
		if cfg.BootstrapWithReplacement {
			for i := range sampled {
				idx := int(rng.Uint64() % uint64(n))
				sampled[i] = returns[idx]
			}
		} else {
			copy(sampled, returns)
			shuffleF64(sampled, rng)
		}

		// Apply execution noise using Student-t or Gaussian.
		pathReturns := make([]float64, n)
		for i, r := range sampled {
			var noise float64
			if cfg.UseStudentT {
				noise = studentTNoise(rng, cfg.DegreesOfFreedom) * cfg.SlippageShockBps / 10_000
			} else {
				noise = rng.NormFloat64() * cfg.SlippageShockBps / 10_000
			}
			fundingNoise := rng.NormFloat64() * cfg.FundingShockUSD / initialCapital
			pathReturns[i] = r - math.Abs(noise) + fundingNoise
		}

		// Compute path equity curve.
		equity := 1.0
		peakEquity := 1.0
		maxDD := 0.0
		for _, r := range pathReturns {
			equity *= (1 + r)
			if equity > peakEquity {
				peakEquity = equity
			}
			dd := (peakEquity - equity) / peakEquity
			if dd > maxDD {
				maxDD = dd
			}
		}

		terminalPnL := (equity - 1) * initialCapital
		terminalPnLs = append(terminalPnLs, terminalPnL)
		drawdowns = append(drawdowns, maxDD*100)

		if terminalPnL > 0 {
			survivals++
		}
		if maxDD > 0.50 {
			ruins++
		}

		// Per-path Sharpe (annualized, assuming ~8 trades/day).
		sharpe := annualizedSharpe(pathReturns)
		perPathSharpes = append(perPathSharpes, sharpe)

		// Per-path CAGR assuming 250 trading days of trades.
		tradingDays := float64(n) / 8.0
		if tradingDays < 1 {
			tradingDays = 1
		}
		years := tradingDays / 250
		if equity > 0 && years > 0 {
			cagr := (math.Pow(equity, 1/years) - 1) * 100
			perPathCAGRs = append(perPathCAGRs, cagr)
		}
	}

	sort.Float64s(terminalPnLs)
	sort.Float64s(drawdowns)
	sort.Float64s(perPathSharpes)
	sort.Float64s(perPathCAGRs)

	return MonteCarloV3Report{
		Paths:        paths,
		WorstCase:    quantile(terminalPnLs, 0),
		BestCase:     quantile(terminalPnLs, 1),
		Median:       quantile(terminalPnLs, 0.5),
		Mean:         meanSlice(terminalPnLs),
		StdDev:       stdDev(terminalPnLs),
		Percentile1:  quantile(terminalPnLs, 0.01),
		Percentile5:  quantile(terminalPnLs, 0.05),
		Percentile25: quantile(terminalPnLs, 0.25),
		Percentile75: quantile(terminalPnLs, 0.75),
		Percentile95: quantile(terminalPnLs, 0.95),
		Percentile99: quantile(terminalPnLs, 0.99),
		SurvivalRate: float64(survivals) / float64(paths) * 100,
		RiskOfRuin:   float64(ruins) / float64(paths) * 100,
		ExpectedSharpe: [2]float64{
			quantile(perPathSharpes, 0.05),
			quantile(perPathSharpes, 0.95),
		},
		ExpectedCAGR: [2]float64{
			quantile(perPathCAGRs, 0.05),
			quantile(perPathCAGRs, 0.95),
		},
		DrawdownDist: DrawdownDistribution{
			Mean:    meanSlice(drawdowns),
			Median:  quantile(drawdowns, 0.5),
			P95:     quantile(drawdowns, 0.95),
			P99:     quantile(drawdowns, 0.99),
			MaxEver: quantile(drawdowns, 1),
		},
		TerminalPnLs: terminalPnLs,
	}
}

// studentTNoise draws a Student-t sample using the ratio-of-uniforms method.
// df should be >= 2; at df=4 it matches typical crypto fat-tail behavior.
func studentTNoise(rng *simpleRNG, df float64) float64 {
	// Box-Muller for normal component.
	z := rng.NormFloat64()
	// Chi-squared(df) via sum of squares.
	chiSq := 0.0
	for i := 0; i < int(df); i++ {
		u := rng.NormFloat64()
		chiSq += u * u
	}
	if chiSq <= 0 {
		return z
	}
	return z / math.Sqrt(chiSq/df)
}

func shuffleF64(s []float64, rng *simpleRNG) {
	for i := len(s) - 1; i > 0; i-- {
		j := int(rng.Uint64() % uint64(i+1))
		s[i], s[j] = s[j], s[i]
	}
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	idx := q * float64(len(sorted)-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func meanSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := 0.0
	for _, v := range values {
		s += v
	}
	return s / float64(len(values))
}

func stdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := meanSlice(values)
	sq := 0.0
	for _, v := range values {
		d := v - m
		sq += d * d
	}
	return math.Sqrt(sq / float64(len(values)-1))
}

func annualizedSharpe(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}
	m := meanSlice(returns)
	sd := stdDev(returns)
	if sd == 0 {
		return 0
	}
	return m / sd * math.Sqrt(float64(len(returns)))
}
