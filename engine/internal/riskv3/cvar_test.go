package riskv3

import (
	"math"
	"testing"
)

func TestComputeCVaR_InsufficientData(t *testing.T) {
	result := ComputeCVaR([]float64{-1, 0}, 1_000_000)
	if result.CVaR95USD != 0 {
		t.Errorf("want 0 CVaR with insufficient data, got %.2f", result.CVaR95USD)
	}
}

func TestComputeCVaR_AllLosses(t *testing.T) {
	// 50 returns uniformly from -5% to -0.1%: all losses
	returns := make([]float64, 50)
	for i := range returns {
		returns[i] = -5.0 + float64(i)*0.1
	}
	result := ComputeCVaR(returns, 1_000_000)

	// CVaR95 should be the average of the worst 5% of returns (2-3 observations)
	if result.CVaR95Pct <= 0 {
		t.Errorf("CVaR95Pct should be positive for all-loss series, got %.4f", result.CVaR95Pct)
	}
	// CVaR99 >= CVaR95 (deeper tail = worse expected loss)
	if result.CVaR99Pct < result.CVaR95Pct-0.01 {
		t.Errorf("CVaR99 (%.2f) should be >= CVaR95 (%.2f)", result.CVaR99Pct, result.CVaR95Pct)
	}
}

func TestComputeCVaR_CVaRExceedsVaR(t *testing.T) {
	// CVaR must always be >= VaR at the same confidence level (coherence property)
	returns := makeReturns(100, -0.2, 2.0)
	equity := 1_000_000.0

	varResult := ComputeVaR(returns, equity)
	cvarResult := ComputeCVaR(returns, equity)

	if cvarResult.CVaR95Pct < varResult.Historical95Pct-0.01 {
		t.Errorf("CVaR95 (%.2f%%) should be >= Historical VaR95 (%.2f%%)",
			cvarResult.CVaR95Pct, varResult.Historical95Pct)
	}
}

func TestComputeCVaR_USD_Scaling(t *testing.T) {
	returns := makeReturns(50, 0, 1.5)
	equity := 2_000_000.0

	result := ComputeCVaR(returns, equity)

	// CVaR95USD = CVaR95Pct / 100 * equity
	expectedUSD := result.CVaR95Pct / 100 * equity
	if math.Abs(result.CVaR95USD-expectedUSD) > 0.01 {
		t.Errorf("CVaR95USD: want %.2f got %.2f", expectedUSD, result.CVaR95USD)
	}
}

func TestComputeCVaR_BreachFlag(t *testing.T) {
	// Very high volatility series — CVaR should breach
	highVolReturns := makeReturns(100, -1.0, 6.0)
	result := ComputeCVaR(highVolReturns, 1_000_000)
	if result.CVaR95Pct > MaxCVaR95Pct && !result.CVaR95Breached {
		t.Errorf("CVaR95Breached should be true when CVaR95Pct (%.2f%%) > limit (%.2f%%)",
			result.CVaR95Pct, MaxCVaR95Pct)
	}
}

func TestComputeCVaR_MaxObservedLoss(t *testing.T) {
	// The worst return is -10%; MaxObservedLossPct should be 10
	returns := make([]float64, 30)
	for i := range returns {
		returns[i] = float64(i - 10) // -10 to 19
	}
	result := ComputeCVaR(returns, 1_000_000)
	if math.Abs(result.MaxObservedLossPct-10.0) > 0.001 {
		t.Errorf("MaxObservedLossPct: want 10.0 got %.2f", result.MaxObservedLossPct)
	}
}

func TestConditionalExpectedShortfall_Simple(t *testing.T) {
	// Sorted: [-4, -3, -2, -1, 0, 1, 2, 3, 4, 5]
	// 95% CVaR: worst 5% = 0.5 observations → use 1 → [-4] → CVaR = 4%
	sorted := []float64{-4, -3, -2, -1, 0, 1, 2, 3, 4, 5}
	cvar := conditionalExpectedShortfall(sorted, 0.95)
	if math.Abs(cvar-4.0) > 0.001 {
		t.Errorf("CVaR95 simple: want 4.0 got %.4f", cvar)
	}
}

func TestConditionalExpectedShortfall_NoLosses(t *testing.T) {
	// All positive returns → CVaR should be 0
	sorted := []float64{1, 2, 3, 4, 5}
	cvar := conditionalExpectedShortfall(sorted, 0.95)
	if cvar != 0 {
		t.Errorf("CVaR should be 0 when all returns positive, got %.4f", cvar)
	}
}
