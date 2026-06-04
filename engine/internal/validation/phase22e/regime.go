package phase22e

import "math"

// ClassifyRegime assigns a Regime to each trade based on a simple
// price-action model using EMA slope + volatility ratio.
//
// Parameters:
//   prices  — daily close prices aligned to the trade window (len ≥ 20)
//   atrPct  — daily ATR as % of price (len ≥ 1 per bar)
//   tradeIdx — index into the price/atr slices for this trade
func ClassifyRegime(prices []float64, atrPct []float64, tradeIdx int) Regime {
	if len(prices) < 20 || tradeIdx < 19 {
		return RegimeRange
	}

	// 20-period EMA slope
	ema := ema20(prices[:tradeIdx+1])
	slopeN := len(ema)
	if slopeN < 2 {
		return RegimeRange
	}
	slope := (ema[slopeN-1] - ema[slopeN-2]) / ema[slopeN-2] * 100 // % daily change in EMA

	// volatility at this point
	vol := 0.0
	if tradeIdx < len(atrPct) {
		vol = atrPct[tradeIdx]
	}

	const (
		highVolThreshold = 2.5  // ATR% > 2.5% → volatile
		trendThreshold   = 0.15 // EMA slope % > 0.15 → trending
	)

	if vol > highVolThreshold {
		return RegimeVolatile
	}
	if slope > trendThreshold {
		return RegimeBull
	}
	if slope < -trendThreshold {
		return RegimeBear
	}
	return RegimeRange
}

// ClassifyRegimeSimple classifies regime from a raw return series.
// Used when per-bar price data is unavailable; operates on cumulative
// returns over a trailing window.
func ClassifyRegimeSimple(trailingReturns []float64) Regime {
	if len(trailingReturns) < 5 {
		return RegimeRange
	}
	cumReturn := 0.0
	for _, r := range trailingReturns {
		cumReturn += r
	}
	vol := stddev(trailingReturns)

	if vol > 0.025 {
		return RegimeVolatile
	}
	if cumReturn > 0.05 {
		return RegimeBull
	}
	if cumReturn < -0.05 {
		return RegimeBear
	}
	return RegimeRange
}

// AssignRegimes batch-classifies trades using their timestamp sequence.
// When no price series is available we use a look-back of the last 20
// trades' net returns as a volatility/trend proxy.
func AssignRegimes(trades []TradeRecord) []TradeRecord {
	out := make([]TradeRecord, len(trades))
	copy(out, trades)

	window := 20
	for i := range out {
		start := i - window
		if start < 0 {
			start = 0
		}
		returns := make([]float64, 0, i-start)
		for j := start; j < i; j++ {
			if trades[j].EntryPrice > 0 {
				r := (trades[j].ExitPrice - trades[j].EntryPrice) / trades[j].EntryPrice
				returns = append(returns, r)
			}
		}
		out[i].Regime = ClassifyRegimeSimple(returns)
	}
	return out
}

// ema20 computes the 20-period EMA of a price series.
func ema20(prices []float64) []float64 {
	if len(prices) < 20 {
		return nil
	}
	k := 2.0 / 21.0
	ema := make([]float64, len(prices))
	// seed with SMA of first 20
	sum := 0.0
	for _, p := range prices[:20] {
		sum += p
	}
	ema[19] = sum / 20.0
	for i := 20; i < len(prices); i++ {
		ema[i] = prices[i]*k + ema[i-1]*(1-k)
	}
	return ema[19:]
}

// RegimeSummary builds a human-readable table of regime performance.
func RegimeSummary(pm PortfolioMetrics) map[Regime]RegimePerformance {
	out := make(map[Regime]RegimePerformance, 4)
	for _, r := range []Regime{RegimeBull, RegimeBear, RegimeRange, RegimeVolatile} {
		if rp, ok := pm.RegimePerf[r]; ok {
			out[r] = *rp
		} else {
			out[r] = RegimePerformance{Regime: r}
		}
	}
	return out
}

// atr computes average true range as a % of close for a window.
func atr(highs, lows, closes []float64, period int) []float64 {
	if len(closes) < period+1 {
		return nil
	}
	trs := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		hl := highs[i] - lows[i]
		hpc := math.Abs(highs[i] - closes[i-1])
		lpc := math.Abs(lows[i] - closes[i-1])
		tr := hl
		if hpc > tr {
			tr = hpc
		}
		if lpc > tr {
			tr = lpc
		}
		trs[i-1] = tr / closes[i] * 100
	}
	out := make([]float64, 0, len(trs)-period+1)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += trs[i]
	}
	out = append(out, sum/float64(period))
	for i := period; i < len(trs); i++ {
		sum = (sum*float64(period-1) + trs[i]) / float64(period)
		out = append(out, sum)
	}
	return out
}
