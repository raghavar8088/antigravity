package optionchain

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"antigravity-engine/internal/delta"
)

// The synthetic Black-Scholes chain could quote any strike at any expiry, with
// no spread and a perfect fill. Delta lists ~456 BTC contracts and nothing else.
// These tests pin the behaviour that difference demands: resolve to a LISTED
// contract, and when nothing is close enough, SKIP rather than quietly trade a
// different instrument.

type fakeSource struct {
	contracts []delta.ChainContract
	marks     map[string]delta.TickerMark
	chainErr  error
	markErr   error
	calls     int64
}

func (f *fakeSource) ListOptionChain(ctx context.Context, u string) ([]delta.ChainContract, error) {
	atomic.AddInt64(&f.calls, 1)
	if f.chainErr != nil {
		return nil, f.chainErr
	}
	return f.contracts, nil
}

func (f *fakeSource) ListOptionTickers(ctx context.Context, u string) (map[string]delta.TickerMark, error) {
	atomic.AddInt64(&f.calls, 1)
	if f.markErr != nil {
		return nil, f.markErr
	}
	return f.marks, nil
}

// buildChain makes a realistic chain: strikes every $1,000, two expiries.
func buildChain(base time.Time) *fakeSource {
	f := &fakeSource{marks: map[string]delta.TickerMark{}}
	for _, exp := range []time.Time{base.Add(24 * time.Hour), base.Add(72 * time.Hour)} {
		for k := 60000.0; k <= 70000.0; k += 1000 {
			for _, typ := range []string{"CALL", "PUT"} {
				sym := fmt.Sprintf("%s-%.0f-%s", typ, k, exp.Format("020106"))
				f.contracts = append(f.contracts, delta.ChainContract{
					ProductID: len(f.contracts) + 1, Symbol: sym,
					OptionType: typ, Strike: k, Expiry: exp,
				})
				f.marks[sym] = delta.TickerMark{
					Symbol: sym, MarkPerBTC: 500, Bid: 490, Ask: 510,
				}
			}
		}
	}
	return f
}

func freshCache(t *testing.T, src Source) *Cache {
	t.Helper()
	c := New(src, "BTC", time.Hour, 5*time.Minute)
	c.Refresh(context.Background())
	return c
}

func TestResolve_SnapsToNearestListedStrike(t *testing.T) {
	base := time.Now().UTC()
	c := freshCache(t, buildChain(base))

	// Ask for 64,400 — not listed. Nearest listed is 64,000.
	q, err := c.Resolve("CALL", 64400, base.Add(24*time.Hour), DefaultTolerance)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if q.Strike != 64000 {
		t.Errorf("strike = %.0f, want 64000 (nearest listed)", q.Strike)
	}
	if q.MarkPerBTC <= 0 {
		t.Error("resolved contract has no mark")
	}
	if q.StrikeDriftPct <= 0 {
		t.Error("drift must be reported when the request was snapped")
	}
}

// The critical one: when nothing is close enough, the caller must be told to
// skip. Substituting a distant contract would mean the desk records a result for
// a trade the strategy never asked for.
func TestResolve_SkipsWhenNothingWithinTolerance(t *testing.T) {
	base := time.Now().UTC()
	c := freshCache(t, buildChain(base))

	// 40,000 is far outside the 60k-70k listed range.
	if _, err := c.Resolve("CALL", 40000, base.Add(24*time.Hour), DefaultTolerance); err == nil {
		t.Fatal("resolved a contract 37% from the requested strike; must skip instead")
	}
}

func TestResolve_SkipsWhenExpiryTooFar(t *testing.T) {
	base := time.Now().UTC()
	c := freshCache(t, buildChain(base))

	// Listed expiries are +24h and +72h. Ask for 30 days out.
	_, err := c.Resolve("CALL", 64000, base.Add(30*24*time.Hour), DefaultTolerance)
	if err == nil {
		t.Fatal("resolved a contract ~27 days from the requested expiry; must skip")
	}
}

// Expiry dominates strike: a same-strike option at the wrong expiry is a
// different instrument in a way a neighbouring strike at the right expiry is not.
func TestResolve_PrefersCorrectExpiryOverCloserStrike(t *testing.T) {
	base := time.Now().UTC()
	src := &fakeSource{marks: map[string]delta.TickerMark{}}
	near := base.Add(24 * time.Hour)
	far := base.Add(72 * time.Hour)

	add := func(sym string, strike float64, exp time.Time) {
		src.contracts = append(src.contracts, delta.ChainContract{
			ProductID: len(src.contracts) + 1, Symbol: sym,
			OptionType: "CALL", Strike: strike, Expiry: exp,
		})
		src.marks[sym] = delta.TickerMark{Symbol: sym, MarkPerBTC: 100, Bid: 95, Ask: 105}
	}
	add("EXACT-STRIKE-FAR-EXPIRY", 64000, far)
	add("NEAR-STRIKE-RIGHT-EXPIRY", 64500, near)

	c := freshCache(t, src)
	q, err := c.Resolve("CALL", 64000, near, DefaultTolerance)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if q.Symbol != "NEAR-STRIKE-RIGHT-EXPIRY" {
		t.Errorf("chose %q; expiry must dominate strike", q.Symbol)
	}
}

// A contract with no live quote cannot be priced honestly, so it must not be
// treated as available.
func TestResolve_IgnoresContractsWithoutMarks(t *testing.T) {
	base := time.Now().UTC()
	src := buildChain(base)
	// Strip the mark from the exact strike the caller wants.
	for sym := range src.marks {
		if sym == fmt.Sprintf("CALL-64000-%s", base.Add(24*time.Hour).Format("020106")) {
			delete(src.marks, sym)
		}
	}
	c := freshCache(t, src)

	q, err := c.Resolve("CALL", 64000, base.Add(24*time.Hour), DefaultTolerance)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if q.MarkPerBTC <= 0 {
		t.Fatal("returned an unquoted contract")
	}
	if q.Strike == 64000 {
		t.Error("chose the unquoted 64000 strike; it must be skipped for a quoted neighbour")
	}
}

// Pricing off a frozen chain fabricates P&L, so a stale snapshot must refuse.
func TestResolve_RefusesStaleSnapshot(t *testing.T) {
	base := time.Now().UTC()
	c := freshCache(t, buildChain(base))

	c.now = func() time.Time { return time.Now().UTC().Add(30 * time.Minute) }
	if !c.Stale() {
		t.Fatal("snapshot should be stale after 30m with a 5m budget")
	}
	if _, err := c.Resolve("CALL", 64000, base.Add(24*time.Hour), DefaultTolerance); err == nil {
		t.Fatal("resolved against a stale snapshot; that fabricates P&L")
	}
}

// One poll serves every strategy — the same load-decoupling property as the
// shared candle feed, applied to the chain.
func TestCache_ManyStrategiesShareOneSnapshot(t *testing.T) {
	base := time.Now().UTC()
	src := buildChain(base)
	c := freshCache(t, src)

	callsAfterRefresh := atomic.LoadInt64(&src.calls)

	const strategies = 200
	var wg sync.WaitGroup
	var ok int64
	for i := 0; i < strategies; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Resolve("PUT", 65000, base.Add(24*time.Hour), DefaultTolerance); err == nil {
				atomic.AddInt64(&ok, 1)
			}
		}()
	}
	wg.Wait()

	if ok != strategies {
		t.Fatalf("only %d/%d strategies resolved", ok, strategies)
	}
	if got := atomic.LoadInt64(&src.calls); got != callsAfterRefresh {
		t.Fatalf("%d strategy resolutions caused %d extra upstream call(s); resolution must be pure",
			strategies, got-callsAfterRefresh)
	}
}

func TestSpreadPct(t *testing.T) {
	q := Quote{MarkPerBTC: 500, Bid: 490, Ask: 510}
	if got := q.SpreadPct(); got < 0.039 || got > 0.041 {
		t.Errorf("SpreadPct() = %.4f, want ~0.04", got)
	}
	// A one-sided book reports zero rather than a fabricated spread.
	if got := (Quote{MarkPerBTC: 500, Bid: 0, Ask: 510}).SpreadPct(); got != 0 {
		t.Errorf("one-sided book SpreadPct() = %.4f, want 0", got)
	}
}

// A failed refresh must not blank the last good snapshot mid-session.
func TestRefresh_FailureKeepsLastGoodSnapshot(t *testing.T) {
	base := time.Now().UTC()
	src := buildChain(base)
	c := freshCache(t, src)

	src.chainErr = fmt.Errorf("429 rate limited")
	c.Refresh(context.Background())

	if _, err := c.Resolve("CALL", 64000, base.Add(24*time.Hour), DefaultTolerance); err != nil {
		t.Fatalf("last good snapshot was discarded on a failed refresh: %v", err)
	}
	if _, _, _, lastErr := c.Stats(); lastErr == "" {
		t.Error("refresh failure was not recorded")
	}
}
