package riskv3

import "math"

// KellyInput is the data needed to compute the Kelly position size.
// All inputs must be for the same strategy and must be positive.
type KellyInput struct {
	WinRate  float64 // fraction in [0, 1] — e.g. 0.55 for 55% win rate
	AvgWin   float64 // average winning trade size (in consistent units)
	AvgLoss  float64 // average losing trade size (must be positive)
	Capital  float64 // total available capital in USD
}

// KellyResult contains the sized recommendations and diagnostic fields.
type KellyResult struct {
	// Fractions — what fraction of capital to risk
	FullKellyFraction    float64 `json:"full_kelly_fraction"`    // raw Kelly%
	HalfKellyFraction    float64 `json:"half_kelly_fraction"`    // recommended (default)
	QuarterKellyFraction float64 `json:"quarter_kelly_fraction"` // conservative mode

	// Recommended values — already capped to MaxKellyFractionPct
	RecommendedFractionPct float64 `json:"recommended_fraction_pct"` // % of capital
	RecommendedUSD         float64 `json:"recommended_usd"`           // capital * fraction
	CapApplied             bool    `json:"cap_applied"`                // true when cap was hit

	// Diagnostics
	KellyEdge      float64 `json:"kelly_edge"`      // expected edge per unit bet
	RewardRiskRatio float64 `json:"reward_risk_ratio"` // avg_win / avg_loss
	WinRate        float64 `json:"win_rate"`
	ProfitFactor   float64 `json:"profit_factor"` // (win_rate * avg_win) / ((1-win_rate) * avg_loss)
	RiskOfRuinPct  float64 `json:"risk_of_ruin_pct"` // estimated % probability of ruin
}

// ComputeKelly calculates the full, half, and quarter Kelly fractions.
//
// Kelly formula:
//
//	b = AvgWin / AvgLoss  (reward-to-risk ratio)
//	p = WinRate
//	q = 1 - p
//	Kelly = (p * b - q) / b = p - q/b
//
// The full Kelly is theoretically optimal for log-wealth maximisation but is
// highly sensitive to estimation error. We recommend half-Kelly in practice
// and cap to MaxKellyFractionPct regardless of mode.
func ComputeKelly(input KellyInput) KellyResult {
	if input.WinRate <= 0 || input.WinRate >= 1 {
		input.WinRate = math.Max(0.01, math.Min(0.99, input.WinRate))
	}
	if input.AvgLoss <= 0 {
		input.AvgLoss = 1 // avoid divide-by-zero; caller should validate
	}
	if input.AvgWin <= 0 {
		input.AvgWin = 0
	}

	p := input.WinRate
	q := 1 - p
	b := input.AvgWin / input.AvgLoss // reward-to-risk ratio

	full := (p*b - q) / b
	if full < 0 {
		full = 0 // negative Kelly = edge is negative; don't bet
	}
	half := full * 0.5
	quarter := full * 0.25

	// Recommended is half-Kelly by default, capped to MaxKellyFractionPct
	recommended := half * 100 // convert to %
	capApplied := false
	if recommended > MaxKellyFractionPct {
		recommended = MaxKellyFractionPct
		capApplied = true
	}

	recommendedUSD := 0.0
	if input.Capital > 0 {
		recommendedUSD = input.Capital * recommended / 100
	}

	profitFactor := 0.0
	if input.AvgLoss > 0 && q > 0 {
		profitFactor = (p * input.AvgWin) / (q * input.AvgLoss)
	}

	edge := p*b - q // expected value per unit bet

	return KellyResult{
		FullKellyFraction:      full,
		HalfKellyFraction:      half,
		QuarterKellyFraction:   quarter,
		RecommendedFractionPct: recommended,
		RecommendedUSD:         recommendedUSD,
		CapApplied:             capApplied,
		KellyEdge:              edge,
		RewardRiskRatio:        b,
		WinRate:                input.WinRate,
		ProfitFactor:           profitFactor,
		RiskOfRuinPct:          estimateRiskOfRuin(input.WinRate, b, recommended/100),
	}
}

// KellyFromStats is a convenience wrapper that accepts win/loss counts and
// average PnL values as commonly reported by the strategy tracker.
func KellyFromStats(totalTrades, wins int, avgWin, avgLoss, capital float64) KellyResult {
	if totalTrades <= 0 {
		return KellyResult{RecommendedFractionPct: MaxKellyFractionPct / 4}
	}
	winRate := float64(wins) / float64(totalTrades)
	return ComputeKelly(KellyInput{
		WinRate: winRate,
		AvgWin:  math.Abs(avgWin),
		AvgLoss: math.Abs(avgLoss),
		Capital: capital,
	})
}

// FractionalKellyUSD returns the recommended position size in USD for the
// requested Kelly mode: "full", "half", or "quarter".
// Defaults to half-Kelly when mode is unrecognised.
func FractionalKellyUSD(result KellyResult, mode string, capital float64) float64 {
	var fraction float64
	switch mode {
	case "full":
		fraction = result.FullKellyFraction
	case "quarter":
		fraction = result.QuarterKellyFraction
	default: // "half" or anything else
		fraction = result.HalfKellyFraction
	}
	if fraction < 0 {
		return 0
	}
	capped := fraction * 100
	if capped > MaxKellyFractionPct {
		capped = MaxKellyFractionPct
	}
	return capital * capped / 100
}

// ─── Risk of ruin estimate ────────────────────────────────────────────────────

// estimateRiskOfRuin computes an approximate risk-of-ruin probability.
//
// Model: geometric random walk with probability p of winning b units and
// probability q=1-p of losing 1 unit. Ruin occurs when equity falls to zero.
//
// Closed-form for geometric betting:
//
//	P(ruin) ≈ ((q/p)^(1/f))  where f is the Kelly fraction
//
// When f=0 (no bet) or edge ≤ 0, P(ruin)=1.
// When f approaches full Kelly, P(ruin) → 0 theoretically.
// In practice, estimation error makes P(ruin) much higher than theory.
func estimateRiskOfRuin(winRate, rewardRisk, kellyFraction float64) float64 {
	if kellyFraction <= 0 || winRate <= 0 || rewardRisk <= 0 {
		return 100
	}
	q := 1 - winRate
	p := winRate
	edge := p*rewardRisk - q
	if edge <= 0 {
		return 100
	}

	// Simplified Gambler's Ruin approximation:
	// P(ruin) = ((1 - edge) / (1 + edge)) ^ (1 / kellyFraction)
	base := (1 - edge) / (1 + edge)
	if base <= 0 {
		return 0
	}
	if base >= 1 {
		return 100
	}
	ruinProb := math.Pow(base, 1.0/kellyFraction) * 100
	return math.Min(100, math.Max(0, ruinProb))
}
