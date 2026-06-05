package phase23a

import (
	"math"
	"math/rand"
	"time"

	"antigravity-engine/internal/validation/phase22e"
)

// RunWalkForwardWithConfig uses a custom WalkForwardConfig.
func RunWalkForwardWithConfig(
	specs []StrategySpec,
	candles []OHLCVCandle,
	cfg BacktestConfig,
	wfCfg WalkForwardConfig,
	rng *rand.Rand,
) []WalkForwardReport {
	if len(candles) == 0 || len(specs) == 0 {
		return nil
	}
	windows := buildWindows(candles, wfCfg.TrainMonths, wfCfg.ValidMonths)
	if len(windows) == 0 {
		return nil
	}
	reports := make([]WalkForwardReport, 0, len(specs))
	for _, spec := range specs {
		rpt := runStrategyWalkForward(spec, candles, windows, cfg, rng)
		reports = append(reports, rpt)
	}
	return reports
}

// RunWalkForward executes rolling walk-forward validation for all strategy specs
// against the provided candle history.
//
// Window structure:
//   Train: DefaultTrainMonths months
//   Validate: DefaultValidMonths months
//   Step: DefaultValidMonths months (non-overlapping validation windows)
//
// Requires ≥ MinWFWindows windows; stops early if not enough data.
func RunWalkForward(
	specs []StrategySpec,
	candles []OHLCVCandle,
	cfg BacktestConfig,
	rng *rand.Rand,
) []WalkForwardReport {
	if len(candles) == 0 || len(specs) == 0 {
		return nil
	}

	windows := buildWindows(candles, DefaultTrainMonths, DefaultValidMonths)
	if len(windows) == 0 {
		return nil
	}

	reports := make([]WalkForwardReport, 0, len(specs))
	for _, spec := range specs {
		rpt := runStrategyWalkForward(spec, candles, windows, cfg, rng)
		reports = append(reports, rpt)
	}
	return reports
}

type wfWindow struct {
	trainFrom, trainTo time.Time
	validFrom, validTo time.Time
}

func buildWindows(candles []OHLCVCandle, trainMonths, validMonths int) []wfWindow {
	if len(candles) == 0 {
		return nil
	}
	start := candles[0].OpenTime
	end := candles[len(candles)-1].CloseTime

	trainDur := time.Duration(trainMonths) * 30 * 24 * time.Hour
	validDur := time.Duration(validMonths) * 30 * 24 * time.Hour

	var windows []wfWindow
	t := start
	for {
		trainEnd := t.Add(trainDur)
		validEnd := trainEnd.Add(validDur)
		if validEnd.After(end) {
			break
		}
		windows = append(windows, wfWindow{
			trainFrom: t,
			trainTo:   trainEnd,
			validFrom: trainEnd,
			validTo:   validEnd,
		})
		t = t.Add(validDur) // step forward by one validation period
	}
	return windows
}

func runStrategyWalkForward(
	spec StrategySpec,
	allCandles []OHLCVCandle,
	windows []wfWindow,
	cfg BacktestConfig,
	rng *rand.Rand,
) WalkForwardReport {
	rpt := WalkForwardReport{
		StrategyID:   spec.ID,
		StrategyName: spec.Name,
	}

	eng := NewBacktestEngine(cfg, rng)
	pfTotal, sharpeTotal, expTotal := 0.0, 0.0, 0.0
	consistentWindows := 0

	for i, w := range windows {
		trainCandles := FilterCandles(allCandles, w.trainFrom, w.trainTo)
		validCandles := FilterCandles(allCandles, w.validFrom, w.validTo)

		trainResult := eng.Run(i, spec, trainCandles)
		validResult := eng.Run(i, spec, validCandles)

		wfw := WalkForwardWindow{
			WindowNum:   i + 1,
			TrainFrom:   w.trainFrom,
			TrainTo:     w.trainTo,
			ValidFrom:   w.validFrom,
			ValidTo:     w.validTo,
			TrainResult: trainResult,
			ValidResult: validResult,
		}
		rpt.Windows = append(rpt.Windows, wfw)

		if len(validResult.Trades) > 0 {
			pf, sharpe, exp, _ := sampleMetrics23(validResult.Trades)
			pfTotal += pf
			sharpeTotal += sharpe
			expTotal += exp
			if pf > 1.0 {
				consistentWindows++
			}
		}
	}

	n := float64(len(rpt.Windows))
	if n > 0 {
		rpt.AvgValidPF = pfTotal / n
		rpt.AvgValidSharpe = sharpeTotal / n
		rpt.AvgValidExpect = expTotal / n
		rpt.Consistency = float64(consistentWindows) / n * 100
	}

	// Compute degradation: avg(validPF - trainPF)
	degTotal := 0.0
	counted := 0
	for _, w := range rpt.Windows {
		if len(w.TrainResult.Trades) > 0 && len(w.ValidResult.Trades) > 0 {
			tpf, _, _, _ := sampleMetrics23(w.TrainResult.Trades)
			vpf, _, _, _ := sampleMetrics23(w.ValidResult.Trades)
			degTotal += vpf - tpf
			counted++
		}
	}
	if counted > 0 {
		rpt.Degradation = degTotal / float64(counted)
	}

	rpt.IsConsistent = rpt.Consistency >= 60
	rpt.IsDegraded = rpt.Degradation < -0.20

	return rpt
}

// AllWalkForwardTrades concatenates all validation-window trades across all strategies.
func AllWalkForwardTrades(reports []WalkForwardReport) []phase22e.TradeRecord {
	var all []phase22e.TradeRecord
	for _, rpt := range reports {
		for _, w := range rpt.Windows {
			all = append(all, w.ValidResult.Trades...)
		}
	}
	return all
}

// StrategyBacktestTrades returns all combined (train+valid) trades for one strategy.
func StrategyBacktestTrades(reports []WalkForwardReport, strategyID string) []phase22e.TradeRecord {
	var trades []phase22e.TradeRecord
	for _, rpt := range reports {
		if rpt.StrategyID != strategyID {
			continue
		}
		for _, w := range rpt.Windows {
			trades = append(trades, w.TrainResult.Trades...)
			trades = append(trades, w.ValidResult.Trades...)
		}
	}
	return trades
}

// sampleMetrics23 is a local clone to avoid import cycles.
func sampleMetrics23(trades []phase22e.TradeRecord) (pf, sharpe, exp, wr float64) {
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
	mn := (gw - gl) / n
	if len(pnls) >= 2 {
		v := 0.0
		for _, p := range pnls {
			d := p - mn
			v += d * d
		}
		variance := v / float64(len(pnls)-1)
		if variance > 0 {
			std := math.Sqrt(variance)
			sharpe = (mn / std) * math.Sqrt(float64(len(pnls)))
		}
	}
	return
}
