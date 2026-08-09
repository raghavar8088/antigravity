package delta

import (
	"testing"
	"time"
)

// A bracket that cannot fill must not leave the position unprotected.
//
// Live case: LABUSD short with a stop at 0.1243. Price gapped to 0.1253, the
// leg triggered and converted to a buy limit AT the trigger — below the market,
// so it could never fill — and rested there while the loss ran from a planned
// -$1.78 to -$3.22. The monitor stood down because a bracket was "attached".
//
// Trusting the bracket absolutely turned a capped loss into an open-ended one.
func TestPerpBackstop_ClosesWhenPriceStaysThroughTheStop(t *testing.T) {
	short := func() *PerpLiveTrade {
		return &PerpLiveTrade{
			Side: SideSell, EntryPrice: 0.123746,
			StopPrice: 0.124346, TargetPrice: 0.12195,
			BracketsAttached: true,
			OpenedAt:         time.Now().Add(-time.Minute),
			ExpiresAt:        time.Now().Add(time.Hour),
		}
	}
	now := time.Now()

	// Exactly the live case: 0.6% beyond the stop, bracket attached.
	if got := perpExitReason(short(), 0.125097, now); got != "SL_BACKSTOP" {
		t.Errorf("position 0.6%% beyond its stop returned %q; it must be closed", got)
	}

	// Just through the stop is the bracket's job, not the monitor's — closing
	// here would reintroduce the front-running that caused the overshoot.
	if got := perpExitReason(short(), 0.124400, now); got != "" {
		t.Errorf("a position barely through its stop returned %q; the venue owns that exit", got)
	}

	// And a healthy position is untouched.
	if got := perpExitReason(short(), 0.1235, now); got != "" {
		t.Errorf("a position inside its levels returned %q", got)
	}
}

// The backstop must work in both directions, or half the book is unprotected.
func TestPerpBackstop_WorksForLongs(t *testing.T) {
	long := &PerpLiveTrade{
		Side: SideBuy, EntryPrice: 100, StopPrice: 99, TargetPrice: 103,
		BracketsAttached: true,
		OpenedAt:         time.Now(),
		ExpiresAt:        time.Now().Add(time.Hour),
	}
	if got := perpExitReason(long, 98.0, time.Now()); got != "SL_BACKSTOP" {
		t.Errorf("long 1%% below its stop returned %q", got)
	}
	if got := perpExitReason(long, 98.9, time.Now()); got != "" {
		t.Errorf("long barely through its stop returned %q; that is the bracket's job", got)
	}
}

// The backstop must never fire on the profitable side — closing a winner
// because it moved toward its target would be catastrophic.
func TestPerpBackstop_NeverFiresOnTheWinningSide(t *testing.T) {
	short := &PerpLiveTrade{
		Side: SideSell, EntryPrice: 0.1237, StopPrice: 0.1243, TargetPrice: 0.1219,
		BracketsAttached: true, OpenedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	// Deep in profit for a short — far BELOW entry.
	if perpStopBreachedBadly(short, 0.1100) {
		t.Error("backstop armed on a short in profit; it would close a winner as a stop-out")
	}
	long := &PerpLiveTrade{
		Side: SideBuy, EntryPrice: 100, StopPrice: 99, TargetPrice: 103,
		BracketsAttached: true, OpenedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	if perpStopBreachedBadly(long, 120) {
		t.Error("backstop armed on a long in profit")
	}
}

// Degenerate inputs must not arm it.
func TestPerpBackstop_FailsClosed(t *testing.T) {
	if perpStopBreachedBadly(nil, 1) {
		t.Error("nil trade armed the backstop")
	}
	if perpStopBreachedBadly(&PerpLiveTrade{Side: SideBuy, StopPrice: 0}, 1) {
		t.Error("a position with no stop armed the backstop")
	}
	if perpStopBreachedBadly(&PerpLiveTrade{Side: SideBuy, StopPrice: 99}, 0) {
		t.Error("a zero mark armed the backstop")
	}
}
