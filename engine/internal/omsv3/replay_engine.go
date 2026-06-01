package omsv3

import (
	"context"
	"fmt"

	"antigravity-engine/internal/ledger"
)

// FullReplayResult holds every rebuilt aggregate for one account.
// All state was reconstructed deterministically from ledger events alone;
// no external database queries were made during the rebuild.
type FullReplayResult struct {
	Orders     map[string]*OrderAggregate    // clientOrderID → aggregate
	Positions  map[string]*PositionAggregate // positionID → aggregate
	Strategies map[string]*StrategyAggregate // strategyID → aggregate
	System     *SystemAggregate
	EventCount int
}

// ReplayAll rebuilds every aggregate for the given accountID from the ledger store.
// This is the primary crash-recovery entry point. Call it at engine boot to
// reconstruct the full trading state without touching any external DB.
//
// Boot sequence:
//
//	ReplayAll → populate in-memory caches → resume trading
func ReplayAll(ctx context.Context, store ledger.Store, accountID string) (FullReplayResult, error) {
	raw, err := ledger.ReplayEverything(ctx, store, accountID)
	if err != nil {
		return FullReplayResult{}, fmt.Errorf("omsv3.ReplayAll: %w", err)
	}

	result := FullReplayResult{
		Orders:     make(map[string]*OrderAggregate),
		Positions:  make(map[string]*PositionAggregate),
		Strategies: make(map[string]*StrategyAggregate),
		EventCount: raw.TotalEventCount,
	}

	// ── Orders ───────────────────────────────────────────────────────────────
	for _, e := range raw.Orders {
		agg, ok := result.Orders[e.AggregateID]
		if !ok {
			agg = NewOrderAggregate(e.AggregateID)
			result.Orders[e.AggregateID] = agg
		}
		if err := agg.ApplyEvent(e); err != nil {
			return result, fmt.Errorf("omsv3.ReplayAll: order %s: %w", e.AggregateID, err)
		}
	}

	// ── Positions ─────────────────────────────────────────────────────────────
	for _, e := range raw.Positions {
		agg, ok := result.Positions[e.AggregateID]
		if !ok {
			agg = NewPositionAggregate(e.AggregateID)
			result.Positions[e.AggregateID] = agg
		}
		if err := agg.ApplyEvent(e); err != nil {
			return result, fmt.Errorf("omsv3.ReplayAll: position %s: %w", e.AggregateID, err)
		}
	}

	// ── Strategies ────────────────────────────────────────────────────────────
	for _, e := range raw.Strategies {
		agg, ok := result.Strategies[e.AggregateID]
		if !ok {
			agg = NewStrategyAggregate(e.AggregateID)
			result.Strategies[e.AggregateID] = agg
		}
		// Allocation-change events don't trigger a state transition.
		if e.EventType == ledger.EventStrategyAllocationChanged {
			if err := agg.ApplyAllocationChange(e); err != nil {
				return result, fmt.Errorf("omsv3.ReplayAll: strategy allocation %s: %w", e.AggregateID, err)
			}
			continue
		}
		if err := agg.ApplyEvent(e); err != nil {
			return result, fmt.Errorf("omsv3.ReplayAll: strategy %s: %w", e.AggregateID, err)
		}
	}

	// ── System ────────────────────────────────────────────────────────────────
	if len(raw.System) > 0 {
		sys, err := ReplaySystemAggregate(raw.System)
		if err != nil {
			return result, fmt.Errorf("omsv3.ReplayAll: system: %w", err)
		}
		result.System = sys
	} else {
		result.System = NewSystemAggregate(accountID)
	}

	return result, nil
}

// ReplayOpenOrders returns only orders that are not in a terminal state.
func ReplayOpenOrders(ctx context.Context, store ledger.Store, accountID string) ([]*OrderAggregate, error) {
	result, err := ReplayAll(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	var open []*OrderAggregate
	for _, agg := range result.Orders {
		if agg.State != StateFilled && agg.State != StateCancelled && agg.State != StateRejected {
			open = append(open, agg)
		}
	}
	return open, nil
}

// ReplayOpenPositions returns only positions that are currently open or reduced.
func ReplayOpenPositions(ctx context.Context, store ledger.Store, accountID string) ([]*PositionAggregate, error) {
	result, err := ReplayAll(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	var open []*PositionAggregate
	for _, agg := range result.Positions {
		if agg.IsOpen() {
			open = append(open, agg)
		}
	}
	return open, nil
}

// ReplayActiveStrategies returns strategies that are currently active (enabled/promoted/resumed).
func ReplayActiveStrategies(ctx context.Context, store ledger.Store, accountID string) ([]*StrategyAggregate, error) {
	result, err := ReplayAll(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	var active []*StrategyAggregate
	for _, agg := range result.Strategies {
		if agg.IsActive() {
			active = append(active, agg)
		}
	}
	return active, nil
}

// ReplayPnL rebuilds the PnL projection from all POSITION_CLOSED events.
// Result is identical to BuildPnLProjection(accountEvents) but uses the
// typed replay path for consistency.
func ReplayPnL(ctx context.Context, store ledger.Store, accountID string) (PnLProjection, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return PnLProjection{}, fmt.Errorf("omsv3.ReplayPnL: %w", err)
	}
	return BuildPnLProjection(events), nil
}

// ReplayExposure rebuilds the current market exposure from open position events.
func ReplayExposure(ctx context.Context, store ledger.Store, accountID string) (ExposureProjection, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return ExposureProjection{}, fmt.Errorf("omsv3.ReplayExposure: %w", err)
	}
	return BuildExposureProjection(events), nil
}

// ReplayDashboard rebuilds all projections needed by the trading dashboard.
func ReplayDashboard(ctx context.Context, store ledger.Store, accountID string) (DashboardProjection, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return DashboardProjection{}, fmt.Errorf("omsv3.ReplayDashboard: %w", err)
	}
	return BuildDashboardProjection(events), nil
}
