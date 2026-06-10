package reconciliationv2

import (
	"context"
	"testing"
	"time"

	"antigravity-engine/internal/killswitch"
	"antigravity-engine/internal/ledger"
	"antigravity-engine/internal/omsv3"
	"antigravity-engine/internal/positions"
	"antigravity-engine/internal/strategy"
)

// TestPostFix_RuntimeVsLedger_NoCriticalBalanceDrift proves the equity projection
// fix prevents false CRITICAL kill-switch triggers at $1M paper baseline.
func TestPostFix_RuntimeVsLedger_NoCriticalBalanceDrift(t *testing.T) {
	store := ledger.NewMemoryStore()
	const initial = 1_000_000.0
	markPrice := 65000.0

	reader := NewLedgerOMSStateReader(store, "btc-paper-1", LedgerOMSReaderConfig{
		InitialBalanceUSD: initial,
		MarkPriceUSD:      func() float64 { return markPrice },
	})
	omsSnap, err := reader.GetOMSSnapshot(context.Background(), "btc-paper-1")
	if err != nil {
		t.Fatalf("GetOMSSnapshot: %v", err)
	}

	posMgr := positions.NewManager()
	adapter := NewPositionManagerExchangeAdapter(posMgr, func() float64 { return initial }, "btc-paper-1")
	runtimeBalances, err := adapter.GetBalances(context.Background())
	if err != nil {
		t.Fatalf("GetBalances: %v", err)
	}

	det := BalanceDriftDetector{}
	mismatches := det.Detect(runtimeBalances, omsSnap.Balance, time.Now())
	for _, m := range mismatches {
		if m.Severity == SeverityCritical {
			t.Fatalf("unexpected CRITICAL balance mismatch: %s", m.Message)
		}
	}
}

// TestPostFix_PositionSideNormalization_BuyVsLong proves BUY ledger positions
// reconcile against LONG runtime positions without ghost/missing CRITICAL alerts.
func TestPostFix_PositionSideNormalization_BuyVsLong(t *testing.T) {
	store := ledger.NewMemoryStore()
	ctx := context.Background()
	accountID := "btc-paper-1"

	posID := "pos-cert-001"
	openEv, err := ledger.NewEvent(ledger.NewEventInput{
		AggregateType: ledger.AggregatePosition,
		AggregateID:   posID,
		EventType:     ledger.EventPositionOpened,
		AccountID:     accountID,
		Symbol:        "BTC-USD",
		Payload: omsv3.PositionOpenedPayload{
			PositionID:  posID,
			Symbol:      "BTC-USD",
			Side:        "BUY",
			EntryPrice:  65000,
			Quantity:    0.1,
			NotionalUSD: 6500,
		},
		Source: "certification",
	})
	if err != nil {
		t.Fatalf("NewEvent: %v", err)
	}
	if _, err := store.Append(ctx, openEv); err != nil {
		t.Fatalf("Append: %v", err)
	}

	reader := NewLedgerOMSStateReader(store, accountID, LedgerOMSReaderConfig{
		InitialBalanceUSD: 1_000_000,
		MarkPriceUSD:      func() float64 { return 65000 },
	})
	omsSnap, err := reader.GetOMSSnapshot(ctx, accountID)
	if err != nil {
		t.Fatalf("GetOMSSnapshot: %v", err)
	}
	if len(omsSnap.Positions) != 1 {
		t.Fatalf("expected 1 OMS position, got %d", len(omsSnap.Positions))
	}
	if omsSnap.Positions[0].Side != "LONG" {
		t.Fatalf("OMS side=%s want LONG", omsSnap.Positions[0].Side)
	}

	posMgr := positions.NewManager()
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    0.1,
		StopLossPct:   0.18,
		TakeProfitPct: 0.50,
	}
	if _, err := posMgr.OpenPosition(sig, 65000, "cert-strat"); err != nil {
		t.Fatalf("OpenPosition: %v", err)
	}
	adapter := NewPositionManagerExchangeAdapter(posMgr, func() float64 { return 1_000_000 }, accountID)
	exPos, err := adapter.GetPositions(ctx)
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}

	det := PositionDriftDetector{}
	mismatches := det.Detect(exPos, omsSnap.Positions, time.Now())
	for _, m := range mismatches {
		if m.Severity == SeverityCritical {
			t.Fatalf("unexpected CRITICAL position mismatch: %s — %s", m.Type, m.Message)
		}
	}
}

// TestPostFix_KillSwitchHook_NoTriggerOnCleanPaperState proves reconciliation
// does not trigger kill switch when runtime and ledger agree at $1M baseline.
func TestPostFix_KillSwitchHook_NoTriggerOnCleanPaperState(t *testing.T) {
	store := ledger.NewMemoryStore()
	ks := killswitch.NewService(store, nil, "btc-paper-1")

	posMgr := positions.NewManager()
	adapter := NewPositionManagerExchangeAdapter(posMgr, func() float64 { return 1_000_000 }, "btc-paper-1")
	reader := NewLedgerOMSStateReader(store, "btc-paper-1", LedgerOMSReaderConfig{
		InitialBalanceUSD: 1_000_000,
		MarkPriceUSD:      func() float64 { return 65000 },
	})
	engine := NewReconciliationEngine(adapter, reader, store, NewLedgerRepairTarget(store), nil, "btc-paper-1")

	entry, err := engine.RunDomain(context.Background(), DomainFull)
	if err != nil {
		t.Fatalf("RunDomain: %v", err)
	}

	hook := CriticalDriftKillSwitchHook(ks)
	hook(context.Background(), DomainFull, entry)

	if ks.IsActive() {
		t.Fatalf("kill switch should remain inactive on clean state, mismatches=%d score=%.2f",
			entry.MismatchCount, entry.DriftScore)
	}
}
