package trading

import (
	"testing"
	"time"
)

func TestExecutionWatchdog_NoTradeThresholds(t *testing.T) {
	w := NewExecutionWatchdog(nil, nil)
	now := time.Now()
	lastFill := now.Add(-7 * time.Hour)

	w.checkNoTradeThresholds(now, now.Sub(lastFill), lastFill)

	fired := w.noTradeAlertFired.Load()
	if fired&(1<<0) == 0 {
		t.Fatal("expected 1h no-trade alert fired")
	}
	if fired&(1<<1) == 0 {
		t.Fatal("expected 6h no-trade alert fired")
	}
	if fired&(1<<2) != 0 {
		t.Fatal("did not expect 12h alert for 7h gap")
	}
}

func TestExecutionWatchdog_HealthSnapshot(t *testing.T) {
	w := NewExecutionWatchdog(nil, nil)
	w.RecordTick()
	w.RecordSignal()
	h := w.Health()
	if h.LastTickAt.IsZero() || h.LastSignalAt.IsZero() {
		t.Fatal("health snapshot should include tick and signal timestamps")
	}
}
