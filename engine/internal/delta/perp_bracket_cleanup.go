package delta

import (
	"context"
	"log"
	"math"
	"time"
)

// venueExitFor asks Delta what a position actually closed at.
//
// Used when the venue closed a position without this process placing the order
// — which, now that brackets do every price exit, is the normal case. The old
// behaviour booked these at the ENTRY price, recording exactly $0.00 for a
// trade that really made or lost money.
//
// Returns the fill price and the reason to book it under. When the fill cannot
// be found the result is UNRECONCILED at price 0, never a fabricated flat: an
// unknown that is visible can be corrected by the reconciler, a loss disguised
// as zero cannot.
func (b *PerpBridge) venueExitFor(ctx context.Context, t *PerpLiveTrade) (float64, string) {
	if b.client == nil || t == nil {
		return 0, ExitReasonUnreconciled
	}
	fills, err := b.client.GetFills(ctx, 100)
	if err != nil {
		log.Printf("[PERP LIVE] %s %s closed on the venue but fills are unreadable (%v) — booking UNRECONCILED",
			t.Strategy, t.Symbol, err)
		return 0, ExitReasonUnreconciled
	}

	// The most recent fill on this symbol on the CLOSING side, after the
	// position opened. Side matters: an entry fill on the same symbol would
	// otherwise be mistaken for the exit and book the trade against its own
	// opening price.
	want := "sell"
	if !t.long() {
		want = "buy"
	}
	best := 0.0
	var bestAt time.Time
	for _, f := range fills {
		if f.Symbol != t.Symbol || f.Side != want || f.Price <= 0 {
			continue
		}
		at, err := time.Parse(time.RFC3339, f.CreatedAt)
		if err != nil {
			continue
		}
		if at.Before(t.OpenedAt) {
			continue
		}
		if at.After(bestAt) {
			bestAt, best = at, f.Price
		}
	}
	if best <= 0 {
		return 0, ExitReasonUnreconciled
	}

	// Which level did it land on? Reported honestly rather than assumed: a
	// bracket fill near the stop is a stop, near the target a target, and
	// anything else is neither and should not claim to be.
	reason := "VENUE_CLOSE"
	if t.StopPrice > 0 && math.Abs(best-t.StopPrice)/t.StopPrice < 0.005 {
		reason = "SL"
	} else if t.TargetPrice > 0 && math.Abs(best-t.TargetPrice)/t.TargetPrice < 0.005 {
		reason = "TP"
	}
	log.Printf("[PERP LIVE] %s %s closed BY THE VENUE @ %.8f (%s)", t.Strategy, t.Symbol, best, reason)
	return best, reason
}

// cancelBracketFor removes any bracket leg still resting for a closed position.
//
// One side of a bracket fills and the other does not. Left alone it stays on the
// venue, and a reduce-only trigger outliving its position can arm against the
// NEXT position in the same symbol — opening or closing exposure nobody asked
// for, at a price chosen for a trade that already ended.
func (b *PerpBridge) cancelBracketFor(ctx context.Context, t *PerpLiveTrade) {
	if b.client == nil || t == nil || !t.BracketsAttached {
		return
	}
	// Only cancel when NO other open trade holds the same product. Two
	// strategies routinely hold one symbol — Delta nets them — and cancelling
	// blind would strip the survivor's protection.
	b.mu.RLock()
	others := 0
	for _, o := range b.open {
		if o != t && o.ProductID == t.ProductID {
			others++
		}
	}
	b.mu.RUnlock()
	if others > 0 {
		log.Printf("[PERP LIVE] %s: leaving brackets in place — %d other position(s) hold this product",
			t.Symbol, others)
		return
	}
	if err := b.client.CancelBracketsForProduct(ctx, t.ProductID); err != nil {
		// Not fatal: a stale trigger is a hazard, but failing to cancel must not
		// stop the close from being booked.
		log.Printf("[PERP LIVE] %s: could not cancel leftover brackets: %v", t.Symbol, err)
	}
}
