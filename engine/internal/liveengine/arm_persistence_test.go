package liveengine

import (
	"context"
	"testing"
	"time"
)

func armCtrl(t *testing.T, kill bool) (*Controller, *bool) {
	t.Helper()
	enabled := false
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		KillSwitchActive:   func() bool { return kill },
		SetEffectorEnabled: func(v bool) { enabled = v },
		AccountEquityUSD:   func(context.Context) (float64, error) { return 117.45, nil },
	})
	return c, &enabled
}

// A deliberate human ON must survive a restart, so a deploy does not silently
// stop live trading.
func TestArmState_ResumesDeliberateOnAfterRestart(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())

	c1, _ := armCtrl(t, false)
	if err := c1.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}

	// Fresh process.
	c2, enabled2 := armCtrl(t, false)
	if c2.IsArmed() {
		t.Fatal("a new controller must start off before restore")
	}
	if !c2.RestoreArmState(context.Background()) {
		t.Fatal("expected the deliberate ON state to resume")
	}
	if !c2.IsArmed() || !*enabled2 {
		t.Fatal("resume must arm the controller and enable the effector")
	}
	var restored bool
	for _, e := range c2.Audit() {
		if e.Action == ActionArmRestored {
			restored = true
		}
	}
	if !restored {
		t.Fatal("resume must be audited as ARM_RESTORED")
	}
}

// Auto-disarm stays one-way: a safety stop must never resume by itself.
func TestArmState_NeverResumesAfterSafetyStop(t *testing.T) {
	for _, tc := range []struct {
		name string
		stop func(c *Controller)
	}{
		{"daily_loss", func(c *Controller) { c.OnDailyLossBreaker("down $20") }},
		{"reconciliation", func(c *Controller) { c.OnReconciliationMismatch("engine 0 vs delta 1") }},
		{"feed_lost", func(c *Controller) { c.OnPriceFeedLost("ws closed") }},
		{"reject_streak", func(c *Controller) {
			c.RecordReject("a")
			c.RecordReject("b")
			c.RecordReject("c")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENGINE_DATA_DIR", t.TempDir())
			c1, _ := armCtrl(t, false)
			if err := c1.Arm("operator", ArmConfirmationPhrase); err != nil {
				t.Fatalf("arm: %v", err)
			}
			tc.stop(c1)
			if c1.IsArmed() {
				t.Fatalf("%s should have stopped the engine", tc.name)
			}

			c2, _ := armCtrl(t, false)
			if c2.RestoreArmState(context.Background()) || c2.IsArmed() {
				t.Fatalf("%s must NOT auto-resume after a restart", tc.name)
			}
		})
	}
}

// A human turning it off must also stay off.
func TestArmState_NeverResumesAfterManualOff(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	c1, _ := armCtrl(t, false)
	_ = c1.Arm("operator", ArmConfirmationPhrase)
	c1.Disarm("operator", "turned off from UI")

	c2, _ := armCtrl(t, false)
	if c2.RestoreArmState(context.Background()) || c2.IsArmed() {
		t.Fatal("a manual off must not auto-resume")
	}
}

func TestArmState_DoesNotResumeWhenKillSwitchActive(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	c1, _ := armCtrl(t, false)
	_ = c1.Arm("operator", ArmConfirmationPhrase)

	c2, _ := armCtrl(t, true) // kill switch active now
	if c2.RestoreArmState(context.Background()) || c2.IsArmed() {
		t.Fatal("must not resume while the kill switch is active")
	}
}

// A box that has been down a long time must not wake up and trade on stale intent.
func TestArmState_DoesNotResumeStaleState(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ENGINE_DATA_DIR", dir)

	past := time.Now().UTC().Add(-24 * time.Hour)
	c1 := New(Hooks{
		IsConfigured:       func() bool { return true },
		SetEffectorEnabled: func(bool) {},
		AccountEquityUSD:   func(context.Context) (float64, error) { return 117.45, nil },
		Now:                func() time.Time { return past },
	})
	if err := c1.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}

	c2, _ := armCtrl(t, false) // "now" is real time — the saved state is a day old
	if c2.RestoreArmState(context.Background()) || c2.IsArmed() {
		t.Fatal("a stale ON state must not auto-resume")
	}
}

// The daily-loss baseline must carry across a restart, so restarting cannot
// reset the $20 stop and let the day's losses run further.
func TestArmState_CarriesDailyLossBaselineAcrossRestart(t *testing.T) {
	t.Setenv("ENGINE_DATA_DIR", t.TempDir())
	c1, _ := armCtrl(t, false)
	_ = c1.Arm("operator", ArmConfirmationPhrase) // baseline $117.45

	c2 := New(Hooks{
		IsConfigured:       func() bool { return true },
		KillSwitchActive:   func() bool { return false },
		SetEffectorEnabled: func(bool) {},
		// Equity is already down $15 by the time the engine comes back.
		AccountEquityUSD: func(context.Context) (float64, error) { return 102.45, nil },
	})
	if !c2.RestoreArmState(context.Background()) {
		t.Fatal("expected resume")
	}
	if got := c2.Snapshot().DayStartEquityUSD; got != 117.45 {
		t.Fatalf("baseline must carry across restart, got %.2f want 117.45", got)
	}
	// Another $5 down (total -$20 from the carried baseline) must trip the stop.
	if !c2.CheckDailyLoss(97.45, time.Now().UTC()) {
		t.Fatal("daily stop must trip against the carried baseline, not a fresh one")
	}
}
