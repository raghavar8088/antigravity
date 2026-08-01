// Command delta_week_backtest fetches one week of REAL BTCUSD candles from
// Delta Exchange's public REST API and runs the two live-whitelisted
// strategies (BB_Squeeze_EFI_ADX_Short, WMA_Bear_Cross_Short) through the
// real V3 backtest engine (internal/backtest), then prints a per-day
// capital-used / PnL / ROI table.
//
// This is a one-off analysis script — not production code. It reuses the
// actual strategy Evaluate() logic and the actual cost-modeled execution
// engine; it does NOT reimplement strategy or fill logic.
package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"antigravity-engine/internal/backtest"
	btExecution "antigravity-engine/internal/backtest/execution"
	v3 "antigravity-engine/internal/backtest/v3"
	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

const (
	symbol           = "BTCUSD" // confirmed working against Delta India API
	initialCapitalUSD = 100_000.0
	lookbackDays      = 14 // extra history before the analysis week, for indicator warmup
	weekDays          = 7
)

var targetStrategies = []string{
	"BB_Squeeze_EFI_ADX_Short",
	"WMA_Bear_Cross_Short",
}

func main() {
	now := time.Now().UTC().Truncate(time.Hour)
	weekEnd := now
	weekStart := weekEnd.Add(-time.Duration(weekDays) * 24 * time.Hour)
	fetchStart := weekStart.Add(-time.Duration(lookbackDays) * 24 * time.Hour)

	fmt.Printf("=== Delta Exchange 1-Week Backtest: %s ===\n", symbol)
	fmt.Printf("Analysis window (UTC): %s -> %s\n", weekStart.Format(time.RFC3339), weekEnd.Format(time.RFC3339))
	fmt.Printf("Fetching from %s for indicator warmup (%d extra days)\n\n", fetchStart.Format(time.RFC3339), lookbackDays)

	resolutions := map[string]string{
		"1m":  "1m",
		"5m":  "5m",
		"15m": "15m",
		"1h":  "1h",
		"4h":  "4h",
	}

	ds := marketdata.MTFDataset{
		Symbol: symbol,
		From:   weekStart,
		To:     weekEnd,
	}

	fetch := func(res string) []marketdata.HistoricalCandle {
		candles, err := marketdata.FetchDeltaHistoricalCandles(symbol, res, fetchStart, weekEnd)
		if err != nil {
			log.Fatalf("fetch %s: %v", res, err)
		}
		fmt.Printf("Fetched %-4s: %d candles (%s -> %s)\n", res, len(candles),
			firstTime(candles), lastTime(candles))
		return candles
	}

	ds.Candles1m = fetch(resolutions["1m"])
	ds.Candles5m = fetch(resolutions["5m"])
	ds.Candles15m = fetch(resolutions["15m"])
	ds.Candles1h = fetch(resolutions["1h"])
	ds.Candles4h = fetch(resolutions["4h"])

	if len(ds.Candles15m) == 0 {
		log.Fatalf("no 15m candles fetched — cannot build ticks")
	}

	fmt.Println("\nSample fetched candles (15m, first 5 of the raw fetch):")
	for i, c := range ds.Candles15m {
		if i >= 5 {
			break
		}
		fmt.Printf("  %s  O=%.1f H=%.1f L=%.1f C=%.1f V=%.1f\n",
			c.OpenTime.Format(time.RFC3339), c.Open, c.High, c.Low, c.Close, c.Volume)
	}
	fmt.Println()

	// Look up the two real strategy structs from the actual registry — no
	// reimplementation of signal logic.
	all := append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...)
	byName := map[string]scalers.RegistryEntry{}
	for _, e := range all {
		byName[e.Name] = e
	}

	var entries []scalers.RegistryEntry
	for _, name := range targetStrategies {
		e, ok := byName[name]
		if !ok {
			log.Fatalf("strategy %q not found in BuildAllScalpers()", name)
		}
		entries = append(entries, e)
	}

	// Real sizing/cost config, matching the honest backtest pipeline
	// (internal/backtest/runner.go DefaultRunConfig) except InitialCapitalUSD,
	// which we override to the requested $100,000 paper capital.
	// SizeBTC=2.5 matches the live FIXED_TRADE_SIZE_BTC value noted in project
	// memory (catalog.go default is 0.1, but the live engine was tuned to 2.5
	// BTC per trade — see sizing_risk_budget_underuse memory note).
	cfg := backtest.DefaultRunConfig(symbol)
	cfg.SizeBTC = 2.5
	cfg.V3Config.InitialCapitalUSD = initialCapitalUSD

	runner := backtest.NewBatchRunner(ds, cfg, entries)
	results := runner.RunAll(func(done, total int, name string) {
		if name != "" {
			fmt.Printf("[%d/%d] ran %s\n", done, total, name)
		}
	})

	fmt.Println()
	for _, r := range results {
		if r.DemoteReason != "" && r.TotalTrades == 0 {
			fmt.Printf("=== %s: run note: %s ===\n", r.StrategyName, r.DemoteReason)
		}
		printDailyTable(r, weekStart, weekEnd)
	}

	printCombinedTotals(results, weekStart, weekEnd)
}

type dayBucket struct {
	date          string
	capitalUsed   float64
	pnl           float64
	tradesOpened  int
}

func printDailyTable(r backtest.StrategyResult, weekStart, weekEnd time.Time) {
	fmt.Printf("\n--- %s ---\n", r.StrategyName)
	fmt.Printf("Total trades in window (all time from fetch): %d\n", r.TotalTrades)
	for _, t := range r.V3Result.Trades {
		fmt.Printf("  [trade] opened=%s closed=%s entry=%.1f exit=%.1f size=%.2f netPnL=%.2f exitReason=%s\n",
			t.OpenedAt.Format(time.RFC3339), t.ClosedAt.Format(time.RFC3339), t.EntryPrice, t.ExitPrice, t.Size, t.NetPnL, t.ExitReason)
	}

	buckets := buildDailyBuckets(r.V3Result.Trades, weekStart, weekEnd)

	fmt.Printf("%-12s %14s %12s %10s %6s\n", "Date (UTC)", "CapitalUsed$", "PnL$", "ROI%", "Trades")
	fmt.Println("--------------------------------------------------------------")
	var totalPnL, totalCapUsed float64
	for _, b := range buckets {
		roiStr := "N/A"
		if b.capitalUsed > 0 {
			roi := b.pnl / b.capitalUsed * 100
			roiStr = fmt.Sprintf("%.3f%%", roi)
		}
		fmt.Printf("%-12s %14.2f %12.2f %10s %6d\n", b.date, b.capitalUsed, b.pnl, roiStr, b.tradesOpened)
		totalPnL += b.pnl
		totalCapUsed += b.capitalUsed
	}
	fmt.Println("--------------------------------------------------------------")
	weekRoiStr := "N/A (no trades opened in window)"
	if totalCapUsed > 0 {
		weekRoiStr = fmt.Sprintf("%.3f%%", totalPnL/totalCapUsed*100)
	}
	fmt.Printf("7-day total: capitalUsed=$%.2f pnl=$%.2f ROI=%s\n", totalCapUsed, totalPnL, weekRoiStr)
}

func printCombinedTotals(results []backtest.StrategyResult, weekStart, weekEnd time.Time) {
	fmt.Println("\n=== COMBINED (both strategies) ===")
	var all []v3.V3Trade
	for _, r := range results {
		all = append(all, r.V3Result.Trades...)
	}
	buckets := buildDailyBuckets(all, weekStart, weekEnd)
	fmt.Printf("%-12s %14s %12s %10s %6s\n", "Date (UTC)", "CapitalUsed$", "PnL$", "ROI%", "Trades")
	fmt.Println("--------------------------------------------------------------")
	var totalPnL, totalCapUsed float64
	for _, b := range buckets {
		roiStr := "N/A"
		if b.capitalUsed > 0 {
			roi := b.pnl / b.capitalUsed * 100
			roiStr = fmt.Sprintf("%.3f%%", roi)
		}
		fmt.Printf("%-12s %14.2f %12.2f %10s %6d\n", b.date, b.capitalUsed, b.pnl, roiStr, b.tradesOpened)
		totalPnL += b.pnl
		totalCapUsed += b.capitalUsed
	}
	fmt.Println("--------------------------------------------------------------")
	weekRoiStr := "N/A (no trades opened in window)"
	if totalCapUsed > 0 {
		weekRoiStr = fmt.Sprintf("%.3f%%", totalPnL/totalCapUsed*100)
	}
	fmt.Printf("7-day combined total: capitalUsed=$%.2f pnl=$%.2f ROI=%s\n", totalCapUsed, totalPnL, weekRoiStr)
}

// buildDailyBuckets buckets trades by the UTC calendar day they were OPENED.
// capitalUsed(day) = sum of notional (EntryPrice * Size) for trades opened
// that day. pnl(day) = sum of NetPnL for trades opened that day (attributed
// to the open day, not the close day, so the capital-used denominator lines
// up with the PnL numerator it produced).
func buildDailyBuckets(trades []v3.V3Trade, weekStart, weekEnd time.Time) []dayBucket {
	byDay := map[string]*dayBucket{}
	var order []string
	for d := weekStart; d.Before(weekEnd); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		byDay[key] = &dayBucket{date: key}
		order = append(order, key)
	}
	sort.Strings(order)

	for _, t := range trades {
		if t.OpenedAt.Before(weekStart) || !t.OpenedAt.Before(weekEnd) {
			continue // outside the 7-day analysis window (e.g. opened during warmup lookback)
		}
		key := t.OpenedAt.Format("2006-01-02")
		b, ok := byDay[key]
		if !ok {
			b = &dayBucket{date: key}
			byDay[key] = b
			order = append(order, key)
			sort.Strings(order)
		}
		b.capitalUsed += t.EntryPrice * t.Size
		b.pnl += t.NetPnL
		b.tradesOpened++
	}

	out := make([]dayBucket, 0, len(order))
	for _, k := range order {
		out = append(out, *byDay[k])
	}
	return out
}

func firstTime(c []marketdata.HistoricalCandle) string {
	if len(c) == 0 {
		return "n/a"
	}
	return c[0].OpenTime.Format(time.RFC3339)
}

func lastTime(c []marketdata.HistoricalCandle) string {
	if len(c) == 0 {
		return "n/a"
	}
	return c[len(c)-1].OpenTime.Format(time.RFC3339)
}

var _ = btExecution.FundingRate{}
var _ = os.Stdout
