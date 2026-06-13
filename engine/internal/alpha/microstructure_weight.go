package alpha

// MicrostructureSignals carries the real-time market microstructure context
// used to adjust signal confidence before execution.
type MicrostructureSignals struct {
	// Phase 1–5 signals
	DerivativesScore float64 // from derivatives.ComputeDerivativesScore, -3 to +3
	OrderBookScore   float64 // from orderbook.OrderBookAnalysis.Score, -3 to +3
	Regime           string  // from regime.Classifier, e.g. "TRENDING_BULL"
	RegimeMult       float64 // position size multiplier from regime engine

	// Phase C signals (0 if unavailable)
	ETFScore       float64 // from etf.ComputeETFScore, -3 to +3
	DominanceScore float64 // from dominance.ComputeDominanceScore, -3 to +3
	MacroScore     float64 // from macro.ComputeMacroScore, -3 to +3
	SentimentScore float64 // from sentiment.ComputeSentimentScore, -3 to +3

	// Phase C temporal modifier (1.0 = no adjustment, 0.7–1.1 range)
	TemporalMod float64
}

// maxMicroConfidence is the absolute ceiling for AI + microstructure combined.
const maxMicroConfidence = 95.0

// ApplyMicrostructureWeight adjusts baseConfidence (0–100) using all available
// real-time signals. Confidence adjustment is capped at ±15%.
// Temporal modifier is returned separately in TemporalMod for position sizing.
//
// The function clamps the result to [0, maxMicroConfidence].
func ApplyMicrostructureWeight(baseConfidence float64, signals MicrostructureSignals, bias string) float64 {
	// Compute average of all available scores.
	available := []float64{signals.DerivativesScore, signals.OrderBookScore}
	if signals.ETFScore != 0 {
		available = append(available, signals.ETFScore)
	}
	if signals.DominanceScore != 0 {
		available = append(available, signals.DominanceScore)
	}
	if signals.MacroScore != 0 {
		available = append(available, signals.MacroScore)
	}
	if signals.SentimentScore != 0 {
		available = append(available, signals.SentimentScore)
	}

	var sum float64
	for _, v := range available {
		sum += v
	}
	avgScore := sum / float64(len(available))

	// Scale average signal score to ±15% confidence modifier.
	modifier := avgScore / 3.0 * 0.15

	var directedMod float64
	switch bias {
	case "BUY":
		directedMod = modifier
	case "SELL":
		directedMod = -modifier
	default:
		directedMod = 0
	}

	adjusted := baseConfidence + directedMod*100
	if adjusted > maxMicroConfidence {
		adjusted = maxMicroConfidence
	}
	if adjusted < 0 {
		adjusted = 0
	}
	return adjusted
}

// derivativesModifier maps a composite derivatives score to a confidence
// modifier using the same table as derivatives.DerivativesScoreToConfidenceModifier.
func derivativesModifier(score float64) float64 {
	switch {
	case score >= 3.0:
		return 0.15
	case score >= 2.0:
		return 0.10
	case score >= 1.0:
		return 0.05
	case score <= -2.0:
		return -0.20
	case score <= -1.0:
		return -0.10
	case score < 0:
		return -0.05
	default:
		return 0.0
	}
}
