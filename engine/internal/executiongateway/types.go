// Package executiongateway is the single authoritative HTTP entry for human-initiated
// execution requests. All broker interaction must originate here after institutional
// risk, OMS, and kill-switch gates — never from Next.js routes or frontend hooks.
package executiongateway

import (
	"context"
	"time"
)

// Request is a human- or service-initiated execution intent (NOT a direct broker order).
type Request struct {
	RequestID    string  `json:"requestId"`
	Venue        string  `json:"venue"` // "paper", "delta", "angelone"
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"` // BUY, SELL, buy, sell, LONG, SHORT
	Size         float64 `json:"size"`
	StrategyName string  `json:"strategyName"`
	Reason       string  `json:"reason"`
	Confidence   float64 `json:"confidence"`
	ProductID    int     `json:"productId,omitempty"`
	Contracts    int     `json:"contracts,omitempty"`
}

// Response is returned after the institutional pipeline processes the request.
type Response struct {
	OK            bool      `json:"ok"`
	Status        string    `json:"status"` // ACCEPTED, REJECTED, BLOCKED
	RequestID     string    `json:"requestId,omitempty"`
	ClientOrderID string    `json:"clientOrderId,omitempty"`
	Message       string    `json:"message,omitempty"`
	ProcessedAt   time.Time `json:"processedAt"`
}

// Executor is implemented by the trading orchestrator.
type Executor interface {
	ProcessExecutionRequest(ctx context.Context, req Request) (Response, error)
}
