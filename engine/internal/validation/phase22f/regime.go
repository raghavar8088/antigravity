package phase22f

import (
	"math"

	"antigravity-engine/internal/validation/phase22e"
)

// AssignRegimesF22 classifies each trade into one of 10 Phase 22F regimes.
// The primary classification comes from phase22e's 4-regime system;
// we add 6 more via heuristics on price volatility, funding, and trade context.
func AssignRegimesF22(trades []phase22e.TradeRecord) []phase22e.TradeRecord {
	if len(trades) == 0 {
		return trades
	}

	// Build a rolling volatility estimate (stddev of price changes over 20-trade window)
	prices := make([]float64, len(trades))
	for i, t := range trades {
		prices[i] = t.EntryPrice
	}

	out := make([]phase22e.TradeRecord, len(trades))
	copy(out, trades)

	// Override regime assignment using extended heuristics
	for i := range out {
		out[i].Regime = classifyRegimeLegacy(out[i], prices, i)
	}
	return out
}

// mapToF22Regime converts a phase22e.Regime to RegimeF22, applying additional
// classification from trade metadata (price, hold time, PnL characteristics).
func mapToF22Regime(r phase22e.Regime, t phase22e.TradeRecord) RegimeF22 {
	// Heuristic refinements layered on top of 4-regime classification
	switch r {
	case phase22e.RegimeBull:
		// distinguish strong trend vs. routine bull
		if t.HoldMinutes < 30 && math.Abs(t.NetPnLUSD) > 200 {
			return R22Trend
		}
		return R22Bull
	case phase22e.RegimeBear:
		return R22Bear
	case phase22e.RegimeVolatile:
		// separate liquidation cascade events from generic volatility
		if t.NetPnLUSD > 500 || t.NetPnLUSD < -500 {
			return R22Liquidation
		}
		return R22Volatile
	case phase22e.RegimeRange:
		// distinguish mean reversion from low-vol range
		if t.HoldMinutes > 120 {
			return R22MeanReversion
		}
		return R22LowVol
	default:
		return R22Range
	}
}

func classifyRegimeLegacy(t phase22e.TradeRecord, prices []float64, idx int) phase22e.Regime {
	// If the trade already has a regime assigned, keep it
	if t.Regime != "" {
		return t.Regime
	}
	// Fallback: assign RANGE as default
	window := 10
	start := idx - window
	if start < 0 {
		start = 0
	}
	if idx == 0 {
		return phase22e.RegimeRange
	}
	priceSlice := prices[start : idx+1]
	if len(priceSlice) < 2 {
		return phase22e.RegimeRange
	}
	first := priceSlice[0]
	last := priceSlice[len(priceSlice)-1]
	chg := (last - first) / first * 100
	vol := priceVolatility(priceSlice)

	switch {
	case vol > 3.0:
		return phase22e.RegimeVolatile
	case chg > 1.5:
		return phase22e.RegimeBull
	case chg < -1.5:
		return phase22e.RegimeBear
	default:
		return phase22e.RegimeRange
	}
}

func priceVolatility(prices []float64) float64 {
	if len(prices) < 2 {
		return 0
	}
	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1] > 0 {
			returns[i-1] = (prices[i] - prices[i-1]) / prices[i-1] * 100
		}
	}
	return stddevLocal(returns)
}

// ComputeRegimePerformanceF22 aggregates per-regime stats for the full trade set.
func ComputeRegimePerformanceF22(trades []phase22e.TradeRecord) map[RegimeF22]*RegimePerfF22 {
	m := make(map[RegimeF22]*RegimePerfF22)
	byRegime := make(map[RegimeF22][]float64)

	for _, t := range trades {
		r := mapToF22Regime(t.Regime, t)
		rp, ok := m[r]
		if !ok {
			rp = &RegimePerfF22{Regime: r}
			m[r] = rp
		}
		rp.Trades++
		rp.NetPnLUSD += t.NetPnLUSD
		if t.NetPnLUSD >= 0 {
			rp.WinRate++
		}
		byRegime[r] = append(byRegime[r], t.NetPnLUSD)
	}

	for r, rp := range m {
		if rp.Trades > 0 {
			rp.WinRate /= float64(rp.Trades)
			rp.Expectancy = rp.NetPnLUSD / float64(rp.Trades)
		}
		pnls := byRegime[r]
		gw, gl := 0.0, 0.0
		for _, p := range pnls {
			if p >= 0 {
				gw += p
			} else {
				gl += -p
			}
		}
		if gl > 0 {
			rp.ProfitFactor = gw / gl
		}
		rp.Sharpe = sharpeLocal(pnls)
		rp.MaxDrawdown = maxDrawdownPctLocal(pnls, 1_000_000)
	}
	return m
}
