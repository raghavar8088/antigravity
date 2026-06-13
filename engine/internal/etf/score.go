package etf

import "fmt"

// ComputeETFScore returns a signal score in [-3, +3] based on ETF flow data.
func ComputeETFScore(data ETFFlowData) float64 {
	total := data.TotalFlowUSD
	score := 0.0
	switch {
	case total > thresholdVeryBullish:
		score = 3.0
	case total > thresholdBullish:
		score = 2.0
	case total > thresholdMildBullish:
		score = 1.0
	case total < thresholdBearish:
		score = -2.0
	case total < thresholdMildBearish:
		score = -1.0
	}
	// Streak bonus: consecutive inflow/outflow days signal institutional trend.
	if data.ConsecutiveInflow >= 5 {
		score += 1.0
	}
	if data.ConsecutiveOutflow >= 5 {
		score -= 1.0
	}
	return clamp(score, -3.0, 3.0)
}

// ETFScoreToPromptText returns a human-readable summary for AI prompt injection.
func ETFScoreToPromptText(data ETFFlowData, score float64) string {
	return fmt.Sprintf(
		"ETF flows: $%.0f total (largest: %s). Trend: %s. "+
			"Streak: %d inflow days / %d outflow days. Score: %+.1f (proxy data: %v)",
		data.TotalFlowUSD, data.LargestSingleETF, data.ETFFlowTrend,
		data.ConsecutiveInflow, data.ConsecutiveOutflow, score, data.IsProxy,
	)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
