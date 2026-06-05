package phase23b

import (
	"math"
	"sort"
	"time"
)

// RunWalkForward performs rolling walk-forward validation for every strategy
// using only real certified trades.  No future data leakage; each validation
// window uses strictly post-train data.
func RunWalkForward(replays []StrategyReplayResult, candles []OHLCVCandle) []WFReport {
	if len(candles) == 0 {
		return nil
	}
	dataStart := candles[0].OpenTime
	dataEnd := candles[len(candles)-1].CloseTime

	windows := buildWFWindows(dataStart, dataEnd, WFTrainMonths, WFValidMonths)
	if len(windows) < WFMinWindows {
		// Not enough data for full WF; run with what we have
		if len(windows) == 0 {
			return nil
		}
	}

	reports := make([]WFReport, 0, len(replays))
	for _, r := range replays {
		if len(r.Trades) == 0 {
			continue
		}
		reports = append(reports, buildWFReport(r, windows))
	}
	return reports
}

type wfWindowDef struct {
	TrainFrom time.Time
	TrainTo   time.Time
	ValidFrom time.Time
	ValidTo   time.Time
}

func buildWFWindows(start, end time.Time, trainMonths, validMonths int) []wfWindowDef {
	var windows []wfWindowDef
	cursor := start
	for {
		trainEnd := cursor.AddDate(0, trainMonths, 0)
		validEnd := trainEnd.AddDate(0, validMonths, 0)
		if validEnd.After(end) {
			break
		}
		windows = append(windows, wfWindowDef{
			TrainFrom: cursor,
			TrainTo:   trainEnd,
			ValidFrom: trainEnd,
			ValidTo:   validEnd,
		})
		// Advance by validMonths (anchored walk-forward)
		cursor = cursor.AddDate(0, validMonths, 0)
	}
	return windows
}

func buildWFReport(r StrategyReplayResult, windows []wfWindowDef) WFReport {
	rep := WFReport{
		StrategyName: r.StrategyName,
		Windows:      make([]WFWindow, 0, len(windows)),
	}

	var validPFs, validSharpes, validExpects []float64
	degradations := make([]float64, 0, len(windows))
	consistentWindows := 0

	for i, w := range windows {
		trainTrades := filterTradesByTime(r.Trades, w.TrainFrom, w.TrainTo)
		validTrades := filterTradesByTime(r.Trades, w.ValidFrom, w.ValidTo)

		trainM := ComputeMetrics(r.StrategyName, trainTrades)
		validM := ComputeMetrics(r.StrategyName, validTrades)

		window := WFWindow{
			WindowNum:    i + 1,
			TrainFrom:    w.TrainFrom,
			TrainTo:      w.TrainTo,
			ValidFrom:    w.ValidFrom,
			ValidTo:      w.ValidTo,
			TrainTrades:  trainTrades,
			ValidTrades:  validTrades,
			TrainMetrics: trainM,
			ValidMetrics: validM,
		}
		rep.Windows = append(rep.Windows, window)

		if validM.ProfitFactor > 1.0 {
			consistentWindows++
		}
		if validM.ProfitFactor > 0 {
			validPFs = append(validPFs, validM.ProfitFactor)
			degradations = append(degradations, validM.ProfitFactor-trainM.ProfitFactor)
		}
		if validM.Sharpe != 0 {
			validSharpes = append(validSharpes, validM.Sharpe)
		}
		if validM.Expectancy != 0 {
			validExpects = append(validExpects, validM.Expectancy)
		}
	}

	if len(windows) > 0 {
		rep.Consistency = float64(consistentWindows) / float64(len(windows)) * 100
	}
	rep.AvgValidPF = mean(validPFs)
	rep.AvgValidSharpe = mean(validSharpes)
	rep.AvgValidExpect = mean(validExpects)
	rep.Degradation = median(degradations)
	rep.IsConsistent = rep.Consistency >= 60
	rep.IsDegraded = rep.Degradation < -0.20

	return rep
}

func filterTradesByTime(trades []CertifiedTrade, from, to time.Time) []CertifiedTrade {
	var out []CertifiedTrade
	for _, t := range trades {
		if !t.EntryTime.Before(from) && t.EntryTime.Before(to) {
			out = append(out, t)
		}
	}
	return out
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sorted := make([]float64, len(xs))
	copy(sorted, xs)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	s := 0.0
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}
