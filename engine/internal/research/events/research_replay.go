package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ReplayResult is the partitioned output of a full research event replay.
type ReplayResult struct {
	Features     []ResearchEvent
	Experiments  []ResearchEvent
	Models       []ResearchEvent
	Promotions   []ResearchEvent
	WalkForwards []ResearchEvent
	MonteCarlos  []ResearchEvent
	Regimes      []ResearchEvent
	Datasets     []ResearchEvent
	Total        int
}

// ReplayResearch replays all research events and partitions by aggregate type.
// Calling this twice on the same store always produces identical results —
// determinism is guaranteed by the append-only, hash-verified event log.
func ReplayResearch(ctx context.Context, store Store) (ReplayResult, error) {
	all, err := store.ReplayAll(ctx)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("research/replay: %w", err)
	}
	var res ReplayResult
	res.Total = len(all)
	for _, ev := range all {
		switch ev.AggregateType {
		case AggFeature:
			res.Features = append(res.Features, ev)
		case AggExperiment:
			res.Experiments = append(res.Experiments, ev)
		case AggModel:
			res.Models = append(res.Models, ev)
		case AggPromotion:
			res.Promotions = append(res.Promotions, ev)
		case AggWalkForward:
			res.WalkForwards = append(res.WalkForwards, ev)
		case AggMonteCarlo:
			res.MonteCarlos = append(res.MonteCarlos, ev)
		case AggRegime:
			res.Regimes = append(res.Regimes, ev)
		case AggDataset:
			res.Datasets = append(res.Datasets, ev)
		}
	}
	return res, nil
}

// ReplayExperiment rebuilds the ExperimentState for one experiment from events.
func ReplayExperiment(ctx context.Context, store Store, experimentID string) (ExperimentState, error) {
	evts, err := store.ReplayByExperiment(ctx, experimentID)
	if err != nil {
		return ExperimentState{}, fmt.Errorf("research/replay experiment %s: %w", experimentID, err)
	}
	state := ExperimentState{ID: experimentID}
	for _, ev := range evts {
		switch ev.EventType {
		case EvtExperimentStarted:
			var p ExperimentStartedPayload
			if json.Unmarshal(ev.Payload, &p) == nil {
				state.Name = p.Name
				state.StrategyID = p.StrategyID
				state.Parameters = p.Parameters
				state.DatasetID = p.DatasetID
				state.Status = "RUNNING"
				state.StartedAt = ev.CreatedAt
			}
		case EvtExperimentCompleted:
			var p ExperimentCompletedPayload
			if json.Unmarshal(ev.Payload, &p) == nil {
				state.Metrics = p.Metrics
				state.DurationMs = p.DurationMs
				state.Status = "COMPLETED"
				state.CompletedAt = ev.CreatedAt
			}
		case EvtExperimentFailed:
			state.Status = "FAILED"
			state.CompletedAt = ev.CreatedAt
		}
	}
	return state, nil
}

// ReplayModel rebuilds a ModelState from events persisted in the research store.
func ReplayModel(ctx context.Context, store Store, modelID string) (ModelState, error) {
	evts, err := store.Replay(ctx, AggModel, modelID)
	if err != nil {
		return ModelState{}, fmt.Errorf("research/replay model %s: %w", modelID, err)
	}
	state := ModelState{ID: modelID}
	for _, ev := range evts {
		switch ev.EventType {
		case EvtModelTrained:
			var p ModelTrainedPayload
			if json.Unmarshal(ev.Payload, &p) == nil {
				state.ModelType = p.ModelType
				state.ExperimentID = p.ExperimentID
				state.FeatureIDs = p.FeatureIDs
				state.Hyperparameters = p.Hyperparameters
				state.TrainMetrics = p.TrainMetrics
				state.ValMetrics = p.ValMetrics
				state.Status = ModelStatusTraining
				state.TrainedAt = ev.CreatedAt
			}
		case EvtModelValidated:
			state.Status = ModelStatusValidated
			state.ValidatedAt = ev.CreatedAt
		case EvtModelApproved:
			state.Status = ModelStatusApproved
			state.ApprovedAt = ev.CreatedAt
		case EvtModelRejected:
			state.Status = ModelStatusRejected
			state.RejectedAt = ev.CreatedAt
		}
	}
	return state, nil
}

// ReplayPromotionPipeline rebuilds the PromotionState for a strategy from events.
func ReplayPromotionPipeline(ctx context.Context, store Store, strategyID string) (PromotionState, error) {
	evts, err := store.Replay(ctx, AggPromotion, strategyID)
	if err != nil {
		return PromotionState{}, fmt.Errorf("research/replay promotion %s: %w", strategyID, err)
	}
	state := PromotionState{StrategyID: strategyID, Status: PromotionStatusResearch}
	for _, ev := range evts {
		switch ev.EventType {
		case EvtPromotionGatePass, EvtPromotionGateFail:
			var p PromotionPayload
			if json.Unmarshal(ev.Payload, &p) == nil {
				if state.GateResults == nil {
					state.GateResults = make(map[string]bool)
				}
				for gate, result := range p.GateResults {
					state.GateResults[gate] = result
				}
			}
			if ev.EventType == EvtPromotionGateFail {
				state.RejectedAt = ev.CreatedAt
			}
		case EvtStrategyPromoted:
			state.Status = PromotionStatusProduction
			state.PromotedAt = ev.CreatedAt
		case EvtStrategyRejected:
			state.Status = PromotionStatusRejected
			state.RejectedAt = ev.CreatedAt
		}
	}
	return state, nil
}

// ─── State types rebuilt by replay ───────────────────────────────────────────

// ExperimentState is the current state of a research experiment, rebuilt from events.
type ExperimentState struct {
	ID          string
	Name        string
	StrategyID  string
	Parameters  map[string]any
	DatasetID   string
	Status      string
	Metrics     ExperimentMetrics
	DurationMs  int64
	StartedAt   time.Time
	CompletedAt time.Time
}

// ModelStatus represents the lifecycle state of a research model.
type ModelStatus string

const (
	ModelStatusTraining  ModelStatus = "TRAINING"
	ModelStatusValidated ModelStatus = "VALIDATED"
	ModelStatusApproved  ModelStatus = "APPROVED"
	ModelStatusRejected  ModelStatus = "REJECTED"
	ModelStatusPromoted  ModelStatus = "PROMOTED"
)

// ModelState is the current state of a research model, rebuilt from events.
type ModelState struct {
	ID              string
	ModelType       string
	ExperimentID    string
	FeatureIDs      []string
	Hyperparameters map[string]any
	TrainMetrics    map[string]float64
	ValMetrics      map[string]float64
	Status          ModelStatus
	TrainedAt       time.Time
	ValidatedAt     time.Time
	ApprovedAt      time.Time
	RejectedAt      time.Time
}

// PromotionStatus represents the lifecycle state of a strategy in the promotion pipeline.
type PromotionStatus string

const (
	PromotionStatusResearch   PromotionStatus = "RESEARCH"
	PromotionStatusCandidate  PromotionStatus = "CANDIDATE"
	PromotionStatusValidated  PromotionStatus = "VALIDATED"
	PromotionStatusApproved   PromotionStatus = "APPROVED"
	PromotionStatusProduction PromotionStatus = "PRODUCTION"
	PromotionStatusRejected   PromotionStatus = "REJECTED"
)

// PromotionState is the current state of a strategy's promotion journey.
type PromotionState struct {
	StrategyID  string
	Status      PromotionStatus
	GateResults map[string]bool
	ApprovedBy  string
	PromotedAt  time.Time
	RejectedAt  time.Time
}
