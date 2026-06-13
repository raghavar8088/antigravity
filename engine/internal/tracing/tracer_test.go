package tracing

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func resetGlobal() {
	globalTracer = nil
	once = sync.Once{}
}

func TestInitTracer_Disabled(t *testing.T) {
	resetGlobal()
	shutdown, err := InitTracer(Config{Enabled: false, SamplingRate: 1.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	shutdown()
}

func TestInitTracer_Enabled(t *testing.T) {
	resetGlobal()
	shutdown, err := InitTracer(Config{
		Enabled:        true,
		JaegerEndpoint: "http://localhost:14268/api/traces",
		SamplingRate:   1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	shutdown()
}

func TestSpanFromContext_Disabled(t *testing.T) {
	resetGlobal()
	_, _ = InitTracer(Config{Enabled: false})
	ctx, span := SpanFromContext(context.Background(), "test.span")
	if ctx == nil {
		t.Fatal("context must not be nil")
	}
	span.SetAttribute("key", "value")
	span.SetError(errors.New("test"))
	span.End() // must not panic
}

func TestSpanFromContext_Enabled(t *testing.T) {
	resetGlobal()
	_, _ = InitTracer(Config{Enabled: true, SamplingRate: 1.0})
	ctx, span := SpanFromContext(context.Background(), "btc.cycle")
	span.SetAttribute("cycle_id", "15m-1234567890")
	span.SetAttribute("regime", "TRENDING_BULL")
	span.SetAttribute("strategy_count", "600")

	// Child span inherits trace ID.
	_, child := SpanFromContext(ctx, "btc.regime.classify")
	child.SetAttribute("regime", "TRENDING_BULL")
	child.SetAttribute("confidence", "0.92")
	child.End()
	span.End()
}

func TestSpanFromContext_NilInit(t *testing.T) {
	resetGlobal()
	// No InitTracer called — should return noop safely.
	ctx, span := SpanFromContext(context.Background(), "test")
	if ctx == nil {
		t.Fatal("context must not be nil")
	}
	span.End()
}

func TestConfigFromEnv_Defaults(t *testing.T) {
	cfg := ConfigFromEnv()
	if cfg.JaegerEndpoint == "" {
		t.Error("default Jaeger endpoint must not be empty")
	}
	if cfg.SamplingRate != 1.0 {
		t.Errorf("default sampling rate must be 1.0, got %v", cfg.SamplingRate)
	}
}

func TestNoopSpan_AllMethods(t *testing.T) {
	var s noopSpan
	s.SetAttribute("k", "v")
	s.SetError(errors.New("e"))
	s.End()
}
