package scalpers

import (
	"fmt"
	"math"
)

// S73 — Long_Short_Ratio_Contrarian
//
// Research:   Binance public topLongShortAccountRatio research and academic
//
//	retail-positioning-as-contrarian-indicator papers — extreme
//	skew in retail account long/short ratios has historically
//	preceded mean-reversion as the crowded side gets squeezed.
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:      Uses ctx.TopTraderLongShortRatio. ratio > 1.5 (more longs than
//
//	shorts among top traders) → contrarian SHORT bias; ratio < 0.7
//	→ contrarian LONG bias. Confirmed by FundingRate agreeing with
//	the same crowded direction (positive funding for the short
//	case, negative for the long case) to avoid fading a ratio
//	extreme that funding doesn't corroborate.
//
// Edge:       Combining two independent crowding measurements (account-ratio
//
//	positioning + funding) reduces false positives from either
//	signal alone.
//
// Data dependency: PENDING — ctx.TopTraderLongShortRatio /
//
//	ctx.TopTraderLSRatioHistory are defined in MarketContext but
//	the live fetcher is NOT yet wired (ctx.PositioningFeedPopulated
//	will read false in production today). This strategy checks
//	PositioningFeedPopulated first and returns NoSignal cleanly
//	when false. Intended source for whoever wires this next:
//	GET https://fapi.binance.com/futures/data/topLongShortAccountRatio
//	(Binance Futures public data API, no auth required, ~15m
//	cadence matches ctx.TopTraderLSRatioHistory's documented
//	cadence in types.go).
type LongShortRatioContrarian struct{}

func (s *LongShortRatioContrarian) Name() string { return "Long_Short_Ratio_Contrarian" }

func (s *LongShortRatioContrarian) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	lsrShortTrigger = 1.5
	lsrLongTrigger  = 0.7
	lsrFundingShort = 0.0001 // raw decimal, must agree (positive) for SHORT confirmation
	lsrFundingLong  = -0.0001
)

func (s *LongShortRatioContrarian) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}
	if !ctx.PositioningFeedPopulated || !ctx.PositioningFeedHealthy {
		// Feed not wired yet — see header "Data dependency: PENDING".
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}

	ratio := ctx.TopTraderLongShortRatio
	if ratio <= 0 {
		return NoSignal(name)
	}

	// 3-bar confirmation: require history to show the extreme isn't a
	// single-reading blip.
	if len(ctx.TopTraderLSRatioHistory) < 3 {
		return NoSignal(name)
	}
	hist := ctx.TopTraderLSRatioHistory
	last3 := hist[len(hist)-3:]

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := math.Max(1.0*atr1h, 0.003*price)

	allAboveShortTrigger := last3[0] > lsrShortTrigger && last3[1] > lsrShortTrigger && last3[2] > lsrShortTrigger
	allBelowLongTrigger := last3[0] < lsrLongTrigger && last3[1] < lsrLongTrigger && last3[2] < lsrLongTrigger

	if ratio > lsrShortTrigger && allAboveShortTrigger && ctx.FundingRate > lsrFundingShort {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.68,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"top-trader L/S ratio contrarian SHORT: ratio=%.2f (>%.1f, 3-bar confirmed), "+
					"funding=%.5f agrees (crowded long)",
				ratio, lsrShortTrigger, ctx.FundingRate,
			),
		}
	}

	if ratio < lsrLongTrigger && allBelowLongTrigger && ctx.FundingRate < lsrFundingLong {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.68,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"top-trader L/S ratio contrarian LONG: ratio=%.2f (<%.1f, 3-bar confirmed), "+
					"funding=%.5f agrees (crowded short)",
				ratio, lsrLongTrigger, ctx.FundingRate,
			),
		}
	}

	return NoSignal(name)
}
