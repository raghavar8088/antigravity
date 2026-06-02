package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// ─────────────────────────────────────────────
// Context keys
// ─────────────────────────────────────────────

type ctxKeyTraceID struct{}
type ctxKeySpanID struct{}
type ctxKeyRequestID struct{}

// WithTraceID stores a trace ID in the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKeyTraceID{}, traceID)
}

// TraceIDFromContext retrieves the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyTraceID{}).(string)
	return v
}

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID{}, requestID)
}

// RequestIDFromContext retrieves the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID{}).(string)
	return v
}

// ─────────────────────────────────────────────
// Span — lightweight in-process trace span
// ─────────────────────────────────────────────

// Span represents a named unit of work within a trace.
// Compatible with OpenTelemetry naming conventions; replace with
// go.opentelemetry.io/otel/trace.Span when OTel SDK is wired.
type Span struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Operation string
	StartAt   time.Time
	EndAt     time.Time
	Tags      map[string]string
	Events    []SpanEvent
	Error     error
	finished  int32 // atomic bool
}

// SpanEvent is a timestamped annotation on a span.
type SpanEvent struct {
	At      time.Time
	Message string
	Attrs   map[string]string
}

// End finalises the span and records latency metrics.
func (s *Span) End() {
	if !atomic.CompareAndSwapInt32(&s.finished, 0, 1) {
		return // already ended
	}
	s.EndAt = time.Now()
	durationMs := float64(s.EndAt.Sub(s.StartAt).Milliseconds())
	recordSpanDuration(s.Operation, durationMs, s.Error)
}

// SetTag attaches a string tag to the span.
func (s *Span) SetTag(key, value string) *Span {
	if s.Tags == nil {
		s.Tags = make(map[string]string)
	}
	s.Tags[key] = value
	return s
}

// RecordError marks the span as failed with the given error.
func (s *Span) RecordError(err error) *Span {
	s.Error = err
	return s
}

// AddEvent appends a timestamped event to the span.
func (s *Span) AddEvent(msg string, attrs map[string]string) *Span {
	s.Events = append(s.Events, SpanEvent{At: time.Now(), Message: msg, Attrs: attrs})
	return s
}

// ─────────────────────────────────────────────
// Tracer — span factory
// ─────────────────────────────────────────────

// Tracer creates spans for distributed tracing.
// This implementation is in-process; wire go.opentelemetry.io/otel for cross-service propagation.
type Tracer struct {
	service string
}

// NewTracer creates a Tracer for the named service.
func NewTracer(service string) *Tracer {
	return &Tracer{service: service}
}

// StartSpan creates a new span in the given context.
// The returned context carries the span's trace and span IDs.
func (t *Tracer) StartSpan(ctx context.Context, operation string) (*Span, context.Context) {
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		traceID = newTraceID()
		ctx = WithTraceID(ctx, traceID)
	}
	parentSpanID, _ := ctx.Value(ctxKeySpanID{}).(string)
	spanID := newSpanID()
	ctx = context.WithValue(ctx, ctxKeySpanID{}, spanID)

	span := &Span{
		TraceID:   traceID,
		SpanID:    spanID,
		ParentID:  parentSpanID,
		Operation: operation,
		StartAt:   time.Now(),
		Tags: map[string]string{
			"service": t.service,
		},
	}
	return span, ctx
}

// ─────────────────────────────────────────────
// Pipeline Stage Tracers — named operations aligned to execution data flow
// ─────────────────────────────────────────────

// Global tracer instance; initialised once at startup.
var globalTracer = NewTracer("trading-engine")

// TraceTickReceived starts a span for the tick receive stage.
func TraceTickReceived(ctx context.Context, exchange, symbol string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "marketdata.tick_received")
	span.SetTag("exchange", exchange).SetTag("symbol", symbol)
	return span, ctx
}

// TraceStrategyEval starts a span for strategy evaluation.
func TraceStrategyEval(ctx context.Context, strategy, symbol string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "strategy.evaluate")
	span.SetTag("strategy", strategy).SetTag("symbol", symbol)
	return span, ctx
}

// TraceRiskCheck starts a span for the risk gate check.
func TraceRiskCheck(ctx context.Context, symbol string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "risk.gate_check")
	span.SetTag("symbol", symbol)
	return span, ctx
}

// TraceOrderSubmit starts a span for order submission to the exchange.
func TraceOrderSubmit(ctx context.Context, exchange, symbol, side string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "oms.order_submit")
	span.SetTag("exchange", exchange).SetTag("symbol", symbol).SetTag("side", side)
	return span, ctx
}

// TraceFillReceived starts a span for processing an exchange fill notification.
func TraceFillReceived(ctx context.Context, exchange, orderID string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "execution.fill_received")
	span.SetTag("exchange", exchange).SetTag("order_id", orderID)
	return span, ctx
}

// TraceLedgerWrite starts a span for ledger event write.
func TraceLedgerWrite(ctx context.Context, aggregateType, eventType string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "ledger.write_event")
	span.SetTag("aggregate_type", aggregateType).SetTag("event_type", eventType)
	return span, ctx
}

// TraceReconciliation starts a span for a reconciliation cycle.
func TraceReconciliation(ctx context.Context, exchange, domain string) (*Span, context.Context) {
	span, ctx := globalTracer.StartSpan(ctx, "reconciliation.cycle")
	span.SetTag("exchange", exchange).SetTag("domain", domain)
	return span, ctx
}

// ─────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────

var spanDurationHistogram = func() interface{} { return nil }() // replaced by metric below

func recordSpanDuration(operation string, ms float64, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	SpanDuration.WithLabelValues(operation, status).Observe(ms)
}

// newTraceID generates a 128-bit hex trace ID.
func newTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// newSpanID generates a 64-bit hex span ID.
func newSpanID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
