package killswitch

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
)

func TestKillSwitchDisabled_IgnoresTriggerAndReportsInactive(t *testing.T) {
	store := ledger.NewMemoryStore()
	exec := &recordingExecutor{}
	service := NewService(store, exec, "acct-1")
	service.SetEnabled(false)

	if err := service.Trigger(context.Background(), Activation{
		Trigger: TriggerOMSDesync,
		Reason:  "should be ignored",
	}); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if service.IsActive() {
		t.Fatal("disabled kill switch must not become active")
	}
	if exec.cancelled != 0 {
		t.Fatal("disabled kill switch must not execute actions")
	}
}

func TestKillSwitchDisableAndRelease(t *testing.T) {
	store := ledger.NewMemoryStore()
	service := NewService(store, nil, "acct-1")
	service.SetEnabled(true)
	ctx := context.Background()

	if err := service.Trigger(ctx, Activation{
		Trigger: TriggerManualOperator,
		Reason:  "test halt",
		Actions: []Action{ActionBlockNewOrders},
	}); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if err := service.DisableAndRelease(ctx); err != nil {
		t.Fatalf("DisableAndRelease: %v", err)
	}
	if service.IsEnabled() || service.IsActive() {
		t.Fatal("expected disabled and inactive after DisableAndRelease")
	}
}
