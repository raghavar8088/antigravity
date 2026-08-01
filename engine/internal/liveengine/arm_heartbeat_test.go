package liveengine

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// The freshness guard exists to stop a box that has been DOWN for hours from
// waking up and trading on stale intent. It was measuring time-since-arming
// instead, so an engine armed 12h ago and running continuously refused to resume
// after a 60-second redeploy — the opposite of the intent. These tests pin both
// halves: a live engine stays resumable, a long outage still does not.

// clockedCtrl builds a controller whose clock the test moves by writing to *clk.
func clockedCtrl(t *testing.T, clk *time.Time) (*Controller, *bool) {
	t.Helper()
	enabled := false
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		KillSwitchActive:   func() bool { return false },
		SetEffectorEnabled: func(v bool) { enabled = v },
		AccountEquityUSD:   func(context.Context) (float64, error) { return 117.45, nil },
		Now:                func() time.Time { return *clk },
	})
	return c, &enabled
}

func readPersistedArmState(t *testing.T) persistedArmState {
	t.Helper()
	data, err := os.ReadFile(armStatePath())
	if err != nil {
		t.Fatalf("read arm state: %v", err)
	}
	var st persistedArmState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("unmarshal arm state: %v", err)
	}
	return st
}

func TestArmStateHeartbeat_KeepsLongRunningArmedEngineResumable(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())

	armed := time.Date(2026, 7, 28, 3, 0, 0, 0, time.UTC)
	clk := armed
	c, _ := clockedCtrl(t, &clk)
	if err := c.Arm("owner", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}

	// Twelve hours later the engine is still armed and still running. The
	// heartbeat tick re-saves, so the state is fresh despite the old arm time.
	clk = armed.Add(12 * time.Hour)
	c.mu.Lock()
	c.persistArmStateLocked()
	c.mu.Unlock()

	st := readPersistedArmState(t)
	if !st.Armed {
		t.Fatal("heartbeat must preserve the ARMED flag")
	}
	if !st.SavedAt.Equal(clk) {
		t.Errorf("SavedAt must track last-known-alive, got %s want %s", st.SavedAt, clk)
	}
	if !st.ArmedAt.Equal(armed) {
		t.Errorf("ArmedAt must stay the original arm time, got %s", st.ArmedAt)
	}

	// A fresh process restarting moments later must resume — this is the exact
	// case that failed in production and stopped live trading after a redeploy.
	clk2 := clk.Add(time.Minute)
	c2, enabled2 := clockedCtrl(t, &clk2)
	if !c2.RestoreArmState(context.Background()) {
		t.Fatal("a continuously-armed engine must resume after a short restart")
	}
	if !c2.IsArmed() || !*enabled2 {
		t.Fatal("resume must arm the controller and enable the effector")
	}
}

// The guard must still refuse a genuinely stale state: a box down longer than
// armStateMaxAge does not resume trading on its own.
func TestArmStateHeartbeat_StillRefusesAfterLongOutage(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())

	saved := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	clk := saved
	c, _ := clockedCtrl(t, &clk)
	if err := c.Arm("owner", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}

	// The box was down past the max age — no heartbeat happened in between.
	clk2 := saved.Add(armStateMaxAge + time.Hour)
	c2, enabled2 := clockedCtrl(t, &clk2)
	if c2.RestoreArmState(context.Background()) {
		t.Fatal("a state older than armStateMaxAge must not auto-resume")
	}
	if c2.IsArmed() || *enabled2 {
		t.Fatal("engine must stay disarmed after a long outage")
	}
}

// The heartbeat only rewrites an ARMED state. A safety stop must keep its reason
// and must never be resurrected by a later tick.
func TestArmStateHeartbeat_DoesNotRewriteSafetyStoppedState(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())

	at := time.Date(2026, 7, 28, 6, 0, 0, 0, time.UTC)
	clk := at
	c, _ := clockedCtrl(t, &clk)
	if err := c.Arm("owner", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}
	c.Disarm("system", "daily_loss_breached")

	st := readPersistedArmState(t)
	if st.Armed {
		t.Fatal("a disarmed state must never be saved as armed")
	}
	if st.DisarmReason == "" {
		t.Error("the safety-stop reason must survive for the resume guard to see it")
	}

	clk2 := at.Add(time.Minute)
	c2, enabled2 := clockedCtrl(t, &clk2)
	if c2.RestoreArmState(context.Background()) {
		t.Fatal("a safety-stopped state must never auto-resume")
	}
	if c2.IsArmed() || *enabled2 {
		t.Fatal("engine must stay disarmed after a safety stop")
	}
}
