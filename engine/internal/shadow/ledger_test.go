package shadow

import (
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/execution"
)

// newTestLedger returns an in-memory ledger (no Mongo).
func newTestLedger() *ShadowLedger {
	return NewShadowLedger(nil)
}

// TestNetPnLSubtractsBothFeeLegs pins the canonical fee model
// (NetPnL = GrossPnL − EntryFee − ExitFee) on shadow closes. A previous bug
// subtracted only the exit fee, overstating every shadow trade by one fee leg
// vs the identical live trade and inflating promotion stats.
func TestNetPnLSubtractsBothFeeLegs(t *testing.T) {
	l := newTestLedger()

	trade, err := l.OpenTrade(Signal{
		Strategy:   "TestStrat",
		Direction:  DirectionLong,
		StopLoss:   99000,
		TakeProfit: 101000,
	}, 100000, 0.5)
	if err != nil {
		t.Fatalf("OpenTrade failed: %v", err)
	}

	closed := l.CheckAndClose(101100) // above TP
	if len(closed) != 1 {
		t.Fatalf("expected 1 closed trade, got %d", len(closed))
	}
	c := closed[0]
	if c.ExitReason != "TP" {
		t.Fatalf("expected exit reason TP, got %s", c.ExitReason)
	}

	entryNotional := c.Size * c.EntryPrice
	exitNotional := c.Size * c.ExitPrice
	wantGross := exitNotional - entryNotional
	wantFees := (entryNotional + exitNotional) * execution.BinanceFuturesTakerFeePct
	wantNet := wantGross - wantFees

	if math.Abs(c.GrossPnL-wantGross) > 1e-6 {
		t.Errorf("GrossPnL = %.6f, want %.6f", c.GrossPnL, wantGross)
	}
	if math.Abs(c.Fees-wantFees) > 1e-6 {
		t.Errorf("Fees = %.6f, want %.6f (both legs)", c.Fees, wantFees)
	}
	if math.Abs(c.NetPnL-wantNet) > 1e-6 {
		t.Errorf("NetPnL = %.6f, want %.6f (gross − entry fee − exit fee)", c.NetPnL, wantNet)
	}
	_ = trade
}

// TestOpenTradeEnforcesPerStrategyCap pins the concurrency cap that mirrors
// positions.NewManager's MaxPerStrategy=2. Without it, a signal persisting
// across 15m eval cycles stacks correlated duplicates that inflate the trade
// count toward the ShadowPromoter 30-trade bar.
func TestOpenTradeEnforcesPerStrategyCap(t *testing.T) {
	l := newTestLedger()
	sig := Signal{Strategy: "CapStrat", Direction: DirectionShort, StopLoss: 105000, TakeProfit: 95000}

	for i := 0; i < 2; i++ {
		if !l.CanOpen("CapStrat") {
			t.Fatalf("CanOpen false at %d open positions, cap is 2", i)
		}
		if _, err := l.OpenTrade(sig, 100000, 0.5); err != nil {
			t.Fatalf("OpenTrade %d failed: %v", i+1, err)
		}
	}

	if l.CanOpen("CapStrat") {
		t.Error("CanOpen true at cap (2 open positions)")
	}
	if _, err := l.OpenTrade(sig, 100000, 0.5); err == nil {
		t.Error("third OpenTrade succeeded; want per-strategy cap error")
	}
	// Cap is per strategy — a different strategy is unaffected.
	if !l.CanOpen("OtherStrat") {
		t.Error("cap leaked across strategies")
	}
	// Closing one position frees a slot.
	l.CheckAndClose(94000) // short TP hit
	if !l.CanOpen("CapStrat") {
		t.Error("CanOpen still false after positions closed")
	}
}

// TestCheckAndCloseTimeExit pins the max-age force close that mirrors the live
// manager's CheckExpiredPositions (MaxPositionAgeMins=240). Without it, shadow
// losers that never hit SL ride forever and never count against the stats.
func TestCheckAndCloseTimeExit(t *testing.T) {
	l := newTestLedger()

	stale, err := l.OpenTrade(Signal{
		Strategy:   "StaleStrat",
		Direction:  DirectionLong,
		StopLoss:   50000, // far away — never hit in this test
		TakeProfit: 200000,
	}, 100000, 0.5)
	if err != nil {
		t.Fatalf("OpenTrade failed: %v", err)
	}
	if _, err := l.OpenTrade(Signal{
		Strategy:   "FreshStrat",
		Direction:  DirectionLong,
		StopLoss:   50000,
		TakeProfit: 200000,
	}, 100000, 0.5); err != nil {
		t.Fatalf("OpenTrade failed: %v", err)
	}
	// Age the first trade past the 240-minute limit.
	stale.OpenedAt = time.Now().UTC().Add(-241 * time.Minute)

	closed := l.CheckAndClose(100500) // between SL and TP for both
	if len(closed) != 1 {
		t.Fatalf("expected exactly 1 TIME close, got %d", len(closed))
	}
	if closed[0].StrategyName != "StaleStrat" || closed[0].ExitReason != "TIME" {
		t.Errorf("closed %s reason=%s, want StaleStrat reason=TIME",
			closed[0].StrategyName, closed[0].ExitReason)
	}
	if l.CountOpen("FreshStrat") != 1 {
		t.Error("fresh trade was closed by the age check")
	}
}

// TestPerformanceAggregation sanity-checks win/loss counting and total NetPnL
// through the running-perf path used by ShadowPromoter.CanPromote.
func TestPerformanceAggregation(t *testing.T) {
	l := newTestLedger()

	// Win: long closed at TP.
	if _, err := l.OpenTrade(Signal{Strategy: "PerfStrat", Direction: DirectionLong, StopLoss: 99000, TakeProfit: 101000}, 100000, 0.5); err != nil {
		t.Fatalf("OpenTrade failed: %v", err)
	}
	l.CheckAndClose(101500)
	// Loss: long closed at SL.
	if _, err := l.OpenTrade(Signal{Strategy: "PerfStrat", Direction: DirectionLong, StopLoss: 99000, TakeProfit: 101000}, 100000, 0.5); err != nil {
		t.Fatalf("OpenTrade failed: %v", err)
	}
	l.CheckAndClose(98500)

	perf := l.GetPerformance("PerfStrat")
	if perf.TotalTrades != 2 || perf.WinCount != 1 || perf.LossCount != 1 {
		t.Errorf("trades/wins/losses = %d/%d/%d, want 2/1/1",
			perf.TotalTrades, perf.WinCount, perf.LossCount)
	}
	if perf.WinRate != 0.5 {
		t.Errorf("WinRate = %.2f, want 0.50", perf.WinRate)
	}
	var wantTotal float64
	for _, c := range l.GetClosedTrades("PerfStrat", 0) {
		wantTotal += c.NetPnL
	}
	if math.Abs(perf.TotalNetPnL-math.Round(wantTotal*100)/100) > 0.01 {
		t.Errorf("TotalNetPnL = %.2f, want %.2f (sum of closed trades)", perf.TotalNetPnL, wantTotal)
	}
}
