package delta

import (
	"testing"
	"time"
)

// When the VENUE holds a bracket, the monitor must not close on price.
//
// Both were live and the monitor kept winning: it polls mark every 15s and
// closes at market, while the bracket waits for last-traded to cross. Measured
// on TSTUSD — bracket limit 0.016308, monitor exited at 0.016330, a price a
// limit order could not have filled. The mechanism built to stop the overshoot
// was being front-run by the one causing it, and every stop-out paid ~1.2-1.4x
// its planned loss.
func TestPerpExitReason_MonitorStandsDownWhenBracketed(t *testing.T) {
	base := func(bracketed bool) *PerpLiveTrade {
		return &PerpLiveTrade{
			Side: SideSell, EntryPrice: 0.016210,
			StopPrice: 0.016308, TargetPrice: 0.015977,
			BracketsAttached: bracketed,
			OpenedAt:         time.Now().Add(-time.Minute),
			ExpiresAt:        time.Now().Add(time.Hour),
		}
	}
	now := time.Now()

	// Mark through the stop. Unbracketed, the monitor is the only protection
	// and must act.
	if got := perpExitReason(base(false), 0.016400, now); got != "SL" {
		t.Errorf("unbracketed position at a broken stop returned %q; the monitor is its only protection", got)
	}
	// Bracketed and JUST through the stop — the venue owns this exit. 0.016320
	// is 0.07% past a 0.016308 stop, inside the backstop threshold, so the
	// monitor must stay out of it. Closing here at market is exactly the
	// front-running that caused the overshoot.
	if got := perpExitReason(base(true), 0.016320, now); got != "" {
		t.Errorf("bracketed position just through its stop returned %q; the venue owns that exit", got)
	}
	// But far through it, the venue has demonstrably not acted and the monitor
	// must override. Asserted here as well as in the backstop tests, because
	// the two behaviours are one decision and reading them apart hides the
	// boundary between them.
	if got := perpExitReason(base(true), 0.016400, now); got != "SL_BACKSTOP" {
		t.Errorf("bracketed position 0.56%% past its stop returned %q; a bracket that has not filled is not protection", got)
	}
	// Same for the target.
	if got := perpExitReason(base(true), 0.015900, now); got != "" {
		t.Errorf("bracketed position returned %q at the target; the venue owns price exits", got)
	}
}

// The TIME STOP always belongs to this process — Delta has no concept of it, so
// standing down on price must not also abandon the clock.
func TestPerpExitReason_TimeStopSurvivesBracketing(t *testing.T) {
	expired := &PerpLiveTrade{
		Side: SideSell, EntryPrice: 0.0162, StopPrice: 0.0163, TargetPrice: 0.0159,
		BracketsAttached: true,
		OpenedAt:         time.Now().Add(-2 * time.Hour),
		ExpiresAt:        time.Now().Add(-time.Minute),
	}
	if got := perpExitReason(expired, 0.0162, time.Now()); got != "TTL" {
		t.Errorf("expired bracketed position returned %q, want TTL — the venue cannot enforce a time stop", got)
	}
	// And a bracketed position that is neither expired nor at a level exits for
	// nothing at all.
	live := *expired
	live.ExpiresAt = time.Now().Add(time.Hour)
	if got := perpExitReason(&live, 0.0162, time.Now()); got != "" {
		t.Errorf("a healthy bracketed position returned %q", got)
	}
}

// A position with no bracket must keep full monitor coverage. The stand-down is
// conditional on protection existing, not on the flag being convenient.
func TestPerpExitReason_UnbracketedKeepsFullCoverage(t *testing.T) {
	long := &PerpLiveTrade{
		Side: SideBuy, EntryPrice: 100, StopPrice: 99, TargetPrice: 103,
		BracketsAttached: false,
		OpenedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
	}
	if got := perpExitReason(long, 98.5, time.Now()); got != "SL" {
		t.Errorf("unbracketed long through its stop returned %q", got)
	}
	if got := perpExitReason(long, 103.5, time.Now()); got != "TP" {
		t.Errorf("unbracketed long through its target returned %q", got)
	}
}
