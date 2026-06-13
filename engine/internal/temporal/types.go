// Package temporal computes time-based trading patterns from historical trade data
// and provides size modifiers based on hour-of-day, day-of-week, and CME windows.
package temporal

import "time"

// TemporalBias classifies whether the current time is historically favorable.
type TemporalBias string

const (
	TemporalFavorable   TemporalBias = "FAVORABLE"
	TemporalNeutral     TemporalBias = "NEUTRAL"
	TemporalUnfavorable TemporalBias = "UNFAVORABLE"
)

// TemporalPattern stores historical performance for a specific hour or day bucket.
type TemporalPattern struct {
	HourUTC     int
	DayOfWeek   int // 0=Sunday
	WinRate     float64
	AvgPnL      float64
	TradeCount  int
	LastUpdated time.Time
}

// TemporalAnalysis is the result of analysing the current moment.
type TemporalAnalysis struct {
	CurrentHourPattern TemporalPattern
	CurrentDayPattern  TemporalPattern
	Bias               TemporalBias
	SizeModifier       float64 // 0.7, 1.0, or 1.1
	CMEWindowActive    bool
	CMEModifier        float64 // 0.7 or 0.85 if CME window, else 1.0
	EffectiveModifier  float64 // SizeModifier × CMEModifier
	Description        string
}
