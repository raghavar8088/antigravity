package pilot

import (
	"context"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// PilotAggregate is the event-sourced state machine for a live trading pilot.
// It governs stage progression, strategy deployment, capital scaling decisions,
// and halt/resume lifecycle.
type PilotAggregate struct {
	PilotID      string
	AccountID    string
	TotalCapital float64 // total available capital in USD

	Stage         Stage
	Halted        bool
	HaltReason    string
	Certification CertificationStatus

	DeployedStrategies map[string]DeployedStrategy
	StageStartedAt     time.Time
	InitiatedAt        time.Time
	UpdatedAt          time.Time

	RiskViolations int
	Version        int64

	store ledger.Store
}

// DeployedStrategy tracks the current allocation state for a live strategy.
type DeployedStrategy struct {
	StrategyID   string
	StrategyName string
	AllocPct     float64
	AllocUSD     float64
	Rank         int
	DeployedAt   time.Time
}

// NewPilotAggregate creates a new pilot aggregate in NOT_STARTED state.
func NewPilotAggregate(pilotID, accountID string, totalCapital float64, store ledger.Store) *PilotAggregate {
	return &PilotAggregate{
		PilotID:            pilotID,
		AccountID:          accountID,
		TotalCapital:       totalCapital,
		Stage:              StageNotStarted,
		DeployedStrategies: make(map[string]DeployedStrategy),
		InitiatedAt:        time.Now().UTC(),
		store:              store,
	}
}

// Initialize emits the pilot creation event. Must be called once after construction.
func (a *PilotAggregate) Initialize(ctx context.Context, initiatedBy string) {
	EmitPilotInitialized(ctx, a.store, PilotInitializedPayload{
		PilotID:      a.PilotID,
		AccountID:    a.AccountID,
		TotalCapital: a.TotalCapital,
		InitiatedBy:  initiatedBy,
		InitiatedAt:  a.InitiatedAt,
	})
}

// AdvanceStage moves the pilot to the next deployment stage.
// Returns an error if the pilot is halted or already at Stage4.
func (a *PilotAggregate) AdvanceStage(ctx context.Context, metrics LiveMetricsSummary) error {
	if a.Halted {
		return fmt.Errorf("pilot %s is halted: %s", a.PilotID, a.HaltReason)
	}
	if a.Stage == Stage4 {
		return fmt.Errorf("pilot %s is already at final stage", a.PilotID)
	}
	prev := a.Stage
	a.Stage++
	a.StageStartedAt = time.Now().UTC()
	a.updatedNow()
	EmitStageAdvanced(ctx, a.store, a.PilotID, a.AccountID, StageAdvancedPayload{
		PilotID:       a.PilotID,
		PreviousStage: prev,
		NewStage:      a.Stage,
		Reason:        "qualifying_metrics_met",
		AdvancedAt:    a.StageStartedAt,
		Metrics:       metrics,
	})
	return nil
}

// DeployStrategy adds a strategy to the live deployment under the current stage's capital limits.
func (a *PilotAggregate) DeployStrategy(ctx context.Context, id, name string, rank int, allocPct float64) error {
	if a.Halted {
		return fmt.Errorf("pilot %s is halted", a.PilotID)
	}
	if a.Stage == StageNotStarted {
		return fmt.Errorf("pilot %s has not started; call AdvanceStage first", a.PilotID)
	}
	maxStrats := a.Stage.MaxStrategies()
	if maxStrats > 0 && len(a.DeployedStrategies) >= maxStrats {
		return fmt.Errorf("stage %s already has maximum %d strategies deployed", a.Stage, maxStrats)
	}
	minPct, maxPct := a.Stage.CapitalRangePct()
	if allocPct < minPct || allocPct > maxPct {
		return fmt.Errorf("allocPct %.4f outside stage %s range [%.4f, %.4f]", allocPct, a.Stage, minPct, maxPct)
	}
	allocUSD := allocPct * a.TotalCapital
	now := time.Now().UTC()
	a.DeployedStrategies[id] = DeployedStrategy{
		StrategyID:   id,
		StrategyName: name,
		AllocPct:     allocPct,
		AllocUSD:     allocUSD,
		Rank:         rank,
		DeployedAt:   now,
	}
	a.updatedNow()
	EmitStrategyDeployed(ctx, a.store, a.AccountID, StrategyDeployedPayload{
		PilotID:      a.PilotID,
		StrategyID:   id,
		StrategyName: name,
		Stage:        a.Stage,
		AllocPct:     allocPct,
		AllocUSD:     allocUSD,
		Rank:         rank,
		DeployedAt:   now,
	})
	return nil
}

// RetractStrategy removes a strategy from live deployment.
func (a *PilotAggregate) RetractStrategy(ctx context.Context, id, reason string) {
	ds, ok := a.DeployedStrategies[id]
	if !ok {
		return
	}
	delete(a.DeployedStrategies, id)
	a.updatedNow()
	emitPilotEvent(ctx, a.store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   a.PilotID,
		EventType:     EventPilotStrategyRetracted,
		AccountID:     a.AccountID,
		StrategyID:    id,
		Payload: StrategyRetractedPayload{
			PilotID:      a.PilotID,
			StrategyID:   id,
			StrategyName: ds.StrategyName,
			Reason:       reason,
			RetractedAt:  time.Now().UTC(),
		},
		Source: "pilot.deployment",
	})
}

// ScaleCapital adjusts a strategy's allocation percentage and emits the scaling event.
func (a *PilotAggregate) ScaleCapital(ctx context.Context, strategyID, direction, trigger string, newAllocPct, metricValue float64) error {
	ds, ok := a.DeployedStrategies[strategyID]
	if !ok {
		return fmt.Errorf("strategy %s not deployed in pilot %s", strategyID, a.PilotID)
	}
	prev := ds
	ds.AllocPct = newAllocPct
	ds.AllocUSD = newAllocPct * a.TotalCapital
	a.DeployedStrategies[strategyID] = ds
	a.updatedNow()
	EmitCapitalScaled(ctx, a.store, a.AccountID, CapitalScaledPayload{
		PilotID:          a.PilotID,
		StrategyID:       strategyID,
		Direction:        direction,
		PreviousAllocPct: prev.AllocPct,
		NewAllocPct:      newAllocPct,
		PreviousAllocUSD: prev.AllocUSD,
		NewAllocUSD:      ds.AllocUSD,
		Trigger:          trigger,
		MetricValue:      metricValue,
		ScaledAt:         time.Now().UTC(),
	})
	return nil
}

// UpdateCertification updates the certification status when it changes.
func (a *PilotAggregate) UpdateCertification(ctx context.Context, newStatus CertificationStatus, reasons []string, metrics LiveMetricsSummary) {
	prev := a.Certification
	a.Certification = newStatus
	a.updatedNow()
	EmitCertificationUpdated(ctx, a.store, a.AccountID, CertificationUpdatedPayload{
		PilotID:   a.PilotID,
		Previous:  prev,
		Current:   newStatus,
		Reasons:   reasons,
		Metrics:   metrics,
		UpdatedAt: a.UpdatedAt,
	})
}

// RecordRiskViolation increments the violation counter and emits a PILOT_RISK_VIOLATION event.
func (a *PilotAggregate) RecordRiskViolation(ctx context.Context, strategyID, violationType, details string) {
	a.RiskViolations++
	a.updatedNow()
	emitPilotEvent(ctx, a.store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   a.PilotID,
		EventType:     EventPilotRiskViolation,
		AccountID:     a.AccountID,
		StrategyID:    strategyID,
		Payload: RiskViolationPayload{
			PilotID:       a.PilotID,
			StrategyID:    strategyID,
			ViolationType: violationType,
			Details:       details,
			OccurredAt:    time.Now().UTC(),
		},
		Source: "pilot.risk",
	})
}

// Halt stops all pilot activity and emits PILOT_HALTED.
func (a *PilotAggregate) Halt(ctx context.Context, reason, haltedBy string) {
	a.Halted = true
	a.HaltReason = reason
	a.updatedNow()
	emitPilotEvent(ctx, a.store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   a.PilotID,
		EventType:     EventPilotHalted,
		AccountID:     a.AccountID,
		Payload: PilotHaltedPayload{
			PilotID:  a.PilotID,
			Reason:   reason,
			HaltedBy: haltedBy,
			HaltedAt: time.Now().UTC(),
		},
		Source: "pilot",
	})
}

// Resume clears the halted state and emits PILOT_RESUMED.
func (a *PilotAggregate) Resume(ctx context.Context) {
	a.Halted = false
	a.HaltReason = ""
	a.updatedNow()
	emitPilotEvent(ctx, a.store, ledger.NewEventInput{
		AggregateType: AggregatePilot,
		AggregateID:   a.PilotID,
		EventType:     EventPilotResumed,
		AccountID:     a.AccountID,
		Payload:       map[string]any{"pilot_id": a.PilotID, "resumed_at": time.Now().UTC()},
		Source:        "pilot",
	})
}

func (a *PilotAggregate) updatedNow() {
	a.UpdatedAt = time.Now().UTC()
	a.Version++
}
