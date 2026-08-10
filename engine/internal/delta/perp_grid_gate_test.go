package delta

import (
	"strings"
	"testing"
)

// COOKIEUSD is the case this gate exists for.
//
// It ticks at 1e-05 against a ~0.0125 price, so a 0.64% stop is 8 ticks wide
// and every rounding, trigger and fill moves in steps worth an eighth of the
// planned risk. Its stop-outs ran 1.5x-1.75x and one could not be bracketed at
// all — the engine funded the position first and discovered that after.
func TestStopSurvivesGrid_RefusesACoarseGrid(t *testing.T) {
	reg := riskTestRegistry(t)

	// Reproduce COOKIEUSD's geometry, not its symbol: entry 0.01249 with a stop
	// at 0.01257 is 0.00008 of price against a 1e-05 tick — 8 ticks.
	//
	// The fixture has no coin that cheap, so the same 8-tick DISTANCE is used on
	// ADAUSD. What makes a grid coarse is the stop's width in ticks, not the
	// symbol: ADAUSD at 0.1726 is fine-grained for an ordinary 0.35% stop (60
	// ticks) and just as unworkable as COOKIEUSD when the stop is this narrow.
	entry := 0.17258828
	stop := entry - 8*0.00001

	reason := stopSurvivesGrid(reg, "ADAUSD", entry, stop)
	if reason == "" {
		t.Fatal("a ~6-tick stop was permitted; this is the geometry that produced unbracketed positions")
	}
	// The refusal must say WHY — "refused" without a cause is how a desk goes
	// quiet for a week before anyone works out the reason.
	for _, want := range []string{"ticks", "ADAUSD"} {
		if !strings.Contains(reason, want) {
			t.Errorf("refusal %q does not mention %q", reason, want)
		}
	}
}

// A stop with real room must pass, or the gate simply stops the desk trading.
func TestStopSurvivesGrid_AllowsAWorkableStop(t *testing.T) {
	reg := riskTestRegistry(t)
	entry := 0.17258828
	// 2% stop = ~345 ticks.
	if reason := stopSurvivesGrid(reg, "ADAUSD", entry, entry*0.98); reason != "" {
		t.Errorf("a 2%% stop was refused: %s", reason)
	}
}

// A missing grid must PERMIT. Refusing every order because a registry field is
// absent is a worse failure than the one being prevented, and it presents as
// the desk silently declining to trade.
func TestStopSurvivesGrid_UnknownProductPermits(t *testing.T) {
	reg := riskTestRegistry(t)
	if reason := stopSurvivesGrid(reg, "NOSUCHUSD", 1, 0.99); reason != "" {
		t.Errorf("unknown product was refused: %s", reason)
	}
	if reason := stopSurvivesGrid(nil, "ADAUSD", 1, 0.99); reason != "" {
		t.Errorf("nil registry was refused: %s", reason)
	}
}

// The entry gate must be strictly stronger than the bracket layer's 2-tick
// check, or a position can still be opened that cannot be bracketed.
func TestStopSurvivesGrid_IsStricterThanTheBracketCheck(t *testing.T) {
	if minEntryStopTicks <= minStopTicks {
		t.Fatalf("entry gate %d ticks is not stricter than the bracket check %g ticks — "+
			"a position could still be funded and then found unprotectable",
			minEntryStopTicks, minStopTicks)
	}
}
