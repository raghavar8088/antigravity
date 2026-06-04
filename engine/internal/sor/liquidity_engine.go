package sor

import (
	"sort"
)

// LiquidityResult describes the executable liquidity for a proposed order
// against one venue's order book. It distinguishes quoted liquidity (top of
// book) from executable liquidity (what can actually be filled walking the book).
type LiquidityResult struct {
	VenueID VenueID

	// Executable analysis
	ExecutableQty   float64 // base-asset qty fillable within MaxWalkBps of touch
	VWAP            float64 // volume-weighted avg price to fill min(qty, executable)
	TouchPrice      float64 // best bid (sell) or best ask (buy)
	WalkBps         float64 // distance from touch to VWAP in bps (price impact estimate)
	TotalBookDepth  float64 // total visible depth on the relevant side

	// Scores (0–1, higher = better)
	DepthScore      float64 // executable qty / requested qty, capped at 1
	LiquidityScore  float64 // composite executable-liquidity score

	// Risk flags
	ThinBook        bool // requested qty consumes too large a share of the book
	LiquidityTrap   bool // top-of-book size tiny relative to deeper levels (spoof-like)
	FullyExecutable bool // executable qty >= requested qty
}

// LiquidityEngine analyses order books to route on executable (not quoted)
// liquidity. It is stateless and deterministic.
type LiquidityEngine struct {
	// MaxWalkBps is how far down the book we are willing to walk to count
	// liquidity as "executable". Beyond this, liquidity is ignored.
	MaxWalkBps float64
	// ThinBookConsumptionPct flags a book as thin when the order would consume
	// more than this fraction of total visible depth on the relevant side.
	ThinBookConsumptionPct float64
	// TrapTopOfBookRatio flags a liquidity trap when top-of-book size is less
	// than this fraction of the average level size deeper in the book.
	TrapTopOfBookRatio float64
}

// NewLiquidityEngine returns a liquidity engine with institutional defaults.
func NewLiquidityEngine() *LiquidityEngine {
	return &LiquidityEngine{
		MaxWalkBps:             50.0, // 50 bps
		ThinBookConsumptionPct: 0.40, // consuming >40% of book = thin
		TrapTopOfBookRatio:     0.10, // top < 10% of avg deeper = trap
	}
}

// Analyse computes executable liquidity for a (side, qty) against the venue's book.
func (e *LiquidityEngine) Analyse(md VenueMarketData, side string, qty float64) LiquidityResult {
	res := LiquidityResult{VenueID: md.VenueID}
	if qty <= 0 {
		return res
	}

	// Choose the side of the book we consume against.
	// BUY consumes asks (ascending); SELL consumes bids (descending).
	var levels []PriceLevel
	if isBuy(side) {
		levels = sortedAsks(md.Asks)
		res.TouchPrice = md.AskPrice
		if res.TouchPrice == 0 && len(levels) > 0 {
			res.TouchPrice = levels[0].Price
		}
	} else {
		levels = sortedBids(md.Bids)
		res.TouchPrice = md.BidPrice
		if res.TouchPrice == 0 && len(levels) > 0 {
			res.TouchPrice = levels[0].Price
		}
	}

	if len(levels) == 0 || res.TouchPrice <= 0 {
		// Fall back to top-of-book size only.
		if isBuy(side) {
			res.TotalBookDepth = md.AskSize
		} else {
			res.TotalBookDepth = md.BidSize
		}
		res.ExecutableQty = res.TotalBookDepth
		res.VWAP = res.TouchPrice
		res.DepthScore = clamp(res.ExecutableQty/qty, 0, 1)
		res.LiquidityScore = res.DepthScore
		res.FullyExecutable = res.ExecutableQty >= qty
		res.ThinBook = !res.FullyExecutable
		return res
	}

	// Walk the book accumulating executable quantity within MaxWalkBps.
	maxWalkPrice := walkLimitPrice(res.TouchPrice, side, e.MaxWalkBps)
	var (
		accQty     float64
		accNotional float64
		totalDepth float64
		deeperSizeSum float64
		deeperCount   int
	)
	for i, lvl := range levels {
		totalDepth += lvl.Size
		if i > 0 {
			deeperSizeSum += lvl.Size
			deeperCount++
		}
		if !withinWalk(lvl.Price, maxWalkPrice, side) {
			continue
		}
		need := qty - accQty
		if need <= 0 {
			continue
		}
		take := lvl.Size
		if take > need {
			take = need
		}
		accQty += take
		accNotional += take * lvl.Price
	}

	res.TotalBookDepth = totalDepth
	res.ExecutableQty = accQty
	if accQty > 0 {
		res.VWAP = accNotional / accQty
		res.WalkBps = priceImpactBps(res.TouchPrice, res.VWAP, side)
	} else {
		res.VWAP = res.TouchPrice
	}

	res.DepthScore = clamp(accQty/qty, 0, 1)
	res.FullyExecutable = accQty >= qty-1e-9

	// Thin-book detection
	if totalDepth > 0 && qty/totalDepth > e.ThinBookConsumptionPct {
		res.ThinBook = true
	}

	// Liquidity-trap detection: tiny touch size vs deeper average.
	if deeperCount > 0 {
		avgDeeper := deeperSizeSum / float64(deeperCount)
		if avgDeeper > 0 && levels[0].Size < avgDeeper*e.TrapTopOfBookRatio {
			res.LiquidityTrap = true
		}
	}

	// Composite liquidity score: depth dominates, penalise impact and traps.
	impactPenalty := clamp(res.WalkBps/e.MaxWalkBps, 0, 1)
	score := res.DepthScore*0.7 + (1-impactPenalty)*0.3
	if res.ThinBook {
		score *= 0.6
	}
	if res.LiquidityTrap {
		score *= 0.5
	}
	res.LiquidityScore = clamp(score, 0, 1)
	return res
}

// DeepestLiquidity returns the venue with the highest executable liquidity score
// for the given order among the provided candidates.
func (e *LiquidityEngine) DeepestLiquidity(reg *VenueRegistry, candidates []*Venue, symbol, side string, qty float64) (VenueID, LiquidityResult) {
	var best VenueID
	var bestRes LiquidityResult
	bestScore := -1.0
	for _, v := range candidates {
		md, ok := reg.MarketData(v.ID, symbol)
		if !ok {
			continue
		}
		r := e.Analyse(md, side, qty)
		if r.LiquidityScore > bestScore {
			bestScore = r.LiquidityScore
			best = v.ID
			bestRes = r
		}
	}
	return best, bestRes
}

// ── helpers ───────────────────────────────────────────────────────────────────

func isBuy(side string) bool {
	return side == "BUY" || side == "LONG" || side == "buy"
}

func sortedAsks(asks []PriceLevel) []PriceLevel {
	out := append([]PriceLevel(nil), asks...)
	sort.Slice(out, func(i, j int) bool { return out[i].Price < out[j].Price })
	return out
}

func sortedBids(bids []PriceLevel) []PriceLevel {
	out := append([]PriceLevel(nil), bids...)
	sort.Slice(out, func(i, j int) bool { return out[i].Price > out[j].Price })
	return out
}

// walkLimitPrice returns the worst price we will accept walking the book.
func walkLimitPrice(touch float64, side string, maxWalkBps float64) float64 {
	delta := touch * maxWalkBps / 10000
	if isBuy(side) {
		return touch + delta // buying: accept higher asks up to touch+delta
	}
	return touch - delta // selling: accept lower bids down to touch-delta
}

func withinWalk(price, limit float64, side string) bool {
	if isBuy(side) {
		return price <= limit
	}
	return price >= limit
}

// priceImpactBps returns the absolute bps distance from touch to vwap.
func priceImpactBps(touch, vwap float64, side string) float64 {
	if touch <= 0 {
		return 0
	}
	var diff float64
	if isBuy(side) {
		diff = vwap - touch
	} else {
		diff = touch - vwap
	}
	if diff < 0 {
		diff = 0
	}
	return diff / touch * 10000
}
