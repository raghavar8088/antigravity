package scalpers

import (
	"fmt"
	"math"
)

// S43 — Stochastic Oscillator Reversion
//
// Research:   George Lane (1984) Slow Stochastic Oscillator.
// Regime:     RANGING only
// Timeframes: 15m (SlowStochastic %K/%D crosses + ADX filter)
// Logic:      Uses SlowStochastic(14,3) on Candles15m. Oversold (<20) or
//             overbought (>80) %K crosses %D, confirmed by ADX()<20 (not
//             strongly trending, i.e. genuinely ranging).
// Edge:       ADX<20 filter avoids the classic stochastic-in-a-trend trap
//             where oscillators stay pinned at extremes through a real move.

type StochasticOscillatorReversion struct{}

func (s *StochasticOscillatorReversion) Name() string { return "Stochastic_Oscillator_Reversion" }

func (s *StochasticOscillatorReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *StochasticOscillatorReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	adx := ADX(ctx.Candles15m, 14)
	if adx >= 20 {
		// Strongly trending — stochastic extremes unreliable.
		return NoSignal(name)
	}

	k, d := SlowStochastic(ctx.Candles15m, 14, 3)
	kPrev, dPrev := SlowStochastic(ctx.Candles15m[:len(ctx.Candles15m)-1], 14, 3)

	price := ctx.Price

	// Bullish: oversold and %K crosses above %D.
	oversold := kPrev < 20 && dPrev < 20
	bullCross := kPrev <= dPrev && k > d

	if oversold && bullCross {
		sl := price - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price + 2.0*risk
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
				"RANGING: ADX=%.1f (<20, ranging), Stoch %%K=%.1f crossed above %%D=%.1f "+
					"from oversold (<20), price=%.0f",
				adx, k, d, price,
			),
		}
	}

	// Bearish: overbought and %K crosses below %D.
	overbought := kPrev > 80 && dPrev > 80
	bearCross := kPrev >= dPrev && k < d

	if overbought && bearCross {
		sl := price + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := price - 2.0*risk
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
				"RANGING: ADX=%.1f (<20, ranging), Stoch %%K=%.1f crossed below %%D=%.1f "+
					"from overbought (>80), price=%.0f",
				adx, k, d, price,
			),
		}
	}

	return NoSignal(name)
}
