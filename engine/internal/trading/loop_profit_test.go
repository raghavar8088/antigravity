package trading

import (
	"math"
	"testing"

	"antigravity-engine/internal/strategy"
)

const signalTolerance = 1e-9

func TestSanitizeSignalForProfitRejectsLowConfidence(t *testing.T) {
	_, allowed := sanitizeSignalForProfit(strategy.Signal{
		Confidence:    0.70,
		StopLossPct:   0.30,
		TakeProfitPct: 0.80,
	})

	if allowed {
		t.Fatal("expected low-confidence signal to be rejected")
	}
}

func TestSanitizeSignalForProfitAppliesDefaults(t *testing.T) {
	sanitized, allowed := sanitizeSignalForProfit(strategy.Signal{
		Confidence: 1.0,
	})

	if !allowed {
		t.Fatal("expected signal to be allowed")
	}
	if math.Abs(sanitized.StopLossPct-defaultSignalStopLossPct) > signalTolerance {
		t.Fatalf("expected default stop loss %.2f, got %.4f", defaultSignalStopLossPct, sanitized.StopLossPct)
	}
	if math.Abs(sanitized.TakeProfitPct-minSignalTakeProfitPct) > signalTolerance {
		t.Fatalf("expected default take profit %.2f, got %.4f", minSignalTakeProfitPct, sanitized.TakeProfitPct)
	}
}

func TestSanitizeSignalForProfitEnforcesRiskRewardAndStopCap(t *testing.T) {
	sanitized, allowed := sanitizeSignalForProfit(strategy.Signal{
		Confidence:    1.0,
		StopLossPct:   2.0,
		TakeProfitPct: 0.5,
	})

	if !allowed {
		t.Fatal("expected signal to be allowed")
	}
	if math.Abs(sanitized.StopLossPct-maxSignalStopLossPct) > signalTolerance {
		t.Fatalf("expected clamped stop loss %.2f, got %.4f", maxSignalStopLossPct, sanitized.StopLossPct)
	}

	expectedTakeProfit := maxSignalStopLossPct * minRewardToRiskRatio
	if math.Abs(sanitized.TakeProfitPct-expectedTakeProfit) > signalTolerance {
		t.Fatalf("expected take profit %.4f, got %.4f", expectedTakeProfit, sanitized.TakeProfitPct)
	}
}

func TestSanitizeSignalForProfitBackfillsMissingConfidence(t *testing.T) {
	sanitized, allowed := sanitizeSignalForProfit(strategy.Signal{
		Confidence:    0,
		StopLossPct:   0.4,
		TakeProfitPct: 0.6,
	})

	if !allowed {
		t.Fatal("expected missing-confidence signal to be accepted")
	}
	if math.Abs(sanitized.Confidence-1.0) > signalTolerance {
		t.Fatalf("expected confidence to backfill to 1.0, got %.4f", sanitized.Confidence)
	}
}
