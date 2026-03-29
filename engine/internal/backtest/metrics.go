package backtest

import "fmt"

// CalculateMetrics digests the exact paper-trading state after a simulation
// and outputs institutional-grade performance numbers like Win Rate & ROI.
func CalculateMetrics(state *PnLState) {
	fmt.Println("\n=== Antigravity Backtest Performance Report ===")
	fmt.Printf("Initial Capital: $%.2f\n", state.InitialBalance)
	fmt.Printf("Final Capital:   $%.2f\n", state.Balance)

	netProfit := state.Balance - state.InitialBalance
	roi := (netProfit / state.InitialBalance) * 100
	fmt.Printf("Net Profit:      $%.2f (%.2f%%)\n", netProfit, roi)

	winCount := 0
	lossCount := 0

	// We only care about SELL trades for PnL calculation since it realizes the loop
	for _, tr := range state.Trades {
		if tr.Side == "SELL" {
			if tr.PnL > 0 {
				winCount++
			} else if tr.PnL < 0 {
				lossCount++
			}
		}
	}

	totalClosedTrades := winCount + lossCount
	winRate := 0.0
	if totalClosedTrades > 0 {
		winRate = float64(winCount) / float64(totalClosedTrades) * 100
	}

	fmt.Printf("Closed Trades:   %d\n", totalClosedTrades)
	fmt.Printf("System Hit Rate: %.2f%%\n", winRate)
	fmt.Println("===============================================")
}
