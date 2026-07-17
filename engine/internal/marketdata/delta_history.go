package marketdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// deltaCandlesResponse mirrors the Delta Exchange /v2/history/candles JSON shape.
// Confirmed live via curl against https://api.india.delta.exchange on 2026-07-11:
// {"success":true,"result":[{"time":unix_sec,"open":..,"high":..,"low":..,"close":..,"volume":..}, ...]}
// result is an array of OBJECTS (not arrays of arrays), newest-first.
type deltaCandlesResponse struct {
	Success bool `json:"success"`
	Result  []struct {
		Time   int64   `json:"time"`
		Open   float64 `json:"open"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
		Volume float64 `json:"volume"`
	} `json:"result"`
}

// FetchDeltaHistoricalCandles fetches real historical OHLCV candles from Delta
// Exchange's public REST API (no auth required) for the given symbol,
// resolution ("1m","5m","15m","1h","4h","1d" etc.), and [from,to) UTC window.
// Delta caps each request's row count, so this paginates by re-querying with
// end = earliest returned candle's time until the whole window is covered.
func FetchDeltaHistoricalCandles(symbol, resolution string, from, to time.Time) ([]HistoricalCandle, error) {
	const baseURL = "https://api.india.delta.exchange/v2/history/candles"
	client := &http.Client{Timeout: 30 * time.Second}

	startSec := from.Unix()
	endSec := to.Unix()

	seen := map[int64]HistoricalCandle{}
	cursorEnd := endSec

	for {
		url := fmt.Sprintf("%s?symbol=%s&resolution=%s&start=%d&end=%d",
			baseURL, symbol, resolution, startSec, cursorEnd)

		resp, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("FetchDeltaHistoricalCandles GET: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("FetchDeltaHistoricalCandles read body: %w", err)
		}
		var parsed deltaCandlesResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("FetchDeltaHistoricalCandles parse (status %d): %w — body: %s", resp.StatusCode, err, truncateStr(string(body), 300))
		}
		if !parsed.Success {
			return nil, fmt.Errorf("FetchDeltaHistoricalCandles: API returned success=false — body: %s", truncateStr(string(body), 300))
		}
		if len(parsed.Result) == 0 {
			break
		}

		minTime := int64(1 << 62)
		for _, c := range parsed.Result {
			if c.Time < minTime {
				minTime = c.Time
			}
			if _, exists := seen[c.Time]; !exists {
				ot := time.Unix(c.Time, 0).UTC()
				seen[c.Time] = HistoricalCandle{
					OpenTime:  ot,
					Open:      c.Open,
					High:      c.High,
					Low:       c.Low,
					Close:     c.Close,
					Volume:    c.Volume,
					CloseTime: ot, // exact close time depends on resolution; caller only needs OpenTime for filtering
				}
			}
		}

		if minTime <= startSec || minTime >= cursorEnd {
			// No further progress possible (single page covered the whole range).
			break
		}
		cursorEnd = minTime - 1
	}

	out := make([]HistoricalCandle, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime.Before(out[j].OpenTime) })
	return out, nil
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
