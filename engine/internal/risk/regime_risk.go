package risk

func RegimeSizingMultiplier(regime MarketRegime) float64 {
	switch regime {
	case RegimeHighVol:
		return 0.50
	case RegimeExtremeVol:
		return 0
	case RegimeRanging:
		return 0.75
	default:
		return 1
	}
}

func CheckRegimeRisk(order RiskOrder) string {
	if order.Regime == RegimeExtremeVol {
		return "extreme volatility regime trading halt"
	}
	if order.Regime == RegimeRanging && order.StrategyFamily == "Trend" {
		return "ranging regime blocks additional trend exposure"
	}
	return ""
}
