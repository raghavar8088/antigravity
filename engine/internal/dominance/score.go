package dominance

import "math"

// ComputeDominanceScore returns [-3, +3] based on BTC.D trend vs price direction.
// priceDir is "UP", "DOWN", or "FLAT".
func ComputeDominanceScore(data DominanceData, priceDir string) float64 {
	score := 0.0
	switch {
	case data.Trend == "RISING" && priceDir == "UP":
		// BTC gaining dominance while rising: genuine strength
		score = 2.0
	case data.Trend == "RISING" && priceDir == "DOWN":
		// BTC dominance rising but price falling: capital rotating into BTC from alts
		score = 0.0
	case data.Trend == "FALLING" && priceDir == "UP":
		// Alts outperforming: alt-season caution
		score = -1.0
	case data.Trend == "FALLING" && priceDir == "DOWN":
		// Broad selloff across crypto
		score = -2.0
	}
	// Strong trend bonus.
	if math.Abs(data.Delta24h) > 1.5 {
		score *= 1.3
	}
	return clamp(score, -3.0, 3.0)
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
