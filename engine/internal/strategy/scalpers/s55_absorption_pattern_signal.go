package scalpers

import (
	"fmt"
	"math"
)

// S55 — Absorption Pattern Signal
//
// Regime:     RANGING, VOLATILE
// Timeframes: 5m (price/CVD comparison) + 15m (ATR/SL reference)
// Research:   Steidlmayer Market Profile theory (1984) — "absorption" occurs
//             when heavy aggressive order flow in one direction fails to move
//             price proportionally, indicating a large passive counterparty
//             (resting bids/offers) is absorbing the flow. Once the
//             aggressive side exhausts, price tends to reverse toward the
//             absorbing side.
// Logic:      Heavy sell-side CVD flow (ctx.CVD significantly below
//             ctx.CVDPrev) while price over the last 3 Candles5m bars has NOT
//             moved down proportionally (i.e. the realized price change is
//             much smaller in magnitude than what the CVD delta would imply)
//             signals bids are absorbing the selling -> LONG. Symmetric case:
//             heavy buy-side CVD flow with price not moving up proportionally
//             signals offers absorbing the buying -> SHORT.
// Edge:       Identifies hidden passive liquidity stepping in front of
//             aggressive flow before the resulting squeeze/reversal is
//             visible in price action alone.

type AbsorptionPatternSignal struct{}

func (s *AbsorptionPatternSignal) Name() string { return "Absorption_Pattern_Signal" }

func (s *AbsorptionPatternSignal) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

func (s *AbsorptionPatternSignal) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles5m) < 4 || len(ctx.Candles15m) < 15 {
		return NoSignal(name)
	}

	price := ctx.Price
	if price <= 0 {
		return NoSignal(name)
	}
	atr15m := ATR(ctx.Candles15m, 14)
	if atr15m == 0 {
		return NoSignal(name)
	}

	c5 := ctx.Candles5m
	n := len(c5)
	// Last 3 CLOSED bars (excluding the still-forming one at n-1).
	if n < 5 {
		return NoSignal(name)
	}
	threeBarsAgo := c5[n-4]
	lastClosed := c5[n-2]
	priceChange3Bar := lastClosed.Close - threeBarsAgo.Close
	priceChangePct := 0.0
	if threeBarsAgo.Close > 0 {
		priceChangePct = math.Abs(priceChange3Bar) / threeBarsAgo.Close
	}

	cvd := ctx.CVD
	cvdPrev := ctx.CVDPrev
	cvdDelta := cvd - cvdPrev
	if cvdPrev == 0 && cvd == 0 {
		return NoSignal(name)
	}
	cvdScale := math.Abs(cvdPrev) + math.Abs(cvd) + 1e-9
	cvdDeltaPct := math.Abs(cvdDelta) / cvdScale

	// "Heavy flow" gate: CVD delta must be a meaningfully large fraction of
	// its own scale (mirrors the toxicity-style proxy used elsewhere — heavy,
	// not noise).
	const heavyFlowThreshold = 0.4
	// "Not moving proportionally" gate: realized price move stays small
	// (<0.15%) despite the heavy CVD delta — the disproportion is what
	// signals absorption rather than a genuine breakout.
	const smallPriceMoveThreshold = 0.0015

	heavySelling := cvdDelta < 0 && cvdDeltaPct >= heavyFlowThreshold
	heavyBuying := cvdDelta > 0 && cvdDeltaPct >= heavyFlowThreshold
	priceNotMoving := priceChangePct < smallPriceMoveThreshold

	slDistMin := math.Max(1.0*atr15m, 0.003*price)

	// Heavy selling absorbed by bids -> expect bounce -> LONG.
	if heavySelling && priceNotMoving {
		sl := price - slDistMin
		tp := price + 2.0*slDistMin
		risk := price - sl
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
				"Absorption LONG: heavy sell CVD delta=%.0f (%.0f%% of scale) but 3-bar price change only %.3f%% — bids absorbing",
				cvdDelta, cvdDeltaPct*100, priceChangePct*100,
			),
		}
	}

	// Heavy buying absorbed by offers -> expect reversal down -> SHORT.
	if heavyBuying && priceNotMoving {
		sl := price + slDistMin
		tp := price - 2.0*slDistMin
		risk := sl - price
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
				"Absorption SHORT: heavy buy CVD delta=%.0f (%.0f%% of scale) but 3-bar price change only %.3f%% — offers absorbing",
				cvdDelta, cvdDeltaPct*100, priceChangePct*100,
			),
		}
	}

	return NoSignal(name)
}
