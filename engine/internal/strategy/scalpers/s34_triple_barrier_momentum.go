package scalpers

import (
	"fmt"
	"math"
)

// S34 — Triple Barrier Momentum
//
// Citation:   Marcos Lopez de Prado, "Advances in Financial Machine
//             Learning" (2018) — the Triple-Barrier Method labels outcomes
//             by which of three barriers (upper/profit, lower/loss, or
//             vertical/time) is hit first.
// Regime:     TRENDING only
// Timeframes: 15m (trend + barrier estimate), 5m (CVD confirm proxy)
// Logic:      Base momentum signal: EMA9 > EMA21 (bull) / EMA9 < EMA21
//             (bear) on Candles15m, confirmed by CVD direction. The trade
//             is then filtered through a SIMPLIFIED barrier-probability
//             proxy: treating price as a driftless Brownian motion with
//             volatility = ATR15m per bar, we estimate P(TP barrier hit
//             before SL barrier) using the standard "first passage of a
//             symmetric range" approximation P(up) ≈ slDist/(slDist+tpDist)
//             adjusted by an empirical momentum-strength tilt derived from
//             the magnitude of the EMA9-EMA21 spread relative to ATR. This
//             is NOT a rigorous solution to the first-passage PDE — it is a
//             fast, monotonic proxy that increases with momentum strength
//             and decreases with barrier asymmetry, used only as a trade
//             filter (threshold 0.55), not a precise probability.
// Edge:       Filters out momentum entries that have unfavorable
//             barrier geometry before risking capital, channeling Lopez de
//             Prado's labeling logic into a real-time entry filter.

type TripleBarrierMomentum struct{}

func (s *TripleBarrierMomentum) Name() string { return "Triple_Barrier_Momentum" }

func (s *TripleBarrierMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

// barrierHitProbability is a SIMPLIFIED proxy (see file header) for
// P(TP hit before SL | momentum strength, vol). Not a rigorous first-passage
// solution — monotonic heuristic only, used purely as a filter threshold.
func barrierHitProbability(slDist, tpDist, momentumStrength float64) float64 {
	if slDist+tpDist == 0 {
		return 0
	}
	base := slDist / (slDist + tpDist) // symmetric-range first-passage baseline
	// Tilt upward with momentum strength, capped at +0.25.
	tilt := math.Min(0.25, momentumStrength*0.5)
	p := base + tilt
	if p > 0.95 {
		p = 0.95
	}
	if p < 0.05 {
		p = 0.05
	}
	return p
}

func (s *TripleBarrierMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 25 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	ema9 := EMA(ctx.Candles15m, 9)
	ema21 := EMA(ctx.Candles15m, 21)

	bullMomentum := ema9 > ema21
	bearMomentum := ema9 < ema21
	if !bullMomentum && !bearMomentum {
		return NoSignal(name)
	}

	cvdLongConfirm := ctx.CVD > ctx.CVDPrev
	cvdShortConfirm := ctx.CVD < ctx.CVDPrev

	price := ctx.Price
	spread := math.Abs(ema9 - ema21)
	momentumStrength := spread / atr15m // normalized momentum magnitude

	if bullMomentum && cvdLongConfirm {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		tpDist := tp - price

		prob := barrierHitProbability(slDist, tpDist, momentumStrength)
		if prob <= 0.55 {
			return NoSignal(name)
		}

		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.55 + (prob-0.55)*0.5,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Triple-barrier momentum bull: EMA9=%.0f>EMA21=%.0f, CVD confirm, "+
					"barrier-prob proxy=%.2f (momentum=%.2f)",
				ema9, ema21, prob, momentumStrength,
			),
		}
	}

	if bearMomentum && cvdShortConfirm {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		tpDist := price - tp

		prob := barrierHitProbability(slDist, tpDist, momentumStrength)
		if prob <= 0.55 {
			return NoSignal(name)
		}

		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.55 + (prob-0.55)*0.5,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Triple-barrier momentum bear: EMA9=%.0f<EMA21=%.0f, CVD confirm, "+
					"barrier-prob proxy=%.2f (momentum=%.2f)",
				ema9, ema21, prob, momentumStrength,
			),
		}
	}

	return NoSignal(name)
}
