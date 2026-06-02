package ha

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/ledger"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	replicationPollInterval = 500 * time.Millisecond
	replicationBatchSize    = 500
	replicationLagWarnSecs  = 5.0
)

// LedgerReplicator continuously streams events from the primary ledger store
// to a replica store. It uses PostgreSQL logical replication semantics:
// events are numbered by sequence; the replicator tracks the last-seen
// sequence and fetches any gap on each poll.
//
// On a standby node this keeps the shadow ledger current so failover
// requires no replay from sequence 0.
type LedgerReplicator struct {
	nodeID  string
	primary *pgxpool.Pool // read source (primary DB)
	replica *pgxpool.Pool // write target (replica DB or same DB for same-region HA)
	localSt ledger.Store  // in-process store to keep warm

	lastSeq atomic.Int64
	lagSecs atomic.Value // stores float64

	mu      sync.Mutex
	running bool

	metricEventsReplicated prometheus.Counter
	metricLagSeconds       prometheus.Gauge
	metricErrors           prometheus.Counter
	metricBehindPrimary    prometheus.Gauge
}

func NewLedgerReplicator(
	nodeID string,
	primary, replica *pgxpool.Pool,
	localStore ledger.Store,
	reg prometheus.Registerer,
) *LedgerReplicator {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	r := &LedgerReplicator{
		nodeID:  nodeID,
		primary: primary,
		replica: replica,
		localSt: localStore,
	}
	r.lagSecs.Store(float64(0))
	f := promauto.With(reg)
	labels := prometheus.Labels{"node_id": nodeID}
	r.metricEventsReplicated = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_ledger_events_replicated_total", Help: "Events replicated to standby",
		ConstLabels: labels,
	})
	r.metricLagSeconds = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_ledger_replication_lag_seconds", Help: "Replication lag",
		ConstLabels: labels,
	})
	r.metricErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_ledger_replication_errors_total", Help: "Replication errors",
		ConstLabels: labels,
	})
	r.metricBehindPrimary = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_ledger_behind_primary_events", Help: "Events behind primary",
		ConstLabels: labels,
	})
	return r
}

// Run starts the continuous replication loop.
func (r *LedgerReplicator) Run(ctx context.Context) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("ledger replicator already running")
	}
	r.running = true
	r.mu.Unlock()

	// Recover our last-replicated sequence from the replica DB.
	seq, err := r.loadCheckpoint(ctx)
	if err != nil {
		log.Printf("[ha/ledger-rep] checkpoint load error (starting from 0): %v", err)
		seq = 0
	}
	r.lastSeq.Store(seq)
	log.Printf("[ha/ledger-rep] starting replication from seq=%d node=%s", seq, r.nodeID)

	ticker := time.NewTicker(replicationPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.replicate(ctx); err != nil {
				r.metricErrors.Inc()
				log.Printf("[ha/ledger-rep] replication error: %v", err)
			}
		}
	}
}

// replicate fetches events after lastSeq from primary and applies them.
func (r *LedgerReplicator) replicate(ctx context.Context) error {
	fromSeq := r.lastSeq.Load()

	// Fetch a batch of events from the primary ledger_events table.
	rows, err := r.primary.Query(ctx, `
		SELECT sequence_no, event_id, aggregate_type, aggregate_id,
		       event_type, payload, payload_hash, idempotency_key,
		       occurred_at, created_at
		FROM ledger_events
		WHERE sequence_no > $1
		ORDER BY sequence_no ASC
		LIMIT $2
	`, fromSeq, replicationBatchSize)
	if err != nil {
		return fmt.Errorf("fetch events from primary: %w", err)
	}
	defer rows.Close()

	type rawEvent struct {
		SeqNo          int64
		EventID        string
		AggregateType  string
		AggregateID    string
		EventType      string
		Payload        []byte
		PayloadHash    string
		IdempotencyKey string
		OccurredAt     time.Time
		CreatedAt      time.Time
	}

	var batch []rawEvent
	for rows.Next() {
		var e rawEvent
		if err := rows.Scan(
			&e.SeqNo, &e.EventID, &e.AggregateType, &e.AggregateID,
			&e.EventType, &e.Payload, &e.PayloadHash, &e.IdempotencyKey,
			&e.OccurredAt, &e.CreatedAt,
		); err != nil {
			return fmt.Errorf("scan event row: %w", err)
		}
		batch = append(batch, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if len(batch) == 0 {
		r.metricLagSeconds.Set(0)
		r.metricBehindPrimary.Set(0)
		return nil
	}

	// Measure primary head to calculate lag.
	primaryHead, err := r.primaryHead(ctx)
	if err == nil {
		behind := primaryHead - batch[len(batch)-1].SeqNo
		r.metricBehindPrimary.Set(float64(behind))
	}

	// Write batch to replica DB using an upsert to be idempotent.
	tx, err := r.replica.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin replica tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, e := range batch {
		_, err := tx.Exec(ctx, `
			INSERT INTO ledger_events (
				sequence_no, event_id, aggregate_type, aggregate_id,
				event_type, payload, payload_hash, idempotency_key,
				occurred_at, created_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (event_id) DO NOTHING
		`, e.SeqNo, e.EventID, e.AggregateType, e.AggregateID,
			e.EventType, e.Payload, e.PayloadHash, e.IdempotencyKey,
			e.OccurredAt, e.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert replicated event seq=%d: %w", e.SeqNo, err)
		}
	}

	maxSeq := batch[len(batch)-1].SeqNo

	// Update replication checkpoint atomically.
	_, err = tx.Exec(ctx, `
		INSERT INTO ha_replication_checkpoint (node_id, last_seq, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (node_id) DO UPDATE SET
			last_seq   = EXCLUDED.last_seq,
			updated_at = EXCLUDED.updated_at
	`, r.nodeID, maxSeq)
	if err != nil {
		return fmt.Errorf("update checkpoint: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit replica tx: %w", err)
	}

	r.lastSeq.Store(maxSeq)
	r.metricEventsReplicated.Add(float64(len(batch)))

	// Measure lag by comparing last event timestamp to now.
	lag := time.Since(batch[len(batch)-1].CreatedAt).Seconds()
	r.lagSecs.Store(lag)
	r.metricLagSeconds.Set(lag)

	if lag > replicationLagWarnSecs {
		log.Printf("[ha/ledger-rep] WARNING: replication lag=%.1fs behind events=%d", lag, len(batch))
	}

	return nil
}

func (r *LedgerReplicator) primaryHead(ctx context.Context) (int64, error) {
	var seq int64
	err := r.primary.QueryRow(ctx, `SELECT COALESCE(MAX(sequence_no), 0) FROM ledger_events`).Scan(&seq)
	return seq, err
}

func (r *LedgerReplicator) loadCheckpoint(ctx context.Context) (int64, error) {
	// Ensure the checkpoint table exists.
	_, _ = r.replica.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS ha_replication_checkpoint (
			node_id    TEXT PRIMARY KEY,
			last_seq   BIGINT NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	var seq int64
	err := r.replica.QueryRow(ctx,
		`SELECT last_seq FROM ha_replication_checkpoint WHERE node_id = $1`,
		r.nodeID,
	).Scan(&seq)
	if err != nil {
		return 0, err // table empty or not found — start from 0
	}
	return seq, nil
}

// LagSeconds returns the current replication lag in seconds.
func (r *LedgerReplicator) LagSeconds() float64 {
	v, _ := r.lagSecs.Load().(float64)
	return v
}

// LastSequence returns the last replicated sequence number.
func (r *LedgerReplicator) LastSequence() int64 {
	return r.lastSeq.Load()
}

// EnsureSchema creates the ledger_events table on the replica if it does not
// already exist. Idempotent — safe to call on every startup.
func (r *LedgerReplicator) EnsureSchema(ctx context.Context) error {
	_, err := r.replica.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS ledger_events (
			sequence_no     BIGSERIAL PRIMARY KEY,
			event_id        TEXT NOT NULL UNIQUE,
			aggregate_type  TEXT NOT NULL,
			aggregate_id    TEXT NOT NULL,
			event_type      TEXT NOT NULL,
			payload         BYTEA NOT NULL,
			payload_hash    TEXT NOT NULL,
			idempotency_key TEXT,
			occurred_at     TIMESTAMPTZ NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_ledger_events_aggregate
			ON ledger_events (aggregate_type, aggregate_id, sequence_no);
	`)
	return err
}
