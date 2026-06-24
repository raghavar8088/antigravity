package scalpers

import (
	"fmt"
	"math"
)

// S48 — Pivot Point Reversion
//
// Research:   Classic floor pivot points — institutional futures trading
//             desk standard (pre-electronic CBOT/CME floor pivots).
// Regime:     RANGING only
// Timeframes: 1h (prior-period OHLC proxy for pivot calc) + 5m (reversal
//             candle trigger)
// Logic:      Feeds PivotPoints() with the most recent completed Candles1h
//             candle as the "prior period" OHLC. When 5m price reaches R1/R2
//             or S1/S2 AND the current 5m bar shows a reversal candle (body
//             < 1/3 of range, i.e. doji-like, or a long wick opposing the
//             approach direction), fade back toward the pivot.
//
// SIMPLIFICATION NOTE: true floor pivots are computed from the PRIOR DAY's
// OHLC. MarketContext does not separately track daily OHLC, so this strategy
// uses the most recently completed 1h candle as a proxy "prior period" —
// levels are intentionally tighter/more local than true daily pivots, which
// suits the engine's short candle buffers (max 72x 1h) better than trying to
// approximate a full daily session.

type PivotPointReversion struct{}

func (s *PivotPointReversion) Name() string { return "Pivot_Point_Reversion" }

func (s *PivotPointReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

// isReversalCandle returns true if the candle has a small body relative to
// its range (doji-like, body < 1/3 of range) or a long wick opposing the
// approach direction (toward=true means price approached from below, so we
// look for an upper wick rejection; toward=false means approached from
// above, so we look for a lower wick rejection).
func isReversalCandle(c Candle, fromBelow bool) bool {
	rng := c.High - c.Low
	if rng <= 0 {
		return false
	}
	body := math.Abs(c.Close - c.Open)
	if body < rng/3.0 {
		return true
	}
	if fromBelow {
		// Approached resistance from below — look for upper wick rejection.
		upperWick := c.High - math.Max(c.Open, c.Close)
		return upperWick > body
	}
	// Approached support from above — look for lower wick rejection.
	lowerWick := math.Min(c.Open, c.Close) - c.Low
	return lowerWick > body
}

func (s *PivotPointReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 2 || len(ctx.Candles5m) < 5 {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	priorPeriod := ctx.Candles1h[len(ctx.Candles1h)-2] // most recent completed 1h candle
	pivots := PivotPoints(priorPeriod)

	price := ctx.Price
	currentBar := ctx.Candles5m[len(ctx.Candles5m)-1]

	const proximityFactor = 0.15 // ATR-scaled proximity to treat "at" a level

	atLevel := func(level float64) bool {
		return math.Abs(price-level) <= proximityFactor*atr5m
	}

	// Resistance levels (R1/R2) — short reversal.
	if (atLevel(pivots.R1) || atLevel(pivots.R2)) && isReversalCandle(currentBar, true) {
		level := pivots.R1
		if atLevel(pivots.R2) {
			level = pivots.R2
		}
		sl := math.Max(currentBar.High, price) + math.Max(1.0*atr5m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := pivots.PP
		reward := price - tp
		if reward < 2.0*risk {
			tp = price - 2.0*risk
			reward = price - tp
		}
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: price=%.0f at pivot resistance=%.0f (PP=%.0f, R1=%.0f, R2=%.0f, "+
					"prior-1h proxy), reversal candle confirmed",
				price, level, pivots.PP, pivots.R1, pivots.R2,
			),
		}
	}

	// Support levels (S1/S2) — long reversal.
	if (atLevel(pivots.S1) || atLevel(pivots.S2)) && isReversalCandle(currentBar, false) {
		level := pivots.S1
		if atLevel(pivots.S2) {
			level = pivots.S2
		}
		sl := math.Min(currentBar.Low, price) - math.Max(1.0*atr5m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := pivots.PP
		reward := tp - price
		if reward < 2.0*risk {
			tp = price + 2.0*risk
			reward = tp - price
		}
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: price=%.0f at pivot support=%.0f (PP=%.0f, S1=%.0f, S2=%.0f, "+
					"prior-1h proxy), reversal candle confirmed",
				price, level, pivots.PP, pivots.S1, pivots.S2,
			),
		}
	}

	return NoSignal(name)
}
