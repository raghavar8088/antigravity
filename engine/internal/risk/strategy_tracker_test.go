package risk

import "testing"

func TestStrategyTrackerDisablesAfterThreeConsecutiveLosses(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	tracker.RecordTradeResult("A", -10)
	tracker.RecordTradeResult("A", -8)
	tracker.RecordTradeResult("A", -7)

	stats, ok := tracker.GetStats("A")
	if !ok {
		t.Fatal("expected strategy stats to exist")
	}
	if !stats.Disabled {
		t.Fatal("expected strategy to be disabled after three consecutive losses")
	}
	if stats.Status != "COOLDOWN" {
		t.Fatalf("expected COOLDOWN status, got %s", stats.Status)
	}
}

func TestStrategyTrackerDisablesUnderperformingStrategy(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	tracker.RecordTradeResult("A", 1)
	tracker.RecordTradeResult("A", -5)
	tracker.RecordTradeResult("A", -5)
	tracker.RecordTradeResult("A", -5)
	tracker.RecordTradeResult("A", -5)
	tracker.RecordTradeResult("A", -5)

	stats, ok := tracker.GetStats("A")
	if !ok {
		t.Fatal("expected strategy stats to exist")
	}
	if !stats.Disabled {
		t.Fatal("expected underperforming strategy to be disabled")
	}
	if stats.Status != "UNDERPERFORMING" && stats.Status != "COOLDOWN" {
		t.Fatalf("expected UNDERPERFORMING or COOLDOWN status, got %s", stats.Status)
	}
}
