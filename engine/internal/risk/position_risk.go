package risk

import "math"

type PositionRiskMetrics struct {
	DollarRisk      float64
	RiskPct         float64
	StopDistanceUSD float64
	StopDistancePct float64
	VolatilityRisk  float64
	LeverageRisk    float64
}

func PositionRisk(pos RiskPosition, equityUSD float64) PositionRiskMetrics {
	stopDistance := math.Abs(pos.EntryPrice - pos.StopLossPrice)
	if pos.StopLossPrice <= 0 || pos.EntryPrice <= 0 {
		stopDistance = pos.EntryPrice * 0.01
	}
	dollarRisk := stopDistance * pos.SizeBTC
	riskPct := 0.0
	if equityUSD > 0 {
		riskPct = dollarRisk / equityUSD * 100
	}
	stopDistancePct := 0.0
	if pos.EntryPrice > 0 {
		stopDistancePct = stopDistance / pos.EntryPrice * 100
	}
	leverageRisk := 0.0
	if pos.Leverage > 1 {
		leverageRisk = (pos.Leverage - 1) / 10
	}
	return PositionRiskMetrics{
		DollarRisk:      dollarRisk,
		RiskPct:         riskPct,
		StopDistanceUSD: stopDistance,
		StopDistancePct: stopDistancePct,
		VolatilityRisk:  stopDistancePct,
		LeverageRisk:    leverageRisk,
	}
}
