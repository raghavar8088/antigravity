// Command mtf_qualify runs the multi-timeframe strategy library through real
// historical candles and grades every strategy on data it was not selected from.
//
// This is the piece the desk did not have. The 738 strategies in the MTF pack
// were wired to exactly one thing — the live paper desk — so not one of them had
// ever been through a backtest, an out-of-sample window, or any qualification
// gate. Every live roster to date was chosen off an in-sample leaderboard, and
// the one time that procedure was checked against out-of-sample results, the
// selected streams performed WORSE than the ones passed over (25.0% vs 27.8%
// win rate over 401 live trades). Ranking on in-sample data is not a weaker
// version of ranking properly; on this desk it has been measurably worse than
// not ranking at all.
//
//	go run ./cmd/mtf_qualify -symbols BTCUSD,ETHUSD -days 120 -out qualify.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	scalpers "antigravity-engine/internal/strategy/scalpers"
)

func main() {
	var (
		symbolsCSV = flag.String("symbols", "BTCUSD,ETHUSD,SOLUSD", "comma-separated symbols")
		days       = flag.Int("days", 120, "days of history to fetch")
		trainFrac  = flag.Float64("train-frac", 0.70, "fraction of history used for TRAIN; the rest is out-of-sample")
		tfCSV      = flag.String("tfs", "", "restrict to these timeframes (default: all in the pack)")
		outPath    = flag.String("out", "mtf_qualify.json", "where to write the verdict file")
		workers    = flag.Int("workers", runtime.NumCPU(), "parallel symbol workers")
		minRR      = flag.Float64("min-rr", 6.0, "reward:risk eligibility bar; 0 disables it and trades each strategy at its own target")
	)
	flag.Parse()
	minRewardRisk = *minRR

	symbols := splitCSV(*symbolsCSV)
	if len(symbols) == 0 {
		log.Fatal("no symbols")
	}
	if *trainFrac <= 0 || *trainFrac >= 1 {
		log.Fatalf("train-frac must be strictly between 0 and 1, got %v", *trainFrac)
	}

	// The pack, grouped by timeframe so each strategy is only ever walked over
	// its own series.
	pack := scalpers.BuildHuntPack()
	wanted := map[string]bool{}
	for _, t := range splitCSV(*tfCSV) {
		wanted[strings.ToLower(t)] = true
	}
	type entry struct {
		name string
		tf   scalpers.HigherTF
		st   scalpers.Strategy
	}
	var entries []entry
	tfSet := map[scalpers.HigherTF]bool{}
	for _, e := range pack {
		if len(e.Timeframes) == 0 {
			continue
		}
		tf := scalpers.HigherTF(e.Timeframes[0])
		if len(wanted) > 0 && !wanted[strings.ToLower(string(tf))] {
			continue
		}
		entries = append(entries, entry{e.Name, tf, e.Strategy})
		tfSet[tf] = true
	}
	if len(entries) == 0 {
		log.Fatal("no strategies selected")
	}
	tfs := make([]scalpers.HigherTF, 0, len(tfSet))
	for tf := range tfSet {
		tfs = append(tfs, tf)
	}
	sort.Slice(tfs, func(i, j int) bool { return tfs[i].Step() < tfs[j].Step() })

	to := time.Now().UTC()
	from := to.AddDate(0, 0, -*days)
	fmt.Printf("STRATEGIES : %d across %d timeframe(s)\n", len(entries), len(tfs))
	fmt.Printf("SYMBOLS    : %d — %s\n", len(symbols), strings.Join(symbols, ", "))
	fmt.Printf("HISTORY    : %s -> %s (%d days)\n", from.Format("2006-01-02"), to.Format("2006-01-02"), *days)
	fmt.Printf("SPLIT      : %.0f%% train / %.0f%% out-of-sample\n", *trainFrac*100, (1-*trainFrac)*100)
	fmt.Printf("R:R BAR    : 1:%.0f (eligibility — signals below it are refused, not rescaled)\n\n", minRewardRisk)

	var (
		mu       sync.Mutex
		verdicts []Verdict
		wg       sync.WaitGroup
		sem      = make(chan struct{}, max(1, *workers))
	)

	for _, sym := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			data, err := Load(sym, from, to, tfs)
			if err != nil {
				log.Printf("  %-12s SKIPPED — %v", sym, err)
				return
			}
			local := make([]Verdict, 0, len(entries))
			for _, e := range entries {
				c := data.Series[e.tf]
				if len(c) < e.tf.MinCandles()+2 {
					continue
				}
				// The split is by BAR INDEX on this timeframe, so train and
				// OOS are the same calendar span for every strategy on it.
				split := int(float64(len(c)) * *trainFrac)
				if split <= e.tf.MinCandles() || split >= len(c)-2 {
					continue
				}

				// TRAIN: bars [0, split). OOS: the whole series, with trades
				// opened before the split discarded afterwards.
				//
				// Run over the full series rather than over c[split:] so the
				// OOS window inherits real indicator state instead of
				// restarting cold — a 120-bar warm-up taken from inside the OOS
				// window would spend the first third of it on values no live
				// strategy would have had.
				full := Simulate(e.st, e.name, sym, e.tf, c)
				boundary := c[split].OpenTime

				var trainT, oosT []Trade
				for _, t := range full.Trades {
					if t.OpenedAt.Before(boundary) {
						trainT = append(trainT, t)
					} else {
						oosT = append(oosT, t)
					}
				}

				v := Verdict{
					Strategy:    e.name,
					Family:      familyOf(e.name),
					TF:          string(e.tf),
					Symbol:      sym,
					Train:       ComputeMetrics(trainT),
					OOS:         ComputeMetrics(oosT),
					Signals:     full.Signals,
					RejectedRR:  full.RejectedRR,
					RejectedBad: full.RejectedBad,
				}
				Qualify(&v)
				local = append(local, v)
			}
			mu.Lock()
			verdicts = append(verdicts, local...)
			mu.Unlock()
			log.Printf("  %-12s done — %d strategy runs", sym, len(local))
		}(sym)
	}
	wg.Wait()

	sort.Slice(verdicts, func(i, j int) bool {
		if verdicts[i].Grade != verdicts[j].Grade {
			return verdicts[i].Grade > verdicts[j].Grade
		}
		return verdicts[i].OOS.Expectancy > verdicts[j].OOS.Expectancy
	})

	report(verdicts)

	f, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("write %s: %v", *outPath, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"generated_at":   time.Now().UTC(),
		"symbols":        symbols,
		"days":           *days,
		"train_frac":     *trainFrac,
		"min_rr":         minRewardRisk,
		"round_trip_pct": roundTripCostPct,
		"verdicts":       verdicts,
	}); err != nil {
		log.Fatalf("encode: %v", err)
	}
	fmt.Printf("\nwrote %s (%d verdicts)\n", *outPath, len(verdicts))
}

func report(v []Verdict) {
	byGrade := map[Grade]int{}
	var totalSignals, totalRR, totalTrades int
	for _, x := range v {
		byGrade[x.Grade]++
		totalSignals += x.Signals
		totalRR += x.RejectedRR
		totalTrades += x.Train.Trades + x.OOS.Trades
	}
	fmt.Printf("\n%s\nRESULTS — %d strategy/symbol runs\n%s\n", strings.Repeat("=", 78), len(v), strings.Repeat("=", 78))
	for g := GradeExcellent; g >= GradeRejected; g-- {
		fmt.Printf("  GRADE %-14s %d\n", g.String(), byGrade[g])
	}

	// The 1:6 rule reported as what it is: a filter with a pass rate.
	if totalSignals > 0 {
		fmt.Printf("\n1:6 ELIGIBILITY — %d signals, %d taken (%.1f%%), %d refused for R:R (%.1f%%)\n",
			totalSignals, totalTrades, float64(totalTrades)/float64(totalSignals)*100,
			totalRR, float64(totalRR)/float64(totalSignals)*100)
	}

	fmt.Printf("\nTop by out-of-sample expectancy (grade 3+ only):\n")
	fmt.Printf("%-42s %-8s %-6s %6s %8s %8s %7s %6s  %s\n",
		"STRATEGY", "SYMBOL", "TF", "OOS n", "exp%", "PF", "WR%", "maxDD", "GRADE")
	shown := 0
	for _, x := range v {
		if x.Grade < GradePromising || shown >= 40 {
			continue
		}
		fmt.Printf("%-42s %-8s %-6s %6d %8.3f %8.2f %7.1f %6.1f  %s\n",
			trunc(x.Strategy, 42), trunc(x.Symbol, 8), x.TF, x.OOS.Trades,
			x.OOS.Expectancy, x.OOS.ProfitFactor, x.OOS.WinRate, x.OOS.MaxDrawdownPct, x.Grade)
		shown++
	}
	if shown == 0 {
		// Said plainly rather than printed as an empty table. An empty
		// leaderboard reads as a broken run; "nothing qualified" is a result.
		fmt.Println("  NONE. No strategy/symbol pair reached grade 3 out of sample.")
	}
}

func familyOf(name string) string {
	// MTF_<tf>_<Family>_<Side>
	parts := strings.Split(name, "_")
	if len(parts) < 4 {
		return name
	}
	return strings.Join(parts[2:len(parts)-1], "_")
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, strings.ToUpper(p))
		}
	}
	return out
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
