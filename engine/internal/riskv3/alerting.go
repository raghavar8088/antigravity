package riskv3

import (
	"context"
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/ledger"
)

// AlertHandler is called by the alert manager when a new alert fires.
// Implementations can send to Slack, PagerDuty, Prometheus AlertManager, etc.
type AlertHandler interface {
	Handle(ctx context.Context, alert Alert) error
}

// AlertManager detects limit breaches from PortfolioMetrics and fires alerts.
// It suppresses duplicate alerts for the same condition within a cooldown window.
type AlertManager struct {
	handlers    []AlertHandler
	ledger      ledger.Store
	accountID   string
	cooldown    time.Duration
	lastFired   map[string]time.Time // violation type → last fire time
}

// NewAlertManager creates an alert manager with a 5-minute alert cooldown.
func NewAlertManager(store ledger.Store, accountID string, handlers ...AlertHandler) *AlertManager {
	return &AlertManager{
		handlers:  handlers,
		ledger:    store,
		accountID: accountID,
		cooldown:  5 * time.Minute,
		lastFired: make(map[string]time.Time),
	}
}

// Evaluate scans the metrics snapshot for limit breaches and fires alerts.
// Call this after every CheckOrder or periodic snapshot.
func (m *AlertManager) Evaluate(ctx context.Context, metrics PortfolioMetrics) {
	now := time.Now().UTC()

	m.check(ctx, now, metrics.HeatPct, HeatWarningPct, HeatCriticalPct,
		ViolationHeatExceeded, "Portfolio Heat Warning",
		fmt.Sprintf("Portfolio heat is %.1f%% (warning threshold: %.0f%%)", metrics.HeatPct, HeatWarningPct))

	m.check(ctx, now, metrics.HeatPct, HeatCriticalPct, HeatKillPct,
		ViolationHeatExceeded, "Portfolio Heat Critical",
		fmt.Sprintf("Portfolio heat is %.1f%% — CRITICAL: reduce positions", metrics.HeatPct))

	m.checkFatal(ctx, now, metrics.HeatPct >= HeatKillPct,
		ViolationHeatExceeded, "Portfolio Heat KILL SWITCH",
		fmt.Sprintf("Portfolio heat %.1f%% exceeds kill threshold %.0f%%", metrics.HeatPct, HeatKillPct))

	m.check(ctx, now, metrics.DailyVaR95Pct, MaxDailyVaR95Pct, MaxDailyVaR99Pct,
		ViolationVaRExceeded, "VaR Breach",
		fmt.Sprintf("Daily VaR 95%% is %.1f%% (limit: %.0f%%)", metrics.DailyVaR95Pct, MaxDailyVaR95Pct))

	m.check(ctx, now, metrics.CVaR95Pct, MaxCVaR95Pct, MaxCVaR99Pct,
		ViolationCVaRExceeded, "CVaR Breach",
		fmt.Sprintf("CVaR 95%% is %.1f%% (limit: %.0f%%)", metrics.CVaR95Pct, MaxCVaR95Pct))

	m.check(ctx, now, metrics.DrawdownPct, MaxDrawdownPct*0.7, MaxDrawdownPct,
		ViolationDrawdownExceeded, "Drawdown Breach",
		fmt.Sprintf("Drawdown is %.1f%% (limit: %.0f%%)", metrics.DrawdownPct, MaxDrawdownPct))

	m.check(ctx, now, metrics.DailyLossPct, MaxDailyLossPct*0.7, MaxDailyLossPct,
		ViolationDailyLossExceeded, "Daily Loss Breach",
		fmt.Sprintf("Daily loss is %.1f%% (limit: %.0f%%)", metrics.DailyLossPct, MaxDailyLossPct))

	m.check(ctx, now, metrics.MaxSymbolConcentrationPct, MaxAssetConcentrationPct*0.8, MaxAssetConcentrationPct,
		ViolationConcentrationExceeded, "Asset Concentration",
		fmt.Sprintf("Symbol concentration is %.1f%% (limit: %.0f%%)", metrics.MaxSymbolConcentrationPct, MaxAssetConcentrationPct))

	m.check(ctx, now, metrics.MaxExchangeConcentrationPct, MaxExchangeConcentrationPct*0.8, MaxExchangeConcentrationPct,
		ViolationExchangeExceeded, "Exchange Concentration",
		fmt.Sprintf("Exchange concentration is %.1f%% (limit: %.0f%%)", metrics.MaxExchangeConcentrationPct, MaxExchangeConcentrationPct))

	m.check(ctx, now, metrics.MaxPairwiseCorr, MaxCorrelationCoeff*0.8, MaxCorrelationCoeff,
		ViolationCorrelationExceeded, "Correlation Spike",
		fmt.Sprintf("Max pairwise correlation is %.2f (limit: %.2f)", metrics.MaxPairwiseCorr, MaxCorrelationCoeff))
}

// check fires a warning alert when current >= warn, critical when current >= crit.
func (m *AlertManager) check(ctx context.Context, now time.Time, current, warn, crit float64,
	violType ViolationType, title, description string) {
	if current >= crit {
		m.fire(ctx, now, Alert{
			Severity:    AlertSeverityCritical,
			Type:        violType,
			Title:       title + " — CRITICAL",
			Description: description,
			Metric:      string(violType),
			Current:     current,
			Limit:       crit,
			AccountID:   m.accountID,
			DetectedAt:  now,
		})
	} else if current >= warn {
		m.fire(ctx, now, Alert{
			Severity:    AlertSeverityWarning,
			Type:        violType,
			Title:       title + " — WARNING",
			Description: description,
			Metric:      string(violType),
			Current:     current,
			Limit:       warn,
			AccountID:   m.accountID,
			DetectedAt:  now,
		})
	}
}

// checkFatal fires a FATAL alert and emits a kill-switch event.
func (m *AlertManager) checkFatal(ctx context.Context, now time.Time, condition bool,
	violType ViolationType, title, description string) {
	if !condition {
		return
	}
	m.fire(ctx, now, Alert{
		Severity:    AlertSeverityFatal,
		Type:        violType,
		Title:       title,
		Description: description,
		AccountID:   m.accountID,
		DetectedAt:  now,
	})
}

// fire dispatches an alert if the cooldown has expired since the last identical alert.
func (m *AlertManager) fire(ctx context.Context, now time.Time, alert Alert) {
	key := string(alert.Type) + ":" + string(alert.Severity)
	if last, ok := m.lastFired[key]; ok && now.Sub(last) < m.cooldown {
		return // within cooldown window — suppress
	}
	m.lastFired[key] = now

	log.Printf("[ALERT] %s | %s | %s | current=%.2f limit=%.2f",
		alert.Severity, alert.Type, alert.Title, alert.Current, alert.Limit)

	// Emit to ledger
	if m.ledger != nil {
		go m.emitAlert(ctx, alert)
	}

	// Dispatch to all handlers
	for _, h := range m.handlers {
		go func(handler AlertHandler, a Alert) {
			if err := handler.Handle(ctx, a); err != nil {
				log.Printf("[ALERT] handler error: %v", err)
			}
		}(h, alert)
	}
}

func (m *AlertManager) emitAlert(ctx context.Context, alert Alert) {
	et := ledger.EventReconciliationAlert
	switch alert.Type {
	case ViolationHeatExceeded:
		et = ledger.EventPortfolioHeatExceeded
	case ViolationVaRExceeded:
		et = ledger.EventVaRBreach
	case ViolationCVaRExceeded:
		et = ledger.EventCVaRBreach
	case ViolationDrawdownExceeded:
		et = ledger.EventMaxDrawdownBreached
	case ViolationGrossExposureExceeded, ViolationConcentrationExceeded, ViolationExchangeExceeded:
		et = ledger.EventExposureLimitExceeded
	}

	event, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregateRisk,
		AggregateID:   "portfolio-" + m.accountID,
		AccountID:     m.accountID,
		EventType:     et,
		Payload:       alert,
		Source:        "riskv3-alert-manager",
	})
	if err != nil {
		log.Printf("[ALERT] emitAlert build event: %v", err)
		return
	}
	if _, err := m.ledger.Append(ctx, event); err != nil {
		log.Printf("[ALERT] emitAlert append: %v", err)
	}
}

// ─── LogAlertHandler — default handler that logs to stdout ───────────────────

// LogAlertHandler writes alerts to the standard logger.
// Replace with a Slack/PagerDuty handler in production.
type LogAlertHandler struct{}

func (LogAlertHandler) Handle(_ context.Context, alert Alert) error {
	log.Printf("[PORTFOLIO ALERT] [%s] %s — %s (current=%.2f limit=%.2f)",
		alert.Severity, alert.Type, alert.Description, alert.Current, alert.Limit)
	return nil
}
