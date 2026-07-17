// Command delta_fetch_history caches REAL Delta Exchange BTCUSD candle history
// to disk for the BTC Pre-Live Engine (Phase 1 of the BTC pre-live plan).
//
// It fetches the last N days (default 365) of candles for every resolution the
// strategy ContextBuilder and the qualification harness read (1m/5m/15m/1h/4h/1d)
// via the same public, no-auth Delta REST endpoint delta_full_qualify already
// uses (marketdata.FetchDeltaHistoricalCandles), and writes one JSON file per
// resolution plus a manifest to --cache-dir (default data/historical/delta).
//
// Additive tooling only: nothing in the existing engine, pre_live process, or
// client reads these files yet — Phase 2 (btc_qualify_25) will consume them.
//
// Usage:
//
//	go run ./cmd/delta_fetch_history                     # 365 days, BTCUSD
//	go run ./cmd/delta_fetch_history -days 365 -symbol BTCUSD
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"antigravity-engine/internal/marketdata"
)

type cachedCandle struct {
	Time   int64   `json:"time"` // unix seconds, UTC, candle open
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

type manifestEntry struct {
	Resolution string `json:"resolution"`
	File       string `json:"file"`
	Candles    int    `json:"candles"`
	FirstUTC   string `json:"first_utc"`
	LastUTC    string `json:"last_utc"`
}

type manifest struct {
	Symbol    string          `json:"symbol"`
	Source    string          `json:"source"`
	FetchedAt string          `json:"fetched_at_utc"`
	FromUTC   string          `json:"from_utc"`
	ToUTC     string          `json:"to_utc"`
	Entries   []manifestEntry `json:"entries"`
}

func main() {
	symbol := flag.String("symbol", "BTCUSD", "Delta Exchange symbol")
	days := flag.Int("days", 365, "how many days of history to fetch")
	cacheDir := flag.String("cache-dir", "data/historical/delta", "output directory")
	flag.Parse()

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -*days)

	if err := os.MkdirAll(*cacheDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *cacheDir, err)
	}

	fmt.Printf("=== Delta Exchange history cache: %s, last %d days ===\n", *symbol, *days)
	fmt.Printf("Window: %s -> %s UTC\n\n", from.Format("2006-01-02"), to.Format("2006-01-02"))

	resolutions := []string{"1m", "5m", "15m", "1h", "4h", "1d"}
	man := manifest{
		Symbol:    *symbol,
		Source:    "https://api.india.delta.exchange/v2/history/candles",
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		FromUTC:   from.Format(time.RFC3339),
		ToUTC:     to.Format(time.RFC3339),
	}

	for _, res := range resolutions {
		start := time.Now()
		fmt.Printf("[%s] fetching...", res)
		candles, err := marketdata.FetchDeltaHistoricalCandles(*symbol, res, from, to)
		if err != nil {
			log.Fatalf("\n[%s] fetch failed: %v", res, err)
		}
		if len(candles) == 0 {
			log.Fatalf("\n[%s] Delta returned zero candles — aborting rather than caching an empty file", res)
		}

		out := make([]cachedCandle, 0, len(candles))
		for _, c := range candles {
			out = append(out, cachedCandle{
				Time: c.OpenTime.Unix(), Open: c.Open, High: c.High,
				Low: c.Low, Close: c.Close, Volume: c.Volume,
			})
		}

		fileName := fmt.Sprintf("%s_%s.json", *symbol, res)
		path := filepath.Join(*cacheDir, fileName)
		data, err := json.Marshal(out)
		if err != nil {
			log.Fatalf("[%s] marshal: %v", res, err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatalf("[%s] write %s: %v", res, path, err)
		}

		first := candles[0].OpenTime
		last := candles[len(candles)-1].OpenTime
		man.Entries = append(man.Entries, manifestEntry{
			Resolution: res, File: fileName, Candles: len(out),
			FirstUTC: first.Format(time.RFC3339), LastUTC: last.Format(time.RFC3339),
		})
		fmt.Printf("\r[%s] %d candles cached (%s -> %s) in %s\n",
			res, len(out), first.Format("2006-01-02"), last.Format("2006-01-02"),
			time.Since(start).Round(time.Second))
	}

	manPath := filepath.Join(*cacheDir, "manifest.json")
	manData, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		log.Fatalf("manifest marshal: %v", err)
	}
	if err := os.WriteFile(manPath, manData, 0o644); err != nil {
		log.Fatalf("manifest write: %v", err)
	}
	fmt.Printf("\nManifest written: %s\n", manPath)
	fmt.Println("Done — cache ready for Phase 2 (btc_qualify_25).")
}
