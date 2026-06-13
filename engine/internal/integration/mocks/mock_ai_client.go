// Package mocks provides test doubles for integration tests.
// All mocks are safe for use in parallel tests.
package mocks

import (
	"fmt"
	"time"

	"antigravity-engine/internal/aiscoring"
)

// MockAIClient returns configurable SignalScore values for testing.
// Implements the aiscoring.AIClient interface.
type MockAIClient struct {
	Confidence  float64
	Direction   string
	Source      string
	ReturnError bool
}

// ScoreSignal satisfies aiscoring.AIClient.
func (m *MockAIClient) ScoreSignal(ctx interface{}, signal interface{}, mctx aiscoring.MarketContext) (*aiscoring.SignalScore, error) {
	if m.ReturnError {
		return nil, fmt.Errorf("mock AI error")
	}
	src := m.Source
	if src == "" {
		src = "mock_ai"
	}
	dir := m.Direction
	if dir == "" {
		dir = "HOLD"
	}
	return &aiscoring.SignalScore{
		Confidence:    m.Confidence,
		RawConfidence: m.Confidence,
		Direction:     dir,
		Source:        src,
		ScoredAt:      time.Now().UTC(),
		IsFallback:    false,
	}, nil
}
