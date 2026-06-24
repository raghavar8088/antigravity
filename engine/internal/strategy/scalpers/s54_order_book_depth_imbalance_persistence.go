package scalpers

import (
	"fmt"
	"math"
)

// S54 — Order Book Depth Imbalance Persistence
//
// Regime:     RANGING, VOLATILE
// Timeframes: 1m (order book snapshot) + 15m (ATR/SL reference)
// Research:   Cartea, Jaimungal & Penalva (2015) "Algorithmic and
//             High-Frequency Trading" — depth-weighted order book imbalance,
//             with closer-to-mid liquidity weighted more heavily than far
//             liquidity, is a stronger predictor of near-term price drift
//             than a flat/raw imbalance measure because near-touch depth is
//             what actually absorbs the next wave of aggressive orders.
// Logic:      Uses the WeightedOBImbalance() indicator (indicators_s30_s79.go)
//             — DIFFERENT from S17 (Orderbook_Imbalance_Persistence), which
//             uses the raw ob.Imbalance field plus flat multi-level agreement.
//             WeightedOBImbalance applies tier weights (0.6/0.3/0.1 for
//             0.1%/0.5%/1% depth bands) so near-touch liquidity dominates the
//             score. This engine does not store a ring buffer of 5+
//             consecutive raw order-book snapshots, so true cross-snapshot
//             "persistence" cannot be measured directly here — SIMPLIFICATION:
//             persistence is approximated by requiring the weighted imbalance
//             sign to agree with the recent CVDHistory direction trend (a
//             cheap proxy for "this skew has been building, not a single
//             flickering snapshot" — if resting book skew and realized trade
//             flow direction have been moving together, that is more likely a
//             durable feature than coincidence). Documented explicitly.
// Edge:       Captures near-touch order book pressure that is corroborated by
//             a consistent recent flow trend, filtering out single-snapshot
//             noise without needing true multi-snapshot history.

type OrderBookDepthImbalancePersistence struct{}

func (s *OrderBookDepthImbalancePersistence) Name() string {
	return "Order_Book_Depth_Imbalance_Persistence"
}

func (s *OrderBookDepthImbalancePersistence) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

func (s *OrderBookDepthImbalancePersistence) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime != RegimeRanging && ctx.Regime != RegimeVolatile {
		return NoSignal(name)
	}

	ob := ctx.OrderBook
	if !ob.IsPopulated() {
		return NoSignal(name)
	}
	if len(ctx.Candles15m) < 15 || len(ctx.CVDHistory) < 3 {
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

	weighted := WeightedOBImbalance(ob)
	const magnitudeThreshold = 0.3
	if math.Abs(weighted) < magnitudeThreshold {
		return NoSignal(name)
	}

	// Persistence proxy: CVDHistory must show a stable 3-reading trend in the
	// SAME direction as the weighted imbalance sign (see header comment).
	nh := len(ctx.CVDHistory)
	hist := ctx.CVDHistory
	cvdRising := hist[nh-2] > hist[nh-3] && hist[nh-1] > hist[nh-2]
	cvdFalling := hist[nh-2] < hist[nh-3] && hist[nh-1] < hist[nh-2]

	slDistMin := math.Max(1.0*atr15m, 0.003*price)

	if weighted >= magnitudeThreshold && cvdRising {
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
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Depth-weighted OB imbalance LONG: weighted=%.2f (>=0.3), 3-bar CVD rising corroborates persistence",
				weighted,
			),
		}
	}

	if weighted <= -magnitudeThreshold && cvdFalling {
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
			Confidence: 0.69,
			StopLoss:   sl,
			TakeProfit: tp,
			Reason: fmt.Sprintf(
				"Depth-weighted OB imbalance SHORT: weighted=%.2f (<=-0.3), 3-bar CVD falling corroborates persistence",
				weighted,
			),
		}
	}

	return NoSignal(name)
}
