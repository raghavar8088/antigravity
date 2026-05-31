package v2

func RecommendLeverage(req TradeRequest, market MarketState, heat HeatSnapshot, drawdown DrawdownDecision, corr CorrelationReport, limits RiskLimits) float64 {
	maxLev := limits.MaxLeverage
	if maxLev <= 0 {
		maxLev = 5
	}
	lev := min(req.RequestedLeverage, maxLev)
	if lev <= 0 {
		lev = 1
	}
	if market.VolatilityPct > 3 || market.Regime == RegimeHighVol {
		lev = min(lev, 2)
	}
	if abs(market.FundingRate) > 0.0008 || market.Regime == RegimeFundingExtreme {
		lev = min(lev, 2)
	}
	if drawdown.DrawdownPct >= 4 || heat.CurrentHeatPct >= 4 {
		lev = min(lev, 1)
	}
	if corr.HiddenConcentration {
		lev = min(lev, 1)
	}
	return clamp(lev, 1, maxLev)
}
