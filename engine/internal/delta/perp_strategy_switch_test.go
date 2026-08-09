package delta

import "testing"

// Unknown strategies must trade. The switch stores the DISABLED set, so a
// strategy added to the roster later behaves as the roster says instead of
// silently sitting out because it was missing from a saved enable-list.
func TestStrategySwitch_UnknownStrategyTrades(t *testing.T) {
	b := &PerpBridge{}
	if !b.StrategyEnabled("ANTI_M1_NR7_Expand_T20_Long") {
		t.Error("an unseen strategy is switched off; the desk would quietly stop trading new roster entries")
	}
}

func TestStrategySwitch_OffThenOnIsReversible(t *testing.T) {
	b := &PerpBridge{}
	const name = "ANTI_D20_VWAP_Reversion"

	b.SetStrategyEnabled(name, false)
	if b.StrategyEnabled(name) {
		t.Fatal("strategy still enabled after being switched off")
	}
	// Only that one.
	if !b.StrategyEnabled("ANTI_D20_MACD_Cross") {
		t.Error("switching one strategy off disabled another")
	}

	b.SetStrategyEnabled(name, true)
	if !b.StrategyEnabled(name) {
		t.Error("strategy did not come back on; a switch that cannot be reversed is a one-way door")
	}
	if len(b.DisabledStrategies()) != 0 {
		t.Errorf("re-enabling left residue: %v", b.DisabledStrategies())
	}
}

// Blank names must not create a phantom entry that shows up in the UI as a
// switched-off strategy nobody can turn back on.
func TestStrategySwitch_IgnoresBlankNames(t *testing.T) {
	b := &PerpBridge{}
	b.SetStrategyEnabled("", false)
	b.SetStrategyEnabled("   ", false)
	if got := b.DisabledStrategies(); len(got) != 0 {
		t.Errorf("blank name produced entries: %v", got)
	}
}

// The switch state must survive a restart: unlike the arm state, which is
// deliberately forgotten, turning a strategy off is a standing decision.
func TestStrategySwitch_RestoresFromDisk(t *testing.T) {
	b := &PerpBridge{}
	b.setDisabledStrategiesLocked([]string{"ANTI_D20_MACD_Cross", " ", "ANTI_M1_HMA21_Flip_Short"})

	if b.StrategyEnabled("ANTI_D20_MACD_Cross") {
		t.Error("restored disable did not apply")
	}
	if !b.StrategyEnabled("ANTI_M1_NR7_Expand_T20_Long") {
		t.Error("restore disabled a strategy that was never switched off")
	}
	if got := len(b.DisabledStrategies()); got != 2 {
		t.Errorf("restored %d disabled strategies, want 2 (the blank must be dropped)", got)
	}
}
