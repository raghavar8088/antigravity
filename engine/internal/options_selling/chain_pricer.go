package options_selling

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Real-chain pricing.
//
// This SELLING desk priced against a synthetic Black-Scholes chain: any strike, any
// expiry, no spread, perfect fill. Delta lists ~457 live BTC contracts and
// nothing else, quotes a real bid/ask, and charges a fee capped at 10% of
// premium per side. Those three facts are why paper results here never
// transferred to the Live Engine.
//
// A ChainPricer swaps the model for the venue. When one is set the desk trades
// only contracts that exist, at the price the market is actually showing. When
// it is nil the desk keeps its old Black-Scholes behaviour unchanged, so the two
// can be compared rather than swapped on faith.
//
// For a SHORT-premium desk the spread cuts the other way: the seller receives
// the bid, not the mark, and pays the ask to close. Marking a short at mid
// flatters every trade on this desk by half the spread on entry and again on
// exit — which on the 1.46%-spread contracts measured on this chain is a
// material overstatement of edge.

// ChainQuote is a resolved, tradeable contract.
type ChainQuote struct {
	// Symbol is the venue's contract identifier, stored on the position so
	// mark-to-market prices the SAME contract that was entered.
	Symbol string
	// Strike and Expiry are the LISTED values, which may differ from what the
	// strategy asked for — see StrikeDriftPct.
	Strike float64
	Expiry time.Time
	// PremiumPerBTC is the mark in USD per BTC of underlying.
	PremiumPerBTC float64
	Bid, Ask      float64
	// StrikeDriftPct is how far the listed strike sits from the requested one.
	StrikeDriftPct float64
}

// SpreadPct is the bid/ask spread as a fraction of the mark, or 0 when the book
// is one-sided.
func (q ChainQuote) SpreadPct() float64 {
	if q.Bid <= 0 || q.Ask <= 0 || q.PremiumPerBTC <= 0 {
		return 0
	}
	return (q.Ask - q.Bid) / q.PremiumPerBTC
}

// ErrNoListedContract means the venue lists nothing close enough to the request.
// The signal must be SKIPPED — never repriced onto a distant contract, which
// would record a result for a trade the strategy never asked for.
var ErrNoListedContract = fmt.Errorf("options: no listed contract within tolerance")

// ChainPricer is the venue-facing pricing source. Implemented by an adapter over
// optionchain.Cache; kept as an interface here so this package does not depend
// on the exchange client.
type ChainPricer interface {
	// ResolveEntry maps a desired option onto a listed contract with a live
	// mark, or returns ErrNoListedContract.
	ResolveEntry(optType string, strike float64, expiry time.Time) (ChainQuote, error)
	// MarkFor returns the current mark (USD per BTC) for an already-resolved
	// contract. false means the contract has no live quote right now, and the
	// caller must hold its last known mark rather than invent one.
	MarkFor(symbol string) (float64, bool)
	// QuoteFor returns the full live quote, including bid and ask.
	//
	// A selling desk needs both sides, not just the mark: it receives the BID on
	// entry and pays the ASK to close. Pricing either leg at mid overstates the
	// edge by half the spread, twice per round trip.
	QuoteFor(symbol string) (ChainQuote, bool)
}

// ChainSkipStats counts signals the venue could not fill. A skip is data: it
// says the strategy needs strikes this exchange does not list, which is a real
// reason it cannot be promoted, not a bug to be tuned away.
type ChainSkipStats struct {
	NoContract int64 `json:"noContract"`
	NoMark     int64 `json:"noMark"`
	Filled     int64 `json:"filled"`
}

type chainSkipCounters struct {
	noContract atomic.Int64
	noMark     atomic.Int64
	filled     atomic.Int64
}

func (c *chainSkipCounters) snapshot() ChainSkipStats {
	return ChainSkipStats{
		NoContract: c.noContract.Load(),
		NoMark:     c.noMark.Load(),
		Filled:     c.filled.Load(),
	}
}

// SetChainPricer switches the desk onto real venue pricing. Passing nil restores
// Black-Scholes, which is what makes an A/B comparison possible.
func (e *Engine) SetChainPricer(p ChainPricer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.chainPricer = p
}

// UsingRealChain reports whether the desk is pricing against the venue.
func (e *Engine) UsingRealChain() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.chainPricer != nil
}

// ChainSkips reports how many signals the venue could not fill.
func (e *Engine) ChainSkips() ChainSkipStats { return e.chainSkips.snapshot() }
