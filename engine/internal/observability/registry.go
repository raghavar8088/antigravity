package observability

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	defaultRegistry     *prometheus.Registry
	defaultRegistryOnce sync.Once
)

// DefaultRegistry returns the singleton custom Prometheus registry.
// It includes Go runtime and process collectors alongside all trading metrics.
func DefaultRegistry() *prometheus.Registry {
	defaultRegistryOnce.Do(func() {
		reg := prometheus.NewRegistry()
		reg.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)
		defaultRegistry = reg
	})
	return defaultRegistry
}

// MetricsHandler returns an HTTP handler that serves Prometheus metrics.
// It combines the global default registry (promauto metrics) and the custom registry.
func MetricsHandler() http.Handler {
	return promhttp.HandlerFor(
		prometheus.Gatherers{
			prometheus.DefaultGatherer,
			DefaultRegistry(),
		},
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
			ErrorHandling:     promhttp.ContinueOnError,
		},
	)
}
