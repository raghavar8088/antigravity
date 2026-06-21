package trading

import (
	"math"
	"testing"

	tconfig "antigravity-engine/internal/config"
	"antigravity-engine/internal/strategy"
)

// TestStrictnessDialHotReloadsWiredThresholds proves the three thresholds that
// were previously dead (SCALER_RR_MINIMUM, MIN_EXECUTION_WEIGHT_TO_TRADE,
// MIN_BRIDGE_CONFIDENCE) now propagate from the central registry into the live
// package vars the gates actually read.
func TestStrictnessDialHotReloadsWiredThresholds(t *testing.T) {
	reg := tconfig.Default()
	origRR, origEW, origBridge := scalerRRMinimum, minExecutionWeightToTrade, minBridgeApprovalConfidence
	t.Cleanup(func() {
		_, _ = reg.Set("SCALER_RR_MINIMUM", origRR, "test_restore")
		_, _ = reg.Set("MIN_EXECUTION_WEIGHT_TO_TRADE", origEW, "test_restore")
		_, _ = reg.Set("MIN_BRIDGE_CONFIDENCE", origBridge, "test_restore")
		RefreshThresholdsFromRegistry()
	})

	if _, err := reg.Set("SCALER_RR_MINIMUM", 1.5, "test"); err != nil {
		t.Fatalf("set SCALER_RR_MINIMUM: %v", err)
	}
	if _, err := reg.Set("MIN_EXECUTION_WEIGHT_TO_TRADE", 0.40, "test"); err != nil {
		t.Fatalf("set MIN_EXECUTION_WEIGHT_TO_TRADE: %v", err)
	}
	if _, err := reg.Set("MIN_BRIDGE_CONFIDENCE", 0.60, "test"); err != nil {
		t.Fatalf("set MIN_BRIDGE_CONFIDENCE: %v", err)
	}
	RefreshThresholdsFromRegistry()

	if math.Abs(scalerRRMinimum-1.5) > signalTolerance {
		t.Fatalf("scalerRRMinimum=%.4f, want 1.5 (SCALER_RR_MINIMUM not hot-reloaded)", scalerRRMinimum)
	}
	if math.Abs(minExecutionWeightToTrade-0.40) > signalTolerance {
		t.Fatalf("minExecutionWeightToTrade=%.4f, want 0.40 (MIN_EXECUTION_WEIGHT_TO_TRADE not hot-reloaded)", minExecutionWeightToTrade)
	}
	if math.Abs(minBridgeApprovalConfidence-0.60) > signalTolerance {
		t.Fatalf("minBridgeApprovalConfidence=%.4f, want 0.60 (MIN_BRIDGE_CONFIDENCE not hot-reloaded)", minBridgeApprovalConfidence)
	}
}

// TestSanitizeScalerSignalUsesConfiguredRRFloor proves the scaler R:R gate now
// honors SCALER_RR_MINIMUM instead of a hardcoded 2.0: a 1.7:1 signal is
// rejected at the 2.0 default but accepted once the floor is loosened to 1.5.
func TestSanitizeScalerSignalUsesConfiguredRRFloor(t *testing.T) {
	reg := tconfig.Default()
	orig := scalerRRMinimum
	t.Cleanup(func() {
		_, _ = reg.Set("SCALER_RR_MINIMUM", orig, "test_restore")
		RefreshThresholdsFromRegistry()
	})

	// SL=0.50%, TP=0.85% → R:R = 1.7. High confidence + ample size so only the
	// R:R gate is in question.
	mk := func() strategy.Signal {
		return strategy.Signal{Confidence: 0.99, StopLossPct: 0.50, TakeProfitPct: 0.85, TargetSize: 0.5}
	}

	if _, err := reg.Set("SCALER_RR_MINIMUM", 2.0, "test"); err != nil {
		t.Fatalf("set floor 2.0: %v", err)
	}
	RefreshThresholdsFromRegistry()
	sigStrict := mk()
	if err := sanitizeScalerSignal(&sigStrict); err == nil {
		t.Fatalf("expected 1.7:1 signal to be rejected at R:R floor 2.0, but it passed")
	}

	if _, err := reg.Set("SCALER_RR_MINIMUM", 1.5, "test"); err != nil {
		t.Fatalf("set floor 1.5: %v", err)
	}
	RefreshThresholdsFromRegistry()
	sigLoose := mk()
	if err := sanitizeScalerSignal(&sigLoose); err != nil {
		t.Fatalf("expected 1.7:1 signal to pass at R:R floor 1.5, got: %v", err)
	}
}
