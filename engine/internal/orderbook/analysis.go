package orderbook

import (
	"math"
	"time"
)

const (
	wallSearchPct    = 0.005 // search within ±0.5% of mid price
	bandWidthPct     = 0.001 // 0.1%-wide price bands for wall clustering
	top10Levels      = 10
	spreadHistoryLen = 20
)

// Analyse computes an OrderBookAnalysis from the current order book snapshot.
func Analyse(book OrderBook, currentPrice float64, spreadHistory []float64) OrderBookAnalysis {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return OrderBookAnalysis{AnalysedAt: time.Now().UTC(), SpreadNormal: true}
	}

	bestBid := book.Bids[0].Price
	bestAsk := book.Asks[0].Price
	mid := (bestBid + bestAsk) / 2.0
	if mid == 0 {
		mid = currentPrice
	}

	// ── Bid wall ─────────────────────────────────────────────────────────────
	bidWallPrice, bidWallSize := findWall(book.Bids, mid, false)

	// ── Ask wall ─────────────────────────────────────────────────────────────
	askWallPrice, askWallSize := findWall(book.Asks, mid, true)

	// ── Depth imbalance ───────────────────────────────────────────────────────
	bidVol := levelSum(book.Bids, top10Levels)
	askVol := levelSum(book.Asks, top10Levels)
	imbalance := 1.0
	if askVol > 0 {
		imbalance = bidVol / askVol
	}
	imbalSignal := "NEUTRAL"
	switch {
	case imbalance > 1.5:
		imbalSignal = "BUY_PRESSURE"
	case imbalance < 0.7:
		imbalSignal = "SELL_PRESSURE"
	}

	// ── Spread ────────────────────────────────────────────────────────────────
	spreadBps := 0.0
	if mid > 0 {
		spreadBps = (bestAsk - bestBid) / mid * 10000
	}
	avgSpread := average(spreadHistory)
	spreadNormal := avgSpread == 0 || spreadBps <= 3*avgSpread

	// ── Score ─────────────────────────────────────────────────────────────────
	score := computeScore(bidWallSize, askWallSize, imbalance, spreadNormal)

	return OrderBookAnalysis{
		BidWallPrice:   bidWallPrice,
		BidWallSize:    bidWallSize,
		AskWallPrice:   askWallPrice,
		AskWallSize:    askWallSize,
		DepthImbalance: imbalance,
		ImbalanceSignal: imbalSignal,
		SpreadBps:      spreadBps,
		SpreadNormal:   spreadNormal,
		Score:          score,
		AnalysedAt:     time.Now().UTC(),
	}
}

// findWall locates the largest total quantity within a 0.1%-wide band
// in the search window closest to the mid price.
// askSide = true for asks (prices above mid), false for bids (below mid).
func findWall(levels []PriceLevel, mid float64, askSide bool) (wallPrice, wallSize float64) {
	lo := mid * (1 - wallSearchPct)
	hi := mid * (1 + wallSearchPct)

	type band struct {
		base float64
		qty  float64
	}
	bands := make(map[int]float64) // band index → total qty

	for _, lv := range levels {
		if askSide && (lv.Price < mid || lv.Price > hi) {
			continue
		}
		if !askSide && (lv.Price > mid || lv.Price < lo) {
			continue
		}
		// Assign to a 0.1%-wide band.
		idx := int(lv.Price / (mid * bandWidthPct))
		bands[idx] += lv.Quantity
	}

	var bestIdx int
	var bestQty float64
	for idx, qty := range bands {
		if qty > bestQty {
			bestQty = qty
			bestIdx = idx
		}
	}

	// Reconstruct approximate price from band index.
	if bestQty > 0 {
		wallPrice = float64(bestIdx) * (mid * bandWidthPct)
		wallSize = bestQty
	}
	return wallPrice, wallSize
}

// levelSum returns the total quantity of the first n levels.
func levelSum(levels []PriceLevel, n int) float64 {
	if n > len(levels) {
		n = len(levels)
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += levels[i].Quantity
	}
	return sum
}

// computeScore maps order book signals to a [-3, +3] score.
func computeScore(bidWall, askWall, imbalance float64, spreadNormal bool) float64 {
	score := 0.0

	switch {
	case bidWall > askWall*1.5: // strong bid wall vs ask wall
		score += 2.0
	case bidWall > askWall: // moderate bid wall
		score += 1.0
	case askWall > bidWall*1.5: // strong ask wall vs bid wall
		score -= 2.0
	case askWall > bidWall:
		score -= 1.0
	}

	if imbalance > 1.5 {
		score += 1.0
	} else if imbalance < 0.7 {
		score -= 1.0
	}

	if !spreadNormal {
		score -= 0.5
	}

	return math.Max(-3, math.Min(3, score))
}

// average returns the mean of vals; returns 0 for empty slice.
func average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
