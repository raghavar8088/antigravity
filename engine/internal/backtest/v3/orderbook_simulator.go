package v3

import (
	"math"
	"sort"
	"time"
)

// PriceLevel represents a single price/size entry in the order book.
type PriceLevel struct {
	Price float64
	Size  float64
}

// OrderBook is an L2 simulated order book reconstructed from tick data.
type OrderBook struct {
	Bids      []PriceLevel // descending price order
	Asks      []PriceLevel // ascending price order
	Mid       float64
	Timestamp time.Time

	// Synthetic generation parameters
	SpreadBps    float64
	DepthLevels  int
	BaseDepthUSD float64
}

// NewOrderBook constructs a synthetic L2 order book from a mid price and market context.
// The depth ladder decays exponentially away from the mid, matching typical BTC/USD microstructure.
func NewOrderBook(mid, volatilityPct, liquidityScore float64, ts time.Time) *OrderBook {
	spreadBps := syntheticSpreadBps(volatilityPct, liquidityScore)
	halfSpread := mid * spreadBps / 20_000 // half spread in price
	depthLevels := 10
	baseDepthUSD := mid * 200 * liquidityScore // base USD depth at best level

	bids := make([]PriceLevel, depthLevels)
	asks := make([]PriceLevel, depthLevels)

	for i := 0; i < depthLevels; i++ {
		decay := math.Exp(-float64(i) * 0.3)
		tickSize := mid * 0.0001 // 1bps per level
		bidPrice := (mid - halfSpread) - float64(i)*tickSize
		askPrice := (mid + halfSpread) + float64(i)*tickSize
		depth := baseDepthUSD * decay
		bids[i] = PriceLevel{Price: bidPrice, Size: depth / bidPrice}
		asks[i] = PriceLevel{Price: askPrice, Size: depth / askPrice}
	}

	return &OrderBook{
		Bids:         bids,
		Asks:         asks,
		Mid:          mid,
		Timestamp:    ts,
		SpreadBps:    spreadBps,
		DepthLevels:  depthLevels,
		BaseDepthUSD: baseDepthUSD,
	}
}

// ApplyStress degrades the order book under a stress multiplier (spreadMult > 1, depthMult < 1).
func (ob *OrderBook) ApplyStress(spreadMult, depthMult float64) {
	if spreadMult <= 0 {
		spreadMult = 1
	}
	if depthMult <= 0 || depthMult > 1 {
		depthMult = 0.1
	}
	mid := ob.Mid
	halfSpread := mid * ob.SpreadBps * spreadMult / 20_000
	for i := range ob.Bids {
		decay := math.Exp(-float64(i) * 0.3)
		tickSize := mid * 0.0001
		bidPrice := (mid - halfSpread) - float64(i)*tickSize
		askPrice := (mid + halfSpread) + float64(i)*tickSize
		depth := ob.BaseDepthUSD * decay * depthMult
		ob.Bids[i] = PriceLevel{Price: bidPrice, Size: depth / bidPrice}
		ob.Asks[i] = PriceLevel{Price: askPrice, Size: depth / askPrice}
	}
}

// BestBid returns the top bid price and size.
func (ob *OrderBook) BestBid() PriceLevel {
	if len(ob.Bids) == 0 {
		return PriceLevel{}
	}
	return ob.Bids[0]
}

// BestAsk returns the top ask price and size.
func (ob *OrderBook) BestAsk() PriceLevel {
	if len(ob.Asks) == 0 {
		return PriceLevel{}
	}
	return ob.Asks[0]
}

// SpreadPriceAbs returns the bid/ask spread in price units.
func (ob *OrderBook) SpreadPriceAbs() float64 {
	if len(ob.Bids) == 0 || len(ob.Asks) == 0 {
		return 0
	}
	return ob.Asks[0].Price - ob.Bids[0].Price
}

// TotalBidDepthUSD returns total USD value available on the bid side up to nLevels.
func (ob *OrderBook) TotalBidDepthUSD(nLevels int) float64 {
	if nLevels > len(ob.Bids) {
		nLevels = len(ob.Bids)
	}
	total := 0.0
	for i := 0; i < nLevels; i++ {
		total += ob.Bids[i].Price * ob.Bids[i].Size
	}
	return total
}

// TotalAskDepthUSD returns total USD value available on the ask side up to nLevels.
func (ob *OrderBook) TotalAskDepthUSD(nLevels int) float64 {
	if nLevels > len(ob.Asks) {
		nLevels = len(ob.Asks)
	}
	total := 0.0
	for i := 0; i < nLevels; i++ {
		total += ob.Asks[i].Price * ob.Asks[i].Size
	}
	return total
}

// BookFillResult is the result of walking the book for a market order.
type BookFillResult struct {
	FilledQuantity   float64
	UnfilledQuantity float64
	AverageFillPrice float64
	LevelsConsumed   int
	PriceImpactBps   float64
	WalkedLevels     []PriceLevel
}

// WalkAsks simulates a market buy order consuming ask liquidity.
func (ob *OrderBook) WalkAsks(quantity float64) BookFillResult {
	return ob.walkSide(ob.Asks, quantity, true)
}

// WalkBids simulates a market sell order consuming bid liquidity.
func (ob *OrderBook) WalkBids(quantity float64) BookFillResult {
	return ob.walkSide(ob.Bids, quantity, false)
}

func (ob *OrderBook) walkSide(levels []PriceLevel, quantity float64, isBuy bool) BookFillResult {
	remaining := quantity
	totalCost := 0.0
	filledQty := 0.0
	walked := make([]PriceLevel, 0)
	levelsConsumed := 0

	for _, lvl := range levels {
		if remaining <= 0 {
			break
		}
		take := math.Min(remaining, lvl.Size)
		totalCost += take * lvl.Price
		filledQty += take
		remaining -= take
		walked = append(walked, PriceLevel{Price: lvl.Price, Size: take})
		levelsConsumed++
	}

	avgPrice := 0.0
	if filledQty > 0 {
		avgPrice = totalCost / filledQty
	}

	impactBps := 0.0
	if ob.Mid > 0 && avgPrice > 0 {
		if isBuy {
			impactBps = (avgPrice - ob.Mid) / ob.Mid * 10_000
		} else {
			impactBps = (ob.Mid - avgPrice) / ob.Mid * 10_000
		}
	}

	return BookFillResult{
		FilledQuantity:   filledQty,
		UnfilledQuantity: remaining,
		AverageFillPrice: avgPrice,
		LevelsConsumed:   levelsConsumed,
		PriceImpactBps:   impactBps,
		WalkedLevels:     walked,
	}
}

// DepthAtPrice returns the cumulative USD depth available up to a given price offset (bps from mid).
func (ob *OrderBook) DepthAtPrice(offsetBps float64, isBid bool) float64 {
	limitPrice := ob.Mid * (1 - offsetBps/10_000)
	if !isBid {
		limitPrice = ob.Mid * (1 + offsetBps/10_000)
	}

	levels := ob.Bids
	if !isBid {
		levels = ob.Asks
	}

	total := 0.0
	for _, lvl := range levels {
		if isBid && lvl.Price >= limitPrice {
			total += lvl.Price * lvl.Size
		} else if !isBid && lvl.Price <= limitPrice {
			total += lvl.Price * lvl.Size
		}
	}
	return total
}

// LiquidityScore derives a 0–1 score from available depth at ±5bps.
func (ob *OrderBook) LiquidityScore() float64 {
	bidDepth := ob.DepthAtPrice(5, true)
	askDepth := ob.DepthAtPrice(5, false)
	totalDepth := bidDepth + askDepth
	if totalDepth <= 0 {
		return 0.1
	}
	// Normalize against $2M reference (BTC perpetual normal depth at ±5bps)
	return clampF(totalDepth/2_000_000, 0.05, 1.0)
}

// syntheticSpreadBps computes a realistic spread from vol and liquidity.
func syntheticSpreadBps(volatilityPct, liquidityScore float64) float64 {
	base := 1.5
	volAdj := volatilityPct * 1.8
	liqAdj := (1 - liquidityScore) * 8
	spread := base + volAdj + liqAdj
	return clampF(spread, 0.5, 200)
}

// clampF is a float64 clamp helper.
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// MidPriceHistory is a ring buffer of recent mid prices for vol estimation.
type MidPriceHistory struct {
	prices []float64
	cap    int
}

func NewMidPriceHistory(cap int) *MidPriceHistory {
	return &MidPriceHistory{prices: make([]float64, 0, cap), cap: cap}
}

func (h *MidPriceHistory) Add(price float64) {
	if len(h.prices) >= h.cap {
		h.prices = h.prices[1:]
	}
	h.prices = append(h.prices, price)
}

// RealizedVolPct computes rolling realized volatility from log returns (annualized).
func (h *MidPriceHistory) RealizedVolPct() float64 {
	if len(h.prices) < 2 {
		return 0.20
	}
	returns := make([]float64, 0, len(h.prices)-1)
	for i := 1; i < len(h.prices); i++ {
		if h.prices[i-1] > 0 {
			returns = append(returns, math.Log(h.prices[i]/h.prices[i-1]))
		}
	}
	if len(returns) == 0 {
		return 0.20
	}
	sum, sum2 := 0.0, 0.0
	for _, r := range returns {
		sum += r
		sum2 += r * r
	}
	n := float64(len(returns))
	variance := (sum2 - sum*sum/n) / (n - 1)
	stdPerTick := math.Sqrt(math.Abs(variance))
	annualized := stdPerTick * math.Sqrt(365*24*60) * 100
	return clampF(annualized, 0.01, 500)
}

// SortedBids returns bids sorted by price descending (best bid first).
func SortedBids(levels []PriceLevel) []PriceLevel {
	cp := append([]PriceLevel(nil), levels...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Price > cp[j].Price })
	return cp
}

// SortedAsks returns asks sorted by price ascending (best ask first).
func SortedAsks(levels []PriceLevel) []PriceLevel {
	cp := append([]PriceLevel(nil), levels...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Price < cp[j].Price })
	return cp
}
