package ha

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	dbHealthCheckInterval = 3 * time.Second
	dbConnectTimeout      = 5 * time.Second
	dbFailoverCooldown    = 30 * time.Second
	dbMaxFailures         = 3
)

// DBRole distinguishes primary from replica.
type DBRole int

const (
	DBPrimary DBRole = iota
	DBReplica
	DBUnknown
)

func (r DBRole) String() string {
	switch r {
	case DBPrimary:
		return "primary"
	case DBReplica:
		return "replica"
	default:
		return "unknown"
	}
}

// DBEndpoint describes a PostgreSQL endpoint with health state.
type DBEndpoint struct {
	Name     string
	ConnStr  string
	Role     DBRole
	Healthy  bool
	Latency  time.Duration
	FailedAt time.Time
	Failures int
}

// DatabaseFailover monitors PostgreSQL endpoints and switches the active pool
// to a healthy replica when the primary becomes unhealthy. After promotion,
// the former replica pool is treated as the new primary.
type DatabaseFailover struct {
	nodeID    string
	endpoints []*DBEndpoint
	pools     map[string]*pgxpool.Pool

	mu           sync.RWMutex
	activePool   *pgxpool.Pool
	activeRole   DBRole
	lastFailover time.Time

	failoverCount atomic.Int64

	onFailoverCbs []func(from, to *DBEndpoint)

	metricActive         prometheus.Gauge
	metricFailovers      prometheus.Counter
	metricHealthy        *prometheus.GaugeVec
	metricLatency        *prometheus.HistogramVec
	metricReplicationLag prometheus.Gauge
}

// DBFailoverConfig configures a database failover monitor.
type DBFailoverConfig struct {
	NodeID   string
	Primary  DBEndpointCfg
	Replicas []DBEndpointCfg
	Reg      prometheus.Registerer
}

// DBEndpointCfg holds connection configuration for one DB endpoint.
type DBEndpointCfg struct {
	Name    string
	ConnStr string
}

func NewDatabaseFailover(cfg DBFailoverConfig) (*DatabaseFailover, error) {
	if cfg.Reg == nil {
		cfg.Reg = prometheus.DefaultRegisterer
	}
	df := &DatabaseFailover{
		nodeID: cfg.NodeID,
		pools:  make(map[string]*pgxpool.Pool),
	}

	// Build endpoints list: primary first, replicas after.
	df.endpoints = append(df.endpoints, &DBEndpoint{
		Name:    cfg.Primary.Name,
		ConnStr: cfg.Primary.ConnStr,
		Role:    DBPrimary,
		Healthy: true,
	})
	for _, r := range cfg.Replicas {
		df.endpoints = append(df.endpoints, &DBEndpoint{
			Name:    r.Name,
			ConnStr: r.ConnStr,
			Role:    DBReplica,
			Healthy: true,
		})
	}

	// Build pools.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, ep := range df.endpoints {
		pool, err := pgxpool.New(ctx, ep.ConnStr)
		if err != nil {
			log.Printf("[ha/db-failover] WARNING: could not connect to %s: %v", ep.Name, err)
			ep.Healthy = false
			continue
		}
		df.pools[ep.Name] = pool
	}

	// Set active pool to primary.
	if p, ok := df.pools[cfg.Primary.Name]; ok {
		df.activePool = p
		df.activeRole = DBPrimary
	} else if len(df.pools) > 0 {
		// Primary unreachable at startup — use first available replica.
		for _, ep := range df.endpoints[1:] {
			if p, ok := df.pools[ep.Name]; ok {
				df.activePool = p
				df.activeRole = DBReplica
				log.Printf("[ha/db-failover] WARNING: primary unreachable at startup, using replica %s", ep.Name)
				break
			}
		}
	}

	f := promauto.With(cfg.Reg)
	labels := prometheus.Labels{"node_id": cfg.NodeID}
	df.metricActive = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_db_active_primary", Help: "1 if active pool is primary",
		ConstLabels: labels,
	})
	df.metricFailovers = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_db_failovers_total", Help: "Total DB failovers",
		ConstLabels: labels,
	})
	df.metricHealthy = f.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "ha_db_endpoint_healthy",
		Help:        "1 if endpoint is healthy",
		ConstLabels: labels,
	}, []string{"endpoint", "role"})
	df.metricLatency = f.NewHistogramVec(prometheus.HistogramOpts{
		Name:        "ha_db_health_check_latency_seconds",
		Help:        "DB health check latency",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	}, []string{"endpoint"})
	df.metricReplicationLag = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_db_replication_lag_seconds", Help: "Replication lag to standby",
		ConstLabels: labels,
	})

	return df, nil
}

// Pool returns the currently active database pool (primary or promoted replica).
func (df *DatabaseFailover) Pool() *pgxpool.Pool {
	df.mu.RLock()
	defer df.mu.RUnlock()
	return df.activePool
}

// ActiveRole returns whether the active pool is primary or replica.
func (df *DatabaseFailover) ActiveRole() DBRole {
	df.mu.RLock()
	defer df.mu.RUnlock()
	return df.activeRole
}

// OnFailover registers a callback invoked after DB failover completes.
func (df *DatabaseFailover) OnFailover(cb func(from, to *DBEndpoint)) {
	df.mu.Lock()
	defer df.mu.Unlock()
	df.onFailoverCbs = append(df.onFailoverCbs, cb)
}

// Run starts the health monitoring loop.
func (df *DatabaseFailover) Run(ctx context.Context) error {
	ticker := time.NewTicker(dbHealthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			df.healthCheck(ctx)
		}
	}
}

func (df *DatabaseFailover) healthCheck(ctx context.Context) {
	for _, ep := range df.endpoints {
		pool, ok := df.pools[ep.Name]
		if !ok {
			ep.Healthy = false
			continue
		}
		start := time.Now()
		hctx, cancel := context.WithTimeout(ctx, dbConnectTimeout)
		err := pool.Ping(hctx)
		cancel()
		latency := time.Since(start)

		ep.Latency = latency
		df.metricLatency.WithLabelValues(ep.Name).Observe(latency.Seconds())

		if err != nil {
			ep.Failures++
			if ep.Healthy && ep.Failures >= dbMaxFailures {
				log.Printf("[ha/db-failover] endpoint %s unhealthy after %d failures: %v",
					ep.Name, ep.Failures, err)
				ep.Healthy = false
				ep.FailedAt = time.Now()
				df.metricHealthy.WithLabelValues(ep.Name, ep.Role.String()).Set(0)
				df.handleUnhealthy(ctx, ep)
			}
		} else {
			if !ep.Healthy {
				log.Printf("[ha/db-failover] endpoint %s recovered", ep.Name)
				ep.Healthy = true
				ep.Failures = 0
			}
			df.metricHealthy.WithLabelValues(ep.Name, ep.Role.String()).Set(1)
		}
	}

	// If we are on primary and it is healthy, update active pool.
	df.mu.Lock()
	for _, ep := range df.endpoints {
		if ep.Role == DBPrimary && ep.Healthy {
			if p, ok := df.pools[ep.Name]; ok && df.activeRole != DBPrimary {
				df.activePool = p
				df.activeRole = DBPrimary
				df.metricActive.Set(1)
				log.Printf("[ha/db-failover] reverted to primary %s", ep.Name)
			}
			break
		}
	}
	if df.activeRole == DBPrimary {
		df.metricActive.Set(1)
	} else {
		df.metricActive.Set(0)
	}
	df.mu.Unlock()

	df.measureReplicationLag(ctx)
}

func (df *DatabaseFailover) handleUnhealthy(ctx context.Context, failed *DBEndpoint) {
	df.mu.Lock()
	// Only failover if the failed endpoint is the currently active one.
	if df.activeRole == DBPrimary && failed.Role != DBPrimary {
		df.mu.Unlock()
		return
	}
	if time.Since(df.lastFailover) < dbFailoverCooldown {
		df.mu.Unlock()
		log.Printf("[ha/db-failover] skipping failover: cooldown active")
		return
	}
	df.mu.Unlock()

	// Find the healthiest replica to promote.
	for _, ep := range df.endpoints {
		if ep == failed || !ep.Healthy {
			continue
		}
		pool, ok := df.pools[ep.Name]
		if !ok {
			continue
		}
		df.mu.Lock()
		prev := df.activeRole
		df.activePool = pool
		df.activeRole = DBReplica
		df.lastFailover = time.Now()
		cbs := make([]func(from, to *DBEndpoint), len(df.onFailoverCbs))
		copy(cbs, df.onFailoverCbs)
		df.mu.Unlock()

		df.metricFailovers.Inc()
		df.failoverCount.Add(1)
		log.Printf("[ha/db-failover] FAILOVER: %s (%s) → %s (%s)",
			failed.Name, DBRole(prev).String(), ep.Name, ep.Role.String())

		for _, cb := range cbs {
			cb(failed, ep)
		}
		return
	}
	log.Printf("[ha/db-failover] CRITICAL: no healthy DB endpoint available — all pools exhausted")
}

func (df *DatabaseFailover) measureReplicationLag(ctx context.Context) {
	df.mu.RLock()
	pool := df.activePool
	role := df.activeRole
	df.mu.RUnlock()

	if pool == nil || role != DBPrimary {
		return
	}

	// pg_stat_replication shows lag for streaming standbys.
	var lagSecs float64
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(EXTRACT(EPOCH FROM MAX(now() - pg_last_xact_replay_timestamp())), 0)
	`).Scan(&lagSecs)
	if err == nil {
		df.metricReplicationLag.Set(lagSecs)
	}
}

// Close shuts down all database pools.
func (df *DatabaseFailover) Close() {
	for _, pool := range df.pools {
		pool.Close()
	}
}
