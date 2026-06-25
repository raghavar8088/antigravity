package backtest

import (
	"sort"
	"time"

	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ContextBuilder reconstructs a scalers.MarketContext at any historical bar index
// from a pre-loaded MTFDataset. It is the backtest-time equivalent of
// ScalerBundle.buildContextLocked() in the live path.
//
// Look-ahead bias guarantee: BuildAt(i) may only include candle data whose
// OpenTime is strictly before dataset.Candles15m[i].OpenTime + 15 minutes.
// All candle slices are sliced to bar i (exclusive) before returning.
type ContextBuilder struct {
	ds marketdata.MTFDataset

	// Pre-converted candle arrays — built once at construction, sliced cheaply per bar.
	sc1m  []scalers.Candle
	sc5m  []scalers.Candle
	sc15m []scalers.Candle
	sc1h  []scalers.Candle
	sc4h  []scalers.Candle

	// Pre-extracted open times for O(log N) binary search per bar.
	t1m  []int64
	t5m  []int64
	t15m []int64
	t1h  []int64
	t4h  []int64

	// Pre-indexed funding/OI sorted slices
	fundingRates []marketdata.HistoricalFundingEntry
	oiHistory    []marketdata.HistoricalOIEntry

	// Pre-extracted funding/OI timestamps for binary search
	tFunding []int64
	tOI      []int64
}

// NewContextBuilder creates a ContextBuilder backed by the given dataset.
// All candle arrays are pre-converted once here so BuildAt is allocation-free.
func NewContextBuilder(ds marketdata.MTFDataset) *ContextBuilder {
	cb := &ContextBuilder{
		ds:           ds,
		fundingRates: ds.FundingRates,
		oiHistory:    ds.OIHistory,
	}

	cb.sc1m = convertCandles(ds.Candles1m)
	cb.sc5m = convertCandles(ds.Candles5m)
	cb.sc15m = convertCandles(ds.Candles15m)
	cb.sc1h = convertCandles(ds.Candles1h)
	cb.sc4h = convertCandles(ds.Candles4h)

	cb.t1m = extractTimes(ds.Candles1m)
	cb.t5m = extractTimes(ds.Candles5m)
	cb.t15m = extractTimes(ds.Candles15m)
	cb.t1h = extractTimes(ds.Candles1h)
	cb.t4h = extractTimes(ds.Candles4h)

	cb.tFunding = make([]int64, len(ds.FundingRates))
	for i, f := range ds.FundingRates {
		cb.tFunding[i] = f.Time.Unix()
	}
	cb.tOI = make([]int64, len(ds.OIHistory))
	for i, o := range ds.OIHistory {
		cb.tOI[i] = o.Time.Unix()
	}

	return cb
}

// BuildAt constructs a MarketContext as-of bar index i on the 15m clock.
// Returns (ctx, true) when there is enough warmup data, (zero, false) otherwise.
func (c *ContextBuilder) BuildAt(i int, regime scalers.Regime) (scalers.MarketContext, bool) {
	if i < 60 || i >= len(c.sc15m) {
		return scalers.MarketContext{}, false
	}

	barTime := c.ds.Candles15m[i].OpenTime
	deadline := barTime.Add(15 * time.Minute)
	deadlineUnix := deadline.Unix()
	barUnix := barTime.Unix()

	price := c.sc15m[i].Close

	// Binary search cutoffs — O(log N) per timeframe
	n1m := upperBound(c.t1m, deadlineUnix)
	n5m := upperBound(c.t5m, deadlineUnix)
	n1h := upperBound(c.t1h, deadlineUnix)
	n4h := upperBound(c.t4h, deadlineUnix)

	// Direct slice of pre-converted arrays + tail trim — zero allocations beyond the slice header
	c1m := tailS(c.sc1m[:n1m], 100)
	c5m := tailS(c.sc5m[:n5m], 100)
	c15m := tailS(c.sc15m[:i], 60)
	c1h := tailS(c.sc1h[:n1h], 72)
	c4h := tailS(c.sc4h[:n4h], 30)

	// Funding rate — binary search
	fundingRate, fundingHistory := c.lookupFunding(barUnix)

	// Open interest — binary search
	oiCurrent, oiPrev, oiHistSlice := c.lookupOI(barUnix)

	// CVD — fast window scan using pre-indexed times
	cvd, cvdPrev := c.fastCVD(barTime)

	return scalers.MarketContext{
		Regime:           regime,
		Price:            price,
		Candles1m:        c1m,
		Candles5m:        c5m,
		Candles15m:       c15m,
		Candles1h:        c1h,
		Candles4h:        c4h,
		CVD:              cvd,
		CVDPrev:          cvdPrev,
		CVDHistory:       []float64{cvdPrev, cvd},
		FundingRate:      fundingRate,
		FundingHistory:   fundingHistory,
		OpenInterest:     oiCurrent,
		OpenInterestPrev: oiPrev,
		OIHistory:        oiHistSlice,
		Now:              barTime,

		MicrostructurePopulated:  false,
		LiquidationFeedPopulated: false,
		DVOLPopulated:            false,
		ETHPricePopulated:        false,
		MacroFeedPopulated:       false,
		PositioningFeedPopulated: false,
		FearGreedPopulated:       false,
	}, true
}

// ── helpers ──────────────────────────────────────────────────────────────────

// convertCandles converts a HistoricalCandle slice to scalers.Candle once.
func convertCandles(hc []marketdata.HistoricalCandle) []scalers.Candle {
	out := make([]scalers.Candle, len(hc))
	for i, c := range hc {
		out[i] = scalers.Candle{
			OpenTime: c.OpenTime,
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   c.Volume,
		}
	}
	return out
}

// extractTimes pre-extracts Unix timestamps for binary search.
func extractTimes(hc []marketdata.HistoricalCandle) []int64 {
	out := make([]int64, len(hc))
	for i, c := range hc {
		out[i] = c.OpenTime.Unix()
	}
	return out
}

// upperBound returns the number of elements with Unix timestamp < deadline.
func upperBound(times []int64, deadline int64) int {
	return sort.Search(len(times), func(i int) bool { return times[i] >= deadline })
}

func tailS(s []scalers.Candle, n int) []scalers.Candle {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func (c *ContextBuilder) lookupFunding(barUnix int64) (float64, []float64) {
	if len(c.tFunding) == 0 {
		return 0, nil
	}
	// last entry with timestamp <= barUnix
	idx := sort.Search(len(c.tFunding), func(i int) bool { return c.tFunding[i] > barUnix }) - 1
	if idx < 0 {
		return 0, nil
	}
	rate := c.fundingRates[idx].Rate
	history := make([]float64, 0, 3)
	for k := idx; k >= 0 && k > idx-3; k-- {
		history = append([]float64{c.fundingRates[k].Rate}, history...)
	}
	return rate, history
}

func (c *ContextBuilder) lookupOI(barUnix int64) (float64, float64, []float64) {
	if len(c.tOI) == 0 {
		return 0, 0, nil
	}
	idx := sort.Search(len(c.tOI), func(i int) bool { return c.tOI[i] > barUnix }) - 1
	if idx < 0 {
		return 0, 0, nil
	}
	cur := c.oiHistory[idx].OI
	prev := 0.0
	if idx > 0 {
		prev = c.oiHistory[idx-1].OI
	}
	hist := make([]float64, 0, 5)
	for k := idx; k >= 0 && k > idx-5; k-- {
		hist = append([]float64{c.oiHistory[k].OI}, hist...)
	}
	return cur, prev, hist
}

// fastCVD computes CVD using the pre-indexed 1m times — binary search to window start.
func (c *ContextBuilder) fastCVD(barTime time.Time) (cvd, cvdPrev float64) {
	const window = 60
	barUnix := barTime.Unix()
	end := sort.Search(len(c.t1m), func(i int) bool { return c.t1m[i] > barUnix })
	start := end - window - 1
	if start < 0 {
		start = 0
	}

	run := 0.0
	prev := 0.0
	for j := start; j < end; j++ {
		c1 := c.sc1m[j]
		var delta float64
		if c1.Close >= c1.Open {
			delta = c1.Volume
		} else {
			delta = -c1.Volume
		}
		run += delta
		if j == end-2 {
			prev = run
		}
	}
	return run, prev
}

// toScalerCandles kept for compatibility with any external callers.
func toScalerCandles(hc []marketdata.HistoricalCandle) []scalers.Candle {
	return convertCandles(hc)
}

func tail[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
