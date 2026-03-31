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

func TestAdjustConfidenceByExecutionWeight(t *testing.T) {
	down := adjustConfidenceByExecutionWeight(1.0, 0.5)
	if math.Abs(down-0.9) > signalTolerance {
		t.Fatalf("expected down-weighted confidence 0.9, got %.4f", down)
	}

	backfilled := adjustConfidenceByExecutionWeight(0, 0.5)
	if math.Abs(backfilled-0.9) > signalTolerance {
		t.Fatalf("expected backfilled confidence 0.9, got %.4f", backfilled)
	}

	upClamped := adjustConfidenceByExecutionWeight(1.4, 1.6)
	if math.Abs(upClamped-1.5) > signalTolerance {
		t.Fatalf("expected clamped confidence 1.5, got %.4f", upClamped)
	}

	floor := adjustConfidenceByExecutionWeight(-1.0, 0.5)
	if floor != 0 {
		t.Fatalf("expected negative confidence to clamp at 0, got %.4f", floor)
	}
}

func TestClassifyMarketRegime(t *testing.T) {
	o := &Orchestrator{}
	if got := o.classifyMarketRegime(); got != marketRegimeUnknown {
		t.Fatalf("expected unknown regime without data, got %s", got)
	}

	o.priceWindow = makeLinearSeries(100, 1.0, 120)
	o.volumeWindow = makeConstantSeries(1.0, 120)
	if got := o.classifyMarketRegime(); got != marketRegimeTrend {
		t.Fatalf("expected trend regime, got %s", got)
	}

	o.priceWindow = makeRangeSeries(100, 0.25, 120)
	o.volumeWindow = makeConstantSeries(1.0, 120)
	if got := o.classifyMarketRegime(); got != marketRegimeRange {
		t.Fatalf("expected range regime, got %s", got)
	}

	o.priceWindow = makeVolatileSeries(100, 120)
	o.volumeWindow = makeConstantSeries(1.0, 120)
	if got := o.classifyMarketRegime(); got != marketRegimeVolatile {
		t.Fatalf("expected volatile regime, got %s", got)
	}
}

func TestIsCategoryAlignedWithRegime(t *testing.T) {
	if !isCategoryAlignedWithRegime("Trend", marketRegimeTrend) {
		t.Fatal("expected Trend category allowed in TREND regime")
	}
	if isCategoryAlignedWithRegime("Mean Reversion", marketRegimeTrend) {
		t.Fatal("expected Mean Reversion blocked in TREND regime")
	}
	if !isCategoryAlignedWithRegime("Mean Reversion", marketRegimeRange) {
		t.Fatal("expected Mean Reversion allowed in RANGE regime")
	}
	if isCategoryAlignedWithRegime("Trend", marketRegimeRange) {
		t.Fatal("expected Trend blocked in RANGE regime")
	}
	if !isCategoryAlignedWithRegime("Any", marketRegimeUnknown) {
		t.Fatal("expected UNKNOWN regime to allow all categories")
	}
	if !isCategoryAlignedWithRegime("Any", marketRegimeMixed) {
		t.Fatal("expected MIXED regime to allow all categories")
	}
}

func makeLinearSeries(start, step float64, n int) []float64 {
	out := make([]float64, n)
	value := start
	for i := 0; i < n; i++ {
		out[i] = value
		value += step
	}
	return out
}

func makeRangeSeries(center, amp float64, n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = center + amp*math.Sin(float64(i)*0.6)
	}
	return out
}

func makeVolatileSeries(center float64, n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < n-25 {
			out[i] = center + 0.12*math.Sin(float64(i)*0.5)
			continue
		}
		// Late spike in realized volatility while keeping mean mostly flat.
		if i%2 == 0 {
			out[i] = center + 3.2
		} else {
			out[i] = center - 3.0
		}
	}
	return out
}

func makeConstantSeries(value float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = value
	}
	return out
}
