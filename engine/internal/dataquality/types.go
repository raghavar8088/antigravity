// Package dataquality provides candle and market data validation for the
// trading engine. Every incoming OHLCV bar passes through ValidateCandle
// before reaching strategy evaluation. Severe data quality issues trigger
// position sizing reductions or trading halts via the kill switch.
package dataquality

import "time"

// Severity classifies how bad a validation failure is.
type Severity string

const (
	SeverityOK       Severity = "OK"
	SeverityMinor    Severity = "MINOR"    // use cached, log warning
	SeverityModerate Severity = "MODERATE" // skip candle, use previous
	SeveritySevere   Severity = "SEVERE"   // mark source degraded
	SeverityCritical Severity = "CRITICAL" // halt trading
)

// Action is the recommended response to a validation result.
type Action string

const (
	ActionProceed        Action = "PROCEED"
	ActionProceedReduced Action = "PROCEED_REDUCED_SIZE" // 50% size
	ActionSkipCandle     Action = "SKIP_CANDLE"
	ActionHalt           Action = "HALT"
)

// OHLCV is the candle shape accepted by the validator. It is intentionally
// independent of marketdata.Candle to avoid package-level circular imports.
type OHLCV struct {
	Symbol string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
	Time   time.Time
}

// ValidationResult is the output of ValidateCandle.
type ValidationResult struct {
	Valid        bool
	Severity     Severity
	FailedChecks []string
	QualityScore float64 // 0–100
	Source       string
	Timestamp    time.Time
}

// SourceStatus enumerates the health state of a data source.
type SourceStatus string

const (
	SourceHealthy     SourceStatus = "HEALTHY"
	SourceDegraded    SourceStatus = "DEGRADED"
	SourceUnavailable SourceStatus = "UNAVAILABLE"
)
