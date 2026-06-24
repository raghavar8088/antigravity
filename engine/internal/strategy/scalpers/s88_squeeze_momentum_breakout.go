package scalpers

import (
	"fmt"
	"math"
)

// S88 — Squeeze Momentum Breakout
//
// Citation:   John Carter, "Mastering the Trade" (2006). Combines Bollinger
//             Bands inside Keltner Channels to identify coiled markets ready
//             for large moves. Widely used by institutional crypto traders.
// Regime:     ALL regimes
// Timeframes: 15m
// Logic:      BB fully inside Keltner Channel (squeeze ON) for 3+ bars, then
//             BB expands outside Keltner (squeeze fires). Momentum histogram
//             positive = LONG, negative = SHORT.

type SqueezeMomentumBreakout struct{}

func (s *SqueezeMomentumBreakout) Name() string { return "Squeeze_Momentum_Breakout" }

func (s *SqueezeMomentumBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *SqueezeMomentumBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	sq := SqueezeDetector(candles)
	if !sq.Fired {
		return NoSignal(name)
	}

	// Verify squeeze was active for at least 3 bars before firing
	squeezeBars := 0
	for i := n - 4; i <= n-2; i++ {
		if i < 0 {
			break
		}
		sqCheck := SqueezeDetector(candles[:i+1])
		if sqCheck.Active {
			squeezeBars++
		}
	}
	if squeezeBars < 3 {
		return NoSignal(name)
	}

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	if sq.Momentum > 0 {
		minSL := math.Max(1.0*atr, 0.003*price)
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
				"Squeeze LONG fired after %d squeeze bars, momentum=%.2f positive",
				squeezeBars, sq.Momentum,
			),
		}
	}

	if sq.Momentum < 0 {
		minSL := math.Max(1.0*atr, 0.003*price)
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
				"Squeeze SHORT fired after %d squeeze bars, momentum=%.2f negative",
				squeezeBars, sq.Momentum,
			),
		}
	}

	return NoSignal(name)
}
