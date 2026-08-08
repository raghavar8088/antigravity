package delta

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

// Venue-side protection and fill-relative levels.
//
// Both exist because of the 2026-08-08 forensic finding: the desk's stops lived
// only inside this process, on a 15-second poll, and its targets were measured
// from a price the live order never traded at.

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
	sp := strconv.FormatFloat(roundToTick(stop, tick), 'f', -1, 64)
	tp := strconv.FormatFloat(roundToTick(target, tick), 'f', -1, 64)

	_, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: plan.ProductID,
		Size:      plan.Contracts,
		// The bracket closes the position, so it sits on the opposite side.
		Side:      oppositeSide(plan.Side),
		OrderType: TypeLimit,
		// Reduce-only: a bracket must never be able to open exposure.
		ReduceOnly:                  true,
		TimeInForce:                 "gtc",
		BracketStopLossPrice:        sp,
		BracketStopLossLimitPrice:   sp,
		BracketTakeProfitPrice:      tp,
		BracketTakeProfitLimitPrice: tp,
	})
	return err
}

func oppositeSide(s OrderSide) OrderSide {
	if s == SideBuy {
		return SideSell
	}
	return SideBuy
}
