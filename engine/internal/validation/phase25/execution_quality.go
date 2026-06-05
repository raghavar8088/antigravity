package phase25

import (
	"math"
	"sort"
)

// BuildExecutionQuality computes full execution latency and cost statistics
// from the live trade set, across all five pipeline stages.
func BuildExecutionQuality(trades []LiveTrade, backtestSlippageBps float64) ExecutionQualityReport {
	r := ExecutionQualityReport{}
	if len(trades) == 0 {
		return r
	}

	// Collect latency samples per stage
	sigSubmit := make([]float64, 0, len(trades))
	submitAck := make([]float64, 0, len(trades))
	ackFill := make([]float64, 0, len(trades))
	fillPos := make([]float64, 0, len(trades))
	posExit := make([]float64, 0, len(trades))
	endToEnd := make([]float64, 0, len(trades))

	totalSlipBps := 0.0
	totalFundingBps := 0.0
	totalFeesBps := 0.0

	for _, t := range trades {
		sigSubmit = append(sigSubmit, t.LatencySignalSubmitMs)
		submitAck = append(submitAck, t.LatencySubmitAckMs)
		ackFill = append(ackFill, t.LatencyAckFillMs)
		fillPos = append(fillPos, t.LatencyFillPositionMs)
		posExit = append(posExit, t.LatencyPositionExitMs)

		e2e := t.LatencySignalSubmitMs + t.LatencySubmitAckMs + t.LatencyAckFillMs +
			t.LatencyFillPositionMs + t.LatencyPositionExitMs
		endToEnd = append(endToEnd, e2e)

		notional := t.EntryPrice * t.Size / 1e4 // crude notional estimate
		if notional > 0 {
			totalSlipBps += t.SlippageUSD / notional * 10000
			totalFundingBps += t.FundingUSD / notional * 10000
			totalFeesBps += t.FeeUSD / notional * 10000
		}
	}

	n := float64(len(trades))
	r.SignalToSubmit = latencyStats(sigSubmit)
	r.SubmitToAck = latencyStats(submitAck)
	r.AckToFill = latencyStats(ackFill)
	r.FillToPosition = latencyStats(fillPos)
	r.PositionToExit = latencyStats(posExit)
	r.EndToEnd = latencyStats(endToEnd)

	r.AvgSlippageBps = totalSlipBps / n
	r.AvgFundingCostBps = totalFundingBps / n
	r.AvgFeesBps = totalFeesBps / n

	// Execution drift vs backtest cost assumption
	if backtestSlippageBps > 0 {
		r.ExecDriftPct = (r.AvgSlippageBps - backtestSlippageBps) / backtestSlippageBps * 100
	}

	// Quality score: 100 = perfect; deduct for latency outliers, slippage drift, high fees
	r.QualityScore = computeExecQuality(r)

	return r
}

func latencyStats(samples []float64) LatencyStats {
	if len(samples) == 0 {
		return LatencyStats{}
	}
	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)

	sum := 0.0
	for _, s := range samples {
		sum += s
	}
	worst := sorted[len(sorted)-1]

	return LatencyStats{
		P50:     percentile25(sorted, 50),
		P95:     percentile25(sorted, 95),
		P99:     percentile25(sorted, 99),
		Worst:   worst,
		Average: sum / float64(len(samples)),
	}
}

func computeExecQuality(r ExecutionQualityReport) float64 {
	score := 100.0

	// Penalise for high P99 end-to-end latency
	if r.EndToEnd.P99 > 50 {
		score -= 20
	} else if r.EndToEnd.P99 > 20 {
		score -= 10
	} else if r.EndToEnd.P99 > 10 {
		score -= 5
	}

	// Penalise for slippage drift beyond backtest assumption
	if r.ExecDriftPct > 50 {
		score -= 25
	} else if r.ExecDriftPct > 25 {
		score -= 15
	} else if r.ExecDriftPct > 10 {
		score -= 5
	}

	// Penalise for very high fees
	if r.AvgFeesBps > 8 {
		score -= 10
	} else if r.AvgFeesBps > 6 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}
