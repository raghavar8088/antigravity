package derivatives

import "time"

// ComputeDerivativesScore combines funding and OI scores into a single
// composite score clamped to [-3, +3].
func ComputeDerivativesScore(funding FundingData, oi OIData) DerivativesScore {
	fScore := FundingScore(&funding)
	oiScore := OIScore(&oi)
	total := clamp(fScore+oiScore, -3.0, 3.0)
	return DerivativesScore{
		FundingScore: fScore,
		OIScore:      oiScore,
		TotalScore:   total,
		Funding:      funding,
		OI:           oi,
		ComputedAt:   time.Now().UTC(),
	}
}

// DerivativesScoreToConfidenceModifier maps a composite score in [-3, +3] to
// a confidence modifier in [-0.20, +0.15]. The modifier is added to base
// signal confidence (after clamping to [0, 1]).
func DerivativesScoreToConfidenceModifier(score float64) float64 {
	switch {
	case score >= 3.0:
		return 0.15
	case score >= 2.0:
		return 0.10
	case score >= 1.0:
		return 0.05
	case score == 0:
		return 0.0
	case score >= -1.0:
		return -0.05
	case score >= -2.0:
		return -0.10
	default: // score < -2
		return -0.20
	}
}

// clamp returns v clamped to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
