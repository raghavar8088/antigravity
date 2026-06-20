// Package paperpersist is the Phase 31A MongoDB-centric paper trading
// persistence layer.
//
// Design principles:
//   - Go Engine on Render is the sole execution authority.
//   - MongoDB Atlas is the single source of truth.
//   - account_key is ALWAYS the server-hardcoded OWNER_ACCOUNT_KEY ("owner_admin").
//     It is NEVER read from query strings, request bodies, or websocket messages.
//   - All writes are idempotent upserts keyed on natural identifiers.
//   - Runs in parallel with existing SQLite + Phase30 persistence (additive only).
//
// Phase 31A collection catalogue:
//
//	paper_trades          — closed trade records (one doc per trade)
//	paper_positions       — open position state (upserted)
//	paper_orders          — OMS lifecycle transition log
//	paper_state           — account snapshot (singleton per account)
//	equity_curve          — 5-minute equity snapshots (TTL 90 days)
//	daily_pnl_history     — midnight-sealed daily PnL
//	strategy_scores       — per-strategy performance scores
//	strategy_health       — per-strategy health status
//	portfolio_metrics     — portfolio-level metrics
//	risk_events           — risk gate decisions
//	worker_events         — background worker heartbeats
//	execution_logs        — execution debug log
package paperpersist

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ── Security: account key is ALWAYS server-sourced ───────────────────────────

// AccountKey returns the server-hardcoded account key. It is the only
// authorised way to obtain the key inside this package.
func AccountKey() string {
	return EffectiveOwnerAccountKey()
}

// ── Schema version ────────────────────────────────────────────────────────────

const schemaVersion = 31

// ── Collection name constants ─────────────────────────────────────────────────

const (
	ColPaperTrades      = "paper_trades"
	ColPaperPositions   = "paper_positions"
	ColPaperOrders      = "paper_orders"
	ColPaperState       = "paper_state"
	ColEquityCurve      = "equity_curve"
	ColDailyPnL         = "daily_pnl_history"
	ColStrategyScores   = "strategy_scores"
	ColStrategyHealth   = "strategy_health"
	ColPortfolioMetrics = "portfolio_metrics"
	ColRiskEvents       = "risk_events"
	ColWorkerEvents     = "worker_events"
	ColExecutionLogs    = "execution_logs"
)

// ── Observability counters ────────────────────────────────────────────────────

// Metrics provides basic observability without requiring Prometheus import.
type Metrics struct {
	WriteLatencySum  int64 // nanoseconds, atomic
	WriteCount       int64 // atomic
	FailedWrites     int64 // atomic
	RetryCount       int64 // atomic
	QueueDepth       int64 // atomic
	PersistenceLagNs int64 // atomic — ns between trade close and MongoDB ack
}

var M = &Metrics{}

func (m *Metrics) RecordWrite(latency time.Duration) {
	atomic.AddInt64(&m.WriteLatencySum, int64(latency))
	atomic.AddInt64(&m.WriteCount, 1)
}

func (m *Metrics) RecordFailed()  { atomic.AddInt64(&m.FailedWrites, 1) }
func (m *Metrics) RecordRetry()   { atomic.AddInt64(&m.RetryCount, 1) }
func (m *Metrics) IncQueue()      { atomic.AddInt64(&m.QueueDepth, 1) }
func (m *Metrics) DecQueue()      { atomic.AddInt64(&m.QueueDepth, -1) }
func (m *Metrics) RecordLag(d time.Duration) {
	atomic.StoreInt64(&m.PersistenceLagNs, int64(d))
}

func (m *Metrics) AvgWriteLatency() time.Duration {
	n := atomic.LoadInt64(&m.WriteCount)
	if n == 0 {
		return 0
	}
	return time.Duration(atomic.LoadInt64(&m.WriteLatencySum) / n)
}

// ── MongoManager ──────────────────────────────────────────────────────────────

// MongoManager owns the MongoDB connection pool, reconnect handling, and
// graceful shutdown. It is the single shared handle for all paperpersist writers.
type MongoManager struct {
	mu        sync.RWMutex
	mc        *mongo.Client
	db        *mongo.Database
	dbName    string
	uri       string
	connected bool

	// closed is set to true once Shutdown() is called.
	closed bool
}

// NewMongoManager creates and connects a MongoManager.
// Returns an error if the initial connection or ping fails.
func NewMongoManager(ctx context.Context) (*MongoManager, error) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "loop_trades"
	}

	m := &MongoManager{uri: uri, dbName: dbName}
	if err := m.connect(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *MongoManager) connect(ctx context.Context) error {
	// M0 connection-cap guard: the pool is opened PER replica-set node, so 20 here
	// becomes ~60 sockets on a 3-node cluster. Cap small and reap idle connections
	// in seconds (not 5 minutes) so redeploys/restarts don't pile up stale sockets
	// against the Atlas M0 500-connection ceiling.
	clientOpts := options.Client().
		ApplyURI(m.uri).
		SetMaxPoolSize(10).
		SetMinPoolSize(0).
		SetMaxConnIdleTime(30 * time.Second).
		SetServerSelectionTimeout(10 * time.Second).
		SetConnectTimeout(15 * time.Second)

	mc, err := mongo.Connect(clientOpts)
	if err != nil {
		return fmt.Errorf("paperpersist connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := mc.Ping(pingCtx, nil); err != nil {
		_ = mc.Disconnect(ctx)
		return fmt.Errorf("paperpersist ping: %w", err)
	}

	m.mu.Lock()
	m.mc = mc
	m.db = mc.Database(m.dbName)
	m.connected = true
	m.mu.Unlock()

	log.Printf("[paperpersist] connected db=%s account_key=%s", m.dbName, AccountKey())
	return nil
}

// DB returns the underlying *mongo.Database handle. Needed by packages that
// require a raw database reference (e.g. mongopersist.EnsureIndexes).
// Returns nil if the manager is not connected.
func (m *MongoManager) DB() *mongo.Database {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

// EnsureIndexes creates all required indexes. Called once at startup.
// Duplicate index creation is harmless and logged, not fatal.
func (m *MongoManager) EnsureIndexes(ctx context.Context) error {
	type spec struct {
		col    string
		keys   bson.D
		unique bool
		sparse bool
		ttlSec int32 // 0 = no TTL
	}

	specs := []spec{
		// paper_trades — one doc per trade, unique on client_trade_id
		{ColPaperTrades, bson.D{{Key: "client_trade_id", Value: 1}}, true, false, 0},
		{ColPaperTrades, bson.D{{Key: "account_key", Value: 1}, {Key: "closed_at", Value: -1}}, false, false, 0},
		{ColPaperTrades, bson.D{{Key: "strategy_id", Value: 1}, {Key: "closed_at", Value: -1}}, false, false, 0},
		{ColPaperTrades, bson.D{{Key: "symbol", Value: 1}}, false, false, 0},
		{ColPaperTrades, bson.D{{Key: "pnl", Value: -1}}, false, false, 0},

		// paper_positions — upserted by position_id
		{ColPaperPositions, bson.D{{Key: "position_id", Value: 1}}, true, false, 0},
		{ColPaperPositions, bson.D{{Key: "account_key", Value: 1}, {Key: "status", Value: 1}}, false, false, 0},
		{ColPaperPositions, bson.D{{Key: "strategy_id", Value: 1}}, false, false, 0},
		{ColPaperPositions, bson.D{{Key: "opened_at", Value: -1}}, false, false, 0},

		// paper_orders — OMS lifecycle log (append-only, keyed by order_id + transition)
		{ColPaperOrders, bson.D{{Key: "order_id", Value: 1}, {Key: "transition_to", Value: 1}}, true, false, 0},
		{ColPaperOrders, bson.D{{Key: "account_key", Value: 1}, {Key: "recorded_at", Value: -1}}, false, false, 0},
		{ColPaperOrders, bson.D{{Key: "strategy_id", Value: 1}}, false, false, 0},

		// paper_state — singleton per account (upserted on account_key)
		{ColPaperState, bson.D{{Key: "account_key", Value: 1}}, true, false, 0},
		{ColPaperState, bson.D{{Key: "snapped_at", Value: -1}}, false, false, 0},

		// equity_curve — TTL 90 days
		{ColEquityCurve, bson.D{{Key: "account_key", Value: 1}, {Key: "ts", Value: -1}}, false, false, 0},
		{ColEquityCurve, bson.D{{Key: "ts", Value: 1}}, false, false, 90 * 24 * 3600},

		// daily_pnl_history — unique per account + date
		{ColDailyPnL, bson.D{{Key: "account_key", Value: 1}, {Key: "date", Value: 1}}, true, false, 0},
		{ColDailyPnL, bson.D{{Key: "date", Value: -1}}, false, false, 0},

		// strategy_scores — keyed on (account_key, strategy_id, source) so the Go engine
		// ("paperpersist-phase31a") and browser mock-trading ("browser") can coexist
		// without document overwrites.
		{ColStrategyScores, bson.D{{Key: "account_key", Value: 1}, {Key: "strategy_id", Value: 1}, {Key: "source", Value: 1}}, true, false, 0},
		{ColStrategyScores, bson.D{{Key: "updated_at", Value: -1}}, false, false, 0},

		// strategy_health — same compound key for the same collision-safe reason
		{ColStrategyHealth, bson.D{{Key: "account_key", Value: 1}, {Key: "strategy_id", Value: 1}, {Key: "source", Value: 1}}, true, false, 0},
		{ColStrategyHealth, bson.D{{Key: "health_status", Value: 1}}, false, false, 0},
		{ColStrategyHealth, bson.D{{Key: "updated_at", Value: -1}}, false, false, 0},

		// portfolio_metrics — singleton per account
		{ColPortfolioMetrics, bson.D{{Key: "account_key", Value: 1}}, true, false, 0},

		// risk_events — append-only
		{ColRiskEvents, bson.D{{Key: "event_id", Value: 1}}, true, false, 0},
		{ColRiskEvents, bson.D{{Key: "account_key", Value: 1}, {Key: "ts", Value: -1}}, false, false, 0},

		// worker_events — append-only, TTL 7 days
		{ColWorkerEvents, bson.D{{Key: "event_id", Value: 1}}, true, false, 0},
		{ColWorkerEvents, bson.D{{Key: "ts", Value: 1}}, false, false, 7 * 24 * 3600},

		// execution_logs — TTL 30 days
		{ColExecutionLogs, bson.D{{Key: "signal_id", Value: 1}}, false, true, 0},
		{ColExecutionLogs, bson.D{{Key: "ts", Value: 1}}, false, false, 30 * 24 * 3600},
	}

	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("paperpersist: not connected")
	}

	for _, s := range specs {
		idxOpts := options.Index().SetSparse(s.sparse)
		if s.unique {
			idxOpts.SetUnique(true)
		}
		if s.ttlSec > 0 {
			idxOpts.SetExpireAfterSeconds(s.ttlSec)
		}
		_, err := db.Collection(s.col).Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    s.keys,
			Options: idxOpts,
		})
		if err != nil {
			log.Printf("[paperpersist] index %s: %v", s.col, err)
		}
	}
	log.Printf("[paperpersist] indexes ensured for %d collections", len(specs))
	return nil
}

// RunPingMonitor starts a background goroutine that pings MongoDB every
// interval and reconnects on failure. It returns when ctx is done.
func (m *MongoManager) RunPingMonitor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			connected := m.connected
			closed := m.closed
			m.mu.RUnlock()

			if closed {
				return
			}

			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := m.mc.Ping(pingCtx, nil)
			cancel()

			if err != nil {
				if connected {
					log.Printf("[paperpersist] ping failed — attempting reconnect: %v", err)
					m.mu.Lock()
					m.connected = false
					m.mu.Unlock()
					M.RecordFailed()
				}
				reconnCtx, rcancel := context.WithTimeout(ctx, 15*time.Second)
				if rerr := m.connect(reconnCtx); rerr != nil {
					log.Printf("[paperpersist] reconnect failed: %v", rerr)
				}
				rcancel()
			}
		}
	}
}

// Shutdown gracefully disconnects MongoDB.
func (m *MongoManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	if m.mc != nil {
		return m.mc.Disconnect(ctx)
	}
	return nil
}

// Col returns a MongoDB collection handle.
func (m *MongoManager) Col(name string) *mongo.Collection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.db == nil {
		return nil
	}
	return m.db.Collection(name)
}

// IsConnected reports whether the MongoDB connection is healthy.
func (m *MongoManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// ── Diagnostics ───────────────────────────────────────────────────────────────

// DiagnosticsReport is returned by Diagnostics() for /api/paper-desk/diagnostics.
type DiagnosticsReport struct {
	Connected        bool              `json:"connected"`
	AccountKey       string            `json:"account_key"`
	DatabaseName     string            `json:"database_name"`
	URI              string            `json:"uri_masked"` // host only, no credentials
	CollectionStatus map[string]bool   `json:"collection_status"`
	MissingCollections []string        `json:"missing_collections"`
	Metrics          MetricsSnapshot   `json:"metrics"`
	PingLatencyMs    int64             `json:"ping_latency_ms"`
	CheckedAt        time.Time         `json:"checked_at"`
}

// MetricsSnapshot is a point-in-time read of the package-level Metrics counters.
type MetricsSnapshot struct {
	WriteCount       int64         `json:"write_count"`
	FailedWrites     int64         `json:"failed_writes"`
	RetryCount       int64         `json:"retry_count"`
	QueueDepth       int64         `json:"queue_depth"`
	AvgWriteLatency  string        `json:"avg_write_latency"`
	PersistenceLag   string        `json:"persistence_lag"`
}

// Diagnostics pings MongoDB, lists all required collections, and returns a
// structured health report. Safe to call from an HTTP handler.
func (m *MongoManager) Diagnostics(ctx context.Context) DiagnosticsReport {
	r := DiagnosticsReport{
		AccountKey:   AccountKey(),
		DatabaseName: m.dbName,
		URI:          maskURI(m.uri),
		CheckedAt:    time.Now().UTC(),
		CollectionStatus: make(map[string]bool),
	}

	// Ping
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	t0 := time.Now()
	pingErr := m.mc.Ping(pingCtx, nil)
	r.PingLatencyMs = time.Since(t0).Milliseconds()
	cancel()
	r.Connected = pingErr == nil

	if !r.Connected {
		return r
	}

	// Check all required collections exist
	required := []string{
		ColPaperTrades, ColPaperPositions, ColPaperOrders, ColPaperState,
		ColEquityCurve, ColDailyPnL, ColStrategyScores, ColStrategyHealth,
		ColPortfolioMetrics, ColRiskEvents, ColWorkerEvents, ColExecutionLogs,
	}

	listCtx, lcancel := context.WithTimeout(ctx, 10*time.Second)
	defer lcancel()

	m.mu.RLock()
	db := m.db
	m.mu.RUnlock()

	existing := make(map[string]bool)
	if db != nil {
		names, err := db.ListCollectionNames(listCtx, bson.M{})
		if err == nil {
			for _, n := range names {
				existing[n] = true
			}
		}
	}

	for _, col := range required {
		r.CollectionStatus[col] = existing[col]
		if !existing[col] {
			r.MissingCollections = append(r.MissingCollections, col)
		}
	}

	r.Metrics = MetricsSnapshot{
		WriteCount:      atomic.LoadInt64(&M.WriteCount),
		FailedWrites:    atomic.LoadInt64(&M.FailedWrites),
		RetryCount:      atomic.LoadInt64(&M.RetryCount),
		QueueDepth:      atomic.LoadInt64(&M.QueueDepth),
		AvgWriteLatency: M.AvgWriteLatency().String(),
		PersistenceLag:  time.Duration(atomic.LoadInt64(&M.PersistenceLagNs)).String(),
	}
	return r
}

// StartupReport logs a human-readable startup diagnostics block.
// Call once after NewMongoManager + EnsureIndexes to confirm production wiring.
func (m *MongoManager) StartupReport(ctx context.Context) {
	r := m.Diagnostics(ctx)
	log.Printf("[paperpersist/diag] ══════════════════════════════════════════")
	log.Printf("[paperpersist/diag] MongoDB startup diagnostics")
	log.Printf("[paperpersist/diag]   connected    = %v (ping %dms)", r.Connected, r.PingLatencyMs)
	log.Printf("[paperpersist/diag]   database     = %s", r.DatabaseName)
	log.Printf("[paperpersist/diag]   uri          = %s", r.URI)
	log.Printf("[paperpersist/diag]   account_key  = %s", r.AccountKey)
	if len(r.MissingCollections) == 0 {
		log.Printf("[paperpersist/diag]   collections  = ALL %d present ✅", len(r.CollectionStatus))
	} else {
		log.Printf("[paperpersist/diag]   collections  = %d missing: %v ⚠️",
			len(r.MissingCollections), r.MissingCollections)
	}
	log.Printf("[paperpersist/diag] ══════════════════════════════════════════")
}

// maskURI strips credentials from a MongoDB URI for safe logging.
func maskURI(uri string) string {
	// mongodb+srv://user:pass@host/... → mongodb+srv://<redacted>@host/...
	if i := strings.Index(uri, "@"); i > 0 {
		if j := strings.Index(uri, "://"); j >= 0 {
			return uri[:j+3] + "<redacted>@" + uri[i+1:]
		}
	}
	return uri
}

// ── Document helpers ──────────────────────────────────────────────────────────

// checksum returns a SHA-256 hex digest of the JSON serialisation of v.
func checksum(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// upsertOne performs an idempotent upsert using the provided filter.
func upsertOne(ctx context.Context, col *mongo.Collection, filter, doc bson.M) error {
	if col == nil {
		return fmt.Errorf("paperpersist: nil collection")
	}
	t0 := time.Now()
	update := bson.M{
		"$set":         doc,
		"$setOnInsert": bson.M{"created_at": t0},
	}
	_, err := col.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	M.RecordWrite(time.Since(t0))
	if err != nil {
		M.RecordFailed()
	}
	return err
}

// insertOne appends a document without upsert (append-only collections).
func insertOne(ctx context.Context, col *mongo.Collection, doc bson.M) error {
	if col == nil {
		return fmt.Errorf("paperpersist: nil collection")
	}
	t0 := time.Now()
	_, err := col.InsertOne(ctx, doc)
	M.RecordWrite(time.Since(t0))
	if err != nil {
		M.RecordFailed()
	}
	return err
}

// baseDoc returns the standard audit fields appended to every document.
// account_key is always sourced from the server constant — never client input.
func baseDoc(now time.Time) bson.M {
	return bson.M{
		"schema_version": schemaVersion,
		"account_key":    AccountKey(), // ← server constant, never client input
		"updated_at":     now,
		"source":         "paperpersist-phase31a",
	}
}
