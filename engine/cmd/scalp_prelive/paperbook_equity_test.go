package main

import (
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/delta"
)

// A registered book runs on its OWN capital, and every figure derived from it
// is divided by that same number.
//
// The failure this guards against does not look like a failure: a $500 book
// reporting against the process default of $100 shows a correct balance and a
// five-times-inflated ROI beside it, on a page whose whole purpose is deciding
// whether a strategy earned anything.
func TestNamedBookRunsOnItsOwnEquity(t *testing.T) {
	delta.RegisterNamedPaperBook("EQTEST", 500, nil)
	t.Cleanup(func() { delete(livePaperBooks, "EQTEST") })

	d := newLivePaperDesk("EQTEST")
	if d.start != 500 {
		t.Fatalf("book opened at $%.2f, want $500", d.start)
	}
	if d.equity != 500 {
		t.Fatalf("balance opened at $%.2f, want $500", d.equity)
	}

	snap := d.snapshot()
	if got := snap["startingEquityUsd"].(float64); got != 500 {
		t.Fatalf("snapshot reports a $%.2f book, want $500", got)
	}
	if got := snap["roiPct"].(float64); got != 0 {
		t.Fatalf("an untouched book reports %.2f%% ROI, want 0", got)
	}

	// A $50 gain is +10% of this book, not +50%.
	d.equity += 50
	snap = d.snapshot()
	if got := snap["netUsd"].(float64); got != 50 {
		t.Fatalf("net = $%.2f, want $50", got)
	}
	if got := snap["roiPct"].(float64); math.Abs(got-10) > 0.001 {
		t.Fatalf("ROI = %.3f%%, want 10%% of a $500 book", got)
	}

	// Reset returns it to ITS opening balance, not the process default.
	d.reset()
	if d.equity != 500 {
		t.Fatalf("reset left the book at $%.2f, want $500", d.equity)
	}
}

// The default books are untouched by the per-book resolution: no registration,
// no override, so they still open at the process-wide $100.
func TestUnregisteredBookKeepsProcessDefault(t *testing.T) {
	d := newLivePaperDesk(delta.PaperAccount02)
	if d.start != livePaperStartingEquity {
		t.Fatalf("account 02 opened at $%.2f, want the process default $%.2f", d.start, livePaperStartingEquity)
	}
}

// A book's positions are sized off its own balance, so the $500 book deploys
// five times the notional of a $100 one on the same signal.
func TestNamedBookSizesOffItsOwnBalance(t *testing.T) {
	delta.RegisterNamedPaperBook("SIZETEST", 500, nil)
	t.Cleanup(func() { delete(livePaperBooks, "SIZETEST") })

	big := newLivePaperDesk("SIZETEST")
	small := newLivePaperDesk(delta.PaperAccount02)

	// A long with a stop well inside the liquidation distance, so neither desk
	// refuses it for a reason unrelated to size.
	for _, d := range []*livePaperDesk{big, small} {
		d.onSignal("S_Long", "ADAUSD", "LONG", 100, 99.3, 102, time.Hour)
	}

	bp := big.open[paperKey("S_Long", "ADAUSD")]
	sp := small.open[paperKey("S_Long", "ADAUSD")]
	if bp == nil || sp == nil {
		t.Fatalf("expected both books to take the signal (big=%v small=%v)", bp != nil, sp != nil)
	}
	ratio := (bp.Entry * bp.Contracts) / (sp.Entry * sp.Contracts)
	if math.Abs(ratio-5) > 0.001 {
		t.Fatalf("the $500 book deployed %.3fx the $100 book's notional, want 5x", ratio)
	}
}
