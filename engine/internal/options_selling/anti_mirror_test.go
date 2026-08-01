package options_selling

import (
	"math"
	"testing"
	"time"
)

// The property the whole exercise exists for: when one half wins, the other
// loses the same gross amount. If it does not hold, an anti-strategy's P&L is
// not evidence about its original and the leaderboard comparison is meaningless.
//
// The old construction — short PUT mirroring a short CALL — failed this in the
// most common case of all: a flat market, where both halves collected decay and
// both WON.

// mirrorPairFixture builds an engine with one original holding a position, plus
// its mirror opened from that same fill.
func mirrorPairFixture(t *testing.T) (e *Engine, orig, mirror *strategyState) {
	t.Helper()
	e = NewEngine()
	e.SetFeedStatus(true, PrimaryVenue)

	for _, s := range e.states {
		if !IsAnti(s.def.Name) {
			orig = s
			break
		}
	}
	if orig == nil {
		t.Fatal("no non-mirror strategy in the engine")
	}
	mirror = e.stateByName(AntiPrefix + orig.def.Name)
	if mirror == nil {
		t.Fatalf("no mirror state for %s", orig.def.Name)
	}

	now := time.Now().UTC()
	pos := &OptionPosition{
		ID: "p1", StrategyID: orig.def.ID, StrategyName: orig.def.Name,
		OptionType: orig.def.Type, Strike: 63000, ExpiryTime: now.Add(2 * time.Hour),
		EntryPremium: 100, CurrentPremium: 100, Quantity: 2, CostBasis: 200,
		EntryBTCPrice: 63000, EntryTime: now,
	}
	orig.position = pos
	e.openMirrorLocked(orig, pos, now)
	return e, orig, mirror
}

// The mirror must hold the SAME contract, on the opposite side.
func TestMirror_SellsTheSameContract(t *testing.T) {
	_, orig, mirror := mirrorPairFixture(t)

	if mirror.position == nil {
		t.Fatal("the original filled but no mirror position was opened")
	}
	m, o := mirror.position, orig.position

	if m.OptionType != o.OptionType {
		t.Errorf("mirror holds a %s against the original's %s — it must SELL the same contract, "+
			"or both halves are long premium and share theta/vega sign", m.OptionType, o.OptionType)
	}
	if m.Strike != o.Strike || !m.ExpiryTime.Equal(o.ExpiryTime) {
		t.Errorf("mirror strike/expiry %v/%v != original %v/%v", m.Strike, m.ExpiryTime, o.Strike, o.ExpiryTime)
	}
	if m.EntryPremium != o.EntryPremium {
		t.Errorf("mirror entered at %v, original at %v — it must inherit the fill", m.EntryPremium, o.EntryPremium)
	}
	if m.Quantity != o.Quantity {
		t.Errorf("mirror size %v != original %v", m.Quantity, o.Quantity)
	}
	if !m.LongPremium {
		t.Error("mirror is not long; a short mirror is a second bet, not an inverse")
	}
	if o.LongPremium {
		t.Error("the original was marked long")
	}
}

// Unrealised P&L must move in exactly opposite directions.
func TestMirror_UnrealisedPnLIsTheNegationOfTheOriginals(t *testing.T) {
	e, orig, mirror := mirrorPairFixture(t)
	now := time.Now().UTC()

	for _, premium := range []float64{150, 65, 100, 5} {
		orig.position.CurrentPremium = premium
		mirror.position.CurrentPremium = premium
		e.markToMarketPositionLocked(orig.position, 0.5, 0.5, 0.35, now)
		e.markToMarketPositionLocked(mirror.position, 0.5, 0.5, 0.35, now)

		sum := orig.position.UnrealizedPnL + mirror.position.UnrealizedPnL
		if math.Abs(sum) > 1e-9 {
			t.Errorf("at premium %v the pair sums to %v, want 0 (orig %v, mirror %v)",
				premium, sum, orig.position.UnrealizedPnL, mirror.position.UnrealizedPnL)
		}
	}
}

// THE CASE THE OLD CONSTRUCTION GOT WRONG: a flat market where the premium
// decays. A short call and a short put both WIN here. One short and one long of
// the same contract cannot.
func TestMirror_InAFlatDecayingMarketOneHalfGains(t *testing.T) {
	e, orig, mirror := mirrorPairFixture(t)
	now := time.Now().UTC()

	// Mark both halves on the same tick. The desk re-prices from the model when
	// no chain pricer is set, so the exact premium is not the test's to choose —
	// what matters is that whatever premium both are marked at, they land on
	// OPPOSITE sides of it.
	e.markToMarketPositionLocked(orig.position, 0.5, 0.5, 0.35, now)
	e.markToMarketPositionLocked(mirror.position, 0.5, 0.5, 0.35, now)

	o := orig.position.UnrealizedPnL
	m := mirror.position.UnrealizedPnL
	if o == 0 || m == 0 {
		t.Skipf("premium landed exactly at entry (orig %v mirror %v) — nothing to compare", o, m)
	}
	if (o > 0) == (m > 0) {
		t.Fatalf("both halves moved the same way (orig %v, mirror %v) — this is exactly the old bug: "+
			"two same-side positions sharing theta and vega sign instead of cancelling", o, m)
	}
}

// A mirror must never decide its own exit. Its original's rules are
// long-premium rules; a short-side reinterpretation would close the two halves
// at different premiums and stop them being an inverse.
func TestMirror_RunsNoExitPolicyOfItsOwn(t *testing.T) {
	e, _, mirror := mirrorPairFixture(t)
	now := time.Now().UTC()

	// A premium move far beyond any threshold either way.
	for _, premium := range []float64{1000, 0.01} {
		mirror.position.CurrentPremium = premium
		if reason := e.markToMarketPositionLocked(mirror.position, 0.5, 0.5, 0.35, now); reason != "" {
			t.Errorf("mirror produced its own exit %q at premium %v; it must wait for its original", reason, premium)
		}
	}
}

// Closing the original must close the mirror on the same tick at the same
// premium, and the realised P&L must cancel.
func TestMirror_ClosesWithItsOriginalAndTheRealisedPnLCancels(t *testing.T) {
	e, orig, mirror := mirrorPairFixture(t)
	now := time.Now().UTC()

	orig.position.CurrentPremium = 145
	e.markToMarketPositionLocked(orig.position, 0.5, 0.5, 0.35, now)
	origPnL := orig.position.UnrealizedPnL

	e.closePositionLocked(orig, ExitTP, now)

	if orig.position != nil {
		t.Fatal("original still open after close")
	}
	if mirror.position != nil {
		t.Fatal("mirror left open after its original closed — it is no longer an inverse of anything")
	}
	if orig.stats.TotalTrades != 1 || mirror.stats.TotalTrades != 1 {
		t.Fatalf("trade counts orig=%d mirror=%d; a pair must book together",
			orig.stats.TotalTrades, mirror.stats.TotalTrades)
	}
	// Gross cancels exactly. Net does not: both halves pay fees, so the pair
	// costs whatever those fees came to. That is the whole economics of an
	// anti-strategy — it earns only when its original has a negative GROSS edge,
	// not merely a negative net — so the pair must never sum ABOVE zero.
	sum := orig.stats.TotalPnL + mirror.stats.TotalPnL
	if sum > 1e-9 {
		t.Errorf("pair netted +%v; a mirrored pair always costs fees", sum)
	}
	if math.Abs(sum) > math.Abs(orig.stats.TotalPnL)*0.5 {
		t.Errorf("pair sums to %v against an original P&L of %v — that is far more than a fee load, so gross did not cancel",
			sum, orig.stats.TotalPnL)
	}
	if (orig.stats.TotalPnL > 0) == (mirror.stats.TotalPnL > 0) {
		t.Errorf("both halves went the same way: orig %v, mirror %v", orig.stats.TotalPnL, mirror.stats.TotalPnL)
	}
	_ = origPnL
}

// The pair must be cash-neutral at open: the original pays the premium, the
// mirror receives it. Otherwise mirroring would quietly change the desk's
// balance and every equity curve drawn from it.
func TestMirror_PairIsCashNeutralAtOpen(t *testing.T) {
	e := NewEngine()
	e.SetFeedStatus(true, PrimaryVenue)

	var orig *strategyState
	for _, s := range e.states {
		if !IsAnti(s.def.Name) {
			orig = s
			break
		}
	}
	now := time.Now().UTC()
	pos := &OptionPosition{
		ID: "p1", StrategyName: orig.def.Name, OptionType: orig.def.Type,
		Strike: 63000, ExpiryTime: now.Add(time.Hour),
		EntryPremium: 100, CurrentPremium: 100, Quantity: 2, CostBasis: 200,
		EntryTime: now,
	}

	before := e.balance
	e.balance += pos.EntryPremium * pos.Quantity // what the caller does for the short leg
	orig.position = pos
	e.openMirrorLocked(orig, pos, now)

	if math.Abs(e.balance-before) > 1e-9 {
		t.Errorf("balance moved by %v opening a pair; the premium received by one half is paid by the other",
			e.balance-before)
	}
}

// Mirrors are not mirrored, and a mirror never opens one for itself.
func TestMirror_DoesNotMirrorItself(t *testing.T) {
	e, _, mirror := mirrorPairFixture(t)
	before := e.mirrorOpens
	e.openMirrorLocked(mirror, mirror.position, time.Now().UTC())
	if e.mirrorOpens != before {
		t.Error("a mirror opened a mirror of itself")
	}
}

// Drift must be counted, not swallowed. That is how the scalp desk's broken
// mirrors went unnoticed for a whole session.
func TestMirror_CountsSkipsWhenAPairDrifts(t *testing.T) {
	e, orig, _ := mirrorPairFixture(t)
	before := e.mirrorSkips
	// The mirror already holds a position; a second open must be refused AND
	// recorded.
	e.openMirrorLocked(orig, orig.position, time.Now().UTC())
	if e.mirrorSkips != before+1 {
		t.Errorf("mirrorSkips = %d, want %d — a refused mirror must be visible", e.mirrorSkips, before+1)
	}
}
