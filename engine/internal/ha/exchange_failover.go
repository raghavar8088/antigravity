package ha

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	exchangeCheckInterval   = 5 * time.Second
	exchangeCheckTimeout    = 3 * time.Second
	exchangeRecoveryWindow  = 30 * time.Second
	exchangeMaxConsFailures = 3
)

// ExchangeStatus describes the health state of one exchange endpoint.
type ExchangeStatus int32

const (
	ExchangeHealthy  ExchangeStatus = 0
	ExchangeDegraded ExchangeStatus = 1
	ExchangeDown     ExchangeStatus = 2
)

func (s ExchangeStatus) String() string {
	switch s {
	case ExchangeHealthy:
		return "healthy"
	case ExchangeDegraded:
		return "degraded"
	default:
		return "down"
	}
}

// ExchangeProbe defines how to health-check one exchange.
type ExchangeProbe struct {
	Name       string
	HealthURL  string // HTTP GET endpoint; 200 = healthy
	Priority   int    // lower = preferred (0 = primary)
	Enabled    bool
	httpClient *http.Client
}

// ExchangeFailover monitors exchange probes and routes market data / order
// submission to the highest-priority healthy exchange.
type ExchangeFailover struct {
	nodeID string
	probes []*exchangeState

	mu          sync.RWMutex
	activeProbe *exchangeState

	onFailoverCbs []func(from, to string)

	metricStatus    *prometheus.GaugeVec
	metricFailovers prometheus.Counter
	metricLatency   *prometheus.HistogramVec
	metricActive    *prometheus.GaugeVec
}

type exchangeState struct {
	ExchangeProbe
	status      atomic.Int32
	consecutive int // consecutive failures
	failedAt    time.Time
	lastCheckAt time.Time
	latency     time.Duration
}

func (es *exchangeState) Status() ExchangeStatus {
	return ExchangeStatus(es.status.Load())
}

// ExchangeFailoverConfig configures the exchange failover manager.
type ExchangeFailoverConfig struct {
	NodeID string
	Probes []ExchangeProbe
	Reg    prometheus.Registerer
}

func NewExchangeFailover(cfg ExchangeFailoverConfig) *ExchangeFailover {
	if cfg.Reg == nil {
		cfg.Reg = prometheus.DefaultRegisterer
	}
	ef := &ExchangeFailover{nodeID: cfg.NodeID}

	hc := &http.Client{Timeout: exchangeCheckTimeout}
	for i := range cfg.Probes {
		p := cfg.Probes[i]
		p.httpClient = hc
		es := &exchangeState{ExchangeProbe: p}
		if p.Enabled {
			es.status.Store(int32(ExchangeHealthy))
		} else {
			es.status.Store(int32(ExchangeDown))
		}
		ef.probes = append(ef.probes, es)
	}

	// Set initial active probe to highest-priority enabled one.
	ef.activeProbe = ef.bestProbe()

	f := promauto.With(cfg.Reg)
	labels := prometheus.Labels{"node_id": cfg.NodeID}
	ef.metricStatus = f.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ha_exchange_status", Help: "Exchange health (0=healthy,1=degraded,2=down)",
		ConstLabels: labels,
	}, []string{"exchange"})
	ef.metricFailovers = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_exchange_failovers_total", Help: "Exchange failover count",
		ConstLabels: labels,
	})
	ef.metricLatency = f.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "ha_exchange_health_check_latency_seconds",
		Help:        "Exchange health check latency",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	}, []string{"exchange"})
	ef.metricActive = f.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ha_exchange_active", Help: "1 if this exchange is the active route",
		ConstLabels: labels,
	}, []string{"exchange"})

	return ef
}

// Run starts the continuous health monitoring loop.
func (ef *ExchangeFailover) Run(ctx context.Context) error {
	ticker := time.NewTicker(exchangeCheckInterval)
	defer ticker.Stop()

	// Immediate first check.
	ef.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ef.checkAll(ctx)
		}
	}
}

func (ef *ExchangeFailover) checkAll(ctx context.Context) {
	for _, es := range ef.probes {
		if !es.Enabled {
			continue
		}
		go ef.checkOne(ctx, es)
	}
	// Re-evaluate active probe after checks.
	time.AfterFunc(exchangeCheckTimeout+200*time.Millisecond, func() {
		ef.rebalance()
	})
}

func (ef *ExchangeFailover) checkOne(ctx context.Context, es *exchangeState) {
	if es.HealthURL == "" {
		return
	}
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, es.HealthURL, nil)
	resp, err := es.httpClient.Do(req)
	latency := time.Since(start)
	es.latency = latency
	es.lastCheckAt = time.Now()

	ef.metricLatency.WithLabelValues(es.Name).Observe(latency.Seconds())

	if err != nil || resp.StatusCode >= 500 {
		if resp != nil {
			resp.Body.Close()
		}
		es.consecutive++
		if es.consecutive >= exchangeMaxConsFailures {
			prev := ExchangeStatus(es.status.Swap(int32(ExchangeDown)))
			if prev != ExchangeDown {
				es.failedAt = time.Now()
				log.Printf("[ha/exchange] %s DOWN after %d failures", es.Name, es.consecutive)
				ef.metricStatus.WithLabelValues(es.Name).Set(2)
			}
		} else if es.consecutive == 1 {
			es.status.CompareAndSwap(int32(ExchangeHealthy), int32(ExchangeDegraded))
			ef.metricStatus.WithLabelValues(es.Name).Set(1)
		}
		return
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		prev := ExchangeStatus(es.status.Swap(int32(ExchangeHealthy)))
		es.consecutive = 0
		ef.metricStatus.WithLabelValues(es.Name).Set(0)
		if prev != ExchangeHealthy {
			log.Printf("[ha/exchange] %s RECOVERED", es.Name)
		}
	}
}

func (ef *ExchangeFailover) rebalance() {
	best := ef.bestProbe()
	if best == nil {
		return
	}

	ef.mu.Lock()
	prev := ef.activeProbe
	if prev == best {
		ef.mu.Unlock()
		return
	}
	ef.activeProbe = best
	cbs := make([]func(string, string), len(ef.onFailoverCbs))
	copy(cbs, ef.onFailoverCbs)
	ef.mu.Unlock()

	prevName := "<none>"
	if prev != nil {
		prevName = prev.Name
		ef.metricActive.WithLabelValues(prev.Name).Set(0)
	}
	ef.metricActive.WithLabelValues(best.Name).Set(1)
	ef.metricFailovers.Inc()

	log.Printf("[ha/exchange] route switched: %s → %s", prevName, best.Name)
	for _, cb := range cbs {
		cb(prevName, best.Name)
	}
}

// bestProbe returns the highest-priority (lowest Priority int) healthy probe.
func (ef *ExchangeFailover) bestProbe() *exchangeState {
	var best *exchangeState
	for _, es := range ef.probes {
		if !es.Enabled || es.Status() == ExchangeDown {
			continue
		}
		if best == nil || es.Priority < best.Priority {
			best = es
		}
	}
	return best
}

// ActiveExchange returns the name of the currently active exchange.
func (ef *ExchangeFailover) ActiveExchange() string {
	ef.mu.RLock()
	defer ef.mu.RUnlock()
	if ef.activeProbe == nil {
		return ""
	}
	return ef.activeProbe.Name
}

// IsHealthy returns true if the named exchange is healthy.
func (ef *ExchangeFailover) IsHealthy(name string) bool {
	for _, es := range ef.probes {
		if es.Name == name {
			return es.Status() == ExchangeHealthy
		}
	}
	return false
}

// OnFailover registers a callback invoked on exchange route changes.
func (ef *ExchangeFailover) OnFailover(cb func(from, to string)) {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	ef.onFailoverCbs = append(ef.onFailoverCbs, cb)
}

// AllStatuses returns the current health status of all probes.
func (ef *ExchangeFailover) AllStatuses() map[string]ExchangeStatus {
	out := make(map[string]ExchangeStatus, len(ef.probes))
	for _, es := range ef.probes {
		out[es.Name] = es.Status()
	}
	return out
}

// ForceFailover explicitly routes away from the named exchange.
func (ef *ExchangeFailover) ForceFailover(ctx context.Context, fromExchange string) error {
	for _, es := range ef.probes {
		if es.Name == fromExchange {
			es.status.Store(int32(ExchangeDown))
			es.consecutive = exchangeMaxConsFailures
			ef.rebalance()
			active := ef.ActiveExchange()
			if active == fromExchange || active == "" {
				return fmt.Errorf("exchange failover: no healthy alternative for %s", fromExchange)
			}
			log.Printf("[ha/exchange] forced failover from %s to %s", fromExchange, active)
			return nil
		}
	}
	return fmt.Errorf("exchange failover: unknown exchange %s", fromExchange)
}
