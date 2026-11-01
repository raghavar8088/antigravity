// Command deep_backtest replays every OHLCV-compatible strategy in the scalpers
// registry over the deep BTCUSDT history (Binance spot 1m, Aug 2017 - Oct 2026,
// built by research/btc_deep_2026_10/deep_fetch.py) and writes one row per
// trade, so yearly and monthly performance can be computed outside the engine.
//
//	-mode fixed   corrected clock (see sim.go): closed-candle contexts at each
//	              15m close, entry at the next 1m open, exits on the 1m path.
//	-mode engine  the existing harness unchanged (ContextBuilder + ScalerV3Adapter
//	              + v3 engine, DefaultRunConfig), for comparison with past runs.
//	-mode probe   prints what a strategy can see at the moment it trades, under
//	              both clocks.
//
// Research tooling only: nothing in the live or pre-live paths reads its output.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

func main() {
	mode := flag.String("mode", "fixed", "fixed | engine | probe")
	dataDir := flag.String("data", "data/historical/deep", "directory with BTCUSDT_1m.csv and BTCUSDT_funding.csv")
	fromStr := flag.String("from", "2018-01-01", "first decision (UTC date); earlier bars are indicator warm-up")
	toStr := flag.String("to", "2026-10-31", "last decision before this UTC date")
	outDir := flag.String("out", "data/historical/deep/runs", "output directory")
	workers := flag.Int("workers", 12, "parallel strategies")
	only := flag.String("only", "", "comma-separated strategy names (default: all OHLCV-compatible)")
	flag.Parse()

	from, err := time.Parse("2006-01-02", *fromStr)
	if err != nil {
		log.Fatal(err)
	}
	to, err := time.Parse("2006-01-02", *toStr)
	if err != nil {
		log.Fatal(err)
	}
	t0 := time.Now()
	ds, err := loadDeep(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d 1m bars (%s .. %s), %d funding prints in %.0fs",
		len(ds.m1.t), ds.m1.c[0].OpenTime.Format("2006-01-02"), ds.m1.c[len(ds.m1.c)-1].OpenTime.Format("2006-01-02"),
		len(ds.funding), time.Since(t0).Seconds())
	decs, err := buildDecisions(ds, from.Unix(), to.Unix()-1)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d decision points %s .. %s (self-check passed: every context is exactly the bars closed by T)",
		len(decs), time.Unix(decs[0].T, 0).UTC().Format("2006-01-02 15:04"), time.Unix(decs[len(decs)-1].T, 0).UTC().Format("2006-01-02 15:04"))

	entries := registry(*only)
	log.Printf("%d OHLCV-compatible strategies", len(entries))

	var sh *engineShared
	if *mode == "engine" || *mode == "probe" {
		// The engine's own warm-up: its 60-bar ContextBuilder floor plus the v3
		// WarmupBars, so the first engine decision lands near the fixed clock's.
		sh = newEngineShared(ds.toMTF(from.Add(-48*time.Hour), to.Add(-time.Second)))
		if *mode == "engine" { // the engine keeps its own copies; free ours
			ds.m1, ds.m5, ds.cvdCum = nil, nil, nil
			runtime.GC()
			debug.FreeOSMemory()
		}
	}
	if *mode == "probe" {
		runProbe(sh, ds, decs)
		return
	}
	if *mode != "fixed" && *mode != "engine" {
		log.Fatalf("unknown -mode %q", *mode)
	}

	var before [5]float64
	if *mode == "fixed" {
		before = fingerprint(ds)
	}
	cfg := simCfg{maxHold: 7 * 24 * 3600, maxPerDay: 5, defSL: 0.35, defTP: 0.70}
	type result struct {
		trades []tradeRec
		st     runStats
	}
	results := make([]result, len(entries))
	jobs := make(chan int, len(entries))
	for i := range entries {
		jobs <- i
	}
	close(jobs)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				e := entries[i]
				start := time.Now()
				var r result
				func() {
					defer func() {
						if p := recover(); p != nil {
							r = result{st: runStats{Name: e.Name, Panic: fmt.Sprint(p)}}
						}
					}()
					if *mode == "fixed" {
						r.trades, r.st = runFixed(e, ds, decs, cfg)
					} else {
						var err error
						r.trades, r.st, err = runEngine(e, sh)
						if err != nil {
							r.st.Panic = err.Error()
						}
					}
				}()
				r.st.Seconds = math.Round(time.Since(start).Seconds()*10) / 10
				r.st.Description = e.Description
				r.st.Timeframes = e.Timeframes
				r.st.Regime = string(scalers.RegimeTrending)
				if len(e.Regimes) > 0 {
					r.st.Regime = string(e.Regimes[0])
				}
				results[i] = r
				mu.Lock()
				done++
				if done%20 == 0 || done == len(entries) {
					log.Printf("  %d/%d done (%.0fs)", done, len(entries), time.Since(t0).Seconds())
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if *mode != "fixed" {
		// the engine's ContextBuilder arrays are internal to the backtest package
	} else if after := fingerprint(ds); after != before {
		log.Fatalf("a strategy modified the shared candle arrays (fingerprint %v -> %v): results are invalid", before, after)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	tf, err := os.Create(filepath.Join(*outDir, "trades_"+*mode+".csv"))
	if err != nil {
		log.Fatal(err)
	}
	w := csv.NewWriter(tf)
	_ = w.Write([]string{"strategy", "side", "decision", "entry", "exit", "entry_px", "exit_px", "reason",
		"sl_pct", "tp_pct", "gross", "funding", "synth_funding", "confidence", "defaulted", "flipped", "net_usd", "size"})
	stats := make([]runStats, 0, len(results))
	nTrades := 0
	for _, r := range results {
		stats = append(stats, r.st)
		for _, t := range r.trades {
			side := "SHORT"
			if t.Long {
				side = "LONG"
			}
			_ = w.Write([]string{t.Strategy, side, strconv.FormatInt(t.Decision, 10), strconv.FormatInt(t.Entry, 10),
				strconv.FormatInt(t.Exit, 10), g(t.EntryPx), g(t.ExitPx), t.Reason, g(t.SLPct), g(t.TPPct),
				g(t.Gross), g(t.Funding), strconv.Itoa(t.SynthFunding), g(t.Confidence),
				strconv.FormatBool(t.Defaulted), strconv.FormatBool(t.Flipped), g(t.NetUSD), g(t.Size)})
			nTrades++
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Fatal(err)
	}
	tf.Close()
	sort.Slice(stats, func(a, b int) bool { return stats[a].Name < stats[b].Name })
	meta := map[string]any{
		"mode": *mode, "from": *fromStr, "to_exclusive": *toStr, "decision_points": len(decs),
		"strategies": len(entries), "trades": nTrades, "seconds": math.Round(time.Since(t0).Seconds()),
		"generated_utc": time.Now().UTC().Format(time.RFC3339), "per_strategy": stats,
	}
	b, _ := json.MarshalIndent(meta, "", " ")
	if err := os.WriteFile(filepath.Join(*outDir, "run_"+*mode+".json"), b, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d trades from %d strategies to %s in %.0fs", nTrades, len(entries), *outDir, time.Since(t0).Seconds())
}

func g(x float64) string { return strconv.FormatFloat(x, 'g', 10, 64) }

// registry is run_backtest's strategy set: every OHLCV-compatible entry of the
// scalpers pack plus the ported HW families, first occurrence of a name kept.
func registry(only string) []scalers.RegistryEntry {
	want := map[string]bool{}
	for _, n := range strings.Split(only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			want[n] = true
		}
	}
	seen := map[string]bool{}
	var out []scalers.RegistryEntry
	dupes := 0
	for _, e := range append(scalers.BuildAllScalpers(), scalers.BuildPortedStrategies()...) {
		if !e.OHLCVCompatible || (len(want) > 0 && !want[e.Name]) {
			continue
		}
		if seen[e.Name] {
			dupes++
			continue
		}
		seen[e.Name] = true
		out = append(out, e)
	}
	if dupes > 0 {
		log.Printf("skipped %d duplicate registry names", dupes)
	}
	return out
}

// fingerprint sums every shared candle array so a strategy that writes into the
// slices it is handed (they are shared, as in the engine's ContextBuilder) is caught.
func fingerprint(ds *dataset) [5]float64 {
	var f [5]float64
	for k, s := range []*series{ds.m1, ds.m5, ds.m15, ds.h1, ds.h4} {
		for _, c := range s.c {
			f[k] += c.Open + 2*c.High + 3*c.Low + 4*c.Close + c.Volume + float64(c.OpenTime.Unix()%9973)
		}
	}
	return f
}
