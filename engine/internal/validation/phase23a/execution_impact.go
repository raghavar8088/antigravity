package phase23a

import (
	"antigravity-engine/internal/validation/phase22e"
	"antigravity-engine/internal/validation/phase22f"
)

// ComputeExecutionImpact calculates Phase 7: gross edge vs net edge for each strategy,
// measuring how much edge is consumed by execution costs.
func ComputeExecutionImpact(
	reports []WalkForwardReport,
	execQuality []phase22f.ExecQualityRecord,
	cfg BacktestConfig,
) []ExecutionImpactResult {
	results := make([]ExecutionImpactResult, 0, len(reports))

	execMap := make(map[string]phase22f.ExecQualityRecord, len(execQuality))
	for _, eq := range execQuality {
		execMap[eq.StrategyID] = eq
	}

	for _, rpt := range reports {
		// Collect all validation trades
		var validTrades []phase22e.TradeRecord
		for _, w := range rpt.Windows {
			validTrades = append(validTrades, w.ValidResult.Trades...)
		}
		if len(validTrades) == 0 {
			continue
		}

		// Gross metrics: PF ignoring fees
		grossGW, grossGL := 0.0, 0.0
		netGW, netGL := 0.0, 0.0
		totalFees, totalSlippage := 0.0, 0.0

		for _, t := range validTrades {
			grossPnL := t.GrossPnLUSD
			if grossPnL >= 0 {
				grossGW += grossPnL
			} else {
				grossGL += -grossPnL
			}
			if t.NetPnLUSD >= 0 {
				netGW += t.NetPnLUSD
			} else {
				netGL += -t.NetPnLUSD
			}
			totalFees += t.FeesUSD
			// estimate slippage from difference between gross and net, minus fees
			impliedSlippage := (t.GrossPnLUSD - t.NetPnLUSD) - t.FeesUSD
			if impliedSlippage > 0 {
				totalSlippage += impliedSlippage
			}
		}

		grossPF := 0.0
		if grossGL > 0 {
			grossPF = grossGW / grossGL
		}
		netPF := 0.0
		if netGL > 0 {
			netPF = netGW / netGL
		}

		// Execution cost in bps per trade
		avgPositionUSD := cfg.InitialCapital * cfg.PositionCapPct
		feeBps := 0.0
		if avgPositionUSD > 0 && len(validTrades) > 0 {
			feeBps = (totalFees / float64(len(validTrades))) / avgPositionUSD * 10000
		}

		// Edge retention
		edgeRetention := 1.0
		if grossPF > 0 {
			edgeRetention = netPF / grossPF
		}

		// Missed entries from execintel data or estimate
		eq := execMap[rpt.StrategyID]
		missedEntries := int(eq.MissedEntryRate * float64(len(validTrades)))

		results = append(results, ExecutionImpactResult{
			StrategyID:       rpt.StrategyID,
			StrategyName:     rpt.StrategyName,
			GrossEdgePF:      grossPF,
			ExecutionCostBps: feeBps + cfg.SlippageBps*2, // round-trip
			NetEdgePF:        netPF,
			SlippageCostUSD:  totalSlippage,
			FeeCostUSD:       totalFees,
			MissedEntries:    missedEntries,
			EdgeRetention:    edgeRetention,
		})
	}
	return results
}
