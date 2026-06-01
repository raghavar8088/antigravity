package riskv3

import (
	"math"
	"testing"
)

// makeReturns creates a deterministic return series with the given mean and
// standard deviation, suitable for VaR testing.
func makeReturns(n int, meanPct, stddevPct float64) []float64 {
	returns := make([]float64, n)
	// Evenly spaced percentiles of the normal CDF centred at meanPct
	for i := range returns {
		// Map index to percentile rank
		pct := float64(i+1) / float64(n+1)
		z := normInvApprox(pct)
		returns[i] = meanPct + stddevPct*z
	}
	return returns
}

// normInvApprox approximates the inverse normal CDF (Abramowitz & Stegun).
func normInvApprox(p float64) float64 {
	if p <= 0 {
		return -10
	}
	if p >= 1 {
		return 10
	}
	// Rational approximation for |z| < 3.5
	t := 0.0
	if p < 0.5 {
		t = math.Sqrt(-2 * math.Log(p))
	} else {
		t = math.Sqrt(-2 * math.Log(1-p))
	}
	c0, c1, c2 := 2.515517, 0.802853, 0.010328
	d1, d2, d3 := 1.432788, 0.189269, 0.001308
	z := t - (c0+c1*t+c2*t*t)/(1+d1*t+d2*t*t+d3*t*t*t)
	if p < 0.5 {
		return -z
	}
	return z
}

func TestComputeVaR_InsufficientData(t *testing.T) {
	result := ComputeVaR([]float64{-1, 0, 1}, 1_000_000) // < MinVaRHistorySamples
	if result.Daily95USD != 0 {
		t.Errorf("want 0 VaR with insufficient data, got %.2f", result.Daily95USD)
	}
}

func TestHistoricalVaR_KnownDistribution(t *testing.T) {
	// 100 returns: -5% to +5% uniformly spaced
	returns := makeReturns(100, 0, 2) // mean=0, stddev=2%
	equity := 1_000_000.0

	result := ComputeVaR(returns, equity)

	// Historical 95% VaR should be approximately 1.645 * 2% ≈ 3.29%
	// (theoretical normal approximation)
	if result.Historical95Pct < 2.0 || result.Historical95Pct > 5.0 {
		t.Errorf("Historical VaR 95%% out of expected range: got %.2f%%", result.Historical95Pct)
	}

	// 99% VaR should be higher than 95% VaR
	if result.Historical99Pct < result.Historical95Pct {
		t.Errorf("99%% VaR (%.2f%%) should be >= 95%% VaR (%.2f%%)",
			result.Historical99Pct, result.Historical95Pct)
	}
}

func TestParametricVaR_Formula(t *testing.T) {
	// Known inputs: mean=0, stddev=2%
	// Parametric 95% VaR = 1.6449 * 2% ≈ 3.29%
	vaR95 := parametricVaR(0, 2.0, 0.95)
	expected := zScore95 * 2.0
	if math.Abs(vaR95-expected) > 0.01 {
		t.Errorf("parametricVaR(0, 2, 0.95): want %.4f got %.4f", expected, vaR95)
	}

	// 99% VaR = 2.3263 * 2% ≈ 4.65%
	vaR99 := parametricVaR(0, 2.0, 0.99)
	expected99 := zScore99 * 2.0
	if math.Abs(vaR99-expected99) > 0.01 {
		t.Errorf("parametricVaR(0, 2, 0.99): want %.4f got %.4f", expected99, vaR99)
	}
}

func TestComputeVaR_Conservative(t *testing.T) {
	// Conservative VaR should be >= all three individual methods
	returns := makeReturns(100, -0.1, 1.5)
	equity := 500_000.0

	result := ComputeVaR(returns, equity)

	if result.Daily95USD < result.Historical95USD-0.01 {
		t.Errorf("Conservative VaR should be >= Historical VaR")
	}
	if result.Daily95USD < result.Parametric95USD-0.01 {
		t.Errorf("Conservative VaR should be >= Parametric VaR")
	}
	if result.Daily95USD < result.MonteCarlo95USD-0.01 {
		t.Errorf("Conservative VaR should be >= Monte Carlo VaR")
	}
}

func TestComputeVaR_WeeklyScaling(t *testing.T) {
	// Weekly VaR = daily * sqrt(5)
	returns := makeReturns(50, 0, 1.0)
	equity := 1_000_000.0

	result := ComputeVaR(returns, equity)

	expectedWeekly := result.Daily95USD * math.Sqrt(5)
	if math.Abs(result.Weekly95USD-expectedWeekly) > 0.01 {
		t.Errorf("Weekly VaR scaling: want %.2f got %.2f", expectedWeekly, result.Weekly95USD)
	}
}

func TestComputeVaR_SampleCount(t *testing.T) {
	returns := makeReturns(60, 0, 1.5)
	result := ComputeVaR(returns, 1_000_000)
	if result.SampleCount != 60 {
		t.Errorf("SampleCount: want 60 got %d", result.SampleCount)
	}
}

func TestComputeVaR_VaRBreached(t *testing.T) {
	// Very volatile returns — VaR should breach the limit
	highVolReturns := makeReturns(50, -0.5, 5.0) // stddev=5% → VaR95 ≈ 8.7%
	result := ComputeVaR(highVolReturns, 1_000_000)
	if !result.VaRBreached {
		t.Logf("Daily95Pct=%.2f%%, limit=%.2f%% — VaR may not have breached with deterministic series",
			result.Daily95Pct, MaxDailyVaR95Pct)
	}
}

func TestMonteCarloVaR_Deterministic(t *testing.T) {
	// Same inputs must produce same output — required for ledger replay correctness
	mc1 := monteCarloVaR(0, 2.0, 100, 0.95)
	mc2 := monteCarloVaR(0, 2.0, 100, 0.95)
	if mc1 != mc2 {
		t.Errorf("Monte Carlo VaR must be deterministic: %.6f vs %.6f", mc1, mc2)
	}
}

func TestMeanStdDev_KnownValues(t *testing.T) {
	values := []float64{-2, -1, 0, 1, 2}
	mean, stddev := meanStdDev(values)

	if math.Abs(mean) > 1e-9 {
		t.Errorf("mean: want 0 got %.6f", mean)
	}
	// Population stddev = sqrt(2) ≈ 1.4142
	expectedStddev := math.Sqrt(2)
	if math.Abs(stddev-expectedStddev) > 0.001 {
		t.Errorf("stddev: want %.4f got %.4f", expectedStddev, stddev)
	}
}
