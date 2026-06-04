package sor

import (
	"context"
	"fmt"
	"time"

	"antigravity-engine/internal/ledger"
)

// ExecutionPlan is the complete, auditable plan for executing a parent order:
// the venue scores, the split across venues, and the time-scheduled children.
type ExecutionPlan struct {
	ParentClientOrderID string
	Symbol              string
	Side                string
	RequestedQty        float64
	Algo                ExecutionAlgo
	SplitMethod         SplitMethod

	Scores      []BestExecutionScore
	Split       SplitPlan
	Children    []ChildOrder
	ReferencePrice float64

	PlannedAt time.Time
}

// VenueAllocationsMap returns venueID → total quantity for the plan.
func (p ExecutionPlan) VenueAllocationsMap() map[VenueID]float64 {
	out := make(map[VenueID]float64)
	for _, a := range p.Split.Allocations {
		out[a.VenueID] += a.Quantity
	}
	return out
}

// ExecutionPlanner turns a best-execution evaluation into a concrete,
// time-scheduled execution plan. It composes the splitter and algo scheduler.
type ExecutionPlanner struct {
	splitter  *OrderSplitter
	scheduler *AlgoScheduler
	registry  *VenueRegistry
	store     ledger.Store
}

// NewExecutionPlanner constructs an execution planner.
func NewExecutionPlanner(splitter *OrderSplitter, scheduler *AlgoScheduler, registry *VenueRegistry, store ledger.Store) *ExecutionPlanner {
	return &ExecutionPlanner{
		splitter:  splitter,
		scheduler: scheduler,
		registry:  registry,
		store:     store,
	}
}

// PlanInput parameterises plan construction.
type PlanInput struct {
	ParentClientOrderID string
	Symbol              string
	Side                string
	Quantity            float64
	OrderType           string
	Algo                ExecutionAlgo
	AlgoParams          AlgoParams
	ForceSplitMethod    SplitMethod
	ReferencePrice      float64
}

// Plan builds an ExecutionPlan from ranked best-execution scores.
func (p *ExecutionPlanner) Plan(ctx context.Context, in PlanInput, scores []BestExecutionScore) (ExecutionPlan, error) {
	plan := ExecutionPlan{
		ParentClientOrderID: in.ParentClientOrderID,
		Symbol:              in.Symbol,
		Side:                in.Side,
		RequestedQty:        in.Quantity,
		Algo:                in.Algo,
		Scores:              scores,
		ReferencePrice:      in.ReferencePrice,
		PlannedAt:           time.Now().UTC(),
	}

	refPrice := in.ReferencePrice
	if refPrice <= 0 {
		refPrice = p.deriveRefPrice(in.Symbol, scores)
		plan.ReferencePrice = refPrice
	}

	// 1. Split across venues.
	split, err := p.splitter.Split(in.ParentClientOrderID, in.Symbol, in.Side, in.Quantity, refPrice, scores, in.ForceSplitMethod)
	if err != nil {
		return plan, fmt.Errorf("planner: split: %w", err)
	}
	plan.Split = split
	plan.SplitMethod = split.Method

	// 2. Schedule each venue allocation into child orders via the algo.
	algoParams := in.AlgoParams
	if algoParams.Algo == "" {
		algoParams.Algo = in.Algo
	}
	if algoParams.Algo == "" {
		algoParams.Algo = AlgoImmediate
	}

	seq := 0
	for _, alloc := range split.Allocations {
		children := p.scheduler.Schedule(in.ParentClientOrderID, in.Symbol, in.Side, in.OrderType, alloc, algoParams, seq)
		plan.Children = append(plan.Children, children...)
		seq += len(children)
	}

	if len(plan.Children) == 0 {
		return plan, fmt.Errorf("planner: produced zero child orders")
	}

	// 3. Emit planning events.
	emitRoute(ctx, p.store, in.ParentClientOrderID, EventExecutionPlanned, in.Symbol, "", ExecutionPlannedPayload{
		ParentClientOrderID: in.ParentClientOrderID,
		Symbol:              in.Symbol,
		Algo:                string(algoParams.Algo),
		SplitMethod:         string(split.Method),
		ChildCount:          len(plan.Children),
		VenueAllocations:    plan.VenueAllocationsMap(),
		PlannedAt:           plan.PlannedAt,
	})
	for _, child := range plan.Children {
		emitRoute(ctx, p.store, in.ParentClientOrderID, EventOrderSplit, in.Symbol, "", OrderSplitPayload{
			ParentClientOrderID: in.ParentClientOrderID,
			ChildOrderID:        child.ChildOrderID,
			VenueID:             child.VenueID,
			Symbol:              child.Symbol,
			Side:                child.Side,
			Quantity:            child.Quantity,
			SequenceIndex:       child.SequenceIndex,
			ScheduledAt:         child.ScheduledAt,
		})
	}

	return plan, nil
}

// deriveRefPrice computes a reference price from the winning venue's market data.
func (p *ExecutionPlanner) deriveRefPrice(symbol string, scores []BestExecutionScore) float64 {
	if w, ok := Winner(scores); ok {
		if md, ok := p.registry.MarketData(w.VenueID, symbol); ok {
			return md.Mid()
		}
	}
	// Fall back to any candidate's mid.
	for _, s := range scores {
		if md, ok := p.registry.MarketData(s.VenueID, symbol); ok && md.Mid() > 0 {
			return md.Mid()
		}
	}
	return 0
}
