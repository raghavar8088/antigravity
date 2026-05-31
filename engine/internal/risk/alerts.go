package risk

import "time"

type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "INFO"
	AlertSeverityWarning  AlertSeverity = "WARNING"
	AlertSeverityCritical AlertSeverity = "CRITICAL"
)

type RiskAlert struct {
	Severity  AlertSeverity
	Title     string
	Message   string
	CreatedAt time.Time
	Channels  []string
}

func GenerateAlerts(metrics PortfolioMetrics) []RiskAlert {
	alerts := make([]RiskAlert, 0, 4)
	if metrics.PortfolioHeatPct > HeatWarningPct {
		alerts = append(alerts, RiskAlert{Severity: AlertSeverityWarning, Title: "Portfolio heat", Message: "portfolio heat above warning threshold", CreatedAt: time.Now().UTC(), Channels: []string{"dashboard", "telegram", "email"}})
	}
	if metrics.DrawdownPct >= DailyLossSoftStopPct {
		alerts = append(alerts, RiskAlert{Severity: AlertSeverityCritical, Title: "Drawdown breach", Message: "drawdown soft stop breached", CreatedAt: time.Now().UTC(), Channels: []string{"dashboard", "telegram", "email"}})
	}
	if metrics.CVaR95USD > 0 && metrics.EquityUSD > 0 && metrics.CVaR95USD/metrics.EquityUSD*100 > 10 {
		alerts = append(alerts, RiskAlert{Severity: AlertSeverityWarning, Title: "CVaR breach", Message: "expected shortfall above risk envelope", CreatedAt: time.Now().UTC(), Channels: []string{"dashboard"}})
	}
	return alerts
}

func (e *PortfolioRiskEngine) Alerts() []RiskAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := append([]RiskAlert(nil), e.alerts...)
	out = append(out, GenerateAlerts(e.metricsLocked())...)
	return out
}

func (e *PortfolioRiskEngine) alertLocked(severity AlertSeverity, title, message string) {
	e.alerts = append(e.alerts, RiskAlert{
		Severity:  severity,
		Title:     title,
		Message:   message,
		CreatedAt: time.Now().UTC(),
		Channels:  []string{"dashboard", "telegram", "email"},
	})
	if len(e.alerts) > 1000 {
		e.alerts = e.alerts[len(e.alerts)-1000:]
	}
}
