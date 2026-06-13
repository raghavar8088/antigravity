// Package calibration analyses historical AI confidence scores vs actual trade
// outcomes to detect overconfidence and apply a correction factor.
package calibration

import "time"

const (
	// MinTradesForCalibration is the minimum closed trades required to compute calibration.
	MinTradesForCalibration = 60
	// BucketSize is the confidence range for each bucket (10 buckets: 0-10, 10-20, ...).
	BucketSize = 10.0
)

// ConfidenceBucket groups trades with similar stated confidence and measures accuracy.
type ConfidenceBucket struct {
	BucketMin     float64 // e.g., 70
	BucketMax     float64 // e.g., 80
	TradeCount    int
	WinCount      int
	ActualWinRate float64 // WinCount / TradeCount
	StatedMid     float64 // midpoint, e.g., 75
}

// CalibrationResult is the output of a calibration run.
type CalibrationResult struct {
	Buckets           []ConfidenceBucket
	CalibrationFactor float64   // actual_avg / stated_avg (< 1 if overconfident)
	IsOverconfident   bool      // true if |stated_avg - actual_avg| > 10%
	TradesAnalysed    int
	ComputedAt        time.Time
	NextRecalibAt     time.Time
}
