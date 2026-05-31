package risk

const (
	MonthlyLossWarningPct  = 8.0
	MonthlyLossSoftStopPct = 10.0
	MonthlyLossHardStopPct = 12.0
)

func MonthlyLossLevel(lossPct float64) RiskLevel {
	switch {
	case lossPct >= MonthlyLossHardStopPct:
		return RiskLevelBlocked
	case lossPct >= MonthlyLossSoftStopPct:
		return RiskLevelCritical
	case lossPct >= MonthlyLossWarningPct:
		return RiskLevelWarning
	default:
		return RiskLevelNormal
	}
}
