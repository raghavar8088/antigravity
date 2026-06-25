package backtest

import (
	"fmt"
	"sync"
	"time"
)

// JobStatus describes the state of a backtest job.
type JobStatus string

const (
	JobStatusPending  JobStatus = "pending"
	JobStatusRunning  JobStatus = "running"
	JobStatusDone     JobStatus = "done"
	JobStatusError    JobStatus = "error"
)

// Job represents a single backtest run job.
type Job struct {
	ID         string
	RunID      string
	Symbol     string
	FromDate   time.Time
	ToDate     time.Time
	Strategies []string // empty = all ported strategies
	Status     JobStatus
	Error      string
	CreatedAt  time.Time
	FinishedAt time.Time
	Progress   int // 0–100
}

// JobStore is an in-memory store for backtest jobs.
// Thread-safe for concurrent access from HTTP handlers and background workers.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]*Job
	seq  int
}

// NewJobStore creates an empty JobStore.
func NewJobStore() *JobStore {
	return &JobStore{jobs: make(map[string]*Job)}
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
	// sort newest first by CreatedAt
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
	}
}

// UpdateProgress sets the progress percentage (0-100).
func (s *JobStore) UpdateProgress(id string, pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Progress = pct
	}
}
