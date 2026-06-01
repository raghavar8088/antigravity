package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSnapshotStore is a durable implementation of SnapshotStore backed by
// PostgreSQL. Snapshots are persisted as JSONB rows keyed by accountID.
//
// Multiple snapshots per account are retained (newest-first via ORDER BY id DESC).
// Latest() returns the most recently written snapshot.
type PostgresSnapshotStore struct {
	pool *pgxpool.Pool
}

// NewPostgresSnapshotStore creates a snapshot store using the given pgxpool.
func NewPostgresSnapshotStore(pool *pgxpool.Pool) *PostgresSnapshotStore {
	return &PostgresSnapshotStore{pool: pool}
}

// CreateSchema creates the ledger_snapshots table if it does not exist.
// Safe to call multiple times (idempotent).
func (s *PostgresSnapshotStore) CreateSchema(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS ledger_snapshots (
    id              BIGSERIAL    PRIMARY KEY,
    snapshot_id     TEXT         NOT NULL,
    account_id      TEXT         NOT NULL,
    global_sequence BIGINT       NOT NULL,
    state_json      JSONB        NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_snapshots_account_created
    ON ledger_snapshots (account_id, created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_snapshots_id
    ON ledger_snapshots (snapshot_id);
`
	if _, err := s.pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("ledger.PostgresSnapshotStore.CreateSchema: %w", err)
	}
	log.Println("[LEDGER] ✅ Snapshot schema ready (ledger_snapshots)")
	return nil
}

// Save persists a snapshot. Inserts a new row — does NOT overwrite; all snapshots
// are retained so restores can choose any point in time.
func (s *PostgresSnapshotStore) Save(ctx context.Context, snap Snapshot) error {
	if snap.AccountID == "" {
		return fmt.Errorf("ledger.PostgresSnapshotStore.Save: accountID is required")
	}
	if snap.SnapshotID == "" {
		id, err := randomID()
		if err != nil {
			return err
		}
		snap.SnapshotID = id
	}
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now().UTC()
	}

	stateJSON, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("ledger.PostgresSnapshotStore.Save: marshal: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO ledger_snapshots (snapshot_id, account_id, global_sequence, state_json, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (snapshot_id) DO NOTHING
	`, snap.SnapshotID, snap.AccountID, snap.GlobalSequence, stateJSON, snap.CreatedAt)
	if err != nil {
		return fmt.Errorf("ledger.PostgresSnapshotStore.Save: %w", err)
	}
	return nil
}

// Latest returns the most recent snapshot for the given accountID.
// Returns ErrNoSnapshot when no snapshot exists.
func (s *PostgresSnapshotStore) Latest(ctx context.Context, accountID string) (Snapshot, error) {
	var stateJSON []byte
	err := s.pool.QueryRow(ctx, `
		SELECT state_json
		FROM ledger_snapshots
		WHERE account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, accountID).Scan(&stateJSON)
	if err != nil {
		if isNoRows(err) {
			return Snapshot{}, fmt.Errorf("%w for account %s", ErrNoSnapshot, accountID)
		}
		return Snapshot{}, fmt.Errorf("ledger.PostgresSnapshotStore.Latest: %w", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(stateJSON, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("ledger.PostgresSnapshotStore.Latest: unmarshal: %w", err)
	}
	return snap, nil
}

// All returns all snapshots for the given accountID in ascending creation order.
func (s *PostgresSnapshotStore) All(ctx context.Context, accountID string) ([]Snapshot, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT state_json
		FROM ledger_snapshots
		WHERE account_id = $1
		ORDER BY created_at ASC, id ASC
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("ledger.PostgresSnapshotStore.All: %w", err)
	}
	defer rows.Close()

	var snaps []Snapshot
	for rows.Next() {
		var stateJSON []byte
		if err := rows.Scan(&stateJSON); err != nil {
			return nil, fmt.Errorf("ledger.PostgresSnapshotStore.All: scan: %w", err)
		}
		var snap Snapshot
		if err := json.Unmarshal(stateJSON, &snap); err != nil {
			return nil, fmt.Errorf("ledger.PostgresSnapshotStore.All: unmarshal: %w", err)
		}
		snaps = append(snaps, snap)
	}
	return snaps, rows.Err()
}
