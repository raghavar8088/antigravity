package execution

// PaperExchangeAdapter simulates exchange responses: execution price, fees,
// and partial fills.
//
// This adapter does NOT own order state, position state, or account balance.
// All state ownership belongs exclusively to OMS v3 and the ledger.
// The adapter is the boundary between OMS v3 and the simulated exchange.
//
// Role in the target architecture:
//   OMS v3 → PaperExchangeAdapter.SimulateFill() → fill result
//   OMS v3 appends EventOrderFilled to ledger (the adapter does not touch the ledger)

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/strategy"
)

// SimulateFillRequest carries everything needed to compute a simulated fill.
type SimulateFillRequest struct {
	Symbol      string
	Side        strategy.Action
	Quantity    float64
	OrderMode   OrderMode
	RequestedAt time.Time
}

// SimulateFillResult is the simulated exchange response.
// It contains everything OMS v3 needs to append EventOrderFilled.
type SimulateFillResult struct {
	ExchangeOrderID string
	FillPrice       float64
	FillQuantity    float64
	FeeUSD          float64
	OrderMode       OrderMode
	SlippageBps     float64
}

// PaperExchangeAdapter simulates exchange execution without owning any state.
// Thread-safe: UpdatePrice and SimulateFill may be called concurrently.
type PaperExchangeAdapter struct {
	mu          sync.RWMutex
	lastPrice   float64
	fillCounter int64 // atomic counter for exchange order ID uniqueness
}

// NewPaperExchangeAdapter creates a PaperExchangeAdapter with no initial price.
func NewPaperExchangeAdapter() *PaperExchangeAdapter {
	return &PaperExchangeAdapter{}
}

// UpdatePrice feeds the latest market price to the adapter.
// Called by the market data ingestion loop on every tick.
func (a *PaperExchangeAdapter) UpdatePrice(price float64) {
	if price <= 0 {
		return
	}
	a.mu.Lock()
	a.lastPrice = price
	a.mu.Unlock()
}

// LastPrice returns the most recent price fed to the adapter.
func (a *PaperExchangeAdapter) LastPrice() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastPrice
}

// SimulateFill computes a fill for the given request.
// Returns an error when no price is available or the request is invalid.
// Does not mutate any state outside this struct.
func (a *PaperExchangeAdapter) SimulateFill(req SimulateFillRequest) (SimulateFillResult, error) {
	a.mu.RLock()
	lastPrice := a.lastPrice
	a.mu.RUnlock()

	if lastPrice <= 0 {
		return SimulateFillResult{}, fmt.Errorf("paper adapter: no market price available")
	}
	if req.Quantity <= 0 {
		return SimulateFillResult{}, fmt.Errorf("paper adapter: quantity must be > 0, got %.8f", req.Quantity)
	}

	fillPrice := paperExecPrice(lastPrice, req.OrderMode, req.Side)
	slippageBps := math.Abs(fillPrice-lastPrice) / lastPrice * 10_000
	feePct := takerFeePct(req.OrderMode)
	notional := req.Quantity * fillPrice
	feeUSD := notional * feePct

	seq := atomic.AddInt64(&a.fillCounter, 1)

	requestedAt := req.RequestedAt
	if requestedAt.IsZero() {
		requestedAt = time.Now().UTC()
	}
	exchangeOrderID := fmt.Sprintf("PAPER-%s-%d-%d",
		req.Symbol, requestedAt.UnixNano()/1e6, seq)

	return SimulateFillResult{
		ExchangeOrderID: exchangeOrderID,
		FillPrice:       fillPrice,
		FillQuantity:    req.Quantity,
		FeeUSD:          feeUSD,
		OrderMode:       req.OrderMode,
		SlippageBps:     slippageBps,
	}, nil
}

// SimulatePartialFill simulates a partial fill at the given ratio of the
// requested quantity. partialRatio must be in (0, 1).
func (a *PaperExchangeAdapter) SimulatePartialFill(req SimulateFillRequest, partialRatio float64) (SimulateFillResult, error) {
	if partialRatio <= 0 || partialRatio >= 1 {
		return SimulateFillResult{}, fmt.Errorf("paper adapter: partial ratio %.4f must be in (0, 1)", partialRatio)
	}
	req.Quantity = req.Quantity * partialRatio
	return a.SimulateFill(req)
}

// paperExecPrice computes the simulated execution price from a reference price.
// The fill is adverse to the taker (buys pay more, sells receive less) to model
// market impact realistically without a full order-book simulation.
func paperExecPrice(refPrice float64, mode OrderMode, action strategy.Action) float64 {
	switch mode {
	case OrderModePostOnly:
		if action == strategy.ActionBuy {
			return refPrice * 0.99995 // maker — slightly better
		}
		return refPrice * 1.00005
	case OrderModeIOC:
		if action == strategy.ActionBuy {
			return refPrice * 1.00012 // aggressive taker
		}
		return refPrice * 0.99988
	default: // MARKET
		if action == strategy.ActionBuy {
			return refPrice * 1.0001
		}
		return refPrice * 0.9999
	}
}
