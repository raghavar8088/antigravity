package scalpers

import (
	"fmt"
	"math"
)

// S97 — Chaikin Money Flow Signal
//
// Citation:   Marc Chaikin, Chaikin Analytics (1980s). CMF measures buying
//             and selling pressure by combining price position within the
//             bar's range with volume. Validated in multiple academic studies
//             as a leading indicator of institutional money flow direction.
// Regime:     ALL regimes
// Timeframes: 1h
// Logic:      CMF(21) crosses above +0.05 from negative territory AND price
//             above EMA20 = LONG momentum. CMF crosses below -0.05 from
//             positive AND price below EMA20 = SHORT.

type ChaikinMoneyFlowSignal struct{}

func (s *ChaikinMoneyFlowSignal) Name() string { return "Chaikin_Money_Flow_Signal" }

func (s *ChaikinMoneyFlowSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *ChaikinMoneyFlowSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles1h) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles1h
	n := len(candles)

	cmf := ChaikinMoneyFlow(candles, 21)
	cmfPrev := ChaikinMoneyFlow(candles[:n-1], 21)

	ema20 := EMA(candles, 20)
	if ema20 == 0 {
		return NoSignal(name)
	}

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	priceAboveEMA := price > ema20
	priceBelowEMA := price < ema20

	// Bullish: CMF crosses above +0.05 from below, price above EMA20
	bullishCross := cmfPrev < 0.05 && cmf >= 0.05 && priceAboveEMA
	// Bearish: CMF crosses below -0.05 from above, price below EMA20
	bearishCross := cmfPrev > -0.05 && cmf <= -0.05 && priceBelowEMA

	if bullishCross {
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
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CMF bullish cross: %.3f→%.3f (>+0.05), price=%.0f above EMA20=%.0f",
				cmfPrev, cmf, price, ema20,
			),
		}
	}

	if bearishCross {
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
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"CMF bearish cross: %.3f→%.3f (<-0.05), price=%.0f below EMA20=%.0f",
				cmfPrev, cmf, price, ema20,
			),
		}
	}

	return NoSignal(name)
}
