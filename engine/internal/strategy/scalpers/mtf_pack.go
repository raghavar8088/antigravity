package scalpers

import (
	"fmt"
	"time"
)

// mtf_pack.go — indicator strategies on 15m / 30m / 1h / 4h / 1d candles.
//
// WHY THIS PACK EXISTS
//
// The 1-minute roster produced a 25-29% win rate across 900+ live trades
// against a 30-36% breakeven, with gross P&L roughly FLAT and fees accounting
// for 42-92% of the loss. The signals were not wrong about direction; they were
// too small to clear the toll. At 1m on thin alts the move being captured is
// frequently smaller than spread plus fees, and every structural advantage in
// that contest belongs to someone else.
//
// A longer holding period does not make a signal smarter. It makes the same
// 0.118% round trip a far smaller fraction of the move, which is the only lever
// here that improves the economics without needing a better prediction.
//
// DESIGN RULES, all of them consequences of what the 1m roster taught:
//
//  1. Every entry must clear a MINIMUM EXPECTED MOVE, expressed as a multiple
//     of the round-trip fee. A setup whose target is 3x the fee is not an edge,
//     it is a coin flip with a commission.
//
//  2. Every strategy states its timeframe and refuses to evaluate below that
//     timeframe's warm-up. Indicators computed on short history are confident
//     and wrong.
//
//  3. Stops come from ATR, not a fixed percentage. A fixed 0.6% stop sat inside
//     the one-minute noise of TSTUSD and outside that of BANKUSD; the same
//     number cannot be right for both.
//
//  4. Two independent conditions minimum — a primary signal and a regime or
//     confirmation filter. Single-condition entries fire constantly, and trade
//     count is what converts a small negative edge into a large loss.
//
// NO MIRRORS. The ANTI_ inversion premise is arithmetically broken under fees:
// if an original nets -g-f, its mirror nets +g-f, so the mirror only profits
// when the original's GROSS loss exceeds the fee. Observed gross was flat,
// meaning g is approximately zero and both sides lose f. Hundreds of mirrors
// clustering at 25-29% is exactly what that predicts.

// roundTripFeePct is Delta's taker cost for a full round trip, in percent.
// Maker execution would roughly third it; until the engine posts passively,
// this is the toll every entry here must clear.
const roundTripFeePct = 0.118

// minTargetFeeMultiple is how many round trips of profit a setup must be
// reaching for before it is worth taking.
//
// 6x means a winning trade keeps about 83% of its gross. At the 1m roster's
// typical target the fee took 6.5% of a win but was charged on every trade,
// win or lose; at a 30% hit rate that is the entire edge.
const minTargetFeeMultiple = 6.0

// mtfStrategy is one indicator strategy bound to one timeframe.
type mtfStrategy struct {
	name string
	tf   HigherTF
	eval func(name string, c []Candle, price float64) Signal
}

func (s *mtfStrategy) Name() string           { return s.name }
func (s *mtfStrategy) ValidRegimes() []Regime { return nil }

func (s *mtfStrategy) Evaluate(ctx MarketContext) Signal {
	if ctx.Price <= 0 {
		return NoSignal(s.name)
	}
	c, ok := s.tf.CandlesFor(ctx)
	if !ok {
		// Not enough history on this timeframe. Explicitly no signal rather
		// than a signal from a short window.
		return NoSignal(s.name)
	}
	return s.eval(s.name, c, ctx.Price)
}

// mtfSignal builds a signal with an ATR-derived stop and a reward-multiple
// target, refusing anything whose target cannot clear the fee bar.
//
// The refusal is the point. Every strategy below can produce a technically
// valid setup in a dead market; without this they would all trade constantly
// in exactly the conditions that cannot pay for the trading.
func mtfSignal(name string, dir Direction, price, atrFrac, rr float64, reason string) Signal {
	if atrFrac <= 0 || price <= 0 {
		return NoSignal(name)
	}
	stopFrac := atrFrac * mtfStopATRMultiple
	targetFrac := stopFrac * rr

	if targetFrac*100 < roundTripFeePct*minTargetFeeMultiple {
		// The move being reached for is too small to pay for itself.
		return NoSignal(name)
	}

	var sl, tp float64
	if dir == DirectionLong {
		sl, tp = price*(1-stopFrac), price*(1+targetFrac)
	} else {
		sl, tp = price*(1+stopFrac), price*(1-targetFrac)
	}
	return Signal{
		Strategy:   name,
		Direction:  dir,
		Confidence: 0.6,
		StopLoss:   sl,
		TakeProfit: tp,
		Reason:     fmt.Sprintf("%s | stop %.2f%% target %.2f%% (%.1fx fee)", reason, stopFrac*100, targetFrac*100, targetFrac*100/roundTripFeePct),
		Timestamp:  time.Now().UTC(),
	}
}

// mtfStopATRMultiple places the stop outside normal candle noise.
//
// 1.5 ATR: below roughly 1 ATR the stop sits inside the range a single ordinary
// candle covers, which is how the 1m roster lost 55% of its trades to SL at a
// median hold of 130 seconds — the stop was resolved by wander before the
// signal could be right or wrong about anything.
const mtfStopATRMultiple = 1.5
