package scalpers

import (
	"fmt"
	"math"
)

// S45 — Commodity Channel Index Reversion
//
// Research:   Donald Lambert (1980) Commodity Channel Index.
// Regime:     RANGING only
// Timeframes: 15m (CCI, period 20)
// Logic:      Uses the CCI() indicator on Candles15m. Requires CCI < -100
//             or > +100 sustained for >=3 bars (extreme reading), then the
//             current bar starting to revert back toward zero.
// Edge:       The 3-bar persistence requirement avoids fading single-bar CCI
//             spikes that are normal noise rather than genuine overextension.

type CommodityChannelIndexReversion struct{}

func (s *CommodityChannelIndexReversion) Name() string {
	return "Commodity_Channel_Index_Reversion"
}

func (s *CommodityChannelIndexReversion) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *CommodityChannelIndexReversion) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 24 {
		return NoSignal(name)
	}

	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	n := len(c15)

	cci0 := CCI(c15[:n-2], 20) // 3 bars ago
	cci1 := CCI(c15[:n-1], 20) // 2 bars ago
	cci2 := CCI(c15, 20)       // current (1 bar ago relative to "reverting" bar logic below)

	price := ctx.Price

	// Bullish: 3 consecutive bars at/below -100, then current reading
	// starting to revert upward (less negative than the prior bar).
	sustainedOversold := cci0 <= -100 && cci1 <= -100 && cci2 <= -100
	revertingUp := cci2 > cci1

	if sustainedOversold && revertingUp {
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
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: CCI sustained <-100 for 3 bars (%.0f, %.0f, %.0f), now reverting "+
					"toward zero, price=%.0f",
				cci0, cci1, cci2, price,
			),
		}
	}

	// Bearish: 3 consecutive bars at/above +100, then current reading
	// starting to revert downward.
	sustainedOverbought := cci0 >= 100 && cci1 >= 100 && cci2 >= 100
	revertingDown := cci2 < cci1

	if sustainedOverbought && revertingDown {
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
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: CCI sustained >+100 for 3 bars (%.0f, %.0f, %.0f), now reverting "+
					"toward zero, price=%.0f",
				cci0, cci1, cci2, price,
			),
		}
	}

	return NoSignal(name)
}
