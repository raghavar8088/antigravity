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

// ClusterState is the unified view of the HA cluster.
type ClusterState struct {
	LeaderNodeID string
	LeaderTerm   int64
	TotalNodes   int
	AliveNodes   int
	DeadNodes    int
	Nodes        []NodeStatus
	LastUpdated  time.Time
	Quorum       bool // true if >50% of registered nodes are alive
}

// Cluster coordinates the leader election, heartbeat, and failover subsystems
// into a single component that application code interacts with.
type Cluster struct {
	nodeID string
	pool   *pgxpool.Pool

	election  *LeaderElection
	heartbeat *Heartbeat
	failover  *Failover

	mu    sync.RWMutex
	state ClusterState

	readyCh chan struct{}
	once    sync.Once

	metricClusterHealthy prometheus.Gauge
	metricQuorum         prometheus.Gauge
	metricTotalNodes     prometheus.Gauge
}

// ClusterConfig holds configuration for a cluster node.
type ClusterConfig struct {
	NodeID     string
	EnginePort int
	Version    string
	Pool       *pgxpool.Pool
	Registerer prometheus.Registerer
}

// NewCluster creates a fully-wired HA cluster coordinator.
func NewCluster(cfg ClusterConfig) *Cluster {
	if cfg.Registerer == nil {
		cfg.Registerer = prometheus.DefaultRegisterer
	}

	election := NewLeaderElection(cfg.NodeID, cfg.Pool, cfg.Registerer)
	hb := NewHeartbeat(cfg.NodeID, cfg.Pool, cfg.EnginePort, cfg.Version, cfg.Registerer)
	fo := NewFailover(cfg.NodeID, cfg.Pool, cfg.Registerer)

	c := &Cluster{
		nodeID:    cfg.NodeID,
		pool:      cfg.Pool,
		election:  election,
		heartbeat: hb,
		failover:  fo,
		readyCh:   make(chan struct{}),
	}

	f := promauto.With(cfg.Registerer)
	labels := prometheus.Labels{"node_id": cfg.NodeID}
	c.metricClusterHealthy = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_cluster_healthy", Help: "1 if cluster has quorum and a leader",
		ConstLabels: labels,
	})
	c.metricQuorum = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_cluster_quorum", Help: "1 if cluster has quorum",
		ConstLabels: labels,
	})
	c.metricTotalNodes = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_cluster_nodes_total", Help: "Total registered nodes",
		ConstLabels: labels,
	})

	// Wire election role changes → heartbeat role reporting.
	election.OnRoleChange(func(r Role, term int64) {
		hb.SetRoleFunc(func() string { return r.String() })
		hb.SetTermFunc(func() int64 { return term })
		c.updateState()
		log.Printf("[ha/cluster] node=%s role=%s term=%d", cfg.NodeID, r, term)
	})

	// Wire heartbeat node events → failover.
	hb.OnNodeDead(func(nodeID string) {
		c.updateState()
		fo.OnNodeDead(nodeID)
	})
	hb.OnNodeRevived(func(nodeID string) {
		c.updateState()
	})

	// Wire leader changes into failover.
	election.OnRoleChange(func(r Role, term int64) {
		if r == RoleLeader {
			fo.OnBecameLeader(term)
		} else if r == RoleFollower {
			fo.OnLostLeadership(term)
		}
	})

	return c
}

// Run starts all HA subsystems concurrently. Blocks until ctx is cancelled
// or a fatal error occurs in any subsystem.
func (c *Cluster) Run(ctx context.Context) error {
	errCh := make(chan error, 3)

	go func() { errCh <- c.heartbeat.Run(ctx) }()
	go func() { errCh <- c.election.Run(ctx) }()
	go func() { errCh <- c.failover.Run(ctx) }()

	// Signal readiness after first heartbeat cycle.
	go func() {
		time.Sleep(heartbeatInterval + 500*time.Millisecond)
		c.once.Do(func() { close(c.readyCh) })
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return fmt.Errorf("ha subsystem error: %w", err)
	}
}

// WaitReady blocks until the cluster has completed its first topology scan.
func (c *Cluster) WaitReady(ctx context.Context) error {
	select {
	case <-c.readyCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsLeader returns true if this node is the current cluster leader.
func (c *Cluster) IsLeader() bool {
	return c.election.IsLeader()
}

// State returns a snapshot of the current cluster topology.
func (c *Cluster) State() ClusterState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// FencingToken returns the current leadership fencing token.
// All writes guarded by leadership must include this token.
func (c *Cluster) FencingToken() int64 {
	return c.election.FencingToken()
}

// Election exposes the leader election component for external wiring.
func (c *Cluster) Election() *LeaderElection {
	return c.election
}

// Failover exposes the failover component for external wiring.
func (c *Cluster) Failover() *Failover {
	return c.failover
}

// RegisterFailoverHandler adds a callback invoked when this node transitions
// to leader (either by winning election or surviving failover).
func (c *Cluster) RegisterFailoverHandler(fn FailoverHandler) {
	c.failover.Register(fn)
}

func (c *Cluster) updateState() {
	nodes := c.heartbeat.Nodes()
	alive := 0
	dead := 0
	leaderID := ""
	leaderTerm := int64(0)

	for i := range nodes {
		n := &nodes[i]
		if n.Alive {
			alive++
			if n.Role == RoleLeader {
				leaderID = n.NodeID
				leaderTerm = n.Term
			}
		} else {
			dead++
		}
	}

	total := alive + dead
	quorum := total == 0 || alive > total/2

	cs := ClusterState{
		LeaderNodeID: leaderID,
		LeaderTerm:   leaderTerm,
		TotalNodes:   total,
		AliveNodes:   alive,
		DeadNodes:    dead,
		Nodes:        nodes,
		LastUpdated:  time.Now(),
		Quorum:       quorum,
	}

	c.mu.Lock()
	c.state = cs
	c.mu.Unlock()

	healthy := quorum && leaderID != ""
	c.metricClusterHealthy.Set(boolToFloat(healthy))
	c.metricQuorum.Set(boolToFloat(quorum))
	c.metricTotalNodes.Set(float64(total))
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
