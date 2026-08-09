package delta

import "testing"

// Switching a stream off must not touch the same strategy on other symbols.
//
// ANTI_Recurrence_Quantification_Signal runs on COOKIEUSD, MUBARAKUSD and
// BLESSUSD as three independent positions. The usual reason to stop one is the
// instrument — COOKIEUSD's tick grid is too coarse for its stops — and stopping
// the logic everywhere because one venue misbehaved would be a much larger
// action than the click implies.
func TestStreamSwitch_IsPerSymbolNotPerStrategy(t *testing.T) {
	b := &PerpBridge{}
	const s = "ANTI_Recurrence_Quantification_Signal"

	b.SetStrategyEnabled(s, "COOKIEUSD", false)

	if b.StrategyEnabled(s, "COOKIEUSD") {
		t.Error("COOKIEUSD stream still enabled after being switched off")
	}
	for _, other := range []string{"MUBARAKUSD", "BLESSUSD"} {
		if !b.StrategyEnabled(s, other) {
			t.Errorf("switching off %s on COOKIEUSD also disabled it on %s", s, other)
		}
	}
}

// Symbol casing must not create two independent switches for one stream — a
// stream switched off as "cookieusd" and still trading as "COOKIEUSD" is the
// worst kind of half-applied control.
func TestStreamSwitch_SymbolCaseInsensitive(t *testing.T) {
	b := &PerpBridge{}
	b.SetStrategyEnabled("ANTI_D20_MACD_Cross", "cookieusd", false)
	if b.StrategyEnabled("ANTI_D20_MACD_Cross", "COOKIEUSD") {
		t.Error("lowercase switch did not apply to the uppercase stream")
	}
}

// Both fields are required: a bare strategy would have to mean "every symbol",
// and switching off three streams when one was named is not a thing a control
// should do silently.
func TestStreamSwitch_RequiresBothFields(t *testing.T) {
	b := &PerpBridge{}
	b.SetStrategyEnabled("ANTI_D20_MACD_Cross", "", false)
	b.SetStrategyEnabled("", "COOKIEUSD", false)
	if got := b.DisabledStrategies(); len(got) != 0 {
		t.Errorf("incomplete request created entries: %v", got)
	}
}
