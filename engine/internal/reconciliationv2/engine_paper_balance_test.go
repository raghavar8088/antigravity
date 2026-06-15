package reconciliationv2

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/positions"
)

func TestEngine_SkipsPaperRuntimeBalanceRecon(t *testing.T) {
	store := ledger.NewMemoryStore()
	posMgr := positions.NewManager()
	adapter := NewPositionManagerExchangeAdapter(posMgr, func() float64 { return 1_000_000 }, "btc-paper-1")
	reader := NewLedgerOMSStateReader(store, "btc-paper-1", LedgerOMSReaderConfig{InitialBalanceUSD: 1_000_000})

	engine := NewReconciliationEngine(adapter, reader, store, NewLedgerRepairTarget(store), nil, "btc-paper-1")
	oms, err := reader.GetOMSSnapshot(context.Background(), "btc-paper-1")
	if err != nil {
		t.Fatalf("GetOMSSnapshot: %v", err)
	}

	mismatches, err := engine.runBalance(context.Background(), oms)
	if err != nil {
		t.Fatalf("runBalance: %v", err)
	}
	if len(mismatches) != 0 {
		t.Fatalf("paper runtime should skip balance recon, got %d mismatches", len(mismatches))
	}

	entry, err := engine.RunDomain(context.Background(), DomainFull)
	if err != nil {
		t.Fatalf("RunDomain full: %v", err)
	}
	for _, m := range entry.Mismatches {
		if m.Domain == DomainBalance {
			t.Fatalf("full audit must not emit balance mismatches for paper runtime: %s", m.Message)
		}
	}
}

func TestEngine_BalanceReconStillRunsForLiveExchange(t *testing.T) {
	adapter := newStubAdapter("binance")
	adapter.balances = []AssetBalance{{Asset: "USDT", EquityUSD: 105000}}
	store := ledger.NewMemoryStore()
	reader := &stubOMSStateReader{snapshot: OMSSnapshot{
		Balance: OMSBalanceSnapshot{EquityUSD: 100000, AvailableUSD: 100000},
	}}
	engine := NewReconciliationEngine(adapter, reader, store, &stubRepairTarget{}, nil, "test-account")

	mismatches, err := engine.runBalance(context.Background(), reader.snapshot)
	if err != nil {
		t.Fatalf("runBalance: %v", err)
	}
	if len(mismatches) == 0 {
		t.Fatal("expected balance drift for live exchange adapter")
	}
}
