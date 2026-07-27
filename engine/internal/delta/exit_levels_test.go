package delta

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// The displayed TP/SL must be the levels the monitor actually closes on
// (+80% / -50% of the entry premium), and their USD outcomes must use the real
// 0.001 BTC contract size.
func TestExitLevelsFor(t *testing.T) {
	// Entry premium 100 USD/BTC, 1 contract => $0.10 premium paid.
	e := ExitLevelsFor(100, 1)
	if !approx(e.TakeProfitPrice, 180) {
		t.Fatalf("TP price got %v want 180", e.TakeProfitPrice)
	}
	if !approx(e.StopLossPrice, 50) {
		t.Fatalf("SL price got %v want 50", e.StopLossPrice)
	}
	// +80% of a $0.10 premium = +$0.08 ; -50% = -$0.05
	if !approx(e.TakeProfitUSD, 0.08) {
		t.Fatalf("TP usd got %v want 0.08", e.TakeProfitUSD)
	}
	if !approx(e.StopLossUSD, -0.05) {
		t.Fatalf("SL usd got %v want -0.05", e.StopLossUSD)
	}
	// The SL outcome must be a loss, the TP a gain — sign discipline matters.
	if e.StopLossUSD >= 0 || e.TakeProfitUSD <= 0 {
		t.Fatal("TP must be positive and SL negative")
	}
}

func TestExitLevelsFor_ZeroInputsAreSafe(t *testing.T) {
	if got := ExitLevelsFor(0, 1); got != (PositionExit{}) {
		t.Fatalf("no entry premium => no levels, got %+v", got)
	}
	if got := ExitLevelsFor(100, 0); got != (PositionExit{}) {
		t.Fatalf("no contracts => no levels, got %+v", got)
	}
}

// Delta reports unrealised_pnl as 0 for these option positions, so the engine
// computes mark-to-market itself rather than showing a false zero on a real loss.
func TestUnrealizedUSD(t *testing.T) {
	// entry 100, mark 64.71, 1 contract => (64.71-100) * 0.001 = -0.03529
	got := UnrealizedUSD(100, 64.71, 1)
	if !approx(got, -0.03529) {
		t.Fatalf("unrealized got %v want -0.03529", got)
	}
	if UnrealizedUSD(100, 180, 1) <= 0 {
		t.Fatal("a mark above entry must be a gain")
	}
	if UnrealizedUSD(0, 50, 1) != 0 || UnrealizedUSD(100, 50, 0) != 0 {
		t.Fatal("zero entry or zero contracts must be 0, not a bogus number")
	}
}
