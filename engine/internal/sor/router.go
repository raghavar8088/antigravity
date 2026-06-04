package sor

import (
	"context"
	"fmt"
	"log"
	"time"

	"antigravity-engine/internal/ledger"
)

// OrderIntent is an OMS v3-approved order handed to the SOR for execution.
// The SOR is the SOLE execution authority: OMS never routes to exchanges directly.
type OrderIntent struct {
	ClientOrderID string        // parent ID, assigned by OMS v3
	Symbol        string
	Side          string        // BUY | SELL
	Quantity      float64
	OrderType     string        // MARKET | LIMIT | POST_ONLY | IOC
	LimitPrice    float64
	Algo          ExecutionAlgo // execution algorithm (default IMMEDIATE)
	AlgoParams    AlgoParams
	ForceSplit    SplitMethod   // optional override
	StrategyName  string
	Urgency       string        // LOW | NORMAL | HIGH
	IsMaker       bool          // passive (post-only/limit) vs aggressive
	HoldingHours  float64       // expected holding period for cost model
	RequestedAt   time.Time
}

// RouteResult is the outcome of a Route call.
type RouteResult struct {
	ParentClientOrderID string
	Plan                ExecutionPlan
	Report              ExecutionReport
	WinningVenue        VenueID
	Routed              bool
	Reason              string
}

// VenueSelector evaluates candidate venues and produces ranked best-execution
// scores. It is a thin orchestration layer over the BestExecutionEngine.
type VenueSelector struct {
	registry *VenueRegistry
	bestExec *BestExecutionEngine
}

// NewVenueSelector constructs a venue selector.
func NewVenueSelector(registry *VenueRegistry, bestExec *BestExecutionEngine) *VenueSelector {
	return &VenueSelector{registry: registry, bestExec: bestExec}
}

// Select returns ranked best-execution scores for an order intent.
func (s *VenueSelector) Select(intent OrderIntent) []BestExecutionScore {
	candidates := s.registry.CandidateVenues(intent.Symbol)
	return s.bestExec.Evaluate(candidates, EvaluationInput{
		Symbol:       intent.Symbol,
		Side:         intent.Side,
		Quantity:     intent.Quantity,
		IsMaker:      intent.IsMaker,
		HoldingHours: intent.HoldingHours,
	})
}

// SmartOrderRouter is the sole execution authority. It receives OMS-approved
// orders, evaluates venues, builds an execution plan, and coordinates execution.
//
// Flow:
//
//	OMS v3 → SOR.Route → VenueSelector → ExecutionPlanner → ExecutionCoordinator → Venues
type SmartOrderRouter struct {
	registry    *VenueRegistry
	selector    *VenueSelector
	planner     *ExecutionPlanner
	coordinator *ExecutionCoordinator
	store       ledger.Store
}

// NewSmartOrderRouter wires the full SOR pipeline.
func NewSmartOrderRouter(
	registry *VenueRegistry,
	selector *VenueSelector,
	planner *ExecutionPlanner,
	coordinator *ExecutionCoordinator,
	store ledger.Store,
) *SmartOrderRouter {
	return &SmartOrderRouter{
		registry:    registry,
		selector:    selector,
		planner:     planner,
		coordinator: coordinator,
		store:       store,
	}
}

// Route is the single entry point for executing an OMS-approved order.
// It is fully event-sourced: every decision emits a ledger event.
func (r *SmartOrderRouter) Route(ctx context.Context, intent OrderIntent) (RouteResult, error) {
	if intent.RequestedAt.IsZero() {
		intent.RequestedAt = time.Now().UTC()
	}
	if intent.Algo == "" {
		intent.Algo = AlgoImmediate
	}
	result := RouteResult{ParentClientOrderID: intent.ClientOrderID}

	// 1. RouteCreated.
	emitRoute(ctx, r.store, intent.ClientOrderID, EventRouteCreated, intent.Symbol, intent.StrategyName, RouteCreatedPayload{
		ParentClientOrderID: intent.ClientOrderID,
		Symbol:              intent.Symbol,
		Side:                intent.Side,
		Quantity:            intent.Quantity,
		OrderType:           intent.OrderType,
		Algo:                string(intent.Algo),
		Urgency:             intent.Urgency,
		StrategyName:        intent.StrategyName,
		CreatedAt:           intent.RequestedAt,
	})

	// 2. Venue selection (best execution).
	scores := r.selector.Select(intent)
	if len(scores) == 0 {
		result.Reason = "no candidate venues with market data"
		log.Printf("[SOR] Route FAILED %s: %s", intent.ClientOrderID, result.Reason)
		return result, fmt.Errorf("sor: %s", result.Reason)
	}

	winner, ok := Winner(scores)
	if !ok {
		result.Reason = "all venues disqualified"
		log.Printf("[SOR] Route FAILED %s: %s", intent.ClientOrderID, result.Reason)
		return result, fmt.Errorf("sor: %s", result.Reason)
	}
	result.WinningVenue = winner.VenueID

	emitRoute(ctx, r.store, intent.ClientOrderID, EventBestExecutionCalculated, intent.Symbol, intent.StrategyName, BestExecutionCalculatedPayload{
		ParentClientOrderID: intent.ClientOrderID,
		Symbol:              intent.Symbol,
		Side:                intent.Side,
		Quantity:            intent.Quantity,
		Scores:              scores,
		WinningVenue:        winner.VenueID,
		CalculatedAt:        time.Now().UTC(),
	})

	// Emit a VenueSelected event per ranked qualified venue (audit trail).
	for _, s := range scores {
		if s.Disqualified {
			continue
		}
		emitRoute(ctx, r.store, intent.ClientOrderID, EventVenueSelected, intent.Symbol, intent.StrategyName, VenueSelectedPayload{
			ParentClientOrderID: intent.ClientOrderID,
			Symbol:              intent.Symbol,
			VenueID:             s.VenueID,
			Rank:                s.Rank,
			Score:               s.Score,
			Reason:              s.Explanation,
			SelectedAt:          time.Now().UTC(),
		})
	}

	// 3. Build execution plan.
	plan, err := r.planner.Plan(ctx, PlanInput{
		ParentClientOrderID: intent.ClientOrderID,
		Symbol:              intent.Symbol,
		Side:                intent.Side,
		Quantity:            intent.Quantity,
		OrderType:           intent.OrderType,
		Algo:                intent.Algo,
		AlgoParams:          intent.AlgoParams,
		ForceSplitMethod:    intent.ForceSplit,
		ReferencePrice:      r.referencePrice(intent, winner),
	}, scores)
	if err != nil {
		result.Reason = err.Error()
		return result, fmt.Errorf("sor: plan: %w", err)
	}
	result.Plan = plan

	// 4. Coordinate execution across venues (with failover).
	report := r.coordinator.Execute(ctx, intent.ClientOrderID, intent.Symbol, intent.Side, intent.Quantity, plan.Children)
	result.Report = report
	result.Routed = report.FilledQty > 0

	if !result.Routed {
		result.Reason = "all child executions rejected"
	}

	log.Printf("[SOR] Routed %s: filled=%.4f/%.4f avg=%.2f venues=%v slippage=%.2fbps complete=%t",
		intent.ClientOrderID, report.FilledQty, intent.Quantity, report.AvgPrice,
		report.VenuesUsed, report.RealizedSlippageBps, report.Complete)

	return result, nil
}

func (r *SmartOrderRouter) referencePrice(intent OrderIntent, winner BestExecutionScore) float64 {
	if intent.LimitPrice > 0 {
		return intent.LimitPrice
	}
	if md, ok := r.registry.MarketData(winner.VenueID, intent.Symbol); ok {
		return md.Mid()
	}
	return 0
}
