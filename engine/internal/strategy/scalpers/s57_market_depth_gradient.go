package scalpers

import (
	"fmt"
	"math"
)

// S57 — Market Depth Gradient
//
// Regime:     RANGING, VOLATILE
// Timeframes: 1m (order book gradient) + 15m (ATR/SL reference)
// Research:   Cont, Kukanov & Stoikov (2014) "The Price Impact of Order Book
//             Events" — the SHAPE of the order book (how depth changes across
//             distance from mid, not just total depth) predicts price impact:
//             a steep/front-loaded book (most liquidity concentrated very
//             close to mid, thin beyond) absorbs less total flow before
//             price must move, so aggressive flow into that side accelerates
//             through it.
// Logic:      gradient_bid = BidDepth01pct - BidDepth1pct (positive and large
//             = front-loaded/thin-behind bid side); gradient_ask =
//             AskDepth01pct - AskDepth1pct symmetric for asks. The side with
//             the LARGER gradient is "steeper" (thinner deep liquidity behind
//             the touch). When ctx.CVD is currently pushing INTO that thin
//             side (CVD falling = selling pressure into a steep/thin bid
//             side; CVD rising = buying pressure into a steep/thin ask side),
//             expect acceleration through the thin side as resting depth
//             runs out faster than usual. Requires OrderBook.IsPopulated().
// Edge:       Anticipates price acceleration (not just direction) by reading
//             order book shape rather than only top-of-book size.

type MarketDepthGradient struct{}

func (s *MarketDepthGradient) Name() string { return "Market_Depth_Gradient" }

func (s *MarketDepthGradient) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

func (s *MarketDepthGradient) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}

	ob := ctx.OrderBook
	if !ob.IsPopulated() {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 15 {
		return NoSignal(name)
	}
	if ob.BidDepth01pct <= 0 || ob.BidDepth1pct <= 0 || ob.AskDepth01pct <= 0 || ob.AskDepth1pct <= 0 {
		// Depth-tier data not actually populated — cannot compute a gradient.
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

	gradientBid := ob.BidDepth01pct - ob.BidDepth1pct
	gradientAsk := ob.AskDepth01pct - ob.AskDepth1pct

	// Normalize gradients relative to their own near-touch depth so we
	// compare "steepness ratio" rather than raw absolute size (avoids one
	// side's larger absolute liquidity dominating purely on size).
	normBid := 0.0
	if ob.BidDepth01pct > 0 {
		normBid = gradientBid / ob.BidDepth01pct
	}
	normAsk := 0.0
	if ob.AskDepth01pct > 0 {
		normAsk = gradientAsk / ob.AskDepth01pct
	}

	const steepnessThreshold = 0.3 // bid/ask side must be meaningfully front-loaded
	slDistMin := math.Max(1.0*atr15m, 0.003*price)

	bidSteeper := normBid > normAsk && normBid >= steepnessThreshold
	askSteeper := normAsk > normBid && normAsk >= steepnessThreshold

	cvdSelling := ctx.CVD < ctx.CVDPrev
	cvdBuying := ctx.CVD > ctx.CVDPrev

	// Selling pressure pushing into a thin/steep bid side -> expect
	// acceleration DOWN through thin bids -> SHORT.
	if bidSteeper && cvdSelling {
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
			Confidence: 0.68,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Market depth gradient SHORT: bid side steeper (normGradBid=%.2f > normGradAsk=%.2f), "+
					"sell flow pushing into thin bid depth (CVD %.0f->%.0f)",
				normBid, normAsk, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	// Buying pressure pushing into a thin/steep ask side -> expect
	// acceleration UP through thin asks -> LONG.
	if askSteeper && cvdBuying {
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
			Confidence: 0.68,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Market depth gradient LONG: ask side steeper (normGradAsk=%.2f > normGradBid=%.2f), "+
					"buy flow pushing into thin ask depth (CVD %.0f->%.0f)",
				normAsk, normBid, ctx.CVDPrev, ctx.CVD,
			),
		}
	}

	return NoSignal(name)
}
