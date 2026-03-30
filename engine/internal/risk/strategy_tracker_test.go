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

func TestStrategyTrackerSizingMultiplierDefaultsAndUnknown(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	if got := tracker.GetSizingMultiplier("UNKNOWN"); got != 1.0 {
		t.Fatalf("expected default multiplier 1.0 for unknown strategy, got %.2f", got)
	}

	if got := tracker.GetSizingMultiplier("A"); got != 0.85 {
		t.Fatalf("expected cold-start multiplier 0.85, got %.2f", got)
	}
}

func TestStrategyTrackerSizingMultiplierBoostsStrongWinners(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	for i := 0; i < 8; i++ {
		tracker.RecordTradeResult("A", 8)
	}

	got := tracker.GetSizingMultiplier("A")
	if got <= 1.3 {
		t.Fatalf("expected strong winners to be boosted above 1.3, got %.2f", got)
	}
	if got > 1.6 {
		t.Fatalf("expected multiplier clamp at 1.6 max, got %.2f", got)
	}
}

func TestStrategyTrackerSizingMultiplierPenalizesLossStreaksAndDisable(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	tracker.RecordTradeResult("A", -5)
	tracker.RecordTradeResult("A", -5)
	mid := tracker.GetSizingMultiplier("A")
	if mid >= 0.6 {
		t.Fatalf("expected loss streak penalty to reduce sizing below 0.6, got %.2f", mid)
	}

	// Third loss triggers cooldown/disable; disabled strategies return min multiplier.
	tracker.RecordTradeResult("A", -5)
	disabled := tracker.GetSizingMultiplier("A")
	if disabled != 0.35 {
		t.Fatalf("expected disabled strategy multiplier 0.35, got %.2f", disabled)
	}
}

func TestStrategyTrackerExecutionWeightDefaults(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	if got := tracker.GetExecutionWeight("UNKNOWN"); got != 1.0 {
		t.Fatalf("expected default execution weight 1.0 for unknown strategy, got %.2f", got)
	}
	if got := tracker.GetExecutionWeight("A"); got != 0.90 {
		t.Fatalf("expected cold-start execution weight 0.90, got %.2f", got)
	}
}

func TestStrategyTrackerExecutionWeightBoostsStrongMatureStrategy(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)
	for i := 0; i < 10; i++ {
		tracker.RecordTradeResult("A", 7)
	}

	got := tracker.GetExecutionWeight("A")
	if got <= 1.1 {
		t.Fatalf("expected mature strong strategy weight above 1.1, got %.2f", got)
	}
}

func TestStrategyTrackerExecutionWeightPenalizesWeakMatureStrategy(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)
	results := []float64{-5, 2, -6, -3, -4, 1, -2, -3, -1, -2}
	for _, pnl := range results {
		tracker.RecordTradeResult("A", pnl)
	}

	got := tracker.GetExecutionWeight("A")
	if got >= 0.8 {
		t.Fatalf("expected weak mature strategy weight below 0.8, got %.2f", got)
	}
}
