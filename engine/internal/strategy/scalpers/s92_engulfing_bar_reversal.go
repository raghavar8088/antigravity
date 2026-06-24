package scalpers

import (
	"fmt"
	"math"
)

// S92 — Engulfing Bar Reversal
//
// Citation:   Steve Nison, "Japanese Candlestick Charting Techniques" (1991);
//             Marshall, Young & Rose (2006) "Do Candlestick Technical Trading
//             Strategies Work in Contemporary Markets?" — Journal of
//             International Financial Markets, Institutions and Money.
// Regime:     RANGING + TRENDING
// Timeframes: 15m
// Logic:      Bearish engulfing at 20-bar high + RSI>60 + CVD declining = SHORT.
//             Bullish engulfing at 20-bar low + RSI<40 + CVD rising = LONG.
//             Second bar body must completely engulf first bar body.

type EngulfingBarReversal struct{}

func (s *EngulfingBarReversal) Name() string { return "Engulfing_Bar_Reversal" }

func (s *EngulfingBarReversal) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeTrending}
}

func (s *EngulfingBarReversal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	bar1 := candles[n-2]
	bar2 := candles[n-1]

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	rsi := RSI(candles, 14)
	cvdRising := ctx.CVD > ctx.CVDPrev
	cvdFalling := ctx.CVD < ctx.CVDPrev

	swing20High := SwingHigh(candles[:n-1], 20)
	swing20Low := SwingLow(candles[:n-1], 20)
	price := ctx.Price

	// Body helpers (open/close regardless of direction)
	bar1BodyHigh := math.Max(bar1.Open, bar1.Close)
	bar1BodyLow := math.Min(bar1.Open, bar1.Close)
	bar2BodyHigh := math.Max(bar2.Open, bar2.Close)
	bar2BodyLow := math.Min(bar2.Open, bar2.Close)

	// Bullish engulfing: bar2 closes higher, body engulfs bar1 body
	bullishEngulf := bar2.Close > bar2.Open && // bar2 is bullish
		bar2BodyHigh > bar1BodyHigh && bar2BodyLow < bar1BodyLow // engulfs bar1 body

	// Bearish engulfing: bar2 closes lower, body engulfs bar1 body
	bearishEngulf := bar2.Close < bar2.Open && // bar2 is bearish
		bar2BodyHigh > bar1BodyHigh && bar2BodyLow < bar1BodyLow // engulfs bar1 body

	// Near support/resistance level (within 0.5% of swing high/low)
	nearSwingHigh := swing20High > 0 && math.Abs(price-swing20High)/swing20High < 0.005
	nearSwingLow := swing20Low > 0 && math.Abs(price-swing20Low)/swing20Low < 0.005

	if bullishEngulf && nearSwingLow && rsi < 40 && cvdRising {
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
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Bullish engulfing at support=%.0f: bar1=%.0f-%.0f, bar2=%.0f-%.0f, RSI=%.1f, CVD rising",
				swing20Low, bar1BodyLow, bar1BodyHigh, bar2BodyLow, bar2BodyHigh, rsi,
			),
		}
	}

	if bearishEngulf && nearSwingHigh && rsi > 60 && cvdFalling {
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
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Bearish engulfing at resistance=%.0f: bar1=%.0f-%.0f, bar2=%.0f-%.0f, RSI=%.1f, CVD falling",
				swing20High, bar1BodyLow, bar1BodyHigh, bar2BodyLow, bar2BodyHigh, rsi,
			),
		}
	}

	return NoSignal(name)
}
