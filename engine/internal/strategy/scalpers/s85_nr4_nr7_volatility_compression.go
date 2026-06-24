package scalpers

import (
	"fmt"
	"math"
)

// S85 — NR4/NR7 Volatility Compression
//
// Citation:   Toby Crabel, "Day Trading With Short Term Price Patterns and
//             Opening Range Breakout" (1990). NR7 (today's range narrower than
//             previous 7) predicts volatility expansion breakouts in futures.
// Regime:     ALL (volatility expansion can occur in any regime)
// Timeframes: 15m
// Logic:      NR7 pattern fires when current candle has the narrowest range of
//             the last 7. Entry when the NEXT candle breaks above NR7 high
//             (LONG) or below NR7 low (SHORT) with volume confirmation.

type NR7VolatilityCompression struct{}

func (s *NR7VolatilityCompression) Name() string { return "NR7_Volatility_Compression" }

func (s *NR7VolatilityCompression) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging, RegimeVolatile}
}

func (s *NR7VolatilityCompression) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if len(ctx.Candles15m) < 10 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	n := len(candles)

	// Check NR7: the candle at index n-2 must have the narrowest range of the 7 candles ending at n-2
	// (i.e., candles[n-8 : n-1]), so we can use candle n-1 as the breakout candle.
	if n < 9 {
		return NoSignal(name)
	}

	// NR7 reference candle is candles[n-2]; breakout candle is candles[n-1]
	nr7Slice := candles[n-8 : n-1] // 7 candles ending with the NR7 candidate
	isNR7 := NarrowRangeN(nr7Slice, 7)
	if !isNR7 {
		return NoSignal(name)
	}

	nr7Candle := candles[n-2]
	breakoutCandle := candles[n-1]
	nr7High := nr7Candle.High
	nr7Low := nr7Candle.Low

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	avgVol := AvgVolume(candles, 20)
	breakoutVol := breakoutCandle.Volume
	volConfirm := avgVol > 0 && breakoutVol >= 1.2*avgVol

	price := ctx.Price

	bullishBreak := breakoutCandle.Close > nr7High && volConfirm
	bearishBreak := breakoutCandle.Close < nr7Low && volConfirm

	if bullishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := nr7Low - 0.2*atr
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
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"NR7 breakout LONG: NR7 range=%.1f, breakout close=%.0f>NR7 high=%.0f, vol=%.0f (%.1fx avg)",
				nr7High-nr7Low, breakoutCandle.Close, nr7High, breakoutVol, breakoutVol/avgVol,
			),
		}
	}

	if bearishBreak {
		minSL := math.Max(1.0*atr, 0.003*price)
		sl := nr7High + 0.2*atr
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
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"NR7 breakout SHORT: NR7 range=%.1f, breakout close=%.0f<NR7 low=%.0f, vol=%.0f (%.1fx avg)",
				nr7High-nr7Low, breakoutCandle.Close, nr7Low, breakoutVol, breakoutVol/avgVol,
			),
		}
	}

	return NoSignal(name)
}
