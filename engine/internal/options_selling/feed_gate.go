package options_selling

import (
	"log"
	"sync/atomic"
	"time"
)

// Real-data gate.
//
// This SELLING desk exists to find strategies worth real money on the Live
// Engine. That makes the integrity of its record the whole product: a trade recorded against
// a price the market never printed is not a weak data point, it is a false one,
// and it is indistinguishable from a real one once it is in the table.
//
// The price feed had a synthetic last resort. When Delta AND Binance both failed
// it substituted a hardcoded constant and the desks kept trading on it —
// opening, marking and closing positions against a number no venue ever quoted.
// Nothing downstream could tell those trades apart from real ones.
//
// # What the gate does
//
//   - Prices from a real venue: trade normally.
//   - No real price available: keep managing OPEN positions, but open nothing
//     new. Custody of an existing position still has to function — abandoning it
//     is worse than marking it against the last real price — while adding new
//     exposure on a price nobody quoted has no upside at all.
//
// # Why it does not simply halt
//
// A desk that stops dead on a momentary feed hiccup produces a gap in the record
// that looks like an absence of signals rather than an absence of data. Refusing
// entries while continuing to manage is the honest middle: the record keeps its
// shape, and every trade in it was opened against a real quote.

// FeedStatus describes where the desk's spot price is coming from.
type FeedStatus struct {
	// Live is false when no real venue price is available.
	Live bool
	// Source is the venue name, or "synthetic" when nothing real was reachable.
	Source string
	// At is when this status was set.
	At time.Time
}

// feedGate is embedded in Engine. Kept in its own type so the selling desk can
// carry an identical one without the two drifting apart.
type feedGate struct {
	live   atomic.Bool
	source atomic.Value // string

	// blockedOpens counts entries refused because no real price was available.
	// A record that quietly lost a day of signals to a dead feed looks exactly
	// like a quiet market unless this is counted.
	blockedOpens atomic.Int64

	// opensOnFallback counts entries taken while the price came from the backup
	// venue rather than the primary. Those trades are real, but they were priced
	// on a book this desk does not execute against, so they are worth being able
	// to separate out when reading the leaderboard.
	opensOnFallback atomic.Int64

	loggedDown atomic.Bool
}

// SetFeedStatus records whether the desk currently has a real venue price.
//
// primary names the venue the desk is meant to trade (Delta). Anything else that
// is still a real market — the Binance fallback — counts as live but is tallied
// separately, because a strategy qualified largely on fallback prices was
// qualified on a book the Live Engine does not execute on.
func (g *feedGate) SetFeedStatus(live bool, source string) {
	prevLive := g.live.Load()
	g.live.Store(live)
	g.source.Store(source)

	switch {
	case !live && !g.loggedDown.Swap(true):
		log.Printf("[OPTIONS SELLING] ⛔ no real market price (source=%q) — managing open positions, opening nothing new. "+
			"Trades against a price no venue quoted would be indistinguishable from real ones in the record.", source)
	case live && !prevLive:
		g.loggedDown.Store(false)
		log.Printf("[OPTIONS SELLING] ✅ real market price restored (source=%q) — entries re-enabled", source)
	}
}

// FeedLive reports whether a real venue price is available.
func (g *feedGate) FeedLive() bool { return g.live.Load() }

// FeedSource is the venue currently supplying the price.
func (g *feedGate) FeedSource() string {
	if s, ok := g.source.Load().(string); ok {
		return s
	}
	return "unknown"
}

// BlockedOpens is how many entries were refused for want of a real price.
func (g *feedGate) BlockedOpens() int64 { return g.blockedOpens.Load() }

// OpensOnFallback is how many entries were taken on the backup venue's prices.
func (g *feedGate) OpensOnFallback() int64 { return g.opensOnFallback.Load() }

// noteBlockedOpen records a refused entry.
func (g *feedGate) noteBlockedOpen() { g.blockedOpens.Add(1) }

// noteOpen records an entry, tallying it separately when the price came from the
// fallback venue.
func (g *feedGate) noteOpen(primaryVenue string) {
	if g.FeedSource() != primaryVenue {
		g.opensOnFallback.Add(1)
	}
}

// PrimaryVenue is the venue this desk is built to trade and be measured on.
const PrimaryVenue = "delta"

// SyntheticSource marks a price that came from no venue at all.
const SyntheticSource = "synthetic"
