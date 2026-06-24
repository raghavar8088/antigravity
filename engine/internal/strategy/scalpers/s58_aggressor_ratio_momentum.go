package scalpers

import (
	"fmt"
	"math"
)

// S58 — Aggressor Ratio Momentum
//
// Regime:     TRENDING, VOLATILE
// Timeframes: 5m (EMA trend filter) + CVDHistory (aggressor proxy)
// Research:   Hasbrouck & Saar (2002) "Limit Orders and Volatility in a
//             Hybrid Market" — a sustained skew in aggressive (market) order
//             flow toward one side, held over a meaningful window rather than
//             a single print, is associated with continuing short-term price
//             momentum in that direction.
// Logic:      Ideal data is AggressorBuyRatio100 > 0.65 sustained for >=5
//             minutes (requires MicrostructurePopulated). That feed is not
//             yet wired live, so this strategy falls back to a proxy derived
//             from ctx.CVDHistory: count how many of the last 5 CVDHistory
//             readings show CVD increasing vs the prior reading; treat
//             >=4-out-of-5 "up" steps as a buy-aggressor-dominant regime
//             (and symmetric <=1-out-of-5 "up" steps, i.e. >=4 down steps, as
//             sell-aggressor-dominant). Combined with price holding above
//             (long case) / below (short case) the 5m EMA20 as the momentum
//             continuation filter.
// Edge:       Confirms that recent aggressive flow has been persistently
//             one-sided (not just a single burst) before trading the
//             continuation of that momentum.

type AggressorRatioMomentum struct{}

func (s *AggressorRatioMomentum) Name() string { return "Aggressor_Ratio_Momentum" }

func (s *AggressorRatioMomentum) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeVolatile}
}

func (s *AggressorRatioMomentum) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 20 {
		return NoSignal(name)
	}

	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}
	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}
	ema20_5m := EMA(ctx.Candles5m, 20)
	if ema20_5m == 0 {
		return NoSignal(name)
	}

	var buyDominant, sellDominant bool
	var detail string

	if ctx.MicrostructurePopulated {
		// Ideal path: live aggressor-buy-ratio feed.
		buyDominant = ctx.AggressorBuyRatio100 > 0.65
		sellDominant = ctx.AggressorBuyRatio100 < 0.35
		detail = fmt.Sprintf("AggressorBuyRatio100=%.2f", ctx.AggressorBuyRatio100)
	} else {
		// PROXY: MicrostructurePopulated==false in production today — derive
		// an aggressor-ratio-like measure from CVDHistory step direction.
		if len(ctx.CVDHistory) < 6 {
			return NoSignal(name)
		}
		hist := ctx.CVDHistory
		nh := len(hist)
		last5 := hist[nh-6:] // 6 readings -> 5 step deltas
		upSteps, downSteps := 0, 0
		for i := 1; i < len(last5); i++ {
			if last5[i] > last5[i-1] {
				upSteps++
			} else if last5[i] < last5[i-1] {
				downSteps++
			}
		}
		buyDominant = upSteps >= 4
		sellDominant = downSteps >= 4
		detail = fmt.Sprintf("PROXY aggressor: %d/5 up-steps, %d/5 down-steps in CVDHistory", upSteps, downSteps)
	}

	if !buyDominant && !sellDominant {
		return NoSignal(name)
	}

	slDistMin := math.Max(1.0*atr5m, 0.003*price)

	if buyDominant && price > ema20_5m {
		sl := price - slDistMin
		tp := price + 2.0*slDistMin
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.65
		if ctx.MicrostructurePopulated {
			conf = 0.72
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Aggressor ratio momentum LONG: %s, price %.0f holding above 5m EMA20=%.0f",
				detail, price, ema20_5m,
			),
		}
	}

	if sellDominant && price < ema20_5m {
		sl := price + slDistMin
		tp := price - 2.0*slDistMin
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.65
		if ctx.MicrostructurePopulated {
			conf = 0.72
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Aggressor ratio momentum SHORT: %s, price %.0f holding below 5m EMA20=%.0f",
				detail, price, ema20_5m,
			),
		}
	}

	return NoSignal(name)
}
