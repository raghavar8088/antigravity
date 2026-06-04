package pms

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// OptimizationMethod identifies the portfolio construction algorithm.
type OptimizationMethod string

const (
	OptMethodMVO               OptimizationMethod = "MVO"
	OptMethodRiskParity        OptimizationMethod = "RISK_PARITY"
	OptMethodMaxDiversification OptimizationMethod = "MAX_DIVERSIFICATION"
	OptMethodVolTargeting      OptimizationMethod = "VOL_TARGETING"
	OptMethodEqualWeight       OptimizationMethod = "EQUAL_WEIGHT"
)

// StrategyProfile is the optimizer input for one strategy.
type StrategyProfile struct {
	StrategyID   string
	StrategyName string
	// Historical return series (daily returns as fractions, e.g. 0.01 = 1%)
	Returns []float64
	// Pre-computed metrics (optional; used if Returns is short)
	ExpectedReturnAnnual float64 // annualised expected return (fraction)
	VolatilityAnnual     float64 // annualised volatility (fraction)
	SharpeRatio          float64
}

// OptimizationInput holds all the data needed to run an optimisation.
type OptimizationInput struct {
	PortfolioID    string
	Strategies     []StrategyProfile
	Method         OptimizationMethod
	TargetVolPct   float64 // for VOL_TARGETING: target portfolio vol in % (e.g. 10.0)
	RiskFreeRate   float64 // annualised risk-free rate (fraction, e.g. 0.05)
	MinWeightPct   float64 // minimum per-strategy weight (default 1%)
	MaxWeightPct   float64 // maximum per-strategy weight (default 30%)
	CashReservePct float64 // minimum cash reserve (default 5%)
}

// OptimizationResult contains the optimizer's recommendation.
type OptimizationResult struct {
	PortfolioID          string
	Method               OptimizationMethod
	OptimalWeights       []StrategyWeight
	ExpectedReturnPct    float64 // annualised expected return (%)
	ExpectedVolPct       float64 // annualised expected volatility (%)
	ExpectedSharpe       float64
	DiversificationRatio float64
	Rationale            string
	GeneratedAt          time.Time
}

// PortfolioOptimizer implements multiple portfolio construction algorithms.
// All algorithms are deterministic and auditable.
type PortfolioOptimizer struct{}

// NewPortfolioOptimizer creates a portfolio optimizer.
func NewPortfolioOptimizer() *PortfolioOptimizer {
	return &PortfolioOptimizer{}
}

// Optimize runs the selected optimisation algorithm and returns a recommendation.
func (o *PortfolioOptimizer) Optimize(ctx context.Context, input OptimizationInput) (OptimizationResult, error) {
	if len(input.Strategies) == 0 {
		return OptimizationResult{}, fmt.Errorf("optimizer: no strategies provided")
	}
	if input.MinWeightPct == 0 {
		input.MinWeightPct = 1.0
	}
	if input.MaxWeightPct == 0 {
		input.MaxWeightPct = 30.0
	}
	if input.CashReservePct == 0 {
		input.CashReservePct = 5.0
	}
	if input.RiskFreeRate == 0 {
		input.RiskFreeRate = 0.05
	}

	// Compute statistics for each strategy
	profiles := enrichProfiles(input.Strategies)

	var weights []StrategyWeight
	var err error

	switch input.Method {
	case OptMethodMVO:
		weights, err = o.meanVarianceOptimize(profiles, input)
	case OptMethodRiskParity:
		weights, err = o.riskParityOptimize(profiles, input)
	case OptMethodMaxDiversification:
		weights, err = o.maxDiversificationOptimize(profiles, input)
	case OptMethodVolTargeting:
		weights, err = o.volTargetingOptimize(profiles, input)
	default:
		weights, err = o.equalWeightOptimize(profiles, input)
	}
	if err != nil {
		return OptimizationResult{}, err
	}

	weights = applyWeightConstraints(weights, input)
	weights = renormaliseWeights(weights, 100.0-input.CashReservePct)

	// Compute portfolio-level statistics
	portReturn, portVol := computePortfolioStats(weights, profiles)
	sharpe := 0.0
	if portVol > 0 {
		sharpe = (portReturn - input.RiskFreeRate*100) / portVol
	}
	divRatio := computeDiversificationRatio(weights, profiles)

	result := OptimizationResult{
		PortfolioID:          input.PortfolioID,
		Method:               input.Method,
		OptimalWeights:       weights,
		ExpectedReturnPct:    portReturn,
		ExpectedVolPct:       portVol,
		ExpectedSharpe:       sharpe,
		DiversificationRatio: divRatio,
		Rationale:            fmt.Sprintf("%s optimization: n=%d strategies, target_vol=%.1f%%", input.Method, len(input.Strategies), input.TargetVolPct),
		GeneratedAt:          time.Now().UTC(),
	}

	// Persist recommendation as an event
	changes := buildWeightChanges(input.Strategies, weights)
	payload := OptimizerRecommendationPayload{
		RecommendationID:     fmt.Sprintf("opt:%s:%d", input.PortfolioID, time.Now().UnixNano()),
		PortfolioID:          input.PortfolioID,
		Method:               string(input.Method),
		ExpectedReturn:       portReturn,
		ExpectedVol:          portVol,
		ExpectedSharpe:       sharpe,
		DiversificationRatio: divRatio,
		Rationale:            result.Rationale,
		Changes:              changes,
		GeneratedAt:          result.GeneratedAt,
	}
	// Current and optimal weights maps for the payload
	currentWeights := make(map[string]float64, len(input.Strategies))
	optimalWeights := make(map[string]float64, len(weights))
	for _, s := range input.Strategies {
		currentWeights[s.StrategyID] = s.ExpectedReturnAnnual // proxy
	}
	for _, w := range weights {
		optimalWeights[w.StrategyID] = w.AllocPct
	}
	payload.CurrentWeights = currentWeights
	payload.OptimalWeights = optimalWeights

	// Emit event (store is nil-safe inside EmitOptimizerRecommendation)
	EmitOptimizerRecommendation(ctx, nil, payload)

	return result, nil
}

// ── Algorithm implementations ─────────────────────────────────────────────────

// meanVarianceOptimize approximates MVO using Sharpe ratio as a proxy for
// the efficient frontier tangency point. Full matrix inversion is avoided
// to keep allocation deterministic without numerical dependencies.
func (o *PortfolioOptimizer) meanVarianceOptimize(profiles []StrategyProfile, input OptimizationInput) ([]StrategyWeight, error) {
	sharpeSum := 0.0
	for _, p := range profiles {
		if p.SharpeRatio > 0 {
			sharpeSum += p.SharpeRatio
		}
	}
	if sharpeSum == 0 {
		return o.equalWeightOptimize(profiles, input)
	}
	weights := make([]StrategyWeight, 0, len(profiles))
	for _, p := range profiles {
		if p.SharpeRatio <= 0 {
			continue
		}
		pct := p.SharpeRatio / sharpeSum * (100.0 - input.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   p.StrategyID,
			StrategyName: p.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("MVO proxy: sharpe=%.2f", p.SharpeRatio),
		})
	}
	return weights, nil
}

// riskParityOptimize gives each strategy equal risk contribution.
// Weight ∝ 1 / vol_i (inverse-volatility parity).
func (o *PortfolioOptimizer) riskParityOptimize(profiles []StrategyProfile, input OptimizationInput) ([]StrategyWeight, error) {
	invVolSum := 0.0
	for _, p := range profiles {
		if p.VolatilityAnnual > 0 {
			invVolSum += 1.0 / p.VolatilityAnnual
		}
	}
	if invVolSum == 0 {
		return o.equalWeightOptimize(profiles, input)
	}
	weights := make([]StrategyWeight, 0, len(profiles))
	for _, p := range profiles {
		if p.VolatilityAnnual <= 0 {
			continue
		}
		pct := (1.0 / p.VolatilityAnnual) / invVolSum * (100.0 - input.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   p.StrategyID,
			StrategyName: p.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("risk_parity: inv_vol=%.4f", 1.0/p.VolatilityAnnual),
		})
	}
	return weights, nil
}

// maxDiversificationOptimize maximises the diversification ratio:
// DR = weighted-average vol / portfolio vol.
// Approximated here by up-weighting strategies with lower pairwise correlation
// to others (proxy: higher vol adjusted by inverse of average correlation rank).
func (o *PortfolioOptimizer) maxDiversificationOptimize(profiles []StrategyProfile, input OptimizationInput) ([]StrategyWeight, error) {
	// Compute pairwise correlations from return series
	n := len(profiles)
	if n == 0 {
		return nil, fmt.Errorf("optimizer: no profiles for max diversification")
	}

	// Score each strategy by (vol / avg_pairwise_corr) — high score → more weight
	scores := make([]float64, n)
	for i, pi := range profiles {
		avgCorr := 0.0
		count := 0
		for j, pj := range profiles {
			if i == j {
				continue
			}
			corr := pearsonCorrelation(pi.Returns, pj.Returns)
			if !math.IsNaN(corr) {
				avgCorr += corr
				count++
			}
		}
		if count > 0 {
			avgCorr /= float64(count)
		} else {
			avgCorr = 0.5 // default assumption
		}
		denom := avgCorr
		if denom <= 0 {
			denom = 0.01
		}
		scores[i] = pi.VolatilityAnnual / denom
	}
	totalScore := 0.0
	for _, s := range scores {
		totalScore += s
	}
	if totalScore == 0 {
		return o.equalWeightOptimize(profiles, input)
	}
	weights := make([]StrategyWeight, 0, n)
	for i, p := range profiles {
		if scores[i] == 0 {
			continue
		}
		pct := scores[i] / totalScore * (100.0 - input.CashReservePct)
		weights = append(weights, StrategyWeight{
			StrategyID:   p.StrategyID,
			StrategyName: p.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("max_div: score=%.4f", scores[i]),
		})
	}
	return weights, nil
}

// volTargetingOptimize scales all strategies so the portfolio hits a
// target annualised volatility. Uses inverse-vol weighting then scales.
func (o *PortfolioOptimizer) volTargetingOptimize(profiles []StrategyProfile, input OptimizationInput) ([]StrategyWeight, error) {
	targetVol := input.TargetVolPct / 100.0 // convert % to fraction
	if targetVol <= 0 {
		targetVol = 0.10 // default 10%
	}

	// Start with inverse-vol weights
	weights, err := o.riskParityOptimize(profiles, input)
	if err != nil || len(weights) == 0 {
		return weights, err
	}

	// Estimate current portfolio vol with these weights
	profileMap := make(map[string]StrategyProfile, len(profiles))
	for _, p := range profiles {
		profileMap[p.StrategyID] = p
	}
	_, portVol := computePortfolioStats(weights, profiles)
	if portVol == 0 {
		return weights, nil
	}

	// Scale weights to hit target vol
	scale := targetVol * 100 / portVol
	if scale > 1.0 {
		scale = 1.0 // cannot exceed 100% allocation
	}
	for i := range weights {
		weights[i].AllocPct *= scale
		weights[i].Rationale = fmt.Sprintf("vol_target=%.1f%%: scale=%.2f", input.TargetVolPct, scale)
	}
	return weights, nil
}

func (o *PortfolioOptimizer) equalWeightOptimize(profiles []StrategyProfile, input OptimizationInput) ([]StrategyWeight, error) {
	n := len(profiles)
	if n == 0 {
		return nil, fmt.Errorf("optimizer: no profiles")
	}
	pct := (100.0 - input.CashReservePct) / float64(n)
	weights := make([]StrategyWeight, n)
	for i, p := range profiles {
		weights[i] = StrategyWeight{
			StrategyID:   p.StrategyID,
			StrategyName: p.StrategyName,
			AllocPct:     pct,
			Rationale:    fmt.Sprintf("equal 1/%d", n),
		}
	}
	return weights, nil
}

// ── Statistical helpers ───────────────────────────────────────────────────────

func enrichProfiles(profiles []StrategyProfile) []StrategyProfile {
	out := make([]StrategyProfile, len(profiles))
	for i, p := range profiles {
		if len(p.Returns) >= 20 {
			p.ExpectedReturnAnnual = mean(p.Returns) * 252
			p.VolatilityAnnual = stddev(p.Returns) * math.Sqrt(252)
			if p.VolatilityAnnual > 0 {
				p.SharpeRatio = (p.ExpectedReturnAnnual - 0.05) / p.VolatilityAnnual
			}
		}
		// Ensure a floor to prevent division by zero
		if p.VolatilityAnnual <= 0 {
			p.VolatilityAnnual = 0.20 // 20% vol assumption
		}
		out[i] = p
	}
	return out
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	m := mean(xs)
	s := 0.0
	for _, x := range xs {
		d := x - m
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)-1))
}

func pearsonCorrelation(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 2 {
		return 0
	}
	ma, mb := mean(a[:n]), mean(b[:n])
	cov, va, vb := 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		da, db := a[i]-ma, b[i]-mb
		cov += da * db
		va += da * da
		vb += db * db
	}
	denom := math.Sqrt(va * vb)
	if denom == 0 {
		return 0
	}
	return cov / denom
}

func computePortfolioStats(weights []StrategyWeight, profiles []StrategyProfile) (retPct, volPct float64) {
	profileMap := make(map[string]StrategyProfile, len(profiles))
	for _, p := range profiles {
		profileMap[p.StrategyID] = p
	}

	weightedReturn := 0.0
	weightedVol := 0.0
	for _, w := range weights {
		p, ok := profileMap[w.StrategyID]
		if !ok {
			continue
		}
		wt := w.AllocPct / 100.0
		weightedReturn += wt * p.ExpectedReturnAnnual * 100
		weightedVol += wt * p.VolatilityAnnual * 100
	}
	return weightedReturn, weightedVol
}

func computeDiversificationRatio(weights []StrategyWeight, profiles []StrategyProfile) float64 {
	profileMap := make(map[string]StrategyProfile, len(profiles))
	for _, p := range profiles {
		profileMap[p.StrategyID] = p
	}
	weightedAvgVol := 0.0
	for _, w := range weights {
		p := profileMap[w.StrategyID]
		weightedAvgVol += (w.AllocPct / 100.0) * p.VolatilityAnnual
	}
	_, portVol := computePortfolioStats(weights, profiles)
	if portVol == 0 {
		return 1.0
	}
	return weightedAvgVol * 100 / portVol
}

func applyWeightConstraints(weights []StrategyWeight, input OptimizationInput) []StrategyWeight {
	for i := range weights {
		if weights[i].AllocPct < input.MinWeightPct {
			weights[i].AllocPct = input.MinWeightPct
		}
		if weights[i].AllocPct > input.MaxWeightPct {
			weights[i].AllocPct = input.MaxWeightPct
		}
	}
	return weights
}

func renormaliseWeights(weights []StrategyWeight, targetTotal float64) []StrategyWeight {
	total := 0.0
	for _, w := range weights {
		total += w.AllocPct
	}
	if total == 0 || targetTotal == 0 {
		return weights
	}
	scale := targetTotal / total
	for i := range weights {
		weights[i].AllocPct = math.Round(weights[i].AllocPct*scale*100) / 100
	}
	return weights
}

func buildWeightChanges(strategies []StrategyProfile, weights []StrategyWeight) []WeightChange {
	currentMap := make(map[string]float64, len(strategies))
	for _, s := range strategies {
		currentMap[s.StrategyID] = 0 // will be populated from allocation engine
	}
	changes := make([]WeightChange, 0, len(weights))
	for _, w := range weights {
		current := currentMap[w.StrategyID]
		changes = append(changes, WeightChange{
			StrategyID:   w.StrategyID,
			StrategyName: w.StrategyName,
			CurrentPct:   current,
			OptimalPct:   w.AllocPct,
			DeltaPct:     w.AllocPct - current,
			Rationale:    w.Rationale,
		})
	}
	sort.Slice(changes, func(i, j int) bool {
		return math.Abs(changes[i].DeltaPct) > math.Abs(changes[j].DeltaPct)
	})
	return changes
}
