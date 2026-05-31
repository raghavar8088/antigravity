package v2

type RegimeRiskDecision struct {
	Allowed            bool
	SizeMultiplier     float64
	LeverageMultiplier float64
	Reason             string
}

func EvaluateRegimeRisk(req TradeRequest, market MarketState) RegimeRiskDecision {
	out := RegimeRiskDecision{Allowed: true, SizeMultiplier: 1, LeverageMultiplier: 1, Reason: "regime risk normal"}
	switch market.Regime {
	case RegimeHighVol:
		out.SizeMultiplier = 0.50
		out.LeverageMultiplier = 0.50
		out.Reason = "high volatility reduces position size"
	case RegimeFundingExtreme:
		out.SizeMultiplier = 0.75
		out.LeverageMultiplier = 0.50
		out.Reason = "funding extreme reduces leverage"
	case RegimeRange:
		if req.Family == FamilyBreakout {
			out.SizeMultiplier = 0.50
			out.Reason = "range regime reduces breakout allocation"
		}
	case RegimeTrendingBull, RegimeTrendingBear:
		if req.Family == FamilyMeanReversion {
			out.SizeMultiplier = 0.50
			out.Reason = "trend regime reduces mean reversion allocation"
		}
	}
	return out
}
