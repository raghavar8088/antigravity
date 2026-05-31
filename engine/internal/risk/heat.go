package risk

const (
	HeatNormalPct    = 6.0
	HeatWarningPct   = 8.0
	HeatCriticalPct  = 10.0
	HeatHardBlockPct = 12.0
)

func PortfolioHeatPct(positions map[string]RiskPosition, equityUSD float64) float64 {
	if equityUSD <= 0 {
		return 0
	}
	total := 0.0
	for _, p := range positions {
		total += PositionRisk(p, equityUSD).RiskPct
	}
	return total
}

func HeatRiskLevel(heatPct float64) RiskLevel {
	switch {
	case heatPct >= HeatHardBlockPct:
		return RiskLevelBlocked
	case heatPct >= HeatCriticalPct:
		return RiskLevelCritical
	case heatPct >= HeatWarningPct:
		return RiskLevelWarning
	default:
		return RiskLevelNormal
	}
}
