package backtest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ResultsStore persists backtest results in a SQLite database.
// Each run is stored with a timestamp and full JSON blob, plus indexed
// scalar metrics for leaderboard queries.
type ResultsStore struct {
	db *sql.DB
}

// StoredResult is a single row from the results table.
type StoredResult struct {
	ID           int64
	RunID        string
	StrategyName string
	Symbol       string
	From         time.Time
	To           time.Time
	RanAt        time.Time
	TotalTrades  int
	WinRate      float64
	Sharpe       float64
	MaxDrawdown  float64
	ProfitFactor float64
	TotalReturn  float64
	ResultJSON   string
}

// OpenResultsStore opens (or creates) the SQLite database at the given path.
func OpenResultsStore(path string) (*ResultsStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("ResultsStore mkdir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=10000")
	if err != nil {
		return nil, fmt.Errorf("ResultsStore open: %w", err)
	}
	// Serialize all writes to avoid SQLITE_IOERR_LOCK under parallel goroutines.
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("ResultsStore migrate: %w", err)
	}
	return &ResultsStore{db: db}, nil
}

// Close closes the underlying database connection.
func (s *ResultsStore) Close() error { return s.db.Close() }

// SaveResult persists a StrategyResult. runID groups a batch of results.
func (s *ResultsStore) SaveResult(runID string, sr StrategyResult) error {
	blob, err := json.Marshal(sr.V3Result)
	if err != nil {
		blob = []byte("{}")
	}
	_, err = s.db.Exec(`
		INSERT INTO backtest_results
		  (run_id, strategy_name, symbol, from_ts, to_ts, ran_at,
		   total_trades, win_rate, sharpe, max_drawdown, profit_factor, total_return, result_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		runID, sr.StrategyName, sr.Symbol,
		sr.From.Unix(), sr.To.Unix(), time.Now().Unix(),
		sr.TotalTrades, sr.WinRate, sr.Sharpe, sr.MaxDrawdownPct,
		sr.ProfitFactor, sr.TotalReturnPct, string(blob),
	)
	return err
}

// SaveBatch persists a slice of results under a single runID.
func (s *ResultsStore) SaveBatch(runID string, results []StrategyResult) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	for _, sr := range results {
		blob, _ := json.Marshal(sr.V3Result)
		if _, err := tx.Exec(`
			INSERT INTO backtest_results
			  (run_id, strategy_name, symbol, from_ts, to_ts, ran_at,
			   total_trades, win_rate, sharpe, max_drawdown, profit_factor, total_return, result_json)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			runID, sr.StrategyName, sr.Symbol,
			sr.From.Unix(), sr.To.Unix(), time.Now().Unix(),
			sr.TotalTrades, sr.WinRate, sr.Sharpe, sr.MaxDrawdownPct,
			sr.ProfitFactor, sr.TotalReturnPct, string(blob),
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// Leaderboard returns up to limit results for a given symbol+runID,
// ordered by Sharpe descending.
func (s *ResultsStore) Leaderboard(runID, symbol string, limit int) ([]StoredResult, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, run_id, strategy_name, symbol, from_ts, to_ts, ran_at,
		       total_trades, win_rate, sharpe, max_drawdown, profit_factor, total_return, result_json
		FROM backtest_results
		WHERE run_id=? AND symbol=?
		ORDER BY sharpe DESC
		LIMIT ?`, runID, symbol, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// AllRuns returns a distinct list of run IDs ordered newest first.
func (s *ResultsStore) AllRuns() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT run_id FROM backtest_results ORDER BY ran_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetResult fetches a single stored result by strategy name and runID.
func (s *ResultsStore) GetResult(runID, strategyName string) (*StoredResult, error) {
	row := s.db.QueryRow(`
		SELECT id, run_id, strategy_name, symbol, from_ts, to_ts, ran_at,
		       total_trades, win_rate, sharpe, max_drawdown, profit_factor, total_return, result_json
		FROM backtest_results
		WHERE run_id=? AND strategy_name=?
		LIMIT 1`, runID, strategyName)
	results, err := scanRows(&singleRow{row})
	if err != nil || len(results) == 0 {
		return nil, err
	}
	return &results[0], nil
}

// DeleteRun removes all results for a given runID.
func (s *ResultsStore) DeleteRun(runID string) error {
	_, err := s.db.Exec(`DELETE FROM backtest_results WHERE run_id=?`, runID)
	return err
}

// ── internals ─────────────────────────────────────────────────────────────────

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS backtest_results (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id        TEXT    NOT NULL,
		strategy_name TEXT    NOT NULL,
		symbol        TEXT    NOT NULL,
		from_ts       INTEGER NOT NULL,
		to_ts         INTEGER NOT NULL,
		ran_at        INTEGER NOT NULL,
		total_trades  INTEGER NOT NULL DEFAULT 0,
		win_rate      REAL    NOT NULL DEFAULT 0,
		sharpe        REAL    NOT NULL DEFAULT 0,
		max_drawdown  REAL    NOT NULL DEFAULT 0,
		profit_factor REAL    NOT NULL DEFAULT 0,
		total_return  REAL    NOT NULL DEFAULT 0,
		result_json   TEXT    NOT NULL DEFAULT '{}'
	);
	CREATE INDEX IF NOT EXISTS idx_bt_run     ON backtest_results(run_id);
	CREATE INDEX IF NOT EXISTS idx_bt_symbol  ON backtest_results(symbol);
	CREATE INDEX IF NOT EXISTS idx_bt_sharpe  ON backtest_results(sharpe DESC);
	`)
	return err
}

type scanner interface {
	Scan(dest ...any) error
	Next() bool
	Err() error
}

// singleRow wraps *sql.Row to satisfy the scanner interface.
type singleRow struct{ r *sql.Row }

func (s *singleRow) Scan(dest ...any) error { return s.r.Scan(dest...) }
func (s *singleRow) Next() bool             { return true }
func (s *singleRow) Err() error             { return nil }

func scanRows(rows scanner) ([]StoredResult, error) {
	var out []StoredResult
	for rows.Next() {
		var sr StoredResult
		var fromUnix, toUnix, ranAtUnix int64
		if err := rows.Scan(
			&sr.ID, &sr.RunID, &sr.StrategyName, &sr.Symbol,
			&fromUnix, &toUnix, &ranAtUnix,
			&sr.TotalTrades, &sr.WinRate, &sr.Sharpe,
			&sr.MaxDrawdown, &sr.ProfitFactor, &sr.TotalReturn,
			&sr.ResultJSON,
		); err != nil {
			return nil, err
		}
		sr.From = time.Unix(fromUnix, 0).UTC()
		sr.To = time.Unix(toUnix, 0).UTC()
		sr.RanAt = time.Unix(ranAtUnix, 0).UTC()
		out = append(out, sr)
	}
	return out, rows.Err()
}
