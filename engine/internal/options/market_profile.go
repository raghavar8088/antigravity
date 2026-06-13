package options

import "time"

// ChainConfig tunes the synthetic option-chain model for a specific market.
type ChainConfig struct {
	WeeklyExpiryWeekday time.Weekday
	ExpiryHourUTC       int
	WeeklyCount         int
	StrikeIncrement     float64
	NumStrikes          int
	FallbackSpot        float64
	SmileFactor         float64
	SkewFactor          float64
	OIBase              float64
	OIDecay             float64
	SpreadBase          float64
	SpreadSlope         float64
	SpreadCap           float64
	VolumeNoiseFloor    float64
	VolumeNoiseRange    float64
}

// MarketProfile calibrates the options engine for a specific underlying market.
type MarketProfile struct {
	Name        string
	DefaultIV   float64
	MinIV       float64
	MaxIV       float64
	ChainConfig ChainConfig
}

var defaultOptionsMarketProfile = MarketProfile{
	Name:      "BTC option scalper",
	DefaultIV: 0.62,  // realised vol has compressed from 0.68 baseline
	MinIV:     0.20,
	MaxIV:     2.50,  // crypto tail-risk spikes can exceed 2.2×
	ChainConfig: ChainConfig{
		WeeklyExpiryWeekday: time.Friday,
		ExpiryHourUTC:       8,
		WeeklyCount:         4,
		StrikeIncrement:     500,
		NumStrikes:          22,  // wider chain coverage at higher spot
		FallbackSpot:        95000,
		SmileFactor:         2.5,
		SkewFactor:          0.28, // slightly more put skew at higher prices
		OIBase:              8000,
		OIDecay:             10.0,
		SpreadBase:          0.011,
		SpreadSlope:         0.11,
		SpreadCap:           0.15,
		VolumeNoiseFloor:    0.05,
		VolumeNoiseRange:    0.25,
	},
}

func (e *Engine) resolvedProfile() MarketProfile {
	if e.marketProfile.Name == "" {
		return defaultOptionsMarketProfile
	}
	return e.marketProfile
}

// PaperBTCFallbackSpot is a positive BTC/USD spot used when no live exchange
// price is available (cold start, WS down, or outbound HTTPS blocked). Keeps
// paper options engines ticking on minimal infrastructure.
func PaperBTCFallbackSpot() float64 {
	return defaultOptionsMarketProfile.ChainConfig.FallbackSpot
}
