package liveengine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func armedController(t *testing.T) (*Controller, *bool) {
	t.Helper()
	enabled := false
	c := New(Hooks{
		SetEffectorEnabled: func(v bool) { enabled = v },
		IsConfigured:       func() bool { return true },
		KillSwitchActive:   func() bool { return false },
	})
	if c.IsArmed() {
		t.Fatal("controller must ship DISARMED")
	}
	if err := c.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm failed: %v", err)
	}
	if !c.IsArmed() || !enabled {
		t.Fatal("expected armed + effector enabled")
	}
	return c, &enabled
}

func TestController_ShipsDisarmed(t *testing.T) {
	c := New(Hooks{})
	if c.IsArmed() {
		t.Fatal("must ship disarmed")
	}
	if c.Snapshot().State != StateDisarmed {
		t.Fatal("snapshot must report DISARMED")
	}
}

func TestController_ArmRequiresExactConfirmation(t *testing.T) {
	c := New(Hooks{IsConfigured: func() bool { return true }})
	for _, bad := range []string{"", "arm live $100", "ARM LIVE", "yes"} {
		if err := c.Arm("op", bad); !errors.Is(err, ErrBadConfirmation) {
			t.Fatalf("confirmation %q must be rejected, got %v", bad, err)
		}
	}
	if c.IsArmed() {
		t.Fatal("must remain disarmed after bad confirmations")
	}
	// The rejections are audited.
	var rejected int
	for _, e := range c.Audit() {
		if e.Action == ActionArmRejected {
			rejected++
		}
	}
	if rejected != 4 {
		t.Fatalf("expected 4 ARM_REJECTED audit entries, got %d", rejected)
	}
}

func TestController_CannotArmWhileKillSwitchActive(t *testing.T) {
	c := New(Hooks{
		IsConfigured:     func() bool { return true },
		KillSwitchActive: func() bool { return true },
	})
	if err := c.Arm("op", ArmConfirmationPhrase); !errors.Is(err, ErrKillSwitch) {
		t.Fatalf("expected kill-switch rejection, got %v", err)
	}
	if c.IsArmed() {
		t.Fatal("must not arm while kill switch active")
	}
}

func TestController_CannotArmWhenNotConfigured(t *testing.T) {
	c := New(Hooks{IsConfigured: func() bool { return false }})
	if err := c.Arm("op", ArmConfirmationPhrase); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected not-configured rejection, got %v", err)
	}
}

func TestController_CeilingCapsAtHundred(t *testing.T) {
	c := New(Hooks{})
	if got := c.TradableEquityUSD(1_000_000); got != MaxTradableUSD {
		t.Fatalf("ceiling must cap at $%.0f, got $%.2f", MaxTradableUSD, got)
	}
	if got := c.TradableEquityUSD(42); got != 42 {
		t.Fatalf("below-ceiling equity must pass through, got %.2f", got)
	}
	if got := c.TradableEquityUSD(-5); got != 0 {
		t.Fatalf("negative equity must floor at 0, got %.2f", got)
	}
}

func TestController_RejectStreakAutoDisarms(t *testing.T) {
	c, enabled := armedController(t)
	c.RecordReject("first")
	c.RecordReject("second")
	if !c.IsArmed() {
		t.Fatal("must still be armed before threshold")
	}
	c.RecordReject("third")
	if c.IsArmed() {
		t.Fatal("must auto-disarm at the reject threshold")
	}
	if *enabled {
		t.Fatal("effector must be disabled on auto-disarm")
	}
	if c.Snapshot().LastDisarmReason != "consecutive_broker_rejects" {
		t.Fatalf("unexpected disarm reason: %s", c.Snapshot().LastDisarmReason)
	}
}

func TestController_FillResetsRejectStreak(t *testing.T) {
	c, _ := armedController(t)
	c.RecordReject("a")
	c.RecordReject("b")
	c.RecordFillOK()
	c.RecordReject("c")
	c.RecordReject("d")
	if !c.IsArmed() {
		t.Fatal("a successful fill must reset the streak, so 2 more rejects must not disarm")
	}
}

func TestController_AutoDisarmIsOneWay(t *testing.T) {
	c, _ := armedController(t)
	c.OnReconciliationMismatch("engine has 1 position, delta has 0")
	if c.IsArmed() {
		t.Fatal("reconciliation mismatch must auto-disarm")
	}
	// Re-arming is a human action — the controller does not self-rearm. Arming
	// again requires the explicit typed confirmation.
	if err := c.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("human re-arm should succeed: %v", err)
	}
	if !c.IsArmed() {
		t.Fatal("human re-arm must work")
	}
}

func TestController_AllAutoDisarmTriggers(t *testing.T) {
	cases := []struct {
		name string
		fire func(c *Controller)
		want string
	}{
		{"daily_loss", func(c *Controller) { c.OnDailyLossBreaker("down 3%") }, "daily_loss_breaker"},
		{"stale_data", func(c *Controller) { c.OnStaleMarketData(30*time.Second, 10*time.Second) }, "stale_market_data"},
		{"feed_lost", func(c *Controller) { c.OnPriceFeedLost("ws closed") }, "price_feed_lost"},
		{"recon", func(c *Controller) { c.OnReconciliationMismatch("mismatch") }, "reconciliation_mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := armedController(t)
			tc.fire(c)
			if c.IsArmed() {
				t.Fatalf("%s must auto-disarm", tc.name)
			}
			if c.Snapshot().LastDisarmReason != tc.want {
				t.Fatalf("got reason %q, want %q", c.Snapshot().LastDisarmReason, tc.want)
			}
		})
	}
}

func TestController_StaleDataWithinBoundDoesNotDisarm(t *testing.T) {
	c, _ := armedController(t)
	c.OnStaleMarketData(5*time.Second, 10*time.Second)
	if !c.IsArmed() {
		t.Fatal("fresh data must not disarm")
	}
}

func TestController_CloseAllAuditsAndInvokesHook(t *testing.T) {
	called := false
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		SetEffectorEnabled: func(bool) {},
		CloseAll: func(context.Context) (map[string]any, error) {
			called = true
			return map[string]any{"closed": 2}, nil
		},
	})
	if _, err := c.CloseAll(context.Background(), "operator"); err != nil {
		t.Fatalf("close-all: %v", err)
	}
	if !called {
		t.Fatal("close-all hook must be invoked")
	}
	var found bool
	for _, e := range c.Audit() {
		if e.Action == ActionCloseAll {
			found = true
		}
	}
	if !found {
		t.Fatal("close-all must be audited")
	}
}
