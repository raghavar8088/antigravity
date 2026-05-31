package v2

type RiskBudgetDecision struct {
	Family       StrategyFamily
	BudgetPct    float64
	UsedPct      float64
	RemainingPct float64
	Allowed      bool
	Reason       string
}

func DefaultRiskBudgets() map[StrategyFamily]float64 {
	return map[StrategyFamily]float64{
		FamilyTrend:         25,
		FamilyOrderFlow:     20,
		FamilyFunding:       15,
		FamilySmartMoney:    15,
		FamilyBreakout:      10,
		FamilyMeanReversion: 10,
		FamilyReserve:       5,
	}
}

func EvaluateRiskBudget(portfolio Portfolio, req TradeRequest, budgets map[StrategyFamily]float64) RiskBudgetDecision {
	budget := budgets[req.Family]
	if budget <= 0 {
		budget = 5
	}
	used := currentFamilyAllocation(portfolio, req.Family)
	out := RiskBudgetDecision{Family: req.Family, BudgetPct: budget, UsedPct: used, RemainingPct: budget - used, Allowed: used <= budget, Reason: "risk budget available"}
	if !out.Allowed {
		out.Reason = "risk budget violation"
	}
	return out
}
