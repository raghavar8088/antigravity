package optionchain

import (
	"time"

	"antigravity-engine/internal/options"
	"antigravity-engine/internal/options_selling"
)

// Adapters bind the shared chain cache to the two option desks.
//
// The desks declare their own ChainPricer interfaces so neither depends on the
// exchange client; these adapters are the only place the two sides meet. Both
// desks read the SAME cache, so ~100 strategies across both share one snapshot
// and produce zero extra upstream requests between refreshes.

// BuyingPricer adapts the cache to the option-BUYING desk.
type BuyingPricer struct {
	cache *Cache
	tol   Tolerance
}

// ForBuying returns a pricer for the long-premium desk.
func ForBuying(c *Cache, tol Tolerance) *BuyingPricer {
	if tol.MaxStrikeDriftPct <= 0 {
		tol = DefaultTolerance
	}
	return &BuyingPricer{cache: c, tol: tol}
}

func (p *BuyingPricer) ResolveEntry(optType string, strike float64, expiry time.Time) (options.ChainQuote, error) {
	q, err := p.cache.Resolve(optType, strike, expiry, p.tol)
	if err != nil {
		// Collapse every "cannot fill" case into the desk's sentinel so the
		// caller has one thing to check and always reacts by skipping.
		return options.ChainQuote{}, options.ErrNoListedContract
	}
	return options.ChainQuote{
		Symbol:         q.Symbol,
		Strike:         q.Strike,
		Expiry:         q.Expiry,
		PremiumPerBTC:  q.MarkPerBTC,
		Bid:            q.Bid,
		Ask:            q.Ask,
		StrikeDriftPct: q.StrikeDriftPct,
	}, nil
}

func (p *BuyingPricer) MarkFor(symbol string) (float64, bool) {
	m, ok := p.cache.MarkFor(symbol)
	if !ok {
		return 0, false
	}
	return m.MarkPerBTC, true
}

// SellingPricer adapts the cache to the option-SELLING desk.
type SellingPricer struct {
	cache *Cache
	tol   Tolerance
}

// ForSelling returns a pricer for the short-premium desk.
func ForSelling(c *Cache, tol Tolerance) *SellingPricer {
	if tol.MaxStrikeDriftPct <= 0 {
		tol = DefaultTolerance
	}
	return &SellingPricer{cache: c, tol: tol}
}

func (p *SellingPricer) ResolveEntry(optType string, strike float64, expiry time.Time) (options_selling.ChainQuote, error) {
	q, err := p.cache.Resolve(optType, strike, expiry, p.tol)
	if err != nil {
		return options_selling.ChainQuote{}, options_selling.ErrNoListedContract
	}
	return options_selling.ChainQuote{
		Symbol:         q.Symbol,
		Strike:         q.Strike,
		Expiry:         q.Expiry,
		PremiumPerBTC:  q.MarkPerBTC,
		Bid:            q.Bid,
		Ask:            q.Ask,
		StrikeDriftPct: q.StrikeDriftPct,
	}, nil
}

func (p *SellingPricer) MarkFor(symbol string) (float64, bool) {
	m, ok := p.cache.MarkFor(symbol)
	if !ok {
		return 0, false
	}
	return m.MarkPerBTC, true
}

// QuoteFor gives the selling desk both sides of the book. It sells at the bid
// and buys back at the ask; pricing either leg at mid would overstate its edge
// by half the spread, twice per round trip.
func (p *SellingPricer) QuoteFor(symbol string) (options_selling.ChainQuote, bool) {
	m, ok := p.cache.MarkFor(symbol)
	if !ok {
		return options_selling.ChainQuote{}, false
	}
	return options_selling.ChainQuote{
		Symbol:        m.Symbol,
		PremiumPerBTC: m.MarkPerBTC,
		Bid:           m.Bid,
		Ask:           m.Ask,
	}, true
}
