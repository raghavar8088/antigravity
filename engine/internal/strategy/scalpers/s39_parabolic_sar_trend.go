package scalpers

import (
	"fmt"
	"math"
)

// S39 — Parabolic SAR Trend
//
// Citation:   J. Welles Wilder, "New Concepts in Technical Trading
//             Systems" (1978) — the Parabolic SAR (stop-and-reverse) trails
//             price with an accelerating offset, flipping when price
//             crosses the SAR dot.
// Regime:     TRENDING only
// Timeframes: 15m (entry trigger) + 1h (trend alignment confirm)
// Logic:      Compute ParabolicSAR() on Candles15m. When SAR indicates an
//             uptrend (dots below price) and the 1h trend is aligned
//             (EMA9 > EMA21), go long; symmetric for short with SAR
//             downtrend + 1h EMA9 < EMA21.
// Edge:       SAR's accelerating trail adapts stop distance to trend
//             maturity; the 1h alignment filter avoids acting on 15m SAR
//             flips that contradict the dominant higher-timeframe trend.

type ParabolicSARTrend struct{}

func (s *ParabolicSARTrend) Name() string { return "Parabolic_SAR_Trend" }

func (s *ParabolicSARTrend) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *ParabolicSARTrend) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 10 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	sarValue, sarUptrend := ParabolicSAR(ctx.Candles15m)
	if sarValue == 0 {
		return NoSignal(name)
	}

	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	bull1h := ema9_1h > ema21_1h
	bear1h := ema9_1h < ema21_1h

	price := ctx.Price

	if sarUptrend && bull1h {
		sl := math.Min(sarValue, price-math.Max(1.0*atr15m, 0.003*price))
		slDist := price - sl
		if slDist < math.Max(1.0*atr15m, 0.003*price) {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Parabolic SAR uptrend (SAR=%.0f) + 1h bullish (EMA9=%.0f>EMA21=%.0f)",
				sarValue, ema9_1h, ema21_1h,
			),
		}
	}

	if !sarUptrend && bear1h {
		sl := math.Max(sarValue, price+math.Max(1.0*atr15m, 0.003*price))
		slDist := sl - price
		if slDist < math.Max(1.0*atr15m, 0.003*price) {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Parabolic SAR downtrend (SAR=%.0f) + 1h bearish (EMA9=%.0f<EMA21=%.0f)",
				sarValue, ema9_1h, ema21_1h,
			),
		}
	}

	return NoSignal(name)
}
