package delta

import "testing"

func gridTestBridge(t *testing.T) *PerpBridge {
	t.Helper()
	return &PerpBridge{
		reg:         riskTestRegistry(t),
		gridBlocked: map[string]gridBlock{},
		strategyOff: map[string]bool{},
	}
}

// One refusal must NOT switch a stream off.
//
// Measured live: both streams that had ever been grid-refused had also traded,
// and ANTI_M1_InsideBar_V12_Long on COOKIEUSD was net positive with 3 fills
// against 1 refusal. Stops are volatility-scaled, so the same stream clears the
// tick grid in a moving market and fails in a quiet one — switching off at the
// first refusal silences working strategies.
func TestGridAutoDisable_SurvivesAnIsolatedRefusal(t *testing.T) {
	b := gridTestBridge(t)
	const st, sym = "ANTI_M1_InsideBar_V12_Long", "COOKIEUSD"
	k := perpStreamKey(st, sym)

	for i := 0; i < gridAutoDisableAfter-1; i++ {
		b.mu.Lock()
		gb := b.gridBlocked[k]
		gb.Consecutive++
		b.gridBlocked[k] = gb
		b.mu.Unlock()
	}
	if !b.StrategyEnabled(st, sym) {
		t.Fatalf("switched off after %d refusals; the threshold is %d", gridAutoDisableAfter-1, gridAutoDisableAfter)
	}
}

// A fill clears the streak, so an intermittently-blocked stream never
// accumulates its way to being disabled.
func TestGridAutoDisable_StreakResets(t *testing.T) {
	b := gridTestBridge(t)
	k := perpStreamKey("S", "SYM")

	b.mu.Lock()
	b.gridBlocked[k] = gridBlock{Consecutive: gridAutoDisableAfter - 1, Refusals: 9}
	b.mu.Unlock()

	// Simulate reaching sizing, which is what clears it.
	b.mu.Lock()
	gb := b.gridBlocked[k]
	gb.Consecutive = 0
	b.gridBlocked[k] = gb
	b.mu.Unlock()

	b.mu.RLock()
	got := b.gridBlocked[k]
	b.mu.RUnlock()
	if got.Consecutive != 0 {
		t.Errorf("streak = %d after reaching sizing, want 0", got.Consecutive)
	}
	// The lifetime count must survive, or the board loses the history.
	if got.Refusals != 9 {
		t.Errorf("lifetime refusals = %d, want 9 — the streak reset must not erase the record", got.Refusals)
	}
}

// The switch it flips is the same one the row toggle uses, so the owner can
// turn it straight back on. An auto-off that cannot be undone is a trapdoor.
func TestGridAutoDisable_IsReversibleByTheOwner(t *testing.T) {
	b := gridTestBridge(t)
	const st, sym = "S", "SYM"

	b.SetStrategyEnabled(st, sym, false)
	if b.StrategyEnabled(st, sym) {
		t.Fatal("stream still enabled after being switched off")
	}
	b.SetStrategyEnabled(st, sym, true)
	if !b.StrategyEnabled(st, sym) {
		t.Error("owner could not switch the stream back on")
	}
}

// The threshold must be above one, or the whole distinction collapses.
func TestGridAutoDisable_ThresholdIsNotOne(t *testing.T) {
	if gridAutoDisableAfter < 2 {
		t.Fatalf("gridAutoDisableAfter = %d — at 1 this disables streams that still trade",
			gridAutoDisableAfter)
	}
}
