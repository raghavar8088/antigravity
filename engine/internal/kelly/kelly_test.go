package kelly

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseInputs() KellyInputs {
	return KellyInputs{
		WinRate:           0.55,
		AvgWinPct:         0.04,  // 4% win
		AvgLossPct:        0.02,  // 2% loss → b = 2.0
		PortfolioValue:    100000,
		MaxPositionPct:    0.10,
		RegimeMult:        1.0,
		DataQualityScore:  90,
		MinTradesRequired: 30,
		TradeCount:        50,
	}
}

// p=0.55, b=2.0 → f* = (0.55×2 - 0.45)/2 = (1.10-0.45)/2 = 0.65/2 = 0.325
// half-Kelly = 0.325 × 0.5 = 0.1625
// capped to 10% → FinalPositionPct = 0.10
func TestKelly_KnownFormula(t *testing.T) {
	result, err := Compute(baseInputs())
	require.NoError(t, err)
	assert.InDelta(t, 0.325, result.RawKelly, 0.001)
	assert.InDelta(t, 0.1625, result.HalfKelly, 0.001)
	// After regime 1.0 and quality 1.0 → 0.1625 → capped to 0.10
	assert.InDelta(t, 0.10, result.FinalPositionPct, 0.001)
	assert.InDelta(t, 10000.0, result.FinalPositionUSD, 1.0)
	assert.True(t, result.WasConstrained)
	assert.Equal(t, "hard_ceiling_10pct", result.ConstraintReason)
}

func TestKelly_HalfKelly_8125Pct_AfterConstraints(t *testing.T) {
	// With MaxPositionPct = 0.20 (but hard limit is still 10%)
	// p=0.55, b=2.0 → half-Kelly = 16.25%, still capped to 10%
	in := baseInputs()
	in.MaxPositionPct = 0.20
	result, err := Compute(in)
	require.NoError(t, err)
	assert.InDelta(t, 0.10, result.FinalPositionPct, 0.001)
}

func TestKelly_HardCeilingAlwaysEnforced(t *testing.T) {
	in := baseInputs()
	in.WinRate = 0.80
	in.AvgWinPct = 0.10
	in.AvgLossPct = 0.01
	in.MaxPositionPct = 0.50
	result, err := Compute(in)
	require.NoError(t, err)
	assert.LessOrEqual(t, result.FinalPositionPct, hardMaxPositionPct())
}

func TestKelly_NegativeKelly_ReturnsZero(t *testing.T) {
	in := baseInputs()
	in.WinRate = 0.30 // losing strategy
	in.AvgWinPct = 0.01
	in.AvgLossPct = 0.05
	result, err := Compute(in)
	require.NoError(t, err)
	assert.Equal(t, 0.0, result.FinalPositionPct)
	assert.Equal(t, 0.0, result.FinalPositionUSD)
	assert.Equal(t, "negative_kelly", result.ConstraintReason)
}

func TestKelly_InsufficientHistory_Returns5Pct(t *testing.T) {
	in := baseInputs()
	in.TradeCount = 10
	in.MinTradesRequired = 30
	result, err := Compute(in)
	require.NoError(t, err)
	assert.InDelta(t, 0.05, result.FinalPositionPct, 0.001)
	assert.Equal(t, "insufficient_history", result.ConstraintReason)
}

func TestKelly_DataQuality50_AppliesHalfMultiplier(t *testing.T) {
	in := baseInputs()
	in.WinRate = 0.60
	in.AvgWinPct = 0.03
	in.AvgLossPct = 0.03 // b=1.0 → f*= (0.6×1 - 0.4)/1 = 0.20, half=0.10
	in.DataQualityScore = 50
	in.MaxPositionPct = 0.10
	result, err := Compute(in)
	require.NoError(t, err)
	// half-kelly=0.10, ×0.5 quality mult = 0.05 → floor applies, final ≤ 0.10
	assert.LessOrEqual(t, result.FinalPositionPct, 0.10)
}

func TestKelly_RegimeMult05_HalvesPosition(t *testing.T) {
	in := baseInputs()
	in.RegimeMult = 0.5
	result, err := Compute(in)
	require.NoError(t, err)
	// half-kelly=0.1625, × 0.5 = 0.08125 → below 10% cap
	assert.InDelta(t, 0.08125, result.FinalPositionPct, 0.001)
}

func TestKelly_InvalidWinRate_ReturnsError(t *testing.T) {
	in := baseInputs()
	in.WinRate = 1.5
	_, err := Compute(in)
	assert.Error(t, err)
}

func TestKelly_ResultIncludes_FinalPositionUSD(t *testing.T) {
	in := baseInputs()
	in.RegimeMult = 0.5 // 8.125% of 100k = $8125
	result, err := Compute(in)
	require.NoError(t, err)
	assert.InDelta(t, 8125.0, result.FinalPositionUSD, 1.0)
}
