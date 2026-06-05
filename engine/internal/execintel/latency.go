package execintel

import (
	"math"
	"sort"
)

// latencySeries is a bounded sample reservoir for one (stage,dimension) pair.
// It keeps the most recent N samples so percentiles reflect current behaviour.
type latencySeries struct {
	samples []float64
	head    int
	count   int
	cap     int
	sum     float64
	max     float64
	min     float64
}

func newLatencySeries(capacity int) *latencySeries {
	return &latencySeries{samples: make([]float64, capacity), cap: capacity, min: math.Inf(1)}
}

func (s *latencySeries) add(ms float64) {
	// Maintain running sum across the ring by subtracting the slot we overwrite.
	if s.count == s.cap {
		s.sum -= s.samples[s.head]
	}
	s.samples[s.head] = ms
	s.head = (s.head + 1) % s.cap
	if s.count < s.cap {
		s.count++
	}
	s.sum += ms
	if ms > s.max {
		s.max = ms
	}
	if ms < s.min {
		s.min = ms
	}
}

// LatencyStats is a percentile summary in milliseconds.
type LatencyStats struct {
	Count int     `json:"count"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Avg   float64 `json:"avg"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

func (s *latencySeries) stats() LatencyStats {
	if s.count == 0 {
		return LatencyStats{}
	}
	buf := make([]float64, s.count)
	copy(buf, s.samples[:s.count])
	sort.Float64s(buf)
	return LatencyStats{
		Count: s.count,
		P50:   percentile(buf, 0.50),
		P95:   percentile(buf, 0.95),
		P99:   percentile(buf, 0.99),
		Avg:   s.sum / float64(s.count),
		Min:   s.min,
		Max:   s.max,
	}
}

// percentile returns the p-th percentile of a pre-sorted slice using
// nearest-rank interpolation. p in [0,1].
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := p * float64(n-1)
	lo := int(math.Floor(rank))
	hi := int(math.Ceil(rank))
	if lo == hi {
		return sorted[lo]
	}
	frac := rank - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

const latencySeriesCap = 2048

// latencyBook holds latency series across three dimensions: per-stage (overall),
// per stage+strategy, and per stage+regime.
type latencyBook struct {
	overall  map[string]*latencySeries // key: stage
	byStrat  map[string]*latencySeries // key: stage|strategy
	byRegime map[string]*latencySeries // key: stage|regime
}

func newLatencyBook() *latencyBook {
	return &latencyBook{
		overall:  make(map[string]*latencySeries),
		byStrat:  make(map[string]*latencySeries),
		byRegime: make(map[string]*latencySeries),
	}
}

func (b *latencyBook) seriesFor(m map[string]*latencySeries, key string) *latencySeries {
	s := m[key]
	if s == nil {
		s = newLatencySeries(latencySeriesCap)
		m[key] = s
	}
	return s
}

func (b *latencyBook) add(stage, strategy, regime string, ms float64) {
	b.seriesFor(b.overall, stage).add(ms)
	if strategy != "" {
		b.seriesFor(b.byStrat, stage+"|"+strategy).add(ms)
	}
	if regime != "" {
		b.seriesFor(b.byRegime, stage+"|"+regime).add(ms)
	}
}

// LatencyReport is the full latency snapshot.
type LatencyReport struct {
	ByStage    map[string]LatencyStats `json:"byStage"`
	ByStrategy map[string]LatencyStats `json:"byStrategy"`
	ByRegime   map[string]LatencyStats `json:"byRegime"`
}

func (b *latencyBook) report() LatencyReport {
	out := LatencyReport{
		ByStage:    make(map[string]LatencyStats, len(b.overall)),
		ByStrategy: make(map[string]LatencyStats, len(b.byStrat)),
		ByRegime:   make(map[string]LatencyStats, len(b.byRegime)),
	}
	for k, s := range b.overall {
		out.ByStage[k] = s.stats()
	}
	for k, s := range b.byStrat {
		out.ByStrategy[k] = s.stats()
	}
	for k, s := range b.byRegime {
		out.ByRegime[k] = s.stats()
	}
	return out
}
