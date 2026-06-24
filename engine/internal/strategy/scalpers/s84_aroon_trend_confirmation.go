package scalpers

import (
	"fmt"
	"math"
)

// S84 — Aroon Trend Confirmation
//
// Citation:   Tushar Chande, "Stocks & Commodities" (September 1995).
//             Aroon measures how recently price made a new high or low within a
//             lookback period. High Aroon Up + low Aroon Down = strong uptrend.
// Regime:     TRENDING
// Timeframes: 1h
// Logic:      AroonUp > 70 AND AroonDown < 30 for 2+ consecutive bars = strong
//             uptrend entry. AroonDown > 70 AND AroonUp < 30 for 2+ bars = short.

type AroonTrendConfirmation struct{}

func (s *AroonTrendConfirmation) Name() string { return "Aroon_Trend_Confirmation" }

func (s *AroonTrendConfirmation) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *AroonTrendConfirmation) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 20 {
		return NoSignal(name)
	}

	candles := ctx.Candles1h
	aroon := Aroon(candles, 14)
	aroonPrev := Aroon(candles[:len(candles)-1], 14)

	atr1h := ATR(candles, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// Strong uptrend: Aroon Up > 70 AND Aroon Down < 30 for 2 consecutive bars
	strongBull := aroon.Up > 70 && aroon.Down < 30 && aroonPrev.Up > 70 && aroonPrev.Down < 30
	// Strong downtrend: Aroon Down > 70 AND Aroon Up < 30 for 2 consecutive bars
	strongBear := aroon.Down > 70 && aroon.Up < 30 && aroonPrev.Down > 70 && aroonPrev.Up < 30

	if strongBull {
		minSL := math.Max(1.0*atr1h, 0.003*price)
		sl := price - minSL
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
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Aroon strong uptrend 2-bar: Up=%.0f Down=%.0f (prev Up=%.0f Down=%.0f)",
				aroon.Up, aroon.Down, aroonPrev.Up, aroonPrev.Down,
			),
		}
	}

	if strongBear {
		minSL := math.Max(1.0*atr1h, 0.003*price)
		sl := price + minSL
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
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Aroon strong downtrend 2-bar: Down=%.0f Up=%.0f (prev Down=%.0f Up=%.0f)",
				aroon.Down, aroon.Up, aroonPrev.Down, aroonPrev.Up,
			),
		}
	}

	return NoSignal(name)
}
