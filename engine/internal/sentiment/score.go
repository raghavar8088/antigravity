package sentiment

// crisisKeywords trigger a bearish floor regardless of overall score.
var crisisKeywords = []string{"hack", "ban", "crash", "exploit", "rug"}

// ComputeSentimentScore converts SentimentData to a [-3, +3] signal.
func ComputeSentimentScore(data SentimentData) float64 {
	base := data.Score * 3.0 // scale -1..+1 to -3..+3
	// High velocity = noise — reduce weight.
	if data.Velocity > 15 {
		base *= 0.7
	}
	// Crisis keywords override bullish signals.
	if containsAny(data.HotKeywords, crisisKeywords) {
		if base > -1.5 {
			base = -1.5
		}
	}
	return clamp(base, -3.0, 3.0)
}

func containsAny(haystack, needles []string) bool {
	for _, n := range needles {
		for _, h := range haystack {
			if h == n {
				return true
			}
		}
	}
	return false
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
