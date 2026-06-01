package v3

import "math"

// ExecutionAnalytics is the full V3 execution quality report for a set of trades.
type ExecutionAnalytics struct {
	TotalTrades int

	// PnL Attribution (gross → net waterfall)
	GrossPnL        float64
	SpreadCost      float64
	SlippageCost    float64
	ImpactCost      float64
	FundingCost     float64
	CommissionCost  float64
	LatencyCost     float64
	NetPnL          float64

	// Per-trade averages
	AvgSpreadBps    float64
	AvgSlippageBps  float64
	AvgImpactBps    float64
	AvgFundingDragPct float64
	AvgCommissionBps float64
	AvgLatencyMs    float64

	// Fill quality
	AvgFillRatio        float64
	PartialFillRate     float64 // % trades with partial fills
	AvgExpectedFill     float64
	AvgActualFill       float64
	FillEfficiencyScore float64 // actual/expected fill ×100

	// Execution Quality Score (0–100)
	ExecutionQualityScore float64

	// Benchmark comparison (set externally if available)
	BuyHoldPnL        float64
	Alpha             float64
	InformationRatio  float64

	// PnL attribution as percentage of gross
	SpreadCostPct    float64
	SlippageCostPct  float64
	ImpactCostPct    float64
	FundingCostPct   float64
	CommissionCostPct float64
	LatencyCostPct   float64
}

// BuildExecutionAnalytics computes V3 execution analytics from a trade slice.
func BuildExecutionAnalytics(trades []V3Trade) ExecutionAnalytics {
	if len(trades) == 0 {
		return ExecutionAnalytics{}
	}

	a := ExecutionAnalytics{TotalTrades: len(trades)}
	partialCount := 0
	totalExpectedFill := 0.0
	totalActualFill := 0.0
	totalSpreadBps := 0.0
	totalSlippageBps := 0.0
	totalImpactBps := 0.0
	totalLatencyMs := 0.0

	for _, t := range trades {
		a.GrossPnL += t.GrossPnL
		a.SpreadCost += t.SpreadCostUSD
		a.SlippageCost += t.SlippageCostUSD
		a.ImpactCost += t.ImpactCostUSD
		a.FundingCost += t.FundingCostUSD
		a.CommissionCost += t.CommissionUSD
		a.LatencyCost += t.LatencyCostUSD
		a.NetPnL += t.NetPnL

		a.AvgFillRatio += t.FillRatio
		if len(t.PartialFillEvents) > 1 {
			partialCount++
		}
		totalExpectedFill += t.EntryFill.ExpectedFill
		totalActualFill += t.EntryFill.ActualFill

		notional := t.EntryPrice * t.Size
		if notional > 0 {
			totalSpreadBps += t.SpreadCostUSD / notional * 10_000
			totalSlippageBps += t.SlippageCostUSD / notional * 10_000
			totalImpactBps += t.ImpactCostUSD / notional * 10_000
		}
		totalLatencyMs += t.SimulatedLatencyMs
	}

	n := float64(len(trades))
	a.AvgFillRatio /= n
	a.AvgSpreadBps = totalSpreadBps / n
	a.AvgSlippageBps = totalSlippageBps / n
	a.AvgImpactBps = totalImpactBps / n
	a.AvgLatencyMs = totalLatencyMs / n
	a.PartialFillRate = float64(partialCount) / n * 100

	if totalExpectedFill > 0 {
		a.AvgExpectedFill = totalExpectedFill / n
		a.AvgActualFill = totalActualFill / n
		a.FillEfficiencyScore = totalActualFill / totalExpectedFill * 100
	}

	// PnL attribution as % of gross.
	if math.Abs(a.GrossPnL) > 0 {
		a.SpreadCostPct = a.SpreadCost / math.Abs(a.GrossPnL) * 100
		a.SlippageCostPct = a.SlippageCost / math.Abs(a.GrossPnL) * 100
		a.ImpactCostPct = a.ImpactCost / math.Abs(a.GrossPnL) * 100
		a.FundingCostPct = a.FundingCost / math.Abs(a.GrossPnL) * 100
		a.CommissionCostPct = a.CommissionCost / math.Abs(a.GrossPnL) * 100
		a.LatencyCostPct = a.LatencyCost / math.Abs(a.GrossPnL) * 100
	}

	a.ExecutionQualityScore = computeEQS(a)
	return a
}

// computeEQS computes the Execution Quality Score (0–100).
// 100 = institutional grade; below 60 = unacceptable.
//
// Scoring penalties:
//   - High average slippage: up to -20 pts
//   - High impact cost: up to -15 pts
//   - Low fill ratio: up to -20 pts
//   - High commission drag: up to -10 pts
//   - High partial fill rate: up to -10 pts
//   - High latency: up to -10 pts
//   - Funding drag: up to -5 pts
//   - Spread drain: up to -10 pts
func computeEQS(a ExecutionAnalytics) float64 {
	score := 100.0

	// Slippage penalty: 0 bps = 0 pts, 10 bps = -20 pts.
	slippagePenalty := clampF(a.AvgSlippageBps/10*20, 0, 20)
	score -= slippagePenalty

	// Impact penalty: 0 bps = 0, 20 bps = -15 pts.
	impactPenalty := clampF(a.AvgImpactBps/20*15, 0, 15)
	score -= impactPenalty

	// Fill ratio penalty: 1.0 = 0, 0.5 = -20 pts.
	fillPenalty := clampF((1-a.AvgFillRatio)*40, 0, 20)
	score -= fillPenalty

	// Commission penalty: 4 bps = 0, 10 bps = -10 pts.
	commPenalty := clampF((a.AvgCommissionBps-4)/6*10, 0, 10)
	score -= commPenalty

	// Partial fill rate penalty: 0% = 0, 50% = -10 pts.
	partialPenalty := clampF(a.PartialFillRate/50*10, 0, 10)
	score -= partialPenalty

	// Latency penalty: 50ms = 0, 500ms = -10 pts.
	latencyPenalty := clampF((a.AvgLatencyMs-50)/450*10, 0, 10)
	score -= latencyPenalty

	// Spread penalty: 2 bps = 0, 20 bps = -10 pts.
	spreadPenalty := clampF((a.AvgSpreadBps-2)/18*10, 0, 10)
	score -= spreadPenalty

	// Funding drag penalty: 0.1% drag = 0, 1% = -5 pts.
	fundingPenalty := clampF(a.AvgFundingDragPct/1*5, 0, 5)
	score -= fundingPenalty

	return clampF(score, 0, 100)
}

// PnLWaterfall returns a structured breakdown of gross → net for reporting.
func PnLWaterfall(a ExecutionAnalytics) map[string]float64 {
	running := a.GrossPnL
	wf := map[string]float64{
		"1_gross_pnl":    a.GrossPnL,
		"2_minus_spread": running - a.SpreadCost,
		"3_minus_slippage": running - a.SpreadCost - a.SlippageCost,
		"4_minus_impact": running - a.SpreadCost - a.SlippageCost - a.ImpactCost,
		"5_minus_funding": running - a.SpreadCost - a.SlippageCost - a.ImpactCost - a.FundingCost,
		"6_minus_commission": running - a.SpreadCost - a.SlippageCost - a.ImpactCost - a.FundingCost - a.CommissionCost,
		"7_minus_latency": running - a.SpreadCost - a.SlippageCost - a.ImpactCost - a.FundingCost - a.CommissionCost - a.LatencyCost,
		"8_net_pnl":       a.NetPnL,
	}
	return wf
}
