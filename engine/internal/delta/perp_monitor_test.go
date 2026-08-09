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
	// Bracketed, the venue owns it — the monitor must NOT close at market.
	if got := perpExitReason(base(true), 0.016400, now); got != "" {
		t.Errorf("bracketed position returned %q; the monitor front-ran the venue and that is the overshoot", got)
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
