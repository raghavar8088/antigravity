package scalpers

import (
	"fmt"
	"math"
)

// S37 — Ichimoku Cloud Breakout
//
// Citation:   Goichi Hosoda, Ichimoku Kinko Hyo ("one glance equilibrium
//             chart"), developed late 1930s-1960s, published 1969 — a
//             multi-component trend system combining the Tenkan/Kijun lines
//             and the Senkou Span A/B cloud.
// Regime:     TRENDING only
// Timeframes: 1h
// Logic:      Compute Ichimoku() on Candles1h. A bullish breakout occurs
//             when price closes above the cloud (max of SpanA, SpanB),
//             confirmed by Tenkan > Kijun. A bearish breakdown occurs when
//             price closes below the cloud (min of SpanA, SpanB), confirmed
//             by Tenkan < Kijun.
// Edge:       The cloud acts as a dynamic support/resistance zone; trading
//             only confirmed breaks with Tenkan/Kijun alignment filters out
//             false breaks while price is still inside or hugging the
//             cloud.

type IchimokuCloudBreakout struct{}

func (s *IchimokuCloudBreakout) Name() string { return "Ichimoku_Cloud_Breakout" }

func (s *IchimokuCloudBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *IchimokuCloudBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 52 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	ichi := Ichimoku(ctx.Candles1h)
	if ichi.SpanA == 0 && ichi.SpanB == 0 {
		return NoSignal(name)
	}

	cloudTop := math.Max(ichi.SpanA, ichi.SpanB)
	cloudBottom := math.Min(ichi.SpanA, ichi.SpanB)

	price := ctx.Price
	bullBreak := price > cloudTop && ichi.Tenkan > ichi.Kijun
	bearBreak := price < cloudBottom && ichi.Tenkan < ichi.Kijun

	if bullBreak {
		sl := cloudTop - math.Max(1.0*atr1h, 0.003*price)
		if sl >= price {
			sl = price - math.Max(1.0*atr1h, 0.003*price)
		}
		slDist := price - sl
		if slDist < math.Max(1.0*atr1h, 0.003*price) {
			return NoSignal(name)
		}
		tp := price + 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Ichimoku bullish cloud breakout: price %.0f > cloud top %.0f, "+
					"Tenkan=%.0f>Kijun=%.0f",
				price, cloudTop, ichi.Tenkan, ichi.Kijun,
			),
		}
	}

	if bearBreak {
		sl := cloudBottom + math.Max(1.0*atr1h, 0.003*price)
		if sl <= price {
			sl = price + math.Max(1.0*atr1h, 0.003*price)
		}
		slDist := sl - price
		if slDist < math.Max(1.0*atr1h, 0.003*price) {
			return NoSignal(name)
		}
		tp := price - 2.0*slDist
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Ichimoku bearish cloud breakdown: price %.0f < cloud bottom %.0f, "+
					"Tenkan=%.0f<Kijun=%.0f",
				price, cloudBottom, ichi.Tenkan, ichi.Kijun,
			),
		}
	}

	return NoSignal(name)
}
