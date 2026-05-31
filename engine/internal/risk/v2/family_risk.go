package v2

type FamilyRiskDecision struct {
	Family              StrategyFamily
	FamilyHeatPct       float64
	FamilyAllocationPct float64
	FamilyCorrelation   float64
	FamilyDrawdownPct   float64
	Allowed             bool
	Reason              string
}

func EvaluateFamilyRisk(portfolio Portfolio, req TradeRequest, maxAllocationPct float64) FamilyRiskDecision {
	if maxAllocationPct <= 0 {
		maxAllocationPct = 30
	}
	equity := max(portfolio.Account.EquityUSD, 1)
	notional := 0.0
	risk := 0.0
	for _, pos := range portfolio.Positions {
		if pos.Family != req.Family {
			continue
		}
		notional += PositionNotionalUSD(pos)
		risk += PositionRiskFromPosition(pos)
	}
	allocation := notional / equity * 100
	heat := risk / equity * 100
	out := FamilyRiskDecision{Family: req.Family, FamilyHeatPct: heat, FamilyAllocationPct: allocation, Allowed: allocation <= maxAllocationPct, Reason: "family risk within limits"}
	if !out.Allowed {
		out.Reason = "strategy family allocation exceeds 30% portfolio cap"
	}
	return out
}
