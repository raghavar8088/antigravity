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

const (
	heartbeatInterval = 2 * time.Second
	heartbeatTimeout  = 8 * time.Second // miss 4 beats → declare dead
	heartbeatTableDDL = `
CREATE TABLE IF NOT EXISTS ha_heartbeats (
	node_id       TEXT PRIMARY KEY,
	role          TEXT NOT NULL DEFAULT 'follower',
	term          BIGINT NOT NULL DEFAULT 0,
	last_beat     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	engine_port   INT,
	version       TEXT,
	metadata      JSONB
)`
)

// NodeStatus captures the last-known state of a cluster node.
type NodeStatus struct {
	NodeID     string
	Role       Role
	Term       int64
	LastBeat   time.Time
	EnginePort int
	Version    string
	Alive      bool
}

// Heartbeat writes periodic keepalive rows into ha_heartbeats and exposes
// the health of all cluster nodes. Other components subscribe to NodeDead
// or NodeRevived events to react to topology changes.
type Heartbeat struct {
	nodeID     string
	pool       *pgxpool.Pool
	enginePort int
	version    string

	mu       sync.RWMutex
	nodes    map[string]*NodeStatus
	deadCbs  []func(nodeID string)
	aliveCbs []func(nodeID string)

	metricBeatsSent   prometheus.Counter
	metricNodesAlive  prometheus.Gauge
	metricNodesDead   prometheus.Gauge
	metricBeatLatency prometheus.Histogram
}

func NewHeartbeat(nodeID string, pool *pgxpool.Pool, enginePort int, version string, reg prometheus.Registerer) *Heartbeat {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	h := &Heartbeat{
		nodeID:     nodeID,
		pool:       pool,
		enginePort: enginePort,
		version:    version,
		nodes:      make(map[string]*NodeStatus),
	}
	f := promauto.With(reg)
	labels := prometheus.Labels{"node_id": nodeID}
	h.metricBeatsSent = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_heartbeat_beats_sent_total", Help: "Total heartbeat writes",
		ConstLabels: labels,
	})
	h.metricNodesAlive = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_heartbeat_nodes_alive", Help: "Number of alive cluster nodes",
		ConstLabels: labels,
	})
	h.metricNodesDead = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_heartbeat_nodes_dead", Help: "Number of dead cluster nodes",
		ConstLabels: labels,
	})
	h.metricBeatLatency = f.NewHistogram(prometheus.HistogramOpts{
		Name:        "ha_heartbeat_write_duration_seconds",
		Help:        "Heartbeat write latency",
		ConstLabels: labels,
		Buckets:     prometheus.DefBuckets,
	})
	return h
}

// OnNodeDead registers a callback invoked when a previously-alive node misses
// enough heartbeats to be declared dead.
func (h *Heartbeat) OnNodeDead(cb func(nodeID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.deadCbs = append(h.deadCbs, cb)
}

// OnNodeRevived registers a callback invoked when a dead node sends a heartbeat.
func (h *Heartbeat) OnNodeRevived(cb func(nodeID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.aliveCbs = append(h.aliveCbs, cb)
}

// Run initialises the heartbeat table then starts the write+read loop.
func (h *Heartbeat) Run(ctx context.Context) error {
	if err := h.ensureSchema(ctx); err != nil {
		return fmt.Errorf("heartbeat schema: %w", err)
	}

	go h.readLoop(ctx)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := h.beat(ctx); err != nil {
				log.Printf("[ha/heartbeat] write error node=%s: %v", h.nodeID, err)
			}
		}
	}
}

func (h *Heartbeat) beat(ctx context.Context) error {
	start := time.Now()
	_, err := h.pool.Exec(ctx, `
		INSERT INTO ha_heartbeats (node_id, role, term, last_beat, engine_port, version)
		VALUES ($1, $2, $3, NOW(), $4, $5)
		ON CONFLICT (node_id) DO UPDATE SET
			role        = EXCLUDED.role,
			term        = EXCLUDED.term,
			last_beat   = EXCLUDED.last_beat,
			engine_port = EXCLUDED.engine_port,
			version     = EXCLUDED.version
	`, h.nodeID, h.currentRole(), h.currentTerm(), h.enginePort, h.version)
	h.metricBeatLatency.Observe(time.Since(start).Seconds())
	if err != nil {
		return err
	}
	h.metricBeatsSent.Inc()
	return nil
}

func (h *Heartbeat) readLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.refreshNodes(ctx); err != nil {
				log.Printf("[ha/heartbeat] refresh error: %v", err)
			}
		}
	}
}

func (h *Heartbeat) refreshNodes(ctx context.Context) error {
	rows, err := h.pool.Query(ctx, `
		SELECT node_id, role, term, last_beat, engine_port, version
		FROM ha_heartbeats
		WHERE last_beat > NOW() - INTERVAL '60 seconds'
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	var alive, dead int

	for rows.Next() {
		var ns NodeStatus
		var roleStr string
		var lastBeat time.Time
		if err := rows.Scan(&ns.NodeID, &roleStr, &ns.Term, &lastBeat, &ns.EnginePort, &ns.Version); err != nil {
			continue
		}
		ns.Role = parseRole(roleStr)
		ns.LastBeat = lastBeat
		ns.Alive = time.Since(lastBeat) < heartbeatTimeout
		seen[ns.NodeID] = struct{}{}

		h.mu.Lock()
		prev, exists := h.nodes[ns.NodeID]
		wasDead := exists && !prev.Alive
		h.nodes[ns.NodeID] = &ns
		h.mu.Unlock()

		if ns.Alive {
			alive++
			if wasDead {
				h.dispatchRevived(ns.NodeID)
			}
		} else {
			dead++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Nodes not in DB for 60s are removed from local map and declared dead.
	h.mu.Lock()
	for id, prev := range h.nodes {
		if _, ok := seen[id]; !ok {
			delete(h.nodes, id)
			if prev.Alive {
				h.mu.Unlock()
				h.dispatchDead(id)
				h.mu.Lock()
			}
		} else if !h.nodes[id].Alive && prev.Alive {
			h.mu.Unlock()
			h.dispatchDead(id)
			h.mu.Lock()
		}
	}
	h.mu.Unlock()

	h.metricNodesAlive.Set(float64(alive))
	h.metricNodesDead.Set(float64(dead))
	return nil
}

func (h *Heartbeat) Nodes() []NodeStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]NodeStatus, 0, len(h.nodes))
	for _, n := range h.nodes {
		out = append(out, *n)
	}
	return out
}

func (h *Heartbeat) IsAlive(nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n, ok := h.nodes[nodeID]
	return ok && n.Alive
}

func (h *Heartbeat) AliveCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, n := range h.nodes {
		if n.Alive {
			count++
		}
	}
	return count
}

func (h *Heartbeat) ensureSchema(ctx context.Context) error {
	_, err := h.pool.Exec(ctx, heartbeatTableDDL)
	return err
}

func (h *Heartbeat) dispatchDead(nodeID string) {
	h.mu.RLock()
	cbs := make([]func(string), len(h.deadCbs))
	copy(cbs, h.deadCbs)
	h.mu.RUnlock()
	log.Printf("[ha/heartbeat] node declared dead: %s", nodeID)
	for _, cb := range cbs {
		cb(nodeID)
	}
}

func (h *Heartbeat) dispatchRevived(nodeID string) {
	h.mu.RLock()
	cbs := make([]func(string), len(h.aliveCbs))
	copy(cbs, h.aliveCbs)
	h.mu.RUnlock()
	log.Printf("[ha/heartbeat] node revived: %s", nodeID)
	for _, cb := range cbs {
		cb(nodeID)
	}
}

// currentRole and currentTerm are injected by the cluster coordinator.
// Defaults are safe until the election loop updates them.
var (
	globalRoleFunc func() string = func() string { return "follower" }
	globalTermFunc func() int64  = func() int64 { return 0 }
)

func (h *Heartbeat) SetRoleFunc(fn func() string) { globalRoleFunc = fn }
func (h *Heartbeat) SetTermFunc(fn func() int64)  { globalTermFunc = fn }
func (h *Heartbeat) currentRole() string          { return globalRoleFunc() }
func (h *Heartbeat) currentTerm() int64           { return globalTermFunc() }

func parseRole(s string) Role {
	switch s {
	case "leader":
		return RoleLeader
	case "candidate":
		return RoleCandidate
	default:
		return RoleFollower
	}
}
