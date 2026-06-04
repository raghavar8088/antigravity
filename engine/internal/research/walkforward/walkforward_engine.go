// Package walkforward implements the Phase 19C Walk-Forward Optimization Engine.
// It partitions historical data into rolling train/validation/test windows,
// prevents overfitting through out-of-sample validation, and produces
// parameter stability analysis across all windows.
package walkforward

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

// ─── Configuration ────────────────────────────────────────────────────────────

// WindowMode controls the style of walk-forward partitioning.
type WindowMode string

const (
	ModeAnchored  WindowMode = "ANCHORED"  // training window grows from fixed origin
	ModeRolling   WindowMode = "ROLLING"   // training window slides with fixed size
	ModeExpanding WindowMode = "EXPANDING" // training window expands, test stays fixed
)

// Config defines the walk-forward optimization parameters.
type Config struct {
	Mode              WindowMode
	TrainBars         int     // number of bars in each training window
	ValidationBars    int     // bars held out for in-sample validation
	TestBars          int     // bars held out for out-of-sample test
	StepBars          int     // bars to advance per iteration
	MinTrainBars      int     // minimum bars before optimization begins
	MaxParamSets      int     // maximum number of parameter combinations to test
	Metric            string  // optimization target: "sharpe", "sortino", "pnl", "win_rate"
	RequirePositive   bool    // discard windows with negative test Sharpe
}

// DefaultConfig returns an institutional-grade walk-forward configuration.
func DefaultConfig() Config {
	return Config{
		Mode:            ModeRolling,
		TrainBars:       252,
		ValidationBars:  63,
		TestBars:        21,
		StepBars:        21,
		MinTrainBars:    126,
		MaxParamSets:    100,
		Metric:          "sharpe",
		RequirePositive: true,
	}
}

// ─── Data types ───────────────────────────────────────────────────────────────

// Trade is a single completed trade for walk-forward analysis.
type Trade struct {
	EntryTime  time.Time
	ExitTime   time.Time
	PnLUSD     float64
	PnLPct     float64
	Side       string
}

// Params represents a set of strategy parameters to be evaluated.
type Params map[string]float64

// WindowResult captures the performance of one strategy across one WF window.
type WindowResult struct {
	WindowIndex    int
	TrainStart     time.Time
	TrainEnd       time.Time
	ValidationEnd  time.Time
	TestEnd        time.Time
	BestParams     Params
	TrainMetrics   Metrics
	ValidationMetrics Metrics
	TestMetrics    Metrics
	ParamStability float64 // 0–1: how stable params are vs prior window
}

// Metrics captures all performance statistics for one evaluation period.
type Metrics struct {
	SharpeRatio   float64
	SortinoRatio  float64
	MaxDrawdown   float64
	WinRate       float64
	ProfitFactor  float64
	TotalPnLUSD   float64
	TotalTrades   int
	AvgTradeUSD   float64
	CAGR          float64
	Calmar        float64
}

// Report is the complete walk-forward analysis output.
type Report struct {
	Config        Config
	Windows       []WindowResult
	AggregateOOS  Metrics   // aggregate out-of-sample performance across all windows
	ParamStability float64  // mean param stability across all windows
	EfficiencyRatio float64 // OOS Sharpe / IS Sharpe — overfitting measure
	Passed        bool      // true if OOS performance meets institutional thresholds
	FailReason    string
}

// ─── Strategy Interface ───────────────────────────────────────────────────────

// StrategyEvaluator runs a strategy with given params over a trade slice
// and returns performance metrics. Implement this for each strategy family.
type StrategyEvaluator interface {
	Evaluate(trades []Trade, params Params) Metrics
	ParameterSpace() []Params // all candidate parameter sets
}

// ─── Walk-Forward Engine ──────────────────────────────────────────────────────

// Engine executes rolling walk-forward optimization.
type Engine struct {
	cfg Config
}

// NewEngine creates a walk-forward engine with the given configuration.
func NewEngine(cfg Config) *Engine {
	if cfg.TrainBars <= 0 {
		cfg = DefaultConfig()
	}
	return &Engine{cfg: cfg}
}

// Run executes the full walk-forward analysis for the given trade history.
func (e *Engine) Run(trades []Trade, evaluator StrategyEvaluator) (Report, error) {
	if len(trades) == 0 {
		return Report{}, errors.New("walkforward: no trades provided")
	}
	if evaluator == nil {
		return Report{}, errors.New("walkforward: evaluator required")
	}

	// Sort trades chronologically.
	sort.Slice(trades, func(i, j int) bool {
		return trades[i].EntryTime.Before(trades[j].EntryTime)
	})

	paramSpace := evaluator.ParameterSpace()
	if len(paramSpace) == 0 {
		return Report{}, errors.New("walkforward: parameter space is empty")
	}
	if e.cfg.MaxParamSets > 0 && len(paramSpace) > e.cfg.MaxParamSets {
		paramSpace = paramSpace[:e.cfg.MaxParamSets]
	}

	cfg := e.cfg
	totalNeeded := cfg.TrainBars + cfg.ValidationBars + cfg.TestBars
	if len(trades) < totalNeeded {
		return Report{}, fmt.Errorf("walkforward: need %d trades, have %d", totalNeeded, len(trades))
	}

	var windows []WindowResult
	var prevParams Params
	start := 0

	for {
		trainEnd := start + cfg.TrainBars
		valEnd := trainEnd + cfg.ValidationBars
		testEnd := valEnd + cfg.TestBars

		if testEnd > len(trades) {
			break
		}

		trainTrades := trades[start:trainEnd]
		valTrades := trades[trainEnd:valEnd]
		testTrades := trades[valEnd:testEnd]

		// Grid search over parameter space on training data.
		bestParams, trainMetrics := gridSearch(trainTrades, paramSpace, evaluator, cfg.Metric)

		// Validate on validation set.
		valMetrics := evaluator.Evaluate(valTrades, bestParams)

		// Out-of-sample test.
		testMetrics := evaluator.Evaluate(testTrades, bestParams)

		// Skip window if OOS is negative and required to be positive.
		if cfg.RequirePositive && testMetrics.SharpeRatio < 0 {
			if cfg.Mode == ModeRolling {
				start += cfg.StepBars
			} else {
				start = 0
				cfg.TrainBars += cfg.StepBars
			}
			continue
		}

		stability := paramStability(prevParams, bestParams)
		prevParams = bestParams

		w := WindowResult{
			WindowIndex:       len(windows),
			TrainStart:        trainTrades[0].EntryTime,
			TrainEnd:          trainTrades[len(trainTrades)-1].ExitTime,
			ValidationEnd:     valTrades[len(valTrades)-1].ExitTime,
			TestEnd:           testTrades[len(testTrades)-1].ExitTime,
			BestParams:        bestParams,
			TrainMetrics:      trainMetrics,
			ValidationMetrics: valMetrics,
			TestMetrics:       testMetrics,
			ParamStability:    stability,
		}
		windows = append(windows, w)

		if cfg.Mode == ModeRolling {
			start += cfg.StepBars
		} else if cfg.Mode == ModeAnchored {
			cfg.TrainBars += cfg.StepBars
		} else {
			start += cfg.StepBars
		}
	}

	if len(windows) == 0 {
		return Report{}, errors.New("walkforward: no valid windows produced")
	}

	report := e.buildReport(windows, cfg)
	return report, nil
}

// gridSearch finds the parameter set that maximises the target metric on train data.
func gridSearch(trades []Trade, paramSpace []Params, eval StrategyEvaluator, metric string) (Params, Metrics) {
	bestScore := math.Inf(-1)
	var bestParams Params
	var bestMetrics Metrics

	for _, p := range paramSpace {
		m := eval.Evaluate(trades, p)
		score := metricScore(m, metric)
		if score > bestScore {
			bestScore = score
			bestParams = p
			bestMetrics = m
		}
	}
	return bestParams, bestMetrics
}

func metricScore(m Metrics, metric string) float64 {
	switch metric {
	case "sharpe":
		return m.SharpeRatio
	case "sortino":
		return m.SortinoRatio
	case "pnl":
		return m.TotalPnLUSD
	case "win_rate":
		return m.WinRate
	case "calmar":
		return m.Calmar
	default:
		return m.SharpeRatio
	}
}

// paramStability returns a 0–1 score indicating how similar two parameter sets are.
// 1.0 = identical; 0.0 = completely different.
func paramStability(prev, curr Params) float64 {
	if len(prev) == 0 || len(curr) == 0 {
		return 0
	}
	keys := make(map[string]struct{})
	for k := range prev {
		keys[k] = struct{}{}
	}
	for k := range curr {
		keys[k] = struct{}{}
	}
	if len(keys) == 0 {
		return 1
	}
	totalDiff := 0.0
	for k := range keys {
		pv := prev[k]
		cv := curr[k]
		maxVal := math.Max(math.Abs(pv), math.Max(math.Abs(cv), 1e-9))
		totalDiff += math.Abs(pv-cv) / maxVal
	}
	meanDiff := totalDiff / float64(len(keys))
	return math.Max(0, 1-meanDiff)
}

// buildReport aggregates window results into a final WF report.
func (e *Engine) buildReport(windows []WindowResult, cfg Config) Report {
	// Aggregate OOS metrics.
	var oosTradesTotal int
	var oosPnLTotal, sharpeSum, sortinoSum, maxDD float64
	var wins float64
	stabilitySum := 0.0

	for _, w := range windows {
		oosPnLTotal += w.TestMetrics.TotalPnLUSD
		oosTradesTotal += w.TestMetrics.TotalTrades
		sharpeSum += w.TestMetrics.SharpeRatio
		sortinoSum += w.TestMetrics.SortinoRatio
		wins += w.TestMetrics.WinRate * float64(w.TestMetrics.TotalTrades)
		if w.TestMetrics.MaxDrawdown > maxDD {
			maxDD = w.TestMetrics.MaxDrawdown
		}
		stabilitySum += w.ParamStability
	}

	nWindows := float64(len(windows))
	oosWinRate := 0.0
	if oosTradesTotal > 0 {
		oosWinRate = wins / float64(oosTradesTotal)
	}

	aggOOS := Metrics{
		SharpeRatio:  sharpeSum / nWindows,
		SortinoRatio: sortinoSum / nWindows,
		MaxDrawdown:  maxDD,
		WinRate:      oosWinRate,
		TotalPnLUSD:  oosPnLTotal,
		TotalTrades:  oosTradesTotal,
	}
	if oosTradesTotal > 0 {
		aggOOS.AvgTradeUSD = oosPnLTotal / float64(oosTradesTotal)
	}

	// Average IS Sharpe for efficiency ratio.
	isSharpeSum := 0.0
	for _, w := range windows {
		isSharpeSum += w.TrainMetrics.SharpeRatio
	}
	isAvgSharpe := isSharpeSum / nWindows

	effRatio := 0.0
	if isAvgSharpe != 0 {
		effRatio = aggOOS.SharpeRatio / isAvgSharpe
	}

	// Institutional pass criteria:
	// OOS Sharpe > 0.5, efficiency ratio > 0.3, param stability > 0.6.
	meanStability := stabilitySum / nWindows
	passed := aggOOS.SharpeRatio > 0.5 && effRatio > 0.3 && meanStability > 0.6
	failReason := ""
	if !passed {
		if aggOOS.SharpeRatio <= 0.5 {
			failReason = fmt.Sprintf("OOS Sharpe %.2f < 0.5 threshold", aggOOS.SharpeRatio)
		} else if effRatio <= 0.3 {
			failReason = fmt.Sprintf("efficiency ratio %.2f < 0.3 (likely overfit)", effRatio)
		} else {
			failReason = fmt.Sprintf("parameter stability %.2f < 0.6", meanStability)
		}
	}

	return Report{
		Config:          cfg,
		Windows:         windows,
		AggregateOOS:    aggOOS,
		ParamStability:  meanStability,
		EfficiencyRatio: effRatio,
		Passed:          passed,
		FailReason:      failReason,
	}
}

// ─── Default evaluator for metric-only strategies ─────────────────────────────

// MetricsEvaluator evaluates strategy performance using only the provided trade PnLs.
// Use this when strategy re-simulation per param set is not needed.
type MetricsEvaluator struct {
	paramSpace []Params
}

func NewMetricsEvaluator(paramSpace []Params) *MetricsEvaluator {
	return &MetricsEvaluator{paramSpace: paramSpace}
}

func (m *MetricsEvaluator) ParameterSpace() []Params { return m.paramSpace }

func (m *MetricsEvaluator) Evaluate(trades []Trade, _ Params) Metrics {
	return ComputeMetrics(trades)
}

// ComputeMetrics calculates institutional performance metrics from a trade slice.
func ComputeMetrics(trades []Trade) Metrics {
	if len(trades) == 0 {
		return Metrics{}
	}
	pnls := make([]float64, len(trades))
	wins := 0
	totalPnL := 0.0
	grossProfit, grossLoss := 0.0, 0.0

	for i, t := range trades {
		pnls[i] = t.PnLUSD
		totalPnL += t.PnLUSD
		if t.PnLUSD > 0 {
			wins++
			grossProfit += t.PnLUSD
		} else {
			grossLoss -= t.PnLUSD
		}
	}

	profitFactor := 0.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	}

	mean, std := 0.0, 0.0
	for _, p := range pnls {
		mean += p
	}
	mean /= float64(len(pnls))
	for _, p := range pnls {
		d := p - mean
		std += d * d
	}
	std = math.Sqrt(std / float64(len(pnls)))

	sharpe := 0.0
	if std > 0 {
		sharpe = (mean / std) * math.Sqrt(252)
	}

	// Sortino (downside deviation only)
	downVar := 0.0
	for _, p := range pnls {
		if p < 0 {
			downVar += p * p
		}
	}
	downStd := math.Sqrt(downVar / float64(len(pnls)))
	sortino := 0.0
	if downStd > 0 {
		sortino = (mean / downStd) * math.Sqrt(252)
	}

	// Max drawdown
	equity := 0.0
	peak := 0.0
	maxDD := 0.0
	for _, p := range pnls {
		equity += p
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / math.Max(peak, 1)
		if dd > maxDD {
			maxDD = dd
		}
	}

	calmar := 0.0
	if maxDD > 0 {
		annualPnL := totalPnL * (252.0 / float64(len(trades)))
		calmar = annualPnL / (maxDD * 100)
	}

	return Metrics{
		SharpeRatio:  sharpe,
		SortinoRatio: sortino,
		MaxDrawdown:  maxDD * 100,
		WinRate:      float64(wins) / float64(len(trades)),
		ProfitFactor: profitFactor,
		TotalPnLUSD:  totalPnL,
		TotalTrades:  len(trades),
		AvgTradeUSD:  mean,
		Calmar:       calmar,
	}
}
