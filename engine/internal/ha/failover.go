package ha

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// FailoverHandler is called when this node transitions to leader.
// It must initialise or take over all leader-only subsystems.
// The fencingToken must be stored and used to validate all writes.
type FailoverHandler interface {
	OnBecameLeader(ctx context.Context, fencingToken int64) error
	OnLostLeadership(ctx context.Context, fencingToken int64) error
}

const (
	failoverGracePeriod = 2 * time.Second // time given to handlers to complete takeover
	maxHandlerTimeout   = 30 * time.Second
)

// Failover coordinates the safe transfer of authority from a dead leader to
// this node. It enforces:
//   - Fencing: all handlers receive the current term as a token; stale writes
//     are rejected by checking token > stored token.
//   - Serialisation: handlers run sequentially in registration order.
//   - Safety window: a grace period separates leadership detection from
//     handler invocation to reduce false-positive failovers.
type Failover struct {
	nodeID string
	pool   *pgxpool.Pool

	mu       sync.RWMutex
	handlers []FailoverHandler

	currentToken int64

	leaderCh chan int64
	lossCh   chan int64

	metricFailoverDuration prometheus.Histogram
	metricHandlerErrors    prometheus.Counter
	metricActiveToken      prometheus.Gauge
}

func NewFailover(nodeID string, pool *pgxpool.Pool, reg prometheus.Registerer) *Failover {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	f := &Failover{
		nodeID:   nodeID,
		pool:     pool,
		leaderCh: make(chan int64, 1),
		lossCh:   make(chan int64, 1),
	}
	pf := promauto.With(reg)
	labels := prometheus.Labels{"node_id": nodeID}
	f.metricFailoverDuration = pf.NewHistogram(prometheus.HistogramOpts{
		Name: "ha_failover_duration_seconds", Help: "Time to complete failover",
		ConstLabels: labels,
		Buckets:     []float64{0.1, 0.5, 1, 2, 5, 10, 30},
	})
	f.metricHandlerErrors = pf.NewCounter(prometheus.CounterOpts{
		Name: "ha_failover_handler_errors_total", Help: "Failover handler errors",
		ConstLabels: labels,
	})
	f.metricActiveToken = pf.NewGauge(prometheus.GaugeOpts{
		Name: "ha_failover_active_token", Help: "Current fencing token",
		ConstLabels: labels,
	})
	return f
}

// Register adds a FailoverHandler. Handlers are invoked in registration order.
func (fo *Failover) Register(h FailoverHandler) {
	fo.mu.Lock()
	defer fo.mu.Unlock()
	fo.handlers = append(fo.handlers, h)
}

// Run processes leadership gain/loss events and dispatches to handlers.
func (fo *Failover) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case token := <-fo.leaderCh:
			fo.executeTakeover(ctx, token)

		case token := <-fo.lossCh:
			fo.executeRelease(ctx, token)
		}
	}
}

// OnBecameLeader is called by the election component when this node wins.
func (fo *Failover) OnBecameLeader(term int64) {
	select {
	case fo.leaderCh <- term:
	default:
		// Channel full means a takeover is already queued.
	}
}

// OnLostLeadership is called when this node steps down.
func (fo *Failover) OnLostLeadership(term int64) {
	select {
	case fo.lossCh <- term:
	default:
	}
}

// OnNodeDead is called by the heartbeat component when another node dies.
// We only need to react if that node was the leader and we won the election —
// the election component handles that; this is a no-op hook for future use.
func (fo *Failover) OnNodeDead(nodeID string) {
	log.Printf("[ha/failover] node dead: %s — waiting for election result", nodeID)
}

func (fo *Failover) executeTakeover(ctx context.Context, token int64) {
	start := time.Now()
	log.Printf("[ha/failover] executing takeover token=%d node=%s", token, fo.nodeID)

	// Grace period: let any in-flight leader operations on the previous node drain.
	select {
	case <-time.After(failoverGracePeriod):
	case <-ctx.Done():
		return
	}

	fo.mu.Lock()
	fo.currentToken = token
	handlers := make([]FailoverHandler, len(fo.handlers))
	copy(handlers, fo.handlers)
	fo.mu.Unlock()

	fo.metricActiveToken.Set(float64(token))

	hctx, cancel := context.WithTimeout(ctx, maxHandlerTimeout)
	defer cancel()

	for _, h := range handlers {
		if err := h.OnBecameLeader(hctx, token); err != nil {
			log.Printf("[ha/failover] handler error during takeover: %v", err)
			fo.metricHandlerErrors.Inc()
			// Continue to next handler — partial failover is better than no failover.
		}
	}

	fo.metricFailoverDuration.Observe(time.Since(start).Seconds())
	log.Printf("[ha/failover] takeover complete token=%d duration=%s", token, time.Since(start))
}

func (fo *Failover) executeRelease(ctx context.Context, token int64) {
	log.Printf("[ha/failover] releasing leadership token=%d node=%s", token, fo.nodeID)

	fo.mu.Lock()
	handlers := make([]FailoverHandler, len(fo.handlers))
	copy(handlers, fo.handlers)
	fo.mu.Unlock()

	hctx, cancel := context.WithTimeout(ctx, maxHandlerTimeout)
	defer cancel()

	for _, h := range handlers {
		if err := h.OnLostLeadership(hctx, token); err != nil {
			log.Printf("[ha/failover] handler error during release: %v", err)
			fo.metricHandlerErrors.Inc()
		}
	}
}

// CurrentToken returns the fencing token for the current leadership period.
func (fo *Failover) CurrentToken() int64 {
	fo.mu.RLock()
	defer fo.mu.RUnlock()
	return fo.currentToken
}

// ValidateToken returns an error if the supplied token is stale.
func (fo *Failover) ValidateToken(token int64) error {
	fo.mu.RLock()
	current := fo.currentToken
	fo.mu.RUnlock()
	if token < current {
		return fmt.Errorf("ha/failover: stale fencing token %d, current %d", token, current)
	}
	return nil
}
