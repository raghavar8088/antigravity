package scalpers

import (
	"fmt"
	"math"
)

// S46 — Price Channel Mean Reversion
//
// Research:   Chester Keltner (1960) original Keltner Channel; Linda Raschke
//             ATR-modified variant (the now-standard EMA±ATR construction).
// Regime:     RANGING only
// Timeframes: 15m (EMA20 ± 2×ATR14 channel)
// Logic:      Builds an ATR-modified Keltner Channel: EMA20 ± 2×ATR14 on
//             Candles15m. Requires the prior bar to have closed outside the
//             channel, and the current bar to have reverted back inside it.
// Edge:       Waiting for the "outside then back inside" 2-bar pattern
//             (rather than fading the outside touch immediately) filters out
//             channel breakouts that keep running.

type PriceChannelMeanReversion struct{}

func (s *PriceChannelMeanReversion) Name() string { return "Price_Channel_Mean_Reversion" }

func (s *PriceChannelMeanReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *PriceChannelMeanReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 22 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	n := len(c15)

	atr15m := ATR(c15, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}
	atr15mPrev := ATR(c15[:n-1], 14)
	if atr15mPrev == 0 {
		return NoSignal(name)
	}

	ema20 := EMA(c15, 20)
	ema20Prev := EMA(c15[:n-1], 20)
	if ema20 == 0 || ema20Prev == 0 {
		return NoSignal(name)
	}

	upper := ema20 + 2*atr15m
	lower := ema20 - 2*atr15m
	upperPrev := ema20Prev + 2*atr15mPrev
	lowerPrev := ema20Prev - 2*atr15mPrev

	prevClose := c15[n-2].Close
	price := ctx.Price

	// Bullish: prior bar closed below lower channel, current price back inside.
	prevOutsideLower := prevClose < lowerPrev
	nowInside := price >= lower && price <= upper

	if prevOutsideLower && nowInside {
		sl := math.Min(prevClose, price) - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := ema20
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
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: prior 15m close=%.0f outside lower Keltner=%.0f (EMA20=%.0f-2ATR), "+
					"price=%.0f reverted back inside channel",
				prevClose, lowerPrev, ema20Prev, price,
			),
		}
	}

	// Bearish: prior bar closed above upper channel, current price back inside.
	prevOutsideUpper := prevClose > upperPrev

	if prevOutsideUpper && nowInside {
		sl := math.Max(prevClose, price) + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := ema20
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
			Confidence: 0.71,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: prior 15m close=%.0f outside upper Keltner=%.0f (EMA20=%.0f+2ATR), "+
					"price=%.0f reverted back inside channel",
				prevClose, upperPrev, ema20Prev, price,
			),
		}
	}

	return NoSignal(name)
}
