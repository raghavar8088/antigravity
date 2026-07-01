package positions

import (
	"math"
	"testing"
	"time"

	"antigravity-engine/internal/strategy"
)

const floatTolerance = 1e-9

func mustOpen(t *testing.T, mgr *Manager, sig strategy.Signal, entry float64, name string) *Position {
	t.Helper()
	pos, err := mgr.OpenPosition(sig, entry, name)
	if err != nil {
		t.Fatalf("OpenPosition: %v", err)
	}
	return pos
}

func TestLongPartialTakeProfitEmitsEventAndKeepsPositionOpen(t *testing.T) {
	mgr := NewManager()
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 1,
	}

	pos := mustOpen(t, mgr,sig, 100, "Test")
	mgr.CheckStopLossAndTakeProfit(101)

	select {
	case event := <-mgr.CloseEvents():
		if event.Reason != ReasonTakeProfit {
			t.Fatalf("expected take profit event, got %s", event.Reason)
		}
		if event.Position.Size != 1.0 {
			t.Fatalf("expected full-size close at TP, got %.2f", event.Position.Size)
		}
		if event.Position.ID != pos.ID {
			t.Fatalf("expected original position id, got %s", event.Position.ID)
		}
	default:
		t.Fatal("expected take profit event")
	}

	positions := mgr.GetOpenPositions()
	if len(positions) != 0 {
		t.Fatalf("expected full TP close to remove the position, got %d open positions", len(positions))
	}
}

func TestLongPositionKeepsFixedStopLossAfterProfitMove(t *testing.T) {
	mgr := NewManager()
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    1,
		StopLossPct:   1,
		TakeProfitPct: 1,
	}

	pos := mustOpen(t, mgr,sig, 100, "NoBreakEven")
	originalStop := pos.StopLoss

	mgr.CheckStopLossAndTakeProfit(100.31)

	positions := mgr.GetOpenPositions()
	if len(positions) != 1 {
		t.Fatalf("expected one open position, got %d", len(positions))
	}
	if math.Abs(positions[0].StopLoss-originalStop) > floatTolerance {
		t.Fatalf("expected stop loss to remain fixed at %.4f, got %.4f", originalStop, positions[0].StopLoss)
	}
	if positions[0].TrailingActive {
		t.Fatal("expected trailing to remain disabled")
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

	pos := mustOpen(t, mgr,sig, 100, "ReverseLong")

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

	pos := mustOpen(t, mgr,sig, 100, "ReverseShort")

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

	pos := mustOpen(t, mgr,sig, 100, "TakeProfitFloor")

	if math.Abs(pos.StopLossPct-1.0) > floatTolerance {
		t.Fatalf("expected reversed stop loss pct 1.0, got %.4f", pos.StopLossPct)
	}
	if math.Abs(pos.TakeProfitPct-0.30) > floatTolerance {
		t.Fatalf("expected take profit floor 0.30, got %.4f", pos.TakeProfitPct)
	}
	if math.Abs(pos.TakeProfit-100.30) > floatTolerance {
		t.Fatalf("expected take profit 100.30, got %.4f", pos.TakeProfit)
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

	pos := mustOpen(t, mgr,sig, 100, "DefaultTargets")

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
	mustOpen(t, mgr,sig, 100, "ExpiryTest")

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
	case event := <-mgr.CloseEvents():
		if event.Reason != ReasonExpired {
			t.Fatalf("expected EXPIRED close reason for expiry, got %s", event.Reason)
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
	mustOpen(t, mgr,sig, 100, "YoungPosition")
	mgr.CheckExpiredPositions(100)

	if len(mgr.GetOpenPositions()) != 1 {
		t.Fatal("expected young position to survive expiry check")
	}
}

func TestCalculatePnLWithFees(t *testing.T) {
	// LONG 0.001 BTC @ $60,000 — flat exit (price unchanged).
	// With FeeRatePct=0.0005: entry notional=$60, exit notional=$60.
	// Round-trip fee = ($60 + $60) × 0.0005 = $0.06. No price move → PnL = −$0.06.
	baseConfig := ManagerConfig{
		MinTakeProfitPct:   0.01,
		MaxPerStrategy:     99,
		MaxPositionAgeMins: 60,
		Leverage:           1.0,
	}
	sig := strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    0.001,
		StopLossPct:   5,
		TakeProfitPct: 5,
	}

	// --- with fees: flat exit should be a loss of $0.06 ---
	cfg := baseConfig
	cfg.FeeRatePct = 0.0005
	mgr := NewManagerWithConfig(cfg)
	pos := mustOpen(t, mgr, sig, 60000, "FeeTest")
	// Force-close at same price (no price move).
	// Use an internal helper: close at 60000 via checkLongPosition.
	// Patch TakeProfit to be exactly the current price so the TP branch fires.
	mgr.mu.Lock()
	mgr.positions[pos.ID].TakeProfit = 60000
	mgr.mu.Unlock()
	mgr.CheckStopLossAndTakeProfit(60000)

	ev := <-mgr.CloseEvents()
	want := -0.06
	if math.Abs(ev.PnL-want) > 1e-6 {
		t.Fatalf("with fees: expected PnL %.6f, got %.6f", want, ev.PnL)
	}

	// --- without fees: flat exit should be exactly zero ---
	cfg2 := baseConfig
	cfg2.FeeRatePct = 0
	mgr2 := NewManagerWithConfig(cfg2)
	pos2 := mustOpen(t, mgr2, sig, 60000, "NoFeeTest")
	mgr2.mu.Lock()
	mgr2.positions[pos2.ID].TakeProfit = 60000
	mgr2.mu.Unlock()
	mgr2.CheckStopLossAndTakeProfit(60000)

	ev2 := <-mgr2.CloseEvents()
	if math.Abs(ev2.PnL) > 1e-9 {
		t.Fatalf("without fees: expected PnL 0, got %.9f", ev2.PnL)
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
	pos := mustOpen(t, mgr,sig, 100, "OldButDisabled")
	mgr.mu.Lock()
	mgr.positions[pos.ID].OpenedAt = time.Now().Add(-120 * time.Minute)
	mgr.mu.Unlock()

	mgr.CheckExpiredPositions(100)

	if len(mgr.GetOpenPositions()) != 1 {
		t.Fatal("expected expiry to be skipped when MaxPositionAgeMins=0")
	}
}
