// Package sharedfeed serves closed OHLCV bars to many strategies from a single
// upstream poll.
//
// Why this exists: the strategy-hunt desks run ~900 concurrent accounts (800
// scalp streams + ~50 option-buying + ~50 option-selling). If each one fetched
// its own candles, that is ~900 requests per cycle and an immediate rate-limit
// ban. Here one poller runs per (symbol, resolution) pair and every strategy
// reads the same stored bars, so request volume is set by how many INSTRUMENTS
// are traded, never by how many strategies trade them:
//
//	per-strategy fetching : 900 strategies -> ~900 req/cycle
//	shared store          : 8 symbols      -> ~8   req/cycle   (900 or 9000 strategies)
//
// Delta Exchange is the primary source because the Live Engine trades Delta — a
// strategy validated on another venue's prices is validated on the wrong book.
// Binance is a fallback used only when Delta fails or rate-limits, so the desks
// keep running rather than going blind; every bar records which venue produced
// it so a mixed-source window is visible rather than silent.
package sharedfeed

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/marketdata"
)

// Source identifies which venue produced a bar. Recorded per snapshot so a
// desk can tell whether it is trading Delta prices or fallback prices.
type Source string

const (
	SourceDelta   Source = "delta"
	SourceBinance Source = "binance"
	SourceNone    Source = ""
)

// Bar is one closed OHLCV candle.
type Bar struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// Snapshot is what a strategy reads: an immutable view of the stored bars plus
// the provenance a caller needs to judge them.
type Snapshot struct {
	Symbol     string
	Resolution string
	Bars       []Bar // oldest first, closed bars only
	Source     Source
	UpdatedAt  time.Time // when the poller last stored new bars
	Stale      bool      // no fresh bar within the staleness budget
	LastErr    string    // last upstream failure, empty when healthy
}

// LastClose returns the most recent closed price, or 0 when there are no bars.
func (s Snapshot) LastClose() float64 {
	if len(s.Bars) == 0 {
		return 0
	}
	return s.Bars[len(s.Bars)-1].Close
}

// Fetcher retrieves closed bars for one instrument from one venue.
type Fetcher func(ctx context.Context, symbol, resolution string, from, to time.Time) ([]Bar, error)

// Config tunes a Feed. Zero values fall back to the defaults below.
type Config struct {
	// Poll is how often each pair is refreshed. Should be at least the bar
	// duration; polling faster only wastes rate-limit budget.
	Poll time.Duration
	// Backfill is how much history to load on first poll.
	Backfill time.Duration
	// MaxBars caps the ring buffer per pair.
	MaxBars int
	// StaleAfter marks a pair stale when no new bar has arrived in this long.
	// A strategy trading a frozen book is worse than one that knows it is blind.
	StaleAfter time.Duration
	// Primary and Fallback fetchers. Fallback may be nil.
	Primary  Fetcher
	Fallback Fetcher
}

func (c Config) withDefaults() Config {
	if c.Poll <= 0 {
		c.Poll = time.Minute
	}
	if c.Backfill <= 0 {
		c.Backfill = 6 * time.Hour
	}
	if c.MaxBars <= 0 {
		c.MaxBars = 1500
	}
	if c.StaleAfter <= 0 {
		c.StaleAfter = 5 * time.Minute
	}
	return c
}

type pairState struct {
	bars      []Bar
	source    Source
	updatedAt time.Time
	lastErr   string
}

// Feed owns one poller per (symbol, resolution) and serves every strategy from
// the stored bars.
type Feed struct {
	cfg Config

	mu    sync.RWMutex
	pairs map[string]*pairState

	now func() time.Time
}

// New creates a Feed. Call Start to begin polling.
func New(cfg Config) *Feed {
	return &Feed{
		cfg:   cfg.withDefaults(),
		pairs: map[string]*pairState{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func key(symbol, resolution string) string {
	return strings.ToUpper(symbol) + "|" + strings.ToLower(resolution)
}

// Start launches one polling goroutine per pair. Returns immediately; the first
// snapshot for a pair is empty until its backfill completes.
//
// The number of goroutines — and therefore the upstream request rate — is a
// function of len(pairs) only. Strategy count never appears in it.
func (f *Feed) Start(ctx context.Context, pairs []Pair) {
	for _, p := range pairs {
		k := key(p.Symbol, p.Resolution)
		f.mu.Lock()
		if _, exists := f.pairs[k]; !exists {
			f.pairs[k] = &pairState{}
		}
		f.mu.Unlock()
		go f.pollLoop(ctx, p)
	}
	log.Printf("[SHAREDFEED] %d pair(s) polling every %s — upstream load is per-pair, not per-strategy",
		len(pairs), f.cfg.Poll)
}

// Pair is one instrument at one resolution.
type Pair struct {
	Symbol     string
	Resolution string
}

func (f *Feed) pollLoop(ctx context.Context, p Pair) {
	// Poll immediately so desks are not blind for a whole interval on boot.
	f.refresh(ctx, p)

	t := time.NewTicker(f.cfg.Poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			f.refresh(ctx, p)
		}
	}
}

// refresh fetches once and stores. Primary first; fallback only on failure, so
// a healthy Delta is never bypassed.
func (f *Feed) refresh(ctx context.Context, p Pair) {
	k := key(p.Symbol, p.Resolution)

	f.mu.RLock()
	st := f.pairs[k]
	var have int
	if st != nil {
		have = len(st.bars)
	}
	f.mu.RUnlock()

	to := f.now()
	from := to.Add(-f.cfg.Backfill)
	if have > 0 {
		// Incremental: re-fetch a short overlap so a bar that closed between
		// polls is never missed, and de-duplication handles the repeats.
		from = to.Add(-10 * f.cfg.Poll)
	}

	bars, src, err := f.fetchWithFallback(ctx, p, from, to)
	if err != nil {
		f.mu.Lock()
		if s := f.pairs[k]; s != nil {
			s.lastErr = err.Error()
		}
		f.mu.Unlock()
		log.Printf("[SHAREDFEED] %s %s: fetch failed: %v", p.Symbol, p.Resolution, err)
		return
	}

	f.store(k, bars, src)
}

func (f *Feed) fetchWithFallback(ctx context.Context, p Pair, from, to time.Time) ([]Bar, Source, error) {
	if f.cfg.Primary != nil {
		bars, err := f.cfg.Primary(ctx, p.Symbol, p.Resolution, from, to)
		if err == nil && len(bars) > 0 {
			return bars, SourceDelta, nil
		}
		if f.cfg.Fallback == nil {
			if err == nil {
				err = fmt.Errorf("primary returned no bars")
			}
			return nil, SourceNone, err
		}
		// Rate limits and outages are exactly why the fallback exists. Log the
		// switch loudly: a desk quietly trading a different venue's prices than
		// the one it will execute on is a silent correctness problem.
		log.Printf("[SHAREDFEED] %s %s: delta unavailable (%v) — falling back to binance",
			p.Symbol, p.Resolution, err)
	}
	if f.cfg.Fallback == nil {
		return nil, SourceNone, fmt.Errorf("no fetcher configured")
	}
	bars, err := f.cfg.Fallback(ctx, p.Symbol, p.Resolution, from, to)
	if err != nil {
		return nil, SourceNone, err
	}
	return bars, SourceBinance, nil
}

// store merges new bars into the ring, de-duplicating by open time and keeping
// the buffer sorted and bounded.
func (f *Feed) store(k string, incoming []Bar, src Source) {
	if len(incoming) == 0 {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	st := f.pairs[k]
	if st == nil {
		st = &pairState{}
		f.pairs[k] = st
	}

	byTime := make(map[int64]Bar, len(st.bars)+len(incoming))
	for _, b := range st.bars {
		byTime[b.OpenTime.Unix()] = b
	}
	for _, b := range incoming {
		byTime[b.OpenTime.Unix()] = b
	}

	merged := make([]Bar, 0, len(byTime))
	for _, b := range byTime {
		merged = append(merged, b)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].OpenTime.Before(merged[j].OpenTime) })

	if len(merged) > f.cfg.MaxBars {
		merged = merged[len(merged)-f.cfg.MaxBars:]
	}

	st.bars = merged
	st.source = src
	st.updatedAt = f.now()
	st.lastErr = ""
}

// Get returns a snapshot for one pair. This is what every strategy calls, and
// it performs NO network I/O — it is a read of already-fetched bars, which is
// the entire reason 900 strategies do not produce 900 requests.
//
// The returned slice is a copy, so a strategy cannot mutate shared state.
func (f *Feed) Get(symbol, resolution string) Snapshot {
	k := key(symbol, resolution)

	f.mu.RLock()
	defer f.mu.RUnlock()

	snap := Snapshot{Symbol: strings.ToUpper(symbol), Resolution: strings.ToLower(resolution)}
	st := f.pairs[k]
	if st == nil {
		snap.Stale = true
		return snap
	}
	snap.Bars = append([]Bar(nil), st.bars...)
	snap.Source = st.source
	snap.UpdatedAt = st.updatedAt
	snap.LastErr = st.lastErr
	snap.Stale = st.updatedAt.IsZero() || f.now().Sub(st.updatedAt) > f.cfg.StaleAfter
	return snap
}

// Health reports one line per pair, for the desk UI and for diagnosing a feed
// that has silently stopped.
func (f *Feed) Health() []Snapshot {
	f.mu.RLock()
	keys := make([]string, 0, len(f.pairs))
	for k := range f.pairs {
		keys = append(keys, k)
	}
	f.mu.RUnlock()

	sort.Strings(keys)
	out := make([]Snapshot, 0, len(keys))
	for _, k := range keys {
		parts := strings.SplitN(k, "|", 2)
		if len(parts) != 2 {
			continue
		}
		out = append(out, f.Get(parts[0], parts[1]))
	}
	return out
}

// DeltaFetcher adapts the existing Delta history client to the Fetcher shape.
// Delta is primary: the Live Engine executes on Delta, so the hunt must measure
// strategies on Delta's own prices.
func DeltaFetcher(ctx context.Context, symbol, resolution string, from, to time.Time) ([]Bar, error) {
	_ = ctx
	rows, err := marketdata.FetchDeltaHistoricalCandles(symbol, resolution, from, to)
	if err != nil {
		return nil, err
	}
	return fromHistorical(rows, to), nil
}

// fromHistorical converts and drops any bar that has not closed yet. Serving an
// in-progress bar would let every strategy act on a price that can still move —
// look-ahead that inflates every result downstream.
func fromHistorical(rows []marketdata.HistoricalCandle, now time.Time) []Bar {
	out := make([]Bar, 0, len(rows))
	for _, r := range rows {
		if !r.CloseTime.IsZero() && r.CloseTime.After(now) {
			continue
		}
		out = append(out, Bar{
			OpenTime: r.OpenTime.UTC(),
			Open:     r.Open,
			High:     r.High,
			Low:      r.Low,
			Close:    r.Close,
			Volume:   r.Volume,
		})
	}
	return out
}
