package backtest

import (
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

	// pre-indexed funding/OI sorted slices
	fundingRates []marketdata.HistoricalFundingEntry
	oiHistory    []marketdata.HistoricalOIEntry
}

// NewContextBuilder creates a ContextBuilder backed by the given dataset.
func NewContextBuilder(ds marketdata.MTFDataset) *ContextBuilder {
	return &ContextBuilder{
		ds:           ds,
		fundingRates: ds.FundingRates,
		oiHistory:    ds.OIHistory,
	}
}

// BuildAt constructs a MarketContext as-of bar index i on the 15m clock.
// Returns (ctx, true) when there is enough warmup data, (zero, false) otherwise.
// Warmup: at least 60 bars of 15m data are required (bar index ≥ 60).
func (c *ContextBuilder) BuildAt(i int, regime scalers.Regime) (scalers.MarketContext, bool) {
	candles15m := c.ds.Candles15m
	if i < 60 || i >= len(candles15m) {
		return scalers.MarketContext{}, false
	}

	// The bar that just closed: candles15m[i]. Its OpenTime is the boundary.
	// We may include bars 0..i-1 (all bars whose OpenTime < candles15m[i].OpenTime + 15min).
	// Slicing to [:i] gives bars 0..i-1 which all opened before bar i closed.
	barTime := candles15m[i].OpenTime // bar[i] open; bar[i] close = barTime + 15m

	price := candles15m[i].Close

	c15m := toScalerCandles(c.ds.Candles15m[:i])

	// Other timeframes: include all bars whose OpenTime < barTime + 15m
	deadline := barTime.Add(15 * time.Minute)
	c1m := toScalerCandlesBefore(c.ds.Candles1m, deadline)
	c5m := toScalerCandlesBefore(c.ds.Candles5m, deadline)
	c1h := toScalerCandlesBefore(c.ds.Candles1h, deadline)
	c4h := toScalerCandlesBefore(c.ds.Candles4h, deadline)

	// Trim to reasonable window sizes (mirrors live ScalerBundle constants)
	c1m = tail(c1m, 100)
	c5m = tail(c5m, 100)
	c15m = tail(c15m, 60)
	c1h = tail(c1h, 72)
	c4h = tail(c4h, 30)

	// Funding rate: most recent entry at or before barTime
	fundingRate := 0.0
	fundingHistory := []float64{}
	for j := len(c.fundingRates) - 1; j >= 0; j-- {
		if !c.fundingRates[j].Time.After(barTime) {
			fundingRate = c.fundingRates[j].Rate
			// last 3 readings
			for k := j; k >= 0 && k > j-3; k-- {
				fundingHistory = append([]float64{c.fundingRates[k].Rate}, fundingHistory...)
			}
			break
		}
	}

	// Open interest: most recent at or before barTime
	oiCurrent := 0.0
	oiPrev := 0.0
	oiHistSlice := []float64{}
	for j := len(c.oiHistory) - 1; j >= 0; j-- {
		if !c.oiHistory[j].Time.After(barTime) {
			oiCurrent = c.oiHistory[j].OI
			if j > 0 {
				oiPrev = c.oiHistory[j-1].OI
			}
			// last 5 readings
			for k := j; k >= 0 && k > j-5; k-- {
				oiHistSlice = append([]float64{c.oiHistory[k].OI}, oiHistSlice...)
			}
			break
		}
	}

	// CVD: approximate from 1m candle close direction × volume (simplified buy-sell proxy)
	cvd, cvdPrev := approximateCVD(c.ds.Candles1m, barTime)

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

		// Microstructure feeds not available in historical data — strategies guard with populated flags.
		MicrostructurePopulated: false,
		LiquidationFeedPopulated: false,
		DVOLPopulated:           false,
		ETHPricePopulated:       false,
		MacroFeedPopulated:      false,
		PositioningFeedPopulated: false,
		FearGreedPopulated:      false,
	}, true
}

// ── helpers ──────────────────────────────────────────────────────────────────

func toScalerCandles(hc []marketdata.HistoricalCandle) []scalers.Candle {
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

func toScalerCandlesBefore(hc []marketdata.HistoricalCandle, before time.Time) []scalers.Candle {
	out := make([]scalers.Candle, 0, len(hc))
	for _, c := range hc {
		if c.OpenTime.Before(before) {
			out = append(out, scalers.Candle{
				OpenTime: c.OpenTime,
				Open:     c.Open,
				High:     c.High,
				Low:      c.Low,
				Close:    c.Close,
				Volume:   c.Volume,
			})
		}
	}
	return out
}

func tail[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// approximateCVD builds a volume-weighted buy/sell delta proxy from 1m candles
// ending at or before barTime. Returns (cvd, cvdPrev) where cvdPrev is the
// value from the preceding bar.
func approximateCVD(hc []marketdata.HistoricalCandle, barTime time.Time) (cvd, cvdPrev float64) {
	const window = 60 // last 60 1m bars
	var eligible []marketdata.HistoricalCandle
	for _, c := range hc {
		if !c.OpenTime.After(barTime) {
			eligible = append(eligible, c)
		}
	}
	start := 0
	if len(eligible) > window+1 {
		start = len(eligible) - window - 1
	}
	eligible = eligible[start:]

	runCVD := 0.0
	prevCVD := 0.0
	for j, c := range eligible {
		var delta float64
		if c.Close >= c.Open {
			delta = c.Volume // approximate: all volume = buy
		} else {
			delta = -c.Volume
		}
		runCVD += delta
		if j == len(eligible)-2 {
			prevCVD = runCVD
		}
	}
	return runCVD, prevCVD
}
