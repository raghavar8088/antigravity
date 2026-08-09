package main

import "testing"

// The option desk is off by default, and the roster must reflect that.
//
// It listed ~100 strategies all reading "NOT LIVE - 0 real fills / 0d", which
// describes a strategy waiting to qualify. None were waiting: they cannot trade
// at all while the switch is off, so the table was describing a desk that does
// not run.
func TestOptionDeskActive_DefaultsOff(t *testing.T) {
	t.Setenv("LIVE_ENGINE_OPTIONS_ENABLED", "")
	if optionDeskActive() {
		t.Error("option desk active with no env set; the owner's instruction is that it defaults OFF")
	}

	for _, off := range []string{"false", "0", "no", "FALSE", "  "} {
		t.Setenv("LIVE_ENGINE_OPTIONS_ENABLED", off)
		if optionDeskActive() {
			t.Errorf("option desk active for %q; only an explicit true may enable it", off)
		}
	}
}

// Turning it back on must still be possible, or the switch is a one-way door.
func TestOptionDeskActive_ExplicitTrueEnables(t *testing.T) {
	for _, on := range []string{"true", "TRUE", "True", " true "} {
		t.Setenv("LIVE_ENGINE_OPTIONS_ENABLED", on)
		if !optionDeskActive() {
			t.Errorf("option desk inactive for %q; the switch must be reversible", on)
		}
	}
}
