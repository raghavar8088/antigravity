package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// htf_store.go — higher-timeframe candles fetched from the venue.
//
// WHY THIS EXISTS
//
// The desk keeps a 6,000-bar 1-minute ring — 4.2 days — and resampled every
// higher timeframe from it. That works for 15m (400 candles) and 30m (200), and
// silently fails for everything longer:
//
//	1h    100 candles available, 120 needed
//	4h     25
//	1d      4
//
// Every strategy on those timeframes had ok=false from its indicators and
// returned no signal, forever. 96 of the 160 strategies in the pack — 60% —
// could never fire, and would have looked exactly like strategies that found no
// setups. Resampling cannot fix it: 120 daily candles needs 172,800 one-minute
// bars per symbol, and there are ~97 symbols.
//
// Delta serves 1h/4h/1d natively. This fetches them, caches them, and refreshes
// in the background so the bar-processing path never waits on a network call.

// htfSeries is one symbol's candles on one timeframe.
type htfSeries struct {
	candles   []scalers.Candle
	fetchedAt time.Time
	err       error
}

// htfStore caches higher-timeframe candles per (symbol, timeframe).
//
// Serves STALE data in preference to none. A four-hour-old 1d series is a far
// better input than an empty one: the alternative is the exact silent-nothing
// this type exists to end.
type htfStore struct {
	mu   sync.RWMutex
	data map[string]htfSeries // key: SYMBOL|tf

	base string
	http *http.Client
}

func newHTFStore(baseURL string) *htfStore {
	return &htfStore{
		data: map[string]htfSeries{},
		base: baseURL,
		http: &http.Client{Timeout: 25 * time.Second},
	}
}

func htfKey(symbol string, tf scalers.HigherTF) string { return symbol + "|" + string(tf) }

// htfRefreshEvery is how often each (symbol, timeframe) is refetched.
//
// Scaled to the candle: refetching a daily series every minute would burn the
// rate limit to learn nothing, since the series changes once a day.
func htfRefreshEvery(tf scalers.HigherTF) time.Duration {
	switch tf {
	case scalers.TF1h:
		return 20 * time.Minute
	case scalers.TF4h:
		return time.Hour
	case scalers.TF1d:
		return 4 * time.Hour
	}
	return time.Hour
}

// Get returns the cached series, and whether one exists.
//
// Never fetches. The caller is the bar loop, and a synchronous network call
// there would stall every symbol behind one slow response.
func (s *htfStore) Get(symbol string, tf scalers.HigherTF) ([]scalers.Candle, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[htfKey(symbol, tf)]
	if !ok || len(v.candles) == 0 {
		return nil, false
	}
	return v.candles, true
}

// deltaResolution maps a timeframe to Delta's resolution parameter.
func deltaResolution(tf scalers.HigherTF) string {
	switch tf {
	case scalers.TF1h:
		return "1h"
	case scalers.TF4h:
		return "4h"
	case scalers.TF1d:
		return "1d"
	}
	return ""
}

// fetch pulls one series from the venue.
func (s *htfStore) fetch(ctx context.Context, symbol string, tf scalers.HigherTF, want int) ([]scalers.Candle, error) {
	res := deltaResolution(tf)
	if res == "" {
		return nil, fmt.Errorf("no venue resolution for %s", tf)
	}
	// Ask for more than needed: the venue may return fewer for a young or thin
	// contract, and a series that is short by one candle is still short.
	span := time.Duration(want+40) * tf.Step()
	end := time.Now().UTC()
	start := end.Add(-span)

	url := fmt.Sprintf("%s/v2/history/candles?resolution=%s&symbol=%s&start=%d&end=%d",
		s.base, res, symbol, start.Unix(), end.Unix())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("candles HTTP %d", resp.StatusCode)
	}

	var body struct {
		Result []struct {
			Time   int64       `json:"time"`
			Open   json.Number `json:"open"`
			High   json.Number `json:"high"`
			Low    json.Number `json:"low"`
			Close  json.Number `json:"close"`
			Volume json.Number `json:"volume"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := make([]scalers.Candle, 0, len(body.Result))
	for _, c := range body.Result {
		o, _ := strconv.ParseFloat(c.Open.String(), 64)
		h, _ := strconv.ParseFloat(c.High.String(), 64)
		l, _ := strconv.ParseFloat(c.Low.String(), 64)
		cl, _ := strconv.ParseFloat(c.Close.String(), 64)
		v, _ := strconv.ParseFloat(c.Volume.String(), 64)
		// A candle with no price is not a candle. Dropping it here keeps a
		// zero out of an indicator, where it would read as a real print.
		if cl <= 0 || h < l {
			continue
		}
		out = append(out, scalers.Candle{
			Open: o, High: h, Low: l, Close: cl, Volume: v,
			OpenTime: time.Unix(c.Time, 0).UTC(),
		})
	}
	// Delta returns newest-first; the packs index the last element as the most
	// recent candle. Sorting rather than assuming: a reversed series would make
	// every indicator read the past as the present, and nothing would error.
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime.Before(out[j].OpenTime) })
	return out, nil
}

// refreshOne fetches and stores a single series, keeping the previous one on
// failure.
func (s *htfStore) refreshOne(ctx context.Context, symbol string, tf scalers.HigherTF) {
	want := tf.MinCandles() + 20
	c, err := s.fetch(ctx, symbol, tf, want)
	s.mu.Lock()
	defer s.mu.Unlock()
	k := htfKey(symbol, tf)
	if err != nil || len(c) == 0 {
		prev := s.data[k]
		prev.err = err
		prev.fetchedAt = time.Now()
		s.data[k] = prev // keep whatever we had; stale beats empty
		return
	}
	s.data[k] = htfSeries{candles: c, fetchedAt: time.Now()}
}

// Run refreshes every (symbol, timeframe) on its own cadence until ctx ends.
//
// Sequential with a small delay rather than parallel: ~97 symbols x 3
// timeframes is 291 requests, and firing them at once is how a desk gets
// rate-limited into having no data at all — which is the state it is trying to
// leave.
func (s *htfStore) Run(ctx context.Context, symbols func() []string) {
	tfs := []scalers.HigherTF{scalers.TF1h, scalers.TF4h, scalers.TF1d}
	next := map[string]time.Time{}

	for {
		if ctx.Err() != nil {
			return
		}
		did, failed := 0, 0
		for _, sym := range symbols() {
			for _, tf := range tfs {
				k := htfKey(sym, tf)
				if t, ok := next[k]; ok && time.Now().Before(t) {
					continue
				}
				s.refreshOne(ctx, sym, tf)
				next[k] = time.Now().Add(htfRefreshEvery(tf))
				did++
				s.mu.RLock()
				if v := s.data[k]; v.err != nil || len(v.candles) == 0 {
					failed++
				}
				s.mu.RUnlock()

				select {
				case <-ctx.Done():
					return
				case <-time.After(150 * time.Millisecond):
				}
			}
		}
		if did > 0 {
			log.Printf("[HTF] refreshed %d series (%d without usable data) — %s",
				did, failed, s.coverage())
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

// coverage summarises how many series are usable, per timeframe.
//
// Reported because "the fetcher is running" and "the strategies can now
// evaluate" are different claims, and only the second one matters.
func (s *htfStore) coverage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type row struct{ ok, total int }
	byTF := map[scalers.HigherTF]*row{}
	for k, v := range s.data {
		var tf scalers.HigherTF
		switch {
		case len(k) > 3 && k[len(k)-3:] == "|1h":
			tf = scalers.TF1h
		case len(k) > 3 && k[len(k)-3:] == "|4h":
			tf = scalers.TF4h
		case len(k) > 3 && k[len(k)-3:] == "|1d":
			tf = scalers.TF1d
		default:
			continue
		}
		if byTF[tf] == nil {
			byTF[tf] = &row{}
		}
		byTF[tf].total++
		if len(v.candles) >= tf.MinCandles() {
			byTF[tf].ok++
		}
	}
	out := ""
	for _, tf := range []scalers.HigherTF{scalers.TF1h, scalers.TF4h, scalers.TF1d} {
		r := byTF[tf]
		if r == nil {
			continue
		}
		if out != "" {
			out += ", "
		}
		out += fmt.Sprintf("%s %d/%d tradable", tf, r.ok, r.total)
	}
	if out == "" {
		return "no series yet"
	}
	return out
}
