package execution

import (
	"testing"

	"antigravity-engine/internal/strategy"
)

func TestSettlePositionShortCoverClampsBalance(t *testing.T) {
	p := NewPaperClient(100)
	p.balanceUSD = 50

	p.SettlePosition(strategy.ActionSell, 1, 100)

	if p.balanceUSD < 0 {
		t.Fatalf("balance went negative: %.4f", p.balanceUSD)
	}
	if p.balanceUSD != 0 {
		t.Fatalf("expected balance clamped to 0, got %.4f", p.balanceUSD)
	}
}

func TestRestoreBalanceClampsNegative(t *testing.T) {
	p := NewPaperClient(1000)
	p.RestoreBalance(-0.29, 0)
	if p.balanceUSD != 0 {
		t.Fatalf("expected 0, got %.4f", p.balanceUSD)
	}
}
