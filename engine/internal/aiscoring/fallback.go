package aiscoring

import "time"

// FallbackScorer provides instant (< 5ms) rule-based signal scores when no
// cached AI score is available. It requires zero external API calls.
type FallbackScorer struct{}

// NewFallbackScorer constructs a ready-to-use FallbackScorer.
func NewFallbackScorer() *FallbackScorer {
	return &FallbackScorer{}
}

// Score computes a confidence score from market indicators alone.
func (f *FallbackScorer) Score(ctx MarketContext) SignalScore {
	confidence := 50.0
	direction := "HOLD"

	// RSI signal
	switch {
	case ctx.RSI14_1h < 30:
		confidence += 15
		direction = "BUY"
	case ctx.RSI14_1h > 70:
		confidence -= 15
		direction = "SELL"
	case ctx.RSI14_1h >= 30 && ctx.RSI14_1h <= 45:
		confidence += 5
		if direction == "HOLD" {
			direction = "BUY"
		}
	case ctx.RSI14_1h >= 55 && ctx.RSI14_1h <= 70:
		confidence -= 5
		if direction == "HOLD" {
			direction = "SELL"
		}
	}

	// MACD histogram signal
	if ctx.MACD_hist > 0 {
		confidence += 10
		if direction == "HOLD" {
			direction = "BUY"
		}
	} else if ctx.MACD_hist < 0 {
		confidence -= 10
		if direction == "HOLD" {
			direction = "SELL"
		}
	}

	// EMA cross signal
	if ctx.EMA9 > ctx.EMA21 {
		confidence += 8
		if direction == "HOLD" {
			direction = "BUY"
		}
	} else if ctx.EMA9 < ctx.EMA21 {
		confidence -= 8
		if direction == "HOLD" {
			direction = "SELL"
		}
	}

	// Volume confirmation
	if ctx.VolumeAvg > 0 {
		ratio := ctx.Volume / ctx.VolumeAvg
		if ratio > 1.2 {
			confidence += 5
		} else if ratio < 0.7 {
			confidence -= 8
		}
	}

	// Bollinger Band position
	if ctx.BBPosition < 0.1 {
		confidence += 12
		direction = "BUY"
	} else if ctx.BBPosition > 0.9 {
		confidence -= 12
		direction = "SELL"
	}

	// Clamp and final direction
	if confidence > 100 {
		confidence = 100
	}
	if confidence < 0 {
		confidence = 0
	}
	if confidence < 45 {
		direction = "HOLD"
	}

	return SignalScore{
		Confidence:    confidence,
		RawConfidence: confidence,
		Direction:     direction,
		Source:        "rule_based",
		ScoredAt:      time.Now().UTC(),
		IsFallback:    true,
	}
}
