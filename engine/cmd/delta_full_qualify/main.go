// Command delta_full_qualify fetches the REAL, FULL available BTCUSD history
// from Delta Exchange's public REST API, splits it into a 70% train / 30%
// out-of-sample validation window (same discipline as BACKTEST.md §4-5b),
// runs every OHLCV-compatible strategy in the scalpers registry (all
// BuildAllScalpers()+BuildPortedStrategies() candidates, not just the 2
// live-whitelisted ones) through the real V3 backtest engine on TRAIN, then
// re-validates train-promising strategies on VALIDATE, and applies the
// strict bar: win rate>50%, profit factor>=1.2, Sharpe>0.5, trades>=30 in
// the OOS validation window.
//
// This is a one-off analysis script. It reuses the real strategy Evaluate()
// logic (scalpers registry) and the real cost-modeled V3 execution engine —
// it does not reimplement signal or fill logic.
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
	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

const symbol = "BTCUSD"

type Metrics struct {
	Trades       int     `json:"trades"`
	WinRatePct   float64 `json:"win_rate_pct"`
	Sharpe       float64 `json:"sharpe"`
	ProfitFactor float64 `json:"profit_factor"`
	ReturnPct    float64 `json:"return_pct"`
	MaxDDPct     float64 `json:"max_dd_pct"`
}

type Row struct {
	Strategy      string  `json:"strategy"`
	Train         Metrics `json:"train"`
	Validate      Metrics `json:"validate"`
	TrainPromo    bool    `json:"train_promising"`
	StrictPass    bool    `json:"strict_oos_pass"`
	NearMissNotes string  `json:"near_miss,omitempty"`
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

// strictOOSPass is the user-specified strict qualification bar applied to
// the VALIDATE (out-of-sample) window only.
func strictOOSPass(m Metrics) bool {
	return m.WinRatePct > 50 && m.ProfitFactor >= 1.2 && m.Sharpe > 0.5 && m.Trades >= 30
}

func main() {
	fetchFrom := flag.String("fetch-from", "2019-01-01", "earliest date to probe Delta for (real availability found automatically)")
	splitStr := flag.Float64("train-frac", 0.70, "fraction of the real available history used for TRAIN")
	outFile := flag.String("out", "data/delta_full_qualify_results.json", "output JSON path")
	only := flag.String("only", "", "comma-separated strategy names to restrict the candidate pool to (empty = all OHLCV-compatible)")
	flag.Parse()

	var onlySet map[string]bool
	if *only != "" {
		onlySet = map[string]bool{}
		for _, n := range splitComma(*only) {
			onlySet[n] = true
		}
	}

	probeFrom, err := time.Parse("2006-01-02", *fetchFrom)
	if err != nil {
		log.Fatalf("invalid --fetch-from: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Hour)

	fmt.Printf("=== Delta Exchange FULL-HISTORY Strategy Qualification: %s ===\n", symbol)
	fmt.Printf("Probing real data availability from %s to %s ...\n", probeFrom.Format("2006-01-02"), now.Format("2006-01-02"))

	// Probe true earliest available data using 1d resolution (cheap) first.
	probe1d, err := marketdata.FetchDeltaHistoricalCandles(symbol, "1d", probeFrom, now)
	if err != nil {
		log.Fatalf("probe fetch failed: %v", err)
	}
	if len(probe1d) == 0 {
		log.Fatalf("no data returned at all from Delta for %s", symbol)
	}
	realStart := probe1d[0].OpenTime
	realEnd := probe1d[len(probe1d)-1].OpenTime
	fmt.Printf("REAL Delta data range confirmed: %s -> %s (%d daily candles)\n\n",
		realStart.Format("2006-01-02"), realEnd.Format("2006-01-02"), len(probe1d))

	totalDur := realEnd.Sub(realStart)
	trainEnd := realStart.Add(time.Duration(float64(totalDur) * (*splitStr)))
	validateStart := trainEnd
	validateEnd := realEnd.Add(24 * time.Hour) // inclusive of last day

	fmt.Printf("TRAIN window   : %s -> %s\n", realStart.Format("2006-01-02"), trainEnd.Format("2006-01-02"))
	fmt.Printf("VALIDATE window: %s -> %s (out-of-sample)\n\n", validateStart.Format("2006-01-02"), validateEnd.Format("2006-01-02"))

	// Fetch full-range candles for every timeframe the ContextBuilder reads
	// (1m/5m/15m/1h/4h) — many strategies gate signals on 1m/5m indicators,
	// so skipping them silently starves almost every strategy of trades
	// (confirmed empirically: 0/303 fired >1 trade over 630 days with 1m/5m
	// omitted). This is the slow part of the run.
	fmt.Println("Fetching full-range candles (this takes a while for multi-year 1m/5m/15m/1h/4h data)...")
	c1m := fetchRes("1m", realStart, realEnd.Add(24*time.Hour))
	c5m := fetchRes("5m", realStart, realEnd.Add(24*time.Hour))
	c15 := fetchRes("15m", realStart, realEnd.Add(24*time.Hour))
	c1h := fetchRes("1h", realStart, realEnd.Add(24*time.Hour))
	c4h := fetchRes("4h", realStart, realEnd.Add(24*time.Hour))
	fmt.Println()

	trainDS := sliceDataset(c1m, c5m, c15, c1h, c4h, realStart, trainEnd)
	validDS := sliceDataset(c1m, c5m, c15, c1h, c4h, validateStart, validateEnd)
	fmt.Printf("TRAIN candles   — 1m:%d 5m:%d 15m:%d 1h:%d 4h:%d\n", len(trainDS.Candles1m), len(trainDS.Candles5m), len(trainDS.Candles15m), len(trainDS.Candles1h), len(trainDS.Candles4h))
	fmt.Printf("VALIDATE candles— 1m:%d 5m:%d 15m:%d 1h:%d 4h:%d\n\n", len(validDS.Candles1m), len(validDS.Candles5m), len(validDS.Candles15m), len(validDS.Candles1h), len(validDS.Candles4h))

	// Full candidate pool: every OHLCV-compatible strategy registered, not
	// just the 2 live-whitelisted names.
	raw := append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...)
	var entries []scalers.RegistryEntry
	for _, e := range raw {
		if !e.OHLCVCompatible {
			continue
		}
		if onlySet != nil && !onlySet[e.Name] {
			continue
		}
		entries = append(entries, e)
	}
	fmt.Printf("Candidate strategies (OHLCV-compatible): %d\n\n", len(entries))

	// Use backtest.DefaultRunConfig UNCHANGED (SizeBTC=0.1, matching the
	// catalog default and the actual honest pipeline that qualified the
	// current 2 live strategies in run_backtest/main.go). An earlier version
	// of this harness overrode SizeBTC to 2.5 (mirroring live FIXED_TRADE_SIZE_BTC)
	// against $100k paper capital — that made one trade's notional (~$237k)
	// breach the 50% CapitalFloorPct halt and froze every strategy after its
	// first trade (every candidate showed exactly 1 trade). Reverted.
	cfg := backtest.DefaultRunConfig(symbol)
	cfg.V3Config.InitialCapitalUSD = 100_000.0

	fmt.Println("=== TRAIN pass ===")
	trainRunner := backtest.NewBatchRunner(trainDS, cfg, entries)
	trainResults := trainRunner.RunAll(func(done, total int, name string) {
		if name != "" && done%10 == 0 {
			fmt.Printf("[train %d/%d] %s\n", done, total, name)
		}
	})
	for i := range trainResults {
		backtest.EvaluatePromotion(&trainResults[i], nil)
	}

	// Train-promising: basic relaxed screen so we don't waste the validate
	// pass on strategies with zero trades or clearly broken signals. This is
	// NOT the qualification bar — it's just a compute filter.
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
	fmt.Printf("\nTrain-promising (basic screen, advances to OOS validate): %d / %d\n\n", len(promising), len(entries))

	fmt.Println("=== VALIDATE (out-of-sample) pass ===")
	validRunner := backtest.NewBatchRunner(validDS, cfg, promising)
	validResults := validRunner.RunAll(func(done, total int, name string) {
		if name != "" && done%10 == 0 {
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
		tr := trainByName[e.Name]
		row := Row{Strategy: e.Name, Train: toMetrics(tr), TrainPromo: false}
		for _, p := range promising {
			if p.Name == e.Name {
				row.TrainPromo = true
			}
		}
		if vr, ok := validByName[e.Name]; ok {
			row.Validate = toMetrics(vr)
			row.StrictPass = strictOOSPass(row.Validate)
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StrictPass != rows[j].StrictPass {
			return rows[i].StrictPass
		}
		return rows[i].Validate.Sharpe > rows[j].Validate.Sharpe
	})

	passCount := 0
	fmt.Println("\n=== STRICT OOS QUALIFICATION RESULTS ===")
	fmt.Printf("%-45s %6s %7s %7s %7s %6s | %6s %7s %7s %7s %6s %5s\n",
		"Strategy", "TrnN", "TrnWR%", "TrnSh", "TrnPF", "TrnN",
		"ValN", "ValWR%", "ValSh", "ValPF", "ValN", "PASS")
	for _, r := range rows {
		if r.StrictPass {
			passCount++
		}
		mark := " "
		if r.StrictPass {
			mark = "***"
		}
		fmt.Printf("%-45s %6d %6.1f%% %7.2f %7.2f | %6d %6.1f%% %7.2f %7.2f %5s\n",
			truncate(r.Strategy, 45), r.Train.Trades, r.Train.WinRatePct, r.Train.Sharpe, r.Train.ProfitFactor,
			r.Validate.Trades, r.Validate.WinRatePct, r.Validate.Sharpe, r.Validate.ProfitFactor, mark)
	}
	fmt.Printf("\n%d / %d candidates PASS the strict OOS bar (WinRate>50%%, PF>=1.2, Sharpe>0.5, Trades>=30 in VALIDATE window)\n", passCount, len(entries))

	out := map[string]interface{}{
		"symbol":            symbol,
		"real_data_start":   realStart.Format("2006-01-02"),
		"real_data_end":     realEnd.Format("2006-01-02"),
		"train_window":      []string{realStart.Format("2006-01-02"), trainEnd.Format("2006-01-02")},
		"validate_window":   []string{validateStart.Format("2006-01-02"), validateEnd.Format("2006-01-02")},
		"candidates_tested": len(entries),
		"strict_pass_count": passCount,
		"ran_at":            time.Now().UTC().Format(time.RFC3339),
		"rows":              rows,
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(*outFile, data, 0o644); err != nil {
		log.Printf("warning: could not write %s: %v", *outFile, err)
	} else {
		fmt.Printf("\nFull results saved to: %s\n", *outFile)
	}
}

func fetchRes(res string, from, to time.Time) []marketdata.HistoricalCandle {
	c, err := marketdata.FetchDeltaHistoricalCandles(symbol, res, from, to)
	if err != nil {
		log.Fatalf("fetch %s: %v", res, err)
	}
	fmt.Printf("  fetched %-4s: %d candles\n", res, len(c))
	return c
}

func sliceDataset(c1m, c5m, c15, c1h, c4h []marketdata.HistoricalCandle, from, to time.Time) marketdata.MTFDataset {
	return marketdata.MTFDataset{
		Symbol:     symbol,
		From:       from,
		To:         to,
		Candles1m:  sliceRange(c1m, from, to),
		Candles5m:  sliceRange(c5m, from, to),
		Candles15m: sliceRange(c15, from, to),
		Candles1h:  sliceRange(c1h, from, to),
		Candles4h:  sliceRange(c4h, from, to),
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
	if f != f { // NaN
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
