package scalpers

import (
	"fmt"
	"math"
)

// S35 — Fractal Breakout
//
// Citation:   Bill Williams, "Trading Chaos" (1995) — 5-bar fractal
//             patterns identify local swing highs/lows; a breakout through
//             a confirmed fractal level signals a new directional move.
// Regime:     TRENDING only
// Timeframes: 15m
// Logic:      A fractal high at candle[i] requires High[i] to exceed the
//             High of the 2 candles on each side (5-bar window). A fractal
//             low is the symmetric case on Low. When price breaks above the
//             most recent confirmed fractal high (or below a confirmed
//             fractal low) with Volume > 1.5x the 20-period AvgVolume(), the
//             breakout is taken.
// Edge:       Fractals identify objectively-defined local extremes; the
//             volume filter screens out low-conviction breakouts that tend
//             to fail and reverse.

type FractalBreakout struct{}

func (s *FractalBreakout) Name() string { return "Fractal_Breakout" }

func (s *FractalBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

// findLastFractalHigh scans backward (excluding the most recent 2 bars,
// since a fractal needs 2 confirming bars after it) for the most recent
// confirmed 5-bar fractal high. Returns (value, found).
func findLastFractalHigh(candles []Candle) (float64, bool) {
	n := len(candles)
	for i := n - 3; i >= 2; i-- {
		c := candles[i]
		if c.High > candles[i-1].High && c.High > candles[i-2].High &&
			c.High > candles[i+1].High && c.High > candles[i+2].High {
			return c.High, true
		}
	}
	return 0, false
}

// findLastFractalLow scans backward for the most recent confirmed 5-bar
// fractal low. Returns (value, found).
func findLastFractalLow(candles []Candle) (float64, bool) {
	n := len(candles)
	for i := n - 3; i >= 2; i-- {
		c := candles[i]
		if c.Low < candles[i-1].Low && c.Low < candles[i-2].Low &&
			c.Low < candles[i+1].Low && c.Low < candles[i+2].Low {
			return c.Low, true
		}
	}
	return 0, false
}

func (s *FractalBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	candles := ctx.Candles15m
	atr15m := ATR(candles, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	avgVol := AvgVolume(candles, 20)
	if avgVol == 0 {
		return NoSignal(name)
	}
	curVol := candles[len(candles)-1].Volume
	volConfirm := curVol > 1.5*avgVol
	if !volConfirm {
		return NoSignal(name)
	}

	price := ctx.Price

	fractalHigh, foundHigh := findLastFractalHigh(candles)
	fractalLow, foundLow := findLastFractalLow(candles)

	if foundHigh && price > fractalHigh {
		sl := fractalHigh - math.Max(1.0*atr15m, 0.003*price)
		// SL must remain below current price for a long
		if sl >= price {
			sl = price - math.Max(1.0*atr15m, 0.003*price)
		}
		slDist := price - sl
		if slDist < math.Max(1.0*atr15m, 0.003*price) {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Fractal breakout long: price %.0f broke fractal high %.0f, "+
					"volume %.0f > 1.5x avg %.0f",
				price, fractalHigh, curVol, avgVol,
			),
		}
	}

	if foundLow && price < fractalLow {
		sl := fractalLow + math.Max(1.0*atr15m, 0.003*price)
		if sl <= price {
			sl = price + math.Max(1.0*atr15m, 0.003*price)
		}
		slDist := sl - price
		if slDist < math.Max(1.0*atr15m, 0.003*price) {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.72,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Fractal breakout short: price %.0f broke fractal low %.0f, "+
					"volume %.0f > 1.5x avg %.0f",
				price, fractalLow, curVol, avgVol,
			),
		}
	}

	return NoSignal(name)
}
