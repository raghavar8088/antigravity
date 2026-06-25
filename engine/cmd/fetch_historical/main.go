package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"antigravity-engine/internal/marketdata"
)

func main() {
	symbol := flag.String("symbol", "BTCUSDT", "trading symbol")
	fromStr := flag.String("from", "2020-01-01", "start date YYYY-MM-DD")
	toStr := flag.String("to", "", "end date YYYY-MM-DD (default: today)")
	cacheDir := flag.String("cache-dir", "data/historical", "cache directory")
	flag.Parse()

	from, err := time.Parse("2006-01-02", *fromStr)
	if err != nil {
		log.Fatalf("invalid --from: %v", err)
	}
	to := time.Now().UTC().Truncate(24 * time.Hour)
	if *toStr != "" {
		to, err = time.Parse("2006-01-02", *toStr)
		if err != nil {
			log.Fatalf("invalid --to: %v", err)
		}
	}

	if err := os.MkdirAll(*cacheDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *cacheDir, err)
	}

	fetcher := marketdata.NewBinanceHistoricalFetcher(*cacheDir)
	fetcher.OnProgress = func(fetched, total int, interval string) {
		fmt.Printf("\r[%s] fetched %d bars...", interval, fetched)
	}

	intervals := []string{"1m", "5m", "15m", "1h", "4h", "1d"}
	for _, interval := range intervals {
		fmt.Printf("\nFetching %s %s [%s → %s]\n", *symbol, interval,
			from.Format("2006-01-02"), to.Format("2006-01-02"))
		candles, err := fetcher.PaginatedFetch(*symbol, interval, from, to)
		if err != nil {
			log.Printf("WARNING: %s %s fetch error: %v (partial data saved)", *symbol, interval, err)
		} else {
			fmt.Printf("\r[%s] %d bars cached ✓                \n", interval, len(candles))
		}
	}

	fmt.Printf("\nFetching funding history for %s...\n", *symbol)
	funding, err := fetcher.FetchFundingHistory(*symbol, from, to)
	if err != nil {
		log.Printf("WARNING: funding fetch error: %v", err)
	} else {
		fmt.Printf("Funding: %d entries cached ✓\n", len(funding))
	}

	fmt.Printf("\nFetching OI history for %s...\n", *symbol)
	oi, err := fetcher.FetchOIHistory(*symbol, "5m", from, to)
	if err != nil {
		log.Printf("WARNING: OI fetch error: %v", err)
	} else {
		fmt.Printf("OI: %d entries cached ✓\n", len(oi))
	}

	fmt.Println("\nDone. All data cached to:", *cacheDir)
}
