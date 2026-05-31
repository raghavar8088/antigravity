package v2

type TailRiskAction string

const (
	TailActionNormal         TailRiskAction = "NORMAL"
	TailActionReduceRisk     TailRiskAction = "REDUCE_RISK"
	TailActionReduceLeverage TailRiskAction = "REDUCE_LEVERAGE"
	TailActionClosePositions TailRiskAction = "CLOSE_POSITIONS"
	TailActionHalt           TailRiskAction = "TRADING_HALT"
)

type TailRiskDecision struct {
	Score      float64
	Action     TailRiskAction
	Reason     string
	Components map[string]float64
}

func EvaluateTailRisk(market MarketState) TailRiskDecision {
	components := map[string]float64{
		"extremeVolatility":  clamp(market.VolatilityPct/8, 0, 1) * 25,
		"flashCrash":         clamp(abs(market.BTCMovePct1m)/3, 0, 1) * 20,
		"liquidityCollapse":  clamp((0.35-market.LiquidityScore)/0.35, 0, 1) * 20,
		"liquidationCascade": clamp(abs(market.LiquidationImbalance)/3, 0, 1) * 20,
		"fundingShock":       clamp(abs(market.FundingRate)/0.0015, 0, 1) * 15,
	}
	score := 0.0
	for _, v := range components {
		score += v
	}
	out := TailRiskDecision{Score: score, Action: TailActionNormal, Reason: "tail risk normal", Components: components}
	switch {
	case score >= 85:
		out.Action = TailActionHalt
		out.Reason = "black swan tail risk"
	case score >= 70:
		out.Action = TailActionClosePositions
		out.Reason = "liquidity or liquidation cascade risk"
	case score >= 50:
		out.Action = TailActionReduceRisk
		out.Reason = "tail risk elevated"
	case score >= 35:
		out.Action = TailActionReduceLeverage
		out.Reason = "tail risk watch"
	}
	return out
}
