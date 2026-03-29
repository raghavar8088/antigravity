package positions

import (
	"testing"

	"antigravity-engine/internal/strategy"
)

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
}
