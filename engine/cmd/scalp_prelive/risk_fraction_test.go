package main

import "testing"

// The default must stay 2% — an override is opt-in, not a silent change.
func TestScalpLiveRiskFraction_DefaultsToTwoPercent(t *testing.T) {
	t.Setenv("SCALP_LIVE_RISK_FRACTION", "")
	if got := scalpLiveRiskFraction(); got != 0.02 {
		t.Errorf("default = %v, want 0.02", got)
	}
}

func TestScalpLiveRiskFraction_HonoursAValidOverride(t *testing.T) {
	t.Setenv("SCALP_LIVE_RISK_FRACTION", "0.0067")
	if got := scalpLiveRiskFraction(); got != 0.0067 {
		t.Errorf("override = %v, want 0.0067", got)
	}
}

// Garbage must fall back to the default rather than to zero.
//
// A zero risk fraction sizes every position at nothing, which presents as a
// desk that is armed, receiving signals and never opening anything — the
// silent-nothing-happens failure this codebase keeps producing.
func TestScalpLiveRiskFraction_RejectsNonsense(t *testing.T) {
	for _, bad := range []string{"nonsense", "0", "-0.5", "1", "1.5", "  "} {
		t.Setenv("SCALP_LIVE_RISK_FRACTION", bad)
		if got := scalpLiveRiskFraction(); got != 0.02 {
			t.Errorf("%q produced %v; it must fall back to the 2%% default", bad, got)
		}
	}
}
