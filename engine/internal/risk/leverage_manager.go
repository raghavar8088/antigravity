package risk

func RecommendedLeverage(regime MarketRegime, drawdownPct, heatPct float64) float64 {
	if drawdownPct >= 7 || heatPct >= HeatCriticalPct {
		return 1
	}
	if drawdownPct >= 5 || heatPct >= HeatWarningPct {
		return 2
	}
	switch regime {
	case RegimeExtremeVol:
		return 1
	case RegimeHighVol:
		return 2
	case RegimeTrendingBull, RegimeTrendingBear:
		return 5
	case RegimeRanging:
		return 3
	default:
		return 2
	}
}
