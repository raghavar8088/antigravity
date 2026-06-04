package live

import (
	"math"
	"sync"
	"time"
)

// FillStats is an aggregated view of execution quality across all recorded fills.
type FillStats struct {
	TotalFills     int
	TotalFilledQty float64
	TotalFeesUSD   float64

	AvgSlippageBps float64
	P50SlippageBps float64
	P95SlippageBps float64
	P99SlippageBps float64

	AvgLatencyMs float64
	P95LatencyMs float64

	FillRate   float64 // fraction of total submitted quantity that was filled
	ComputedAt time.Time
}

// FillAnalyzer accumulates fill-level execution quality metrics and computes percentile statistics.
type FillAnalyzer struct {
	mu         sync.RWMutex
	fills      []FillReport
	slippages  []float64
	latencies  []int64
	totalQty   float64
	filledQty  float64
	totalFees  float64
}

func NewFillAnalyzer() *FillAnalyzer {
	return &FillAnalyzer{}
}

// Record adds a fill report to the analyzer.
func (fa *FillAnalyzer) Record(fill FillReport) {
	fa.mu.Lock()
	defer fa.mu.Unlock()
	fa.fills = append(fa.fills, fill)
	fa.slippages = append(fa.slippages, fill.SlippageBps)
	fa.latencies = append(fa.latencies, fill.LatencyMs)
	fa.filledQty += fill.FilledQty
	fa.totalFees += fill.FeesUSD
}

// Stats computes and returns current execution quality statistics.
func (fa *FillAnalyzer) Stats() FillStats {
	fa.mu.RLock()
	defer fa.mu.RUnlock()
	n := len(fa.fills)
	if n == 0 {
		return FillStats{ComputedAt: time.Now().UTC()}
	}

	slips := insertionSortedF(fa.slippages)
	lats := insertionSortedI(fa.latencies)

	var sumSlip float64
	for _, v := range slips {
		sumSlip += v
	}
	var sumLat int64
	for _, v := range lats {
		sumLat += v
	}

	fillRate := 1.0
	if fa.totalQty > 0 {
		fillRate = fa.filledQty / fa.totalQty
	}

	return FillStats{
		TotalFills:     n,
		TotalFilledQty: fa.filledQty,
		TotalFeesUSD:   fa.totalFees,
		AvgSlippageBps: sumSlip / float64(n),
		P50SlippageBps: percF(slips, 50),
		P95SlippageBps: percF(slips, 95),
		P99SlippageBps: percF(slips, 99),
		AvgLatencyMs:   float64(sumLat) / float64(n),
		P95LatencyMs:   float64(percI(lats, 95)),
		FillRate:       fillRate,
		ComputedAt:     time.Now().UTC(),
	}
}

// insertionSortedF returns a sorted copy of a float64 slice.
// Insertion sort is efficient for the small, append-only slices typical of fill records.
func insertionSortedF(src []float64) []float64 {
	out := make([]float64, len(src))
	copy(out, src)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func insertionSortedI(src []int64) []int64 {
	out := make([]int64, len(src))
	copy(out, src)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func percF(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100.0*float64(n))) - 1
	return sorted[clampIdx(idx, n)]
}

func percI(sorted []int64, p float64) int64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(math.Ceil(p/100.0*float64(n))) - 1
	return sorted[clampIdx(idx, n)]
}

func clampIdx(idx, n int) int {
	if idx < 0 {
		return 0
	}
	if idx >= n {
		return n - 1
	}
	return idx
}
