package macro

// ComputeMacroScore returns [-3, +3] based on macro environment.
func ComputeMacroScore(data MacroData) float64 {
	score := 0.0
	if data.MacroCoupled {
		switch data.SPY_Dir_1h {
		case "UP":
			score += 2.0
		case "DOWN":
			score -= 2.0
		}
	}
	switch data.DXY_Trend {
	case "RISING":
		score -= 1.0 // strong dollar = BTC headwind
	case "FALLING":
		score += 0.5
	}
	switch {
	case data.VIX > 35:
		score -= 1.5 // extreme fear = risk-off
	case data.VIX > 25:
		score -= 0.5
	case data.VIX < 15:
		score += 0.5 // complacency = risk-on
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
