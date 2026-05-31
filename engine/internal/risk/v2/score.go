package v2

func RiskScore(heat HeatSnapshot, vr VaRReport, cv CVaRReport, corr CorrelationReport, exposure ExposureReport, drawdown DrawdownDecision, tail TailRiskDecision) float64 {
	score := 100.0
	score -= clamp(heat.CurrentHeatPct*5, 0, 35)
	score -= clamp(vr.DailyVaRPct*4, 0, 25)
	score -= clamp(cv.CVaR95Pct*3, 0, 20)
	score -= clamp(corr.MaxCorrelation*20, 0, 20)
	score -= clamp(exposure.GrossExposurePct/20, 0, 10)
	score -= clamp(drawdown.DrawdownPct*4, 0, 30)
	score -= clamp(tail.Score/2, 0, 35)
	return clamp(score, 0, 100)
}
