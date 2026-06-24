package scalpers

import (
	"fmt"
	"math"
)

// S89 — Historical Volatility Percentile Breakout
//
// Citation:   Euan Sinclair, "Volatility Trading" (2008). When realized
//             volatility is in the bottom 20th percentile of its history,
//             the subsequent expansion move tends to be significantly larger
//             than average. Validated in BTC by multiple crypto vol papers.
// Regime:     ALL regimes
// Timeframes: 1h (for vol percentile), 15m (for breakout entry)
// Logic:      BB width percentile rank (proxy for realized vol) on 1h candles
//             < 20th percentile → low-vol compression. Price then breaks the
//             10-bar high or low on 15m = high-probability expansion entry.

type HistoricalVolPercentileBreakout struct{}

func (s *HistoricalVolPercentileBreakout) Name() string {
	return "Historical_Vol_Percentile_Breakout"
}

func (s *HistoricalVolPercentileBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *HistoricalVolPercentileBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles1h) < 40 || len(ctx.Candles15m) < 15 {
		return NoSignal(name)
	}

	// BB width percentile on 1h — proxy for realized vol rank
	volPct := BBWidthPercentile(ctx.Candles1h, 20, 20)
	if volPct >= 0.20 {
		return NoSignal(name) // not in low-vol compression zone
	}

	// Breakout: 10-bar high/low on 15m
	candles15m := ctx.Candles15m
	n := len(candles15m)
	swing10High := SwingHigh(candles15m[:n-1], 10)
	swing10Low := SwingLow(candles15m[:n-1], 10)

	lastCandle := candles15m[n-1]
	price := ctx.Price

	atr := ATR(candles15m, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	bullishBreak := lastCandle.Close > swing10High
	bearishBreak := lastCandle.Close < swing10Low

	if bullishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := swing10Low - 0.1*atr
		if price-sl < minSL {
			sl = price - minSL
		}
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		if (tp-price)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VolPct LONG breakout: 1h vol pct=%.2f (<20th), 15m close=%.0f>10-bar high=%.0f",
				volPct, lastCandle.Close, swing10High,
			),
		}
	}

	if bearishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := swing10High + 0.1*atr
		if sl-price < minSL {
			sl = price + minSL
		}
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		if (price-tp)/slDist < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"VolPct SHORT breakout: 1h vol pct=%.2f (<20th), 15m close=%.0f<10-bar low=%.0f",
				volPct, lastCandle.Close, swing10Low,
			),
		}
	}

	return NoSignal(name)
}
