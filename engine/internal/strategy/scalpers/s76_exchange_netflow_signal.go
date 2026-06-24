package scalpers

import (
	"fmt"
	"math"
)

// S76 — Exchange_Netflow_Signal
//
// Research:   Glassnode research on exchange inflow/outflow as a leading
//
//	indicator — large net outflows from exchanges historically
//	precede accumulation/bullish continuation (coins moving to
//	cold storage / long-term holding), while net inflows precede
//	distribution/selling pressure.
//
// Regime:     TRENDING, RANGING
// Timeframes: 1h
// Logic:      This engine has no free real-time on-chain exchange
//
//	netflow API wired, so we substitute a PROXY using
//	ctx.OpenInterest trend: rising OI combined with a stable
//	(low-range) price over the recent window is read as an
//	"accumulation-like" pattern (positions building without
//	aggressive directional price impact — analogous to outflow
//	accumulation), per the spec's own documented fallback
//	guidance. Compares the last 2 OIHistory readings for the OI
//	trend leg and Candles1h price range stability for the second
//	leg.
//
// Edge:       (As a proxy) sustained OI build without commensurate price
//
//	range expansion suggests quiet accumulation rather than
//	aggressive directional speculation — a reasonable stand-in
//	pattern shape for the real on-chain netflow signal until a
//	real feed is wired.
//
// Data dependency: PROXY — substituting ctx.OpenInterest / ctx.OIHistory
//
//	(already live) for the absent real on-chain exchange netflow
//	feed, exactly as documented in the spec's fallback guidance.
//	No real exchange wallet flow data is used; this is NOT a
//	substitute for true on-chain netflow once such a feed exists.
type ExchangeNetflowSignal struct{}

func (s *ExchangeNetflowSignal) Name() string { return "Exchange_Netflow_Signal" }

func (s *ExchangeNetflowSignal) ValidRegimes() []Regime {
	return []Regime{RegimeTrending, RegimeRanging}
}

const (
	enfOIBuildThreshold    = 0.02  // +2% OI build between last 2 OIHistory readings
	enfPriceStabilityRange = 0.015 // price range over lookback considered "stable" (1.5%)
)

func (s *ExchangeNetflowSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeTrending && ctx.Regime != RegimeRanging {
		return NoSignal(name)
	}
	if len(ctx.Candles1h) < 15 {
		return NoSignal(name)
	}
	if len(ctx.OIHistory) < 2 {
		return NoSignal(name)
	}

	oiHist := ctx.OIHistory
	latest := oiHist[len(oiHist)-1]
	prev := oiHist[len(oiHist)-2]
	if prev == 0 {
		return NoSignal(name)
	}
	oiChange := (latest - prev) / prev

	// 3-bar confirmation on price stability: use the last 4 1h candles (3
	// completed intervals) to assess range stability, not a single bar.
	n := len(ctx.Candles1h)
	tail := ctx.Candles1h[n-4:]
	high := tail[0].High
	low := tail[0].Low
	for _, c := range tail[1:] {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
	}
	price := ctx.Price
	if price <= 0 || low <= 0 {
		return NoSignal(name)
	}
	rangePct := (high - low) / price
	priceStable := rangePct <= enfPriceStabilityRange

	atr1h := ATR(ctx.Candles1h, 14)
	if atr1h == 0 {
		return NoSignal(name)
	}
	slDist := math.Max(1.0*atr1h, 0.003*price)

	ema9 := EMA(ctx.Candles1h, 9)
	ema21 := EMA(ctx.Candles1h, 21)

	// Accumulation-like pattern: OI building, price range stable.
	if oiChange >= enfOIBuildThreshold && priceStable {
		// Direction inferred from the (slight) underlying EMA bias to avoid
		// a directionless entry — accumulation precedes continuation in
		// whichever direction the quiet build is leaning.
		if ema9 >= ema21 {
			sl := price - slDist
			tp1 := price + 2.0*slDist
			tp2 := price + 3.0*slDist
			if sl >= price || tp1 <= price {
				return NoSignal(name)
			}
			return Signal{
				Strategy:    name,
				Direction:   DirectionLong,
				Confidence:  0.58,
				StopLoss:    sl,
				TakeProfit:  tp1,
				TakeProfit2: tp2,
				Reason: fmt.Sprintf(
					"netflow proxy LONG: OI build %.2f%% with stable price range %.2f%% "+
						"(accumulation-like pattern), EMA9>=EMA21",
					oiChange*100, rangePct*100,
				),
			}
		}
		sl := price + slDist
		tp1 := price - 2.0*slDist
		tp2 := price - 3.0*slDist
		if sl <= price || tp1 >= price {
			return NoSignal(name)
		}
		return Signal{
			Strategy:    name,
			Direction:   DirectionShort,
			Confidence:  0.58,
			StopLoss:    sl,
			TakeProfit:  tp1,
			TakeProfit2: tp2,
			Reason: fmt.Sprintf(
				"netflow proxy SHORT: OI build %.2f%% with stable price range %.2f%% "+
					"(accumulation-like pattern), EMA9<EMA21",
				oiChange*100, rangePct*100,
			),
		}
	}

	return NoSignal(name)
}
