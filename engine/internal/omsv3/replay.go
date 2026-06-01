package omsv3

import (
	"context"
	"fmt"
	"log"

	"antigravity-engine/internal/ledger"
)

// ReplayOrdersForAccount replays all order events for an account and returns
// one OrderAggregate per distinct ClientOrderID. Events are filtered by
// AggregateType == ORDER. Corrupted aggregates are skipped with a log warning.
func ReplayOrdersForAccount(ctx context.Context, store ledger.Store, accountID string) ([]*OrderAggregate, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("omsv3.ReplayOrdersForAccount: %w", err)
	}

	byID := make(map[string][]ledger.Event, len(events)/4)
	for _, event := range events {
		if event.AggregateType == ledger.AggregateOrder {
			byID[event.AggregateID] = append(byID[event.AggregateID], event)
		}
	}

	result := make([]*OrderAggregate, 0, len(byID))
	for id, evts := range byID {
		agg, err := Replay(evts)
		if err != nil {
			log.Printf("[OMS V3] ReplayOrdersForAccount: skipping corrupted order %s: %v", id, err)
			continue
		}
		result = append(result, agg)
	}
	return result, nil
}

// ReplayPositionsForAccount replays all position events for an account and
// returns one PositionAggregate per distinct PositionID. Corrupted aggregates
// are skipped with a log warning.
func ReplayPositionsForAccount(ctx context.Context, store ledger.Store, accountID string) ([]*PositionAggregate, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("omsv3.ReplayPositionsForAccount: %w", err)
	}

	byID := make(map[string][]ledger.Event, len(events)/4)
	for _, event := range events {
		if event.AggregateType == ledger.AggregatePosition {
			byID[event.AggregateID] = append(byID[event.AggregateID], event)
		}
	}

	result := make([]*PositionAggregate, 0, len(byID))
	for id, evts := range byID {
		agg, err := ReplayPosition(evts)
		if err != nil {
			log.Printf("[OMS V3] ReplayPositionsForAccount: skipping corrupted position %s: %v", id, err)
			continue
		}
		result = append(result, agg)
	}
	return result, nil
}

// ReplayOpenPositionsForAccount is a convenience wrapper that replays all
// position aggregates for an account and returns only those currently open.
func ReplayOpenPositionsForAccount(ctx context.Context, store ledger.Store, accountID string) ([]*PositionAggregate, error) {
	all, err := ReplayPositionsForAccount(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	open := make([]*PositionAggregate, 0, len(all))
	for _, agg := range all {
		if agg.IsOpen() {
			open = append(open, agg)
		}
	}
	return open, nil
}
