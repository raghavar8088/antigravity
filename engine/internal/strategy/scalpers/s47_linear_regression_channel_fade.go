package scalpers

import (
	"fmt"
	"math"
)

// S47 — Linear Regression Channel Fade
//
// Research:   Documented in Murphy, "Technical Analysis of the Financial
//             Markets" (1999) — linear regression channels as a statistical
//             mean-reversion envelope.
// Regime:     RANGING only
// Timeframes: 15m (LinearRegressionChannel, period 30)
// Logic:      Uses the LinearRegressionChannel() indicator on Candles15m
//             (period 30). When price reaches ValueAtEnd ± 2×StdErr, and the
//             current bar shows reversion back inside that band versus the
//             prior bar, fade back toward the regression line.
// Edge:       The regression line itself reflects the prevailing trend slope
//             (not a flat mean), so the channel adapts to drift rather than
//             fighting it outright.

type LinearRegressionChannelFade struct{}

func (s *LinearRegressionChannelFade) Name() string { return "Linear_Regression_Channel_Fade" }

func (s *LinearRegressionChannelFade) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *LinearRegressionChannelFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 32 {
		return NoSignal(name)
	}

	c15 := ctx.Candles15m
	n := len(c15)

	atr15m := ATR(c15, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	lrc := LinearRegressionChannel(c15, 30)
	lrcPrev := LinearRegressionChannel(c15[:n-1], 30)
	if lrc.StdErr == 0 || lrcPrev.StdErr == 0 {
		return NoSignal(name)
	}

	upper := lrc.ValueAtEnd + 2*lrc.StdErr
	lower := lrc.ValueAtEnd - 2*lrc.StdErr
	upperPrev := lrcPrev.ValueAtEnd + 2*lrcPrev.StdErr
	lowerPrev := lrcPrev.ValueAtEnd - 2*lrcPrev.StdErr

	prevClose := c15[n-2].Close
	price := ctx.Price

	// Bullish: prior bar was below lower band, now reverting back inside.
	prevOutsideLower := prevClose < lowerPrev
	nowInside := price >= lower && price <= upper

	if prevOutsideLower && nowInside {
		sl := math.Min(prevClose, price) - math.Max(1.0*atr15m, 0.003*price)
		slDist := price - sl
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := lrc.ValueAtEnd
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
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: prior 15m close=%.0f below regression lower band=%.0f "+
					"(value=%.0f, stdErr=%.2f), price=%.0f reverted back inside",
				prevClose, lowerPrev, lrc.ValueAtEnd, lrc.StdErr, price,
			),
		}
	}

	// Bearish: prior bar was above upper band, now reverting back inside.
	prevOutsideUpper := prevClose > upperPrev

	if prevOutsideUpper && nowInside {
		sl := math.Max(prevClose, price) + math.Max(1.0*atr15m, 0.003*price)
		slDist := sl - price
		if slDist < 0.003*price {
			return NoSignal(name)
		}
		risk := slDist
		tp := lrc.ValueAtEnd
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
			Confidence: 0.70,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING: prior 15m close=%.0f above regression upper band=%.0f "+
					"(value=%.0f, stdErr=%.2f), price=%.0f reverted back inside",
				prevClose, upperPrev, lrc.ValueAtEnd, lrc.StdErr, price,
			),
		}
	}

	return NoSignal(name)
}
