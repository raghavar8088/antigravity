package riskv3

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics holds all Prometheus gauges and counters for riskv3.
// Use NewPrometheusMetrics() to construct; attach to the Engine via SetPrometheusMetrics.
type PrometheusMetrics struct {
	// Gauges — current portfolio risk state
	HeatPct            prometheus.Gauge
	VaR95Pct           prometheus.Gauge
	VaR99Pct           prometheus.Gauge
	CVaR95Pct          prometheus.Gauge
	CVaR99Pct          prometheus.Gauge
	DrawdownPct        prometheus.Gauge
	DailyLossPct       prometheus.Gauge
	WeeklyLossPct      prometheus.Gauge
	GrossExposurePct   prometheus.Gauge
	NetExposurePct     prometheus.Gauge
	OpenPositions      prometheus.Gauge
	RiskScore          prometheus.Gauge
	ConcentrationScore prometheus.Gauge
	MaxCorrelation     prometheus.Gauge
	KellyFractionPct   prometheus.Gauge

	// Counters — cumulative decision statistics
	RiskChecksTotal   *prometheus.CounterVec // label: status (approved|blocked)
	ViolationsTotal   *prometheus.CounterVec // label: violation_type
	AlertsFiredTotal  *prometheus.CounterVec // label: severity
}

// NewPrometheusMetrics creates and registers all riskv3 Prometheus metrics.
// Safe to call multiple times — uses promauto which panics on duplicate registration,
// so call it once per process.
func NewPrometheusMetrics(accountID string) *PrometheusMetrics {
	labels := prometheus.Labels{"account_id": accountID}
	constLabels := prometheus.Labels{"account_id": accountID}

	return &PrometheusMetrics{
		HeatPct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "portfolio_heat_pct",
			Help:        "Current portfolio heat as % of equity (sum of position dollar risks / equity * 100)",
			ConstLabels: constLabels,
		}),
		VaR95Pct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "var_95_pct",
			Help:        "Daily Value at Risk at 95% confidence as % of equity",
			ConstLabels: constLabels,
		}),
		VaR99Pct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "var_99_pct",
			Help:        "Daily Value at Risk at 99% confidence as % of equity",
			ConstLabels: constLabels,
		}),
		CVaR95Pct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "cvar_95_pct",
			Help:        "Conditional VaR (Expected Shortfall) at 95% confidence as % of equity",
			ConstLabels: constLabels,
		}),
		CVaR99Pct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "cvar_99_pct",
			Help:        "Conditional VaR at 99% confidence as % of equity",
			ConstLabels: constLabels,
		}),
		DrawdownPct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "drawdown_pct",
			Help:        "Current drawdown from high-water mark as %",
			ConstLabels: constLabels,
		}),
		DailyLossPct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "daily_loss_pct",
			Help:        "Today's realised loss as % of equity (positive = loss)",
			ConstLabels: constLabels,
		}),
		WeeklyLossPct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "weekly_loss_pct",
			Help:        "This week's realised loss as % of equity",
			ConstLabels: constLabels,
		}),
		GrossExposurePct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "gross_exposure_pct",
			Help:        "Gross portfolio notional as % of equity",
			ConstLabels: constLabels,
		}),
		NetExposurePct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "net_exposure_pct",
			Help:        "Net portfolio notional (longs - shorts) as % of equity",
			ConstLabels: constLabels,
		}),
		OpenPositions: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "open_positions",
			Help:        "Number of currently open positions",
			ConstLabels: constLabels,
		}),
		RiskScore: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "risk_score",
			Help:        "Composite portfolio risk score 0-100 (higher = safer; <70 blocks new orders)",
			ConstLabels: constLabels,
		}),
		ConcentrationScore: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "concentration_score",
			Help:        "Portfolio concentration score 0-100 (higher = more concentrated = worse)",
			ConstLabels: constLabels,
		}),
		MaxCorrelation: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "max_pairwise_correlation",
			Help:        "Highest absolute pairwise Pearson correlation between open strategy return series",
			ConstLabels: constLabels,
		}),
		KellyFractionPct: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace:   "riskv3",
			Name:        "kelly_fraction_pct",
			Help:        "Current half-Kelly recommended position size as % of equity",
			ConstLabels: constLabels,
		}),
		RiskChecksTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "riskv3",
			Name:        "risk_checks_total",
			Help:        "Total pre-trade risk checks by decision status",
			ConstLabels: labels,
		}, []string{"status"}),
		ViolationsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "riskv3",
			Name:        "violations_total",
			Help:        "Total limit violations by type",
			ConstLabels: labels,
		}, []string{"violation_type"}),
		AlertsFiredTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "riskv3",
			Name:        "alerts_fired_total",
			Help:        "Total portfolio risk alerts fired by severity",
			ConstLabels: labels,
		}, []string{"severity"}),
	}
}

// RecordDecision increments the risk check counter for the given decision.
func (m *PrometheusMetrics) RecordDecision(decision OrderDecision) {
	if m == nil {
		return
	}
	status := "approved"
	if !decision.IsApproved() {
		status = "blocked"
		for _, v := range decision.Violations {
			m.ViolationsTotal.WithLabelValues(string(v.Type)).Inc()
		}
	}
	m.RiskChecksTotal.WithLabelValues(status).Inc()
}

// RecordAlert increments the alert counter for the given severity.
func (m *PrometheusMetrics) RecordAlert(severity AlertSeverity) {
	if m == nil {
		return
	}
	m.AlertsFiredTotal.WithLabelValues(string(severity)).Inc()
}
