package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────
// Execution Pipeline Latency Stages
// Target: Tick → Fill < 150ms
//
// Stage 1:  Tick Receive              → Strategy Evaluation
// Stage 2:  Strategy Signal           → Risk Gate Decision
// Stage 3:  Risk Gate Approval        → OMS Order Created
// Stage 4:  OMS Order Dispatch        → Exchange Acknowledgement
// Stage 5:  Exchange Acknowledgement  → Fill Confirmation
// Stage 6:  Fill Confirmation         → Ledger Write
// Stage 7:  Ledger Write              → CQRS Projection Update
// Stage 8:  Projection Update         → Next.js API Response
// ─────────────────────────────────────────────

// LatencyPipelineStages enumerates the named stages in the execution pipeline.
type LatencyPipelineStage string

const (
	StageTickToStrategy     LatencyPipelineStage = "tick_to_strategy"
	StageStrategyToRisk     LatencyPipelineStage = "strategy_to_risk"
	StageRiskToOMS          LatencyPipelineStage = "risk_to_oms"
	StageOMSToExchange      LatencyPipelineStage = "oms_to_exchange"
	StageExchangeToFill     LatencyPipelineStage = "exchange_to_fill"
	StageFillToLedger       LatencyPipelineStage = "fill_to_ledger"
	StageLedgerToProjection LatencyPipelineStage = "ledger_to_projection"
	StageProjectionToUI     LatencyPipelineStage = "projection_to_ui"
	StageTickToFill         LatencyPipelineStage = "tick_to_fill_e2e"
)

var (
	// PipelineStageLatency measures each stage of the execution pipeline.
	PipelineStageLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "latency",
		Name:      "pipeline_stage_ms",
		Help:      "Execution pipeline stage latency in milliseconds. Observe each stage independently.",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 25, 50, 100, 150, 250, 500},
	}, []string{"stage", "exchange", "symbol"})

	// PipelineE2ELatency is the total tick-to-fill end-to-end latency.
	// CRITICAL: alert when P99 > 150ms.
	PipelineE2ELatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "trading",
		Subsystem: "latency",
		Name:      "e2e_ms",
		Help:      "End-to-end tick-to-fill latency in milliseconds. SLO: P99 < 150ms.",
		Buckets:   []float64{5, 10, 25, 50, 75, 100, 125, 150, 200, 300, 500, 1000},
	}, []string{"exchange", "symbol"})

	// LatencyBreachCount counts SLO breaches (P99 > 150ms).
	LatencyBreachCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "trading",
		Subsystem: "latency",
		Name:      "slo_breaches_total",
		Help:      "Total execution latency SLO breaches (signal→fill > 150ms).",
	}, []string{"exchange", "symbol"})

	// ComponentLatencyP99 is the current P99 latency per pipeline stage (updated periodically).
	ComponentLatencyP99 = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "trading",
		Subsystem: "latency",
		Name:      "component_p99_ms",
		Help:      "Current P99 latency per pipeline component in milliseconds.",
	}, []string{"component"})
)

// ─────────────────────────────────────────────
// PipelineTimer — helper to time pipeline stages
// ─────────────────────────────────────────────

// PipelineTimer accumulates stage timings for a single order execution path.
type PipelineTimer struct {
	exchange string
	symbol   string
	start    time.Time
	stages   []stageRecord
}

type stageRecord struct {
	stage      LatencyPipelineStage
	durationMs float64
}

// NewPipelineTimer creates a new timer anchored to the tick receive time.
func NewPipelineTimer(exchange, symbol string) *PipelineTimer {
	return &PipelineTimer{
		exchange: exchange,
		symbol:   symbol,
		start:    time.Now(),
	}
}

// NewPipelineTimerAt creates a timer anchored to the provided tick timestamp.
// Use this when the tick arrival time is known precisely.
func NewPipelineTimerAt(exchange, symbol string, tickAt time.Time) *PipelineTimer {
	return &PipelineTimer{
		exchange: exchange,
		symbol:   symbol,
		start:    tickAt,
	}
}

// RecordStage records the latency for the given pipeline stage.
// stageStart is when this stage began (usually the end of the previous stage).
func (p *PipelineTimer) RecordStage(stage LatencyPipelineStage, stageStart time.Time) time.Time {
	now := time.Now()
	ms := float64(now.Sub(stageStart).Microseconds()) / 1000.0
	PipelineStageLatency.WithLabelValues(string(stage), p.exchange, p.symbol).Observe(ms)
	p.stages = append(p.stages, stageRecord{stage: stage, durationMs: ms})
	return now
}

// Finalise records the total end-to-end latency and checks SLO compliance.
// Call this when the fill has been confirmed and written to the ledger.
func (p *PipelineTimer) Finalise() {
	totalMs := float64(time.Since(p.start).Microseconds()) / 1000.0
	PipelineE2ELatency.WithLabelValues(p.exchange, p.symbol).Observe(totalMs)

	if totalMs > 150.0 {
		LatencyBreachCount.WithLabelValues(p.exchange, p.symbol).Inc()
		Logger.Warn("execution latency SLO breach",
			"stage", "tick_to_fill",
			"exchange", p.exchange,
			"symbol", p.symbol,
			"latency_ms", totalMs,
			"slo_ms", 150.0,
		)
	}
}

// ─────────────────────────────────────────────
// LatencyTracer — wraps context for stage-by-stage timing
// ─────────────────────────────────────────────

type ctxKeyPipelineTimer struct{}

// WithPipelineTimer stores a PipelineTimer in the context.
func WithPipelineTimer(ctx context.Context, pt *PipelineTimer) context.Context {
	return context.WithValue(ctx, ctxKeyPipelineTimer{}, pt)
}

// PipelineTimerFromContext retrieves the PipelineTimer from context.
// Returns nil if none is present.
func PipelineTimerFromContext(ctx context.Context) *PipelineTimer {
	v, _ := ctx.Value(ctxKeyPipelineTimer{}).(*PipelineTimer)
	return v
}

// RecordPipelineStage records a stage timing using the timer in the context.
// No-ops if no timer is present.
func RecordPipelineStage(ctx context.Context, stage LatencyPipelineStage, stageStart time.Time) time.Time {
	if pt := PipelineTimerFromContext(ctx); pt != nil {
		return pt.RecordStage(stage, stageStart)
	}
	return time.Now()
}
