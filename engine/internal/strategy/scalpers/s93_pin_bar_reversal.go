package scalpers

import (
	"fmt"
	"math"
)

// S93 — Pin Bar Reversal
//
// Citation:   Steve Nison, "Japanese Candlestick Charting Techniques" (1991)
//             (hammer/shooting star). Crypto validation: multiple academic
//             papers 2018-2023 confirm wick rejection at swing levels is a
//             statistically significant reversal signal in BTC markets.
// Regime:     RANGING + TRENDING
// Timeframes: 15m
// Logic:      Pin bar with wick >2.5× body size, body in upper/lower 30% of
//             candle range, at a significant swing level (within 0.3% of
//             20-bar SwingHigh or SwingLow), confirmed by volume >1.5× average
//             and CVD opposing the wick direction.

type PinBarReversal struct{}

func (s *PinBarReversal) Name() string { return "Pin_Bar_Reversal" }

func (s *PinBarReversal) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}

func (s *PinBarReversal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)
	pin := candles[n-1]

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	totalRange := pin.High - pin.Low
	if totalRange == 0 {
		return NoSignal(name)
	}

	bodyHigh := math.Max(pin.Open, pin.Close)
	bodyLow := math.Min(pin.Open, pin.Close)
	bodySize := bodyHigh - bodyLow

	// Body must be at least a minimal size to avoid doji confusion
	if bodySize < 0.05*totalRange {
		return NoSignal(name)
	}

	upperWick := pin.High - bodyHigh
	lowerWick := bodyLow - pin.Low

	// Swing levels
	swing20High := SwingHigh(candles[:n-1], 20)
	swing20Low := SwingLow(candles[:n-1], 20)
	price := ctx.Price

	avgVol := AvgVolume(candles, 20)
	volConfirm := avgVol > 0 && pin.Volume >= 1.5*avgVol

	cvdRising := ctx.CVD > ctx.CVDPrev
	cvdFalling := ctx.CVD < ctx.CVDPrev

	// Bullish pin bar (hammer): long lower wick, small body in upper 30%
	// Body in upper 30%: bodyLow >= pin.Low + 0.70*totalRange
	lowerWickDominant := lowerWick >= 2.5*bodySize
	bodyInUpperThird := bodyLow >= pin.Low+0.70*totalRange
	nearSwingLow := swing20Low > 0 && math.Abs(price-swing20Low)/swing20Low < 0.003

	if lowerWickDominant && bodyInUpperThird && nearSwingLow && volConfirm && cvdRising {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := pin.Low - 0.2*atr
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
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Bullish pin bar at support=%.0f: wick=%.1f, body=%.1f (%.1fx), vol=%.1fx avg, CVD rising",
				swing20Low, lowerWick, bodySize, lowerWick/bodySize, pin.Volume/avgVol,
			),
		}
	}

	// Bearish pin bar (shooting star): long upper wick, small body in lower 30%
	upperWickDominant := upperWick >= 2.5*bodySize
	bodyInLowerThird := bodyHigh <= pin.Low+0.30*totalRange
	nearSwingHigh := swing20High > 0 && math.Abs(price-swing20High)/swing20High < 0.003

	if upperWickDominant && bodyInLowerThird && nearSwingHigh && volConfirm && cvdFalling {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := pin.High + 0.2*atr
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
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Bearish pin bar at resistance=%.0f: wick=%.1f, body=%.1f (%.1fx), vol=%.1fx avg, CVD falling",
				swing20High, upperWick, bodySize, upperWick/bodySize, pin.Volume/avgVol,
			),
		}
	}

	return NoSignal(name)
}
