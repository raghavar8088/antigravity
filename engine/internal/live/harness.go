// Package live provides the LiveHarness — the bridge between paper simulation and real
// exchange execution. It verifies execution path parity, tracks fill quality, and detects
// orphan orders and ghost positions.
package live

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Mode distinguishes between paper simulation and real exchange execution.
type Mode int

const (
	ModePaper Mode = iota
	ModeLive
)

func (m Mode) String() string {
	if m == ModeLive {
		return "LIVE"
	}
	return "PAPER"
}

// OrderRequest is a normalized order passed to the harness from the strategy layer.
type OrderRequest struct {
	ClientOrderID string
	StrategyID    string
	Symbol        string
	Side          string  // BUY | SELL
	Quantity      float64
	OrderType     string  // MARKET | LIMIT
	LimitPrice    float64
	TimeInForce   string
	CreatedAt     time.Time
}

// FillReport is the result returned after an order is executed or simulated.
type FillReport struct {
	ClientOrderID string
	Symbol        string
	Side          string
	FilledQty     float64
	AvgPrice      float64
	FeesUSD       float64
	SlippageBps   float64
	LatencyMs     int64
	Mode          Mode
	FilledAt      time.Time
}

// Executor is implemented by both the paper executor and real exchange adapters.
type Executor interface {
	Submit(ctx context.Context, req OrderRequest) (FillReport, error)
	Cancel(ctx context.Context, clientOrderID string) error
}

// LiveHarness routes orders to the correct executor, records fill quality, and tracks
// orphan orders. The active mode switches are safe for concurrent use.
type LiveHarness struct {
	mu      sync.RWMutex
	mode    Mode
	paper   Executor
	live    Executor
	parity  *ParityChecker
	fills   *FillAnalyzer
	orphans *OrphanDetector
}

// NewLiveHarness constructs a harness in paper mode. liveExecutor may be nil during
// pilot stages that have not yet enabled live trading.
func NewLiveHarness(paperExecutor, liveExecutor Executor) *LiveHarness {
	return &LiveHarness{
		mode:    ModePaper,
		paper:   paperExecutor,
		live:    liveExecutor,
		parity:  NewParityChecker(),
		fills:   NewFillAnalyzer(),
		orphans: NewOrphanDetector(),
	}
}

// SetMode switches between paper and live execution modes.
// Returns an error if switching to LiveMode with no live executor configured.
func (h *LiveHarness) SetMode(m Mode) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if m == ModeLive && h.live == nil {
		return fmt.Errorf("live executor not configured; cannot switch to live mode")
	}
	h.mode = m
	return nil
}

// CurrentMode returns the active execution mode.
func (h *LiveHarness) CurrentMode() Mode {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.mode
}

// Submit routes the order to the active executor, records the fill, and tracks the
// order as open until Cancel or a complete fill confirms it closed.
func (h *LiveHarness) Submit(ctx context.Context, req OrderRequest) (FillReport, error) {
	h.mu.RLock()
	mode := h.mode
	h.mu.RUnlock()

	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now().UTC()
	}

	start := time.Now()
	var report FillReport
	var err error

	switch mode {
	case ModePaper:
		report, err = h.paper.Submit(ctx, req)
	case ModeLive:
		report, err = h.live.Submit(ctx, req)
	}

	if err == nil {
		report.LatencyMs = time.Since(start).Milliseconds()
		report.Mode = mode
		h.fills.Record(report)
		h.orphans.TrackOpen(req.ClientOrderID, req.StrategyID, req.Symbol)
		if report.FilledQty >= req.Quantity {
			h.orphans.TrackClose(req.ClientOrderID)
		}
	}
	return report, err
}

// Cancel passes a cancellation to the active executor and removes the order from orphan tracking.
func (h *LiveHarness) Cancel(ctx context.Context, clientOrderID string) error {
	h.mu.RLock()
	mode := h.mode
	h.mu.RUnlock()

	var err error
	switch mode {
	case ModePaper:
		err = h.paper.Cancel(ctx, clientOrderID)
	case ModeLive:
		err = h.live.Cancel(ctx, clientOrderID)
	}
	if err == nil {
		h.orphans.TrackClose(clientOrderID)
	}
	return err
}

// VerifyParity submits the same order to both paper and live executors and compares fills.
// Used to certify that paper and live execution paths are functionally equivalent.
// Both executors must be configured.
func (h *LiveHarness) VerifyParity(ctx context.Context, req OrderRequest) (*ParityResult, error) {
	if h.paper == nil || h.live == nil {
		return nil, fmt.Errorf("both paper and live executors required for parity verification")
	}
	return h.parity.Check(ctx, req, h.paper, h.live)
}

// Fills returns the fill quality analyzer.
func (h *LiveHarness) Fills() *FillAnalyzer { return h.fills }

// Orphans returns the orphan order detector.
func (h *LiveHarness) Orphans() *OrphanDetector { return h.orphans }
