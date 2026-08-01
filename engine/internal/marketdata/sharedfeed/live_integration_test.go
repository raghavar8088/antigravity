package sharedfeed

import (
	"context"
	"os"
	"testing"
	"time"
)

// Live integration check against the real venues. Skipped unless
// SHAREDFEED_LIVE_TEST=1, so ordinary `go test ./...` stays offline and fast.
//
//	SHAREDFEED_LIVE_TEST=1 go test ./internal/marketdata/sharedfeed/ -run Live -v
//
// This exists because the unit tests use fake fetchers: they prove the fan-out
// and fallback logic, not that Delta's schema still parses. Both matter.
func TestLive_DeltaAndBinanceReturnRealBars(t *testing.T) {
	if os.Getenv("SHAREDFEED_LIVE_TEST") != "1" {
		t.Skip("set SHAREDFEED_LIVE_TEST=1 to run against live venues")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	to := time.Now().UTC()
	from := to.Add(-2 * time.Hour)

	t.Run("delta", func(t *testing.T) {
		bars, err := DeltaFetcher(ctx, "BTCUSD", "1m", from, to)
		if err != nil {
			t.Fatalf("delta fetch failed: %v", err)
		}
		if len(bars) < 30 {
			t.Fatalf("delta returned %d bars for a 2h 1m window, expected ~120", len(bars))
		}
		assertSaneBars(t, bars, to)
		t.Logf("delta: %d bars, last close %.2f", len(bars), bars[len(bars)-1].Close)
	})

	t.Run("binance_fallback", func(t *testing.T) {
		bars, err := BinanceFetcher(ctx, "BTCUSD", "1m", from, to) // note: Delta symbol in
		if err != nil {
			t.Fatalf("binance fetch failed: %v", err)
		}
		if len(bars) < 30 {
			t.Fatalf("binance returned %d bars, expected ~120", len(bars))
		}
		assertSaneBars(t, bars, to)
		t.Logf("binance: %d bars, last close %.2f", len(bars), bars[len(bars)-1].Close)
	})

	// The whole point: one poll feeds many readers, off real data.
	t.Run("feed_end_to_end", func(t *testing.T) {
		f := New(Config{
			Poll:     time.Minute,
			Backfill: 2 * time.Hour,
			Primary:  DeltaFetcher,
			Fallback: BinanceFetcher,
		})
		fctx, fcancel := context.WithCancel(ctx)
		defer fcancel()
		f.Start(fctx, []Pair{{"BTCUSD", "1m"}, {"ETHUSD", "1m"}})

		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if len(f.Get("BTCUSD", "1m").Bars) > 0 && len(f.Get("ETHUSD", "1m").Bars) > 0 {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}

		for _, sym := range []string{"BTCUSD", "ETHUSD"} {
			s := f.Get(sym, "1m")
			if len(s.Bars) == 0 {
				t.Fatalf("%s: feed stored no bars", sym)
			}
			if s.Stale {
				t.Errorf("%s: freshly polled pair reported stale", sym)
			}
			if s.Source != SourceDelta && s.Source != SourceBinance {
				t.Errorf("%s: unexpected source %q", sym, s.Source)
			}
			t.Logf("%s: %d bars from %s, last close %.2f", sym, len(s.Bars), s.Source, s.LastClose())
		}
	})
}

func assertSaneBars(t *testing.T, bars []Bar, now time.Time) {
	t.Helper()
	for i, b := range bars {
		if b.High < b.Low {
			t.Fatalf("bar %d: high %.2f < low %.2f", i, b.High, b.Low)
		}
		if b.Close <= 0 || b.Open <= 0 {
			t.Fatalf("bar %d: non-positive price", i)
		}
		if b.OpenTime.After(now) {
			t.Fatalf("bar %d: open time %s is in the future — an unclosed bar leaked through", i, b.OpenTime)
		}
		if i > 0 && !b.OpenTime.After(bars[i-1].OpenTime) {
			t.Fatalf("bar %d: not strictly after previous", i)
		}
	}
}
