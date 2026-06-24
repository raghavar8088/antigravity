package scalpers

import (
	"fmt"
	"math"
)

// S33 — Adaptive Moving Average Trend
//
// Citation:   Perry Kaufman, "Smarter Trading" (1995) — Kaufman Adaptive
//             Moving Average (KAMA), which speeds up in trending markets
//             (high efficiency ratio) and slows down in choppy markets.
// Regime:     TRENDING only
// Timeframes: 15m
// Logic:      Compute KAMA() on Candles15m. Trade only when the efficiency
//             ratio exceeds 0.6 (strongly trending, low noise) and the KAMA
//             slope (current KAMA vs. KAMA computed 1 bar earlier) is
//             clearly directional.
// Edge:       The efficiency-ratio gate avoids whipsaws in choppy/ranging
//             conditions that defeat fixed-period moving averages.

type AdaptiveMovingAverageTrend struct{}

func (s *AdaptiveMovingAverageTrend) Name() string { return "Adaptive_Moving_Average_Trend" }

func (s *AdaptiveMovingAverageTrend) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *AdaptiveMovingAverageTrend) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 24 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	kama, er := KAMA(candles, 20)
	kamaPrev, _ := KAMA(candles[:len(candles)-1], 20)

	if kama == 0 || kamaPrev == 0 {
		return NoSignal(name)
	}
	if er <= 0.6 {
		return NoSignal(name)
	}

	slope := kama - kamaPrev
	price := ctx.Price

	// Directional threshold: slope must exceed a small fraction of ATR to be
	// considered "clearly directional" rather than noise.
	minSlope := 0.05 * atr15m

	if slope > minSlope {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"KAMA trending up (ER=%.2f, KAMA=%.0f, prev=%.0f, slope=%.2f)",
				er, kama, kamaPrev, slope,
			),
		}
	}

	if slope < -minSlope {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"KAMA trending down (ER=%.2f, KAMA=%.0f, prev=%.0f, slope=%.2f)",
				er, kama, kamaPrev, slope,
			),
		}
	}

	return NoSignal(name)
}
