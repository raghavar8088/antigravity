package liveengine

import (
	"context"
	"testing"
)

// A reconciliation count mismatch must be surfaced but must NOT stop the engine:
// the adoption sweep normally reconciles it on the next tick, and a fill landing
// between the two API reads is a false positive. Halting on it repeatedly
// stopped live trading for benign reasons.
func TestReconciliationMismatch_DoesNotStopTheEngine(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	enabled := false
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		KillSwitchActive:   func() bool { return false },
		SetEffectorEnabled: func(v bool) { enabled = v },
		AccountEquityUSD:   func(context.Context) (float64, error) { return 117.45, nil },
	})
	if err := c.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}

	c.NoteReconciliationMismatch("engine=0 delta=1")

	if !c.IsArmed() || !enabled {
		t.Fatal("a reconciliation mismatch must NOT stop the engine")
	}
	// ...but it must still be recorded so it cannot pass unnoticed.
	found := false
	for _, e := range c.Audit() {
		if e.Action == ActionReconMismatch {
			found = true
		}
	}
	if !found {
		t.Fatal("the mismatch must still be audited even though it does not halt")
	}
}

// The genuine safety stops remain one-way and unaffected.
func TestOtherSafetyStopsStillHalt(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	for name, stop := range map[string]func(*Controller){
		"daily_loss":    func(c *Controller) { c.OnDailyLossBreaker("down $20") },
		"feed_lost":     func(c *Controller) { c.OnPriceFeedLost("ws closed") },
		"reject_streak": func(c *Controller) { c.RecordReject("a"); c.RecordReject("b"); c.RecordReject("c") },
	} {
		t.Run(name, func(t *testing.T) {
			c := New(Hooks{
				IsConfigured:       func() bool { return true },
				KillSwitchActive:   func() bool { return false },
				SetEffectorEnabled: func(bool) {},
				AccountEquityUSD:   func(context.Context) (float64, error) { return 117.45, nil },
			})
			_ = c.Arm("operator", ArmConfirmationPhrase)
			stop(c)
			if c.IsArmed() {
				t.Fatalf("%s must still stop the engine", name)
			}
		})
	}
}
