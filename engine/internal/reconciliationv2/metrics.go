package reconciliationv2

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the Phase 15E Exchange Reconciliation Authority.
// Namespace: recon_v2 — does not conflict with riskv3's risk_v3 namespace.
// Uses a private registry so multiple instances (e.g. in tests) don't conflict.
type Metrics struct {
	Registry           *prometheus.Registry
	DriftScore         *prometheus.GaugeVec
	MismatchCount      *prometheus.GaugeVec
	CriticalCount      *prometheus.GaugeVec
	RepairCount        *prometheus.CounterVec
	EscalationCount    *prometheus.CounterVec
	CycleCount         *prometheus.CounterVec
	CycleDurationMs    *prometheus.HistogramVec
	ExchangeHealthy    *prometheus.GaugeVec
	BalanceDriftPct    *prometheus.GaugeVec
	PositionDriftCount *prometheus.GaugeVec
	OrderDriftCount    *prometheus.GaugeVec
	FillDriftCount     *prometheus.GaugeVec
	ExposureDriftPct   *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metrics in a private registry.
// The caller may expose the registry via promhttp.HandlerFor(m.Registry, ...).
func NewMetrics(accountID string) *Metrics {
	reg := prometheus.NewRegistry()
	labels := prometheus.Labels{"account": accountID}

	m := &Metrics{
		Registry: reg,
		DriftScore: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "drift_score",
			Help:        "Overall drift score [0–100] for the most recent reconciliation cycle.",
			ConstLabels: labels,
		}, []string{"exchange", "domain"}),

		MismatchCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "mismatch_count",
			Help:        "Number of mismatches detected per domain.",
			ConstLabels: labels,
		}, []string{"exchange", "domain", "severity"}),

		CriticalCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "critical_mismatch_count",
			Help:        "Number of CRITICAL mismatches in the most recent cycle.",
			ConstLabels: labels,
		}, []string{"exchange", "domain"}),

		RepairCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "recon_v2",
			Name:        "repairs_total",
			Help:        "Total automatic repairs performed.",
			ConstLabels: labels,
		}, []string{"exchange", "domain", "repair_type", "success"}),

		EscalationCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "recon_v2",
			Name:        "escalations_total",
			Help:        "Total mismatches escalated to manual intervention.",
			ConstLabels: labels,
		}, []string{"exchange", "domain", "severity"}),

		CycleCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace:   "recon_v2",
			Name:        "cycles_total",
			Help:        "Total reconciliation cycles completed.",
			ConstLabels: labels,
		}, []string{"exchange", "domain"}),

		CycleDurationMs: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace:   "recon_v2",
			Name:        "cycle_duration_ms",
			Help:        "Reconciliation cycle duration in milliseconds.",
			ConstLabels: labels,
			Buckets:     []float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
		}, []string{"exchange", "domain"}),

		ExchangeHealthy: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "exchange_healthy",
			Help:        "1 if exchange adapter health check passed, 0 otherwise.",
			ConstLabels: labels,
		}, []string{"exchange"}),

		BalanceDriftPct: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "balance_drift_pct",
			Help:        "Latest balance drift percentage between exchange and OMS.",
			ConstLabels: labels,
		}, []string{"exchange", "metric"}),

		PositionDriftCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "position_drift_count",
			Help:        "Number of position mismatches detected.",
			ConstLabels: labels,
		}, []string{"exchange", "type"}),

		OrderDriftCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "order_drift_count",
			Help:        "Number of order mismatches detected.",
			ConstLabels: labels,
		}, []string{"exchange", "type"}),

		FillDriftCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "fill_drift_count",
			Help:        "Number of fill mismatches detected.",
			ConstLabels: labels,
		}, []string{"exchange", "type"}),

		ExposureDriftPct: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   "recon_v2",
			Name:        "exposure_drift_pct",
			Help:        "Latest exposure drift percentage.",
			ConstLabels: labels,
		}, []string{"exchange", "metric"}),
	}

	reg.MustRegister(
		m.DriftScore,
		m.MismatchCount,
		m.CriticalCount,
		m.RepairCount,
		m.EscalationCount,
		m.CycleCount,
		m.CycleDurationMs,
		m.ExchangeHealthy,
		m.BalanceDriftPct,
		m.PositionDriftCount,
		m.OrderDriftCount,
		m.FillDriftCount,
		m.ExposureDriftPct,
	)

	return m
}

// RecordCycle updates all metrics after a reconciliation cycle completes.
func (m *Metrics) RecordCycle(exchange string, domain MismatchDomain, entry AuditEntry) {
	d := string(domain)
	m.DriftScore.WithLabelValues(exchange, d).Set(entry.DriftScore)
	m.CycleCount.WithLabelValues(exchange, d).Inc()
	m.CycleDurationMs.WithLabelValues(exchange, d).Observe(float64(entry.DurationMs))

	// Reset mismatch gauges for this domain then recount.
	m.MismatchCount.Reset()
	m.CriticalCount.Reset()
	m.PositionDriftCount.Reset()
	m.OrderDriftCount.Reset()
	m.FillDriftCount.Reset()

	sevCounts := map[string]float64{}
	for _, mm := range entry.Mismatches {
		sevCounts[string(mm.Severity)]++
		switch mm.Domain {
		case DomainPosition:
			m.PositionDriftCount.WithLabelValues(exchange, mm.Type).Inc()
		case DomainOrder:
			m.OrderDriftCount.WithLabelValues(exchange, mm.Type).Inc()
		case DomainFill:
			m.FillDriftCount.WithLabelValues(exchange, mm.Type).Inc()
		}
	}
	for sev, cnt := range sevCounts {
		m.MismatchCount.WithLabelValues(exchange, d, sev).Set(cnt)
		if sev == string(SeverityCritical) {
			m.CriticalCount.WithLabelValues(exchange, d).Set(cnt)
		}
	}

	// Record repairs and escalations.
	for _, r := range entry.RepairResults {
		success := "true"
		if !r.Success {
			success = "false"
		}
		if r.RepairType == RepairTypeManualIntervention {
			m.EscalationCount.WithLabelValues(exchange, d, string(SeverityCritical)).Inc()
		} else {
			m.RepairCount.WithLabelValues(exchange, d, r.RepairType, success).Inc()
		}
	}
}

// SetExchangeHealth updates the exchange health gauge.
func (m *Metrics) SetExchangeHealth(exchange string, healthy bool) {
	v := 0.0
	if healthy {
		v = 1.0
	}
	m.ExchangeHealthy.WithLabelValues(exchange).Set(v)
}

// SetBalanceDrift records the latest balance drift metrics.
func (m *Metrics) SetBalanceDrift(exchange, metric string, driftPct float64) {
	m.BalanceDriftPct.WithLabelValues(exchange, metric).Set(driftPct)
}

// SetExposureDrift records the latest exposure drift metrics.
func (m *Metrics) SetExposureDrift(exchange, metric string, driftPct float64) {
	m.ExposureDriftPct.WithLabelValues(exchange, metric).Set(driftPct)
}
