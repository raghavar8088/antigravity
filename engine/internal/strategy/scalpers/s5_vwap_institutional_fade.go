package scalpers

import "fmt"

// S5 — VWAP Institutional Fade
//
// Regime:     RANGING only
// Timeframes: 5m (entry) + 15m (VWAP / structure)
// Logic:      Price extends aggressively away from session VWAP (>1.5×ATR).
//             Order book shows large wall on the extended side (institutional
//             supply/demand). RSI > 70 / < 30. Session filter: London or NY only.
//             Fade back toward VWAP. Confidence: 0.73.

type VWAPInstitutionalFade struct{}

func (s *VWAPInstitutionalFade) Name() string { return "VWAP_Institutional_Fade" }

func (s *VWAPInstitutionalFade) ValidRegimes() []Regime {
	return []Regime{RegimeRanging}
}

func (s *VWAPInstitutionalFade) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if ctx.SessionName != "LONDON" && ctx.SessionName != "NEW_YORK" {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 25 || len(ctx.Candles15m) < 20 {
		return NoSignal(name)
	}

	vwap := VWAP(ctx.Candles15m)
	if vwap == 0 {
		return NoSignal(name)
	}

	atr5m := ATR(ctx.Candles5m, 14)
	if atr5m == 0 {
		return NoSignal(name)
	}

	rsi5m := RSI(ctx.Candles5m, 14)
	price := ctx.Price
	ob := ctx.OrderBook
	deviation := price - vwap
	absDeviation := deviation
	if absDeviation < 0 {
		absDeviation = -absDeviation
	}
	extended := absDeviation > 1.5*atr5m

	if !extended {
		return NoSignal(name)
	}

	if deviation > 0 && rsi5m > 70 {
		askWallDominant := ob.AskWallSize > ob.BidWallSize*1.5
		if !askWallDominant {
			return NoSignal(name)
		}
		sl := price + 0.4*atr5m
		tp := vwap
		risk := sl - price
		reward := price - tp
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING/%s: price %.0f extended %.0f above VWAP=%.0f (>1.5×ATR=%.0f), "+
					"RSI=%.1f overbought, ask wall %.2f vs bid %.2f",
				ctx.SessionName, price, deviation, vwap, atr5m, rsi5m,
				ob.AskWallSize, ob.BidWallSize,
			),
		}
	}

	if deviation < 0 && rsi5m < 30 {
		bidWallDominant := ob.BidWallSize > ob.AskWallSize*1.5
		if !bidWallDominant {
			return NoSignal(name)
		}
		sl := price - 0.4*atr5m
		tp := vwap
		risk := price - sl
		reward := tp - price
		if reward < 2*risk || risk <= 0 {
			return NoSignal(name)
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: 0.73,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"RANGING/%s: price %.0f extended %.0f below VWAP=%.0f (>1.5×ATR=%.0f), "+
					"RSI=%.1f oversold, bid wall %.2f vs ask %.2f",
				ctx.SessionName, price, -deviation, vwap, atr5m, rsi5m,
				ob.BidWallSize, ob.AskWallSize,
			),
		}
	}

	return NoSignal(name)
}
