package scalpers

import (
	"fmt"
	"math"
)

// S74 — Taker_Buy_Sell_Volume_Ratio
//
// Research:   Order-flow-imbalance-as-price-impact-predictor academic
//
//	literature (microstructure research showing aggressive taker
//	flow imbalance carries short-horizon directional predictive
//	power beyond simple price momentum).
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:      Uses ctx.TakerBuySellRatio / ctx.TakerBuySellRatioHistory.
//
//	ratio > 1.3 sustained for 3+ readings in history → LONG
//	(aggressive buying pressure persistent, not a single spike);
//	ratio < 0.7 sustained for 3+ readings → SHORT.
//
// Edge:       Sustained (not single-reading) taker imbalance reflects
//
//	genuine directional aggression rather than a single large
//	order, historically more persistent/tradeable.
//
// Data dependency: PENDING — ctx.TakerBuySellRatio / TakerBuySellRatioHistory
//
//	are defined in MarketContext but the live fetcher is NOT yet
//	wired (ctx.PositioningFeedPopulated reads false in production
//	today). This strategy checks PositioningFeedPopulated first
//	and returns NoSignal cleanly when false. Intended source for
//	whoever wires this next:
//	GET https://fapi.binance.com/futures/data/takerlongshortRatio
//	(Binance Futures public data API, no auth required, ~5m
//	cadence matches ctx.TakerBuySellRatioHistory's documented
//	cadence in types.go).
type TakerBuySellVolumeRatio struct{}

func (s *TakerBuySellVolumeRatio) Name() string { return "Taker_Buy_Sell_Volume_Ratio" }

func (s *TakerBuySellVolumeRatio) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	tbsrLongTrigger  = 1.3
	tbsrShortTrigger = 0.7
)

func (s *TakerBuySellVolumeRatio) Evaluate(ctx MarketContext) Signal {
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
	// 3-bar confirmation: need at least 3 history readings, all sustained
	// beyond the trigger, not just the latest snapshot.
	if len(ctx.TakerBuySellRatioHistory) < 3 {
		return NoSignal(name)
	}

	hist := ctx.TakerBuySellRatioHistory
	last3 := hist[len(hist)-3:]
	ratio := ctx.TakerBuySellRatio
	if ratio <= 0 {
		return NoSignal(name)
	}

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	price := ctx.Price
	slDist := math.Max(1.0*atr1h, 0.003*price)

	sustainedBuy := last3[0] > tbsrLongTrigger && last3[1] > tbsrLongTrigger && last3[2] > tbsrLongTrigger
	sustainedSell := last3[0] < tbsrShortTrigger && last3[1] < tbsrShortTrigger && last3[2] < tbsrShortTrigger

	if ratio > tbsrLongTrigger && sustainedBuy {
		sl := price - slDist
		tp1 := price + 2.0*slDist
		tp2 := price + 3.0*slDist
		if sl >= price || tp1 <= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionLong,
			Confidence:  0.66,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"taker buy/sell ratio LONG: ratio=%.2f sustained >%.1f over last 3 readings (%.2f,%.2f,%.2f)",
				ratio, tbsrLongTrigger, last3[0], last3[1], last3[2],
			),
		}
	}

	if ratio < tbsrShortTrigger && sustainedSell {
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.66,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"taker buy/sell ratio SHORT: ratio=%.2f sustained <%.1f over last 3 readings (%.2f,%.2f,%.2f)",
				ratio, tbsrShortTrigger, last3[0], last3[1], last3[2],
			),
		}
	}

	return NoSignal(name)
}
