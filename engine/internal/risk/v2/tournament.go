package v2

type RiskPromotionScore struct {
	RiskAdjustedScore  float64
	RiskContribution   float64
	VaREfficiency      float64
	SharpeStability    float64
	DrawdownStability  float64
	CorrelationPenalty float64
	CompositeRiskScore float64
}

type RiskPromotionDecision struct {
	Approved bool
	Reasons  []string
	Score    RiskPromotionScore
}

func ScoreRiskPromotion(metrics StrategyMetrics, decision RiskDecision) RiskPromotionScore {
	varEfficiency := 100 - clamp(decision.VaR.DailyVaRPct*10, 0, 100)
	sharpeStability := clamp(metrics.Sharpe/2, 0, 1) * 100
	drawdownStability := 100 - clamp(metrics.MaxDrawdownPct*8, 0, 100)
	corrPenalty := clamp(decision.Correlation.MaxCorrelation*100, 0, 100)
	riskContribution := clamp(decision.Attribution.TotalRiskScore, 0, 100)
	riskAdjusted := decision.RiskScore*0.40 + varEfficiency*0.20 + sharpeStability*0.15 + drawdownStability*0.15 + (100-corrPenalty)*0.10
	return RiskPromotionScore{
		RiskAdjustedScore:  riskAdjusted,
		RiskContribution:   riskContribution,
		VaREfficiency:      varEfficiency,
		SharpeStability:    sharpeStability,
		DrawdownStability:  drawdownStability,
		CorrelationPenalty: corrPenalty,
		CompositeRiskScore: riskAdjusted - riskContribution*0.10,
	}
}

func DecideRiskPromotion(metrics StrategyMetrics, decision RiskDecision) RiskPromotionDecision {
	score := ScoreRiskPromotion(metrics, decision)
	out := RiskPromotionDecision{Approved: true, Score: score}
	if metrics.OOSProfitFactor <= 1 {
		out.Approved = false
		out.Reasons = append(out.Reasons, "positive OOS PF required")
	}
	if metrics.OOSExpectancyUSD <= 0 {
		out.Approved = false
		out.Reasons = append(out.Reasons, "positive OOS expectancy required")
	}
	if metrics.Sharpe <= 1.2 {
		out.Approved = false
		out.Reasons = append(out.Reasons, "Sharpe must exceed 1.2")
	}
	if metrics.MaxDrawdownPct >= 10 {
		out.Approved = false
		out.Reasons = append(out.Reasons, "max drawdown above threshold")
	}
	if decision.Correlation.MaxCorrelation >= 0.80 {
		out.Approved = false
		out.Reasons = append(out.Reasons, "correlation above threshold")
	}
	if score.CompositeRiskScore <= 70 {
		out.Approved = false
		out.Reasons = append(out.Reasons, "risk score must exceed 70")
	}
	return out
}
