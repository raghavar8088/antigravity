package riskv3

import "sort"

// CVaRResult contains Conditional Value at Risk (Expected Shortfall) metrics.
// CVaR is always >= VaR at the same confidence level because it measures the
// average loss in the tail, not just the threshold.
type CVaRResult struct {
	// 95% confidence — average loss beyond the 95% VaR threshold
	CVaR95USD float64 `json:"cvar_95_usd"`
	CVaR95Pct float64 `json:"cvar_95_pct"`

	// 99% confidence — average loss beyond the 99% VaR threshold
	CVaR99USD float64 `json:"cvar_99_usd"`
	CVaR99Pct float64 `json:"cvar_99_pct"`

	// Tail loss statistics
	TailLossCount95  int     `json:"tail_loss_count_95"`  // number of observations in the 95% tail
	TailLossCount99  int     `json:"tail_loss_count_99"`
	MaxObservedLossPct float64 `json:"max_observed_loss_pct"` // worst single-period loss

	// Breach flags
	CVaR95Breached bool `json:"cvar_95_breached"` // CVaR95Pct > MaxCVaR95Pct
	CVaR99Breached bool `json:"cvar_99_breached"` // CVaR99Pct > MaxCVaR99Pct

	SampleCount int `json:"sample_count"`
}

// ComputeCVaR calculates the Conditional Value at Risk (Expected Shortfall)
// at the 95% and 99% confidence levels.
//
// CVaR (also called Expected Shortfall) is the expected loss given that the
// loss exceeds the VaR threshold. It is a coherent risk measure that captures
// tail risk better than VaR alone.
//
// Formula:
//
//	CVaR(α) = E[Loss | Loss > VaR(α)]
//	        = average of all losses in the worst (1-α) fraction of outcomes
//
// Inputs are identical to ComputeVaR: a return series and current equity.
func ComputeCVaR(returns []float64, equity float64) CVaRResult {
	if len(returns) < MinVaRHistorySamples || equity <= 0 {
		return CVaRResult{SampleCount: len(returns)}
	}

	sorted := make([]float64, len(returns))
	copy(sorted, returns)
	sort.Float64s(sorted) // ascending: most negative (worst loss) first

	cvar95Pct := conditionalExpectedShortfall(sorted, 0.95)
	cvar99Pct := conditionalExpectedShortfall(sorted, 0.99)

	// Count tail observations
	tail95Count := tailCount(sorted, 0.95)
	tail99Count := tailCount(sorted, 0.99)

	// Worst single-period loss (most negative return → highest loss)
	maxLoss := 0.0
	if sorted[0] < 0 {
		maxLoss = -sorted[0]
	}

	return CVaRResult{
		CVaR95USD:          cvar95Pct / 100 * equity,
		CVaR95Pct:          cvar95Pct,
		CVaR99USD:          cvar99Pct / 100 * equity,
		CVaR99Pct:          cvar99Pct,
		TailLossCount95:    tail95Count,
		TailLossCount99:    tail99Count,
		MaxObservedLossPct: maxLoss,
		CVaR95Breached:     cvar95Pct > MaxCVaR95Pct,
		CVaR99Breached:     cvar99Pct > MaxCVaR99Pct,
		SampleCount:        len(returns),
	}
}

// conditionalExpectedShortfall computes CVaR at the given confidence level
// from a pre-sorted (ascending) return slice.
// Returns the loss % (positive = loss).
func conditionalExpectedShortfall(sortedReturns []float64, confidence float64) float64 {
	n := len(sortedReturns)
	if n == 0 {
		return 0
	}
	// Number of tail observations at (1-confidence) level
	cutoff := int(float64(n) * (1 - confidence))
	if cutoff < 1 {
		cutoff = 1
	}
	if cutoff > n {
		cutoff = n
	}

	// Average the worst `cutoff` losses
	sum := 0.0
	count := 0
	for i := 0; i < cutoff; i++ {
		if sortedReturns[i] < 0 {
			sum += sortedReturns[i]
			count++
		}
	}
	if count == 0 {
		// No losses in tail — use worst observed return if any, else 0
		if sortedReturns[0] < 0 {
			return -sortedReturns[0]
		}
		return 0
	}

	// CVaR = average of tail losses (losses are positive)
	cvar := -(sum / float64(count))
	if cvar < 0 {
		return 0
	}
	return cvar
}

// tailCount returns the number of returns in the (1-confidence) worst tail.
func tailCount(sortedReturns []float64, confidence float64) int {
	n := len(sortedReturns)
	cutoff := int(float64(n) * (1 - confidence))
	if cutoff < 1 {
		cutoff = 1
	}
	count := 0
	for i := 0; i < cutoff && i < n; i++ {
		if sortedReturns[i] < 0 {
			count++
		}
	}
	return count
}
