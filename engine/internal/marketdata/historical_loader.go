package marketdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MTFDataset holds pre-loaded OHLCV candles across all timeframes for a single symbol.
type MTFDataset struct {
	Symbol       string
	From, To     time.Time
	Candles1m    []HistoricalCandle
	Candles5m    []HistoricalCandle
	Candles15m   []HistoricalCandle
	Candles1h    []HistoricalCandle
	Candles4h    []HistoricalCandle
	Candles1d    []HistoricalCandle
	FundingRates []HistoricalFundingEntry
	OIHistory    []HistoricalOIEntry
}

// HistoricalLoader reads cached historical data from disk.
type HistoricalLoader struct {
	cacheDir string
}

// NewHistoricalLoader creates a loader backed by the given cache directory.
func NewHistoricalLoader(cacheDir string) *HistoricalLoader {
	return &HistoricalLoader{cacheDir: cacheDir}
}

// LoadFromCache reads the cached JSON file for a symbol+interval combination.
func (l *HistoricalLoader) LoadFromCache(symbol, interval string) ([]HistoricalCandle, error) {
	path := filepath.Join(l.cacheDir, fmt.Sprintf("%s_%s.json", symbol, interval))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadFromCache %s %s: %w", symbol, interval, err)
	}
	var candles []HistoricalCandle
	if err := json.Unmarshal(data, &candles); err != nil {
		return nil, fmt.Errorf("LoadFromCache parse %s %s: %w", symbol, interval, err)
	}
	return candles, nil
}

// LoadFundingFromCache reads the cached funding rate history.
func (l *HistoricalLoader) LoadFundingFromCache(symbol string) ([]HistoricalFundingEntry, error) {
	path := filepath.Join(l.cacheDir, fmt.Sprintf("%s_funding.json", symbol))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadFundingFromCache %s: %w", symbol, err)
	}
	var entries []HistoricalFundingEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("LoadFundingFromCache parse %s: %w", symbol, err)
	}
	return entries, nil
}

// LoadOIFromCache reads the cached open-interest history.
func (l *HistoricalLoader) LoadOIFromCache(symbol string) ([]HistoricalOIEntry, error) {
	path := filepath.Join(l.cacheDir, fmt.Sprintf("%s_oi.json", symbol))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadOIFromCache %s: %w", symbol, err)
	}
	var entries []HistoricalOIEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("LoadOIFromCache parse %s: %w", symbol, err)
	}
	return entries, nil
}

// BuildMTFDataset loads all timeframes for a symbol, filters to [from, to],
// and returns a populated MTFDataset.
func (l *HistoricalLoader) BuildMTFDataset(symbol string, from, to time.Time) (MTFDataset, error) {
	ds := MTFDataset{Symbol: symbol, From: from, To: to}

	load := func(interval string) ([]HistoricalCandle, error) {
		candles, err := l.LoadFromCache(symbol, interval)
		if err != nil {
			return nil, err
		}
		return filterCandles(candles, from, to), nil
	}

	var err error
	if ds.Candles1m, err = load("1m"); err != nil {
		return ds, fmt.Errorf("BuildMTFDataset 1m: %w", err)
	}
	if ds.Candles5m, err = load("5m"); err != nil {
		return ds, fmt.Errorf("BuildMTFDataset 5m: %w", err)
	}
	if ds.Candles15m, err = load("15m"); err != nil {
		return ds, fmt.Errorf("BuildMTFDataset 15m: %w", err)
	}
	if ds.Candles1h, err = load("1h"); err != nil {
		return ds, fmt.Errorf("BuildMTFDataset 1h: %w", err)
	}
	if ds.Candles4h, err = load("4h"); err != nil {
		return ds, fmt.Errorf("BuildMTFDataset 4h: %w", err)
	}
	// 1d is optional — don't fail if missing
	if c, e := load("1d"); e == nil {
		ds.Candles1d = c
	}

	// Funding and OI are optional — degrade gracefully
	if funding, e := l.LoadFundingFromCache(symbol); e == nil {
		ds.FundingRates = filterFunding(funding, from, to)
	}
	if oi, e := l.LoadOIFromCache(symbol); e == nil {
		ds.OIHistory = filterOI(oi, from, to)
	}

	return ds, nil
}

// EnsureData checks whether full [from, to] data is cached; fetches missing intervals.
func (l *HistoricalLoader) EnsureData(
	symbol string, from, to time.Time,
	fetcher *BinanceHistoricalFetcher,
) error {
	intervals := []string{"1m", "5m", "15m", "1h", "4h", "1d"}
	for _, interval := range intervals {
		if !fetcher.IsCached(symbol, interval, from, to) {
			if _, err := fetcher.PaginatedFetch(symbol, interval, from, to); err != nil {
				return fmt.Errorf("EnsureData %s %s: %w", symbol, interval, err)
			}
		}
	}
	// Funding
	fundingPath := filepath.Join(l.cacheDir, fmt.Sprintf("%s_funding.json", symbol))
	if _, err := os.Stat(fundingPath); os.IsNotExist(err) {
		if _, err := fetcher.FetchFundingHistory(symbol, from, to); err != nil {
			return fmt.Errorf("EnsureData funding: %w", err)
		}
	}
	// OI
	oiPath := filepath.Join(l.cacheDir, fmt.Sprintf("%s_oi.json", symbol))
	if _, err := os.Stat(oiPath); os.IsNotExist(err) {
		if _, err := fetcher.FetchOIHistory(symbol, "5m", from, to); err != nil {
			return fmt.Errorf("EnsureData OI: %w", err)
		}
	}
	return nil
}

// ── filter helpers ─────────────────────────────────────────────────────────────

func filterCandles(candles []HistoricalCandle, from, to time.Time) []HistoricalCandle {
	out := make([]HistoricalCandle, 0, len(candles))
	for _, c := range candles {
		if !c.OpenTime.Before(from) && !c.OpenTime.After(to) {
			out = append(out, c)
		}
	}
	return out
}

func filterFunding(entries []HistoricalFundingEntry, from, to time.Time) []HistoricalFundingEntry {
	out := make([]HistoricalFundingEntry, 0, len(entries))
	for _, e := range entries {
		if !e.Time.Before(from) && !e.Time.After(to) {
			out = append(out, e)
		}
	}
	return out
}

func filterOI(entries []HistoricalOIEntry, from, to time.Time) []HistoricalOIEntry {
	out := make([]HistoricalOIEntry, 0, len(entries))
	for _, e := range entries {
		if !e.Time.Before(from) && !e.Time.After(to) {
			out = append(out, e)
		}
	}
	return out
}
