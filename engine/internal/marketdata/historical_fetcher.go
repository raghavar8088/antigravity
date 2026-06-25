package marketdata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// HistoricalCandle is a single OHLCV bar from historical data.
type HistoricalCandle struct {
	OpenTime  time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime time.Time
}

// HistoricalFundingEntry is a single funding rate reading.
type HistoricalFundingEntry struct {
	Time time.Time
	Rate float64
}

// HistoricalOIEntry is a single open-interest snapshot.
type HistoricalOIEntry struct {
	Time time.Time
	OI   float64
}

// BinanceHistoricalFetcher fetches and caches historical market data from Binance.
type BinanceHistoricalFetcher struct {
	httpClient  *http.Client
	cacheDir    string
	rateLimiter <-chan time.Time
	OnProgress  func(fetched, total int, interval string)
}

// NewBinanceHistoricalFetcher creates a fetcher that caches files under cacheDir.
func NewBinanceHistoricalFetcher(cacheDir string) *BinanceHistoricalFetcher {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		// best-effort; CachePath will fail on write if dir is missing
	}
	return &BinanceHistoricalFetcher{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		cacheDir:    cacheDir,
		rateLimiter: time.Tick(500 * time.Millisecond),
	}
}

// CachePath returns the JSON cache file path for a symbol+interval combination.
func (f *BinanceHistoricalFetcher) CachePath(symbol, interval string) string {
	return filepath.Join(f.cacheDir, fmt.Sprintf("%s_%s.json", symbol, interval))
}

// IsCached returns true when a cache file exists and spans at least [from, to].
func (f *BinanceHistoricalFetcher) IsCached(symbol, interval string, from, to time.Time) bool {
	candles, err := f.readCache(symbol, interval)
	if err != nil || len(candles) == 0 {
		return false
	}
	first := candles[0].OpenTime
	last := candles[len(candles)-1].CloseTime
	return !first.After(from) && !last.Before(to)
}

// FetchKlines fetches up to 1000 bars in one HTTP request.
// startMs and endMs are Unix milliseconds (0 = no bound).
func (f *BinanceHistoricalFetcher) FetchKlines(
	symbol, interval string, startMs, endMs int64,
) ([]HistoricalCandle, error) {
	<-f.rateLimiter

	url := fmt.Sprintf(
		"https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=1000",
		symbol, interval,
	)
	if startMs > 0 {
		url += "&startTime=" + strconv.FormatInt(startMs, 10)
	}
	if endMs > 0 {
		url += "&endTime=" + strconv.FormatInt(endMs, 10)
	}

	resp, err := f.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("FetchKlines HTTP: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("FetchKlines read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FetchKlines status %d: %s", resp.StatusCode, string(body))
	}

	return parseBinanceKlines(body)
}

// PaginatedFetch fetches the full date range, paginating automatically.
// Writes partial results to cache so interrupted runs can resume.
func (f *BinanceHistoricalFetcher) PaginatedFetch(
	symbol, interval string, from, to time.Time,
) ([]HistoricalCandle, error) {
	// Resume from last cached bar if possible.
	cached, _ := f.readCache(symbol, interval)
	startMs := from.UnixMilli()
	if len(cached) > 0 {
		last := cached[len(cached)-1]
		if last.CloseTime.After(from) {
			startMs = last.CloseTime.UnixMilli() + 1
		}
	}
	endMs := to.UnixMilli()
	allCandles := append([]HistoricalCandle(nil), cached...)

	page := 0
	for {
		batch, err := f.FetchKlines(symbol, interval, startMs, endMs)
		if err != nil {
			// persist what we have so next run can resume
			_ = f.writeCache(symbol, interval, allCandles)
			return allCandles, fmt.Errorf("PaginatedFetch page %d: %w", page, err)
		}
		if len(batch) == 0 {
			break
		}
		allCandles = append(allCandles, batch...)
		page++

		if f.OnProgress != nil {
			f.OnProgress(len(allCandles), 0, interval)
		}

		last := batch[len(batch)-1]
		// Done if we received fewer than 1000 bars or last bar covers end time.
		if len(batch) < 1000 || !last.CloseTime.Before(to) {
			break
		}
		startMs = last.CloseTime.UnixMilli() + 1
	}

	if err := f.writeCache(symbol, interval, allCandles); err != nil {
		return allCandles, fmt.Errorf("PaginatedFetch cache write: %w", err)
	}
	return allCandles, nil
}

// FetchFundingHistory fetches all historical funding rates for a symbol.
func (f *BinanceHistoricalFetcher) FetchFundingHistory(
	symbol string, from, to time.Time,
) ([]HistoricalFundingEntry, error) {
	var all []HistoricalFundingEntry
	startMs := from.UnixMilli()
	endMs := to.UnixMilli()

	for {
		<-f.rateLimiter
		url := fmt.Sprintf(
			"https://fapi.binance.com/fapi/v1/fundingRate?symbol=%s&startTime=%d&endTime=%d&limit=1000",
			symbol, startMs, endMs,
		)
		resp, err := f.httpClient.Get(url)
		if err != nil {
			return all, fmt.Errorf("FetchFundingHistory HTTP: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return all, fmt.Errorf("FetchFundingHistory status %d: %s", resp.StatusCode, string(body))
		}

		var raw []struct {
			FundingTime int64   `json:"fundingTime"`
			FundingRate string  `json:"fundingRate"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return all, fmt.Errorf("FetchFundingHistory parse: %w", err)
		}
		if len(raw) == 0 {
			break
		}
		for _, r := range raw {
			rate, _ := strconv.ParseFloat(r.FundingRate, 64)
			all = append(all, HistoricalFundingEntry{
				Time: time.UnixMilli(r.FundingTime).UTC(),
				Rate: rate,
			})
		}
		if len(raw) < 1000 {
			break
		}
		startMs = raw[len(raw)-1].FundingTime + 1
	}

	// Write to dedicated cache file
	path := filepath.Join(f.cacheDir, fmt.Sprintf("%s_funding.json", symbol))
	if data, err := json.Marshal(all); err == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
	return all, nil
}

// FetchOIHistory fetches historical open-interest snapshots.
func (f *BinanceHistoricalFetcher) FetchOIHistory(
	symbol, period string, from, to time.Time,
) ([]HistoricalOIEntry, error) {
	if period == "" {
		period = "5m"
	}
	var all []HistoricalOIEntry
	startMs := from.UnixMilli()
	endMs := to.UnixMilli()

	for {
		<-f.rateLimiter
		url := fmt.Sprintf(
			"https://fapi.binance.com/futures/data/openInterestHist?symbol=%s&period=%s&startTime=%d&endTime=%d&limit=500",
			symbol, period, startMs, endMs,
		)
		resp, err := f.httpClient.Get(url)
		if err != nil {
			return all, fmt.Errorf("FetchOIHistory HTTP: %w", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return all, fmt.Errorf("FetchOIHistory status %d: %s", resp.StatusCode, string(body))
		}

		var raw []struct {
			Timestamp        int64  `json:"timestamp"`
			SumOpenInterest  string `json:"sumOpenInterest"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return all, fmt.Errorf("FetchOIHistory parse: %w", err)
		}
		if len(raw) == 0 {
			break
		}
		for _, r := range raw {
			oi, _ := strconv.ParseFloat(r.SumOpenInterest, 64)
			all = append(all, HistoricalOIEntry{
				Time: time.UnixMilli(r.Timestamp).UTC(),
				OI:   oi,
			})
		}
		if len(raw) < 500 {
			break
		}
		startMs = raw[len(raw)-1].Timestamp + 1
	}

	path := filepath.Join(f.cacheDir, fmt.Sprintf("%s_oi.json", symbol))
	if data, err := json.Marshal(all); err == nil {
		_ = os.WriteFile(path, data, 0o644)
	}
	return all, nil
}

// ── private helpers ────────────────────────────────────────────────────────────

func (f *BinanceHistoricalFetcher) readCache(symbol, interval string) ([]HistoricalCandle, error) {
	data, err := os.ReadFile(f.CachePath(symbol, interval))
	if err != nil {
		return nil, err
	}
	var candles []HistoricalCandle
	if err := json.Unmarshal(data, &candles); err != nil {
		return nil, err
	}
	return candles, nil
}

func (f *BinanceHistoricalFetcher) writeCache(symbol, interval string, candles []HistoricalCandle) error {
	data, err := json.Marshal(candles)
	if err != nil {
		return err
	}
	return os.WriteFile(f.CachePath(symbol, interval), data, 0o644)
}

// parseBinanceKlines decodes the Binance klines JSON array.
// Each element is: [openTime, open, high, low, close, volume, closeTime, ...]
func parseBinanceKlines(body []byte) ([]HistoricalCandle, error) {
	var raw [][]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parseBinanceKlines: %w", err)
	}
	out := make([]HistoricalCandle, 0, len(raw))
	for _, r := range raw {
		if len(r) < 7 {
			continue
		}
		var openMs, closeMs int64
		if err := json.Unmarshal(r[0], &openMs); err != nil {
			continue
		}
		if err := json.Unmarshal(r[6], &closeMs); err != nil {
			continue
		}
		var openS, highS, lowS, closeS, volS string
		_ = json.Unmarshal(r[1], &openS)
		_ = json.Unmarshal(r[2], &highS)
		_ = json.Unmarshal(r[3], &lowS)
		_ = json.Unmarshal(r[4], &closeS)
		_ = json.Unmarshal(r[5], &volS)

		o, _ := strconv.ParseFloat(openS, 64)
		h, _ := strconv.ParseFloat(highS, 64)
		l, _ := strconv.ParseFloat(lowS, 64)
		c, _ := strconv.ParseFloat(closeS, 64)
		v, _ := strconv.ParseFloat(volS, 64)

		out = append(out, HistoricalCandle{
			OpenTime:  time.UnixMilli(openMs).UTC(),
			Open:      o,
			High:      h,
			Low:       l,
			Close:     c,
			Volume:    v,
			CloseTime: time.UnixMilli(closeMs).UTC(),
		})
	}
	return out, nil
}
