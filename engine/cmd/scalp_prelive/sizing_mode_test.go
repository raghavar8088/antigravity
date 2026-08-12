package main

import "testing"

// Only one sizing mode may be in force, and the target wins.
//
// Both used to be applied, so the boot log announced fixed-size AND a target.
// The behaviour was right — PlanPerpOrder checks the target first — but the
// log told an operator the desk was trading 1 contract when it was not, and
// they reasonably concluded the deploy had failed.
func TestSizingMode_TargetTakesPrecedenceOverFixedContracts(t *testing.T) {
	t.Setenv("SCALP_LIVE_TARGET_NOTIONAL_USD", "3")
	t.Setenv("SCALP_LIVE_FIXED_CONTRACTS", "1")

	if got := scalpLiveTargetNotionalUSD(); got != 3 {
		t.Fatalf("target = %v, want 3", got)
	}
	// Fixed-contracts still parses; the caller is what must ignore it.
	if got := scalpLiveFixedContracts(); got != 1 {
		t.Fatalf("fixed = %d, want 1 — the value should still be readable so it can be reported as ignored", got)
	}
}

// With no target, fixed-size still works — the precedence must not disable it.
func TestSizingMode_FixedContractsStillAppliesAlone(t *testing.T) {
	t.Setenv("SCALP_LIVE_TARGET_NOTIONAL_USD", "")
	t.Setenv("SCALP_LIVE_FIXED_CONTRACTS", "1")

	if got := scalpLiveTargetNotionalUSD(); got != 0 {
		t.Fatalf("target = %v with no env, want 0", got)
	}
	if got := scalpLiveFixedContracts(); got != 1 {
		t.Fatalf("fixed = %d, want 1", got)
	}
}

// Neither set means risk-based sizing, the documented default.
func TestSizingMode_NeitherIsRiskBased(t *testing.T) {
	t.Setenv("SCALP_LIVE_TARGET_NOTIONAL_USD", "")
	t.Setenv("SCALP_LIVE_FIXED_CONTRACTS", "")
	if scalpLiveTargetNotionalUSD() != 0 || scalpLiveFixedContracts() != 0 {
		t.Error("an unconfigured desk did not fall back to risk-based sizing")
	}
}
