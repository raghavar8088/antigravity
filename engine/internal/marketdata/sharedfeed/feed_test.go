package sharedfeed

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mkBars(start time.Time, n int, px float64) []Bar {
	out := make([]Bar, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Bar{
			OpenTime: start.Add(time.Duration(i) * time.Minute),
			Open:     px, High: px + 1, Low: px - 1, Close: px + float64(i),
			Volume: 10,
		})
	}
	return out
}

// THE point of this package: upstream request count must be a function of how
// many instruments are traded, never of how many strategies trade them. Without
// this, ~900 hunt accounts would issue ~900 requests per cycle and be banned.
func TestFeed_RequestCountIsIndependentOfStrategyCount(t *testing.T) {
	var fetches int64
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)

	f := New(Config{
		Poll:    time.Hour, // one poll during the test
		MaxBars: 500,
		Primary: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			atomic.AddInt64(&fetches, 1)
			return mkBars(base, 30, 100), nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx, []Pair{{"BTCUSD", "1m"}, {"ETHUSD", "1m"}})

	waitForBars(t, f, "BTCUSD", "1m")
	waitForBars(t, f, "ETHUSD", "1m")

	// 900 "strategies" all read concurrently, exactly as the hunt desks will.
	const strategies = 900
	var wg sync.WaitGroup
	var reads int64
	for i := 0; i < strategies; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s := f.Get("BTCUSD", "1m"); len(s.Bars) > 0 {
				atomic.AddInt64(&reads, 1)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt64(&reads); got != strategies {
		t.Fatalf("only %d/%d strategies got bars", got, strategies)
	}
	// Two pairs polled once each. NOT 900.
	if got := atomic.LoadInt64(&fetches); got != 2 {
		t.Fatalf("upstream fetches = %d, want 2 (one per pair) — %d strategy reads must add zero requests",
			got, strategies)
	}
}

// Reads must never touch the network, or the decoupling above is an illusion.
func TestFeed_GetPerformsNoFetch(t *testing.T) {
	var fetches int64
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)

	f := New(Config{
		Poll: time.Hour,
		Primary: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			atomic.AddInt64(&fetches, 1)
			return mkBars(base, 10, 50), nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx, []Pair{{"BTCUSD", "1m"}})
	waitForBars(t, f, "BTCUSD", "1m")

	after := atomic.LoadInt64(&fetches)
	for i := 0; i < 100; i++ {
		_ = f.Get("BTCUSD", "1m")
	}
	if got := atomic.LoadInt64(&fetches); got != after {
		t.Fatalf("Get() issued %d extra fetch(es); reads must be pure", got-after)
	}
}

// Delta is primary because the Live Engine executes on Delta. Binance is used
// only when Delta fails — a healthy Delta must never be bypassed.
func TestFeed_FallsBackToBinanceOnlyWhenDeltaFails(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	var deltaCalls, binanceCalls int64

	deltaFails := true
	f := New(Config{
		Poll: time.Hour,
		Primary: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			atomic.AddInt64(&deltaCalls, 1)
			if deltaFails {
				return nil, fmt.Errorf("429 rate limited")
			}
			return mkBars(base, 5, 100), nil
		},
		Fallback: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			atomic.AddInt64(&binanceCalls, 1)
			return mkBars(base, 5, 200), nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx, []Pair{{"BTCUSD", "1m"}})
	waitForBars(t, f, "BTCUSD", "1m")

	snap := f.Get("BTCUSD", "1m")
	if snap.Source != SourceBinance {
		t.Fatalf("source = %q, want binance after a delta rate limit", snap.Source)
	}
	if atomic.LoadInt64(&binanceCalls) == 0 {
		t.Fatal("fallback never invoked despite delta failing")
	}

	// Delta healthy again: the fallback must not be used.
	deltaFails = false
	before := atomic.LoadInt64(&binanceCalls)
	f.refresh(ctx, Pair{"BTCUSD", "1m"})

	if snap := f.Get("BTCUSD", "1m"); snap.Source != SourceDelta {
		t.Fatalf("source = %q, want delta once delta recovered", snap.Source)
	}
	if atomic.LoadInt64(&binanceCalls) != before {
		t.Error("fallback was called while delta was healthy")
	}
}

// A pair that stops updating must be visibly stale. A desk trading a frozen
// book is worse than one that knows it is blind.
func TestFeed_MarksStaleWhenUpstreamStops(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	f := New(Config{
		Poll:       time.Hour,
		StaleAfter: 2 * time.Minute,
		Primary: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			return mkBars(base, 5, 100), nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx, []Pair{{"BTCUSD", "1m"}})
	waitForBars(t, f, "BTCUSD", "1m")

	if f.Get("BTCUSD", "1m").Stale {
		t.Fatal("a freshly updated pair must not be stale")
	}

	// Advance the clock past the staleness budget.
	f.now = func() time.Time { return time.Now().UTC().Add(10 * time.Minute) }
	if !f.Get("BTCUSD", "1m").Stale {
		t.Fatal("a pair with no fresh bar must be marked stale")
	}
}

// Repeated polls overlap deliberately so a bar closing between polls is never
// missed; the store must de-duplicate rather than accumulate copies.
func TestFeed_DeduplicatesOverlappingPolls(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	f := New(Config{
		Poll: time.Hour,
		Primary: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			return mkBars(base, 20, 100), nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f.Start(ctx, []Pair{{"BTCUSD", "1m"}})
	waitForBars(t, f, "BTCUSD", "1m")
	for i := 0; i < 5; i++ {
		f.refresh(ctx, Pair{"BTCUSD", "1m"})
	}

	bars := f.Get("BTCUSD", "1m").Bars
	if len(bars) != 20 {
		t.Fatalf("got %d bars after 6 overlapping polls, want 20 de-duplicated", len(bars))
	}
	for i := 1; i < len(bars); i++ {
		if !bars[i].OpenTime.After(bars[i-1].OpenTime) {
			t.Fatalf("bars not strictly ordered at %d", i)
		}
	}
}

// A strategy must not be able to corrupt shared state for the other 899.
func TestFeed_SnapshotIsACopy(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Minute).Add(-time.Hour)
	f := New(Config{
		Poll: time.Hour,
		Primary: func(ctx context.Context, sym, res string, from, to time.Time) ([]Bar, error) {
			return mkBars(base, 5, 100), nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.Start(ctx, []Pair{{"BTCUSD", "1m"}})
	waitForBars(t, f, "BTCUSD", "1m")

	s := f.Get("BTCUSD", "1m")
	s.Bars[0].Close = -999

	if f.Get("BTCUSD", "1m").Bars[0].Close == -999 {
		t.Fatal("mutating a snapshot changed shared state")
	}
}

func TestBinanceSymbolMapping(t *testing.T) {
	// Delta quotes against USD, Binance against USDT. Getting this wrong makes
	// every fallback request 400 and look like a network fault.
	cases := map[string]string{
		"BTCUSD": "BTCUSDT", "ETHUSD": "ETHUSDT", "SOLUSD": "SOLUSDT",
		"AVAXUSD": "AVAXUSDT", "BTCUSDT": "BTCUSDT", "btcusd": "BTCUSDT",
	}
	for in, want := range cases {
		if got := binanceSymbol(in); got != want {
			t.Errorf("binanceSymbol(%q) = %q, want %q", in, got, want)
		}
	}
}

func waitForBars(t *testing.T, f *Feed, sym, res string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(f.Get(sym, res).Bars) > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no bars stored for %s %s within timeout", sym, res)
}
