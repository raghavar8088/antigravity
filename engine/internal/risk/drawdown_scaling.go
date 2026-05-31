package risk

func DrawdownPct(equityUSD, highWatermarkUSD float64) float64 {
	if highWatermarkUSD <= 0 || equityUSD >= highWatermarkUSD {
		return 0
	}
	return (highWatermarkUSD - equityUSD) / highWatermarkUSD * 100
}

func DrawdownSizingMultiplier(drawdownPct float64) float64 {
	switch {
	case drawdownPct > 10:
		return 0
	case drawdownPct >= 7:
		return 0.25
	case drawdownPct >= 5:
		return 0.50
	case drawdownPct >= 3:
		return 0.75
	default:
		return 1.0
	}
}

func DrawdownTradingHalt(drawdownPct float64) bool {
	return drawdownPct > 10
}
