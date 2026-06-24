package scalpers

import (
	"fmt"
	"math"
)

// S30 — Dual Momentum Trend
//
// Citation:   Gary Antonacci, "Dual Momentum Investing" (2014) — combines
//             absolute momentum (is the asset's own trend positive?) with
//             relative momentum (is the move confirmed by participation?).
// Regime:     TRENDING only
// Timeframes: 1h (bias) + 15m (confirm) + 5m (trigger)
// Logic:      Absolute momentum: BTC return is positive (long) or negative
//             (short) and aligned across 15m/1h/4h-equivalent windows using
//             Candles1h, Candles15m and Candles1m as proxies for the
//             multi-horizon return check. Relative momentum confirmed by
//             volume: current volume > rolling average volume.
// Edge:       Filters out trend signals that aren't backed by real
//             participation — avoids low-volume drift being misread as trend.

type DualMomentumTrend struct{}

func (s *DualMomentumTrend) Name() string { return "Dual_Momentum_Trend" }

func (s *DualMomentumTrend) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *DualMomentumTrend) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 13 || len(ctx.Candles15m) < 25 || len(ctx.Candles5m) < 21 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	// Absolute momentum: return over 12 bars on each timeframe, aligned in sign.
	ret1h := barReturn(ctx.Candles1h, 12)
	ret15m := barReturn(ctx.Candles15m, 12)
	ret5m := barReturn(ctx.Candles5m, 12)

	longAbs := ret1h > 0 && ret15m > 0 && ret5m > 0
	shortAbs := ret1h < 0 && ret15m < 0 && ret5m < 0
	if !longAbs && !shortAbs {
		return NoSignal(name)
	}

	// Relative momentum: volume participation confirms the move.
	avgVol15m := AvgVolume(ctx.Candles15m, 20)
	curVol15m := ctx.Candles15m[len(ctx.Candles15m)-1].Volume
	volConfirm := avgVol15m > 0 && curVol15m > avgVol15m

	if !volConfirm {
		return NoSignal(name)
	}

	price := ctx.Price

	if longAbs {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
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
				"Dual momentum bullish: 1h ret=%.4f, 15m ret=%.4f, 5m ret=%.4f, "+
					"volume %.0f > avg %.0f",
				ret1h, ret15m, ret5m, curVol15m, avgVol15m,
			),
		}
	}

	if shortAbs {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
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
				"Dual momentum bearish: 1h ret=%.4f, 15m ret=%.4f, 5m ret=%.4f, "+
					"volume %.0f > avg %.0f",
				ret1h, ret15m, ret5m, curVol15m, avgVol15m,
			),
		}
	}

	return NoSignal(name)
}

// barReturn computes the simple return over the trailing `lookback` bars.
func barReturn(candles []Candle, lookback int) float64 {
	n := len(candles)
	if n < lookback+1 {
		return 0
	}
	start := candles[n-1-lookback].Close
	end := candles[n-1].Close
	if start == 0 {
		return 0
	}
	return (end - start) / start
}
