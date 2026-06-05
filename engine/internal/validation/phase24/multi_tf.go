package phase24

import (
	"math"

	"antigravity-engine/internal/validation/phase23b"
)

// aggregateCandles reduces a 1m candle slice to a coarser timeframe by
// grouping every n candles into one OHLCV bar.
func aggregateCandles(candles []phase23b.OHLCVCandle, n int) []phase23b.OHLCVCandle {
	if n <= 1 || len(candles) == 0 {
		return candles
	}
	out := make([]phase23b.OHLCVCandle, 0, len(candles)/n)
	for i := 0; i+n <= len(candles); i += n {
		group := candles[i : i+n]
		agg := phase23b.OHLCVCandle{
			Symbol:    group[0].Symbol,
			Open:      group[0].Open,
			High:      group[0].High,
			Low:       group[0].Low,
			Close:     group[n-1].Close,
			OpenTime:  group[0].OpenTime,
			CloseTime: group[n-1].CloseTime,
		}
		for _, c := range group {
			agg.Volume += c.Volume
			agg.QuoteVolume += c.QuoteVolume
			agg.Trades += c.Trades
			if c.High > agg.High {
				agg.High = c.High
			}
			if c.Low < agg.Low {
				agg.Low = c.Low
			}
			// Carry last non-zero funding/OI
			if c.FundingRate != 0 {
				agg.FundingRate = c.FundingRate
			}
			if c.OpenInterest > 0 {
				agg.OpenInterest = c.OpenInterest
			}
			agg.LiquidationUSD += c.LiquidationUSD
		}
		// Weighted average taker buy pct
		totalVol := 0.0
		takerBuyVol := 0.0
		for _, c := range group {
			totalVol += c.Volume
			takerBuyVol += c.Volume * c.TakerBuyPct
		}
		if totalVol > 0 {
			agg.TakerBuyPct = takerBuyVol / totalVol
		}
		out = append(out, agg)
	}
	return out
}

// tfMultiplier returns the number of 1m candles per bar for a given timeframe.
func tfMultiplier(tf Timeframe) int {
	switch tf {
	case TF3m:
		return 3
	case TF5m:
		return 5
	case TF15m:
		return 15
	default:
		return 1
	}
}

// candlesForTimeframe returns candles at the requested timeframe.
func candlesForTimeframe(base []phase23b.OHLCVCandle, tf Timeframe) []phase23b.OHLCVCandle {
	return aggregateCandles(base, tfMultiplier(tf))
}

// ── MaxHoldMins per timeframe ─────────────────────────────────────────────────

func maxHoldMinsFor(tf Timeframe) int {
	switch tf {
	case TF1m:
		return 1440 // 24h
	case TF3m:
		return 480 // 24h / 3
	case TF5m:
		return 288 // 24h / 5
	case TF15m:
		return 96 // 24h / 15
	default:
		return 1440
	}
}

// ── Ulcer Index ───────────────────────────────────────────────────────────────

// ulcerIndex computes the Ulcer Index from a sequence of equity values.
// UI = sqrt(mean(DD²)) where DD is the percentage drawdown at each point.
func ulcerIndex(equity []float64) float64 {
	if len(equity) < 2 {
		return 0
	}
	peak := equity[0]
	sumSqDD := 0.0
	for _, e := range equity {
		if e > peak {
			peak = e
		}
		if peak > 0 {
			dd := (peak - e) / peak * 100
			sumSqDD += dd * dd
		}
	}
	return math.Sqrt(sumSqDD / float64(len(equity)))
}

// ── Recovery Factor ───────────────────────────────────────────────────────────

// recoveryFactor = net profit / max drawdown (in USD).
func recoveryFactor(netProfitUSD, maxDDUSD float64) float64 {
	if maxDDUSD == 0 {
		return 0
	}
	return netProfitUSD / maxDDUSD
}

// ── Calmar Ratio ──────────────────────────────────────────────────────────────

// calmar = CAGR / MaxDD (percentage).
func calmar(cagrPct, maxDDPct float64) float64 {
	if maxDDPct == 0 {
		return 0
	}
	return cagrPct / maxDDPct
}

// ── Consecutive wins/losses ───────────────────────────────────────────────────

func maxConsecutive(trades []phase23b.CertifiedTrade) (wins, losses int) {
	cur := 0
	maxW, maxL := 0, 0
	for _, t := range trades {
		if t.NetPnLUSD > 0 {
			if cur > 0 {
				cur++
			} else {
				cur = 1
			}
			if cur > maxW {
				maxW = cur
			}
		} else {
			if cur < 0 {
				cur--
			} else {
				cur = -1
			}
			if -cur > maxL {
				maxL = -cur
			}
		}
	}
	return maxW, maxL
}
