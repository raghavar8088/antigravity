// Package experiments implements the Phase 19G Experiment Tracking System.
// Every experiment is uniquely identified, versioned, and reproducible.
// Tracks parameters, datasets, metrics, and produces a full audit trail.
package experiments

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ─── Experiment States ────────────────────────────────────────────────────────

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusRunning   Status = "RUNNING"
	StatusCompleted Status = "COMPLETED"
	StatusFailed    Status = "FAILED"
	StatusCancelled Status = "CANCELLED"
)

// ─── Core Types ───────────────────────────────────────────────────────────────

// Metrics is the full set of financial performance metrics for an experiment.
type Metrics struct {
	SharpeRatio  float64 `json:"sharpe_ratio"`
	SortinoRatio float64 `json:"sortino_ratio"`
	MaxDrawdown  float64 `json:"max_drawdown_pct"`
	WinRate      float64 `json:"win_rate"`
	ProfitFactor float64 `json:"profit_factor"`
	Alpha        float64 `json:"alpha"`
	Beta         float64 `json:"beta"`
	TotalPnLUSD  float64 `json:"total_pnl_usd"`
	TotalTrades  int     `json:"total_trades"`
	AvgTradeUSD  float64 `json:"avg_trade_usd"`
	CAGR         float64 `json:"cagr"`
	Calmar       float64 `json:"calmar"`
	R2           float64 `json:"r2"`     // for ML models
	ICIR         float64 `json:"icir"`   // IC information ratio
}

// Experiment is a single research experiment with full reproducibility metadata.
type Experiment struct {
	ID            string
	Name          string
	Description   string
	StrategyID    string
	ModelID       string
	DatasetID     string
	Parameters    map[string]any
	Tags          []string
	ResearcherID  string
	ParentID      string // parent experiment for comparison

	Status       Status
	Metrics      Metrics
	TrainMetrics Metrics
	ValMetrics   Metrics

	// Logs is an ordered list of metric observations over training/evaluation.
	Logs         []MetricLog

	StartedAt    time.Time
	CompletedAt  time.Time
	DurationMs   int64
	ErrorMessage string

	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MetricLog captures a single metric observation at a step.
type MetricLog struct {
	Step      int
	Metric    string
	Value     float64
	LoggedAt  time.Time
}

// Comparison holds a side-by-side comparison of two experiments.
type Comparison struct {
	BaseID      string
	CompareID   string
	BaseMetrics Metrics
	CmpMetrics  Metrics
	Deltas      map[string]float64 // metric name → (cmp - base) / |base|
	Winner      string             // "base", "compare", or "tie"
}

// ─── Tracker ─────────────────────────────────────────────────────────────────

// Tracker is the experiment registry and audit log.
type Tracker struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	byStrategy  map[string][]string // strategyID → []experimentID
	byDataset   map[string][]string // datasetID → []experimentID
}

// NewTracker creates an empty experiment tracker.
func NewTracker() *Tracker {
	return &Tracker{
		experiments: make(map[string]*Experiment),
		byStrategy:  make(map[string][]string),
		byDataset:   make(map[string][]string),
	}
}

// Create registers a new experiment and returns its ID.
func (t *Tracker) Create(ctx context.Context, exp Experiment) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if exp.Name == "" {
		return "", errors.New("experiments: experiment name required")
	}
	if exp.ID == "" {
		exp.ID = fmt.Sprintf("exp_%d", time.Now().UnixNano())
	}
	exp.Status = StatusPending
	exp.CreatedAt = time.Now().UTC()
	exp.UpdatedAt = exp.CreatedAt

	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.experiments[exp.ID]; exists {
		return "", fmt.Errorf("experiments: id %s already exists", exp.ID)
	}
	t.experiments[exp.ID] = &exp
	t.byStrategy[exp.StrategyID] = append(t.byStrategy[exp.StrategyID], exp.ID)
	t.byDataset[exp.DatasetID] = append(t.byDataset[exp.DatasetID], exp.ID)
	return exp.ID, nil
}

// Start marks an experiment as RUNNING.
func (t *Tracker) Start(ctx context.Context, id string) error {
	return t.update(ctx, id, func(e *Experiment) error {
		if e.Status != StatusPending {
			return fmt.Errorf("experiments: cannot start %s in status %s", id, e.Status)
		}
		e.Status = StatusRunning
		e.StartedAt = time.Now().UTC()
		return nil
	})
}

// LogMetric appends a metric observation to the experiment log.
func (t *Tracker) LogMetric(ctx context.Context, id, metric string, value float64, step int) error {
	return t.update(ctx, id, func(e *Experiment) error {
		e.Logs = append(e.Logs, MetricLog{
			Step: step, Metric: metric, Value: value, LoggedAt: time.Now().UTC(),
		})
		return nil
	})
}

// Complete marks an experiment as COMPLETED with final metrics.
func (t *Tracker) Complete(ctx context.Context, id string, m Metrics) error {
	return t.update(ctx, id, func(e *Experiment) error {
		if e.Status != StatusRunning {
			return fmt.Errorf("experiments: cannot complete %s in status %s", id, e.Status)
		}
		e.Status = StatusCompleted
		e.Metrics = m
		e.CompletedAt = time.Now().UTC()
		e.DurationMs = e.CompletedAt.Sub(e.StartedAt).Milliseconds()
		return nil
	})
}

// Fail marks an experiment as FAILED.
func (t *Tracker) Fail(ctx context.Context, id, errMsg string) error {
	return t.update(ctx, id, func(e *Experiment) error {
		e.Status = StatusFailed
		e.ErrorMessage = errMsg
		e.CompletedAt = time.Now().UTC()
		return nil
	})
}

// Get retrieves an experiment by ID.
func (t *Tracker) Get(id string) (Experiment, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	e, ok := t.experiments[id]
	if !ok {
		return Experiment{}, fmt.Errorf("experiments: not found: %s", id)
	}
	return *e, nil
}

// ListByStrategy returns all experiments for a given strategy, newest first.
func (t *Tracker) ListByStrategy(strategyID string) []Experiment {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ids := t.byStrategy[strategyID]
	exps := make([]Experiment, 0, len(ids))
	for _, id := range ids {
		if e, ok := t.experiments[id]; ok {
			exps = append(exps, *e)
		}
	}
	sort.Slice(exps, func(i, j int) bool {
		return exps[i].CreatedAt.After(exps[j].CreatedAt)
	})
	return exps
}

// BestByMetric returns the experiment with the highest value for the given metric.
func (t *Tracker) BestByMetric(strategyID, metric string) (Experiment, error) {
	exps := t.ListByStrategy(strategyID)
	best := -math.MaxFloat64
	var bestExp *Experiment
	for i := range exps {
		if exps[i].Status != StatusCompleted {
			continue
		}
		v := metricValue(exps[i].Metrics, metric)
		if v > best {
			best = v
			bestExp = &exps[i]
		}
	}
	if bestExp == nil {
		return Experiment{}, fmt.Errorf("experiments: no completed experiments for strategy %s", strategyID)
	}
	return *bestExp, nil
}

// Compare produces a side-by-side metric comparison between two experiments.
func (t *Tracker) Compare(baseID, compareID string) (Comparison, error) {
	base, err := t.Get(baseID)
	if err != nil {
		return Comparison{}, err
	}
	cmp, err := t.Get(compareID)
	if err != nil {
		return Comparison{}, err
	}

	deltas := make(map[string]float64)
	metricNames := []string{
		"sharpe", "sortino", "max_drawdown", "win_rate",
		"profit_factor", "alpha", "beta", "total_pnl",
	}
	for _, name := range metricNames {
		bv := metricValue(base.Metrics, name)
		cv := metricValue(cmp.Metrics, name)
		if bv != 0 {
			deltas[name] = (cv - bv) / math.Abs(bv)
		} else if cv != 0 {
			deltas[name] = 1.0
		}
	}

	winner := "tie"
	if deltas["sharpe"] > 0.05 {
		winner = "compare"
	} else if deltas["sharpe"] < -0.05 {
		winner = "base"
	}

	return Comparison{
		BaseID:      baseID,
		CompareID:   compareID,
		BaseMetrics: base.Metrics,
		CmpMetrics:  cmp.Metrics,
		Deltas:      deltas,
		Winner:      winner,
	}, nil
}

// TotalExperiments returns the total number of registered experiments.
func (t *Tracker) TotalExperiments() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.experiments)
}

// LeaderboardByMetric returns the top-N experiments sorted by a given metric.
func (t *Tracker) LeaderboardByMetric(metric string, topN int) []Experiment {
	t.mu.RLock()
	all := make([]Experiment, 0, len(t.experiments))
	for _, e := range t.experiments {
		if e.Status == StatusCompleted {
			all = append(all, *e)
		}
	}
	t.mu.RUnlock()

	sort.Slice(all, func(i, j int) bool {
		return metricValue(all[i].Metrics, metric) > metricValue(all[j].Metrics, metric)
	})
	if topN > 0 && len(all) > topN {
		all = all[:topN]
	}
	return all
}

// update applies a mutation function to an experiment under write lock.
func (t *Tracker) update(ctx context.Context, id string, fn func(*Experiment) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.experiments[id]
	if !ok {
		return fmt.Errorf("experiments: not found: %s", id)
	}
	if err := fn(e); err != nil {
		return err
	}
	e.UpdatedAt = time.Now().UTC()
	return nil
}

func metricValue(m Metrics, name string) float64 {
	switch name {
	case "sharpe", "sharpe_ratio":
		return m.SharpeRatio
	case "sortino", "sortino_ratio":
		return m.SortinoRatio
	case "max_drawdown":
		return -m.MaxDrawdown // lower is better
	case "win_rate":
		return m.WinRate
	case "profit_factor":
		return m.ProfitFactor
	case "alpha":
		return m.Alpha
	case "beta":
		return -math.Abs(m.Beta) // lower beta is better
	case "total_pnl":
		return m.TotalPnLUSD
	case "calmar":
		return m.Calmar
	default:
		return m.SharpeRatio
	}
}
