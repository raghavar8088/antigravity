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
	DefaultIV: 0.80,
	MinIV:     0.30,
	MaxIV:     3.00,
	ChainConfig: ChainConfig{
		WeeklyExpiryWeekday: time.Friday,
		ExpiryHourUTC:       8,
		WeeklyCount:         4,
		StrikeIncrement:     500,
		NumStrikes:          20,
		FallbackSpot:        67000,
		SmileFactor:         2.5,
		SkewFactor:          0.25,
		OIBase:              8000,
		OIDecay:             10.0,
		SpreadBase:          0.012,
		SpreadSlope:         0.12,
		SpreadCap:           0.15,
		VolumeNoiseFloor:    0.05,
		VolumeNoiseRange:    0.25,
	},
}

var niftyOptionsMarketProfile = MarketProfile{
	Name:      "NIFTY 50 option scalper",
	DefaultIV: 0.18,
	MinIV:     0.08,
	MaxIV:     0.75,
	ChainConfig: ChainConfig{
		WeeklyExpiryWeekday: time.Thursday,
		ExpiryHourUTC:       10,
		WeeklyCount:         4,
		StrikeIncrement:     50,
		NumStrikes:          18,
		FallbackSpot:        22500,
		SmileFactor:         1.15,
		SkewFactor:          0.18,
		OIBase:              150000,
		OIDecay:             7.0,
		SpreadBase:          0.008,
		SpreadSlope:         0.05,
		SpreadCap:           0.08,
		VolumeNoiseFloor:    0.06,
		VolumeNoiseRange:    0.22,
	},
}

func (e *Engine) resolvedProfile() MarketProfile {
	if e.marketProfile.Name == "" {
		return defaultOptionsMarketProfile
	}
	return e.marketProfile
}
