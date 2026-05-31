package risk

const (
	WeeklyLossWarningPct  = 4.0
	WeeklyLossSoftStopPct = 6.0
	WeeklyLossHardStopPct = 8.0
)

func WeeklyLossLevel(lossPct float64) RiskLevel {
	switch {
	case lossPct >= WeeklyLossHardStopPct:
		return RiskLevelBlocked
	case lossPct >= WeeklyLossSoftStopPct:
		return RiskLevelCritical
	case lossPct >= WeeklyLossWarningPct:
		return RiskLevelWarning
	default:
		return RiskLevelNormal
	}
}
