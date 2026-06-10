package reconciliationv2

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/positions"
)

func TestLedgerOMSStateReader_GetOMSSnapshot_Empty(t *testing.T) {
	store := ledger.NewMemoryStore()
	reader := NewLedgerOMSStateReader(store, "btc-paper-1")
	snap, err := reader.GetOMSSnapshot(context.Background(), "btc-paper-1")
	if err != nil {
		t.Fatalf("GetOMSSnapshot: %v", err)
	}
	if snap.AccountID != "btc-paper-1" {
		t.Fatalf("accountID=%s", snap.AccountID)
	}
	if len(snap.Positions) != 0 {
		t.Fatalf("expected no positions, got %d", len(snap.Positions))
	}
}

func TestPositionManagerExchangeAdapter_GetPositions(t *testing.T) {
	posMgr := positions.NewManager()
	adapter := NewPositionManagerExchangeAdapter(posMgr, func() float64 { return 1_000_000 }, "btc-paper-1")
	positions, err := adapter.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected empty positions, got %d", len(positions))
	}
	if adapter.Name() != "engine-runtime" {
		t.Fatalf("name=%s", adapter.Name())
	}
}
