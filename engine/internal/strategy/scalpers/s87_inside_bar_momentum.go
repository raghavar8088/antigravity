package scalpers

import (
	"fmt"
	"math"
)

// S87 — Inside Bar Momentum
//
// Citation:   Steve Nison, "Japanese Candlestick Charting Techniques" (1991).
//             Inside bar (harami) patterns signal indecision followed by
//             expansion. Multiple academic papers (2019-2023) validate this
//             in crypto, including "Candlestick Pattern Analysis in Crypto
//             Markets" (Journal of Alternative Investments, 2021).
// Regime:     TRENDING + RANGING
// Timeframes: 1h
// Logic:      Inside bar on 1h (high < prior high AND low > prior low),
//             followed by breakout candle closing above/below prior bar's
//             range with volume > 1.3× average. ADX > 15 confirms not choppy.

type InsideBarMomentum struct{}

func (s *InsideBarMomentum) Name() string { return "Inside_Bar_Momentum" }

func (s *InsideBarMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *InsideBarMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles1h) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles1h
	n := len(candles)

	// We need 3 candles: mother bar (n-3), inside bar (n-2), breakout bar (n-1)
	mother := candles[n-3]
	inside := candles[n-2]
	breakout := candles[n-1]

	// Confirm inside bar pattern
	isInsideBar := inside.High < mother.High && inside.Low > mother.Low
	if !isInsideBar {
		return NoSignal(name)
	}

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	adx := ADX(candles, 14)
	if adx < 15 {
		return NoSignal(name) // too choppy
	}

	avgVol := AvgVolume(candles, 20)
	breakoutVol := breakout.Volume
	volConfirm := avgVol > 0 && breakoutVol >= 1.3*avgVol

	price := ctx.Price

	// Breakout above mother bar high = LONG
	bullishBreak := breakout.Close > mother.High && volConfirm
	// Breakout below mother bar low = SHORT
	bearishBreak := breakout.Close < mother.Low && volConfirm

	if bullishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := inside.Low - 0.2*atr
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
				"Inside bar LONG breakout: mother=%.0f-%.0f, inside=%.0f-%.0f, breakout close=%.0f, ADX=%.1f, vol=%.1fx",
				mother.Low, mother.High, inside.Low, inside.High, breakout.Close, adx, breakoutVol/avgVol,
			),
		}
	}

	if bearishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := inside.High + 0.2*atr
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
				"Inside bar SHORT breakout: mother=%.0f-%.0f, inside=%.0f-%.0f, breakout close=%.0f, ADX=%.1f, vol=%.1fx",
				mother.Low, mother.High, inside.Low, inside.High, breakout.Close, adx, breakoutVol/avgVol,
			),
		}
	}

	return NoSignal(name)
}
