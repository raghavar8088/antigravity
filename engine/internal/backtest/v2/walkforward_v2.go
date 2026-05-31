package v2

import (
	"errors"
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type WalkForwardMode string

const (
	WalkRolling   WalkForwardMode = "ROLLING"
	WalkExpanding WalkForwardMode = "EXPANDING"
	WalkAnchored  WalkForwardMode = "ANCHORED"
)

type StrategyFactory func() strategy.Strategy

type WalkForwardConfig struct {
	Mode            WalkForwardMode
	TrainWindow     time.Duration
	TestWindow      time.Duration
	Step            time.Duration
	StrategyFactory StrategyFactory
	BacktestConfig  Config
}

type WalkForwardSlice struct {
	TrainStart  time.Time
	TrainEnd    time.Time
	TestStart   time.Time
	TestEnd     time.Time
	ISMetrics   Metrics
	OOSMetrics  Metrics
	Degradation float64
	Stable      bool
}

type WalkForwardReport struct {
	Slices                []WalkForwardSlice
	AverageDegradation    float64
	PositiveOOSPF         bool
	PositiveOOSExpectancy bool
	Promotable            bool
}

func (e *Engine) RunWalkForward(ticks []marketdata.Tick, cfg WalkForwardConfig) (WalkForwardReport, error) {
	if cfg.StrategyFactory == nil {
		return WalkForwardReport{}, errors.New("walk-forward validation requires a strategy factory to avoid state leakage")
	}
	if cfg.TrainWindow <= 0 || cfg.TestWindow <= 0 {
		return WalkForwardReport{}, errors.New("walk-forward train and test windows must be positive")
	}
	if cfg.Step <= 0 {
		cfg.Step = cfg.TestWindow
	}
	sorted := SortTicks(ticks)
	if len(sorted) == 0 {
		return WalkForwardReport{}, nil
	}
	start := tickTime(sorted[0])
	end := tickTime(sorted[len(sorted)-1])
	var report WalkForwardReport
	for cursor := start; cursor.Add(cfg.TrainWindow+cfg.TestWindow).Before(end) || cursor.Add(cfg.TrainWindow+cfg.TestWindow).Equal(end); cursor = cursor.Add(cfg.Step) {
		trainStart := cursor
		trainEnd := cursor.Add(cfg.TrainWindow)
		if cfg.Mode == WalkExpanding || cfg.Mode == WalkAnchored {
			trainStart = start
		}
		testStart := trainEnd
		testEnd := testStart.Add(cfg.TestWindow)
		isTicks := windowTicks(sorted, trainStart, trainEnd)
		oosTicks := windowTicks(sorted, testStart, testEnd)
		if len(isTicks) == 0 || len(oosTicks) == 0 {
			continue
		}
		isRes, err := e.Run(Input{Ticks: isTicks, Strategy: cfg.StrategyFactory(), Config: cfg.BacktestConfig})
		if err != nil {
			return report, err
		}
		oosRes, err := e.Run(Input{Ticks: oosTicks, Strategy: cfg.StrategyFactory(), Config: cfg.BacktestConfig})
		if err != nil {
			return report, err
		}
		deg := 0.0
		if isRes.Metrics.Expectancy != 0 {
			deg = (isRes.Metrics.Expectancy - oosRes.Metrics.Expectancy) / abs(isRes.Metrics.Expectancy) * 100
		}
		report.Slices = append(report.Slices, WalkForwardSlice{
			TrainStart:  trainStart,
			TrainEnd:    trainEnd,
			TestStart:   testStart,
			TestEnd:     testEnd,
			ISMetrics:   isRes.Metrics,
			OOSMetrics:  oosRes.Metrics,
			Degradation: deg,
			Stable:      oosRes.Metrics.Expectancy > 0 && oosRes.Metrics.ProfitFactor > 1,
		})
	}
	report.PositiveOOSPF = true
	report.PositiveOOSExpectancy = true
	for _, s := range report.Slices {
		report.AverageDegradation += s.Degradation
		if s.OOSMetrics.ProfitFactor <= 1 {
			report.PositiveOOSPF = false
		}
		if s.OOSMetrics.Expectancy <= 0 {
			report.PositiveOOSExpectancy = false
		}
	}
	if len(report.Slices) > 0 {
		report.AverageDegradation /= float64(len(report.Slices))
	}
	report.Promotable = len(report.Slices) > 0 && report.PositiveOOSPF && report.PositiveOOSExpectancy
	return report, nil
}

func windowTicks(ticks []marketdata.Tick, start, end time.Time) []marketdata.Tick {
	out := make([]marketdata.Tick, 0)
	for _, t := range ticks {
		ts := tickTime(t)
		if (ts.Equal(start) || ts.After(start)) && ts.Before(end) {
			out = append(out, t)
		}
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
