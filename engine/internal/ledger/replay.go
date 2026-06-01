package ledger

import (
	"context"
	"fmt"
	"sort"
)

// ReplayResult holds the segregated event slices produced by a full-account replay.
// Each slice contains only events of the matching AggregateType, in sequence order.
type ReplayResult struct {
	Orders          []Event // AggregateOrder
	Positions       []Event // AggregatePosition
	Risk            []Event // AggregateRisk
	Strategies      []Event // AggregateStrategy
	Exchange        []Event // AggregateExchange
	System          []Event // AggregateSystem
	Reconciliation  []Event // AggregateReconciliation
	MarketData      []Event // AggregateMarketData
	Account         []Event // AggregateAccount
	TotalEventCount int
}

// ReplayEverything replays all events for the given accountID from the Store,
// partitions them by AggregateType, and returns the result. The same sequence of
// calls always produces identical output regardless of machine or wall-clock time
// (determinism guarantee).
func ReplayEverything(ctx context.Context, store Store, accountID string) (ReplayResult, error) {
	events, err := store.ReplayAccount(ctx, accountID)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("ledger.ReplayEverything: %w", err)
	}

	// Ensure events are in creation-time order with sequence as tiebreaker.
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].CreatedAt.Equal(events[j].CreatedAt) {
			return events[i].SequenceNo < events[j].SequenceNo
		}
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})

	var res ReplayResult
	res.TotalEventCount = len(events)
	for _, e := range events {
		switch e.AggregateType {
		case AggregateOrder:
			res.Orders = append(res.Orders, e)
		case AggregatePosition:
			res.Positions = append(res.Positions, e)
		case AggregateRisk:
			res.Risk = append(res.Risk, e)
		case AggregateStrategy:
			res.Strategies = append(res.Strategies, e)
		case AggregateExchange:
			res.Exchange = append(res.Exchange, e)
		case AggregateSystem:
			res.System = append(res.System, e)
		case AggregateReconciliation:
			res.Reconciliation = append(res.Reconciliation, e)
		case AggregateMarketData:
			res.MarketData = append(res.MarketData, e)
		case AggregateAccount:
			res.Account = append(res.Account, e)
		}
	}
	return res, nil
}

// ReplayOrders replays all ORDER events for the given accountID.
// Returns events grouped by ClientOrderID in the order they were first seen.
func ReplayOrders(ctx context.Context, store Store, accountID string) (map[string][]Event, error) {
	result, err := ReplayEverything(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]Event)
	for _, e := range result.Orders {
		grouped[e.AggregateID] = append(grouped[e.AggregateID], e)
	}
	return grouped, nil
}

// ReplayPositions replays all POSITION events for the given accountID.
// Returns events grouped by PositionID.
func ReplayPositions(ctx context.Context, store Store, accountID string) (map[string][]Event, error) {
	result, err := ReplayEverything(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]Event)
	for _, e := range result.Positions {
		grouped[e.AggregateID] = append(grouped[e.AggregateID], e)
	}
	return grouped, nil
}

// ReplayRisk replays all RISK events for the given accountID.
func ReplayRisk(ctx context.Context, store Store, accountID string) ([]Event, error) {
	result, err := ReplayEverything(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	return result.Risk, nil
}

// ReplayStrategies replays all STRATEGY events for the given accountID.
// Returns events grouped by StrategyID.
func ReplayStrategies(ctx context.Context, store Store, accountID string) (map[string][]Event, error) {
	result, err := ReplayEverything(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	grouped := make(map[string][]Event)
	for _, e := range result.Strategies {
		grouped[e.AggregateID] = append(grouped[e.AggregateID], e)
	}
	return grouped, nil
}

// ReplaySystem replays all SYSTEM events for the given accountID.
func ReplaySystem(ctx context.Context, store Store, accountID string) ([]Event, error) {
	result, err := ReplayEverything(ctx, store, accountID)
	if err != nil {
		return nil, err
	}
	return result.System, nil
}

// ReplayAggregate is a convenience wrapper that replays events for a single
// aggregate by type + ID. It is equivalent to calling store.Replay directly but
// returns a typed error with the aggregate coordinates in the message.
func ReplayAggregate(ctx context.Context, store Store, aggregateType AggregateType, aggregateID string) ([]Event, error) {
	events, err := store.Replay(ctx, aggregateType, aggregateID)
	if err != nil {
		return nil, fmt.Errorf("ledger.ReplayAggregate(%s:%s): %w", aggregateType, aggregateID, err)
	}
	return events, nil
}

// VerifySequence checks that the given slice of events for one aggregate has no
// gaps and no duplicates in SequenceNo. Returns an error describing the first
// violation found. Used by the test suite and the crash-recovery routine.
func VerifySequence(events []Event) error {
	for i, e := range events {
		expected := int64(i + 1)
		if e.SequenceNo != expected {
			return fmt.Errorf("ledger.VerifySequence: aggregate %s:%s gap at index %d: expected seq %d got %d",
				e.AggregateType, e.AggregateID, i, expected, e.SequenceNo)
		}
	}
	return nil
}

// DetectOutOfOrder returns the indices of events that are out of creation-time
// order relative to their predecessor. A non-empty result indicates the event
// store received events out of order (e.g. from a network partition).
func DetectOutOfOrder(events []Event) []int {
	var bad []int
	for i := 1; i < len(events); i++ {
		if events[i].CreatedAt.Before(events[i-1].CreatedAt) {
			bad = append(bad, i)
		}
	}
	return bad
}

// DeduplicateEvents removes events with duplicate EventIDs, keeping the first
// occurrence. This is safe to call during replay of an untrusted event stream.
func DeduplicateEvents(events []Event) []Event {
	seen := make(map[string]struct{}, len(events))
	out := make([]Event, 0, len(events))
	for _, e := range events {
		if _, dup := seen[e.EventID]; dup {
			continue
		}
		seen[e.EventID] = struct{}{}
		out = append(out, e)
	}
	return out
}
