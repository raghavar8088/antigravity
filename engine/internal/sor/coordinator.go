package sor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/ledger"
)

// Fill is the realized execution result of one child order.
type Fill struct {
	ChildOrderID        string
	ParentClientOrderID string
	VenueID             VenueID
	Symbol              string
	Side                string
	RequestedQty        float64
	FilledQty           float64
	AvgPrice            float64
	FeesUSD             float64
	SlippageBps         float64
	LatencyMs           float64
	Status              string // FILLED | PARTIAL | REJECTED
	RejectReason        string
	FilledAt            time.Time
}

// VenueAdapter is the SOR's execution interface to a single venue.
// Implementations: ExecutionVenueAdapter (live, wraps execution.ExchangeAdapter)
// and SimulatedVenueAdapter (backtest).
type VenueAdapter interface {
	VenueID() VenueID
	// Execute places a child order and returns the realized fill.
	Execute(ctx context.Context, child ChildOrder) (Fill, error)
	// Cancel cancels an in-flight child order.
	Cancel(ctx context.Context, childOrderID, symbol string) error
}

// ── Live bridge: wraps execution.ExchangeAdapter ──────────────────────────────

// ExecutionVenueAdapter bridges an existing execution.ExchangeAdapter into the
// SOR VenueAdapter interface. It preserves the OMS authority boundary: the
// adapter only places orders; OMS v3 + reconciliation own fill state. The Fill
// returned here is the best-effort acknowledgement; the authoritative fill is
// reconciled later by the Reconciliation Authority.
type ExecutionVenueAdapter struct {
	id      VenueID
	adapter execution.ExchangeAdapter
}

// NewExecutionVenueAdapter wraps an execution.ExchangeAdapter.
func NewExecutionVenueAdapter(id VenueID, adapter execution.ExchangeAdapter) *ExecutionVenueAdapter {
	return &ExecutionVenueAdapter{id: id, adapter: adapter}
}

func (a *ExecutionVenueAdapter) VenueID() VenueID { return a.id }

func (a *ExecutionVenueAdapter) Execute(ctx context.Context, child ChildOrder) (Fill, error) {
	start := time.Now()
	ack, err := a.adapter.PlaceOrder(ctx, execution.OrderRequest{
		ClientOrderID: child.ChildOrderID,
		Symbol:        child.Symbol,
		Side:          child.Side,
		Quantity:      child.Quantity,
		OrderType:     child.OrderType,
		LimitPrice:    child.LimitPrice,
		RequestedAt:   time.Now().UTC(),
	})
	latency := float64(time.Since(start).Milliseconds())

	fill := Fill{
		ChildOrderID:        child.ChildOrderID,
		ParentClientOrderID: child.ParentClientOrderID,
		VenueID:             a.id,
		Symbol:              child.Symbol,
		Side:                child.Side,
		RequestedQty:        child.Quantity,
		LatencyMs:           latency,
		FilledAt:            time.Now().UTC(),
	}
	if err != nil || ack.Status == "REJECTED" {
		fill.Status = "REJECTED"
		fill.RejectReason = ack.RejectReason
		if err != nil && fill.RejectReason == "" {
			fill.RejectReason = err.Error()
		}
		return fill, err
	}

	// Acknowledged. Best-effort fill at the reference price; the authoritative
	// fill price/qty is reconciled by the Reconciliation Authority from the
	// exchange order-status feed.
	fill.Status = "FILLED"
	fill.FilledQty = child.Quantity
	fill.AvgPrice = child.ReferencePrice
	return fill, nil
}

func (a *ExecutionVenueAdapter) Cancel(ctx context.Context, childOrderID, symbol string) error {
	return a.adapter.CancelOrder(ctx, childOrderID, symbol)
}

// ── Execution Coordinator ─────────────────────────────────────────────────────

// ExecutionReport aggregates all child fills for a parent order.
type ExecutionReport struct {
	ParentClientOrderID string
	Symbol              string
	Side                string
	RequestedQty        float64
	FilledQty           float64
	AvgPrice            float64
	TotalFeesUSD        float64
	RealizedSlippageBps float64
	VenuesUsed          []VenueID
	Fills               []Fill
	Rejections          int
	DurationMs          int64
	Complete            bool // FilledQty >= RequestedQty (within tolerance)
	StartedAt           time.Time
	CompletedAt         time.Time
}

// ExecutionCoordinator dispatches child orders to venue adapters, applies
// failover on rejection, records fills/slippage, and emits lifecycle events.
type ExecutionCoordinator struct {
	mu       sync.RWMutex
	adapters map[VenueID]VenueAdapter
	registry *VenueRegistry
	slippage *SlippageEngine
	health   *ExchangeHealthEngine
	failover *FailoverController
	store    ledger.Store

	// FillTolerance is the acceptable shortfall fraction to mark complete.
	FillTolerance float64
}

// NewExecutionCoordinator constructs a coordinator.
func NewExecutionCoordinator(
	registry *VenueRegistry,
	slippage *SlippageEngine,
	health *ExchangeHealthEngine,
	failover *FailoverController,
	store ledger.Store,
) *ExecutionCoordinator {
	return &ExecutionCoordinator{
		adapters:      make(map[VenueID]VenueAdapter),
		registry:      registry,
		slippage:      slippage,
		health:        health,
		failover:      failover,
		store:         store,
		FillTolerance: 0.01,
	}
}

// RegisterAdapter binds a venue adapter for execution.
func (c *ExecutionCoordinator) RegisterAdapter(a VenueAdapter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.adapters[a.VenueID()] = a
}

func (c *ExecutionCoordinator) adapter(id VenueID) (VenueAdapter, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := c.adapters[id]
	return a, ok
}

// Execute runs all child orders for a parent and returns an aggregated report.
// Child orders execute in sequence-index order (deterministic for replay/tests).
func (c *ExecutionCoordinator) Execute(ctx context.Context, parentID, symbol, side string, requestedQty float64, children []ChildOrder) ExecutionReport {
	start := time.Now()
	report := ExecutionReport{
		ParentClientOrderID: parentID,
		Symbol:              symbol,
		Side:                side,
		RequestedQty:        requestedQty,
		StartedAt:           start,
	}

	emitRoute(ctx, c.store, parentID, EventExecutionStarted, symbol, "", ExecutionStartedPayload{
		ParentClientOrderID: parentID,
		ChildCount:          len(children),
		StartedAt:           start,
	})

	venuesSeen := make(map[VenueID]bool)
	var notional, filledQty, fees float64

	for _, child := range children {
		fill := c.executeChildWithFailover(ctx, child, symbol)
		report.Fills = append(report.Fills, fill)

		if fill.Status == "REJECTED" {
			report.Rejections++
			emitRoute(ctx, c.store, parentID, EventVenueRejected, symbol, "", VenueRejectedPayload{
				ParentClientOrderID: parentID,
				ChildOrderID:        child.ChildOrderID,
				VenueID:             fill.VenueID,
				Reason:              fill.RejectReason,
				RejectedAt:          time.Now().UTC(),
			})
			continue
		}

		venuesSeen[fill.VenueID] = true
		filledQty += fill.FilledQty
		notional += fill.FilledQty * fill.AvgPrice
		fees += fill.FeesUSD

		// Record realized slippage vs reference.
		realizedSlip := slippageVsRef(child.ReferencePrice, fill.AvgPrice, side)
		fill.SlippageBps = realizedSlip
		c.registry.RecordRouteOutcome(fill.VenueID, true, realizedSlip, fill.LatencyMs)
		if c.slippage != nil {
			expBps := 0.0
			c.slippage.RecordRealized(ctx, fill.VenueID, symbol, side, fill.FilledQty, expBps, realizedSlip)
		}
		if c.health != nil {
			c.health.Observe(ctx, fill.VenueID, SignalSuccess, fill.LatencyMs)
		}

		emitRoute(ctx, c.store, parentID, EventChildFilled, symbol, "", ChildFilledPayload{
			ParentClientOrderID: parentID,
			ChildOrderID:        child.ChildOrderID,
			VenueID:             fill.VenueID,
			Symbol:              symbol,
			Side:                side,
			FilledQty:           fill.FilledQty,
			AvgPrice:            fill.AvgPrice,
			FeesUSD:             fill.FeesUSD,
			SlippageBps:         realizedSlip,
			FilledAt:            fill.FilledAt,
		})
	}

	report.FilledQty = filledQty
	report.TotalFeesUSD = fees
	if filledQty > 0 {
		report.AvgPrice = notional / filledQty
	}
	for v := range venuesSeen {
		report.VenuesUsed = append(report.VenuesUsed, v)
	}
	report.Complete = filledQty >= requestedQty*(1-c.FillTolerance)
	report.CompletedAt = time.Now().UTC()
	report.DurationMs = report.CompletedAt.Sub(start).Milliseconds()

	// Aggregate realized slippage vs the parent reference (first child's ref).
	parentRef := 0.0
	if len(children) > 0 {
		parentRef = children[0].ReferencePrice
	}
	report.RealizedSlippageBps = slippageVsRef(parentRef, report.AvgPrice, side)

	emitRoute(ctx, c.store, parentID, EventExecutionCompleted, symbol, "", ExecutionCompletedPayload{
		ParentClientOrderID: parentID,
		Symbol:              symbol,
		RequestedQty:        requestedQty,
		FilledQty:           filledQty,
		AvgPrice:            report.AvgPrice,
		TotalFeesUSD:        fees,
		RealizedSlippageBps: report.RealizedSlippageBps,
		VenuesUsed:          report.VenuesUsed,
		DurationMs:          report.DurationMs,
		CompletedAt:         report.CompletedAt,
	})

	return report
}

// executeChildWithFailover attempts the child on its assigned venue and, on
// rejection, reroutes to the next-best healthy venue via the failover controller.
func (c *ExecutionCoordinator) executeChildWithFailover(ctx context.Context, child ChildOrder, symbol string) Fill {
	attemptVenue := child.VenueID
	var lastFill Fill

	for attempt := 0; attempt < c.failover.MaxAttempts; attempt++ {
		adapter, ok := c.adapter(attemptVenue)
		if !ok {
			lastFill = Fill{
				ChildOrderID:        child.ChildOrderID,
				ParentClientOrderID: child.ParentClientOrderID,
				VenueID:             attemptVenue,
				Symbol:              symbol,
				Side:                child.Side,
				RequestedQty:        child.Quantity,
				Status:              "REJECTED",
				RejectReason:        "no adapter registered for venue",
				FilledAt:            time.Now().UTC(),
			}
		} else {
			childOnVenue := child
			childOnVenue.VenueID = attemptVenue
			fill, err := adapter.Execute(ctx, childOnVenue)
			lastFill = fill
			if err == nil && fill.Status != "REJECTED" {
				return fill
			}
			// Record failure signals.
			c.registry.RecordRouteOutcome(attemptVenue, false, 0, fill.LatencyMs)
			if c.health != nil {
				c.health.Observe(ctx, attemptVenue, SignalOrderReject, fill.LatencyMs)
			}
		}

		// Pick a failover venue.
		next, ok := c.failover.NextVenue(symbol, attemptVenue, child.Side, child.Quantity)
		if !ok {
			break
		}
		emitVenue(ctx, c.store, attemptVenue, EventVenueFailed, VenueFailedPayload{
			VenueID:     attemptVenue,
			Reason:      "child_order_rejected_failover",
			HealthScore: c.healthScore(attemptVenue),
			FailedAt:    time.Now().UTC(),
		})
		attemptVenue = next
	}
	return lastFill
}

func (c *ExecutionCoordinator) healthScore(v VenueID) float64 {
	if c.health == nil {
		return 0
	}
	return c.health.Score(v)
}

// slippageVsRef returns signed slippage in bps (positive = adverse) of an
// execution price vs a reference price.
func slippageVsRef(ref, exec float64, side string) float64 {
	if ref <= 0 || exec <= 0 {
		return 0
	}
	var diff float64
	if isBuy(side) {
		diff = exec - ref // paying more than ref is adverse
	} else {
		diff = ref - exec // receiving less than ref is adverse
	}
	return diff / ref * 10000
}

// CancelParent cancels all in-flight children for a parent order.
func (c *ExecutionCoordinator) CancelParent(ctx context.Context, parentID, symbol string, children []ChildOrder, reason string, filledQty float64) {
	for _, child := range children {
		if a, ok := c.adapter(child.VenueID); ok {
			_ = a.Cancel(ctx, child.ChildOrderID, symbol)
		}
	}
	emitRoute(ctx, c.store, parentID, EventExecutionCancelled, symbol, "", ExecutionCancelledPayload{
		ParentClientOrderID: parentID,
		Reason:              reason,
		FilledQty:           filledQty,
		CancelledAt:         time.Now().UTC(),
	})
}

var _ = fmt.Sprintf
