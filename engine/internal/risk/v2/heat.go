package v2

type HeatLevel string

const (
	HeatNormal      HeatLevel = "NORMAL"
	HeatReduce      HeatLevel = "REDUCE_POSITION_SIZES"
	HeatBlocked     HeatLevel = "NO_NEW_TRADES"
	HeatForceReduce HeatLevel = "FORCE_RISK_REDUCTION"
)

type HeatSnapshot struct {
	CurrentHeatPct float64
	AverageHeatPct float64
	MaxHeatPct     float64
	HeatTrend      float64
	Level          HeatLevel
	Action         string
	SizeMultiplier float64
}

func CalculateHeat(portfolio Portfolio, req TradeRequest, proposedSizeBTC float64) HeatSnapshot {
	equity := portfolio.Account.EquityUSD
	if equity <= 0 {
		equity = 100_000
	}
	totalRisk := 0.0
	for _, pos := range portfolio.Positions {
		totalRisk += PositionRiskFromPosition(pos)
	}
	totalRisk += PositionRiskUSD(req, proposedSizeBTC)
	heat := totalRisk / equity * 100
	out := HeatSnapshot{CurrentHeatPct: heat, AverageHeatPct: heat, MaxHeatPct: heat, SizeMultiplier: 1}
	switch {
	case heat > 8:
		out.Level = HeatForceReduce
		out.Action = "force risk reduction"
		out.SizeMultiplier = 0
	case heat > 6:
		out.Level = HeatBlocked
		out.Action = "block new trades"
		out.SizeMultiplier = 0
	case heat >= 4:
		out.Level = HeatReduce
		out.Action = "reduce position sizes"
		out.SizeMultiplier = 0.50
	default:
		out.Level = HeatNormal
		out.Action = "normal"
	}
	return out
}
