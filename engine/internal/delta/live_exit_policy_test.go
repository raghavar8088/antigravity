package delta

import "testing"

// The exit policy is what turns a long-option book's payoff from capped-upside
// into convex. These tests pin the shape, because the previous fixed +80% cap
// was the defect: it clipped the right tail that pays for every losing premium.

func TestEvaluateExit_HoldsBelowTheTrailArm(t *testing.T) {
	// +30% with no giveback: not yet armed, nothing to do.
	d := EvaluateExit(100, 130, 130)
	if d.Reason != "" {
		t.Fatalf("expected hold at +30%%, got %q", d.Reason)
	}
	if d.TrailArmed {
		t.Fatal("trail must not arm below the arm threshold")
	}
}

func TestEvaluateExit_NoLongerCapsWinnersAt80Pct(t *testing.T) {
	// The regression that matters: at +80% the old policy closed. The new policy
	// holds, because the position is still making new highs.
	d := EvaluateExit(100, 180, 180)
	if d.Reason != "" {
		t.Fatalf("a winner at its high must not be closed, got %q", d.Reason)
	}
	if !d.TrailArmed {
		t.Fatal("trail must be armed at +80%")
	}

	// And it keeps holding far beyond the old cap.
	if d := EvaluateExit(100, 250, 250); d.Reason != "" {
		t.Fatalf("expected hold at +150%%, got %q", d.Reason)
	}
}

func TestEvaluateExit_TrailsOnGivebackFromPeak(t *testing.T) {
	// Peak +50%, now back to +5%: that is a 30% giveback of the peak mark.
	d := EvaluateExit(100, 105, 150)
	if d.Reason != "trailing_exit" {
		t.Fatalf("expected trailing_exit, got %q (giveback %.3f)", d.Reason, d.GivebackPct)
	}

	// A shallower pullback from the same peak is still held.
	if d := EvaluateExit(100, 130, 150); d.Reason != "" {
		t.Fatalf("a 13%% giveback must be held, got %q", d.Reason)
	}
}

func TestEvaluateExit_TrailStaysDisarmedOnSmallWinners(t *testing.T) {
	// Peak of only +20%, then a full round trip back to entry. The trail never
	// armed, so the hard stop — not the trail — is the operative protection.
	d := EvaluateExit(100, 100, 120)
	if d.TrailArmed {
		t.Fatal("trail must stay disarmed when the peak never reached the arm level")
	}
	if d.Reason != "" {
		t.Fatalf("expected hold, got %q", d.Reason)
	}
}

func TestEvaluateExit_HardStopStillFires(t *testing.T) {
	d := EvaluateExit(100, 50, 120)
	if d.Reason != "stop_loss_50pct" {
		t.Fatalf("expected stop_loss_50pct, got %q", d.Reason)
	}
}

func TestEvaluateExit_HardStopWinsOverTrail(t *testing.T) {
	// Ran to +60% (armed), then collapsed through the stop. The loss-cutting exit
	// must take precedence over the trailing exit so the reason is not misreported.
	d := EvaluateExit(100, 40, 160)
	if d.Reason != "stop_loss_50pct" {
		t.Fatalf("expected stop_loss_50pct to win, got %q", d.Reason)
	}
}

func TestEvaluateExit_HardCapBanksARunaway(t *testing.T) {
	d := EvaluateExit(100, 400, 400)
	if d.Reason != "take_profit_hard_cap" {
		t.Fatalf("expected take_profit_hard_cap at +300%%, got %q", d.Reason)
	}
}

func TestEvaluateExit_PeakNeverTrailsTheMark(t *testing.T) {
	// A stale or missing peak must not manufacture a giveback.
	d := EvaluateExit(100, 150, 0)
	if d.Reason != "" {
		t.Fatalf("a zero peak must not trigger an exit, got %q", d.Reason)
	}
	if d.GivebackPct != 0 {
		t.Fatalf("giveback must be 0 when the mark is the peak, got %.3f", d.GivebackPct)
	}
}

func TestEvaluateExit_ZeroEntryIsInert(t *testing.T) {
	if d := EvaluateExit(0, 100, 100); d.Reason != "" {
		t.Fatalf("an unpriced entry must produce no exit decision, got %q", d.Reason)
	}
}
