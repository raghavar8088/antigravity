package observability

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SpanDuration tracks the duration of traced operations.
// Registered here to avoid import cycles between tracing.go and metrics.go.
var SpanDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "trading",
	Subsystem: "trace",
	Name:      "span_duration_ms",
	Help:      "Duration of traced spans in milliseconds per operation and status.",
	Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50, 100, 250, 500, 1000},
}, []string{"operation", "status"})

// Config holds startup configuration for the observability stack.
type Config struct {
	// Service name embedded in every log line.
	ServiceName string
	// LogLevel controls minimum log severity (slog.LevelDebug … LevelError).
	LogLevel slog.Level
	// MetricsPath is the HTTP path that serves Prometheus metrics (default: /metrics).
	MetricsPath string
}

// DefaultConfig returns production-safe observability defaults.
func DefaultConfig() Config {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	path := os.Getenv("METRICS_PATH")
	if path == "" {
		path = "/metrics"
	}
	name := os.Getenv("SERVICE_NAME")
	if name == "" {
		name = "trading-engine"
	}
	return Config{
		ServiceName: name,
		LogLevel:    level,
		MetricsPath: path,
	}
}

// Init initialises the full observability stack.
// Call once at engine startup before serving traffic.
func Init(cfg Config) {
	InitLogger(cfg.ServiceName, cfg.LogLevel, os.Stdout)
	Logger.Info("observability stack initialised",
		slog.String("service", cfg.ServiceName),
		slog.String("metrics_path", cfg.MetricsPath),
	)
}

// RegisterMetricsRoute registers the /metrics endpoint on the given mux.
func RegisterMetricsRoute(mux *http.ServeMux, path string) {
	if path == "" {
		path = "/metrics"
	}
	mux.Handle(path, MetricsHandler())
}

// HealthRoute returns an HTTP handler for a fast /healthz liveness probe.
// For the full health report use HealthHandler from health.go.
func HealthRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}

// StartWorkerHeartbeat starts a goroutine that publishes heartbeats for the named worker.
// Call this from each long-running background goroutine.
func StartWorkerHeartbeat(ctx context.Context, workerName string, interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				WorkerHeartbeat.WithLabelValues(workerName).SetToCurrentTime()
			case <-ctx.Done():
				return
			}
		}
	}()
}
