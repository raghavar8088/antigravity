package scalpers

import (
	"fmt"
	"math"
)

// S14 — Liquidation Cascade Fade
//
// Regime:     VOLATILE only
// Timeframes: 1m (liquidation rate), 15m (ATR/SL reference)
// Logic:
//
//	A liquidation cascade is a spike in forced-liquidation volume in one
//	direction (longs or shorts) that overshoots price beyond fair value as
//	the exchange force-closes leveraged positions. These cascades are
//	well-documented BTC perpetual microstructure events (e.g. the
//	coinglass.com liquidation heatmap, and academic/practitioner writeups on
//	"liquidation cascades" / "long squeeze" / "short squeeze" dynamics).
//	Once the cascade DECELERATES (liquidation rate drops well off its peak),
//	the overshoot tends to mean-revert — so we fade the cascade direction
//	once velocity has cooled, rather than catching the falling knife mid-cascade.
//
// Statefulness note: this codebase's other strategies (S6, S9, etc.) are
// stateless — Evaluate() derives everything from MarketContext alone, no
// internal struct fields beyond the receiver. There is no established
// pattern in this package for per-strategy mutable state (e.g. a struct with
// a sync.RWMutex-protected rolling-peak tracker). Introducing one here would
// require coordinating instance lifetime with the registry (multiple
// goroutines could call Evaluate() concurrently — see types.go: Strategy is
// just an interface, RegistryEntry stores a single shared instance) and would
// add a new pattern this chunk of work has no mandate to design unilaterally.
// JUDGMENT CALL: keep S14 stateless like its siblings. The "rolling peak vs
// current rate" comparison is instead derived entirely from MarketContext:
//   - LongLiquidationsAvg1h / ShortLiquidationsAvg1h = trailing 1h avg rate (USD/min)
//   - LongLiquidationsRate1m / ShortLiquidationsRate1m = most recent 1m rate (USD/min)
//   - LongLiquidationsUSD5m / ShortLiquidationsUSD5m = trailing 5m total (used as a
//     proxy for "was there a recent cascade peak" — if the 5m total is large relative
//     to what the current 1m rate would produce over 5 minutes, the cascade has
//     decelerated from an earlier peak within the same 5m window).
//
// This avoids needing a dedicated in-memory peak tracker while still requiring
// BOTH "a cascade happened" (5m total >> current rate × 5) AND "it's decelerating
// now" (current 1m rate < 50% of the implied recent peak rate) before firing.
type LiquidationCascadeFade struct{}

func (s *LiquidationCascadeFade) Name() string { return "Liquidation_Cascade_Fade" }

func (s *LiquidationCascadeFade) ValidRegimes() []Regime {
	return []Regime{RegimeVolatile}
}

func (s *LiquidationCascadeFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if !ctx.LiquidationFeedPopulated {
		return NoSignal(name)
	}
	if !ctx.LiquidationFeedHealthy {
		// Graceful degradation: feed is stale/disconnected — don't trade on stale data.
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 15 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}

	// Implied recent-peak rate (USD/min) from the 5m rolling total, by side.
	impliedPeakLong := ctx.LongLiquidationsUSD5m / 5.0
	impliedPeakShort := ctx.ShortLiquidationsUSD5m / 5.0

	// ── Long-liquidation cascade fade (longs got wrecked → price overshot down) ──
	// Condition 1: a real cascade happened — implied peak rate > 3x trailing 1h avg.
	longCascadeHappened := ctx.LongLiquidationsAvg1h > 0 && impliedPeakLong > 3.0*ctx.LongLiquidationsAvg1h
	// Condition 2: deceleration — current 1m rate has cooled to <50% of the implied peak.
	longDecelerating := impliedPeakLong > 0 && ctx.LongLiquidationsRate1m < 0.5*impliedPeakLong

	if longCascadeHappened && longDecelerating {
		// Price overshot DOWN on the long-liquidation cascade → fade by going LONG
		// (expect mean-reversion back up once forced selling has exhausted itself).
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Long-liquidation cascade fade: implied peak=%.0f USD/min (1h avg=%.0f), "+
					"now decelerated to %.0f USD/min — fading the overshoot LONG",
				impliedPeakLong, ctx.LongLiquidationsAvg1h, ctx.LongLiquidationsRate1m,
			),
		}
	}

	// ── Short-liquidation cascade fade (shorts got squeezed → price overshot up) ──
	shortCascadeHappened := ctx.ShortLiquidationsAvg1h > 0 && impliedPeakShort > 3.0*ctx.ShortLiquidationsAvg1h
	shortDecelerating := impliedPeakShort > 0 && ctx.ShortLiquidationsRate1m < 0.5*impliedPeakShort

	if shortCascadeHappened && shortDecelerating {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist <= 0 {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Short-liquidation cascade fade: implied peak=%.0f USD/min (1h avg=%.0f), "+
					"now decelerated to %.0f USD/min — fading the squeeze SHORT",
				impliedPeakShort, ctx.ShortLiquidationsAvg1h, ctx.ShortLiquidationsRate1m,
			),
		}
	}

	return NoSignal(name)
}
