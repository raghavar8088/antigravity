package scalpers

import (
	"fmt"
	"math"
)

// S52 — Bid/Ask Spread Regime
//
// Regime:     RANGING, VOLATILE
// Timeframes: 1m (spread/imbalance read) + 15m (ATR/SL reference)
// Research:   Glosten & Milgrom (1985) "Bid, Ask and Transaction Prices in a
//             Specialist Market" — market makers widen spreads when they
//             suspect informed trading; a widening effective spread combined
//             with a directional order-book skew signals the dealer is
//             repricing away from informed flow, and price tends to follow
//             that skew direction as liquidity is pulled from one side.
// Logic:      Ideal data is EffectiveSpread widening vs EffectiveSpreadAvg
//             (requires MicrostructurePopulated). That feed is not yet wired
//             live in production, so this strategy falls back to
//             OrderBook.IsPopulated() + ob.Imbalance directional reasoning:
//             when liquidity is visibly imbalanced (one side's wall is much
//             thinner — a proxy for "spread is effectively wider on that
//             side" since a thin wall means less resting depth to absorb
//             flow before price gaps), trade in the direction of the heavier
//             side. Documented fallback per task spec.
// Edge:       Detects dealer-side liquidity withdrawal before it shows up as
//             realized price movement, entering ahead of the repricing.

type BidAskSpreadRegime struct{}

func (s *BidAskSpreadRegime) Name() string { return "Bid_Ask_Spread_Regime" }

func (s *BidAskSpreadRegime) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

func (s *BidAskSpreadRegime) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 15 {
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

	var spreadWidening bool
	var detail string
	const imbalanceThreshold = 0.35

	if ctx.MicrostructurePopulated {
		// Ideal path: live effective-spread feed.
		spreadWidening = ctx.EffectiveSpreadAvg > 0 && ctx.EffectiveSpread > 1.3*ctx.EffectiveSpreadAvg
		detail = fmt.Sprintf("EffectiveSpread=%.4f > 1.3x avg=%.4f", ctx.EffectiveSpread, ctx.EffectiveSpreadAvg)
	} else {
		// FALLBACK: MicrostructurePopulated==false in production today — use
		// OrderBook population + imbalance as documented in spec.
		ob := ctx.OrderBook
		if !ob.IsPopulated() {
			return NoSignal(name)
		}
		spreadWidening = math.Abs(ob.Imbalance) >= imbalanceThreshold
		detail = fmt.Sprintf("FALLBACK: OrderBook.Imbalance=%.2f (no live spread feed)", ob.Imbalance)
	}

	if !spreadWidening {
		return NoSignal(name)
	}

	ob := ctx.OrderBook
	if !ob.IsPopulated() {
		return NoSignal(name)
	}

	slDistMin := math.Max(1.0*atr15m, 0.003*price)

	if ob.Imbalance >= imbalanceThreshold {
		sl := price - slDistMin
		tp := price + 2.0*slDistMin
		risk := price - sl
		reward := tp - price
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.64
		if ctx.MicrostructurePopulated {
			conf = 0.71
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionLong,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Bid/ask spread regime LONG: %s, bid-side imbalance=%.2f",
				detail, ob.Imbalance,
			),
		}
	}

	if ob.Imbalance <= -imbalanceThreshold {
		sl := price + slDistMin
		tp := price - 2.0*slDistMin
		risk := sl - price
		reward := price - tp
		if risk <= 0 || reward/risk < 2.0 {
			return NoSignal(name)
		}
		conf := 0.64
		if ctx.MicrostructurePopulated {
			conf = 0.71
		}
		return Signal{
			Strategy:   name,
			Direction:  DirectionShort,
			Confidence: conf,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Bid/ask spread regime SHORT: %s, ask-side imbalance=%.2f",
				detail, ob.Imbalance,
			),
		}
	}

	return NoSignal(name)
}
