package dataquality

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validCandle() OHLCV {
	return OHLCV{
		Symbol: "BTCUSDT",
		Open:   50000,
		High:   50500,
		Low:    49800,
		Close:  50200,
		Volume: 100,
		Time:   time.Now().UTC(),
	}
}

func TestValidCandle_PassesAllChecks(t *testing.T) {
	v := NewValidator()
	result := v.ValidateCandle(validCandle(), "binance", 60)
	assert.True(t, result.Valid)
	assert.Equal(t, SeverityOK, result.Severity)
	assert.Equal(t, 100.0, result.QualityScore)
	assert.Empty(t, result.FailedChecks)
}

func TestNaNPrice_TriggersSevereAndSkip(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.Close = math.NaN()
	// One SEVERE check: score = 100 - 50 = 50 → ActionSkipCandle
	result := v.ValidateCandle(c, "binance", 60)
	assert.False(t, result.Valid)
	assert.Equal(t, SeveritySevere, result.Severity)
	assert.Equal(t, ActionSkipCandle, v.GetAction(result.QualityScore))
}

func TestNaNAndZeroVolume_TriggersHalt(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.Close = math.NaN()
	c.Volume = 0 // two SEVERE checks: 100 - 50 - 50 = 0 → ActionHalt
	result := v.ValidateCandle(c, "binance", 60)
	assert.False(t, result.Valid)
	assert.Equal(t, SeveritySevere, result.Severity)
	assert.Equal(t, ActionHalt, v.GetAction(result.QualityScore))
}

func TestInfPrice_TriggersSevere(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.High = math.Inf(1)
	result := v.ValidateCandle(c, "binance", 60)
	assert.False(t, result.Valid)
	assert.Equal(t, SeveritySevere, result.Severity)
}

func TestStaleCandle_TriggersModerate(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.Time = time.Now().Add(-5 * time.Minute) // stale for 1m candle
	result := v.ValidateCandle(c, "binance", 60)
	assert.False(t, result.Valid)
	assert.Equal(t, SeverityModerate, result.Severity)
}

func TestOHLCIntegrityHighLow_TriggersSevere(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.High = 49000 // high < low is impossible
	c.Low = 50000
	result := v.ValidateCandle(c, "binance", 60)
	assert.False(t, result.Valid)
	assert.Equal(t, SeveritySevere, result.Severity)
}

func TestZeroVolume_TriggersSevere(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.Volume = 0
	result := v.ValidateCandle(c, "binance", 60)
	assert.Contains(t, result.FailedChecks, "zero/negative volume")
	assert.Equal(t, SeveritySevere, result.Severity)
}

func TestCrossSourceDivergence_ReturnsFalseAbove05Pct(t *testing.T) {
	v := NewValidator()
	// 1% divergence — should fail
	assert.False(t, v.ValidateCrossSource(50000, "binance", 50501, "coinbase"))
	// 0.3% divergence — should pass
	assert.True(t, v.ValidateCrossSource(50000, "binance", 50150, "coinbase"))
}

func TestQualityScoreMath_BoundaryScores(t *testing.T) {
	v := NewValidator()

	// Single MODERATE failure: 100 - 25 = 75 → PROCEED_REDUCED
	c := validCandle()
	c.Time = time.Now().Add(-5 * time.Minute) // stale = MODERATE
	result := v.ValidateCandle(c, "binance", 60)
	assert.InDelta(t, 75.0, result.QualityScore, 0.1)
	assert.Equal(t, ActionProceedReduced, v.GetAction(result.QualityScore))
}

func TestGetAction_CorrectActionsForBoundaries(t *testing.T) {
	v := NewValidator()
	tests := []struct {
		score    float64
		expected Action
	}{
		{100, ActionProceed},
		{80, ActionProceed},
		{79, ActionProceedReduced},
		{60, ActionProceedReduced},
		{59, ActionSkipCandle},
		{30, ActionSkipCandle},
		{29, ActionHalt},
		{0, ActionHalt},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, v.GetAction(tt.score),
			"score=%.0f expected=%s", tt.score, tt.expected)
	}
}

func TestSourceHealth_MarkAndGet(t *testing.T) {
	v := NewValidator()
	v.MarkSourceDegraded("binance")
	v.MarkSourceUnavailable("coinbase")
	health := v.GetSourceHealth()
	assert.Equal(t, string(SourceDegraded), health["binance"])
	assert.Equal(t, string(SourceUnavailable), health["coinbase"])
}

func TestScoreNeverBelowZero(t *testing.T) {
	v := NewValidator()
	c := validCandle()
	c.Close = math.NaN()
	c.High = math.NaN()
	c.Low = math.NaN()
	c.Volume = 0
	result := v.ValidateCandle(c, "binance", 60)
	assert.GreaterOrEqual(t, result.QualityScore, 0.0)
}
