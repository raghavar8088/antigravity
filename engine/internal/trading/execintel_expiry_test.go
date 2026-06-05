package trading

import (
	"testing"
	"time"

	"antigravity-engine/internal/execintel"
)

// TestOperationalGuardNeverExceedsHardExpiry proves that for every timeframe the
// orchestrator's operational stale-guard (signalMaxAge) is no looser than the
// Phase 22D hard expiry ceiling. Because the loop rejects when EITHER bound is
// breached, this invariant guarantees no signal older than its hard ceiling can
// ever reach execution.
func TestOperationalGuardNeverExceedsHardExpiry(t *testing.T) {
	timeframes := []string{"tick", "1m", "3m", "5m", "15m", "1h"}
	for _, tf := range timeframes {
		op := signalMaxAge(tf)
		hard := execintel.HardExpiry(tf)
		if op > hard {
			t.Errorf("timeframe %s: operational guard %v exceeds hard ceiling %v — a stale signal could slip through",
				tf, op, hard)
		}
	}
}

// TestExpiryEnforcementRejectsAgedSignal proves the combined gate flags an aged
// signal as expired for every timeframe at an age just beyond the hard ceiling.
func TestExpiryEnforcementRejectsAgedSignal(t *testing.T) {
	for _, tf := range []string{"1m", "3m", "5m", "15m"} {
		age := execintel.HardExpiry(tf) + time.Second
		if !execintel.IsExpired(tf, age) {
			t.Errorf("timeframe %s aged %v must be expired", tf, age)
		}
		// And the operational guard (stricter) must also reject it.
		if age <= signalMaxAge(tf) {
			t.Errorf("timeframe %s: aged signal %v should exceed operational guard %v", tf, age, signalMaxAge(tf))
		}
	}
}
