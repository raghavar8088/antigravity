package main

import (
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/delta"
)

// The Live Engine Paper Desk must charge what the venue charges.
//
// The whole reason it exists is that the scalp desk's numbers did not transfer:
// 79.7% wins and +$37 gross on paper, 33.3% and -$13.91 with money. If this desk
// used a cheaper fee than the bridge, it would reproduce that failure with a new
// name on it.
func TestLivePaper_ChargesTakerOnBothLegs(t *testing.T) {
	d := newLivePaperDesk()
	// A long that runs to target.
	d.onSignal("S", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)
	d.onBar("ADAUSD", 0.2021, 0.2000, 0.2021)

	if len(d.closed) != 1 {
		t.Fatalf("expected one closed trade, got %d", len(d.closed))
	}
	tr := d.closed[0]
	contracts := d.closed[0].Contracts
	wantFees := (0.2000 + 0.2021) * contracts * delta.PerpTakerFeeRate

	if math.Abs(tr.FeesUSD-wantFees) > 1e-9 {
		t.Errorf("fees %.6f, want %.6f (taker on BOTH legs at %.5f)", tr.FeesUSD, wantFees, delta.PerpTakerFeeRate)
	}
	if tr.FeesUSD <= 0 {
		t.Error("a round trip paid no fees")
	}
	if math.Abs(tr.NetUSD-(tr.GrossUSD-tr.FeesUSD)) > 1e-9 {
		t.Errorf("net %.6f != gross %.6f - fees %.6f", tr.NetUSD, tr.GrossUSD, tr.FeesUSD)
	}
}

// ONE $100 for the whole desk, not $100 each.
//
// Per-strategy accounts quietly multiplied the capital: ten strategies meant
// $1,000 deployed while every row reported a return as if it owned the whole
// $100. The live bridge has one wallet, one aggregate leverage cap and one
// concurrency cap, so the paper mirror must have the same.
func TestLivePaper_OneSharedBalanceNotOnePerStrategy(t *testing.T) {
	d := newLivePaperDesk()
	d.onSignal("WINNER", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)
	d.onSignal("LOSER", "AVAXUSD", "LONG", 6.500, 6.4772, 6.5683, time.Hour)
	d.onBar("ADAUSD", 0.2021, 0.2000, 0.2021)  // winner hits target
	d.onBar("AVAXUSD", 6.5000, 6.4772, 6.4772) // loser hits stop

	snap := d.snapshot()
	accts := snap["accounts"].([]paperAccount)
	if len(accts) != 2 {
		t.Fatalf("expected 2 strategy rows, got %d", len(accts))
	}

	// The desk reports ONE equity, and it is the sum of every contribution.
	sum := 0.0
	for _, a := range accts {
		sum += a.NetUSD
	}
	equity := snap["equityUsd"].(float64)
	if math.Abs(equity-(livePaperStartingEquity+sum)) > 0.011 {
		t.Errorf("desk equity $%.2f != $100 + summed contributions $%.2f — the balance is not shared",
			equity, sum)
	}
	if equity == livePaperStartingEquity {
		t.Error("equity unchanged after a win and a loss closed")
	}
}

// Capital is finite. A fourth idea cannot be funded by pretending the first
// three were free — the live bridge refuses, so this must too.
func TestLivePaper_RespectsConcurrencyAndLeverageCaps(t *testing.T) {
	d := newLivePaperDesk()
	for i, sym := range []string{"AUSD", "BUSD", "CUSD", "DUSD", "EUSD"} {
		d.onSignal("S", sym, "LONG", 100, 99, 103, time.Hour)
		if want := i + 1; len(d.open) != want && want <= livePaperMaxConcurrent {
			t.Errorf("after %d signals the desk holds %d positions", want, len(d.open))
		}
	}
	if len(d.open) > livePaperMaxConcurrent {
		t.Errorf("desk holds %d positions, cap is %d", len(d.open), livePaperMaxConcurrent)
	}
	// And total deployed capital must respect the aggregate cap.
	if got, cap := d.openNotionalLocked(), livePaperStartingEquity*livePaperMaxLeverage; got > cap+0.01 {
		t.Errorf("deployed $%.2f against a $%.2f aggregate cap", got, cap)
	}
}

// Sizing must shrink with a drawdown. A desk that keeps deploying $300 after
// losing half its balance is running leverage it did not choose.
func TestLivePaper_SizeFollowsTheSharedBalance(t *testing.T) {
	d := newLivePaperDesk()
	d.onSignal("S", "ADAUSD", "LONG", 100, 99, 103, time.Hour)
	first := d.open[paperKey("S", "ADAUSD")].Contracts * 100

	// Take a large loss, then size again.
	d.equity = 50
	d.onBar("ADAUSD", 103, 99, 99) // stop out, clears the position
	d.equity = 50                  // hold the drawdown steady for the assertion
	d.onSignal("S", "ADAUSD", "LONG", 100, 99, 103, time.Hour)
	second := d.open[paperKey("S", "ADAUSD")].Contracts * 100

	if second >= first {
		t.Errorf("notional after a 50%% drawdown was $%.2f, not below the original $%.2f", second, first)
	}
}

// STOP takes precedence when one BAR reaches both levels. There is no way to
// know which was touched first, and assuming the target is the optimism that
// made the old leaderboard unreliable.
func TestLivePaper_StopWinsWhenBothLevelsAreSatisfied(t *testing.T) {
	d := newLivePaperDesk()
	// A well-formed long, and a bar whose range covers BOTH levels — the case
	// that actually happens and where the choice matters.
	d.onSignal("S", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)
	d.onBar("ADAUSD", 0.2025, 0.1990, 0.2010) // low pierced the stop, high the target

	if len(d.closed) != 1 {
		t.Fatalf("expected one close, got %d", len(d.closed))
	}
	if got := d.closed[0].Reason; got != "SL" {
		t.Errorf("exit reason %q; the stop must win when both levels are reachable", got)
	}
}

// One position per stream, matching the live bridge. Stacking would give the
// paper desk leverage the real one is not allowed to take.
func TestLivePaper_OnePositionPerStream(t *testing.T) {
	d := newLivePaperDesk()
	d.onSignal("S", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)
	d.onSignal("S", "ADAUSD", "LONG", 0.2005, 0.1998, 0.2026, time.Hour)
	if len(d.open) != 1 {
		t.Errorf("stream holds %d positions; the live bridge allows one", len(d.open))
	}
}

// Only PROMOTED streams belong here. Widening it to every stream would make it
// the scalp leaderboard again, which is the surface that could not be trusted.
func TestLivePaper_OnlyLiveRoutedStreamsAreRecorded(t *testing.T) {
	live := delta.ScalpLiveStreams()
	if len(live) == 0 {
		t.Fatal("no live streams configured; this test would pass vacuously")
	}
	if !delta.PerpStreamPermitted(live[0].Strategy, live[0].Symbol) {
		t.Errorf("%v is on the live selection but was not permitted", live[0])
	}
	if delta.PerpStreamPermitted("Some_Unpromoted_Strategy", live[0].Symbol) {
		t.Error("an unpromoted strategy was permitted onto the live paper desk")
	}
	// Right strategy, wrong symbol must also be refused — the pairing is the gate.
	if delta.PerpStreamPermitted(live[0].Strategy, "NOTASYMBOLUSD") {
		t.Error("a strategy was permitted on a symbol it was not selected for")
	}
}

// Reset must clear everything. A history recorded under two different rule sets
// is worse than no history, because it still looks complete.
func TestLivePaper_ResetClearsAccountsAndOpenPositions(t *testing.T) {
	d := newLivePaperDesk()
	d.onSignal("S", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)
	d.onBar("ADAUSD", 0.2021, 0.2000, 0.2021)
	d.onSignal("S", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)

	if n := d.reset(); n != 1 {
		t.Errorf("reset reported %d cleared trades, want 1", n)
	}
	if d.equity != livePaperStartingEquity {
		t.Errorf("equity after reset $%.2f, want $%.2f", d.equity, livePaperStartingEquity)
	}
	if len(d.accounts) != 0 || len(d.open) != 0 || len(d.closed) != 0 {
		t.Errorf("after reset: %d accounts, %d open, %d closed — all must be zero",
			len(d.accounts), len(d.open), len(d.closed))
	}
}

// A malformed signal must be refused rather than opening a position with no
// stop — an unbounded loss wearing the appearance of a managed trade.
func TestLivePaper_RefusesSignalsWithoutLevels(t *testing.T) {
	d := newLivePaperDesk()
	for _, tc := range []struct{ entry, stop, target float64 }{
		{0, 0.19, 0.21}, {0.20, 0, 0.21}, {0.20, 0.19, 0},
	} {
		d.onSignal("S", "ADAUSD", "LONG", tc.entry, tc.stop, tc.target, time.Hour)
	}
	if len(d.open) != 0 {
		t.Errorf("%d position(s) opened from malformed signals", len(d.open))
	}
}

// A spike through the stop that closes back inside must still stop out.
//
// Checking only the close silently deletes these, and it only ever deletes
// LOSSES — a wick that reached the target and closed back would be recorded as
// a win by the venue's resting order but ignored here. The bias runs one way.
func TestLivePaper_IntrabarStopIsNotMissed(t *testing.T) {
	d := newLivePaperDesk()
	d.onSignal("S", "ADAUSD", "LONG", 0.2000, 0.1993, 0.2021, time.Hour)
	// Low pierces the stop; the bar closes above the entry, looking like a win.
	d.onBar("ADAUSD", 0.2010, 0.1990, 0.2005)

	if len(d.closed) != 1 {
		t.Fatalf("expected the stop to fire on the wick, got %d closes", len(d.closed))
	}
	if got := d.closed[0].Reason; got != "SL" {
		t.Errorf("exit %q; a bar whose LOW pierced the stop must stop out even if it closed higher", got)
	}
	if d.closed[0].NetUSD >= 0 {
		t.Errorf("a stop-out booked %+.4f; it must be a loss", d.closed[0].NetUSD)
	}
}
