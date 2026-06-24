package scalpers

import (
	"fmt"
	"math"
)

// S90 — Three Bar Reversal
//
// Citation:   Thomas Bulkowski, "Encyclopedia of Chart Patterns" (2000).
//             Three-bar reversal: two consecutive same-direction bars followed
//             by a reversal bar closing beyond the second bar's extreme.
//             Validated extensively in futures markets for exhaustion entries.
// Regime:     RANGING (reversals work best in non-trending markets)
// Timeframes: 15m
// Logic:      Bullish: two consecutive down bars, third closes above bar-2 high.
//             RSI < 30 on third bar confirms exhaustion. Bearish: mirrored.

type ThreeBarReversal struct{}

func (s *ThreeBarReversal) Name() string { return "Three_Bar_Reversal" }

func (s *ThreeBarReversal) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *ThreeBarReversal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	bar1 := candles[n-3]
	bar2 := candles[n-2]
	bar3 := candles[n-1]

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	rsi := RSI(candles, 14)
	price := ctx.Price

	// Bullish three-bar reversal: down, down, close above bar2.High
	bar1Down := bar1.Close < bar1.Open
	bar2Down := bar2.Close < bar2.Open
	bar3BullClose := bar3.Close > bar2.High

	if bar1Down && bar2Down && bar3BullClose && rsi < 35 {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := bar2.Low - 0.2*atr
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
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"3-bar bullish reversal: down-down-close above bar2 high=%.0f, RSI=%.1f (<35)",
				bar2.High, rsi,
			),
		}
	}

	// Bearish three-bar reversal: up, up, close below bar2.Low
	bar1Up := bar1.Close > bar1.Open
	bar2Up := bar2.Close > bar2.Open
	bar3BearClose := bar3.Close < bar2.Low

	if bar1Up && bar2Up && bar3BearClose && rsi > 65 {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := bar2.High + 0.2*atr
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
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"3-bar bearish reversal: up-up-close below bar2 low=%.0f, RSI=%.1f (>65)",
				bar2.Low, rsi,
			),
		}
	}

	return NoSignal(name)
}
