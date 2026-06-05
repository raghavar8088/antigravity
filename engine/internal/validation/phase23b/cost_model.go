package phase23b

import "math"

// BuildCostBreakdowns computes full execution cost analysis for every strategy
// using only real trade records from the replay.
func BuildCostBreakdowns(replays []StrategyReplayResult) []CostBreakdown {
	breakdowns := make([]CostBreakdown, 0, len(replays))
	for _, r := range replays {
		if len(r.Trades) == 0 {
			continue
		}
		breakdowns = append(breakdowns, computeCostBreakdown(r))
	}
	return breakdowns
}

func computeCostBreakdown(r StrategyReplayResult) CostBreakdown {
	n := float64(len(r.Trades))
	cb := CostBreakdown{
		StrategyName: r.StrategyName,
		TradeCount:   len(r.Trades),
	}

	totalEntryFee, totalExitFee, totalSlippage, totalFunding := 0.0, 0.0, 0.0, 0.0
	totalGross, sumWin, sumLoss := 0.0, 0.0, 0.0
	grossSumWin, grossSumLoss := 0.0, 0.0

	for _, t := range r.Trades {
		totalEntryFee += t.EntryFeeUSD
		totalExitFee += t.ExitFeeUSD
		totalSlippage += t.SlippageUSD
		totalFunding += t.FundingUSD
		totalGross += t.GrossPnLUSD

		if t.NetPnLUSD > 0 {
			sumWin += t.NetPnLUSD
		} else {
			sumLoss += math.Abs(t.NetPnLUSD)
		}
		if t.GrossPnLUSD > 0 {
			grossSumWin += t.GrossPnLUSD
		} else {
			grossSumLoss += math.Abs(t.GrossPnLUSD)
		}
	}

	cb.AvgTakerFeeUSD = (totalEntryFee + totalExitFee) / n
	cb.AvgSlippageUSD = totalSlippage / n
	cb.AvgFundingUSD = totalFunding / n
	totalCost := totalEntryFee + totalExitFee + totalSlippage + totalFunding
	cb.TotalCostUSD = totalCost

	// Cost per trade in bps (assuming avg trade size from first trade)
	if len(r.Trades) > 0 && r.Trades[0].SizeUSD > 0 {
		cb.CostPerTradeBps = (totalCost / n) / r.Trades[0].SizeUSD * 10000
	}

	if grossSumLoss > 0 {
		cb.GrossPF = grossSumWin / grossSumLoss
	}
	if sumLoss > 0 {
		cb.NetPF = sumWin / sumLoss
	}
	if cb.GrossPF > 0 {
		cb.EdgeRetention = cb.NetPF / cb.GrossPF
	}
	if totalGross != 0 {
		cb.FundingImpact = totalFunding / math.Abs(totalGross) * 100
	}

	return cb
}
