package positions

import (
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/strategy"
)

const floatTolerance = 1e-9

func TestLongPartialTakeProfitEmitsEventAndKeepsPositionOpen(t *testing.T) {
	mgr := NewManager()
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 1,
	}

	pos := mgr.OpenPosition(sig, 100, "Test")
	mgr.CheckStopLossAndTakeProfit(101)

	select {
	case event := <-mgr.CloseEvents:
		if event.Reason != ReasonTakeProfit {
			t.Fatalf("expected take profit event, got %s", event.Reason)
		}
		if event.Position.Size != 0.5 {
			t.Fatalf("expected partial size 0.5, got %.2f", event.Position.Size)
		}
		if event.Position.ID != pos.ID+"-TP1" {
			t.Fatalf("expected partial id suffix, got %s", event.Position.ID)
		}
	default:
		t.Fatal("expected partial take profit event")
	}

	positions := mgr.GetOpenPositions()
	if len(positions) != 1 {
		t.Fatalf("expected one remaining open position, got %d", len(positions))
	}
	if positions[0].Size != 0.5 {
		t.Fatalf("expected remaining size 0.5, got %.2f", positions[0].Size)
	}
	if !positions[0].PartialClosed {
		t.Fatal("expected position to be marked partial closed")
	}
	if positions[0].BreakEvenMoved {
		t.Fatal("expected partial take profit to avoid moving stop to break even")
	}
}

func TestLongPositionDoesNotAutoMoveToBreakEven(t *testing.T) {
	mgr := NewManager()
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 1,
	}

	pos := mgr.OpenPosition(sig, 100, "NoBreakEven")
	originalStop := pos.StopLoss

	mgr.CheckStopLossAndTakeProfit(100.31)

	positions := mgr.GetOpenPositions()
	if len(positions) != 1 {
		t.Fatalf("expected one open position, got %d", len(positions))
	}
	if math.Abs(positions[0].StopLoss-originalStop) > floatTolerance {
		t.Fatalf("expected stop loss to remain %.4f, got %.4f", originalStop, positions[0].StopLoss)
	}
	if positions[0].BreakEvenMoved {
		t.Fatal("expected break-even flag to remain disabled")
	}
}

func TestOpenPositionReversesLongStopLossAndTakeProfit(t *testing.T) {
	mgr := NewManager()
	mgr.config.ReverseTargets = true
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   0.5,
		TakeProfitPct: 1.5,
	}

	pos := mgr.OpenPosition(sig, 100, "ReverseLong")

	if pos.StopLossPct != 1.5 {
		t.Fatalf("expected reversed stop loss pct 1.5, got %.2f", pos.StopLossPct)
	}
	if pos.TakeProfitPct != 0.5 {
		t.Fatalf("expected reversed take profit pct 0.5, got %.2f", pos.TakeProfitPct)
	}
	if math.Abs(pos.StopLoss-98.5) > floatTolerance {
		t.Fatalf("expected stop loss 98.5, got %.4f", pos.StopLoss)
	}
	if math.Abs(pos.TakeProfit-100.5) > floatTolerance {
		t.Fatalf("expected take profit 100.5, got %.4f", pos.TakeProfit)
	}
}

func TestOpenPositionReversesShortStopLossAndTakeProfit(t *testing.T) {
	mgr := NewManager()
	mgr.config.ReverseTargets = true
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionSell,
		TargetSize:    1,
		StopLossPct:   0.4,
		TakeProfitPct: 1.2,
	}

	pos := mgr.OpenPosition(sig, 100, "ReverseShort")

	if pos.StopLossPct != 1.2 {
		t.Fatalf("expected reversed stop loss pct 1.2, got %.2f", pos.StopLossPct)
	}
	if pos.TakeProfitPct != 0.4 {
		t.Fatalf("expected reversed take profit pct 0.4, got %.2f", pos.TakeProfitPct)
	}
	if math.Abs(pos.StopLoss-101.2) > floatTolerance {
		t.Fatalf("expected stop loss 101.2, got %.4f", pos.StopLoss)
	}
	if math.Abs(pos.TakeProfit-99.6) > floatTolerance {
		t.Fatalf("expected take profit 99.6, got %.4f", pos.TakeProfit)
	}
}

func TestOpenPositionAppliesTakeProfitFloor(t *testing.T) {
	mgr := NewManager()
	mgr.config.ReverseTargets = true
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   0.10,
		TakeProfitPct: 1.00,
	}

	pos := mgr.OpenPosition(sig, 100, "TakeProfitFloor")

	if math.Abs(pos.StopLossPct-1.0) > floatTolerance {
		t.Fatalf("expected reversed stop loss pct 1.0, got %.4f", pos.StopLossPct)
	}
	if math.Abs(pos.TakeProfitPct-0.35) > floatTolerance {
		t.Fatalf("expected take profit floor 0.35, got %.4f", pos.TakeProfitPct)
	}
	if math.Abs(pos.TakeProfit-100.35) > floatTolerance {
		t.Fatalf("expected take profit 100.35, got %.4f", pos.TakeProfit)
	}
}

func TestOpenPositionUsesNormalTargetsByDefault(t *testing.T) {
	mgr := NewManager()
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   0.4,
		TakeProfitPct: 1.1,
	}

	pos := mgr.OpenPosition(sig, 100, "DefaultTargets")

	if math.Abs(pos.StopLossPct-0.4) > floatTolerance {
		t.Fatalf("expected stop loss pct 0.4, got %.4f", pos.StopLossPct)
	}
	if math.Abs(pos.TakeProfitPct-1.1) > floatTolerance {
		t.Fatalf("expected take profit pct 1.1, got %.4f", pos.TakeProfitPct)
	}
	if math.Abs(pos.StopLoss-99.6) > floatTolerance {
		t.Fatalf("expected stop loss 99.6, got %.4f", pos.StopLoss)
	}
	if math.Abs(pos.TakeProfit-101.1) > floatTolerance {
		t.Fatalf("expected take profit 101.1, got %.4f", pos.TakeProfit)
	}
}

func TestCheckExpiredPositionsClosesStalePosition(t *testing.T) {
	mgr := NewManager()
	mgr.config.MaxPositionAgeMins = 0.001 // ~60ms — expires almost immediately

	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 2,
	}
	mgr.OpenPosition(sig, 100, "ExpiryTest")

	// Position should still be alive immediately after opening.
	if len(mgr.GetOpenPositions()) != 1 {
		t.Fatal("expected one open position after open")
	}

	// Wait for the position to age past MaxPositionAgeMins.
	time.Sleep(100 * time.Millisecond)
	mgr.CheckExpiredPositions(100)

	if len(mgr.GetOpenPositions()) != 0 {
		t.Fatal("expected position to be expired and removed")
	}

	select {
	case event := <-mgr.CloseEvents:
		if event.Reason != ReasonManual {
			t.Fatalf("expected MANUAL close reason for expiry, got %s", event.Reason)
		}
	default:
		t.Fatal("expected a close event for the expired position")
	}
}

func TestCheckExpiredPositionsSkipsYoungPosition(t *testing.T) {
	mgr := NewManager()
	mgr.config.MaxPositionAgeMins = 60 // 60 minutes — position won't expire

	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 2,
	}
	mgr.OpenPosition(sig, 100, "YoungPosition")
	mgr.CheckExpiredPositions(100)

	if len(mgr.GetOpenPositions()) != 1 {
		t.Fatal("expected young position to survive expiry check")
	}
}

func TestCheckExpiredPositionsDisabledWhenZero(t *testing.T) {
	mgr := NewManager()
	mgr.config.MaxPositionAgeMins = 0 // disabled

	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 2,
	}
	// Manually backdate position to simulate old age.
	pos := mgr.OpenPosition(sig, 100, "OldButDisabled")
	mgr.mu.Lock()
	mgr.positions[pos.ID].OpenedAt = time.Now().Add(-120 * time.Minute)
	mgr.mu.Unlock()

	mgr.CheckExpiredPositions(100)

	if len(mgr.GetOpenPositions()) != 1 {
		t.Fatal("expected expiry to be skipped when MaxPositionAgeMins=0")
	}
}
