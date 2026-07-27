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

func TestController_DailyLossBreakerAutoDisarms(t *testing.T) {
	t.Setenv("LIVE_ENGINE_MAX_DAILY_LOSS_USD", "")
	equity := 100.0
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		SetEffectorEnabled: func(bool) {},
		AccountEquityUSD:   func(context.Context) (float64, error) { return equity, nil },
	})
	if err := c.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}
	now := time.Now().UTC()

	// Down $19 — under the $20 default limit, stays armed.
	if tripped := c.CheckDailyLoss(81, now); tripped || !c.IsArmed() {
		t.Fatalf("should not trip at -$19 (tripped=%v armed=%v)", tripped, c.IsArmed())
	}
	// Down $20 — hits the limit, auto-disarms.
	if tripped := c.CheckDailyLoss(80, now); !tripped {
		t.Fatal("expected daily-loss breaker to trip at -$20")
	}
	if c.IsArmed() {
		t.Fatal("must be disarmed after the daily-loss breaker trips")
	}
	if c.Snapshot().LastDisarmReason != "daily_loss_breaker" {
		t.Fatalf("unexpected reason: %s", c.Snapshot().LastDisarmReason)
	}
}

func TestController_DailyLossLimitConfigurable(t *testing.T) {
	t.Setenv("LIVE_ENGINE_MAX_DAILY_LOSS_USD", "5")
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		SetEffectorEnabled: func(bool) {},
		AccountEquityUSD:   func(context.Context) (float64, error) { return 100, nil },
	})
	_ = c.Arm("operator", ArmConfirmationPhrase)
	now := time.Now().UTC()
	if tripped := c.CheckDailyLoss(96, now); tripped {
		t.Fatal("should not trip at -$4 with a $5 limit")
	}
	if tripped := c.CheckDailyLoss(95, now); !tripped {
		t.Fatal("expected trip at -$5 with a $5 limit")
	}
}

func TestController_DailyLossRollsOverAtMidnight(t *testing.T) {
	// Pin the clock so arming baselines on day1 regardless of the real date —
	// otherwise this test's meaning changes with the calendar.
	day1 := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	day2 := day1.Add(24 * time.Hour)
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		SetEffectorEnabled: func(bool) {},
		AccountEquityUSD:   func(context.Context) (float64, error) { return 100, nil },
		Now:                func() time.Time { return day1 },
	})
	if err := c.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}
	// Same day, -$20 would trip — proves the baseline is day1's $100.
	// (checked on a copy of intent: we go straight to the rollover case below)

	// New day re-baselines to the current equity, so yesterday's drawdown
	// does not carry into the new day's limit.
	if tripped := c.CheckDailyLoss(70, day2); tripped {
		t.Fatal("new day must re-baseline, not trip on yesterday's drawdown")
	}
	// Same (new) day, another -$20 from the new $70 baseline trips.
	if tripped := c.CheckDailyLoss(50, day2); !tripped {
		t.Fatal("expected trip -$20 below the new day's baseline")
	}
}

// The kill-switch toggle must halt AND disarm — an armed engine can never sit
// behind an active halt — and releasing must be audited.
func TestController_KillSwitchToggleHaltsAndDisarms(t *testing.T) {
	ksActive := false
	c := New(Hooks{
		IsConfigured:       func() bool { return true },
		SetEffectorEnabled: func(bool) {},
		KillSwitchActive:   func() bool { return ksActive },
		SetKillSwitch: func(_ context.Context, active bool, _, _ string) error {
			ksActive = active
			return nil
		},
	})
	if err := c.Arm("operator", ArmConfirmationPhrase); err != nil {
		t.Fatalf("arm: %v", err)
	}

	// Toggle ON → halts and disarms.
	if err := c.SetKillSwitch(context.Background(), true, "operator", ""); err != nil {
		t.Fatalf("halt: %v", err)
	}
	if !ksActive {
		t.Fatal("kill switch should be active")
	}
	if c.IsArmed() {
		t.Fatal("halting must disarm the live engine")
	}

	// Toggle OFF → resumes (but does NOT re-arm; arming stays a human action).
	if err := c.SetKillSwitch(context.Background(), false, "operator", ""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if ksActive {
		t.Fatal("kill switch should be released")
	}
	if c.IsArmed() {
		t.Fatal("releasing the kill switch must never auto-arm")
	}

	var on, off int
	for _, e := range c.Audit() {
		switch e.Action {
		case ActionKillSwitchOn:
			on++
		case ActionKillSwitchOff:
			off++
		}
	}
	if on != 1 || off != 1 {
		t.Fatalf("kill switch toggles must be audited, got on=%d off=%d", on, off)
	}
}

func TestController_KillSwitchToggleUnwiredErrors(t *testing.T) {
	c := New(Hooks{IsConfigured: func() bool { return true }})
	if err := c.SetKillSwitch(context.Background(), true, "operator", ""); err == nil {
		t.Fatal("expected an error when kill-switch control is not wired")
	}
	if c.Snapshot().KillSwitchControllable {
		t.Fatal("snapshot must report the kill switch as not controllable when unwired")
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
