// Command delta_day_backtest fetches REAL BTCUSD candles from Delta
// Exchange's public REST API for a UTC calendar-day range (plus warmup
// lookback for indicators) and runs the strategies that have passed the
// strict OOS bar in delta_full_qualify (engine/data/delta_full_qualify_results.json
// + delta_batch10_qualify_results.json — 24 total as of 2026-07-11) through
// the real V3 backtest engine, using the real 0.1 BTC catalog default
// sizing. Reports, per day and per strategy, capital used (notional of
// trades opened that day) and ROI (day PnL / capital used).
//
// This is a one-off analysis script — reuses the real strategy Evaluate()
// logic and the real cost-modeled V3 execution engine, no reimplementation.
package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"antigravity-engine/internal/backtest"
	v3 "antigravity-engine/internal/backtest/v3"
	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

const symbol = "BTCUSD"

// qualified22 — passed the strict OOS bar in delta_full_qualify_results.json
// (run 2026-07-11): win rate>50%, PF>=1.2, Sharpe>0.5, trades>=30 in the OOS
// validate window.
var qualified22 = []string{
	"Lower_High_CMF_Bear_Short", "OBV_Bear_CMF_Short", "Three_Bear_Candles_Short",
	"Three_Bear_Candles_ADX_Short", "BB_Squeeze_EFI_ADX_Short", "Lower_High_Confirm_Short",
	"HMA_CMF_Bear_Short", "OBV_Slope_EFI_ADX_Short", "Fisher_Zero_CMF_ADX_Short",
	"CMF_Extreme_Bear_Short", "CMF_Cross_Bullish_Long", "Two_Bar_Bear_Reversal_Short",
	"Fisher_Deep_Bear_Short", "Close_Low_Range_ADX_Short", "Donchian_CMF_Bear_Short",
	"Keltner_CMF_Bear_Short", "BB_Width_Expand_Bear_Short", "Close_Low_Range_Bear_Short",
	"Two_Bar_Bear_Reversal_ADX_Short", "Bearish_Engulf_EFI_ADX_Short", "Bearish_Engulfing_Short",
	"ADX_Surge_Breakout_Long",
}

// qualifiedBatch10 — the 2/10 new candidates that passed the same strict bar
// in delta_batch10_qualify_results.json (run 2026-07-11 follow-up).
var qualifiedBatch10 = []string{
	"Donchian_ADX_CMF_Bear_Short", "HMA_ADX_CMF_Bear_Short",
}

// qualifiedBatch18Research — the 2/18 research-driven round-2 candidates that
// passed the same strict bar in delta_batch18_research_qualify_results.json
// (run 2026-07-11 follow-up).
var qualifiedBatch18Research = []string{
	"HMA_Slope_EMA_MACD_Bear_Short", "EMA_MACD_CMF_Bull_Long",
}

func main() {
	dayStr := flag.String("day", "2026-07-10", "single UTC calendar day to analyze (YYYY-MM-DD); ignored if --from/--to given")
	fromStr := flag.String("from", "", "UTC range start (YYYY-MM-DD), inclusive — use with --to for a multi-day run")
	toStr := flag.String("to", "", "UTC range end (YYYY-MM-DD), inclusive")
	lookbackDays := flag.Int("lookback-days", 21, "extra days fetched before the range for indicator warmup")
	strategiesFlag := flag.String("strategies", "", "comma-separated strategy names to run (empty = qualified22 + qualifiedBatch10 + qualifiedBatch18Research, 26 total)")
	allOHLCV := flag.Bool("all-ohlcv", false, "run the FULL OHLCV-compatible backtestable pool (excludes the *_CURVEFIT_DEMO strategies)")
	flag.Parse()

	targetNames := append(append(append([]string{}, qualified22...), qualifiedBatch10...), qualifiedBatch18Research...)
	if *strategiesFlag != "" {
		targetNames = splitComma(*strategiesFlag)
	}

	var rangeStart, rangeEnd time.Time
	var err error
	if *fromStr != "" && *toStr != "" {
		rangeStart, err = time.Parse("2006-01-02", *fromStr)
		if err != nil {
			log.Fatalf("invalid --from: %v", err)
		}
		toDay, err2 := time.Parse("2006-01-02", *toStr)
		if err2 != nil {
			log.Fatalf("invalid --to: %v", err2)
		}
		rangeEnd = toDay.Add(24 * time.Hour) // inclusive end-of-day
	} else {
		d, err2 := time.Parse("2006-01-02", *dayStr)
		if err2 != nil {
			log.Fatalf("invalid --day: %v", err2)
		}
		rangeStart = d
		rangeEnd = d.Add(24 * time.Hour)
	}
	rangeStart = rangeStart.UTC()
	rangeEnd = rangeEnd.UTC()
	now := time.Now().UTC()
	if rangeEnd.After(now) {
		rangeEnd = now // don't pretend future data exists; today may be partial
	}
	fetchStart := rangeStart.Add(-time.Duration(*lookbackDays) * 24 * time.Hour)

	fmt.Printf("=== Delta Exchange Multi-Day Backtest: %s ===\n", symbol)
	fmt.Printf("Analysis range (UTC): %s -> %s\n", rangeStart.Format(time.RFC3339), rangeEnd.Format(time.RFC3339))
	fmt.Printf("Fetching from %s for indicator warmup (%d extra days)\n\n", fetchStart.Format(time.RFC3339), *lookbackDays)

	fetch := func(res string) []marketdata.HistoricalCandle {
		c, err := marketdata.FetchDeltaHistoricalCandles(symbol, res, fetchStart, rangeEnd)
		if err != nil {
			log.Fatalf("fetch %s: %v", res, err)
		}
		fmt.Printf("Fetched %-4s: %d candles (%s -> %s)\n", res, len(c), firstTime(c), lastTime(c))
		return c
	}

	ds := marketdata.MTFDataset{Symbol: symbol, From: rangeStart, To: rangeEnd}
	ds.Candles1m = fetch("1m")
	ds.Candles5m = fetch("5m")
	ds.Candles15m = fetch("15m")
	ds.Candles1h = fetch("1h")
	ds.Candles4h = fetch("4h")
	fmt.Println()

	if len(ds.Candles15m) == 0 {
		log.Fatalf("no 15m candles fetched — cannot build ticks")
	}
	coversRange := false
	for _, c := range ds.Candles15m {
		if !c.OpenTime.Before(rangeStart) && c.OpenTime.Before(rangeEnd) {
			coversRange = true
			break
		}
	}
	if !coversRange {
		log.Fatalf("fetched real data does not cover target range %s->%s — refusing to fabricate results", rangeStart, rangeEnd)
	}
	fmt.Printf("Confirmed real Delta candles exist inside target range (source: api.india.delta.exchange).\n\n")

	all := append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...)
	byName := map[string]scalers.RegistryEntry{}
	for _, e := range all {
		byName[e.Name] = e
	}
	var entries []scalers.RegistryEntry
	if *allOHLCV {
		// Full backtestable pool: every OHLCV-compatible registered strategy,
		// EXCLUDING the deliberately curve-fit demo strategies.
		seen := map[string]bool{}
		for _, e := range all {
			if !e.OHLCVCompatible || seen[e.Name] {
				continue
			}
			if strings.HasSuffix(e.Name, "_CURVEFIT_DEMO") {
				continue
			}
			seen[e.Name] = true
			entries = append(entries, e)
		}
		fmt.Printf("Running FULL OHLCV pool: %d strategies\n\n", len(entries))
	} else {
		for _, name := range targetNames {
			e, ok := byName[name]
			if !ok {
				log.Fatalf("strategy %q not found in registry", name)
			}
			entries = append(entries, e)
		}
	}

	// Real sizing: catalog default FIXED_TRADE_SIZE_BTC = 0.1 BTC.
	cfg := backtest.DefaultRunConfig(symbol) // SizeBTC=0.1 by default
	cfg.V3Config.InitialCapitalUSD = 100_000.0

	runner := backtest.NewBatchRunner(ds, cfg, entries)
	results := runner.RunAll(func(done, total int, name string) {
		if name != "" {
			fmt.Printf("[%d/%d] ran %s\n", done, total, name)
		}
	})
	fmt.Println()

	// Bucket all trades opened within the range by UTC calendar day.
	type dayBucket struct {
		date        string
		capitalUsed float64
		pnl         float64
		trades      int
	}
	byDay := map[string]*dayBucket{}
	var dayOrder []string
	for d := time.Date(rangeStart.Year(), rangeStart.Month(), rangeStart.Day(), 0, 0, 0, 0, time.UTC); d.Before(rangeEnd); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		byDay[key] = &dayBucket{date: key}
		dayOrder = append(dayOrder, key)
	}

	type stratFire struct {
		strategy string
		trades   []v3.V3Trade
	}
	var fires []stratFire
	for _, r := range results {
		var sf stratFire
		sf.strategy = r.StrategyName
		for _, t := range r.V3Result.Trades {
			if t.OpenedAt.Before(rangeStart) || !t.OpenedAt.Before(rangeEnd) {
				continue
			}
			key := t.OpenedAt.Format("2006-01-02")
			b, ok := byDay[key]
			if !ok {
				b = &dayBucket{date: key}
				byDay[key] = b
				dayOrder = append(dayOrder, key)
				sort.Strings(dayOrder)
			}
			b.capitalUsed += t.EntryPrice * t.Size
			b.pnl += t.NetPnL
			b.trades++
			sf.trades = append(sf.trades, t)
		}
		if len(sf.trades) > 0 {
			fires = append(fires, sf)
		}
	}

	fmt.Printf("=== Aggregate across %d strategies, by UTC day (SizeBTC=0.1) ===\n", len(entries))
	fmt.Printf("%-12s %8s %14s %12s %10s\n", "Date", "Trades", "CapitalUsed$", "PnL$", "ROI%")
	fmt.Println(repeat("-", 62))
	var totalCap, totalPnL float64
	var totalTrades int
	for _, k := range dayOrder {
		b := byDay[k]
		roiStr := "N/A"
		if b.capitalUsed > 0 {
			roiStr = fmt.Sprintf("%.3f%%", b.pnl/b.capitalUsed*100)
		}
		partial := ""
		if k == now.Format("2006-01-02") && rangeEnd.Equal(now) {
			partial = " (partial day)"
		}
		fmt.Printf("%-12s %8d %14.2f %12.2f %10s%s\n", b.date, b.trades, b.capitalUsed, b.pnl, roiStr, partial)
		totalCap += b.capitalUsed
		totalPnL += b.pnl
		totalTrades += b.trades
	}
	fmt.Println(repeat("-", 62))
	weekRoi := "N/A (no trades opened in range)"
	if totalCap > 0 {
		weekRoi = fmt.Sprintf("%.3f%%", totalPnL/totalCap*100)
	}
	fmt.Printf("%-12s %8d %14.2f %12.2f %10s\n", "TOTAL", totalTrades, totalCap, totalPnL, weekRoi)

	fmt.Println("\nStrategies that fired at least one trade in the range:")
	if len(fires) == 0 {
		fmt.Println("  (none)")
	}
	for _, sf := range fires {
		fmt.Printf("  %s: %d trades\n", sf.strategy, len(sf.trades))
		for _, t := range sf.trades {
			fmt.Printf("    opened=%s entry=%.1f exit=%.1f size=%.3f netPnL=%.2f exitReason=%s\n",
				t.OpenedAt.Format(time.RFC3339), t.EntryPrice, t.ExitPrice, t.Size, t.NetPnL, t.ExitReason)
		}
	}
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
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

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
