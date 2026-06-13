// Package aiscoring provides non-blocking, cache-backed AI signal scoring.
//
// Signals are submitted asynchronously via SubmitForScoring. The execution
// path calls GetCachedScore to retrieve the most recent pre-computed score
// without ever blocking. A FallbackScorer provides instant rule-based scores
// when no cached AI score is available.
package aiscoring

import "time"

// SignalScore is the output of one scoring cycle.
type SignalScore struct {
	Confidence    float64   // 0–100, calibrated
	RawConfidence float64   // 0–100, before calibration
	Direction     string    // "BUY" | "SELL" | "HOLD"
	Reasoning     string    // AI explanation (empty when fallback)
	Source        string    // "ai_openai" | "ai_gemini" | "ai_groq" | "rule_based"
	ScoredAt      time.Time
	IsStale       bool // true if score is > 60 seconds old
	IsFallback    bool // true if rule-based fallback was used
}

// ScoringRequest is the input to the async scorer.
type ScoringRequest struct {
	Signal    interface{}   // strategy signal payload
	Context   MarketContext // market snapshot at signal time
	RequestID string        // UUID for tracing
	Priority  int           // 0 = normal, 1 = high
}

// MarketContext carries the market indicators needed for scoring.
// FundingRate and OITrend are populated after Phase 3.
type MarketContext struct {
	Price       float64
	RSI14_1h    float64
	MACD_hist   float64
	EMA9        float64
	EMA21       float64
	ATR14       float64
	BBPosition  float64 // 0–1 position within Bollinger Bands
	Volume      float64
	VolumeAvg   float64
	Regime      string
	FundingRate float64 // populated after Phase 3
	OITrend     string  // populated after Phase 3
}

// AIClient is the interface that wraps an upstream AI provider.
// Implementations live in engine/internal/ai/.
type AIClient interface {
	ScoreSignal(ctx interface{}, signal interface{}, mctx MarketContext) (*SignalScore, error)
}
