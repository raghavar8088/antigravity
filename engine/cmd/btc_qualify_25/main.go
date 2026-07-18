// Command btc_qualify_25 qualifies the BTC Pre-Live Engine's strategy whitelist
// (Phase 2 of the BTC pre-live plan).
//
// It reads the LAST YEAR of real Delta Exchange BTCUSD candles from the Phase 1
// disk cache (cmd/delta_fetch_history -> data/historical/delta), splits it
// 70% TRAIN / 30% out-of-sample VALIDATE, runs every OHLCV-compatible strategy
// in the scalpers registry through the real cost-modeled V3 backtest engine
// (identical discipline and machinery to cmd/delta_full_qualify — no
// reimplemented signal or fill logic), and applies the same strict OOS bar:
//
//	win rate > 50%, profit factor >= 1.2, Sharpe > 0.5, trades >= 30 (VALIDATE)
//
// Ranking ("highest accuracy" per the module spec): Tier A = strict passers,
// ranked by OOS win rate desc (PF, then Sharpe as tie-breaks — the strict bar
// already floors robustness). If fewer than 25 pass, Tier B near-misses fill
// the remainder: 3-of-4 criteria met with every metric above relaxed floors
// (WR>45%, PF>=1.0, Sharpe>0.25, trades>=20), ranked the same way, and
// honestly labeled tier "B" in the output.
//
// Output: data/btc_prelive_whitelist.json — consumed by the Phase 3 paper desk
// (its live whitelist) and copied to client/public/btc-prelive/ for the
// Qualification tab UI. Additive tooling: nothing existing reads or runs this.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"antigravity-engine/internal/backtest"
	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// symbol is set from -symbol at startup (BTCUSD for the Delta cache,
// BTCUSDT for the Binance cache — both are real BTC perp/spot history).
var symbol = "BTCUSD"

type Metrics struct {
	Trades       int     `json:"trades"`
	WinRatePct   float64 `json:"win_rate_pct"`
	Sharpe       float64 `json:"sharpe"`
	ProfitFactor float64 `json:"profit_factor"`
	ReturnPct    float64 `json:"return_pct"`
	MaxDDPct     float64 `json:"max_dd_pct"`
}

type Row struct {
	Rank       int     `json:"rank,omitempty"`
	Strategy   string  `json:"strategy"`
	Tier       string  `json:"tier,omitempty"` // "A" strict pass, "B" near-miss filler, "" not selected
	Train      Metrics `json:"train"`
	Validate   Metrics `json:"validate"`
	TrainPromo bool    `json:"train_promising"`
	StrictPass bool    `json:"strict_oos_pass"`
	CriteriaOK int     `json:"criteria_met"` // 0-4 strict criteria met on VALIDATE
}

func toMetrics(r backtest.StrategyResult) Metrics {
	return Metrics{
		Trades:       r.TotalTrades,
		WinRatePct:   round2(r.WinRate * 100),
		Sharpe:       round2(r.Sharpe),
		ProfitFactor: round2(r.ProfitFactor),
		ReturnPct:    round2(r.TotalReturnPct),
		MaxDDPct:     round2(r.MaxDrawdownPct),
	}
}

func criteriaMet(m Metrics) int {
	n := 0
	if m.WinRatePct > 50 {
		n++
	}
	if m.ProfitFactor >= 1.2 {
		n++
	}
	if m.Sharpe > 0.5 {
		n++
	}
	if m.Trades >= 30 {
		n++
	}
	return n
}

func strictOOSPass(m Metrics) bool { return criteriaMet(m) == 4 }

// nearMiss: 3 of 4 strict criteria met AND every metric above a relaxed floor —
// the honest "Tier B" pool used only if fewer than 25 strategies pass strictly.
func nearMiss(m Metrics) bool {
	return criteriaMet(m) == 3 &&
		m.WinRatePct > 45 && m.ProfitFactor >= 1.0 && m.Sharpe > 0.25 && m.Trades >= 20
}

// accuracyRank orders by OOS win rate desc, then PF, then Sharpe.
func accuracyRank(a, b Row) bool {
	if a.Validate.WinRatePct != b.Validate.WinRatePct {
		return a.Validate.WinRatePct > b.Validate.WinRatePct
	}
	if a.Validate.ProfitFactor != b.Validate.ProfitFactor {
		return a.Validate.ProfitFactor > b.Validate.ProfitFactor
	}
	return a.Validate.Sharpe > b.Validate.Sharpe
}

type cachedCandle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// loadCached reads one resolution's candles from a disk cache. Two formats are
// supported, detected automatically:
//   - the Phase 1 Delta cache (cmd/delta_fetch_history): compact {time,open,...}
//     objects with unix-seconds times
//   - the Binance cache (cmd/fetch_historical / BinanceHistoricalFetcher):
//     Go-marshaled marketdata.HistoricalCandle objects with RFC3339 OpenTime
func loadCached(cacheDir, res string) []marketdata.HistoricalCandle {
	path := filepath.Join(cacheDir, fmt.Sprintf("%s_%s.json", symbol, res))
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("cache read %s: %v — run cmd/delta_fetch_history (Delta) or cmd/fetch_historical (Binance) first", path, err)
	}

	var out []marketdata.HistoricalCandle
	var compact []cachedCandle
	if err := json.Unmarshal(data, &compact); err == nil && len(compact) > 0 && compact[0].Time > 0 {
		out = make([]marketdata.HistoricalCandle, 0, len(compact))
		for _, c := range compact {
			ot := time.Unix(c.Time, 0).UTC()
			out = append(out, marketdata.HistoricalCandle{
				OpenTime: ot, Open: c.Open, High: c.High, Low: c.Low,
				Close: c.Close, Volume: c.Volume, CloseTime: ot,
			})
		}
	} else {
		if err := json.Unmarshal(data, &out); err != nil {
			log.Fatalf("cache parse %s (neither compact nor HistoricalCandle format): %v", path, err)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime.Before(out[j].OpenTime) })
	fmt.Printf("  loaded %-4s: %d candles from cache\n", res, len(out))
	return out
}

func main() {
	cacheDir := flag.String("cache-dir", "data/historical/delta", "candle cache directory")
	symbolFlag := flag.String("symbol", "BTCUSD", "cached symbol (BTCUSD=Delta cache, BTCUSDT=Binance cache)")
	windowYears := flag.Float64("window-years", 0, "if >0, restrict to the LAST N years of the cache (0 = full cache)")
	trainFrac := flag.Float64("train-frac", 0.70, "fraction of history used for TRAIN")
	topN := flag.Int("top", 25, "whitelist size")
	outFile := flag.String("out", "data/btc_prelive_whitelist.json", "output JSON path")
	flag.Parse()
	symbol = *symbolFlag

	fmt.Printf("=== BTC Pre-Live Engine — Phase 2 strategy qualification (top %d, %s) ===\n", *topN, symbol)
	fmt.Printf("Loading candle cache from %s...\n", *cacheDir)
	c1m := loadCached(*cacheDir, "1m")
	c5m := loadCached(*cacheDir, "5m")
	c15 := loadCached(*cacheDir, "15m")
	c1h := loadCached(*cacheDir, "1h")
	c4h := loadCached(*cacheDir, "4h")

	realStart := c1d0(c1h)
	realEnd := c1h[len(c1h)-1].OpenTime
	if *windowYears > 0 {
		cut := realEnd.Add(-time.Duration(*windowYears * 365 * 24 * float64(time.Hour)))
		if cut.After(realStart) {
			realStart = cut
			fmt.Printf("Window restricted to last %.1f years: from %s\n", *windowYears, realStart.Format("2006-01-02"))
		}
	}
	totalDur := realEnd.Sub(realStart)
	trainEnd := realStart.Add(time.Duration(float64(totalDur) * (*trainFrac)))
	validateEnd := realEnd.Add(time.Hour)

	source := "delta_exchange_cached_phase1"
	if symbol == "BTCUSDT" {
		source = "binance_spot_cached"
	}
	fmt.Printf("\nData window    : %s -> %s (source: %s, cache %s)\n",
		realStart.Format("2006-01-02"), realEnd.Format("2006-01-02"), source, *cacheDir)
	fmt.Printf("TRAIN window   : %s -> %s\n", realStart.Format("2006-01-02"), trainEnd.Format("2006-01-02"))
	fmt.Printf("VALIDATE window: %s -> %s (out-of-sample)\n\n", trainEnd.Format("2006-01-02"), validateEnd.Format("2006-01-02"))

	trainDS := sliceDataset(c1m, c5m, c15, c1h, c4h, realStart, trainEnd)
	validDS := sliceDataset(c1m, c5m, c15, c1h, c4h, trainEnd, validateEnd)
	fmt.Printf("TRAIN candles   — 1m:%d 5m:%d 15m:%d 1h:%d 4h:%d\n", len(trainDS.Candles1m), len(trainDS.Candles5m), len(trainDS.Candles15m), len(trainDS.Candles1h), len(trainDS.Candles4h))
	fmt.Printf("VALIDATE candles— 1m:%d 5m:%d 15m:%d 1h:%d 4h:%d\n\n", len(validDS.Candles1m), len(validDS.Candles5m), len(validDS.Candles15m), len(validDS.Candles1h), len(validDS.Candles4h))

	raw := append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...)
	raw = append(raw, scalers.BuildDelta20Pack()...)
	var entries []scalers.RegistryEntry
	for _, e := range raw {
		if e.OHLCVCompatible {
			entries = append(entries, e)
		}
	}
	fmt.Printf("Candidate strategies (OHLCV-compatible): %d\n\n", len(entries))

	// Identical config to the honest pipeline (delta_full_qualify / run_backtest).
	cfg := backtest.DefaultRunConfig(symbol)
	cfg.V3Config.InitialCapitalUSD = 100_000.0

	fmt.Println("=== TRAIN pass ===")
	trainRunner := backtest.NewBatchRunner(trainDS, cfg, entries)
	trainResults := trainRunner.RunAll(func(done, total int, name string) {
		if name != "" && done%20 == 0 {
			fmt.Printf("[train %d/%d] %s\n", done, total, name)
		}
	})
	for i := range trainResults {
		backtest.EvaluatePromotion(&trainResults[i], nil)
	}

	// Relaxed compute filter only (NOT the qualification bar) — skip strategies
	// with zero/broken signals so the validate pass isn't wasted on them.
	var promising []scalers.RegistryEntry
	trainByName := map[string]backtest.StrategyResult{}
	for _, r := range trainResults {
		trainByName[r.StrategyName] = r
		if r.TotalTrades >= 10 && r.WinRate > 0.40 && r.ProfitFactor >= 1.0 {
			for _, e := range entries {
				if e.Name == r.StrategyName {
					promising = append(promising, e)
					break
				}
			}
		}
	}
	fmt.Printf("\nTrain-promising (advances to OOS validate): %d / %d\n\n", len(promising), len(entries))

	fmt.Println("=== VALIDATE (out-of-sample) pass ===")
	validRunner := backtest.NewBatchRunner(validDS, cfg, promising)
	validResults := validRunner.RunAll(func(done, total int, name string) {
		if name != "" && done%20 == 0 {
			fmt.Printf("[validate %d/%d] %s\n", done, total, name)
		}
	})
	for i := range validResults {
		backtest.EvaluatePromotion(&validResults[i], nil)
	}
	validByName := map[string]backtest.StrategyResult{}
	for _, r := range validResults {
		validByName[r.StrategyName] = r
	}

	var rows []Row
	for _, e := range entries {
		row := Row{Strategy: e.Name, Train: toMetrics(trainByName[e.Name])}
		for _, p := range promising {
			if p.Name == e.Name {
				row.TrainPromo = true
			}
		}
		if vr, ok := validByName[e.Name]; ok {
			row.Validate = toMetrics(vr)
			row.CriteriaOK = criteriaMet(row.Validate)
			row.StrictPass = strictOOSPass(row.Validate)
		}
		rows = append(rows, row)
	}

	// Tier A: strict passers by accuracy rank. Tier B: near-misses fill to topN.
	var tierA, tierB []Row
	for _, r := range rows {
		if r.StrictPass {
			tierA = append(tierA, r)
		} else if r.TrainPromo && nearMiss(r.Validate) {
			tierB = append(tierB, r)
		}
	}
	sort.Slice(tierA, func(i, j int) bool { return accuracyRank(tierA[i], tierA[j]) })
	sort.Slice(tierB, func(i, j int) bool { return accuracyRank(tierB[i], tierB[j]) })

	var selected []Row
	for _, r := range tierA {
		if len(selected) >= *topN {
			break
		}
		r.Tier = "A"
		r.Rank = len(selected) + 1
		selected = append(selected, r)
	}
	for _, r := range tierB {
		if len(selected) >= *topN {
			break
		}
		r.Tier = "B"
		r.Rank = len(selected) + 1
		selected = append(selected, r)
	}

	fmt.Println("\n=== BTC PRE-LIVE WHITELIST (ranked by OOS accuracy) ===")
	fmt.Printf("%-4s %-2s %-45s %7s %7s %7s %6s %8s %7s\n", "Rank", "T", "Strategy", "ValWR%", "ValPF", "ValSh", "ValN", "ValRet%", "ValDD%")
	for _, r := range selected {
		fmt.Printf("%-4d %-2s %-45s %6.1f%% %7.2f %7.2f %6d %7.1f%% %6.1f%%\n",
			r.Rank, r.Tier, truncate(r.Strategy, 45),
			r.Validate.WinRatePct, r.Validate.ProfitFactor, r.Validate.Sharpe,
			r.Validate.Trades, r.Validate.ReturnPct, r.Validate.MaxDDPct)
	}
	fmt.Printf("\nStrict passers (Tier A): %d | Near-miss fillers used (Tier B): %d | Whitelist size: %d / %d target\n",
		len(tierA), len(selected)-min(len(tierA), *topN), len(selected), *topN)

	names := make([]string, 0, len(selected))
	for _, r := range selected {
		names = append(names, r.Strategy)
	}

	out := map[string]interface{}{
		"module":            "btc_pre_live_engine",
		"symbol":            symbol,
		"data_source":       source,
		"data_start":        realStart.Format("2006-01-02"),
		"data_end":          realEnd.Format("2006-01-02"),
		"train_window":      []string{realStart.Format("2006-01-02"), trainEnd.Format("2006-01-02")},
		"validate_window":   []string{trainEnd.Format("2006-01-02"), validateEnd.Format("2006-01-02")},
		"strict_bar":        "VALIDATE: win_rate>50%, profit_factor>=1.2, sharpe>0.5, trades>=30",
		"ranking_rule":      "Tier A strict passers by OOS win rate (PF, Sharpe tie-breaks); Tier B 3-of-4 near-misses fill to target only if needed",
		"candidates_tested": len(entries),
		"tier_a_count":      len(tierA),
		"tier_b_available":  len(tierB),
		"whitelist":         names,
		"selected":          selected,
		"all_rows":          rows,
		"ran_at":            time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(*outFile, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", *outFile, err)
	}
	fmt.Printf("\nWhitelist saved to: %s\n", *outFile)
}

func c1d0(c []marketdata.HistoricalCandle) time.Time {
	if len(c) == 0 {
		log.Fatal("empty candle cache")
	}
	return c[0].OpenTime
}

func sliceDataset(c1m, c5m, c15, c1h, c4h []marketdata.HistoricalCandle, from, to time.Time) marketdata.MTFDataset {
	return marketdata.MTFDataset{
		Symbol: symbol, From: from, To: to,
		Candles1m: sliceRange(c1m, from, to), Candles5m: sliceRange(c5m, from, to),
		Candles15m: sliceRange(c15, from, to), Candles1h: sliceRange(c1h, from, to),
		Candles4h: sliceRange(c4h, from, to),
	}
}

func sliceRange(c []marketdata.HistoricalCandle, from, to time.Time) []marketdata.HistoricalCandle {
	out := make([]marketdata.HistoricalCandle, 0, len(c))
	for _, x := range c {
		if !x.OpenTime.Before(from) && x.OpenTime.Before(to) {
			out = append(out, x)
		}
	}
	return out
}

func round2(f float64) float64 {
	if f != f {
		return 0
	}
	i := int(f*100 + sign(f)*0.5)
	return float64(i) / 100
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
