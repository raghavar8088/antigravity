package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// Delta returns candles newest-first. The packs index the LAST element as the
// most recent, so a series stored in venue order makes every indicator read the
// past as the present — and nothing errors.
func TestHTFStore_SortsOldestFirst(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type c struct {
			Time                           int64   `json:"time"`
			Open, High, Low, Close, Volume float64 `json:"open,omitempty"`
		}
		// Newest first, as the venue sends it.
		out := map[string]any{"result": []map[string]any{
			{"time": now.Unix(), "open": 3.0, "high": 3.1, "low": 2.9, "close": 3.0, "volume": 1},
			{"time": now.Add(-time.Hour).Unix(), "open": 2.0, "high": 2.1, "low": 1.9, "close": 2.0, "volume": 1},
			{"time": now.Add(-2 * time.Hour).Unix(), "open": 1.0, "high": 1.1, "low": 0.9, "close": 1.0, "volume": 1},
		}}
		_ = c{}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	s := newHTFStore(srv.URL)
	got, err := s.fetch(context.Background(), "TSTUSD", scalers.TF1h, 3)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d candles, want 3", len(got))
	}
	for i := 1; i < len(got); i++ {
		if !got[i-1].OpenTime.Before(got[i].OpenTime) {
			t.Fatalf("candles are not oldest-first at index %d", i)
		}
	}
	if got[len(got)-1].Close != 3.0 {
		t.Errorf("last candle close %.2f, want the most recent (3.00)", got[len(got)-1].Close)
	}
}

// A candle with no price must be dropped, not stored as a zero.
//
// A zero close reaching an indicator reads as a real print at zero, which moves
// every average through the floor — the same class of failure as an
// uncalculated value being treated as a measurement.
func TestHTFStore_DropsPricelessCandles(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{
			{"time": now.Unix(), "open": 1.0, "high": 1.1, "low": 0.9, "close": 1.0, "volume": 1},
			{"time": now.Add(-time.Hour).Unix(), "open": 0, "high": 0, "low": 0, "close": 0, "volume": 0},
		}})
	}))
	defer srv.Close()

	got, err := newHTFStore(srv.URL).fetch(context.Background(), "X", scalers.TF1h, 2)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candles, want 1 — the zero-priced candle should be dropped", len(got))
	}
}

// A failed refresh must keep the previous series. Stale data beats none: an
// empty series makes every strategy on that timeframe silently stop, which is
// the state this store exists to end.
func TestHTFStore_FailedRefreshKeepsTheOldSeries(t *testing.T) {
	fail := false
	now := time.Now().UTC().Truncate(time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []map[string]any{
			{"time": now.Unix(), "open": 1.0, "high": 1.1, "low": 0.9, "close": 1.0, "volume": 1},
		}})
	}))
	defer srv.Close()

	s := newHTFStore(srv.URL)
	s.refreshOne(context.Background(), "X", scalers.TF1h)
	if _, ok := s.Get("X", scalers.TF1h); !ok {
		t.Fatal("first refresh stored nothing")
	}

	fail = true
	s.refreshOne(context.Background(), "X", scalers.TF1h)
	if _, ok := s.Get("X", scalers.TF1h); !ok {
		t.Error("a failed refresh discarded the previous series")
	}
}

// Get must never block on the network — it runs inside the bar loop, where one
// slow response would stall every symbol behind it.
func TestHTFStore_GetDoesNotFetch(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_ = json.NewEncoder(w).Encode(map[string]any{"result": []any{}})
	}))
	defer srv.Close()

	s := newHTFStore(srv.URL)
	for i := 0; i < 5; i++ {
		if _, ok := s.Get("X", scalers.TF4h); ok {
			t.Fatal("Get returned data that was never fetched")
		}
	}
	if hits != 0 {
		t.Errorf("Get made %d network calls; it must serve from cache only", hits)
	}
}

// Refresh cadence must scale with the candle — refetching a daily series every
// minute burns the rate limit to learn nothing.
func TestHTFRefreshEvery_ScalesWithTheCandle(t *testing.T) {
	if htfRefreshEvery(scalers.TF1h) >= htfRefreshEvery(scalers.TF4h) {
		t.Error("1h should refresh more often than 4h")
	}
	if htfRefreshEvery(scalers.TF4h) >= htfRefreshEvery(scalers.TF1d) {
		t.Error("4h should refresh more often than 1d")
	}
}
