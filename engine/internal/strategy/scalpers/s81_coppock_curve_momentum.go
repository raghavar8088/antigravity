package scalpers

import (
	"fmt"
	"math"
)

// S81 — Coppock Curve Momentum
//
// Citation:   Edwin Coppock, "A Study in Stock Market Timing" (Barron's, 1962).
//             Originally a long-term monthly stock market bottom detector,
//             adapted for 4h BTC candles by multiple crypto quant researchers.
// Regime:     TRENDING + RANGING
// Timeframes: 4h
// Logic:      Coppock = WMA(ROC(14) + ROC(11), 10). Cross from negative to
//             positive = LONG momentum. Cross from positive to negative = SHORT.
// Edge:       The WMA smoothing of a dual-ROC sum reduces false crossovers
//             compared to a single momentum indicator.

type CoppockCurveMomentum struct{}

func (s *CoppockCurveMomentum) Name() string { return "Coppock_Curve_Momentum" }

func (s *CoppockCurveMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *CoppockCurveMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles4h) < 28 {
		return NoSignal(name)
	}

	candles := ctx.Candles4h
	cc := CoppockCurve(candles, 14, 11, 10)
	ccPrev := CoppockCurve(candles[:len(candles)-1], 14, 11, 10)

	if cc == 0 && ccPrev == 0 {
		return NoSignal(name)
	}

	atr := ATR(ctx.Candles4h, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	price := ctx.Price

	// Bullish cross: was negative, now positive
	bullishCross := ccPrev <= 0 && cc > 0
	// Bearish cross: was positive, now negative
	bearishCross := ccPrev >= 0 && cc < 0

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
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Coppock bullish cross (%.3f → %.3f), 4h momentum turning positive",
				ccPrev, cc,
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
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Coppock bearish cross (%.3f → %.3f), 4h momentum turning negative",
				ccPrev, cc,
			),
		}
	}

	return NoSignal(name)
}
