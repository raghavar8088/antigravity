package main

import (
	"math"
	"testing"
	"time"
)

// The caps became runtime-configurable so Top Crypto Trading could run $10,000
// books with no position ceiling. Zero means UNLIMITED for both caps, and the
// dangerous misreading is the opposite one — zero meaning "nothing allowed" —
// which would produce a desk that accepts every signal and opens nothing.
func withPaperSettings(t *testing.T, equity, leverage, positionUSD float64, concurrent int, fn func()) {
	t.Helper()
	oe, ol, op, oc := livePaperStartingEquity, livePaperMaxLeverage, livePaperPositionUSD, livePaperMaxConcurrent
	livePaperStartingEquity, livePaperMaxLeverage, livePaperPositionUSD, livePaperMaxConcurrent = equity, leverage, positionUSD, concurrent
	t.Cleanup(func() {
		livePaperStartingEquity, livePaperMaxLeverage, livePaperPositionUSD, livePaperMaxConcurrent = oe, ol, op, oc
	})
	fn()
}

// openN offers n distinct streams to a fresh book and returns how many filled.
func openN(d *livePaperDesk, n int) int {
	for i := 0; i < n; i++ {
		d.onSignal(
			"MTF_1h_Wedge_Long"+string(rune('A'+i%26))+string(rune('a'+i/26)),
			"BTCUSD", "LONG", 100, 99, 106, time.Hour,
		)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.open)
}

func TestPaperConcurrencyCapEnforced(t *testing.T) {
	withPaperSettings(t, 100, 3, 0, 3, func() {
		d := newLivePaperDesk("test")
		if got := openN(d, 10); got != 3 {
			t.Fatalf("with maxConcurrent=3 the book holds %d positions, want 3", got)
		}
	})
}

func TestPaperConcurrencyUnlimited(t *testing.T) {
	withPaperSettings(t, 10000, 0, 300, 0, func() {
		d := newLivePaperDesk("test")
		got := openN(d, 40)
		if got != 40 {
			t.Fatalf("with maxConcurrent=0 (unlimited) the book holds %d of 40 offered — zero must mean unlimited, not none", got)
		}
		// Every position sized at the explicit notional, not the old formula.
		d.mu.Lock()
		defer d.mu.Unlock()
		for _, p := range d.open {
			if n := p.Entry * p.Contracts; math.Abs(n-300) > 0.01 {
				t.Fatalf("position notional %.2f, want the configured 300", n)
			}
		}
	})
}

// With an aggregate cap still set, unlimited concurrency must NOT mean unlimited
// exposure — the budget still binds.
func TestPaperAggregateCapStillBindsWhenConcurrencyUnlimited(t *testing.T) {
	withPaperSettings(t, 1000, 3, 300, 0, func() {
		d := newLivePaperDesk("test")
		// 3x on $1,000 = $3,000 of budget, at $300 each = 10 positions.
		if got := openN(d, 40); got != 10 {
			t.Fatalf("aggregate cap allowed %d positions, want 10 ($3,000 budget / $300 each)", got)
		}
	})
}

func TestPaperEquityConfigurable(t *testing.T) {
	withPaperSettings(t, 10000, 0, 300, 0, func() {
		d := newLivePaperDesk("test")
		snap := d.snapshot()
		if got := snap["startingEquityUsd"]; got != 10000.0 {
			t.Fatalf("startingEquityUsd = %v, want 10000", got)
		}
		// Unlimited must be reported as -1, never 0: a reader seeing
		// maxConcurrent 0 would conclude nothing may open, the exact opposite.
		if got := snap["maxConcurrent"]; got != -1 {
			t.Fatalf("maxConcurrent = %v, want -1 for unlimited", got)
		}
		if got := snap["maxLeverage"]; got != -1.0 {
			t.Fatalf("maxLeverage = %v, want -1 for unlimited", got)
		}
		if got := snap["maxNotionalUsd"]; got != -1.0 {
			t.Fatalf("maxNotionalUsd = %v, want -1 for unlimited", got)
		}
	})
}

// The default path — every other desk — must be byte-for-byte unchanged.
func TestPaperDefaultsUnchanged(t *testing.T) {
	withPaperSettings(t, 100, 3, 0, 3, func() {
		d := newLivePaperDesk("test")
		openN(d, 5)
		d.mu.Lock()
		defer d.mu.Unlock()
		if len(d.open) != 3 {
			t.Fatalf("default book holds %d, want 3", len(d.open))
		}
		for _, p := range d.open {
			// equity*leverage/concurrent = 100*3/3 = 100 per position.
			if n := p.Entry * p.Contracts; math.Abs(n-100) > 0.01 {
				t.Fatalf("default position notional %.2f, want 100", n)
			}
		}
	})
}
