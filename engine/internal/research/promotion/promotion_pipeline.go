// Package promotion implements the Phase 19J Strategy Promotion Pipeline.
// A strategy enters production ONLY after passing all mandatory gates:
// Walk-Forward → Monte Carlo → Regime → Risk → Research Review → Approval.
// States: RESEARCH → CANDIDATE → VALIDATED → APPROVED → PRODUCTION
// Strategies that fail any gate return to RESEARCH state for iteration.
package promotion

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ─── Pipeline States ──────────────────────────────────────────────────────────

type State string

const (
	StateResearch   State = "RESEARCH"
	StateCandidate  State = "CANDIDATE"
	StateValidated  State = "VALIDATED"
	StateApproved   State = "APPROVED"
	StateProduction State = "PRODUCTION"
	StateRejected   State = "REJECTED"
)

// ─── Gate Names ───────────────────────────────────────────────────────────────

const (
	GateWalkForward     = "walk_forward"
	GateMonteCarlo      = "monte_carlo"
	GateRegime          = "regime"
	GateRisk            = "risk"
	GateResearchReview  = "research_review"
	GateApproval        = "approval"
)

var requiredGates = []string{
	GateWalkForward, GateMonteCarlo, GateRegime,
	GateRisk, GateResearchReview, GateApproval,
}

// ─── Core Types ───────────────────────────────────────────────────────────────

// GateResult records the pass/fail outcome of a single promotion gate.
type GateResult struct {
	Gate      string
	Passed    bool
	Score     float64
	Details   string
	Reviewer  string
	ReviewedAt time.Time
}

// StrategyRecord tracks a strategy through the full promotion lifecycle.
type StrategyRecord struct {
	StrategyID   string
	Name         string
	Description  string
	FamilyName   string
	ResearcherID string
	ExperimentID string
	ModelID      string

	State       State
	GateResults map[string]GateResult
	History     []StateTransition

	// Promotion metadata
	PromotedBy  string
	PromotedAt  time.Time

	// Rejection metadata
	RejectedBy  string
	RejectedAt  time.Time
	RejectedReason string

	CreatedAt   time.Time
	UpdatedAt   time.Time

	// Performance summary at the time of promotion decision.
	SharpeOOS   float64
	MaxDrawdown float64
	WinRate     float64
	RiskOfRuin  float64
}

// StateTransition records a single state change in the pipeline.
type StateTransition struct {
	From      State
	To        State
	Actor     string
	Reason    string
	ChangedAt time.Time
}

// ─── Pipeline ─────────────────────────────────────────────────────────────────

// Pipeline manages the institutional strategy promotion workflow.
type Pipeline struct {
	mu         sync.RWMutex
	strategies map[string]*StrategyRecord
	history    []StateTransition
}

// NewPipeline creates an empty promotion pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{strategies: make(map[string]*StrategyRecord)}
}

// Submit adds a strategy to the pipeline in RESEARCH state.
func (p *Pipeline) Submit(ctx context.Context, rec StrategyRecord) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if rec.StrategyID == "" {
		return "", errors.New("promotion: strategy id required")
	}
	if rec.ResearcherID == "" {
		return "", errors.New("promotion: researcher id required")
	}
	rec.State = StateResearch
	rec.GateResults = make(map[string]GateResult)
	rec.History = []StateTransition{{
		From: "", To: StateResearch, Actor: rec.ResearcherID,
		Reason: "submitted to research pipeline", ChangedAt: time.Now().UTC(),
	}}
	rec.CreatedAt = time.Now().UTC()
	rec.UpdatedAt = rec.CreatedAt

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.strategies[rec.StrategyID]; exists {
		return "", fmt.Errorf("promotion: strategy %s already in pipeline", rec.StrategyID)
	}
	p.strategies[rec.StrategyID] = &rec
	return rec.StrategyID, nil
}

// Nominate advances a RESEARCH strategy to CANDIDATE, signalling it is
// ready for gate evaluation.
func (p *Pipeline) Nominate(ctx context.Context, strategyID, nominatedBy string) error {
	return p.transition(ctx, strategyID, StateResearch, StateCandidate, nominatedBy,
		"nominated for promotion gate evaluation")
}

// RecordGateResult saves the outcome of a gate evaluation.
func (p *Pipeline) RecordGateResult(ctx context.Context, strategyID string, result GateResult) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	if rec.State != StateCandidate && rec.State != StateValidated {
		return fmt.Errorf("promotion: cannot record gate for strategy in state %s", rec.State)
	}
	result.ReviewedAt = time.Now().UTC()
	rec.GateResults[result.Gate] = result
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

// TryValidate checks if all pre-approval gates have passed and advances the
// strategy to VALIDATED if so.
func (p *Pipeline) TryValidate(ctx context.Context, strategyID, actor string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return false, fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	if rec.State != StateCandidate {
		return false, fmt.Errorf("promotion: strategy must be CANDIDATE to validate (got %s)", rec.State)
	}

	// Check pre-approval gates (everything except the final approval gate).
	preGates := []string{GateWalkForward, GateMonteCarlo, GateRegime, GateRisk, GateResearchReview}
	for _, gate := range preGates {
		gr, ok := rec.GateResults[gate]
		if !ok {
			return false, fmt.Errorf("promotion: gate %s not yet evaluated", gate)
		}
		if !gr.Passed {
			return false, nil // gates failed — stays in CANDIDATE for iteration
		}
	}

	// All pre-gates passed: advance to VALIDATED.
	rec.State = StateValidated
	rec.History = append(rec.History, StateTransition{
		From: StateCandidate, To: StateValidated, Actor: actor,
		Reason: "all pre-approval gates passed", ChangedAt: time.Now().UTC(),
	})
	rec.UpdatedAt = time.Now().UTC()
	return true, nil
}

// Approve advances a VALIDATED strategy to APPROVED.
// This requires the final manual approval gate to pass.
func (p *Pipeline) Approve(ctx context.Context, strategyID, approvedBy, reason string) error {
	if approvedBy == "" {
		return errors.New("promotion: approver identity required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	if rec.State != StateValidated {
		return fmt.Errorf("promotion: cannot approve strategy in state %s (need VALIDATED)", rec.State)
	}
	// Record approval gate.
	rec.GateResults[GateApproval] = GateResult{
		Gate: GateApproval, Passed: true,
		Details: reason, Reviewer: approvedBy, ReviewedAt: time.Now().UTC(),
	}
	rec.State = StateApproved
	rec.History = append(rec.History, StateTransition{
		From: StateValidated, To: StateApproved, Actor: approvedBy,
		Reason: reason, ChangedAt: time.Now().UTC(),
	})
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

// Promote moves an APPROVED strategy to PRODUCTION.
// This is irreversible and permanently wires the strategy for live deployment.
// The production trading system is notified via the PromotionNotifier interface.
func (p *Pipeline) Promote(ctx context.Context, strategyID, promotedBy string, notifier PromotionNotifier) error {
	if promotedBy == "" {
		return errors.New("promotion: promoter identity required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	if rec.State != StateApproved {
		return fmt.Errorf("promotion: cannot promote strategy in state %s (need APPROVED)", rec.State)
	}

	// Verify all gates passed before committing to production.
	for _, gate := range requiredGates {
		gr, ok := rec.GateResults[gate]
		if !ok || !gr.Passed {
			return fmt.Errorf("promotion: gate %q not passed — promotion blocked", gate)
		}
	}

	rec.State = StateProduction
	rec.PromotedBy = promotedBy
	rec.PromotedAt = time.Now().UTC()
	rec.History = append(rec.History, StateTransition{
		From: StateApproved, To: StateProduction, Actor: promotedBy,
		Reason: "all gates passed, promoted to production", ChangedAt: time.Now().UTC(),
	})
	rec.UpdatedAt = time.Now().UTC()

	// Notify the production system — this is the ONLY interface point between
	// research and production. It passes metadata only, never credentials.
	if notifier != nil {
		pn := PromotionNotification{
			StrategyID:   strategyID,
			FamilyName:   rec.FamilyName,
			SharpeOOS:    rec.SharpeOOS,
			MaxDrawdown:  rec.MaxDrawdown,
			WinRate:      rec.WinRate,
			PromotedBy:   promotedBy,
			PromotedAt:   rec.PromotedAt,
			GateResults:  makeGateSummary(rec.GateResults),
		}
		if err := notifier.NotifyPromotion(ctx, pn); err != nil {
			// Log but don't fail — the registry is the source of truth.
			_ = err
		}
	}
	return nil
}

// Reject sends a strategy back to RESEARCH state with a failure reason.
func (p *Pipeline) Reject(ctx context.Context, strategyID, rejectedBy, reason string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	fromState := rec.State
	rec.State = StateRejected
	rec.RejectedBy = rejectedBy
	rec.RejectedAt = time.Now().UTC()
	rec.RejectedReason = reason
	rec.History = append(rec.History, StateTransition{
		From: fromState, To: StateRejected, Actor: rejectedBy,
		Reason: reason, ChangedAt: time.Now().UTC(),
	})
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

// Resubmit returns a REJECTED strategy to RESEARCH for further iteration.
func (p *Pipeline) Resubmit(ctx context.Context, strategyID, actor string) error {
	return p.transition(ctx, strategyID, StateRejected, StateResearch, actor,
		"resubmitted for further research iteration")
}

// Get retrieves a strategy record by ID.
func (p *Pipeline) Get(strategyID string) (StrategyRecord, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return StrategyRecord{}, fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	return *rec, nil
}

// ListByState returns all strategies in the given state.
func (p *Pipeline) ListByState(state State) []StrategyRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []StrategyRecord
	for _, rec := range p.strategies {
		if rec.State == state {
			out = append(out, *rec)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

// PipelineSummary returns counts by state for operational dashboards.
func (p *Pipeline) PipelineSummary() map[State]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	counts := make(map[State]int)
	for _, rec := range p.strategies {
		counts[rec.State]++
	}
	return counts
}

// BlockedStrategies returns all strategies blocked by a specific failed gate.
func (p *Pipeline) BlockedStrategies(gate string) []StrategyRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []StrategyRecord
	for _, rec := range p.strategies {
		if rec.State == StateCandidate || rec.State == StateValidated {
			if gr, ok := rec.GateResults[gate]; ok && !gr.Passed {
				out = append(out, *rec)
			}
		}
	}
	return out
}

// GateSummary returns the pass/fail/pending status of all gates for a strategy.
func (p *Pipeline) GateSummary(strategyID string) (map[string]string, error) {
	rec, err := p.Get(strategyID)
	if err != nil {
		return nil, err
	}
	return makeGateSummary(rec.GateResults), nil
}

// ─── Production Notification Interface ───────────────────────────────────────

// PromotionNotifier is the ONLY interface the research pipeline uses to
// communicate with the production system. It passes metadata only — never
// credentials, never execution instructions. The production system decides
// independently whether to activate the promoted strategy.
type PromotionNotifier interface {
	NotifyPromotion(ctx context.Context, notification PromotionNotification) error
}

// PromotionNotification carries strategy metadata to the production system.
// It contains NO broker credentials, NO order parameters, NO execution logic.
type PromotionNotification struct {
	StrategyID   string
	FamilyName   string
	SharpeOOS    float64
	MaxDrawdown  float64
	WinRate      float64
	PromotedBy   string
	PromotedAt   time.Time
	GateResults  map[string]string // gate → "PASS" or "FAIL"
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (p *Pipeline) transition(ctx context.Context, strategyID string, from, to State, actor, reason string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	rec, ok := p.strategies[strategyID]
	if !ok {
		return fmt.Errorf("promotion: strategy %s not found", strategyID)
	}
	if rec.State != from {
		return fmt.Errorf("promotion: expected state %s, got %s", from, rec.State)
	}
	rec.State = to
	rec.History = append(rec.History, StateTransition{
		From: from, To: to, Actor: actor,
		Reason: reason, ChangedAt: time.Now().UTC(),
	})
	rec.UpdatedAt = time.Now().UTC()
	return nil
}

func makeGateSummary(gateResults map[string]GateResult) map[string]string {
	summary := make(map[string]string, len(requiredGates))
	for _, gate := range requiredGates {
		gr, ok := gateResults[gate]
		switch {
		case !ok:
			summary[gate] = "PENDING"
		case gr.Passed:
			summary[gate] = "PASS"
		default:
			summary[gate] = "FAIL"
		}
	}
	return summary
}
