package ha

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// haAdvisoryLockID is the PostgreSQL advisory lock key used for cluster-wide
// leader election. Derived from ASCII "TRADING" = 0x54524144494E47.
const haAdvisoryLockID int64 = 0x54524144494E47

const (
	renewInterval   = 3 * time.Second
	campaignTimeout = 5 * time.Second
)

type Role int32

const (
	RoleFollower  Role = 0
	RoleLeader    Role = 1
	RoleCandidate Role = 2
)

func (r Role) String() string {
	switch r {
	case RoleLeader:
		return "leader"
	case RoleCandidate:
		return "candidate"
	default:
		return "follower"
	}
}

type RoleChangeCallback func(newRole Role, term int64)

// LeaderElection implements distributed leader election using PostgreSQL
// session-level advisory locks. The lock is held for the lifetime of the
// PostgreSQL connection — if the connection drops, the lock is automatically
// released and another node can acquire it. This gives us:
//   - No stale locks (connection-scoped, not transaction-scoped)
//   - No split-brain (only one lock holder at a time)
//   - Automatic failover on crash (connection dies → lock released)
type LeaderElection struct {
	nodeID string
	pool   *pgxpool.Pool

	role atomic.Int32
	term atomic.Int64

	mu        sync.RWMutex
	callbacks []RoleChangeCallback

	// leaderConn holds the dedicated connection that owns the advisory lock.
	// We keep a dedicated connection (not from the pool) so the lock lifetime
	// is independent of query traffic.
	leaderConn *pgx.Conn
	connMu     sync.Mutex

	cancelFn context.CancelFunc

	metricIsLeader  prometheus.Gauge
	metricTerm      prometheus.Gauge
	metricFailovers prometheus.Counter
	metricErrors    prometheus.Counter
}

func NewLeaderElection(nodeID string, pool *pgxpool.Pool, reg prometheus.Registerer) *LeaderElection {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	le := &LeaderElection{nodeID: nodeID, pool: pool}
	f := promauto.With(reg)
	labels := prometheus.Labels{"node_id": nodeID}
	le.metricIsLeader = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_leader_is_leader", Help: "1 if this node holds leadership",
		ConstLabels: labels,
	})
	le.metricTerm = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_leader_term", Help: "Current election term",
		ConstLabels: labels,
	})
	le.metricFailovers = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_leader_failovers_total", Help: "Total leadership transitions",
		ConstLabels: labels,
	})
	le.metricErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_leader_election_errors_total", Help: "Total election errors",
		ConstLabels: labels,
	})
	return le
}

func (le *LeaderElection) OnRoleChange(cb RoleChangeCallback) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.callbacks = append(le.callbacks, cb)
}

func (le *LeaderElection) IsLeader() bool {
	return Role(le.role.Load()) == RoleLeader
}

func (le *LeaderElection) CurrentRole() Role {
	return Role(le.role.Load())
}

func (le *LeaderElection) CurrentTerm() int64 {
	return le.term.Load()
}

func (le *LeaderElection) NodeID() string {
	return le.nodeID
}

// Run starts the election loop. It campaigns for leadership continuously,
// renewing the lock when leader and retrying when follower. Blocks until ctx
// is cancelled.
func (le *LeaderElection) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	le.cancelFn = cancel
	defer func() {
		cancel()
		le.stepDown()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := le.campaign(ctx); err != nil {
			le.metricErrors.Inc()
			log.Printf("[ha/leader] campaign error node=%s: %v", le.nodeID, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(renewInterval):
			}
			continue
		}

		// Became leader — hold lock, renewing the connection keepalive.
		le.holdLeadership(ctx)
	}
}

// campaign attempts to acquire the advisory lock using a fresh dedicated connection.
// Returns nil only when leadership is acquired.
func (le *LeaderElection) campaign(ctx context.Context) error {
	le.setRole(RoleCandidate)

	cctx, cancel := context.WithTimeout(ctx, campaignTimeout)
	defer cancel()

	conn, err := pgx.Connect(cctx, le.pool.Config().ConnString())
	if err != nil {
		return fmt.Errorf("connect for election: %w", err)
	}

	var acquired bool
	if err := conn.QueryRow(cctx,
		"SELECT pg_try_advisory_lock($1)", haAdvisoryLockID,
	).Scan(&acquired); err != nil {
		conn.Close(context.Background())
		return fmt.Errorf("pg_try_advisory_lock: %w", err)
	}

	if !acquired {
		conn.Close(context.Background())
		le.setRole(RoleFollower)
		// Back-off before next attempt.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(renewInterval):
		}
		return nil // Loop will retry.
	}

	// Lock acquired — record the connection.
	le.connMu.Lock()
	le.leaderConn = conn
	le.connMu.Unlock()

	newTerm := le.term.Add(1)
	le.setRole(RoleLeader)
	le.metricFailovers.Inc()
	log.Printf("[ha/leader] node=%s became leader term=%d", le.nodeID, newTerm)
	return nil
}

// holdLeadership keeps the leader connection alive with periodic pings.
// Returns when leadership is lost (connection dropped or ctx cancelled).
func (le *LeaderElection) holdLeadership(ctx context.Context) {
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			le.stepDown()
			return
		case <-ticker.C:
			le.connMu.Lock()
			conn := le.leaderConn
			le.connMu.Unlock()

			if conn == nil {
				le.stepDown()
				return
			}
			// Ping to detect dead connection quickly.
			pctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if err := conn.Ping(pctx); err != nil {
				cancel()
				log.Printf("[ha/leader] leader ping failed node=%s: %v — stepping down", le.nodeID, err)
				le.stepDown()
				return
			}
			cancel()
		}
	}
}

func (le *LeaderElection) stepDown() {
	le.connMu.Lock()
	conn := le.leaderConn
	le.leaderConn = nil
	le.connMu.Unlock()

	if conn != nil {
		// Explicitly release the advisory lock before closing.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", haAdvisoryLockID)
		cancel()
		conn.Close(context.Background())
	}
	le.setRole(RoleFollower)
}

func (le *LeaderElection) setRole(r Role) {
	old := Role(le.role.Swap(int32(r)))
	if old == r {
		return
	}
	term := le.term.Load()
	le.metricIsLeader.Set(map[Role]float64{RoleLeader: 1, RoleFollower: 0, RoleCandidate: 0}[r])
	le.metricTerm.Set(float64(term))

	le.mu.RLock()
	cbs := make([]RoleChangeCallback, len(le.callbacks))
	copy(cbs, le.callbacks)
	le.mu.RUnlock()

	for _, cb := range cbs {
		cb(r, term)
	}
}

// WaitForLeader blocks until this node becomes leader or ctx is cancelled.
func (le *LeaderElection) WaitForLeader(ctx context.Context) error {
	for {
		if le.IsLeader() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// FencingToken returns a monotonically increasing token that must be attached
// to all writes during the current leadership term. Any write with a stale
// token must be rejected by the receiving system.
func (le *LeaderElection) FencingToken() int64 {
	return le.term.Load()
}
