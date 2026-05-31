package risk

import "math"

func RiskOfRuinPct(winRate, rewardRisk, riskPerTradePct, currentDrawdownPct float64) float64 {
	if riskPerTradePct <= 0 {
		return 0
	}
	if winRate <= 0 {
		return 100
	}
	if rewardRisk <= 0 {
		rewardRisk = 1
	}
	edge := winRate*rewardRisk - (1 - winRate)
	if edge <= 0 {
		return math.Min(100, 60+currentDrawdownPct*4)
	}
	ruinCapitalPct := math.Max(1, 50-currentDrawdownPct)
	betsToRuin := ruinCapitalPct / riskPerTradePct
	prob := math.Pow((1-edge)/(1+edge), betsToRuin) * 100
	if prob < 0 {
		return 0
	}
	if prob > 100 {
		return 100
	}
	return prob
}
