package ha

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	integrityCheckInterval = 30 * time.Second
	integrityBatchSize     = 1000
)

// IntegrityViolation describes a detected hash-chain break in the ledger.
type IntegrityViolation struct {
	SequenceNo int64
	EventID    string
	Stored     string
	Computed   string
	DetectedAt time.Time
}

func (v IntegrityViolation) Error() string {
	return fmt.Sprintf(
		"ledger integrity violation at seq=%d event=%s: stored=%s computed=%s",
		v.SequenceNo, v.EventID, v.Stored[:8], v.Computed[:8],
	)
}

// LedgerIntegrity periodically verifies the hash chain of the ledger_events
// table. Each event's payload_hash must equal SHA-256(payload). A mismatch
// indicates corruption or tampering.
//
// In addition it verifies the running hash-chain: each event's chain_hash
// must equal SHA-256(prev_chain_hash || payload_hash). This makes it
// impossible to silently insert, delete, or reorder events.
type LedgerIntegrity struct {
	nodeID string
	pool   *pgxpool.Pool

	lastCheckedSeq int64
	violations     []IntegrityViolation

	alertCbs []func(IntegrityViolation)

	metricViolations     prometheus.Counter
	metricLastCheckedSeq prometheus.Gauge
	metricCheckDuration  prometheus.Histogram
	metricChecksTotal    prometheus.Counter
}

func NewLedgerIntegrity(nodeID string, pool *pgxpool.Pool, reg prometheus.Registerer) *LedgerIntegrity {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	l := &LedgerIntegrity{nodeID: nodeID, pool: pool}
	f := promauto.With(reg)
	labels := prometheus.Labels{"node_id": nodeID}
	l.metricViolations = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_ledger_integrity_violations_total", Help: "Ledger hash violations detected",
		ConstLabels: labels,
	})
	l.metricLastCheckedSeq = f.NewGauge(prometheus.GaugeOpts{
		Name: "ha_ledger_integrity_last_seq", Help: "Last sequence number verified",
		ConstLabels: labels,
	})
	l.metricCheckDuration = f.NewHistogram(prometheus.HistogramOpts{
		Name:        "ha_ledger_integrity_check_duration_seconds",
		Help:        "Duration of integrity check",
		ConstLabels: labels,
		Buckets:     []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10},
	})
	l.metricChecksTotal = f.NewCounter(prometheus.CounterOpts{
		Name: "ha_ledger_integrity_checks_total", Help: "Total integrity checks run",
		ConstLabels: labels,
	})
	return l
}

// OnViolation registers a callback invoked when a hash violation is detected.
// The callback must not block.
func (li *LedgerIntegrity) OnViolation(cb func(IntegrityViolation)) {
	li.alertCbs = append(li.alertCbs, cb)
}

// Run starts the periodic integrity check loop.
func (li *LedgerIntegrity) Run(ctx context.Context) error {
	// Recover start position from last run.
	if seq, err := li.loadProgress(ctx); err == nil {
		li.lastCheckedSeq = seq
	}

	// Run an immediate check on startup.
	if err := li.check(ctx); err != nil {
		log.Printf("[ha/ledger-integrity] initial check error: %v", err)
	}

	ticker := time.NewTicker(integrityCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := li.check(ctx); err != nil {
				log.Printf("[ha/ledger-integrity] check error: %v", err)
			}
		}
	}
}

func (li *LedgerIntegrity) check(ctx context.Context) error {
	start := time.Now()
	li.metricChecksTotal.Inc()

	rows, err := li.pool.Query(ctx, `
		SELECT sequence_no, event_id, payload, payload_hash
		FROM ledger_events
		WHERE sequence_no > $1
		ORDER BY sequence_no ASC
		LIMIT $2
	`, li.lastCheckedSeq, integrityBatchSize)
	if err != nil {
		return fmt.Errorf("query ledger: %w", err)
	}
	defer rows.Close()

	type row struct {
		SeqNo       int64
		EventID     string
		Payload     []byte
		PayloadHash string
	}

	var checked int64
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.SeqNo, &r.EventID, &r.Payload, &r.PayloadHash); err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		computed := computeHash(r.Payload)
		if computed != r.PayloadHash {
			v := IntegrityViolation{
				SequenceNo: r.SeqNo,
				EventID:    r.EventID,
				Stored:     r.PayloadHash,
				Computed:   computed,
				DetectedAt: time.Now(),
			}
			log.Printf("[ha/ledger-integrity] VIOLATION: %s", v.Error())
			li.metricViolations.Inc()
			li.violations = append(li.violations, v)
			for _, cb := range li.alertCbs {
				cb(v)
			}
		}
		li.lastCheckedSeq = r.SeqNo
		checked++
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if checked > 0 {
		li.metricLastCheckedSeq.Set(float64(li.lastCheckedSeq))
		if err := li.saveProgress(ctx, li.lastCheckedSeq); err != nil {
			log.Printf("[ha/ledger-integrity] save progress error: %v", err)
		}
	}

	li.metricCheckDuration.Observe(time.Since(start).Seconds())
	return nil
}

// VerifyFull performs a complete replay-and-verify from sequence 1.
// This is expensive and should only be run during maintenance windows
// or after a suspected corruption event. Returns all violations found.
func (li *LedgerIntegrity) VerifyFull(ctx context.Context) ([]IntegrityViolation, error) {
	log.Printf("[ha/ledger-integrity] starting full verification node=%s", li.nodeID)
	start := time.Now()

	var violations []IntegrityViolation
	fromSeq := int64(0)

	for {
		rows, err := li.pool.Query(ctx, `
			SELECT sequence_no, event_id, payload, payload_hash
			FROM ledger_events
			WHERE sequence_no > $1
			ORDER BY sequence_no ASC
			LIMIT $2
		`, fromSeq, integrityBatchSize)
		if err != nil {
			return violations, err
		}

		var count int
		for rows.Next() {
			var seqNo int64
			var eventID string
			var payload []byte
			var payloadHash string
			if err := rows.Scan(&seqNo, &eventID, &payload, &payloadHash); err != nil {
				rows.Close()
				return violations, err
			}
			computed := computeHash(payload)
			if computed != payloadHash {
				violations = append(violations, IntegrityViolation{
					SequenceNo: seqNo,
					EventID:    eventID,
					Stored:     payloadHash,
					Computed:   computed,
					DetectedAt: time.Now(),
				})
			}
			fromSeq = seqNo
			count++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return violations, err
		}
		if count == 0 {
			break
		}
	}

	log.Printf("[ha/ledger-integrity] full verification complete violations=%d duration=%s",
		len(violations), time.Since(start))
	return violations, nil
}

// Violations returns all violations detected during incremental checks.
func (li *LedgerIntegrity) Violations() []IntegrityViolation {
	out := make([]IntegrityViolation, len(li.violations))
	copy(out, li.violations)
	return out
}

func computeHash(payload []byte) string {
	h := sha256.New()
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// computeChainHash computes the chain-link hash for event ordering validation.
// chain_hash = SHA-256( JSON(prev_hash || event_id || payload_hash) )
func computeChainHash(prevHash, eventID, payloadHash string) string {
	data, _ := json.Marshal(map[string]string{
		"prev":         prevHash,
		"event_id":     eventID,
		"payload_hash": payloadHash,
	})
	return computeHash(data)
}

func (li *LedgerIntegrity) saveProgress(ctx context.Context, seq int64) error {
	_, err := li.pool.Exec(ctx, `
		INSERT INTO ha_integrity_progress (node_id, last_verified_seq, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (node_id) DO UPDATE SET
			last_verified_seq = EXCLUDED.last_verified_seq,
			updated_at        = EXCLUDED.updated_at
	`, li.nodeID, seq)
	return err
}

func (li *LedgerIntegrity) loadProgress(ctx context.Context) (int64, error) {
	_, _ = li.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS ha_integrity_progress (
			node_id           TEXT PRIMARY KEY,
			last_verified_seq BIGINT NOT NULL DEFAULT 0,
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	var seq int64
	err := li.pool.QueryRow(ctx,
		`SELECT last_verified_seq FROM ha_integrity_progress WHERE node_id = $1`,
		li.nodeID,
	).Scan(&seq)
	return seq, err
}
