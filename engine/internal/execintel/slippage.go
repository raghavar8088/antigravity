package execintel

import (
	"math"
	"sort"
	"strings"
)

// SlippageSample is one fill's realized slippage versus the decision price.
type SlippageSample struct {
	Strategy    string
	AlphaSource string
	Category    string
	Session     string // ASIA | LONDON | NEW_YORK (UTC-derived)
	Regime      string
	Direction   string  // BUY | SELL
	SignalPrice float64 // price at decision time
	FilledPrice float64 // actual execution price
	Size        float64
}

// SlippageBps returns signed slippage in basis points. Positive = adverse
// (filled worse than decision price for the trade's direction).
func (s SlippageSample) SlippageBps() float64 {
	if s.SignalPrice <= 0 || s.FilledPrice <= 0 {
		return 0
	}
	raw := (s.FilledPrice - s.SignalPrice) / s.SignalPrice * 10000
	if strings.EqualFold(s.Direction, "SELL") {
		// A SELL filled below the decision price is adverse → make adverse positive.
		return -raw
	}
	return raw
}

// CostUSD returns the dollar cost of this fill's slippage (adverse = positive cost).
func (s SlippageSample) CostUSD() float64 {
	if s.SignalPrice <= 0 || s.FilledPrice <= 0 {
		return 0
	}
	return math.Abs(s.FilledPrice-s.SignalPrice) * s.Size * sign(s.SlippageBps())
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

const slippageSeriesCap = 4096

type slippageSeries struct {
	bps     []float64
	head    int
	count   int
	sumBps  float64
	sumCost float64
	worst   float64
}

func newSlippageSeries() *slippageSeries {
	return &slippageSeries{bps: make([]float64, slippageSeriesCap)}
}

func (s *slippageSeries) add(bps, costUSD float64) {
	if s.count == slippageSeriesCap {
		s.sumBps -= s.bps[s.head]
	}
	s.bps[s.head] = bps
	s.head = (s.head + 1) % slippageSeriesCap
	if s.count < slippageSeriesCap {
		s.count++
	}
	s.sumBps += bps
	s.sumCost += costUSD
	if bps > s.worst {
		s.worst = bps
	}
}

// SlippageStats summarizes slippage for a dimension.
type SlippageStats struct {
	Count      int     `json:"count"`
	AvgBps     float64 `json:"avgBps"`
	MedianBps  float64 `json:"medianBps"`
	WorstBps   float64 `json:"worstBps"`
	TotalCost  float64 `json:"totalCostUSD"`
	AvgPctMove float64 `json:"avgPct"` // avg slippage as a percentage (bps/100)
}

func (s *slippageSeries) stats() SlippageStats {
	if s.count == 0 {
		return SlippageStats{}
	}
	buf := make([]float64, s.count)
	copy(buf, s.bps[:s.count])
	sort.Float64s(buf)
	avg := s.sumBps / float64(s.count)
	return SlippageStats{
		Count:      s.count,
		AvgBps:     avg,
		MedianBps:  percentile(buf, 0.50),
		WorstBps:   s.worst,
		TotalCost:  s.sumCost,
		AvgPctMove: avg / 100.0,
	}
}

type slippageBook struct {
	overall    *slippageSeries
	byStrategy map[string]*slippageSeries
	byAlpha    map[string]*slippageSeries
	bySession  map[string]*slippageSeries
	byRegime   map[string]*slippageSeries
	byDir      map[string]*slippageSeries
}

func newSlippageBook() *slippageBook {
	return &slippageBook{
		overall:    newSlippageSeries(),
		byStrategy: make(map[string]*slippageSeries),
		byAlpha:    make(map[string]*slippageSeries),
		bySession:  make(map[string]*slippageSeries),
		byRegime:   make(map[string]*slippageSeries),
		byDir:      make(map[string]*slippageSeries),
	}
}

func seriesIn(m map[string]*slippageSeries, key string) *slippageSeries {
	if key == "" {
		key = "unknown"
	}
	s := m[key]
	if s == nil {
		s = newSlippageSeries()
		m[key] = s
	}
	return s
}

func (b *slippageBook) add(sample SlippageSample) {
	bps := sample.SlippageBps()
	cost := sample.CostUSD()
	b.overall.add(bps, cost)
	seriesIn(b.byStrategy, sample.Strategy).add(bps, cost)
	seriesIn(b.byAlpha, sample.AlphaSource).add(bps, cost)
	seriesIn(b.bySession, sample.Session).add(bps, cost)
	seriesIn(b.byRegime, sample.Regime).add(bps, cost)
	seriesIn(b.byDir, strings.ToUpper(sample.Direction)).add(bps, cost)
}

// SlippageReport is the full slippage snapshot.
type SlippageReport struct {
	Overall    SlippageStats            `json:"overall"`
	ByStrategy map[string]SlippageStats `json:"byStrategy"`
	ByAlpha    map[string]SlippageStats `json:"byAlpha"`
	BySession  map[string]SlippageStats `json:"bySession"`
	ByRegime   map[string]SlippageStats `json:"byRegime"`
	ByDir      map[string]SlippageStats `json:"byDirection"`
}

func mapStats(m map[string]*slippageSeries) map[string]SlippageStats {
	out := make(map[string]SlippageStats, len(m))
	for k, s := range m {
		out[k] = s.stats()
	}
	return out
}

func (b *slippageBook) report() SlippageReport {
	return SlippageReport{
		Overall:    b.overall.stats(),
		ByStrategy: mapStats(b.byStrategy),
		ByAlpha:    mapStats(b.byAlpha),
		BySession:  mapStats(b.bySession),
		ByRegime:   mapStats(b.byRegime),
		ByDir:      mapStats(b.byDir),
	}
}

// SessionForUTC maps a UTC time to a trading session bucket used for slippage
// attribution. Matches the session boundaries in internal/alpha/session.
func SessionForUTC(hourUTC int) string {
	switch {
	case hourUTC >= 0 && hourUTC < 8:
		return "ASIA"
	case hourUTC >= 8 && hourUTC < 13:
		return "LONDON"
	default:
		return "NEW_YORK"
	}
}
