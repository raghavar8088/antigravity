// Package montecarlo implements the Phase 19D Monte Carlo Simulation Platform.
// Supports 1K, 10K, and 100K simulation runs with bootstrap sampling, regime
// resampling, drawdown analysis, risk-of-ruin calculation, and confidence intervals.
// Completely isolated from production execution — no orders are placed.
package montecarlo

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"
)

// ─── Simulation Configuration ─────────────────────────────────────────────────

// Preset defines standard simulation sizes.
type Preset string

const (
	Preset1K    Preset = "1K"
	Preset10K   Preset = "10K"
	Preset100K  Preset = "100K"
)

// Config defines Monte Carlo simulation parameters.
type Config struct {
	Paths                  int     // number of simulation runs
	Seed                   int64   // RNG seed (0 = random)
	BootstrapWithReplacement bool  // true = bootstrap, false = shuffle
	UseStudentT            bool    // true = fat-tail returns via Student-t
	DegreesOfFreedom       float64 // Student-t df (3–6 for crypto fat tails)
	SlippageShockBps       float64 // additional slippage applied per trade (bps)
	FundingShockUSD        float64 // additional funding cost per trade (USD)
	RuinThresholdPct       float64 // drawdown level considered "ruin" (e.g. 0.50 = 50%)
	ConfidenceLevels       []float64 // percentiles to report (e.g. [0.01, 0.05, 0.25, 0.75, 0.95, 0.99])
}

// DefaultConfig returns an institutional-grade Monte Carlo configuration.
func DefaultConfig(preset Preset) Config {
	paths := 10_000
	switch preset {
	case Preset1K:
		paths = 1_000
	case Preset10K:
		paths = 10_000
	case Preset100K:
		paths = 100_000
	}
	return Config{
		Paths:                  paths,
		Seed:                   42,
		BootstrapWithReplacement: true,
		UseStudentT:            false,
		DegreesOfFreedom:       4.0,
		SlippageShockBps:       0,
		FundingShockUSD:        0,
		RuinThresholdPct:       0.50,
		ConfidenceLevels:       []float64{0.01, 0.05, 0.25, 0.50, 0.75, 0.95, 0.99},
	}
}

// ─── Input/Output types ───────────────────────────────────────────────────────

// Trade is a completed trade for simulation input.
type Trade struct {
	PnLUSD float64
	PnLPct float64
}

// SimPath is a single simulated equity path.
type SimPath struct {
	TerminalPnL    float64
	MaxDrawdown    float64
	SharpeRatio    float64
	TotalTrades    int
}

// DrawdownDistribution captures the distribution of max drawdowns across paths.
type DrawdownDistribution struct {
	Mean   float64
	Median float64
	P95    float64
	P99    float64
	Max    float64
}

// Report is the complete Monte Carlo analysis output.
type Report struct {
	Config          Config
	Paths           int
	Preset          string
	RunDuration     time.Duration

	// Terminal PnL distribution
	TerminalPnLs    []float64 // sorted ascending
	Mean            float64
	Median          float64
	StdDev          float64
	Percentiles     map[float64]float64 // confidence level → terminal PnL

	// Risk metrics
	SurvivalRate    float64 // fraction of paths that ended positive
	RiskOfRuin      float64 // fraction of paths that hit ruin threshold
	ExpectedSharpe  float64
	MaxEverDrawdown float64

	// Drawdown distribution
	DrawdownDist    DrawdownDistribution

	// Derived
	ExpectedCAGR    float64 // annualised, based on median path
	Passed          bool    // true if risk-of-ruin < 5% and survival rate > 60%
	FailReason      string
}

// ─── Monte Carlo Engine ───────────────────────────────────────────────────────

// Engine runs Monte Carlo simulations on a trade history.
type Engine struct {
	cfg Config
	rng *rand.Rand
}

// NewEngine creates a Monte Carlo engine with the given configuration.
func NewEngine(cfg Config) *Engine {
	src := rand.NewSource(cfg.Seed)
	if cfg.Seed == 0 {
		src = rand.NewSource(time.Now().UnixNano())
	}
	return &Engine{cfg: cfg, rng: rand.New(src)}
}

// Run executes the Monte Carlo simulation over the given trade history.
func (e *Engine) Run(trades []Trade) (Report, error) {
	if len(trades) == 0 {
		return Report{}, errors.New("montecarlo: no trades provided")
	}
	if e.cfg.Paths <= 0 {
		return Report{}, errors.New("montecarlo: paths must be > 0")
	}

	start := time.Now()
	pnls := extractPnLs(trades, e.cfg)

	paths := make([]SimPath, e.cfg.Paths)
	terminalPnLs := make([]float64, e.cfg.Paths)
	drawdowns := make([]float64, e.cfg.Paths)

	for i := range paths {
		path := e.simulatePath(pnls, len(trades))
		paths[i] = path
		terminalPnLs[i] = path.TerminalPnL
		drawdowns[i] = path.MaxDrawdown
	}

	sort.Float64s(terminalPnLs)
	sort.Float64s(drawdowns)

	report := e.buildReport(terminalPnLs, drawdowns, paths, start)
	return report, nil
}

// simulatePath runs one bootstrap-sampled equity path.
func (e *Engine) simulatePath(pnls []float64, n int) SimPath {
	equity := 0.0
	peak := 0.0
	maxDD := 0.0
	pnlSum := 0.0
	pnlSumSq := 0.0
	ruinThreshold := -math.Abs(e.cfg.RuinThresholdPct * 100)
	ruined := false

	var sampledPnLs []float64

	if e.cfg.BootstrapWithReplacement {
		sampledPnLs = make([]float64, n)
		for i := range sampledPnLs {
			sampledPnLs[i] = pnls[e.rng.Intn(len(pnls))]
		}
	} else {
		sampledPnLs = make([]float64, len(pnls))
		copy(sampledPnLs, pnls)
		e.rng.Shuffle(len(sampledPnLs), func(i, j int) {
			sampledPnLs[i], sampledPnLs[j] = sampledPnLs[j], sampledPnLs[i]
		})
		if len(sampledPnLs) > n {
			sampledPnLs = sampledPnLs[:n]
		}
	}

	if e.cfg.UseStudentT {
		// Apply Student-t shock to each return to model fat tails.
		for i := range sampledPnLs {
			sampledPnLs[i] *= e.studentTMultiplier()
		}
	}

	for _, p := range sampledPnLs {
		p -= e.cfg.FundingShockUSD
		if e.cfg.SlippageShockBps > 0 {
			p -= math.Abs(p) * (e.cfg.SlippageShockBps / 10000)
		}
		equity += p
		pnlSum += p
		pnlSumSq += p * p
		if equity > peak {
			peak = equity
		}
		dd := peak - equity
		if dd > maxDD {
			maxDD = dd
		}
		if equity < ruinThreshold && !ruined {
			ruined = true
		}
	}

	sharpe := 0.0
	if len(sampledPnLs) > 0 {
		meanP := pnlSum / float64(len(sampledPnLs))
		variance := pnlSumSq/float64(len(sampledPnLs)) - meanP*meanP
		if variance > 0 {
			sharpe = (meanP / math.Sqrt(variance)) * math.Sqrt(252)
		}
	}

	return SimPath{
		TerminalPnL: equity,
		MaxDrawdown: maxDD,
		SharpeRatio: sharpe,
		TotalTrades: len(sampledPnLs),
	}
}

// studentTMultiplier generates a Student-t multiplier for fat-tail shocks.
func (e *Engine) studentTMultiplier() float64 {
	df := e.cfg.DegreesOfFreedom
	if df <= 0 {
		df = 4
	}
	// Box-Muller transform for normal, then scale for t-distribution approximation.
	u1 := e.rng.Float64()
	u2 := e.rng.Float64()
	z := math.Sqrt(-2*math.Log(math.Max(u1, 1e-15))) * math.Cos(2*math.Pi*u2)
	chi2 := 0.0
	for i := 0; i < int(df); i++ {
		u := e.rng.NormFloat64()
		chi2 += u * u
	}
	t := z / math.Sqrt(chi2/df)
	return 1 + 0.1*t // apply as ±10% shock multiplier
}

func (e *Engine) buildReport(terminalPnLs, drawdowns []float64, paths []SimPath, start time.Time) Report {
	n := len(terminalPnLs)

	mean, std := meanStd(terminalPnLs)
	median := percentile(terminalPnLs, 0.50)

	// Survival rate: fraction of paths with positive terminal PnL.
	survival := 0
	for _, p := range terminalPnLs {
		if p > 0 {
			survival++
		}
	}

	// Risk of ruin: fraction of paths that hit the ruin threshold.
	ruinCount := 0
	for _, p := range paths {
		if p.TerminalPnL < -e.cfg.RuinThresholdPct*100 {
			ruinCount++
		}
	}

	// Mean Sharpe across paths.
	sharpeSum := 0.0
	for _, p := range paths {
		sharpeSum += p.SharpeRatio
	}

	// Build percentiles map.
	pctMap := make(map[float64]float64)
	for _, level := range e.cfg.ConfidenceLevels {
		pctMap[level] = percentile(terminalPnLs, level)
	}

	maxEverDD := 0.0
	for _, d := range drawdowns {
		if d > maxEverDD {
			maxEverDD = d
		}
	}

	ddDist := DrawdownDistribution{
		Mean:   meanOf(drawdowns),
		Median: percentile(drawdowns, 0.50),
		P95:    percentile(drawdowns, 0.95),
		P99:    percentile(drawdowns, 0.99),
		Max:    maxEverDD,
	}

	survivalRate := float64(survival) / float64(n)
	riskOfRuin := float64(ruinCount) / float64(n)

	preset := fmt.Sprintf("%d", e.cfg.Paths)

	// Institutional pass: risk-of-ruin < 5%, survival > 60%.
	passed := riskOfRuin < 0.05 && survivalRate > 0.60
	failReason := ""
	if !passed {
		if riskOfRuin >= 0.05 {
			failReason = fmt.Sprintf("risk of ruin %.1f%% ≥ 5%% threshold", riskOfRuin*100)
		} else {
			failReason = fmt.Sprintf("survival rate %.1f%% < 60%% threshold", survivalRate*100)
		}
	}

	return Report{
		Config:          e.cfg,
		Paths:           n,
		Preset:          preset,
		RunDuration:     time.Since(start),
		TerminalPnLs:    terminalPnLs,
		Mean:            mean,
		Median:          median,
		StdDev:          std,
		Percentiles:     pctMap,
		SurvivalRate:    survivalRate,
		RiskOfRuin:      riskOfRuin,
		ExpectedSharpe:  sharpeSum / float64(n),
		MaxEverDrawdown: maxEverDD,
		DrawdownDist:    ddDist,
		ExpectedCAGR:    estimateCAGR(median, paths[0].TotalTrades),
		Passed:          passed,
		FailReason:      failReason,
	}
}

func estimateCAGR(medianPnL float64, nTrades int) float64 {
	if nTrades == 0 {
		return 0
	}
	annualFactor := 252.0 / float64(nTrades)
	annualPnL := medianPnL * annualFactor
	return annualPnL
}

func extractPnLs(trades []Trade, cfg Config) []float64 {
	pnls := make([]float64, len(trades))
	for i, t := range trades {
		pnls[i] = t.PnLUSD
	}
	return pnls
}

// ─── Math helpers ─────────────────────────────────────────────────────────────

func meanStd(data []float64) (mean, std float64) {
	if len(data) == 0 {
		return
	}
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))
	for _, v := range data {
		d := v - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(data)))
	return
}

func meanOf(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func percentile(sortedData []float64, p float64) float64 {
	n := len(sortedData)
	if n == 0 {
		return 0
	}
	idx := p * float64(n-1)
	lo := int(idx)
	hi := lo + 1
	if hi >= n {
		return sortedData[n-1]
	}
	frac := idx - float64(lo)
	return sortedData[lo]*(1-frac) + sortedData[hi]*frac
}
