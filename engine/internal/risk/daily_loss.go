package risk

const (
	DailyLossWarningPct  = 2.0
	DailyLossSoftStopPct = 3.0
	DailyLossHardStopPct = 5.0
)

func DailyLossLevel(lossPct float64) RiskLevel {
	switch {
	case lossPct >= DailyLossHardStopPct:
		return RiskLevelBlocked
	case lossPct >= DailyLossSoftStopPct:
		return RiskLevelCritical
	case lossPct >= DailyLossWarningPct:
		return RiskLevelWarning
	default:
		return RiskLevelNormal
	}
}
