package killswitch

import (
	"context"
	"testing"

	"antigravity-engine/internal/ledger"
)

func TestKillSwitchPersistsAndExecutesActions(t *testing.T) {
	store := ledger.NewMemoryStore()
	exec := &recordingExecutor{}
	service := NewService(store, exec, "acct-1")

	err := service.Trigger(context.Background(), Activation{
		Trigger: TriggerOMSDesync,
		Reason:  "OMS does not match exchange",
		Actions: []Action{ActionCancelOpenOrders, ActionFlattenPositions, ActionSendAlerts},
	})
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if !service.IsActive() {
		t.Fatal("expected kill switch active")
	}
	if exec.cancelled != 1 || exec.flattened != 1 || exec.alerted != 1 {
		t.Fatalf("expected all actions once, got cancel=%d flatten=%d alert=%d", exec.cancelled, exec.flattened, exec.alerted)
	}
	events, err := store.Replay(context.Background(), ledger.AggregateRisk, string(TriggerOMSDesync))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if len(events) != 1 || events[0].EventType != ledger.EventKillSwitchTriggered {
		t.Fatalf("expected kill switch event, got %#v", events)
	}
}

type recordingExecutor struct {
	cancelled int
	flattened int
	alerted   int
}

func (r *recordingExecutor) CancelOpenOrders(context.Context, string) error {
	r.cancelled++
	return nil
}

func (r *recordingExecutor) FlattenPositions(context.Context, string) error {
	r.flattened++
	return nil
}

func (r *recordingExecutor) SendAlert(context.Context, Activation) error {
	r.alerted++
	return nil
}
