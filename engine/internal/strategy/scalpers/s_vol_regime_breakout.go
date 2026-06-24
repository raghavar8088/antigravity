package scalpers

import "fmt"

// S11 — Vol_Regime_Breakout
//
// Regime:     VOLATILE, TRENDING
// Timeframes: 1h (DVOL rolling high) + 15m (directional confirmation candle)
// Rollout:    Phase 2 (see rollout_phase.go)
//
// Logic: when DVOL breaks above its own rolling 20-period high (vol regime
// shift — the market is repricing forward-looking volatility higher) AND
// realized vol is simultaneously accelerating (RV24 > RV24 one bar back),
// trade a continuation breakout in the direction of the most recent strong
// directional 15m candle. This captures the early stage of a volatility
// expansion regime where directional follow-through is statistically more
// likely than mean reversion (unlike the calm/ranging regime S10/S12 target).
//
// Graceful degradation: DVOL rolling-high breakout requires the DVOL feed.
// If DVOL is unpopulated/unhealthy, this strategy falls back to using
// RealizedVol's own rolling 20-period high as the breakout trigger instead
// (RV is always computable from candles already in MarketContext).

const (
	volBreakoutLookback   = 20 // rolling window for "own high" breakout check
	volBreakoutStrongBody = 0.6 // candle body must be >= 60% of its range to count as "strong"
)

type VolRegimeBreakout struct{}

func (s *VolRegimeBreakout) Name() string { return "Vol_Regime_Breakout" }

func (s *VolRegimeBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeVolatile, RegimeTrending}
}

func (s *VolRegimeBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < volBreakoutLookback+25 || len(ctx.Candles15m) < 5 {
		return NoSignal(name)
	}

	rvNow := RealizedVol(ctx.Candles1h, 24)
	rvPrev := RealizedVol(ctx.Candles1h[:len(ctx.Candles1h)-1], 24)
	if rvNow == 0 || rvPrev == 0 {
		return NoSignal(name)
	}
	rvAccelerating := rvNow > rvPrev

	var breakoutConfirmed bool
	var volDesc string
	usingDVOL := ctx.DVOLPopulated && ctx.DVOLHealthy && len(ctx.DVOLHistory) >= volBreakoutLookback
	if usingDVOL {
		hist := ctx.DVOLHistory
		rollingHigh := hist[0]
		for _, v := range hist {
			if v > rollingHigh {
				rollingHigh = v
			}
		}
		breakoutConfirmed = ctx.DVOL > rollingHigh
		volDesc = fmt.Sprintf("DVOL=%.1f broke own %d-period high=%.1f", ctx.DVOL, volBreakoutLookback, rollingHigh)
	} else {
		// Fallback: RV's own rolling high over the same lookback window of 1h bars.
		if len(ctx.Candles1h) < volBreakoutLookback+25 {
			return NoSignal(name)
		}
		rollingHigh := 0.0
		for i := 0; i < volBreakoutLookback; i++ {
			end := len(ctx.Candles1h) - volBreakoutLookback + i
			v := RealizedVol(ctx.Candles1h[:end], 24)
			if v > rollingHigh {
				rollingHigh = v
			}
		}
		breakoutConfirmed = rvNow > rollingHigh
		volDesc = fmt.Sprintf("DVOL_unavailable, RV-only fallback: RV=%.1f broke own %d-period high=%.1f", rvNow, volBreakoutLookback, rollingHigh)
	}

	if !breakoutConfirmed || !rvAccelerating {
		return NoSignal(name)
	}

	// 3-bar confirmation: require the last 3 closed 15m candles to show a
	// consistent directional bias (not a single-bar noise spike), then use the
	// most recent strong-bodied candle to set direction.
	c15 := ctx.Candles15m
	last3 := c15[len(c15)-3:]
	upCount, downCount := 0, 0
	for _, c := range last3 {
		if c.Close > c.Open {
			upCount++
		} else if c.Close < c.Open {
			downCount++
		}
	}

	lastCandle := c15[len(c15)-1]
	candleRange := lastCandle.High - lastCandle.Low
	if candleRange == 0 {
		return NoSignal(name)
	}
	body := lastCandle.Close - lastCandle.Open
	bodyFrac := abs64(body) / candleRange
	strongCandle := bodyFrac >= volBreakoutStrongBody

	if !strongCandle {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := max64(1.0*atr15m, 0.003*price)

	if upCount >= 2 && body > 0 {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  confidenceForVolFeed(usingDVOL, 0.70),
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"Vol regime breakout LONG: %s, RV accelerating (%.1f>%.1f), 3-bar bias up (%d/3), strong bullish candle",
				volDesc, rvNow, rvPrev, upCount,
			),
		}
	}

	if downCount >= 2 && body < 0 {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  confidenceForVolFeed(usingDVOL, 0.70),
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"Vol regime breakout SHORT: %s, RV accelerating (%.1f>%.1f), 3-bar bias down (%d/3), strong bearish candle",
				volDesc, rvNow, rvPrev, downCount,
			),
		}
	}

	return NoSignal(name)
}

// confidenceForVolFeed reduces confidence when running on the RV-only
// fallback (DVOL feed down/unpopulated) since RV reacts slower than IV.
func confidenceForVolFeed(usingDVOL bool, base float64) float64 {
	if usingDVOL {
		return base
	}
	return base - 0.15
}
