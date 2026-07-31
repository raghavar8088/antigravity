package options

import (
	"fmt"
	"log"
	"time"
)

// Fill-time mirroring for anti-strategies.
//
// # What was wrong
//
// An ANTI_ strategy was built by flipping the option TYPE — a long CALL's mirror
// was a long PUT — and swapping the exits. Both halves stayed LONG PREMIUM.
//
// That is not an inverse. A long call and a long put share the sign of theta and
// vega: both decay as time passes, both gain when volatility rises. In a flat
// market BOTH lose, and the pair's combined P&L is not "minus fees" but "minus
// fees minus two lots of decay". The property the mirror exists to demonstrate —
// if the original loses, this one gains — simply does not hold, and it fails
// worst in exactly the quiet conditions where most of these strategies trade.
//
// # What replaces it
//
// The exact inverse of "buy this contract at premium P" is "SELL THIS SAME
// CONTRACT at premium P". Same strike, same expiry, same option type, opposite
// side. Then every tick that moves the premium moves the two by equal and
// opposite amounts, by construction rather than by approximation.
//
// So the mirror is no longer a strategy that decides anything. It is a position
// opened at the moment its original fills, on the original's own contract, and
// closed at the moment its original closes, at the same premium. It evaluates no
// signals and runs no exit policy of its own.
//
// This is the same correction the scalp desk needed, for the same underlying
// reason: a mirror that makes its own decisions is not a mirror. There it was
// competing for its own fill; here it was holding a different instrument.
//
// # What the pair can and cannot earn
//
// Gross cancels exactly, so a pair nets the fees both sides paid. An
// anti-strategy is therefore profitable only when its original has a genuinely
// negative GROSS edge — not merely a negative net. A strategy that earns before
// costs and loses after has a mirror that loses on both counts.

// mirrorNameFor returns the mirror's strategy name, or "" if the name is itself
// a mirror. Mirrors are not mirrored: doing so returns the original under a
// confusing name and gives one hypothesis two accounts.
func mirrorNameFor(name string) string {
	if IsAnti(name) {
		return ""
	}
	return AntiPrefix + name
}

// stateByName finds a strategy state. Callers hold e.mu.
func (e *Engine) stateByName(name string) *strategyState {
	for _, s := range e.states {
		if s.def.Name == name {
			return s
		}
	}
	return nil
}

// openMirrorLocked opens the inverse of a fill the original just took.
//
// Caller holds e.mu and has already set orig.position.
func (e *Engine) openMirrorLocked(orig *strategyState, pos *OptionPosition, now time.Time) {
	if orig == nil || pos == nil || pos.ShortPremium {
		return
	}
	name := mirrorNameFor(orig.def.Name)
	if name == "" {
		return
	}
	m := e.stateByName(name)
	if m == nil {
		return // mirrors disabled (ANTI_STRATEGIES=false)
	}
	if m.position != nil {
		// Should not happen: a pair opens together and closes together. Skipping
		// preserves the one-position-per-strategy invariant the original obeys,
		// but it means the pair has drifted, so it is counted rather than
		// swallowed — that is how the scalp desk's broken mirrors went unnoticed.
		e.mirrorSkips++
		return
	}

	// A COPY of the original's contract. Same symbol, strike, expiry, type and
	// entry premium — anything else and the two are no longer opposite sides of
	// one instrument.
	mp := *pos
	mp.ID = fmt.Sprintf("%s-anti", pos.ID)
	mp.StrategyID = m.def.ID
	mp.StrategyName = name
	mp.ShortPremium = true
	mp.UnrealizedPnL = 0
	mp.PeakGainPct = 0

	// Selling the contract credits the premium, exactly as the selling desk
	// accounts for a short. The original was debited the same amount, so the
	// pair is cash-neutral at open.
	e.balance += mp.EntryPremium * mp.Quantity

	m.position = &mp
	m.stats.HasPosition = true
	m.stats.Status = optionStatusInPosition
	e.mirrorOpens++
}

// closeMirrorLocked closes the mirror of an original that is closing now, at the
// SAME premium.
//
// The mirror deliberately runs no exit policy. Its original's exit rules are
// long-premium rules — trailing stops on gains, theta-bleed cutoffs, thesis
// breaks — and re-deriving a short-side equivalent would make the two halves
// close at different premiums on different ticks, which is precisely what stops
// them being an inverse. Inheriting the exit keeps the P&L exactly negated.
//
// Caller holds e.mu and must call this BEFORE clearing orig.position.
func (e *Engine) closeMirrorLocked(orig *strategyState, reason string, now time.Time) {
	if orig == nil || orig.position == nil || orig.position.ShortPremium {
		return
	}
	name := mirrorNameFor(orig.def.Name)
	if name == "" {
		return
	}
	m := e.stateByName(name)
	if m == nil || m.position == nil {
		return
	}
	// Mark the mirror at the price the original is closing at, so the two
	// halves settle on one number rather than on two independent marks.
	m.position.CurrentPremium = orig.position.CurrentPremium
	e.closePositionLocked(m, reason+"_MIRROR", now)
}

// MirrorOpens is how many inverse positions were opened from an original's fill.
func (e *Engine) MirrorOpens() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mirrorOpens
}

// MirrorSkips counts pairs that drifted out of step. It must stay at zero: any
// other value means some ANTI_ rows are no longer exact inverses, and their
// combined P&L stops being readable as "minus fees".
func (e *Engine) MirrorSkips() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.mirrorSkips
}

// logMirrorImbalance reports a drifted pair once, loudly.
func (e *Engine) logMirrorImbalance(name string) {
	log.Printf("[OPTIONS] ⚠️  mirror %s could not open — the pair has drifted and its combined P&L is no longer -fees", name)
}
