// Phase 20M — Fund Operations Observability
// Prometheus metrics for fund NAV, investor counts, capital flows,
// fees, compliance violations, and report generation latency.
// Namespace: fundops — no collision with production or research metrics.
package fundops

import (
	"encoding/json"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// FundMetrics holds all Prometheus instruments for fund operations.
type FundMetrics struct {
	Registry *prometheus.Registry

	// NAV
	FundNAV        *prometheus.GaugeVec  // labels: fund_id
	FundNAVPerUnit *prometheus.GaugeVec  // labels: fund_id
	FundUnits      *prometheus.GaugeVec  // labels: fund_id
	FundReturn     *prometheus.GaugeVec  // labels: fund_id, period (daily/ytd)

	// Investors
	InvestorCount  *prometheus.GaugeVec  // labels: fund_id, status
	TotalAUM       *prometheus.GaugeVec  // labels: fund_id

	// Capital Flows
	CapitalSubscribed *prometheus.CounterVec // labels: fund_id
	CapitalRedeemed   *prometheus.CounterVec // labels: fund_id
	DistributionsPaid *prometheus.CounterVec // labels: fund_id

	// Fees
	AccruedMgmtFee *prometheus.GaugeVec   // labels: fund_id
	AccruedPerfFee *prometheus.GaugeVec   // labels: fund_id
	PaidFeesTotal  *prometheus.CounterVec // labels: fund_id, fee_type

	// Compliance
	ComplianceViolations *prometheus.GaugeVec   // labels: fund_id, severity
	ComplianceChecks     *prometheus.CounterVec // labels: fund_id, result (CLEAN/VIOLATIONS)

	// Tax Lots
	OpenTaxLots   *prometheus.GaugeVec  // labels: fund_id
	ClosedTaxLots *prometheus.CounterVec // labels: fund_id
	RealizedGain  *prometheus.GaugeVec  // labels: fund_id

	// Reporting
	ReportGenLatency *prometheus.HistogramVec // labels: fund_id, report_type
	AuditExports     *prometheus.CounterVec   // labels: fund_id

	// Event Store
	FundEvents *prometheus.CounterVec // labels: fund_id, event_type
}

// NewFundMetrics creates and registers all fund operations Prometheus metrics.
func NewFundMetrics(fundID string) *FundMetrics {
	reg := prometheus.NewRegistry()
	labels := prometheus.Labels{"fund": fundID}
	m := &FundMetrics{Registry: reg}

	m.FundNAV = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "nav_usd",
		Help: "Current total fund NAV in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.FundNAVPerUnit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "nav_per_unit",
		Help: "Current NAV per unit.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.FundUnits = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "total_units",
		Help: "Total outstanding units.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.FundReturn = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "return_pct",
		Help: "Fund return percentage by period.", ConstLabels: labels,
	}, []string{"fund_id", "period"})

	m.InvestorCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "investor_count",
		Help: "Number of investors by status.", ConstLabels: labels,
	}, []string{"fund_id", "status"})

	m.TotalAUM = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "total_aum_usd",
		Help: "Total assets under management in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.CapitalSubscribed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "capital_subscribed_usd_total",
		Help: "Total capital subscribed in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.CapitalRedeemed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "capital_redeemed_usd_total",
		Help: "Total capital redeemed in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.DistributionsPaid = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "distributions_paid_usd_total",
		Help: "Total distributions paid in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.AccruedMgmtFee = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "accrued_mgmt_fee_usd",
		Help: "Current accrued management fee in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.AccruedPerfFee = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "accrued_perf_fee_usd",
		Help: "Current accrued performance fee in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.PaidFeesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "fees_paid_usd_total",
		Help: "Total fees paid by type.", ConstLabels: labels,
	}, []string{"fund_id", "fee_type"})

	m.ComplianceViolations = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "compliance_violations",
		Help: "Active compliance violations by severity.", ConstLabels: labels,
	}, []string{"fund_id", "severity"})

	m.ComplianceChecks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "compliance_checks_total",
		Help: "Total compliance checks by result.", ConstLabels: labels,
	}, []string{"fund_id", "result"})

	m.OpenTaxLots = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "open_tax_lots",
		Help: "Number of open tax lots.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.ClosedTaxLots = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "closed_tax_lots_total",
		Help: "Total closed tax lots.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.RealizedGain = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "fundops", Name: "realized_gain_usd",
		Help: "Cumulative realized gain/loss in USD.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.ReportGenLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "fundops", Name: "report_generation_latency_ms",
		Help:    "Investor report generation latency in milliseconds.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		ConstLabels: labels,
	}, []string{"fund_id", "report_type"})

	m.AuditExports = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "audit_exports_total",
		Help: "Total audit packages generated.", ConstLabels: labels,
	}, []string{"fund_id"})

	m.FundEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "fundops", Name: "events_total",
		Help: "Total fund operation events by type.", ConstLabels: labels,
	}, []string{"fund_id", "event_type"})

	for _, c := range []prometheus.Collector{
		m.FundNAV, m.FundNAVPerUnit, m.FundUnits, m.FundReturn,
		m.InvestorCount, m.TotalAUM,
		m.CapitalSubscribed, m.CapitalRedeemed, m.DistributionsPaid,
		m.AccruedMgmtFee, m.AccruedPerfFee, m.PaidFeesTotal,
		m.ComplianceViolations, m.ComplianceChecks,
		m.OpenTaxLots, m.ClosedTaxLots, m.RealizedGain,
		m.ReportGenLatency, m.AuditExports, m.FundEvents,
	} {
		reg.MustRegister(c)
	}
	return m
}

// RecordNAV updates NAV-related gauges.
func (m *FundMetrics) RecordNAV(fundID string, nav, perUnit, units, dailyReturn, ytdReturn float64) {
	m.FundNAV.WithLabelValues(fundID).Set(nav)
	m.FundNAVPerUnit.WithLabelValues(fundID).Set(perUnit)
	m.FundUnits.WithLabelValues(fundID).Set(units)
	m.FundReturn.WithLabelValues(fundID, "daily").Set(dailyReturn)
	m.FundReturn.WithLabelValues(fundID, "ytd").Set(ytdReturn)
}

// RecordCapitalFlow records a capital movement.
func (m *FundMetrics) RecordCapitalFlow(fundID, flowType string, amountUSD float64) {
	switch flowType {
	case "SUBSCRIPTION":
		m.CapitalSubscribed.WithLabelValues(fundID).Add(amountUSD)
	case "REDEMPTION":
		m.CapitalRedeemed.WithLabelValues(fundID).Add(amountUSD)
	case "DISTRIBUTION":
		m.DistributionsPaid.WithLabelValues(fundID).Add(amountUSD)
	}
}

// RecordReportGeneration records investor report generation latency.
func (m *FundMetrics) RecordReportGeneration(fundID, reportType string, dur time.Duration) {
	m.ReportGenLatency.WithLabelValues(fundID, reportType).Observe(float64(dur.Milliseconds()))
}

// RecordEvent records a fund event emission.
func (m *FundMetrics) RecordEvent(fundID string, evType EventType) {
	m.FundEvents.WithLabelValues(fundID, string(evType)).Add(1)
}

// ─── Utility ──────────────────────────────────────────────────────────────────

// unmarshal is a package-level JSON unmarshal helper.
func unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
