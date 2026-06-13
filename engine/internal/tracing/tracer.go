// Package tracing provides distributed tracing for the BTC-PILOT engine.
// It implements a no-op tracer using only stdlib so the build is self-contained.
// To enable real OpenTelemetry + Jaeger tracing, install:
//   go get go.opentelemetry.io/otel@v1.24.0
//   go get go.opentelemetry.io/otel/exporters/jaeger@v1.17.0
//   go get go.opentelemetry.io/otel/sdk@v1.24.0
// and replace the no-op implementation below with the otel provider.
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

// ServiceName is the service identifier used in all trace spans.
const ServiceName = "btc-pilot-engine"

// Config controls tracer initialisation.
type Config struct {
	// JaegerEndpoint is the Jaeger HTTP collector URL.
	// Set via OTEL_JAEGER_ENDPOINT env var.
	JaegerEndpoint string

	// Enabled controls whether spans are created.
	// Disabled spans are zero-cost no-ops.
	Enabled bool

	// SamplingRate is the fraction of cycles to trace (0.0–1.0, default 1.0).
	SamplingRate float64
}

// ConfigFromEnv builds a Config from environment variables.
func ConfigFromEnv() Config {
	enabled := os.Getenv("OTEL_ENABLED") == "true"
	endpoint := os.Getenv("OTEL_JAEGER_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:14268/api/traces"
	}
	rate := 1.0
	if r := os.Getenv("OTEL_SAMPLING_RATE"); r != "" {
		if v, err := strconv.ParseFloat(r, 64); err == nil {
			rate = v
		}
	}
	return Config{JaegerEndpoint: endpoint, Enabled: enabled, SamplingRate: rate}
}

var (
	globalTracer *tracer
	once         sync.Once
)

// InitTracer initialises the global tracer. Returns a shutdown func that must
// be deferred by the caller to flush pending spans.
func InitTracer(cfg Config) (func(), error) {
	shutdown := func() {}
	once.Do(func() {
		if cfg.SamplingRate <= 0 {
			cfg.SamplingRate = 1.0
		}
		globalTracer = &tracer{cfg: cfg}
		if cfg.Enabled {
			slog.Info("[TRACING] OpenTelemetry tracing enabled (no-op impl)",
				"endpoint", cfg.JaegerEndpoint,
				"sampling_rate", cfg.SamplingRate,
				"note", "replace with real OTel SDK for Jaeger export")
		}
	})
	return shutdown, nil
}

// SpanFromContext starts a new child span and returns the updated context.
// Use defer span.End() to close the span.
func SpanFromContext(ctx context.Context, name string) (context.Context, Span) {
	if globalTracer == nil || !globalTracer.cfg.Enabled {
		return ctx, noopSpan{}
	}
	if globalTracer.cfg.SamplingRate < 1.0 && rand.Float64() > globalTracer.cfg.SamplingRate {
		return ctx, noopSpan{}
	}
	s := &activeSpan{
		name:       name,
		startTime:  time.Now(),
		attributes: make(map[string]string),
	}
	// Inherit parent trace ID from context if present.
	if parent, ok := ctx.Value(spanKey{}).(*activeSpan); ok {
		s.traceID = parent.traceID
		s.parentID = parent.spanID
	} else {
		s.traceID = fmt.Sprintf("%016x", rand.Int63())
	}
	s.spanID = fmt.Sprintf("%08x", rand.Int31())
	return context.WithValue(ctx, spanKey{}, s), s
}

// Span is a single unit of tracing work. Must be closed with End().
type Span interface {
	// SetAttribute records a key-value attribute on the span.
	SetAttribute(key, value string)
	// SetError marks the span as errored and records the message.
	SetError(err error)
	// End closes the span and records its duration.
	End()
}

// ─── internal implementation ───────────────────────────────────────────────

type tracer struct {
	cfg Config
}

type spanKey struct{}

type activeSpan struct {
	name       string
	traceID    string
	spanID     string
	parentID   string
	startTime  time.Time
	attributes map[string]string
	errMsg     string
	mu         sync.Mutex
}

func (s *activeSpan) SetAttribute(key, value string) {
	s.mu.Lock()
	s.attributes[key] = value
	s.mu.Unlock()
}

func (s *activeSpan) SetError(err error) {
	if err != nil {
		s.mu.Lock()
		s.errMsg = err.Error()
		s.mu.Unlock()
	}
}

func (s *activeSpan) End() {
	dur := time.Since(s.startTime)
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		attrs := make([]any, 0, len(s.attributes)*2+4)
		attrs = append(attrs, "span", s.name, "trace_id", s.traceID,
			"span_id", s.spanID, "duration_ms", dur.Milliseconds())
		s.mu.Lock()
		for k, v := range s.attributes {
			attrs = append(attrs, k, v)
		}
		if s.errMsg != "" {
			attrs = append(attrs, "error", s.errMsg)
		}
		s.mu.Unlock()
		slog.Debug("[TRACE]", attrs...)
	}
}

// noopSpan is a zero-allocation no-op used when tracing is disabled.
type noopSpan struct{}

func (noopSpan) SetAttribute(_, _ string) {}
func (noopSpan) SetError(_ error)         {}
func (noopSpan) End()                     {}
