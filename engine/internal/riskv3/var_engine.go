package riskv3

import (
	"math"
	"math/rand"
	"sort"
)

// VaRResult contains the Value-at-Risk metrics computed by three methods.
// The conservative estimate (max across methods) is recommended for risk limits.
type VaRResult struct {
	// Historical VaR — percentile of empirical return distribution
	Historical95USD float64 `json:"historical_95_usd"`
	Historical99USD float64 `json:"historical_99_usd"`
	Historical95Pct float64 `json:"historical_95_pct"`
	Historical99Pct float64 `json:"historical_99_pct"`

	// Parametric VaR — assumes normal distribution
	Parametric95USD float64 `json:"parametric_95_usd"`
	Parametric99USD float64 `json:"parametric_99_usd"`

	// Monte Carlo VaR — simulated from fitted normal distribution
	MonteCarlo95USD float64 `json:"monte_carlo_95_usd"`
	MonteCarlo99USD float64 `json:"monte_carlo_99_usd"`

	// Conservative (max across methods) — used for hard limit checks
	Daily95USD float64 `json:"daily_var_95_usd"`
	Daily99USD float64 `json:"daily_var_99_usd"`
	Daily95Pct float64 `json:"daily_var_95_pct"`
	Daily99Pct float64 `json:"daily_var_99_pct"`

	// Scaled estimates
	Weekly95USD  float64 `json:"weekly_var_95_usd"`  // daily * sqrt(5)
	Monthly95USD float64 `json:"monthly_var_95_usd"` // daily * sqrt(21)

	// Diagnostics
	SampleCount  int     `json:"sample_count"`
	ReturnStdDev float64 `json:"return_std_dev"`
	ReturnMean   float64 `json:"return_mean"`
	VaRBreached  bool    `json:"var_breached"` // true when Daily95Pct > MaxDailyVaR95Pct
}

// ComputeVaR computes Value-at-Risk using three methods and returns the
// conservative (maximum) estimate across methods.
//
// returns is a slice of portfolio return percentages (e.g. [-1.2, 0.3, -0.5 …]).
// Losses are represented as negative values.
// equity is the current portfolio equity in USD.
//
// Minimum MinVaRHistorySamples samples are required; returns a zero VaRResult otherwise.
func ComputeVaR(returns []float64, equity float64) VaRResult {
	if len(returns) < MinVaRHistorySamples || equity <= 0 {
		return VaRResult{SampleCount: len(returns)}
	}

	mean, stddev := meanStdDev(returns)
	hist95 := historicalVaR(returns, 0.95)
	hist99 := historicalVaR(returns, 0.99)
	param95 := parametricVaR(mean, stddev, 0.95)
	param99 := parametricVaR(mean, stddev, 0.99)
	mc95 := monteCarloVaR(mean, stddev, len(returns), 0.95)
	mc99 := monteCarloVaR(mean, stddev, len(returns), 0.99)

	// Conservative: max absolute loss across all three methods
	cons95 := math.Max(math.Max(hist95, param95), mc95)
	cons99 := math.Max(math.Max(hist99, param99), mc99)

	daily95USD := cons95 / 100 * equity
	daily99USD := cons99 / 100 * equity
	daily95Pct := cons95
	daily99Pct := cons99

	return VaRResult{
		Historical95USD: hist95 / 100 * equity,
		Historical99USD: hist99 / 100 * equity,
		Historical95Pct: hist95,
		Historical99Pct: hist99,
		Parametric95USD: param95 / 100 * equity,
		Parametric99USD: param99 / 100 * equity,
		MonteCarlo95USD: mc95 / 100 * equity,
		MonteCarlo99USD: mc99 / 100 * equity,
		Daily95USD:      daily95USD,
		Daily99USD:      daily99USD,
		Daily95Pct:      daily95Pct,
		Daily99Pct:      daily99Pct,
		Weekly95USD:     daily95USD * math.Sqrt(5),
		Monthly95USD:    daily95USD * math.Sqrt(21),
		SampleCount:     len(returns),
		ReturnStdDev:    stddev,
		ReturnMean:      mean,
		VaRBreached:     daily95Pct > MaxDailyVaR95Pct,
	}
}

// ─── Historical VaR ───────────────────────────────────────────────────────────

// historicalVaR computes the percentile of the empirical loss distribution.
// Returns the loss percentage (positive value = loss).
// confidence = 0.95 means the 5th percentile of returns (95th percentile of losses).
func historicalVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	sorted := make([]float64, len(returns))
	copy(sorted, returns)
	sort.Float64s(sorted) // ascending: worst losses first (most negative)

	// Index of the (1-confidence) quantile in the sorted (ascending) returns
	idx := int(math.Floor((1 - confidence) * float64(len(sorted))))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	// VaR is the negative of the return at the loss quantile (losses are positive)
	loss := -sorted[idx]
	if loss < 0 {
		return 0
	}
	return loss
}

// ─── Parametric VaR ───────────────────────────────────────────────────────────

// z-scores for standard normal distribution at common confidence levels.
const (
	zScore95 = 1.6449 // Φ^-1(0.95)
	zScore99 = 2.3263 // Φ^-1(0.99)
)

// parametricVaR computes VaR assuming normally distributed returns.
// Returns the loss percentage (positive value = loss).
func parametricVaR(mean, stddev, confidence float64) float64 {
	var z float64
	switch {
	case confidence >= 0.99:
		z = zScore99
	default:
		z = zScore95
	}
	// VaR = -(μ - z*σ)
	varPct := -(mean - z*stddev)
	if varPct < 0 {
		return 0
	}
	return varPct
}

// ─── Monte Carlo VaR ──────────────────────────────────────────────────────────

// monteCarloVaR simulates MonteCarloVaRPaths return paths from a fitted normal
// distribution and takes the (1-confidence) percentile.
// The random seed is deterministic (based on sample count) so results are
// reproducible for the same return series length.
func monteCarloVaR(mean, stddev float64, sampleCount int, confidence float64) float64 {
	paths := MonteCarloVaRPaths
	simulated := make([]float64, paths)

	// Deterministic seed for reproducibility across replay
	rng := rand.New(rand.NewSource(int64(sampleCount*37 + 17)))
	for i := range simulated {
		simulated[i] = mean + stddev*rng.NormFloat64()
	}

	sort.Float64s(simulated) // ascending: worst losses first

	idx := int(math.Floor((1 - confidence) * float64(paths)))
	if idx < 0 {
		idx = 0
	}
	if idx >= paths {
		idx = paths - 1
	}

	loss := -simulated[idx]
	if loss < 0 {
		return 0
	}
	return loss
}

// ─── Statistical helpers ─────────────────────────────────────────────────────

// meanStdDev computes the mean and population standard deviation of a slice.
func meanStdDev(values []float64) (mean, stddev float64) {
	if len(values) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))

	variance := 0.0
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	variance /= float64(len(values))
	return mean, math.Sqrt(variance)
}
