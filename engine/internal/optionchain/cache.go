// Package optionchain serves a live Delta option chain to many strategies from
// a single poll, and resolves a strategy's desired strike/expiry to a contract
// that actually exists.
//
// Why this replaces the synthetic Black-Scholes chain:
//
// The options desks priced against a model that could quote ANY strike at ANY
// expiry, with no spread and a perfect fill. Delta lists ~456 BTC contracts
// across 7 expiries and nothing else. A strategy that wants a strike Delta does
// not list cannot trade it at any price — a constraint the model hid entirely,
// which is why paper results were not transferable to the Live Engine.
//
// Two properties matter for the strategy hunt:
//
//  1. One poll serves every strategy. ~100 option strategies resolving their own
//     contracts would be ~100 requests per cycle; here it is 2 (chain + marks).
//  2. A signal that cannot be filled is RECORDED as skipped, not silently
//     repriced onto a different contract. A skip is data — it says the strategy
//     needs strikes this venue does not offer.
package optionchain

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/delta"
)

// Quote is a resolved, tradeable contract with its live mark.
type Quote struct {
	Symbol     string
	ProductID  int
	OptionType string // "CALL" | "PUT"
	Strike     float64
	Expiry     time.Time

	// MarkPerBTC is the premium quoted in USD per BTC of underlying.
	MarkPerBTC float64
	Bid        float64
	Ask        float64

	// StrikeDriftPct is how far the resolved strike sits from the one the
	// strategy asked for, as a fraction. Non-zero means the request was snapped
	// to a listed contract, and the desk is trading something slightly different
	// from what the signal specified.
	StrikeDriftPct float64
	// ExpiryDrift is the gap between the requested and resolved expiry.
	ExpiryDrift time.Duration
}

// SpreadPct is the bid/ask spread as a fraction of the mark. Zero when the book
// is one-sided or unquoted — which is itself a reason to decline a trade.
func (q Quote) SpreadPct() float64 {
	if q.Bid <= 0 || q.Ask <= 0 || q.MarkPerBTC <= 0 {
		return 0
	}
	return (q.Ask - q.Bid) / q.MarkPerBTC
}

// ErrNoContract means the venue lists nothing close enough to the request. The
// caller must skip the signal — never substitute a distant contract.
var ErrNoContract = fmt.Errorf("optionchain: no listed contract within tolerance")

// Tolerance bounds how far a resolution may drift from what was asked.
type Tolerance struct {
	// MaxStrikeDriftPct rejects a snap that lands too far from the requested
	// strike. A 5% drift on a short-dated option is a materially different bet.
	MaxStrikeDriftPct float64
	// MaxExpiryDrift rejects a contract expiring too far from the request.
	MaxExpiryDrift time.Duration
}

// DefaultTolerance is deliberately tight. Loosening it does not create liquidity;
// it just hides the fact that the strategy is trading a different instrument.
var DefaultTolerance = Tolerance{
	MaxStrikeDriftPct: 0.03,
	MaxExpiryDrift:    48 * time.Hour,
}

type snapshot struct {
	contracts []delta.ChainContract
	marks     map[string]delta.TickerMark
	takenAt   time.Time
	lastErr   string
}

// Source is the subset of the Delta client this package needs, so tests do not
// require a live exchange.
type Source interface {
	ListOptionChain(ctx context.Context, underlying string) ([]delta.ChainContract, error)
	ListOptionTickers(ctx context.Context, underlying string) (map[string]delta.TickerMark, error)
}

// Cache holds one chain snapshot and serves all strategies from it.
type Cache struct {
	src        Source
	underlying string
	refresh    time.Duration
	staleAfter time.Duration

	mu   sync.RWMutex
	snap snapshot

	now func() time.Time
}

// New creates a Cache. Call Start to begin polling.
func New(src Source, underlying string, refresh, staleAfter time.Duration) *Cache {
	if refresh <= 0 {
		refresh = time.Minute
	}
	if staleAfter <= 0 {
		staleAfter = 5 * time.Minute
	}
	return &Cache{
		src: src, underlying: strings.ToUpper(underlying),
		refresh: refresh, staleAfter: staleAfter,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Start polls the chain on an interval. Two upstream requests per cycle,
// regardless of how many strategies read it.
func (c *Cache) Start(ctx context.Context) {
	go func() {
		c.Refresh(ctx)
		t := time.NewTicker(c.refresh)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.Refresh(ctx)
			}
		}
	}()
}

// Refresh fetches the chain and its marks once.
func (c *Cache) Refresh(ctx context.Context) {
	contracts, err := c.src.ListOptionChain(ctx, c.underlying)
	if err != nil {
		c.noteErr(fmt.Sprintf("chain: %v", err))
		return
	}
	marks, err := c.src.ListOptionTickers(ctx, c.underlying)
	if err != nil {
		// Contracts without marks are useless for pricing, so this is a failure
		// even though the listing succeeded.
		c.noteErr(fmt.Sprintf("marks: %v", err))
		return
	}

	c.mu.Lock()
	c.snap = snapshot{contracts: contracts, marks: marks, takenAt: c.now()}
	c.mu.Unlock()
}

func (c *Cache) noteErr(msg string) {
	c.mu.Lock()
	c.snap.lastErr = msg
	c.mu.Unlock()
	log.Printf("[OPTIONCHAIN] refresh failed: %s", msg)
}

// Stale reports whether the snapshot is too old to price against. Pricing a
// position off a frozen chain fabricates P&L.
func (c *Cache) Stale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snap.takenAt.IsZero() || c.now().Sub(c.snap.takenAt) > c.staleAfter
}

// Stats reports what the cache is holding, for the desk UI.
func (c *Cache) Stats() (contracts, quoted int, takenAt time.Time, lastErr string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.snap.contracts), len(c.snap.marks), c.snap.takenAt, c.snap.lastErr
}

// Resolve maps a strategy's requested strike/expiry/type onto the nearest LISTED
// contract with a live mark, or returns ErrNoContract.
//
// Selection prefers the closest expiry first, then the closest strike within it.
// Expiry dominates because a same-strike option at a different expiry is a
// different instrument in a way a neighbouring strike at the right expiry is not.
func (c *Cache) Resolve(optType string, strike float64, expiry time.Time, tol Tolerance) (Quote, error) {
	if tol.MaxStrikeDriftPct <= 0 {
		tol = DefaultTolerance
	}
	c.mu.RLock()
	snap := c.snap
	c.mu.RUnlock()

	if len(snap.contracts) == 0 {
		return Quote{}, fmt.Errorf("optionchain: empty snapshot (%s)", snap.lastErr)
	}
	if snap.takenAt.IsZero() || c.now().Sub(snap.takenAt) > c.staleAfter {
		return Quote{}, fmt.Errorf("optionchain: snapshot stale since %s", snap.takenAt.Format(time.RFC3339))
	}

	want := strings.ToUpper(optType)
	if want != "CALL" && want != "PUT" {
		return Quote{}, fmt.Errorf("optionchain: bad option type %q", optType)
	}

	// Candidates of the right type that still have a live mark. A contract with
	// no quote cannot be priced honestly, so it does not count as available.
	type cand struct {
		c    delta.ChainContract
		mark delta.TickerMark
	}
	cands := make([]cand, 0, 64)
	for _, ct := range snap.contracts {
		if ct.OptionType != want {
			continue
		}
		m, ok := snap.marks[ct.Symbol]
		if !ok || m.MarkPerBTC <= 0 {
			continue
		}
		cands = append(cands, cand{ct, m})
	}
	if len(cands) == 0 {
		return Quote{}, ErrNoContract
	}

	sort.Slice(cands, func(i, j int) bool {
		di := absDur(cands[i].c.Expiry.Sub(expiry))
		dj := absDur(cands[j].c.Expiry.Sub(expiry))
		if di != dj {
			return di < dj
		}
		return math.Abs(cands[i].c.Strike-strike) < math.Abs(cands[j].c.Strike-strike)
	})

	best := cands[0]
	expiryDrift := absDur(best.c.Expiry.Sub(expiry))
	if tol.MaxExpiryDrift > 0 && expiryDrift > tol.MaxExpiryDrift {
		return Quote{}, ErrNoContract
	}

	// Within the chosen expiry, take the nearest strike.
	bestExpiry := best.c.Expiry
	for _, cd := range cands {
		if !cd.c.Expiry.Equal(bestExpiry) {
			continue
		}
		if math.Abs(cd.c.Strike-strike) < math.Abs(best.c.Strike-strike) {
			best = cd
		}
	}

	drift := 0.0
	if strike > 0 {
		drift = math.Abs(best.c.Strike-strike) / strike
	}
	if drift > tol.MaxStrikeDriftPct {
		return Quote{}, ErrNoContract
	}

	return Quote{
		Symbol:         best.c.Symbol,
		ProductID:      best.c.ProductID,
		OptionType:     best.c.OptionType,
		Strike:         best.c.Strike,
		Expiry:         best.c.Expiry,
		MarkPerBTC:     best.mark.MarkPerBTC,
		Bid:            best.mark.Bid,
		Ask:            best.mark.Ask,
		StrikeDriftPct: drift,
		ExpiryDrift:    expiryDrift,
	}, nil
}

// MarkFor returns the current mark for an already-resolved symbol, which is what
// mark-to-market needs on every tick. Returns false when the contract has no
// live quote — the caller must hold its last known mark rather than invent one.
func (c *Cache) MarkFor(symbol string) (delta.TickerMark, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.snap.marks[symbol]
	if !ok || m.MarkPerBTC <= 0 {
		return delta.TickerMark{}, false
	}
	return m, true
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
