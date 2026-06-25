package backtest

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// JobStatus describes the state of a backtest job.
type JobStatus string

const (
	JobStatusPending JobStatus = "pending"
	JobStatusRunning JobStatus = "running"
	JobStatusDone    JobStatus = "done"
	JobStatusError   JobStatus = "error"
)

// Job represents a single backtest run job.
type Job struct {
	ID         string    `json:"id"`
	RunID      string    `json:"runId"`
	Symbol     string    `json:"symbol"`
	FromDate   time.Time `json:"fromDate"`
	ToDate     time.Time `json:"toDate"`
	Strategies []string  `json:"strategies"`
	Status     JobStatus `json:"status"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	Progress   int       `json:"progress"`
}

// JobStore is a SQLite-backed store for backtest jobs.
// Falls back to in-memory if db is nil (e.g. store not yet open).
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job // in-memory cache
	db   *sql.DB
	seq  int
}

// NewJobStore creates an empty in-memory JobStore (no persistence).
func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*Job)}
}

// NewJobStoreWithDB creates a JobStore backed by the given SQLite db.
// It replays any existing jobs from the db into memory on startup.
func NewJobStoreWithDB(db *sql.DB) (*JobStore, error) {
	js := &JobStore{jobs: make(map[string]*Job), db: db}
	if err := migrateJobs(db); err != nil {
		return nil, err
	}
	if err := js.loadFromDB(); err != nil {
		return nil, err
	}
	return js, nil
}

// Create adds a new job and returns it.
func (s *JobStore) Create(symbol string, from, to time.Time, strategies []string) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	j := &Job{
		ID:         fmt.Sprintf("job_%d_%d", time.Now().UnixMilli(), s.seq),
		RunID:      fmt.Sprintf("run_%d", time.Now().UnixMilli()),
		Symbol:     symbol,
		FromDate:   from,
		ToDate:     to,
		Strategies: strategies,
		Status:     JobStatusPending,
		CreatedAt:  time.Now().UTC(),
	}
	s.jobs[j.ID] = j
	s.persistJob(j)
	return j
}

// Get returns a job by ID.
func (s *JobStore) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

// List returns all jobs, newest first.
func (s *JobStore) List() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	for i := 0; i < len(out)-1; i++ {
		for k := i + 1; k < len(out); k++ {
			if out[k].CreatedAt.After(out[i].CreatedAt) {
				out[i], out[k] = out[k], out[i]
			}
		}
	}
	return out
}

// UpdateStatus atomically updates job status.
func (s *JobStore) UpdateStatus(id string, status JobStatus, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.Error = errMsg
		if status == JobStatusDone || status == JobStatusError {
			j.FinishedAt = time.Now().UTC()
			j.Progress = 100
		}
		s.persistJob(j)
	}
}

// UpdateProgress sets the progress percentage (0-100).
func (s *JobStore) UpdateProgress(id string, pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Progress = pct
		s.persistJob(j)
	}
}

// ── SQLite persistence ────────────────────────────────────────────────────────

func migrateJobs(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS backtest_jobs (
		id          TEXT PRIMARY KEY,
		run_id      TEXT NOT NULL,
		symbol      TEXT NOT NULL,
		from_ts     INTEGER NOT NULL,
		to_ts       INTEGER NOT NULL,
		status      TEXT NOT NULL DEFAULT 'pending',
		error_msg   TEXT NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL,
		finished_at INTEGER NOT NULL DEFAULT 0,
		progress    INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_jobs_created ON backtest_jobs(created_at DESC);
	`)
	return err
}

func (s *JobStore) persistJob(j *Job) {
	if s.db == nil {
		return
	}
	finishedAt := int64(0)
	if !j.FinishedAt.IsZero() {
		finishedAt = j.FinishedAt.Unix()
	}
	s.db.Exec(`
		INSERT INTO backtest_jobs (id, run_id, symbol, from_ts, to_ts, status, error_msg, created_at, finished_at, progress)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			status=excluded.status, error_msg=excluded.error_msg,
			finished_at=excluded.finished_at, progress=excluded.progress`,
		j.ID, j.RunID, j.Symbol,
		j.FromDate.Unix(), j.ToDate.Unix(),
		string(j.Status), j.Error,
		j.CreatedAt.Unix(), finishedAt, j.Progress,
	)
}

func (s *JobStore) loadFromDB() error {
	rows, err := s.db.Query(`
		SELECT id, run_id, symbol, from_ts, to_ts, status, error_msg, created_at, finished_at, progress
		FROM backtest_jobs ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var j Job
		var fromTs, toTs, createdAt, finishedAt int64
		var status, errMsg string
		if err := rows.Scan(&j.ID, &j.RunID, &j.Symbol, &fromTs, &toTs,
			&status, &errMsg, &createdAt, &finishedAt, &j.Progress); err != nil {
			return err
		}
		j.FromDate = time.Unix(fromTs, 0).UTC()
		j.ToDate = time.Unix(toTs, 0).UTC()
		j.CreatedAt = time.Unix(createdAt, 0).UTC()
		if finishedAt > 0 {
			j.FinishedAt = time.Unix(finishedAt, 0).UTC()
		}
		j.Status = JobStatus(status)
		j.Error = errMsg
		// Mark any job that was running at restart as error
		if j.Status == JobStatusRunning || j.Status == JobStatusPending {
			j.Status = JobStatusError
			j.Error = "engine restarted mid-job — please re-run"
		}
		s.jobs[j.ID] = &j
	}
	return rows.Err()
}
