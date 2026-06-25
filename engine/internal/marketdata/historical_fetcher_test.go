package marketdata

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// makeFakePage returns a Binance klines JSON array with `count` bars starting at startMs.
func makeFakePage(startMs int64, count int, intervalMs int64) []byte {
	bars := make([][]interface{}, count)
	for i := 0; i < count; i++ {
		openMs := startMs + int64(i)*intervalMs
		closeMs := openMs + intervalMs - 1
		bars[i] = []interface{}{
			openMs, "50000", "51000", "49000", "50500", "100.5", closeMs,
			"5025000", 100, "60", "3015000", "0",
		}
	}
	data, _ := json.Marshal(bars)
	return data
}

func TestFetchKlinesSinglePage(t *testing.T) {
	page := makeFakePage(1_700_000_000_000, 100, 60_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(page)
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := NewBinanceHistoricalFetcher(dir)
	f.httpClient = srv.Client()
	// Replace the base URL by overriding the HTTP client's transport
	f.httpClient.Transport = urlRewriteTransport(srv.URL)
	f.rateLimiter = make(chan time.Time, 10)
	for i := 0; i < 10; i++ {
		f.rateLimiter.(chan time.Time) <- time.Now()
	}

	candles, err := f.FetchKlines("BTCUSDT", "1m", 0, 0)
	if err != nil {
		t.Fatalf("FetchKlines error: %v", err)
	}
	if len(candles) != 100 {
		t.Fatalf("expected 100 candles, got %d", len(candles))
	}
}

func TestPaginatedFetchTwoPages(t *testing.T) {
	calls := 0
	// First call returns 1000 bars; second returns 50 → pagination ends.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Write(makeFakePage(1_700_000_000_000, 1000, 60_000))
		} else {
			w.Write(makeFakePage(1_700_000_060_000_000, 50, 60_000))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := NewBinanceHistoricalFetcher(dir)
	f.httpClient = srv.Client()
	f.httpClient.Transport = urlRewriteTransport(srv.URL)
	f.rateLimiter = unlimitedTicker()

	from := time.UnixMilli(1_700_000_000_000).UTC()
	to := from.Add(24 * time.Hour)
	candles, err := f.PaginatedFetch("BTCUSDT", "1m", from, to)
	if err != nil {
		t.Fatalf("PaginatedFetch error: %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected >= 2 HTTP calls for pagination, got %d", calls)
	}
	_ = candles
}

func TestCacheWrittenAfterFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(makeFakePage(1_700_000_000_000, 50, 60_000))
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := NewBinanceHistoricalFetcher(dir)
	f.httpClient = srv.Client()
	f.httpClient.Transport = urlRewriteTransport(srv.URL)
	f.rateLimiter = unlimitedTicker()

	from := time.UnixMilli(1_700_000_000_000).UTC()
	to := from.Add(time.Hour)
	_, err := f.PaginatedFetch("BTCUSDT", "1m", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(f.CachePath("BTCUSDT", "1m")); os.IsNotExist(err) {
		t.Fatal("cache file not created")
	}
}

func TestRateLimiterEnforced(t *testing.T) {
	calls := 0
	callTimes := make([]time.Time, 0, 4)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		callTimes = append(callTimes, time.Now())
		// Return small page so pagination ends quickly after 3 pages
		if calls < 3 {
			w.Write(makeFakePage(int64(calls-1)*60_000_000, 1000, 60_000))
		} else {
			w.Write(makeFakePage(int64(calls-1)*60_000_000, 5, 60_000))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	f := NewBinanceHistoricalFetcher(dir)
	f.httpClient = srv.Client()
	f.httpClient.Transport = urlRewriteTransport(srv.URL)
	// Real rate limiter at 400ms for test
	f.rateLimiter = time.Tick(400 * time.Millisecond)

	from := time.UnixMilli(0).UTC()
	to := from.Add(48 * time.Hour)
	_, _ = f.PaginatedFetch("BTCUSDT", "1m", from, to)

	for i := 1; i < len(callTimes); i++ {
		gap := callTimes[i].Sub(callTimes[i-1])
		if gap < 380*time.Millisecond {
			t.Fatalf("rate limiter gap too short between calls %d and %d: %v", i-1, i, gap)
		}
	}
}

func TestResumeFromCache(t *testing.T) {
	// Pre-fill cache with 999 bars.
	dir := t.TempDir()
	preCached := make([]HistoricalCandle, 999)
	base := int64(1_700_000_000_000)
	for i := range preCached {
		preCached[i] = HistoricalCandle{
			OpenTime:  time.UnixMilli(base + int64(i)*60_000).UTC(),
			Close:     50000,
			CloseTime: time.UnixMilli(base + int64(i)*60_000 + 59_999).UTC(),
		}
	}
	data, _ := json.Marshal(preCached)
	os.WriteFile(filepath.Join(dir, "BTCUSDT_1m.json"), data, 0o644)

	fetchedPage := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetchedPage = true
		// Return 10 more bars starting right after the cached ones
		startMs := base + 999*60_000
		w.Write(makeFakePage(startMs, 10, 60_000))
	}))
	defer srv.Close()

	f := NewBinanceHistoricalFetcher(dir)
	f.httpClient = srv.Client()
	f.httpClient.Transport = urlRewriteTransport(srv.URL)
	f.rateLimiter = unlimitedTicker()

	from := time.UnixMilli(base).UTC()
	to := from.Add(2 * time.Hour)
	candles, err := f.PaginatedFetch("BTCUSDT", "1m", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fetchedPage {
		t.Fatal("expected a fetch call for bars after cache, got none")
	}
	if len(candles) < 999 {
		t.Fatalf("expected at least 999 candles (cached + new), got %d", len(candles))
	}
}

// ── test helpers ──────────────────────────────────────────────────────────────

// urlRewriteTransport rewrites all requests to point at the test server.
type rewriteTransport struct {
	base    string
	wrapped http.RoundTripper
}

func urlRewriteTransport(base string) http.RoundTripper {
	return &rewriteTransport{base: base, wrapped: http.DefaultTransport}
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = req.URL.Host
	// Point to test server host
	u := *req.URL
	u.Host = t.base[len("http://"):]
	u.Scheme = "http"
	req2.URL = &u
	return t.wrapped.RoundTrip(req2)
}

func unlimitedTicker() <-chan time.Time {
	ch := make(chan time.Time, 100)
	for i := 0; i < 100; i++ {
		ch <- time.Now()
	}
	return ch
}
