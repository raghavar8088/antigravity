package sor

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// SplitMethod determines how a parent order is divided across venues.
type SplitMethod string

const (
	SplitSingleVenue       SplitMethod = "SINGLE_VENUE"
	SplitProportional      SplitMethod = "PROPORTIONAL"       // equal across venues
	SplitLiquidityWeighted SplitMethod = "LIQUIDITY_WEIGHTED" // weighted by executable depth
	SplitScoreWeighted     SplitMethod = "SCORE_WEIGHTED"     // weighted by best-exec score
)

// VenueAllocation is one venue's share of a split parent order.
type VenueAllocation struct {
	VenueID      VenueID
	Quantity     float64
	Weight       float64 // 0–1 share of parent
	RefPrice     float64 // reference (mid/VWAP) price for this venue
	Rationale    string
}

// SplitPlan is the output of the order splitter: how much goes to each venue.
type SplitPlan struct {
	ParentClientOrderID string
	Symbol              string
	Side                string
	TotalQuantity       float64
	Method              SplitMethod
	Allocations         []VenueAllocation
	CreatedAt           time.Time
}

// OrderSplitter divides large parent orders across multiple venues to source
// the deepest aggregate liquidity while preserving parent-child relationships.
type OrderSplitter struct {
	liquidity *LiquidityEngine
	registry  *VenueRegistry

	// MinChildNotionalUSD prevents creating dust child orders.
	MinChildNotionalUSD float64
	// SingleVenueThreshold: if the top venue can fully fill and the order is
	// below this notional, route single-venue (avoids unnecessary fragmentation).
	SingleVenueMaxNotionalUSD float64
	// MaxVenues caps how many venues a single parent is split across.
	MaxVenues int
}

// NewOrderSplitter constructs an order splitter.
func NewOrderSplitter(liquidity *LiquidityEngine, registry *VenueRegistry) *OrderSplitter {
	return &OrderSplitter{
		liquidity:                 liquidity,
		registry:                  registry,
		MinChildNotionalUSD:       50.0,
		SingleVenueMaxNotionalUSD: 100_000.0,
		MaxVenues:                 4,
	}
}

// Split builds a SplitPlan from ranked best-execution scores.
// scores must be sorted best-first and already filtered of disqualified venues
// by the caller when forceMethod == SplitSingleVenue.
func (s *OrderSplitter) Split(
	parentID, symbol, side string,
	totalQty, refPrice float64,
	scores []BestExecutionScore,
	forceMethod SplitMethod,
) (SplitPlan, error) {
	plan := SplitPlan{
		ParentClientOrderID: parentID,
		Symbol:              symbol,
		Side:                side,
		TotalQuantity:       totalQty,
		CreatedAt:           time.Now().UTC(),
	}

	qualified := make([]BestExecutionScore, 0, len(scores))
	for _, sc := range scores {
		if !sc.Disqualified {
			qualified = append(qualified, sc)
		}
	}
	if len(qualified) == 0 {
		return plan, fmt.Errorf("sor: no qualified venues to split across")
	}

	notional := totalQty * refPrice
	method := forceMethod
	if method == "" {
		// Auto-select: single venue if the best venue fully covers and order is small.
		if qualified[0].FullyExecutable && notional <= s.SingleVenueMaxNotionalUSD {
			method = SplitSingleVenue
		} else {
			method = SplitLiquidityWeighted
		}
	}
	plan.Method = method

	switch method {
	case SplitSingleVenue:
		plan.Allocations = []VenueAllocation{{
			VenueID:   qualified[0].VenueID,
			Quantity:  totalQty,
			Weight:    1.0,
			RefPrice:  refPrice,
			Rationale: "single-venue: best score, full executable",
		}}

	case SplitProportional:
		plan.Allocations = s.splitEqual(qualified, totalQty, refPrice)

	case SplitScoreWeighted:
		plan.Allocations = s.splitByWeight(qualified, totalQty, refPrice, func(sc BestExecutionScore) float64 {
			return sc.Score
		}, "score-weighted")

	default: // SplitLiquidityWeighted
		plan.Allocations = s.splitByWeight(qualified, totalQty, refPrice, func(sc BestExecutionScore) float64 {
			return math.Max(sc.ExecutableQty, 0)
		}, "liquidity-weighted")
	}

	// Drop dust allocations and renormalise.
	plan.Allocations = s.pruneDust(plan.Allocations, refPrice, totalQty)
	if len(plan.Allocations) == 0 {
		return plan, fmt.Errorf("sor: split produced no executable allocations")
	}
	return plan, nil
}

func (s *OrderSplitter) splitEqual(scores []BestExecutionScore, totalQty, refPrice float64) []VenueAllocation {
	n := s.cap(len(scores))
	per := totalQty / float64(n)
	out := make([]VenueAllocation, 0, n)
	allocated := 0.0
	for i := 0; i < n; i++ {
		qty := per
		if i == n-1 {
			qty = totalQty - allocated // last venue absorbs rounding
		}
		allocated += qty
		out = append(out, VenueAllocation{
			VenueID:   scores[i].VenueID,
			Quantity:  qty,
			Weight:    qty / totalQty,
			RefPrice:  refPrice,
			Rationale: "proportional equal split",
		})
	}
	return out
}

func (s *OrderSplitter) splitByWeight(
	scores []BestExecutionScore,
	totalQty, refPrice float64,
	weightFn func(BestExecutionScore) float64,
	label string,
) []VenueAllocation {
	n := s.cap(len(scores))
	top := scores[:n]

	totalWeight := 0.0
	for _, sc := range top {
		totalWeight += math.Max(weightFn(sc), 0)
	}
	if totalWeight <= 0 {
		return s.splitEqual(scores, totalQty, refPrice)
	}

	out := make([]VenueAllocation, 0, n)
	allocated := 0.0
	for i, sc := range top {
		w := math.Max(weightFn(sc), 0) / totalWeight
		qty := totalQty * w
		if i == n-1 {
			qty = totalQty - allocated
		}
		allocated += qty
		out = append(out, VenueAllocation{
			VenueID:   sc.VenueID,
			Quantity:  qty,
			Weight:    w,
			RefPrice:  refPrice,
			Rationale: fmt.Sprintf("%s w=%.3f", label, w),
		})
	}
	return out
}

func (s *OrderSplitter) cap(n int) int {
	if s.MaxVenues > 0 && n > s.MaxVenues {
		return s.MaxVenues
	}
	return n
}

func (s *OrderSplitter) pruneDust(allocs []VenueAllocation, refPrice, totalQty float64) []VenueAllocation {
	if len(allocs) == 0 {
		return allocs
	}
	kept := make([]VenueAllocation, 0, len(allocs))
	dustQty := 0.0
	for _, a := range allocs {
		if a.Quantity*refPrice < s.MinChildNotionalUSD {
			dustQty += a.Quantity
			continue
		}
		kept = append(kept, a)
	}

	// If all allocations are dust (total notional < min), consolidate into the
	// largest allocation rather than returning empty (avoids "no executable" error).
	if len(kept) == 0 {
		best := allocs[0]
		for _, a := range allocs[1:] {
			if a.Quantity > best.Quantity {
				best = a
			}
		}
		best.Quantity = totalQty
		best.Weight = 1.0
		return []VenueAllocation{best}
	}

	// Re-add dust quantity to the largest remaining allocation.
	if dustQty > 0 {
		sort.Slice(kept, func(i, j int) bool { return kept[i].Quantity > kept[j].Quantity })
		kept[0].Quantity += dustQty
	}
	// Recompute weights.
	for i := range kept {
		kept[i].Weight = kept[i].Quantity / totalQty
	}
	return kept
}
