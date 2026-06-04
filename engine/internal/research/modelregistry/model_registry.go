// Package modelregistry implements the Phase 19H Model Registry.
// No model may enter production without passing the full approval workflow.
// States: TRAINING → VALIDATED → APPROVED / REJECTED → PROMOTED.
package modelregistry

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Model Lifecycle States ───────────────────────────────────────────────────

type State string

const (
	StateTraining  State = "TRAINING"
	StateValidated State = "VALIDATED"
	StateApproved  State = "APPROVED"
	StateRejected  State = "REJECTED"
	StatePromoted  State = "PROMOTED"
)

// validTransitions maps allowed state machine transitions.
var validTransitions = map[State][]State{
	StateTraining:  {StateValidated, StateRejected},
	StateValidated: {StateApproved, StateRejected},
	StateApproved:  {StatePromoted, StateRejected},
	StateRejected:  {StateTraining}, // can retrain from rejected
	StatePromoted:  {},              // terminal
}

// ─── Core Types ───────────────────────────────────────────────────────────────

// ModelVersion captures a specific version of a registered model.
type ModelVersion struct {
	Version        int
	ArtifactPath   string
	FeatureIDs     []string
	TrainMetrics   map[string]float64
	ValMetrics     map[string]float64
	DatasetID      string
	ModelType      string
	Hyperparameters map[string]any
	CreatedAt      time.Time
}

// ApprovalRecord captures one approval or rejection decision.
type ApprovalRecord struct {
	DecisionBy  string
	Decision    string // "APPROVED" or "REJECTED"
	Reason      string
	ReviewedAt  time.Time
}

// Model is a registered research model with full lifecycle tracking.
type Model struct {
	ID            string
	Name          string
	Description   string
	ExperimentID  string
	StrategyID    string
	ResearcherID  string
	ModelType     string

	State         State
	CurrentVersion int
	Versions      []ModelVersion

	// Promotion requirements checklist.
	WalkForwardPassed bool
	MonteCarloPassed  bool
	RegimePassed      bool
	RiskPassed        bool

	ApprovalHistory []ApprovalRecord
	PromotedVersion  int
	PromotedAt       time.Time
	PromotedBy       string

	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ─── Registry ─────────────────────────────────────────────────────────────────

// Registry maintains all research models and enforces lifecycle governance.
type Registry struct {
	mu     sync.RWMutex
	models map[string]*Model
}

// NewRegistry creates a new model registry.
func NewRegistry() *Registry {
	return &Registry{models: make(map[string]*Model)}
}

// Register adds a new model to the registry in TRAINING state.
func (r *Registry) Register(ctx context.Context, m Model) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if m.Name == "" {
		return "", errors.New("modelregistry: model name required")
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("mdl_%d", time.Now().UnixNano())
	}
	m.State = StateTraining
	m.CurrentVersion = 1
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.models[m.ID]; exists {
		return "", fmt.Errorf("modelregistry: id %s already exists", m.ID)
	}
	r.models[m.ID] = &m
	return m.ID, nil
}

// AddVersion adds a new trained version artifact to an existing model.
func (r *Registry) AddVersion(ctx context.Context, modelID string, v ModelVersion) error {
	return r.update(ctx, modelID, func(m *Model) error {
		v.Version = m.CurrentVersion
		m.CurrentVersion++
		v.CreatedAt = time.Now().UTC()
		m.Versions = append(m.Versions, v)
		return nil
	})
}

// Validate transitions a model from TRAINING to VALIDATED.
// Requires all metric gates to be satisfied.
func (r *Registry) Validate(ctx context.Context, modelID string, valMetrics map[string]float64) error {
	return r.transition(ctx, modelID, StateValidated, "", "", func(m *Model) error {
		// Minimum validation gate: val Sharpe > 0.3.
		if sharpe, ok := valMetrics["sharpe_ratio"]; ok && sharpe < 0.3 {
			return fmt.Errorf("modelregistry: validation failed — val Sharpe %.3f < 0.30", sharpe)
		}
		return nil
	})
}

// SetGateResult records the pass/fail result of a promotion gate.
func (r *Registry) SetGateResult(ctx context.Context, modelID, gate string, passed bool) error {
	return r.update(ctx, modelID, func(m *Model) error {
		switch gate {
		case "walkforward":
			m.WalkForwardPassed = passed
		case "montecarlo":
			m.MonteCarloPassed = passed
		case "regime":
			m.RegimePassed = passed
		case "risk":
			m.RiskPassed = passed
		default:
			return fmt.Errorf("modelregistry: unknown gate %q", gate)
		}
		return nil
	})
}

// Approve transitions a VALIDATED model to APPROVED.
// Requires all four promotion gates to have passed.
func (r *Registry) Approve(ctx context.Context, modelID, approvedBy, reason string) error {
	if approvedBy == "" {
		return errors.New("modelregistry: approver identity required")
	}
	return r.transition(ctx, modelID, StateApproved, approvedBy, reason, func(m *Model) error {
		if !m.WalkForwardPassed {
			return errors.New("modelregistry: walk-forward gate not passed")
		}
		if !m.MonteCarloPassed {
			return errors.New("modelregistry: monte carlo gate not passed")
		}
		if !m.RegimePassed {
			return errors.New("modelregistry: regime gate not passed")
		}
		if !m.RiskPassed {
			return errors.New("modelregistry: risk gate not passed")
		}
		return nil
	})
}

// Reject transitions a model to REJECTED state at any point in the lifecycle.
func (r *Registry) Reject(ctx context.Context, modelID, rejectedBy, reason string) error {
	return r.transition(ctx, modelID, StateRejected, rejectedBy, reason, nil)
}

// Promote marks an APPROVED model as PROMOTED to production.
// This is the FINAL gate — after this, the strategy becomes available to
// the production Promotion Pipeline for human review and execution wiring.
func (r *Registry) Promote(ctx context.Context, modelID, promotedBy string) error {
	if promotedBy == "" {
		return errors.New("modelregistry: promoter identity required")
	}
	return r.update(ctx, modelID, func(m *Model) error {
		if m.State != StateApproved {
			return fmt.Errorf("modelregistry: can only promote APPROVED models, got %s", m.State)
		}
		if err := validateTransition(m.State, StatePromoted); err != nil {
			return err
		}
		m.State = StatePromoted
		m.PromotedVersion = m.CurrentVersion - 1
		m.PromotedAt = time.Now().UTC()
		m.PromotedBy = promotedBy
		m.ApprovalHistory = append(m.ApprovalHistory, ApprovalRecord{
			DecisionBy: promotedBy, Decision: "PROMOTED",
			Reason: "all gates passed + operator approval", ReviewedAt: time.Now().UTC(),
		})
		return nil
	})
}

// Get retrieves a model by ID.
func (r *Registry) Get(id string) (Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.models[id]
	if !ok {
		return Model{}, fmt.Errorf("modelregistry: not found: %s", id)
	}
	return *m, nil
}

// ListByState returns all models in a given state.
func (r *Registry) ListByState(state State) []Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Model
	for _, m := range r.models {
		if m.State == state {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// TotalModels returns the number of registered models.
func (r *Registry) TotalModels() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (r *Registry) transition(ctx context.Context, id string, to State, actor, reason string, fn func(*Model) error) error {
	return r.update(ctx, id, func(m *Model) error {
		if err := validateTransition(m.State, to); err != nil {
			return err
		}
		if fn != nil {
			if err := fn(m); err != nil {
				return err
			}
		}
		m.State = to
		if actor != "" {
			m.ApprovalHistory = append(m.ApprovalHistory, ApprovalRecord{
				DecisionBy: actor, Decision: string(to),
				Reason: reason, ReviewedAt: time.Now().UTC(),
			})
		}
		return nil
	})
}

func (r *Registry) update(ctx context.Context, id string, fn func(*Model) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.models[id]
	if !ok {
		return fmt.Errorf("modelregistry: not found: %s", id)
	}
	if err := fn(m); err != nil {
		return err
	}
	m.UpdatedAt = time.Now().UTC()
	return nil
}

func validateTransition(from, to State) error {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return nil
		}
	}
	return fmt.Errorf("modelregistry: invalid transition %s → %s", from, to)
}
