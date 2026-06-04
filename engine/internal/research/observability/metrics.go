// Package observability provides Phase 19L research-specific Prometheus metrics.
// Uses a private registry to avoid conflicts with the production metric namespace.
// Namespace: research — does not overlap with production recon_v2 or risk_v3.
package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus instruments for the research platform.
type Metrics struct {
	Registry *prometheus.Registry

	// Feature Store
	FeaturesRegistered      prometheus.Gauge
	FeatureVectorsStored    prometheus.Counter
	FeatureGenLatency       *prometheus.HistogramVec // labels: category
	FeatureVersionsTotal    prometheus.Gauge

	// Experiments
	ExperimentsTotal   *prometheus.GaugeVec   // labels: status
	ExperimentDuration *prometheus.HistogramVec // labels: strategy_family
	SharpeOOS          *prometheus.HistogramVec // labels: strategy_id
	MaxDrawdownOOS     *prometheus.HistogramVec // labels: strategy_id

	// Walk-Forward
	WalkForwardRuns     prometheus.Counter
	WalkForwardPassed   prometheus.Counter
	WalkForwardFailed   prometheus.Counter
	WFWindowCount       prometheus.Histogram
	WFEfficiencyRatio   prometheus.Histogram // OOS/IS Sharpe ratio

	// Monte Carlo
	MonteCarloRuns      prometheus.Counter
	MCRiskOfRuin        *prometheus.HistogramVec // labels: strategy_id
	MCSurvivalRate      *prometheus.HistogramVec // labels: strategy_id
	MCSimDuration       *prometheus.HistogramVec // labels: preset (1K/10K/100K)

	// Model Registry
	ModelsTotal         *prometheus.GaugeVec  // labels: state
	ModelTrainingTime   prometheus.Histogram
	ModelValSharpe      prometheus.Histogram

	// Alpha Decay
	AlphaDecayAlerts    *prometheus.CounterVec // labels: state (WARNING/CRITICAL/EXPIRED)
	ICCurrent           *prometheus.GaugeVec   // labels: strategy_id
	HalfLifeDays        *prometheus.GaugeVec   // labels: strategy_id

	// Promotion Pipeline
	PromotionTotal      *prometheus.GaugeVec // labels: state
	PromotionGatePasses *prometheus.CounterVec // labels: gate
	PromotionGateFails  *prometheus.CounterVec // labels: gate

	// Research Throughput
	ResearchEventsTotal  prometheus.Counter
	ResearchReplayTime   prometheus.Histogram
	DataLakeDatasets     prometheus.Gauge
	DataLakeTotalBytes   prometheus.Gauge
}

// NewMetrics creates and registers all research Prometheus metrics.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{Registry: reg}

	// ── Feature Store ──────────────────────────────────────────────────────
	m.FeaturesRegistered = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "featurestore",
		Name: "features_registered_total",
		Help: "Total number of registered feature definitions.",
	})
	m.FeatureVectorsStored = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "featurestore",
		Name: "vectors_stored_total",
		Help: "Total feature vectors stored.",
	})
	m.FeatureGenLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "featurestore",
		Name:    "generation_latency_ms",
		Help:    "Feature generation latency in milliseconds.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
	}, []string{"category"})
	m.FeatureVersionsTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "featurestore",
		Name: "versions_total",
		Help: "Total number of feature versions committed.",
	})

	// ── Experiments ────────────────────────────────────────────────────────
	m.ExperimentsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "experiments",
		Name: "total",
		Help: "Total experiments by status.",
	}, []string{"status"})
	m.ExperimentDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "experiments",
		Name:    "duration_seconds",
		Help:    "Experiment duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12),
	}, []string{"strategy_family"})
	m.SharpeOOS = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "experiments",
		Name:    "sharpe_oos",
		Help:    "Out-of-sample Sharpe ratio distribution.",
		Buckets: []float64{-2, -1, -0.5, 0, 0.25, 0.5, 0.75, 1, 1.5, 2, 3},
	}, []string{"strategy_id"})
	m.MaxDrawdownOOS = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "experiments",
		Name:    "max_drawdown_oos_pct",
		Help:    "Out-of-sample max drawdown (%).",
		Buckets: []float64{5, 10, 15, 20, 25, 30, 40, 50, 75},
	}, []string{"strategy_id"})

	// ── Walk-Forward ───────────────────────────────────────────────────────
	m.WalkForwardRuns = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "walkforward",
		Name: "runs_total", Help: "Total walk-forward runs.",
	})
	m.WalkForwardPassed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "walkforward",
		Name: "passed_total", Help: "Walk-forward runs that passed institutional thresholds.",
	})
	m.WalkForwardFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "walkforward",
		Name: "failed_total", Help: "Walk-forward runs that failed.",
	})
	m.WFWindowCount = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "walkforward",
		Name:    "window_count",
		Help:    "Number of windows per walk-forward run.",
		Buckets: []float64{5, 10, 20, 30, 50, 75, 100},
	})
	m.WFEfficiencyRatio = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "walkforward",
		Name:    "efficiency_ratio",
		Help:    "OOS Sharpe / IS Sharpe — overfitting measure.",
		Buckets: []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.2},
	})

	// ── Monte Carlo ────────────────────────────────────────────────────────
	m.MonteCarloRuns = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "montecarlo",
		Name: "runs_total", Help: "Total Monte Carlo simulation runs.",
	})
	m.MCRiskOfRuin = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "montecarlo",
		Name:    "risk_of_ruin",
		Help:    "Risk of ruin distribution across strategies.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.10, 0.20, 0.50},
	}, []string{"strategy_id"})
	m.MCSurvivalRate = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "montecarlo",
		Name:    "survival_rate",
		Help:    "Fraction of paths ending positive.",
		Buckets: []float64{0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 0.99},
	}, []string{"strategy_id"})
	m.MCSimDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "montecarlo",
		Name:    "simulation_duration_ms",
		Help:    "Monte Carlo simulation duration in milliseconds.",
		Buckets: []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"preset"})

	// ── Model Registry ─────────────────────────────────────────────────────
	m.ModelsTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "models",
		Name: "total",
		Help: "Number of models by lifecycle state.",
	}, []string{"state"})
	m.ModelTrainingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "models",
		Name:    "training_duration_seconds",
		Help:    "Model training duration in seconds.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 12),
	})
	m.ModelValSharpe = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "models",
		Name:    "validation_sharpe",
		Help:    "Validation Sharpe ratio for trained models.",
		Buckets: []float64{-1, 0, 0.25, 0.5, 0.75, 1, 1.5, 2, 3},
	})

	// ── Alpha Decay ────────────────────────────────────────────────────────
	m.AlphaDecayAlerts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "alpha_decay",
		Name: "alerts_total",
		Help: "Alpha decay alerts by severity.",
	}, []string{"state"})
	m.ICCurrent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "alpha_decay",
		Name: "ic_current",
		Help: "Current Information Coefficient per strategy.",
	}, []string{"strategy_id"})
	m.HalfLifeDays = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "alpha_decay",
		Name: "half_life_days",
		Help: "Estimated alpha half-life in days.",
	}, []string{"strategy_id"})

	// ── Promotion Pipeline ─────────────────────────────────────────────────
	m.PromotionTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "promotion",
		Name: "strategies_total",
		Help: "Number of strategies by promotion state.",
	}, []string{"state"})
	m.PromotionGatePasses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "promotion",
		Name: "gate_passes_total",
		Help: "Number of gate passes per gate.",
	}, []string{"gate"})
	m.PromotionGateFails = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "promotion",
		Name: "gate_fails_total",
		Help: "Number of gate failures per gate.",
	}, []string{"gate"})

	// ── Research Throughput ────────────────────────────────────────────────
	m.ResearchEventsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "research", Subsystem: "events",
		Name: "total",
		Help: "Total research events emitted.",
	})
	m.ResearchReplayTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "research", Subsystem: "events",
		Name:    "replay_duration_ms",
		Help:    "Research event replay duration in milliseconds.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
	})
	m.DataLakeDatasets = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "datalake",
		Name: "datasets_total",
		Help: "Total datasets in the research data lake.",
	})
	m.DataLakeTotalBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "research", Subsystem: "datalake",
		Name: "total_bytes",
		Help: "Total bytes stored in the research data lake.",
	})

	// Register all metrics.
	for _, c := range []prometheus.Collector{
		m.FeaturesRegistered, m.FeatureVectorsStored,
		m.FeatureGenLatency, m.FeatureVersionsTotal,
		m.ExperimentsTotal, m.ExperimentDuration,
		m.SharpeOOS, m.MaxDrawdownOOS,
		m.WalkForwardRuns, m.WalkForwardPassed,
		m.WalkForwardFailed, m.WFWindowCount, m.WFEfficiencyRatio,
		m.MonteCarloRuns, m.MCRiskOfRuin, m.MCSurvivalRate, m.MCSimDuration,
		m.ModelsTotal, m.ModelTrainingTime, m.ModelValSharpe,
		m.AlphaDecayAlerts, m.ICCurrent, m.HalfLifeDays,
		m.PromotionTotal, m.PromotionGatePasses, m.PromotionGateFails,
		m.ResearchEventsTotal, m.ResearchReplayTime,
		m.DataLakeDatasets, m.DataLakeTotalBytes,
	} {
		reg.MustRegister(c)
	}

	return m
}

// RecordFeatureGeneration records a feature generation latency observation.
func (m *Metrics) RecordFeatureGeneration(category string, dur time.Duration) {
	m.FeatureGenLatency.WithLabelValues(category).Observe(float64(dur.Milliseconds()))
	m.FeatureVectorsStored.Add(1)
}

// RecordWalkForward records the outcome of a walk-forward run.
func (m *Metrics) RecordWalkForward(passed bool, windowCount int, efficiencyRatio float64) {
	m.WalkForwardRuns.Add(1)
	if passed {
		m.WalkForwardPassed.Add(1)
	} else {
		m.WalkForwardFailed.Add(1)
	}
	m.WFWindowCount.Observe(float64(windowCount))
	m.WFEfficiencyRatio.Observe(efficiencyRatio)
}

// RecordMonteCarlo records a Monte Carlo simulation result.
func (m *Metrics) RecordMonteCarlo(strategyID, preset string, riskOfRuin, survivalRate float64, dur time.Duration) {
	m.MonteCarloRuns.Add(1)
	m.MCRiskOfRuin.WithLabelValues(strategyID).Observe(riskOfRuin)
	m.MCSurvivalRate.WithLabelValues(strategyID).Observe(survivalRate)
	m.MCSimDuration.WithLabelValues(preset).Observe(float64(dur.Milliseconds()))
}

// RecordDecay records alpha decay monitoring results.
func (m *Metrics) RecordDecay(strategyID, state string, ic, halfLife float64) {
	m.ICCurrent.WithLabelValues(strategyID).Set(ic)
	m.HalfLifeDays.WithLabelValues(strategyID).Set(halfLife)
	if state == "WARNING" || state == "CRITICAL" || state == "EXPIRED" {
		m.AlphaDecayAlerts.WithLabelValues(state).Add(1)
	}
}

// RecordGateResult records a promotion gate pass or fail.
func (m *Metrics) RecordGateResult(gate string, passed bool) {
	if passed {
		m.PromotionGatePasses.WithLabelValues(gate).Add(1)
	} else {
		m.PromotionGateFails.WithLabelValues(gate).Add(1)
	}
}
