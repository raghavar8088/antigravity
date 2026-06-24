package scalpers

import (
	"fmt"
	"math"
)

// S38 — Donchian Channel Breakout
//
// Citation:   Richard Donchian, originator of the 20-day breakout channel
//             system later popularized as the "Turtle Trading" rules
//             (Dennis & Eckhardt, 1980s).
// Regime:     TRENDING only
// Timeframes: 1h
// Logic:      Compute DonchianChannel(period=20) on Candles1h. A breakout
//             above the channel high, confirmed by ctx.CVD > ctx.CVDPrev
//             (buying pressure), triggers a long. A breakdown below the
//             channel low, confirmed by ctx.CVD < ctx.CVDPrev (selling
//             pressure), triggers a short.
// Edge:       Classic trend-following breakout system; the CVD confirmation
//             leg adds an order-flow filter the original Turtle rules
//             lacked, screening out breakouts unsupported by real buying or
//             selling pressure.

type DonchianChannelBreakout struct{}

func (s *DonchianChannelBreakout) Name() string { return "Donchian_Channel_Breakout" }

func (s *DonchianChannelBreakout) ValidRegimes() []Regime {
	return []Regime{RegimeTrending}
}

func (s *DonchianChannelBreakout) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 21 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}

	dcHigh, dcLow := DonchianChannel(ctx.Candles1h, 20)
	if dcHigh == 0 && dcLow == 0 {
		return NoSignal(name)
	}

	price := ctx.Price
	cvdBuying := ctx.CVD > ctx.CVDPrev
	cvdSelling := ctx.CVD < ctx.CVDPrev

	if price > dcHigh && cvdBuying {
		sl := dcHigh - math.Max(1.0*atr1h, 0.003*price)
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
				"Donchian breakout long: price %.0f > 20-period high %.0f, "+
					"CVD rising (%.0f→%.0f)",
				price, dcHigh, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	if price < dcLow && cvdSelling {
		sl := dcLow + math.Max(1.0*atr1h, 0.003*price)
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
				"Donchian breakdown short: price %.0f < 20-period low %.0f, "+
					"CVD falling (%.0f→%.0f)",
				price, dcLow, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	return NoSignal(name)
}
