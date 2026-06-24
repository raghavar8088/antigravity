package scalpers

import (
	"fmt"
	"math"
)

// S86 — Volatility Ratio Breakout
//
// Citation:   Jack Schwager, "Schwager on Futures: Technical Analysis" (1996).
//             Low volatility (current ATR / average ATR < 0.5) precedes large
//             directional moves. Validated extensively in liquid futures markets.
// Regime:     ALL regimes
// Timeframes: 15m
// Logic:      ATR ratio (current bar ATR / 20-period average ATR) < 0.5 for 3+
//             consecutive bars. Entry when price breaks 20-bar high (LONG) or
//             low (SHORT) with increasing volume.

type VolatilityRatioBreakout struct{}

func (s *VolatilityRatioBreakout) Name() string { return "Volatility_Ratio_Breakout" }

func (s *VolatilityRatioBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *VolatilityRatioBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 30 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	// Average ATR over last 20 bars (using last 21 candles for 20 TR values)
	avgATR := ATR(candles, 20)
	if avgATR == 0 {
		return NoSignal(name)
	}

	// Check ATR ratio < 0.5 for 3 consecutive bars ending at n-2 (bar before signal bar)
	compressionBars := 0
	for i := n - 4; i <= n-2; i++ {
		if i < 1 {
			break
		}
		barATR := candles[i].High - candles[i].Low // single-bar range as ATR proxy
		if barATR/avgATR < 0.5 {
			compressionBars++
		}
	}
	if compressionBars < 3 {
		return NoSignal(name)
	}

	// Breakout level: 20-bar high/low excluding the current candle
	swing20High := SwingHigh(candles[:n-1], 20)
	swing20Low := SwingLow(candles[:n-1], 20)

	lastCandle := candles[n-1]
	price := ctx.Price

	avgVol := AvgVolume(candles, 20)
	prevVol := candles[n-2].Volume
	curVol := lastCandle.Volume
	volIncreasing := avgVol > 0 && curVol > prevVol && curVol > 0.8*avgVol

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	bullishBreak := lastCandle.Close > swing20High && volIncreasing
	bearishBreak := lastCandle.Close < swing20Low && volIncreasing

	if bullishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := swing20Low - 0.1*atr
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
				"VolRatio breakout LONG: 3-bar ATR compression, close=%.0f>20-bar high=%.0f, vol increasing",
				lastCandle.Close, swing20High,
			),
		}
	}

	if bearishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := swing20High + 0.1*atr
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
				"VolRatio breakout SHORT: 3-bar ATR compression, close=%.0f<20-bar low=%.0f, vol increasing",
				lastCandle.Close, swing20Low,
			),
		}
	}

	return NoSignal(name)
}
