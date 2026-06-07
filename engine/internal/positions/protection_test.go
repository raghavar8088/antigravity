package positions

import (
	"testing"

	"antigravity-engine/internal/strategy"
)

func TestEnsurePositionProtectionDefaultsMissingSLTP(t *testing.T) {
	pos := Position{
		ID:         "POS-test",
		Symbol:     "BTC-USD",
		Side:       strategy.ActionBuy,
		EntryPrice: 100_000,
		Size:       0.01,
		Status:     "OPEN",
	}
	if err := EnsurePositionProtection(&pos); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pos.StopLoss <= 0 || pos.TakeProfit <= 0 {
		t.Fatalf("expected SL/TP defaults, got sl=%.2f tp=%.2f", pos.StopLoss, pos.TakeProfit)
	}
	if pos.TakeProfit <= pos.EntryPrice {
		t.Fatalf("long TP must be above entry")
	}
}

func TestValidateOpenSignalRejectsMissingStop(t *testing.T) {
	err := ValidateOpenSignal(strategy.Signal{
		Symbol:        "BTC-USD",
		Action:        strategy.ActionBuy,
		TargetSize:    0.01,
		TakeProfitPct: 0.5,
	})
	if err == nil {
		t.Fatal("expected error for missing stop loss pct")
	}
}
