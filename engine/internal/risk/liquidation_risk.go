package risk

import "math"

type LiquidationRiskMetrics struct {
	LiquidationPrice         float64
	DistanceToLiquidationPct float64
	MarginBufferPct          float64
	Probability              float64
}

func LiquidationRisk(pos RiskPosition) LiquidationRiskMetrics {
	if pos.EntryPrice <= 0 || pos.Leverage <= 0 {
		return LiquidationRiskMetrics{DistanceToLiquidationPct: 100, MarginBufferPct: 100}
	}
	maintenance := 0.005
	liq := 0.0
	if pos.Side == SideShort {
		liq = pos.EntryPrice * (1 + 1/pos.Leverage - maintenance)
	} else {
		liq = pos.EntryPrice * (1 - 1/pos.Leverage + maintenance)
	}
	mark := pos.MarkPrice
	if mark <= 0 {
		mark = pos.EntryPrice
	}
	distance := math.Abs(mark-liq) / mark * 100
	prob := 0.0
	if distance < 20 {
		prob = (20 - distance) / 20
	}
	return LiquidationRiskMetrics{
		LiquidationPrice:         liq,
		DistanceToLiquidationPct: distance,
		MarginBufferPct:          distance,
		Probability:              prob,
	}
}

func CheckLiquidationRisk(pos RiskPosition, limits RiskLimits) string {
	if limits.MinLiquidationBufferPct <= 0 {
		return ""
	}
	r := LiquidationRisk(pos)
	if r.DistanceToLiquidationPct < limits.MinLiquidationBufferPct {
		return "liquidation buffer below threshold"
	}
	return ""
}
