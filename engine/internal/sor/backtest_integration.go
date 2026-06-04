package sor

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

// SimulatedVenueAdapter is a deterministic, order-book-aware venue adapter for
// backtesting. It walks the simulated book to produce realistic fills with
// slippage, models maker/taker fees, and can be configured to fail (to exercise
// venue-failover paths in backtests).
//
// It satisfies the VenueAdapter interface so the exact same SOR pipeline used
// in production is exercised in backtests (Phase 18N requirement).
type SimulatedVenueAdapter struct {
	mu       sync.Mutex
	id       VenueID
	registry *VenueRegistry

	// FailRate is the probability [0,1] that a child order is rejected,
	// used to simulate venue instability. Deterministic via the rng seed.
	FailRate float64
	// LatencyMs is the simulated round-trip latency reported on fills.
	LatencyMs float64
	// PartialFillRate is the probability a fill is partial (fills 50–90%).
	PartialFillRate float64

	rng *rand.Rand
}

// NewSimulatedVenueAdapter creates a deterministic simulated venue.
func NewSimulatedVenueAdapter(id VenueID, registry *VenueRegistry, seed int64) *SimulatedVenueAdapter {
	return &SimulatedVenueAdapter{
		id:        id,
		registry:  registry,
		LatencyMs: 5,
		rng:       rand.New(rand.NewSource(seed)),
	}
}

func (a *SimulatedVenueAdapter) VenueID() VenueID { return a.id }

// Execute simulates filling a child order against the venue's current book.
func (a *SimulatedVenueAdapter) Execute(ctx context.Context, child ChildOrder) (Fill, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	fill := Fill{
		ChildOrderID:        child.ChildOrderID,
		ParentClientOrderID: child.ParentClientOrderID,
		VenueID:             a.id,
		Symbol:              child.Symbol,
		Side:                child.Side,
		RequestedQty:        child.Quantity,
		LatencyMs:           a.LatencyMs,
		FilledAt:            time.Now().UTC(),
	}

	// Simulated rejection.
	if a.FailRate > 0 && a.rng.Float64() < a.FailRate {
		fill.Status = "REJECTED"
		fill.RejectReason = "simulated venue rejection"
		return fill, nil
	}

	md, ok := a.registry.MarketData(a.id, child.Symbol)
	if !ok {
		fill.Status = "REJECTED"
		fill.RejectReason = "no simulated market data"
		return fill, nil
	}

	// Determine fill quantity (possibly partial).
	fillQty := child.Quantity
	if a.PartialFillRate > 0 && a.rng.Float64() < a.PartialFillRate {
		fillQty = child.Quantity * (0.5 + 0.4*a.rng.Float64())
	}

	// Walk the book to compute a realistic VWAP fill price.
	vwap, filled := simulateBookWalk(md, child.Side, fillQty)
	if filled <= 0 {
		fill.Status = "REJECTED"
		fill.RejectReason = "no liquidity in simulated book"
		return fill, nil
	}

	// Fees: maker if post-only, else taker.
	fees := a.registry.EffectiveFees(a.id, child.Symbol)
	feeBps := fees.TakerBps
	if child.OrderType == "POST_ONLY" {
		feeBps = fees.MakerBps
	}
	notional := filled * vwap
	feesUSD := notional * feeBps / 10000

	fill.Status = "FILLED"
	if filled < child.Quantity-1e-12 {
		fill.Status = "PARTIAL"
	}
	fill.FilledQty = filled
	fill.AvgPrice = vwap
	fill.FeesUSD = feesUSD
	fill.SlippageBps = slippageVsRef(child.ReferencePrice, vwap, child.Side)
	return fill, nil
}

func (a *SimulatedVenueAdapter) Cancel(ctx context.Context, childOrderID, symbol string) error {
	return nil
}

// simulateBookWalk walks the order book consuming `qty` and returns the VWAP
// and total filled. Mirrors the LiquidityEngine walk but executes fully (no
// MaxWalkBps cap — the backtest fills whatever the book offers).
func simulateBookWalk(md VenueMarketData, side string, qty float64) (vwap, filled float64) {
	var levels []PriceLevel
	if isBuy(side) {
		levels = sortedAsks(md.Asks)
		if len(levels) == 0 && md.AskPrice > 0 {
			levels = []PriceLevel{{Price: md.AskPrice, Size: md.AskSize}}
		}
	} else {
		levels = sortedBids(md.Bids)
		if len(levels) == 0 && md.BidPrice > 0 {
			levels = []PriceLevel{{Price: md.BidPrice, Size: md.BidSize}}
		}
	}
	if len(levels) == 0 {
		return md.Mid(), 0
	}

	var notional float64
	remaining := qty
	for _, lvl := range levels {
		if remaining <= 0 {
			break
		}
		take := math.Min(lvl.Size, remaining)
		notional += take * lvl.Price
		filled += take
		remaining -= take
	}
	if filled > 0 {
		vwap = notional / filled
	}
	return vwap, filled
}

// BuildBacktestSOR constructs a fully-wired SOR with simulated venue adapters
// for use inside Backtest Engine V3. It returns the router plus the registry so
// the backtest can stream market data into it.
//
// This guarantees backtests reflect real SOR behaviour: venue selection,
// liquidity routing, order splitting, slippage, fee models, and venue failures.
func BuildBacktestSOR(store interface {
	// minimal ledger.Store subset is satisfied by the real store; pass nil to
	// disable event emission in lightweight backtests.
}, venues []VenueID, seed int64) *BacktestSOR {
	_ = store
	reg := NewVenueRegistry()
	for _, v := range venues {
		reg.RegisterDefault(v, string(v), FeeStructure{MakerBps: 1, TakerBps: 5})
	}

	liq := NewLiquidityEngine()
	fees := NewFeeOptimizer()
	slip := NewSlippageEngine(nil)
	health := NewExchangeHealthEngine(reg, nil)
	best := NewBestExecutionEngine(reg, liq, fees, slip, health)
	selector := NewVenueSelector(reg, best)
	splitter := NewOrderSplitter(liq, reg)
	scheduler := NewAlgoScheduler()
	planner := NewExecutionPlanner(splitter, scheduler, reg, nil)
	failover := NewFailoverController(reg, liq, health, nil)
	coord := NewExecutionCoordinator(reg, slip, health, failover, nil)

	adapters := make(map[VenueID]*SimulatedVenueAdapter, len(venues))
	for i, v := range venues {
		a := NewSimulatedVenueAdapter(v, reg, seed+int64(i))
		coord.RegisterAdapter(a)
		adapters[v] = a
	}

	router := NewSmartOrderRouter(reg, selector, planner, coord, nil)
	return &BacktestSOR{
		Router:   router,
		Registry: reg,
		Health:   health,
		Adapters: adapters,
	}
}

// BacktestSOR bundles the simulated SOR components for a backtest run.
type BacktestSOR struct {
	Router   *SmartOrderRouter
	Registry *VenueRegistry
	Health   *ExchangeHealthEngine
	Adapters map[VenueID]*SimulatedVenueAdapter
}

// FeedBook updates the simulated order book for a venue/symbol.
func (b *BacktestSOR) FeedBook(md VenueMarketData) {
	b.Registry.UpdateMarketData(md)
}

// SetVenueFailRate configures a simulated venue's rejection probability.
func (b *BacktestSOR) SetVenueFailRate(v VenueID, rate float64) {
	if a, ok := b.Adapters[v]; ok {
		a.mu.Lock()
		a.FailRate = rate
		a.mu.Unlock()
	}
}
