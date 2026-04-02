package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OrdersPlaced = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "raig_orders_total",
		Help: "Total number of orders successfully sent to the exchange",
	}, []string{"symbol", "side"})

	OrdersFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "raig_orders_failed_total",
		Help: "Total number of algorithmic orders physically rejected by risk or exchange",
	})

	CurrentExposure = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raig_exposure_btc",
		Help: "Current Real-Time Bitcoin holding calculated in execution loop",
	}, []string{"strategy"})

	WebSocketLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "raig_ws_latency_ms",
		Help:    "Latency in millisecond to process a live websocket tick",
		Buckets: []float64{1, 5, 10, 50, 100, 500},
	})
)

// RecordOrder increments the Prometheus counter tracking institutional flows.
func RecordOrder(symbol, side string) {
	OrdersPlaced.WithLabelValues(symbol, side).Inc()
}

func RecordFailedOrder() {
	OrdersFailed.Inc()
}

func SetExposure(strategyName string, amount float64) {
	CurrentExposure.WithLabelValues(strategyName).Set(amount)
}
