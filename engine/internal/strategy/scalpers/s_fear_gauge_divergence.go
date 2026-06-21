package scalpers

import "fmt"

// S13 — originally scoped as "Term_Structure_Skew_Signal" (DVOL term
// structure / options skew across expiries). INFEASIBLE within this scope:
// Deribit's term-structure and 25-delta skew data require per-expiry option
// chain queries (get_book_summary_by_currency + per-instrument greeks, or the
// dedicated skew endpoints) and meaningful aggregation logic across multiple
// expiries — this is materially more REST surface area and parsing complexity
// than a simple polled REST value, and out of scope for "simple REST in scope"
// per the task brief. Documenting as infeasible here and implementing the
// specified fallback instead:
//
// Fear_Gauge_Divergence — DVOL-vs-price divergence.
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h (DVOL + price divergence over 3+ bars)
// Rollout:    Phase 4 (see rollout_phase.go)
//
// Logic:
//   - DVOL rising while price rising = bearish leading signal (vol "should"
//     fall in a healthy uptrend; rising vol alongside rising price signals
//     uneasy/distributive buying — a leading bearish divergence).
//   - DVOL falling while price falling = capitulation/bottoming signal (vol
//     compression during a selloff suggests selling pressure is exhausting,
//     not accelerating — a leading bullish reversal signal).
//
// Requires 3+ bar confirmation: both the price trend AND the DVOL trend must
// hold consistently across the last 3 closed 1h bars (not a single-bar
// comparison, which fires on noise).
//
// Graceful degradation: if DVOL is unpopulated/unhealthy, this strategy
// cannot compute its core signal (DVOL-vs-price divergence has no sensible
// RealizedVol substitute — RV is backward-looking and would just restate
// price action) and returns NoSignal with reduced/no confidence rather than
// firing on stale or fabricated data. This is the one strategy in the family
// where there is no reasonable proxy fallback; it stays dormant until DVOL
// recovers, which is documented and acceptable since S10-S12 already cover
// the RV-only degradation path for the family as a whole.

type FearGaugeDivergence struct{}

func (s *FearGaugeDivergence) Name() string { return "Fear_Gauge_Divergence" }

func (s *FearGaugeDivergence) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

func (s *FearGaugeDivergence) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	// No RV substitute makes sense for this strategy — if DVOL is down, stay dormant.
	if !ctx.DVOLPopulated || !ctx.DVOLHealthy || len(ctx.DVOLHistory) < 3 {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 4 {
		return NoSignal(name)
	}

	c1h := ctx.Candles1h
	last4 := c1h[len(c1h)-4:] // need 4 closes to get 3 bar-over-bar deltas
	dvolHist := ctx.DVOLHistory
	if len(dvolHist) < 4 {
		return NoSignal(name)
	}
	dvolLast4 := dvolHist[len(dvolHist)-4:]

	priceUpBars, priceDownBars := 0, 0
	dvolUpBars, dvolDownBars := 0, 0
	for i := 1; i < 4; i++ {
		if last4[i].Close > last4[i-1].Close {
			priceUpBars++
		} else if last4[i].Close < last4[i-1].Close {
			priceDownBars++
		}
		if dvolLast4[i] > dvolLast4[i-1] {
			dvolUpBars++
		} else if dvolLast4[i] < dvolLast4[i-1] {
			dvolDownBars++
		}
	}

	// Require all 3 consecutive bar-over-bar deltas to agree (3-bar confirmation).
	priceRisingConfirmed := priceUpBars == 3
	priceFallingConfirmed := priceDownBars == 3
	dvolRisingConfirmed := dvolUpBars == 3
	dvolFallingConfirmed := dvolDownBars == 3

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := max64(1.0*atr1h, 0.003*price)

	// Bearish leading divergence: price up + DVOL up, 3-bar confirmed both ways.
	if priceRisingConfirmed && dvolRisingConfirmed {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.62,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"Fear gauge bearish divergence: price rising 3/3 bars (%.0f→%.0f) while DVOL rising 3/3 (%.1f→%.1f) — uneasy buying",
				last4[0].Close, last4[3].Close, dvolLast4[0], dvolLast4[3],
			),
		}
	}

	// Bullish capitulation/bottom signal: price down + DVOL down, 3-bar confirmed.
	if priceFallingConfirmed && dvolFallingConfirmed {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.62,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"Fear gauge capitulation signal: price falling 3/3 bars (%.0f→%.0f) while DVOL falling 3/3 (%.1f→%.1f) — selling exhausting",
				last4[0].Close, last4[3].Close, dvolLast4[0], dvolLast4[3],
			),
		}
	}

	return NoSignal(name)
}
