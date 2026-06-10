package reconciliationv2

import (
	"context"
	"fmt"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
)

// LedgerRepairTarget rebuilds OMS projections by replaying ledger events.
type LedgerRepairTarget struct {
	store ledger.Store
}

// NewLedgerRepairTarget creates a repair target backed by the shared ledger store.
func NewLedgerRepairTarget(store ledger.Store) *LedgerRepairTarget {
	return &LedgerRepairTarget{store: store}
}

// RebuildProjections replays all account events to validate projection rebuild path.
func (t *LedgerRepairTarget) RebuildProjections(ctx context.Context, accountID string) error {
	if t.store == nil {
		return fmt.Errorf("ledger repair target: store is nil")
	}
	events, err := t.store.ReplayAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("ledger repair target: replay account: %w", err)
	}
	_ = omsv3.BuildOrderProjections(events)
	_ = omsv3.BuildPositionProjections(events)
	_ = omsv3.BuildPnLProjection(events)
	_ = omsv3.BuildExposureProjection(events)
	return nil
}

// RebuildAggregate replays a single aggregate stream.
func (t *LedgerRepairTarget) RebuildAggregate(ctx context.Context, aggregateType, aggregateID string) error {
	if t.store == nil {
		return fmt.Errorf("ledger repair target: store is nil")
	}
	events, err := t.store.Replay(ctx, ledger.AggregateType(aggregateType), aggregateID)
	if err != nil {
		return fmt.Errorf("ledger repair target: replay aggregate: %w", err)
	}
	switch ledger.AggregateType(aggregateType) {
	case ledger.AggregateOrder:
		if _, err := omsv3.Replay(events); err != nil {
			return err
		}
	case ledger.AggregatePosition:
		if _, err := omsv3.ReplayPosition(events); err != nil {
			return err
		}
	}
	return nil
}
