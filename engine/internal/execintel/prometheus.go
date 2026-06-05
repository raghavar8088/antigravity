package execintel

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for the Phase 22D execution-intelligence layer. These are
// updated from a Snapshot via PublishPrometheus, typically on a periodic tick.
var (
	promConversionRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "conversion_rate_pct",
		Help:      "Signal funnel conversion rates (approval/execution/profit).",
	}, []string{"kind"})

	promMissedEntryRate = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "missed_entry_rate_pct",
		Help:      "Percentage of generated signals that never became trades.",
	})

	promLatencyP99 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "latency_p99_ms",
		Help:      "P99 latency in milliseconds per lifecycle stage.",
	}, []string{"stage"})

	promLatencyP50 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "latency_p50_ms",
		Help:      "P50 latency in milliseconds per lifecycle stage.",
	}, []string{"stage"})

	promSlippageAvgBps = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "slippage_avg_bps",
		Help:      "Average adverse slippage across all fills, in basis points.",
	})

	promExecQuality = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "quality_score",
		Help:      "Composite execution quality score 0-100.",
	})

	promTPNetImpact = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "tp_override_net_impact_usd",
		Help:      "Net realized USD impact of TP overrides (positive = helping).",
	})

	promRejections = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "execintel",
		Name:      "rejections_total",
		Help:      "Missed-entry rejections by canonical reason.",
	}, []string{"reason"})
)

// PublishPrometheus mirrors a Snapshot into the registered Prometheus gauges.
func PublishPrometheus(snap Snapshot) {
	promConversionRate.WithLabelValues("approval").Set(snap.Conversion.ApprovalRatePct)
	promConversionRate.WithLabelValues("execution").Set(snap.Conversion.ExecutionRatePct)
	promConversionRate.WithLabelValues("profit").Set(snap.Conversion.ProfitConversionPct)
	promConversionRate.WithLabelValues("win").Set(snap.Conversion.WinRatePct)

	promMissedEntryRate.Set(snap.Missed.MissedEntryRate)
	promSlippageAvgBps.Set(snap.Slippage.Overall.AvgBps)
	promExecQuality.Set(snap.Quality.Score)
	promTPNetImpact.Set(snap.TPOverride.NetImpactUSD)

	for stage, st := range snap.Latency.ByStage {
		promLatencyP99.WithLabelValues(stage).Set(st.P99)
		promLatencyP50.WithLabelValues(stage).Set(st.P50)
	}
	for reason, count := range snap.Missed.ByReason {
		promRejections.WithLabelValues(string(reason)).Set(float64(count))
	}
}
