package main

import "testing"

// One desk's size multiplier must not touch any other desk.
//
// Every sizing knob before this one was process-wide: six numbered books, the
// gold book and the live paper mirror all run in a single binary and read the
// same environment. Scaling the Gold Desk through SCALP_PAPER_POSITION_USD
// would have silently resized the other seven, and their records are the only
// thing those desks exist to produce.
func TestLivePaperSizeMultiplier_IsPerAccount(t *testing.T) {
	t.Setenv("SCALP_PAPER_SIZE_MULT_GOLD", "20")

	if got := livePaperSizeMultiplier("GOLD"); got != 20 {
		t.Errorf("GOLD multiplier = %v, want 20", got)
	}
	for _, acct := range []string{"01", "02", "03", "04", "05", "06"} {
		if got := livePaperSizeMultiplier(acct); got != 1 {
			t.Errorf("account %s picked up GOLD's multiplier (%v); sizing must be per account", acct, got)
		}
	}
}

// An unset, zero or unparseable value must mean 1, never 0.
//
// A zero multiplier would size every position at zero notional and the desk
// would go silent — a typo in an env var turning a book off with no error
// anywhere, which is the failure mode this codebase keeps producing.
func TestLivePaperSizeMultiplier_DefaultsToOne(t *testing.T) {
	if got := livePaperSizeMultiplier("NOSUCHDESK"); got != 1 {
		t.Errorf("unset multiplier = %v, want 1", got)
	}
	for _, bad := range []string{"", "0", "-5", "abc"} {
		t.Setenv("SCALP_PAPER_SIZE_MULT_GOLD", bad)
		if got := livePaperSizeMultiplier("GOLD"); got != 1 {
			t.Errorf("multiplier %q gave %v, want 1 — a bad value must not silence the desk", bad, got)
		}
	}
}

// Case and whitespace must not decide whether a desk is scaled. The account
// name arrives from a constant, but the env key is written by hand.
func TestLivePaperSizeMultiplier_AccountNameIsNormalised(t *testing.T) {
	t.Setenv("SCALP_PAPER_SIZE_MULT_GOLD", "20")
	for _, name := range []string{"GOLD", "gold", " Gold "} {
		if got := livePaperSizeMultiplier(name); got != 20 {
			t.Errorf("account %q gave %v, want 20", name, got)
		}
	}
}
