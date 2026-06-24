package scalpers

import (
	"fmt"
	"math"
)

// S36 — SuperTrend Momentum
//
// Citation:   Olivier Seban, SuperTrend indicator (popularized in
//             practitioner technical-analysis literature, early 2000s) —
//             ATR-based dynamic trailing band that flips when price closes
//             through it, used as a trend-following entry/exit signal.
// Regime:     TRENDING only
// Timeframes: 15m (entry trigger) + 1h (trend alignment confirm)
// Logic:      Compute SuperTrend(period=10, multiplier=3) on Candles15m.
//             When SuperTrend flips to (or is currently) an uptrend and the
//             1h trend is aligned (EMA9 > EMA21), go long; symmetric for
//             short.
// Edge:       SuperTrend's ATR-adaptive band reduces whipsaw vs. fixed
//             moving-average crosses; the 1h alignment filter avoids
//             trading 15m SuperTrend flips against the higher-timeframe
//             trend.

type SuperTrendMomentum struct{}

func (s *SuperTrendMomentum) Name() string { return "SuperTrend_Momentum" }

func (s *SuperTrendMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *SuperTrendMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 22 || len(ctx.Candles1h) < 22 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	stValue, stUptrend := SuperTrend(ctx.Candles15m, 10, 3.0)
	if stValue == 0 {
		return NoSignal(name)
	}

	ema9_1h := EMA(ctx.Candles1h, 9)
	ema21_1h := EMA(ctx.Candles1h, 21)
	bull1h := ema9_1h > ema21_1h
	bear1h := ema9_1h < ema21_1h

	price := ctx.Price

	if stUptrend && bull1h {
		sl := math.Min(stValue, price-math.Max(1.0*atr15m, 0.003*price))
		slDist := price - sl
		if slDist < math.Max(1.0*atr15m, 0.003*price) {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"SuperTrend uptrend (ST=%.0f) + 1h bullish (EMA9=%.0f>EMA21=%.0f)",
				stValue, ema9_1h, ema21_1h,
			),
		}
	}

	if !stUptrend && bear1h {
		sl := math.Max(stValue, price+math.Max(1.0*atr15m, 0.003*price))
		slDist := sl - price
		if slDist < math.Max(1.0*atr15m, 0.003*price) {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.74,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"SuperTrend downtrend (ST=%.0f) + 1h bearish (EMA9=%.0f<EMA21=%.0f)",
				stValue, ema9_1h, ema21_1h,
			),
		}
	}

	return NoSignal(name)
}
