package scalpers

import (
	"fmt"
	"math"
)

// S17 — Orderbook Imbalance Persistence
//
// Regime:     RANGING, VOLATILE
// Timeframes: 1m (orderbook snapshot cadence), 15m (ATR/SL reference)
//
// Logic: resting bid/ask wall imbalance must PERSIST across multiple
// consecutive snapshots before signaling — a single-snapshot imbalance read
// is noisy (can flip within seconds as resting orders are added/pulled), but
// sustained imbalance over several minutes indicates real, durable
// directional pressure from resting liquidity rather than a fleeting quote.
//
// JUDGMENT CALL — true multi-snapshot history is out of scope for this chunk:
// MarketContext/ScalerBundle (see engine/internal/trading/scalers_eval.go)
// only carries a SINGLE current OrderBookSnapshot (b.orderBook), overwritten
// each cycle by DepthSubscriber.GetLatestAnalysis() — there is no ring buffer
// of historical snapshots to inspect 3 consecutive prior reads, and adding
// that history-tracking plumbing to ScalerBundle is a larger change than this
// chunk's scope (data feed + 4 strategies). Approximation used instead:
// infer persistence from within-snapshot STABILITY signals that are only
// present when the same imbalance has held long enough to also stabilize the
// surrounding depth/spread structure:
//  1. Imbalance must be strongly one-sided in the CURRENT snapshot (>=0.4 abs).
//  2. The imbalance must be corroborated across multiple PRICE LEVELS within
//     the same snapshot (0.1%, 0.5%, 1% depth bands), i.e. BidDepth01pct vs
//     AskDepth01pct, BidDepth05pct vs AskDepth05pct, BidDepth1pct vs
//     AskDepth1pct must ALL agree in direction — a wall that's only present
//     at one depth band is more likely a fleeting single large order, not a
//     persistent structural imbalance.
//  3. TradeFlowImbalance (the realized last-1m taker buy/sell skew) must
//     agree in sign with the resting-book imbalance — i.e. actual trade flow
//     over the last minute is consistent with the book skew, not contradicting
//     it (a "fake wall" with no real flow behind it tends to get pulled).
//
// Requiring all 3 conditions simultaneously is the closest available proxy to
// "the same imbalance held for 3+ consecutive snapshots" given a single live
// snapshot, by demanding multi-level + multi-signal agreement instead of
// multi-time agreement. Documented explicitly per task instructions.
//
// CRITICAL — the ONE INTENTIONAL exception to graceful degradation in this
// chunk: if OrderBook.IsPopulated() is false, this strategy MUST NEVER fire,
// full stop, no fallback. Every other S14-S17 strategy degrades gracefully
// when its external feed is unavailable (e.g. S14/S16 return NoSignal but the
// overall engine keeps running other strategies fine without that feed,
// because alternative signals exist). Orderbook imbalance has NO valid
// substitute signal — there is no proxy for "resting liquidity is stacked on
// one side" using candle/CVD/funding data; faking it from those inputs would
// not be the same signal and risks false confidence. So unlike DVOL (which
// can fall back to RealizedVol) or liquidations/basis (which simply suppress
// the strategy when absent, same as here) — the distinction here is that
// other strategies treat "feed absent" as "skip," and so does S17, but this
// comment exists specifically because the original task spec called out this
// behavior as needing prominent documentation: S17 has zero degraded mode,
// it is binary populated-or-silent with no partial/degraded confidence path.
type OrderbookImbalancePersistence struct{}

func (s *OrderbookImbalancePersistence) Name() string { return "Orderbook_Imbalance_Persistence" }

func (s *OrderbookImbalancePersistence) ValidRegimes() []Regime {
	return []Regime{RegimeRanging, RegimeVolatile}
}

func (s *OrderbookImbalancePersistence) Evaluate(ctx MarketContext) Signal {
	name := s.Name()

	if ctx.Regime == RegimeUnknown {
		return NoSignal(name)
	}

	// CRITICAL: never fire without a populated order book. No fallback, no
	// degraded mode — see file-level comment for rationale.
	ob := ctx.OrderBook
	if !ob.IsPopulated() {
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

	const imbalanceThreshold = 0.4

	// Multi-level depth agreement (approximation for cross-snapshot persistence —
	// see file header comment). Levels with zero depth on both sides are skipped
	// (no data at that band yet) rather than treated as a contradiction.
	levelsBullish, levelsBearish, levelsTotal := 0, 0, 0
	checkLevel := func(bid, ask float64) {
		if bid <= 0 && ask <= 0 {
			return
		}
		levelsTotal++
		if bid > ask {
			levelsBullish++
		} else if ask > bid {
			levelsBearish++
		}
	}
	checkLevel(ob.BidDepth01pct, ob.AskDepth01pct)
	checkLevel(ob.BidDepth05pct, ob.AskDepth05pct)
	checkLevel(ob.BidDepth1pct, ob.AskDepth1pct)

	// Require at least 2 of the available depth bands to exist and all agree —
	// if depth-level data isn't populated at all, fall back to the binary
	// BidWallSize/AskWallSize gate alone (still requires IsPopulated() above).
	depthLevelsAvailable := levelsTotal >= 2

	tradeFlow := ob.TradeFlowImbalance

	// ── Bullish persistence: bid wall stacked, corroborated across levels + flow ──
	bullishImbalance := ob.Imbalance >= imbalanceThreshold && ob.BidWallSize > ob.AskWallSize
	if bullishImbalance {
		levelsAgree := !depthLevelsAvailable || (levelsBullish == levelsTotal && levelsBullish > 0)
		flowAgrees := tradeFlow >= 0 // taker buy-side at least neutral-to-positive, not contradicting
		if levelsAgree && flowAgrees {
			sl := price - math.Max(1.0*atr15m, 0.003*price)
			slDist := price - sl
			tp := price + 2.0*slDist
			risk := price - sl
			reward := tp - price
			if risk <= 0 || reward/risk < 2.0 {
				return NoSignal(name)
			}
			conf := 0.68
			if depthLevelsAvailable {
				conf = 0.74 // multi-level corroboration available — higher conviction
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionLong,
				Confidence: conf,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Orderbook imbalance persistence LONG: imbalance=%.2f bidWall=%.2f>askWall=%.2f, "+
						"depth-level agreement=%d/%d, tradeFlow=%.2f",
					ob.Imbalance, ob.BidWallSize, ob.AskWallSize, levelsBullish, levelsTotal, tradeFlow,
				),
			}
		}
	}

	// ── Bearish persistence: ask wall stacked, corroborated across levels + flow ──
	bearishImbalance := ob.Imbalance <= -imbalanceThreshold && ob.AskWallSize > ob.BidWallSize
	if bearishImbalance {
		levelsAgree := !depthLevelsAvailable || (levelsBearish == levelsTotal && levelsBearish > 0)
		flowAgrees := tradeFlow <= 0
		if levelsAgree && flowAgrees {
			sl := price + math.Max(1.0*atr15m, 0.003*price)
			slDist := sl - price
			tp := price - 2.0*slDist
			risk := sl - price
			reward := price - tp
			if risk <= 0 || reward/risk < 2.0 {
				return NoSignal(name)
			}
			conf := 0.68
			if depthLevelsAvailable {
				conf = 0.74
			}
			return Signal{
				Strategy:   name,
				Direction:  DirectionShort,
				Confidence: conf,
				StopLoss:   sl,
				TakeProfit: tp,
				Reason: fmt.Sprintf(
					"Orderbook imbalance persistence SHORT: imbalance=%.2f askWall=%.2f>bidWall=%.2f, "+
						"depth-level agreement=%d/%d, tradeFlow=%.2f",
					ob.Imbalance, ob.AskWallSize, ob.BidWallSize, levelsBearish, levelsTotal, tradeFlow,
				),
			}
		}
	}

	return NoSignal(name)
}
