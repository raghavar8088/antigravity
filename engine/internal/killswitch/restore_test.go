package killswitch

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
)

func TestRestoreFromLedger_AutoReleaseReconFalsePositive(t *testing.T) {
	store := ledger.NewMemoryStore()
	svc := NewService(store, nil, "btc-paper-1")
	ctx := context.Background()

	if err := svc.Trigger(ctx, Activation{
		Trigger: TriggerOMSDesync,
		Reason:  "reconciliation critical drift (balance): balance equity_drift — exchange=1000000 OMS=0",
		Actions: []Action{ActionBlockNewOrders},
	}); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if !svc.IsActive() {
		t.Fatal("expected active before restore")
	}

	if err := svc.RestoreFromLedger(ctx); err != nil {
		t.Fatalf("RestoreFromLedger: %v", err)
	}
	if svc.IsActive() {
		t.Fatal("expected auto-release of stale recon false positive")
	}

	events, err := store.ReplayAccount(ctx, "btc-paper-1")
	if err != nil {
		t.Fatalf("ReplayAccount: %v", err)
	}
	released := false
	for _, ev := range events {
		if ev.EventType == ledger.EventKillSwitchReleased {
			released = true
		}
	}
	if !released {
		t.Fatal("expected EventKillSwitchReleased after auto-heal")
	}
}

func TestRestoreFromLedger_KeepsLegitimateActive(t *testing.T) {
	store := ledger.NewMemoryStore()
	svc := NewService(store, nil, "btc-paper-1")
	ctx := context.Background()

	if err := svc.Trigger(ctx, Activation{
		Trigger: TriggerDailyLoss,
		Reason:  "daily loss breach exceeded 3%",
		Actions: []Action{ActionBlockNewOrders},
	}); err != nil {
		t.Fatalf("trigger: %v", err)
	}

	if err := svc.RestoreFromLedger(ctx); err != nil {
		t.Fatalf("RestoreFromLedger: %v", err)
	}
	if !svc.IsActive() {
		t.Fatal("legitimate daily loss kill switch must remain active")
	}
}
