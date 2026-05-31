package v2

import "time"

type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "INFO"
	AlertWarning  AlertSeverity = "WARNING"
	AlertCritical AlertSeverity = "CRITICAL"
)

type RiskAlert struct {
	Severity  AlertSeverity
	Title     string
	Message   string
	Channels  []string
	CreatedAt time.Time
}

func GenerateAlerts(decision RiskDecision) []RiskAlert {
	var alerts []RiskAlert
	add := func(sev AlertSeverity, title, msg string, channels ...string) {
		if len(channels) == 0 {
			channels = []string{"dashboard"}
		}
		alerts = append(alerts, RiskAlert{Severity: sev, Title: title, Message: msg, Channels: channels, CreatedAt: nowUTC()})
	}
	if decision.Heat.Level == HeatReduce || decision.Heat.Level == HeatBlocked || decision.Heat.Level == HeatForceReduce {
		add(AlertWarning, "Heat Breach", decision.Heat.Action, "dashboard", "telegram")
	}
	if decision.VaR.Breached {
		add(AlertCritical, "VaR Breach", "daily VaR exceeded limit", "dashboard", "telegram", "email")
	}
	if decision.CVaR.CVaR95Pct > 9 {
		add(AlertCritical, "CVaR Breach", "expected shortfall exceeded risk envelope", "dashboard", "email")
	}
	if decision.Drawdown.DrawdownPct >= 4 {
		add(AlertWarning, "Drawdown Escalation", decision.Drawdown.Action, "dashboard", "telegram")
	}
	if decision.Correlation.HiddenConcentration {
		add(AlertWarning, "Correlation Spike", "hidden BTC concentration or strategy clustering detected", "dashboard")
	}
	if decision.TailRisk.Action != TailActionNormal {
		add(AlertCritical, "Tail Risk Event", decision.TailRisk.Reason, "dashboard", "telegram", "email", "webhook")
	}
	if decision.Budget.Allowed == false {
		add(AlertWarning, "Risk Budget Violation", decision.Budget.Reason, "dashboard")
	}
	return alerts
}
