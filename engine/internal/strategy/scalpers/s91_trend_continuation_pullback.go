package scalpers

import (
	"fmt"
	"math"
)

// S91 — Trend Continuation Pullback (AB=CD / Fibonacci)
//
// Citation:   Scott Carney, "Harmonic Trading, Volume 1" (2010). The AB=CD
//             measured move pattern: price makes a leg (A→B), pulls back to a
//             Fibonacci level (B→C), then continues the trend (C→D ≈ AB).
//             Used by institutional desks for structured trend entries.
// Regime:     TRENDING
// Timeframes: 1h (for leg identification and Fibonacci levels)
// Logic:      Identify AB leg >1.5% move on 1h. Pullback to 50-62% Fibonacci
//             retracement (B→C). Resume in AB direction with CVD confirmation.

type TrendContinuationPullback struct{}

func (s *TrendContinuationPullback) Name() string { return "Trend_Continuation_Pullback" }

func (s *TrendContinuationPullback) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *TrendContinuationPullback) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 20 {
		return NoSignal(name)
	}

	candles := ctx.Candles1h
	n := len(candles)
	price := ctx.Price

	atr := ATR(candles, 14)
	if atr == 0 {
		return NoSignal(name)
	}

	// Find AB leg: look back up to 10 bars for a significant swing
	// A = swing extreme, B = recent swing opposite
	// Bullish AB: A=swing low, B=swing high after A
	// Find the most recent significant leg (>1.5%)
	const minLegPct = 0.015

	// Bullish continuation: look for low→high leg, then pullback
	// Scan last 10 bars for a meaningful upswing endpoint (B)
	swingHigh10 := SwingHigh(candles[:n-1], 10)
	swingLow10 := SwingLow(candles[:n-1], 10)

	// Detect bullish AB leg
	legUp := swingHigh10 > 0 && swingLow10 > 0 && (swingHigh10-swingLow10)/swingLow10 >= minLegPct
	// Detect bearish AB leg
	legDown := swingHigh10 > 0 && swingLow10 > 0 && (swingHigh10-swingLow10)/swingHigh10 >= minLegPct

	// CVD must confirm direction
	cvdRising := ctx.CVD > ctx.CVDPrev
	cvdFalling := ctx.CVD < ctx.CVDPrev

	// Bullish pullback: price near 50-62% fib retrace of upswing, CVD rising
	if legUp && cvdRising {
		fibLevels := FibRetracement(swingHigh10, swingLow10, 0.382, 0.500, 0.618)
		fib618 := fibLevels[2] // deepest retrace (lowest price)
		fib382 := fibLevels[0] // shallowest retrace (highest price)
		inFibZone := price >= fib618 && price <= fib382

		if inFibZone {
			minSL := math.Max(1.0*atr, 0.003*price)
			sl := swingLow10 - 0.3*atr
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
					"AB=CD pullback LONG: leg %.0f→%.0f (%.1f%%), fib zone %.0f-%.0f, CVD rising",
					swingLow10, swingHigh10, (swingHigh10-swingLow10)/swingLow10*100,
					fib618, fib382,
				),
			}
		}
	}

	// Bearish pullback: price near 50-62% fib retrace of downswing, CVD falling
	if legDown && cvdFalling {
		fibLevels := FibRetracement(swingHigh10, swingLow10, 0.382, 0.500, 0.618)
		// For bearish: retrace is upward from swingLow
		fib382Bear := swingLow10 + 0.382*(swingHigh10-swingLow10)
		fib618Bear := swingLow10 + 0.618*(swingHigh10-swingLow10)
		_ = fibLevels
		inFibZone := price >= fib382Bear && price <= fib618Bear

		if inFibZone {
			minSL := math.Max(1.0*atr, 0.003*price)
			sl := swingHigh10 + 0.3*atr
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
					"AB=CD pullback SHORT: leg %.0f→%.0f (%.1f%%), fib zone %.0f-%.0f, CVD falling",
					swingHigh10, swingLow10, (swingHigh10-swingLow10)/swingHigh10*100,
					fib382Bear, fib618Bear,
				),
			}
		}
	}

	return NoSignal(name)
}
