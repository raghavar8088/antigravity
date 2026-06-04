package sor

import (
	"fmt"
	"time"
)

// ExecutionAlgo identifies a child-order scheduling algorithm.
type ExecutionAlgo string

const (
	AlgoImmediate       ExecutionAlgo = "IMMEDIATE" // one slice, now
	AlgoTWAP            ExecutionAlgo = "TWAP"      // equal slices over time
	AlgoVWAP            ExecutionAlgo = "VWAP"      // volume-profile weighted slices
	AlgoPOV             ExecutionAlgo = "POV"       // percentage of volume
	AlgoIceberg         ExecutionAlgo = "ICEBERG"   // small visible clips
	AlgoLiquiditySeeking ExecutionAlgo = "LIQUIDITY_SEEKING"
	AlgoAdaptive        ExecutionAlgo = "ADAPTIVE"
)

// ChildOrder is a single executable slice routed to one venue.
// It preserves the parent-child relationship for audit and reconciliation.
type ChildOrder struct {
	ChildOrderID        string
	ParentClientOrderID string
	VenueID             VenueID
	Symbol              string
	Side                string
	Quantity            float64
	OrderType           string  // MARKET | LIMIT | POST_ONLY | IOC
	LimitPrice          float64
	ReferencePrice      float64 // mid/VWAP reference at planning time
	SequenceIndex       int     // ordering within the parent execution
	ScheduledAt         time.Time
}

// AlgoParams parameterises an execution algorithm.
type AlgoParams struct {
	Algo ExecutionAlgo
	// Slices is the number of time-slices (TWAP/VWAP). 0 → auto.
	Slices int
	// Duration is the total execution horizon. 0 → immediate.
	Duration time.Duration
	// POVRate is the target participation rate for POV (0–1).
	POVRate float64
	// IcebergClipQty is the visible clip size for Iceberg.
	IcebergClipQty float64
	// VolumeProfile is an optional intraday volume curve (per slice). If empty,
	// VWAP falls back to a symmetric U-shaped default.
	VolumeProfile []float64
	// StartAt is the schedule anchor; defaults to now.
	StartAt time.Time
}

// AlgoScheduler expands a per-venue allocation into time-scheduled child orders.
type AlgoScheduler struct {
	// MaxSlices caps slice count to bound child-order cardinality.
	MaxSlices int
}

// NewAlgoScheduler constructs an algo scheduler.
func NewAlgoScheduler() *AlgoScheduler {
	return &AlgoScheduler{MaxSlices: 50}
}

// Schedule turns a single venue allocation into one or more scheduled child orders.
func (s *AlgoScheduler) Schedule(
	parentID, symbol, side, orderType string,
	alloc VenueAllocation,
	params AlgoParams,
	childSeqStart int,
) []ChildOrder {
	start := params.StartAt
	if start.IsZero() {
		start = time.Now().UTC()
	}

	switch params.Algo {
	case AlgoTWAP:
		return s.scheduleTWAP(parentID, symbol, side, orderType, alloc, params, start, childSeqStart)
	case AlgoVWAP:
		return s.scheduleVWAP(parentID, symbol, side, orderType, alloc, params, start, childSeqStart)
	case AlgoPOV:
		return s.schedulePOV(parentID, symbol, side, orderType, alloc, params, start, childSeqStart)
	case AlgoIceberg:
		return s.scheduleIceberg(parentID, symbol, side, orderType, alloc, params, start, childSeqStart)
	case AlgoLiquiditySeeking, AlgoAdaptive:
		// Liquidity-seeking / adaptive execute opportunistically: a few IOC clips.
		return s.scheduleAdaptive(parentID, symbol, side, orderType, alloc, params, start, childSeqStart)
	default: // AlgoImmediate
		return []ChildOrder{s.child(parentID, symbol, side, orderType, alloc, alloc.Quantity, childSeqStart, start)}
	}
}

func (s *AlgoScheduler) scheduleTWAP(parentID, symbol, side, orderType string, alloc VenueAllocation, p AlgoParams, start time.Time, seq int) []ChildOrder {
	n := s.sliceCount(p.Slices, 5)
	per := alloc.Quantity / float64(n)
	interval := s.sliceInterval(p.Duration, n)
	out := make([]ChildOrder, 0, n)
	allocated := 0.0
	for i := 0; i < n; i++ {
		qty := per
		if i == n-1 {
			qty = alloc.Quantity - allocated
		}
		allocated += qty
		out = append(out, s.child(parentID, symbol, side, orderType, alloc, qty, seq+i, start.Add(time.Duration(i)*interval)))
	}
	return out
}

func (s *AlgoScheduler) scheduleVWAP(parentID, symbol, side, orderType string, alloc VenueAllocation, p AlgoParams, start time.Time, seq int) []ChildOrder {
	profile := p.VolumeProfile
	n := s.sliceCount(p.Slices, len(profile))
	if n == 0 {
		n = 5
	}
	if len(profile) != n {
		profile = defaultVolumeProfile(n) // U-shaped intraday curve
	}
	total := 0.0
	for _, w := range profile {
		total += w
	}
	if total <= 0 {
		total = 1
	}
	interval := s.sliceInterval(p.Duration, n)
	out := make([]ChildOrder, 0, n)
	allocated := 0.0
	for i := 0; i < n; i++ {
		qty := alloc.Quantity * profile[i] / total
		if i == n-1 {
			qty = alloc.Quantity - allocated
		}
		allocated += qty
		out = append(out, s.child(parentID, symbol, side, orderType, alloc, qty, seq+i, start.Add(time.Duration(i)*interval)))
	}
	return out
}

func (s *AlgoScheduler) schedulePOV(parentID, symbol, side, orderType string, alloc VenueAllocation, p AlgoParams, start time.Time, seq int) []ChildOrder {
	rate := p.POVRate
	if rate <= 0 {
		rate = 0.10 // 10% of volume
	}
	// Each clip targets `rate` participation; clip size derived from rate.
	// With no live volume feed at planning time, we approximate clip count
	// from the participation rate (lower rate → more, smaller clips).
	n := s.sliceCount(0, int(1.0/rate))
	if n < 1 {
		n = 1
	}
	per := alloc.Quantity / float64(n)
	interval := s.sliceInterval(p.Duration, n)
	out := make([]ChildOrder, 0, n)
	allocated := 0.0
	for i := 0; i < n; i++ {
		qty := per
		if i == n-1 {
			qty = alloc.Quantity - allocated
		}
		allocated += qty
		child := s.child(parentID, symbol, side, "IOC", alloc, qty, seq+i, start.Add(time.Duration(i)*interval))
		out = append(out, child)
	}
	return out
}

func (s *AlgoScheduler) scheduleIceberg(parentID, symbol, side, orderType string, alloc VenueAllocation, p AlgoParams, start time.Time, seq int) []ChildOrder {
	clip := p.IcebergClipQty
	if clip <= 0 || clip >= alloc.Quantity {
		clip = alloc.Quantity / 5 // default: 5 clips
	}
	if clip <= 0 {
		clip = alloc.Quantity
	}
	out := make([]ChildOrder, 0)
	remaining := alloc.Quantity
	i := 0
	for remaining > 1e-12 {
		qty := clip
		if qty > remaining {
			qty = remaining
		}
		remaining -= qty
		// Iceberg clips are passive (post-only) to hide size.
		out = append(out, s.child(parentID, symbol, side, "POST_ONLY", alloc, qty, seq+i, start))
		i++
		if i >= s.MaxSlices {
			// Absorb any remainder into the last clip.
			if remaining > 0 {
				out[len(out)-1].Quantity += remaining
			}
			break
		}
	}
	return out
}

func (s *AlgoScheduler) scheduleAdaptive(parentID, symbol, side, orderType string, alloc VenueAllocation, p AlgoParams, start time.Time, seq int) []ChildOrder {
	// Adaptive: front-load with an aggressive IOC clip, then passive clips.
	n := s.sliceCount(p.Slices, 3)
	out := make([]ChildOrder, 0, n)
	// Geometric front-loading: first clip 50%, rest split the remainder.
	first := alloc.Quantity * 0.5
	out = append(out, s.child(parentID, symbol, side, "IOC", alloc, first, seq, start))
	rest := alloc.Quantity - first
	if n > 1 {
		per := rest / float64(n-1)
		allocated := first
		interval := s.sliceInterval(p.Duration, n)
		for i := 1; i < n; i++ {
			qty := per
			if i == n-1 {
				qty = alloc.Quantity - allocated
			}
			allocated += qty
			out = append(out, s.child(parentID, symbol, side, "POST_ONLY", alloc, qty, seq+i, start.Add(time.Duration(i)*interval)))
		}
	}
	return out
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (s *AlgoScheduler) child(parentID, symbol, side, orderType string, alloc VenueAllocation, qty float64, seq int, at time.Time) ChildOrder {
	return ChildOrder{
		ChildOrderID:        fmt.Sprintf("%s-c%d-%s", parentID, seq, alloc.VenueID),
		ParentClientOrderID: parentID,
		VenueID:             alloc.VenueID,
		Symbol:              symbol,
		Side:                side,
		Quantity:            qty,
		OrderType:           orderType,
		ReferencePrice:      alloc.RefPrice,
		SequenceIndex:       seq,
		ScheduledAt:         at,
	}
}

func (s *AlgoScheduler) sliceCount(requested, fallback int) int {
	n := requested
	if n <= 0 {
		n = fallback
	}
	if n <= 0 {
		n = 1
	}
	if s.MaxSlices > 0 && n > s.MaxSlices {
		n = s.MaxSlices
	}
	return n
}

func (s *AlgoScheduler) sliceInterval(duration time.Duration, n int) time.Duration {
	if duration <= 0 || n <= 1 {
		return 0
	}
	return duration / time.Duration(n)
}

// defaultVolumeProfile returns a U-shaped intraday volume curve (heavier at
// open and close), the institutional default for VWAP scheduling.
func defaultVolumeProfile(n int) []float64 {
	if n <= 0 {
		return nil
	}
	profile := make([]float64, n)
	for i := 0; i < n; i++ {
		denom := n - 1
		if denom == 0 {
			denom = 1
		}
		x := float64(i) / float64(denom) // 0..1
		// U-shape: parabola with minimum at midday.
		profile[i] = 1.0 + 2.0*(x-0.5)*(x-0.5)*2.0
	}
	return profile
}
