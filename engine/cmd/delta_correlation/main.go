// Command delta_correlation runs all 26 qualified strategies over the FULL
// real Delta history (2023-12-29 -> 2026-07-11, train+validate combined,
// same real data used to qualify them) and groups strategies whose trades
// fire on the same bar + same direction into correlated clusters, so
// capital can be allocated per-cluster instead of per-strategy-name.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"antigravity-engine/internal/backtest"
	v3 "antigravity-engine/internal/backtest/v3"
	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

func writeJSON(path string, v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Printf("warning: could not write %s: %v", path, err)
	}
}

const symbol = "BTCUSD"

var all26 = []string{
	"Lower_High_CMF_Bear_Short", "OBV_Bear_CMF_Short", "Three_Bear_Candles_Short",
	"Three_Bear_Candles_ADX_Short", "BB_Squeeze_EFI_ADX_Short", "Lower_High_Confirm_Short",
	"HMA_CMF_Bear_Short", "OBV_Slope_EFI_ADX_Short", "Fisher_Zero_CMF_ADX_Short",
	"CMF_Extreme_Bear_Short", "CMF_Cross_Bullish_Long", "Two_Bar_Bear_Reversal_Short",
	"Fisher_Deep_Bear_Short", "Close_Low_Range_ADX_Short", "Donchian_CMF_Bear_Short",
	"Keltner_CMF_Bear_Short", "BB_Width_Expand_Bear_Short", "Close_Low_Range_Bear_Short",
	"Two_Bar_Bear_Reversal_ADX_Short", "Bearish_Engulf_EFI_ADX_Short", "Bearish_Engulfing_Short",
	"ADX_Surge_Breakout_Long",
	"Donchian_ADX_CMF_Bear_Short", "HMA_ADX_CMF_Bear_Short",
	"HMA_Slope_EMA_MACD_Bear_Short", "EMA_MACD_CMF_Bull_Long",
}

func main() {
	outFile := flag.String("out", "data/delta_correlation_results.json", "output JSON path")
	flag.Parse()

	now := time.Now().UTC().Truncate(time.Hour)
	realStart, err := time.Parse("2006-01-02", "2023-12-29")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Fetching FULL real Delta history %s -> %s for correlation analysis...\n", realStart.Format("2006-01-02"), now.Format("2006-01-02"))

	fetch := func(res string) []marketdata.HistoricalCandle {
		c, err := marketdata.FetchDeltaHistoricalCandles(symbol, res, realStart, now)
		if err != nil {
			log.Fatalf("fetch %s: %v", res, err)
		}
		fmt.Printf("  fetched %-4s: %d candles\n", res, len(c))
		return c
	}
	ds := marketdata.MTFDataset{Symbol: symbol, From: realStart, To: now}
	ds.Candles1m = fetch("1m")
	ds.Candles5m = fetch("5m")
	ds.Candles15m = fetch("15m")
	ds.Candles1h = fetch("1h")
	ds.Candles4h = fetch("4h")
	fmt.Println()

	all := append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...)
	byName := map[string]scalers.RegistryEntry{}
	for _, e := range all {
		byName[e.Name] = e
	}
	var entries []scalers.RegistryEntry
	for _, name := range all26 {
		e, ok := byName[name]
		if !ok {
			log.Fatalf("strategy %q not found", name)
		}
		entries = append(entries, e)
	}

	cfg := backtest.DefaultRunConfig(symbol) // SizeBTC=0.1
	cfg.V3Config.InitialCapitalUSD = 100_000.0

	runner := backtest.NewBatchRunner(ds, cfg, entries)
	results := runner.RunAll(func(done, total int, name string) {
		if name != "" && done%5 == 0 {
			fmt.Printf("[%d/%d] %s\n", done, total, name)
		}
	})
	fmt.Println()

	// Bucket each strategy's trades into (4h bar, direction) keys.
	type tradeKey struct {
		bar4h string
		dir   string
	}
	stratKeys := map[string]map[tradeKey]bool{}
	stratTradeCount := map[string]int{}
	for _, r := range results {
		km := map[tradeKey]bool{}
		for _, t := range r.V3Result.Trades {
			bar := t.OpenedAt.Truncate(4 * time.Hour).Format(time.RFC3339)
			km[tradeKey{bar, string(t.Side)}] = true
		}
		stratKeys[r.StrategyName] = km
		stratTradeCount[r.StrategyName] = len(r.V3Result.Trades)
	}

	// Pairwise overlap: fraction of A's (bar,dir) trade-slots that B also
	// has a trade in (same bar, same direction).
	type pairOverlap struct {
		a, b       string
		overlapPct float64 // relative to the smaller-trade-count strategy
	}
	var pairs []pairOverlap
	for i := 0; i < len(all26); i++ {
		for j := i + 1; j < len(all26); j++ {
			a, b := all26[i], all26[j]
			ka, kb := stratKeys[a], stratKeys[b]
			if len(ka) == 0 || len(kb) == 0 {
				continue
			}
			shared := 0
			small := ka
			if len(kb) < len(ka) {
				small = kb
			}
			other := kb
			if len(kb) < len(ka) {
				other = ka
			}
			for k := range small {
				if other[k] {
					shared++
				}
			}
			pct := float64(shared) / float64(len(small)) * 100
			if pct > 0 {
				pairs = append(pairs, pairOverlap{a, b, pct})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].overlapPct > pairs[j].overlapPct })

	fmt.Println("=== Pairwise trade overlap (same 4h bar + same direction), threshold for clustering >50% ===")
	fmt.Printf("%-38s %-38s %8s\n", "Strategy A", "Strategy B", "Overlap%")
	for _, p := range pairs {
		if p.overlapPct >= 20 {
			fmt.Printf("%-38s %-38s %7.1f%%\n", p.a, p.b, p.overlapPct)
		}
	}

	// Union-find clustering on pairs with overlap >= 50%.
	parent := map[string]string{}
	var find func(string) string
	find = func(x string) string {
		if parent[x] == "" {
			parent[x] = x
		}
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for _, n := range all26 {
		find(n)
	}
	for _, p := range pairs {
		if p.overlapPct >= 50 {
			union(p.a, p.b)
		}
	}
	clusters := map[string][]string{}
	for _, n := range all26 {
		r := find(n)
		clusters[r] = append(clusters[r], n)
	}
	var clusterList [][]string
	for _, members := range clusters {
		sort.Strings(members)
		clusterList = append(clusterList, members)
	}
	sort.Slice(clusterList, func(i, j int) bool { return len(clusterList[i]) > len(clusterList[j]) })

	fmt.Println("\n=== CLUSTERS (>=50% same-bar-same-direction overlap => treated as one bet) ===")
	for i, c := range clusterList {
		fmt.Printf("Cluster %d (%d members): %v\n", i+1, len(c), c)
	}

	out := map[string]interface{}{
		"symbol":            symbol,
		"full_range":        []string{realStart.Format("2006-01-02"), now.Format("2006-01-02")},
		"strategy_trades":   stratTradeCount,
		"pairwise_overlaps": pairs,
		"clusters":          clusterList,
		"ran_at":            time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(*outFile, out)
	fmt.Printf("\nSaved: %s\n", *outFile)
	_ = v3.V3Trade{}
}
