package delta

import (
	"context"
	"fmt"
	"math"
)

// Venue-side protection and fill-relative levels.
//
// Both exist because of the 2026-08-08 forensic finding: the desk's stops lived
// only inside this process, on a 15-second poll, and its targets were measured
// from a price the live order never traded at.

// minStopTicks is how many price ticks a stop must sit away from entry.
//
// On a contract whose tick is coarse relative to its price, a percentage stop
// can be smaller than one tick and therefore unrepresentable. 1000SATSUSD marks
// at 0.00001055 against a 0.0000001 tick, so the desk's 0.35% stop is 0.37 of a
// tick: it rounds to a whole tick, 0.95%, and the position carries ~3x the risk
// the strategy chose while the leaderboard reports the intended figure.
//
// Two ticks minimum, so rounding cannot move a level onto or past the entry.
const minStopTicks = 2.0

// stopLimitSlippageOfRisk is how far past the trigger a stop's LIMIT sits,
// as a fraction of the STOP DISTANCE rather than of price.
//
// It was 0.5% of price, which fixed stops that could not fill and created the
// opposite failure. On COOKIEUSD the whole stop distance is 0.64% of price, so
// 0.5% of price was 0.78x the entire risk budget: the order filled reliably,
// at up to 1.78x the intended loss. Measured live — planned stop 0.01259,
// filled 0.012650 against a limit of 0.012653, i.e. at the worst price the
// order allowed.
//
// Tying it to the stop distance bounds the damage: whatever the symbol, the
// realised loss cannot exceed the planned loss by more than this fraction.
// 20% caps a stop-out at 1.2x its intended size, which is a cost worth paying
// for a fill; the previous constant admitted 1.78x on this symbol and more on
// tighter ones.
const stopLimitSlippageOfRisk = 0.20

// stopLimitSlippageMinTicks keeps the limit marketable when the stop distance
// is itself only a few ticks. A percentage of a tiny risk budget can round to
// zero on a coarse grid, which puts the limit back ON the trigger — the exact
// unfillable stop this whole mechanism exists to avoid.
const stopLimitSlippageMinTicks = 2.0

// stopLimitSlip is the price offset past the trigger for a stop's limit leg.
func stopLimitSlip(entry, stop, tick float64) float64 {
	risk := math.Abs(entry - stop)
	slip := risk * stopLimitSlippageOfRisk
	if tick > 0 {
		if floor := tick * stopLimitSlippageMinTicks; slip < floor {
			slip = floor
		}
	}
	return slip
}

// StopHasTickResolution reports whether a stop can be expressed on this
// contract's price grid.
func StopHasTickResolution(entry, stop, tick float64) bool {
	if tick <= 0 {
		// Unknown grid: permit, because refusing every order on a missing
		// registry field would be a worse failure than the one being prevented.
		return true
	}
	return math.Abs(entry-stop) >= tick*minStopTicks
}

// PerpRewardRisk is the house reward-to-risk ratio for live perpetual orders.
//
// Matches the paper desk's targetRewardRisk. At 1:3 a strategy needs a 25% win
// rate to break even before costs; the mirrors' previous 1:0.50 needed 66.7%,
// and they delivered 33.3% with real money.
const PerpRewardRisk = 3.0

// perpLevelsFromFill re-derives stop and target from the price actually filled.
//
// The RISK distance is preserved from the plan — that is the strategy's own
// decision and the basis the position was sized on. Reward is then set by the
// house ratio. Anchoring both to the fill keeps the distances the strategy
// intended, wherever the market put us.
func perpLevelsFromFill(fill float64, plan PerpOrderPlan) (stop, target float64) {
	if fill <= 0 {
		return plan.StopPrice, plan.TargetPrice
	}
	// Distance the plan intended, measured against the price it was built on.
	ref := plan.LimitPrice
	if ref <= 0 {
		ref = fill
	}
	risk := math.Abs(ref - plan.StopPrice)
	if risk <= 0 {
		return plan.StopPrice, plan.TargetPrice
	}
	if plan.Side == SideBuy {
		return fill - risk, fill + risk*PerpRewardRisk
	}
	return fill + risk, fill - risk*PerpRewardRisk
}

// attachBrackets submits stop-loss and take-profit legs that live on the VENUE.
//
// Delta holds these in its own order book, so they execute on a price touch
// rather than when this process next looks. That closes the gap that let three
// measured stop-outs exit at 0.805%, 1.018% and 1.054% against a 0.700% stop —
// and let one 0.754% adverse move miss the stop entirely.
//
// Reduce-only by construction on Delta's side: a bracket can only close the
// position it is attached to.
func (b *PerpBridge) attachBrackets(ctx context.Context, plan PerpOrderPlan, stop, target float64) error {
	if b.client == nil {
		return fmt.Errorf("no Delta client")
	}
	if stop <= 0 || target <= 0 {
		return fmt.Errorf("refusing to attach a bracket with a non-positive level (stop %.8f target %.8f)", stop, target)
	}
	// A stop on the wrong side of the fill would close the position instantly at
	// a loss. Cheap to check, and the failure is expensive.
	if plan.Side == SideBuy && stop >= target {
		return fmt.Errorf("long bracket inverted: stop %.8f >= target %.8f", stop, target)
	}
	if plan.Side == SideSell && stop <= target {
		return fmt.Errorf("short bracket inverted: stop %.8f <= target %.8f", stop, target)
	}

	// Tick size comes from the registry. A price off the tick is rejected by the
	// venue, which would leave the position open and unprotected — the exact
	// state these brackets exist to prevent.
	tick := 0.0
	if pr, ok := b.reg.Lookup(plan.Symbol); ok {
		tick = pr.TickSize
	}
	if !StopHasTickResolution(plan.LimitPrice, stop, tick) {
		return fmt.Errorf("stop is under %g ticks from entry on %s (tick %g) — it cannot be expressed on this price grid",
			minStopTicks, plan.Symbol, tick)
	}

	// Formatted to the tick's OWN decimal count. FormatFloat with -1 printed
	// the float noise left by roundToTick — 0.012860000000000001 — and Delta
	// refused it as off-grid, leaving the position unprotected.
	sp := formatTickPrice(stop, tick)
	tp := formatTickPrice(target, tick)
	// Room for the stop's limit to be marketable when it triggers. Direction
	// matters: closing a LONG sells, so the limit goes BELOW the trigger;
	// closing a SHORT buys, so it goes ABOVE.
	slip := stopLimitSlip(plan.LimitPrice, stop, tick)
	slipRaw := stop + slip
	if plan.Side == SideBuy {
		slipRaw = stop - slip
	}
	slipLimit := formatTickPrice(slipRaw, tick)
	if sp == "" || tp == "" || slipLimit == "" {
		return fmt.Errorf("bracket: could not express stop %.10f / target %.10f on a %g tick", stop, target, tick)
	}

	// The dedicated bracket endpoint, not the entry order.
	//
	// Putting these parameters on PlaceOrder produced HTTP 400 bad_schema on
	// every attempt for three hours: "Limit price required for limit orders"
	// and "invalid value" on bracket_take_profit_limit_price. Both legs need a
	// trigger AND a limit, and they belong to the position, not the fill.
	return b.client.PlaceBracket(ctx, BracketRequest{
		ProductID:     plan.ProductID,
		ProductSymbol: plan.Symbol,
		StopLoss: &BracketLeg{
			OrderType: TypeLimit,
			StopPrice: sp,
			// The limit sits BEYOND the trigger, not on it.
			//
			// Setting them equal produced a stop that triggered and then could
			// not fill. Live case: LABUSD short, stop 0.1243, price gapped to
			// 0.1253. The leg converted to a buy limit at 0.1243 — below the
			// market — and rested unfilled while the loss ran from the planned
			// -$1.78 to -$3.22, with the monitor standing down because a
			// bracket was "attached".
			//
			// A stop that cannot fill is not protection. Slippage room makes it
			// marketable on touch, which is the whole point of a stop; the cost
			// is a few basis points of give, against an unbounded downside.
			LimitPrice: slipLimit,
		},
		TakeProfit: &BracketLeg{
			OrderType:  TypeLimit,
			StopPrice:  tp,
			LimitPrice: tp,
		},
		// Last traded price, not mark. The mark is an index and can sit away
		// from where an order would actually fill — the same gap that let the
		// polling monitor exit at 0.830% against a 0.580% stop.
		TriggerMethod: "last_traded_price",
	})
}
func oppositeSide(s OrderSide) OrderSide {
	if s == SideBuy {
		return SideSell
	}
	return SideBuy
}

// perpOutcomeUSD is what a position pays at its target and costs at its stop,
// both NET of the round-trip taker fee.
//
// The fee is charged on entry and exit whichever way the trade goes, so it
// SHRINKS the win and DEEPENS the loss — it does not cancel between them. A
// table showing gross outcomes would overstate the reward and understate the
// risk at the same time, on a desk where the round trip is comparable to the
// move being targeted.
func perpOutcomeUSD(t *PerpLiveTrade, contractValue float64) (ifTarget, ifStop float64) {
	if t == nil || contractValue <= 0 || t.EntryPrice <= 0 || t.Contracts == 0 {
		return 0, 0
	}
	n := float64(t.Contracts)
	if n < 0 {
		n = -n
	}
	qty := n * contractValue

	if t.TargetPrice > 0 {
		gross := math.Abs(t.TargetPrice-t.EntryPrice) * qty
		fees := (t.EntryPrice + t.TargetPrice) * qty * PerpTakerFeeRate
		ifTarget = gross - fees
	}
	if t.StopPrice > 0 {
		gross := math.Abs(t.StopPrice-t.EntryPrice) * qty
		fees := (t.EntryPrice + t.StopPrice) * qty * PerpTakerFeeRate
		// Negative: this is what the position COSTS.
		ifStop = -(gross + fees)
	}
	return ifTarget, ifStop
}
