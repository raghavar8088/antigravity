package scalpers

import (
	"fmt"
	"math"
)

// S68 — Excess Return Momentum
//
// Research: Cong, L.W., Tang, K., Wang, Y. & Yang, X. (2021) "Crypto
// Factors", working paper — documents momentum/excess-return factors in
// crypto markets analogous to traditional equity factor models.
// No AI/LLM/ML-library calls — explicit rule-based replication of the cited
// research's feature signal: hand-coded rolling-return-vs-own-mean
// computation at 3 horizons, no fitted factor model.
//
// Regime:     TRENDING
// Timeframes: 15m, 1h, 4h
// Logic:      At each of 3 horizons (15m, 1h, 4h), compute BTC's most recent
//             period return minus that timeframe's own trailing-period mean
//             return ("excess return"). Require all three excess returns
//             positive AND increasing in magnitude (accelerating) → LONG;
//             all negative and accelerating (more negative) → SHORT.
// Edge:       Accelerating positive excess return across multiple horizons
//             simultaneously is a stronger momentum-factor confirmation than
//             any single-horizon return reading alone.
//
// SUBSTITUTION NOTE: The cited literature's crypto factor models typically
// benchmark against a true 30-day rolling mean return. This engine's candle
// buffers do not retain 30 days of history at any timeframe (Candles15m
// caps at 60 bars ~ 15h, Candles1h caps at 72 bars = 3 days, Candles4h caps
// at 30 bars = 5 days). We substitute each timeframe's own trailing-period
// mean return (computed from whatever history the buffer holds) as the
// baseline instead of a literal 30-day mean — documented here as a
// deliberate, buffer-constrained substitution.

type ExcessReturnMomentum struct{}

func (s *ExcessReturnMomentum) Name() string { return "Excess_Return_Momentum" }

func (s *ExcessReturnMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

// excessReturn computes the most recent single-bar return minus the mean
// single-bar return over the trailing window, for a given candle slice.
func excessReturn(candles []Candle) (excess float64, ok bool) {
	n := len(candles)
	if n < 10 {
		return 0, false
	}
	rets := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		if candles[i-1].Close <= 0 {
			continue
		}
		rets = append(rets, (candles[i].Close-candles[i-1].Close)/candles[i-1].Close)
	}
	if len(rets) < 5 {
		return 0, false
	}
	var mean float64
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	last := rets[len(rets)-1]
	return last - mean, true
}

func (s *ExcessReturnMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 15 || len(ctx.Candles1h) < 15 || len(ctx.Candles4h) < 15 {
		return NoSignal(name)
	}

	ex15m, ok1 := excessReturn(ctx.Candles15m)
	ex1h, ok2 := excessReturn(ctx.Candles1h)
	ex4h, ok3 := excessReturn(ctx.Candles4h)
	if !ok1 || !ok2 || !ok3 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	price := ctx.Price

	allPositive := ex15m > 0 && ex1h > 0 && ex4h > 0
	allNegative := ex15m < 0 && ex1h < 0 && ex4h < 0

	// Accelerating in magnitude: each longer horizon's excess return
	// magnitude should be >= the shorter horizon's (momentum building, not
	// fading) — require strictly increasing magnitude 15m -> 1h -> 4h.
	// This check is sign-agnostic (applies to both the all-positive LONG
	// case and the all-negative SHORT case).
	accelerating := math.Abs(ex1h) > math.Abs(ex15m) && math.Abs(ex4h) > math.Abs(ex1h)

	if allPositive && accelerating {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Excess return momentum LONG: 15m=%.5f 1h=%.5f 4h=%.5f all positive, accelerating magnitude (trailing-period mean baseline substitution — see header note)",
				ex15m, ex1h, ex4h,
			),
		}
	}

	if allNegative && accelerating {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Excess return momentum SHORT: 15m=%.5f 1h=%.5f 4h=%.5f all negative, accelerating magnitude (trailing-period mean baseline substitution — see header note)",
				ex15m, ex1h, ex4h,
			),
		}
	}

	return NoSignal(name)
}
