package risk

import "testing"

func TestStrategyTrackerDisablesAfterFiveConsecutiveLosses(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100000)

	tracker.RecordTradeResult("A", -10)
	tracker.RecordTradeResult("A", -8)
	tracker.RecordTradeResult("A", -7)
	tracker.RecordTradeResult("A", -6)
	tracker.RecordTradeResult("A", -5)

	stats, ok := tracker.GetStats("A")
	if !ok {
		t.Fatal("expected strategy stats to exist")
	}
	if !stats.Disabled {
		t.Fatal("expected strategy to be disabled after five consecutive losses")
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
	if got := tracker.GetExecutionWeight("A"); got != 1.10 {
		t.Fatalf("expected cold-start execution weight 1.10, got %.2f", got)
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

// ── BuildRiskMetrics tests ────────────────────────────────────────────────────

func TestBuildRiskMetrics_ColdStart(t *testing.T) {
	tracker := NewStrategyTracker([]string{"X"}, []string{"Trend"}, []string{"1m"}, 100_000)
	m := tracker.BuildRiskMetrics("X")

	if m.Strategy != "X" {
		t.Fatalf("expected strategy name X, got %s", m.Strategy)
	}
	// Cold start: TotalTrades==0 → neutral WinRate, zero OOS fields
	if m.WinRate != 0.5 {
		t.Fatalf("cold-start WinRate should be 0.5, got %.3f", m.WinRate)
	}
	if m.OOSProfitFactor != 0 {
		t.Fatalf("cold-start OOSProfitFactor should be 0 (conservative), got %.3f", m.OOSProfitFactor)
	}
	if m.OOSExpectancyUSD != 0 {
		t.Fatalf("cold-start OOSExpectancyUSD should be 0, got %.3f", m.OOSExpectancyUSD)
	}
	if m.TotalTrades != 0 {
		t.Fatalf("expected 0 trades, got %d", m.TotalTrades)
	}
	if m.HealthScore != 50 {
		t.Fatalf("cold-start health should be neutral 50, got %.1f", m.HealthScore)
	}
}

func TestBuildRiskMetrics_UnknownStrategy(t *testing.T) {
	tracker := NewStrategyTracker([]string{"A"}, []string{"Trend"}, []string{"1m"}, 100_000)
	m := tracker.BuildRiskMetrics("DOES_NOT_EXIST")
	if m.Strategy != "DOES_NOT_EXIST" {
		t.Fatalf("expected strategy name DOES_NOT_EXIST, got %s", m.Strategy)
	}
	if m.TotalTrades != 0 {
		t.Fatalf("unknown strategy should have 0 trades, got %d", m.TotalTrades)
	}
}

func TestBuildRiskMetrics_RealData_WinsAndLosses(t *testing.T) {
	tracker := NewStrategyTracker([]string{"Y"}, []string{"Breakout"}, []string{"5m"}, 100_000)

	// 7 wins ($20 each), 3 losses ($10 each)
	for i := 0; i < 7; i++ {
		tracker.RecordTradeResult("Y", 20)
	}
	for i := 0; i < 3; i++ {
		tracker.RecordTradeResult("Y", -10)
	}

	m := tracker.BuildRiskMetrics("Y")

	// WinRate = 7/10 = 0.70
	if m.WinRate < 0.69 || m.WinRate > 0.71 {
		t.Fatalf("expected WinRate ~0.70, got %.3f", m.WinRate)
	}
	// ProfitFactor = (7*20) / (3*10) = 140/30 ≈ 4.67
	if m.ProfitFactor < 4.5 || m.ProfitFactor > 4.8 {
		t.Fatalf("expected ProfitFactor ~4.67, got %.3f", m.ProfitFactor)
	}
	// OOS fields must mirror live PF/Expectancy when TotalTrades > 0
	if m.OOSProfitFactor != m.ProfitFactor {
		t.Fatalf("OOSProfitFactor should equal live ProfitFactor, got oos=%.3f pf=%.3f", m.OOSProfitFactor, m.ProfitFactor)
	}
	if m.OOSExpectancyUSD != m.ExpectancyUSD {
		t.Fatalf("OOSExpectancyUSD should equal live ExpectancyUSD, got oos=%.3f exp=%.3f", m.OOSExpectancyUSD, m.ExpectancyUSD)
	}
	// ExpectancyUSD = 0.70*20 - 0.30*10 = 14 - 3 = 11
	if m.ExpectancyUSD < 10.5 || m.ExpectancyUSD > 11.5 {
		t.Fatalf("expected ExpectancyUSD ~11, got %.3f", m.ExpectancyUSD)
	}
	if m.TotalTrades != 10 {
		t.Fatalf("expected 10 trades, got %d", m.TotalTrades)
	}
	// AverageLossUSD should be negative (riskv2 convention)
	if m.AverageLossUSD >= 0 {
		t.Fatalf("AverageLossUSD should be negative, got %.3f", m.AverageLossUSD)
	}
}

func TestBuildRiskMetrics_AllWins_ProfitFactorCapped(t *testing.T) {
	tracker := NewStrategyTracker([]string{"Z"}, []string{"Momentum"}, []string{"1m"}, 100_000)
	for i := 0; i < 5; i++ {
		tracker.RecordTradeResult("Z", 15)
	}
	m := tracker.BuildRiskMetrics("Z")
	// No losses → profitFactor set to 3.0 (sentinel for "all wins")
	if m.ProfitFactor != 3.0 {
		t.Fatalf("expected PF=3.0 for all-win strategy, got %.3f", m.ProfitFactor)
	}
	if m.OOSProfitFactor != 3.0 {
		t.Fatalf("expected OOSProfitFactor=3.0, got %.3f", m.OOSProfitFactor)
	}
}

func TestBuildRiskMetrics_FamilyMapping(t *testing.T) {
	families := map[string]string{
		"Trend":          "TREND",
		"Breakout":       "BREAKOUT",
		"Mean Reversion": "MEAN_REVERSION",
		"Microstructure": "ORDER_FLOW",
		"Smart Money":    "SMART_MONEY",
		"Volume Elite":   "VOLUME_PROFILE",
		"Adaptive":       "MSS",
		"Unknown Cat":    "RESERVE",
	}
	names := make([]string, 0, len(families))
	cats := make([]string, 0, len(families))
	for cat, _ := range families {
		names = append(names, "strat_"+cat)
		cats = append(cats, cat)
	}
	tracker := NewStrategyTracker(names, cats, make([]string, len(names)), 100_000)
	for i, name := range names {
		m := tracker.BuildRiskMetrics(name)
		expected := families[cats[i]]
		if string(m.Family) != expected {
			t.Errorf("category %q: expected family %s, got %s", cats[i], expected, m.Family)
		}
	}
}

func TestBuildRiskMetrics_DrawdownTracking(t *testing.T) {
	tracker := NewStrategyTracker([]string{"D"}, []string{"Trend"}, []string{"1m"}, 100_000)
	tracker.RecordTradeResult("D", 100)  // peak = 100
	tracker.RecordTradeResult("D", -30) // drawdown from 100 → 70 = 30%
	m := tracker.BuildRiskMetrics("D")
	if m.MaxDrawdownPct <= 0 {
		t.Fatalf("expected positive MaxDrawdownPct after drawdown, got %.3f", m.MaxDrawdownPct)
	}
}

func TestBuildRiskMetrics_KellyIntegration(t *testing.T) {
	// Verify that Kelly receives non-zero OOS fields from BuildRiskMetrics,
	// producing higher confidence for profitable strategies.
	tracker := NewStrategyTracker([]string{"K"}, []string{"Trend"}, []string{"1m"}, 100_000)
	for i := 0; i < 20; i++ {
		tracker.RecordTradeResult("K", 25)
	}
	for i := 0; i < 8; i++ {
		tracker.RecordTradeResult("K", -10)
	}
	m := tracker.BuildRiskMetrics("K")

	if m.OOSProfitFactor <= 1.0 {
		t.Fatalf("expected OOSProfitFactor > 1.0 for profitable strategy, got %.3f", m.OOSProfitFactor)
	}
	if m.OOSExpectancyUSD <= 0 {
		t.Fatalf("expected positive OOSExpectancyUSD for profitable strategy, got %.3f", m.OOSExpectancyUSD)
	}
}
