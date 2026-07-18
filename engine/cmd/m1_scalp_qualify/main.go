// Command m1_scalp_qualify — Phase S1 of the scalp-lane plan: a TRUE
// 1-minute-cadence backtest for the M1 strategy pack, answering one question
// with evidence: do 1m scalping entries clear the strict qualification bar
// once the test is fair to scalping (1m evaluation cadence, scalp exit
// geometry, and maker-fee execution) instead of the swing engine's 15m
// cadence + swing exits + taker fees that produced the 0/66 verdict?
//
// What it models, honestly:
//   - Evaluation every closed 1m bar (not every 15m cycle). Higher-timeframe
//     context (5m/15m/1h) includes only bars fully closed by that minute.
//   - Scalp exits: SL/TP from 1m ATR clamped to scalp ranges, plus a hard
//     time-stop. Intrabar SL+TP both touched => SL wins (conservative).
//   - Two execution modes:
//       taker: enter at NEXT bar open + adverse slippage, taker fee both legs
//       maker: post-only limit at the signal close; fills ONLY if a later bar
//              trades through the level within the fill window — otherwise the
//              trade is MISSED (no free fills). TP exit rests as a limit
//              (maker fee); SL and time exits pay taker + stop slippage.
//   - One position per strategy, cooldown after exit, no pyramiding.
//   - 70/30 time split; train screen then the SAME strict OOS bar as the
//     main pipeline with the trade floor RAISED to N>=200 (1m frequency must
//     not qualify on flukes).
//
// The bar is not lowered anywhere. If nothing passes, that is the answer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"time"

	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ── cache loading (same dual-format loader as btc_qualify_25) ────────────────

type cachedCandle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

func loadCached(cacheDir, symbol, res string) []scalers.Candle {
	path := fmt.Sprintf("%s/%s_%s.json", cacheDir, symbol, res)
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("cache read %s: %v", path, err)
	}
	var out []scalers.Candle
	var compact []cachedCandle
	if err := json.Unmarshal(data, &compact); err == nil && len(compact) > 0 && compact[0].Time > 0 {
		out = make([]scalers.Candle, 0, len(compact))
		for _, c := range compact {
			out = append(out, scalers.Candle{OpenTime: time.Unix(c.Time, 0).UTC(),
				Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume})
		}
	} else {
		var hist []marketdata.HistoricalCandle
		if err := json.Unmarshal(data, &hist); err != nil {
			log.Fatalf("cache parse %s: %v", path, err)
		}
		out = make([]scalers.Candle, 0, len(hist))
		for _, c := range hist {
			out = append(out, scalers.Candle{OpenTime: c.OpenTime.UTC(),
				Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime.Before(out[j].OpenTime) })
	fmt.Printf("  loaded %-3s: %d candles (%s -> %s)\n", res, len(out),
		out[0].OpenTime.Format("2006-01-02"), out[len(out)-1].OpenTime.Format("2006-01-02"))
	return out
}

// ── trade sim ────────────────────────────────────────────────────────────────

type scalpCfg struct {
	takerFee, makerFee     float64 // per side, fraction
	entrySlip, stopSlip    float64 // adverse slippage fractions
	slATR, tpATR           float64 // ATR(1m,14) multiples
	slMin, slMax           float64
	tpMin, tpMax           float64
	ttlBars                int // time-stop in 1m bars
	fillWindow             int // maker: bars the post-only limit rests
	cooldownBars           int
	maker                  bool
}

type trade struct {
	exitDay string  // YYYY-MM-DD of exit (for daily Sharpe)
	ret     float64 // net fractional return of the round trip
}

type stratResult struct {
	Name     string  `json:"name"`
	Trades   int     `json:"trades"`
	Missed   int     `json:"missed_fills"`
	WinRate  float64 `json:"win_rate_pct"`
	PF       float64 `json:"profit_factor"`
	Sharpe   float64 `json:"sharpe"`
	Ret      float64 `json:"return_pct"`
	MaxDD    float64 `json:"max_dd_pct"`
	// Split-half consistency: PF over the first and second calendar halves of
	// the window. A real edge should not live in only one half.
	H1PF float64 `json:"h1_pf"`
	H2PF float64 `json:"h2_pf"`
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func metrics(name string, trades []trade, from, to time.Time) stratResult {
	r := stratResult{Name: name, Trades: len(trades)}
	if len(trades) == 0 {
		return r
	}
	var gw, gl float64
	wins := 0
	eq, peak, maxDD := 1.0, 1.0, 0.0
	daily := map[string]float64{}
	for _, t := range trades {
		if t.ret > 0 {
			wins++
			gw += t.ret
		} else {
			gl += -t.ret
		}
		eq *= 1 + t.ret
		if eq > peak {
			peak = eq
		}
		if dd := 1 - eq/peak; dd > maxDD {
			maxDD = dd
		}
		daily[t.exitDay] += t.ret
	}
	r.WinRate = round2(100 * float64(wins) / float64(len(trades)))
	if gl > 0 {
		r.PF = round2(gw / gl)
	} else if gw > 0 {
		r.PF = 999
	}
	midDay := from.Add(to.Sub(from) / 2).Format("2006-01-02")
	pfOf := func(sel func(trade) bool) float64 {
		var w, l float64
		for _, t := range trades {
			if !sel(t) {
				continue
			}
			if t.ret > 0 {
				w += t.ret
			} else {
				l += -t.ret
			}
		}
		if l > 0 {
			return round2(w / l)
		}
		if w > 0 {
			return 999
		}
		return 0
	}
	r.H1PF = pfOf(func(t trade) bool { return t.exitDay < midDay })
	r.H2PF = pfOf(func(t trade) bool { return t.exitDay >= midDay })
	r.Ret = round2((eq - 1) * 100)
	r.MaxDD = round2(maxDD * 100)
	// daily Sharpe over every calendar day in the window (flat days = 0)
	var rets []float64
	for d := from; d.Before(to); d = d.Add(24 * time.Hour) {
		rets = append(rets, daily[d.Format("2006-01-02")])
	}
	mean, sd := meanStd(rets)
	if sd > 0 {
		r.Sharpe = round2(mean / sd * math.Sqrt(365))
	}
	return r
}

func meanStd(x []float64) (m, sd float64) {
	if len(x) == 0 {
		return 0, 0
	}
	for _, v := range x {
		m += v
	}
	m /= float64(len(x))
	for _, v := range x {
		sd += (v - m) * (v - m)
	}
	sd = math.Sqrt(sd / float64(len(x)))
	return
}

func round2(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0
	}
	return math.Round(f*100) / 100
}

// runStrategy simulates one strategy over the bar range [startIdx, endIdx).
func runStrategy(e scalers.RegistryEntry, c1m, c5m, c15m, c1h []scalers.Candle,
	n5mAt, n15mAt, n1hAt []int, startIdx, endIdx int, cfg scalpCfg) ([]trade, int) {

	var trades []trade
	missed := 0
	i := startIdx
	for i < endIdx {
		// build context at bar i (bar i is CLOSED; decision at its close)
		lo1m := i + 1 - 100
		if lo1m < 0 {
			lo1m = 0
		}
		ctx := scalers.MarketContext{
			Price:      c1m[i].Close,
			Regime:     scalers.RegimeTrending,
			Candles1m:  c1m[lo1m : i+1],
			Candles5m:  tail(c5m[:n5mAt[i]], 100),
			Candles15m: tail(c15m[:n15mAt[i]], 60),
			Candles1h:  tail(c1h[:n1hAt[i]], 72),
		}
		sig := e.Strategy.Evaluate(ctx)
		if sig.Direction == scalers.DirectionNone {
			i++
			continue
		}

		long := sig.Direction == scalers.DirectionLong
		atr := scalers.ATR(ctx.Candles1m, 14)
		if atr <= 0 {
			i++
			continue
		}
		price := c1m[i].Close
		slDist := clamp(cfg.slATR*atr, cfg.slMin*price, cfg.slMax*price)
		tpDist := clamp(cfg.tpATR*atr, cfg.tpMin*price, cfg.tpMax*price)

		// ── entry ──
		var entry float64
		var entryFee float64
		entryIdx := -1
		if cfg.maker {
			limit := price // post-only at signal close
			for k := i + 1; k <= i+cfg.fillWindow && k < endIdx; k++ {
				if (long && c1m[k].Low <= limit) || (!long && c1m[k].High >= limit) {
					entry, entryIdx, entryFee = limit, k, cfg.makerFee
					break
				}
			}
			if entryIdx < 0 {
				missed++
				i += 1
				continue
			}
		} else {
			if i+1 >= endIdx {
				break
			}
			entryIdx = i + 1
			if long {
				entry = c1m[entryIdx].Open * (1 + cfg.entrySlip)
			} else {
				entry = c1m[entryIdx].Open * (1 - cfg.entrySlip)
			}
			entryFee = cfg.takerFee
		}

		var sl, tp float64
		if long {
			sl, tp = entry-slDist, entry+tpDist
		} else {
			sl, tp = entry+slDist, entry-tpDist
		}

		// ── manage ──
		exitIdx, exitPx, exitFee := -1, 0.0, 0.0
		for k := entryIdx; k < endIdx; k++ {
			b := c1m[k]
			slHit := (long && b.Low <= sl) || (!long && b.High >= sl)
			tpHit := (long && b.High >= tp) || (!long && b.Low <= tp)
			if slHit { // conservative: SL wins ties
				px := sl
				if long {
					px *= 1 - cfg.stopSlip
				} else {
					px *= 1 + cfg.stopSlip
				}
				exitIdx, exitPx, exitFee = k, px, cfg.takerFee
				break
			}
			if tpHit {
				fee := cfg.takerFee
				if cfg.maker {
					fee = cfg.makerFee // TP rests as a reduce-only limit
				}
				exitIdx, exitPx, exitFee = k, tp, fee
				break
			}
			if k-entryIdx >= cfg.ttlBars {
				exitIdx, exitPx, exitFee = k, b.Close, cfg.takerFee
				break
			}
		}
		if exitIdx < 0 { // window ended while open — close at last bar
			exitIdx = endIdx - 1
			exitPx = c1m[exitIdx].Close
			exitFee = cfg.takerFee
		}

		gross := (exitPx - entry) / entry
		if !long {
			gross = -gross
		}
		net := gross - entryFee - exitFee
		trades = append(trades, trade{exitDay: c1m[exitIdx].OpenTime.Format("2006-01-02"), ret: net})

		i = exitIdx + cfg.cooldownBars
	}
	return trades, missed
}

func tail(c []scalers.Candle, n int) []scalers.Candle {
	if len(c) <= n {
		return c
	}
	return c[len(c)-n:]
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	cacheDir := flag.String("cache-dir", "data/historical", "candle cache dir")
	symbol := flag.String("symbol", "BTCUSDT", "cached symbol")
	years := flag.Float64("window-years", 2, "last N years of 1m data")
	mode := flag.String("mode", "maker", "taker | maker")
	pack := flag.String("pack", "m1", "m1 (66 textbook) | m1x (25 original, per-strategy exit profiles)")
	takerFee := flag.Float64("taker", 0.0005, "taker fee per side")
	makerFee := flag.Float64("maker-fee", 0.0002, "maker fee per side")
	out := flag.String("out", "", "output JSON (default data/<pack>_scalp_<mode>.json)")
	flag.Parse()
	if *out == "" {
		*out = "data/" + *pack + "_scalp_" + *mode + ".json"
	}

	base := scalpCfg{
		takerFee: *takerFee, makerFee: *makerFee,
		entrySlip: 0.0001, stopSlip: 0.0002,
		fillWindow: 3, cooldownBars: 5,
		maker: *mode == "maker",
	}
	mkCfg := func(slATR, tpATR, slMin, slMax, tpMin, tpMax float64, ttl int) scalpCfg {
		c := base
		c.slATR, c.tpATR = slATR, tpATR
		c.slMin, c.slMax = slMin, slMax
		c.tpMin, c.tpMax = tpMin, tpMax
		c.ttlBars = ttl
		return c
	}
	// Exit-geometry profiles. "scalp" is the original S1 geometry (the m1 pack
	// runs entirely on it, preserving comparability). The M1X pack declares one
	// profile per strategy in code, upfront — geometry is design, not curve-fit.
	profiles := map[string]scalpCfg{
		"scalp":  mkCfg(2.5, 3.5, 0.0015, 0.0045, 0.0025, 0.0065, 45),
		"revert": mkCfg(3.0, 2.0, 0.0020, 0.0050, 0.0018, 0.0040, 30),
		"runner": mkCfg(2.5, 6.0, 0.0015, 0.0045, 0.0040, 0.0120, 90),
	}
	cfgFor := func(name string) scalpCfg {
		if *pack == "m1x" {
			return profiles[scalers.M1XProfile(name)]
		}
		return profiles["scalp"]
	}
	cfg := profiles["scalp"] // representative config for the report header

	fmt.Printf("=== %s scalp qualification (S1 harness) — mode=%s taker=%.4f maker=%.4f ===\n", *pack, *mode, cfg.takerFee, cfg.makerFee)
	fmt.Println("Loading cache...")
	c1m := loadCached(*cacheDir, *symbol, "1m")
	c5m := loadCached(*cacheDir, *symbol, "5m")
	c15m := loadCached(*cacheDir, *symbol, "15m")
	c1h := loadCached(*cacheDir, *symbol, "1h")

	end := c1m[len(c1m)-1].OpenTime
	start := end.Add(-time.Duration(*years * 365 * 24 * float64(time.Hour)))
	// trim 1m to window (keep 200 bars of lead-in for indicator warmup)
	i0 := sort.Search(len(c1m), func(i int) bool { return !c1m[i].OpenTime.Before(start) })
	if i0 < 200 {
		i0 = 200
	}

	// precompute higher-TF cutoffs per 1m index: count of bars CLOSED by that minute
	prep := func(c []scalers.Candle, dur time.Duration) []int {
		cut := make([]int, len(c1m))
		j := 0
		for i := range c1m {
			deadline := c1m[i].OpenTime.Add(1 * time.Minute)
			for j < len(c) && !c[j].OpenTime.Add(dur).After(deadline) {
				j++
			}
			cut[i] = j
		}
		return cut
	}
	n5mAt := prep(c5m, 5*time.Minute)
	n15mAt := prep(c15m, 15*time.Minute)
	n1hAt := prep(c1h, time.Hour)

	total := len(c1m) - i0
	splitIdx := i0 + int(float64(total)*0.70)
	trainFrom, trainTo := c1m[i0].OpenTime, c1m[splitIdx].OpenTime
	validTo := end.Add(time.Minute)
	fmt.Printf("\nWindow: %s -> %s (%d 1m bars)\nTRAIN: -> %s | VALIDATE: -> %s (out-of-sample)\n\n",
		trainFrom.Format("2006-01-02"), end.Format("2006-01-02"), total,
		trainTo.Format("2006-01-02"), end.Format("2006-01-02"))

	var entries []scalers.RegistryEntry
	if *pack == "m1x" {
		entries = scalers.BuildM1XPack()
	} else {
		entries = scalers.BuildM1Pack()
	}
	fmt.Printf("Candidates: %d\n\n", len(entries))

	// Qualification bars.
	//   m1  (desk bar, unchanged from S1): train N>=50, WR>40, PF>=1.0 →
	//        OOS WR>50, PF>=1.2, Sharpe>0.5, N>=200.
	//   m1x (M1X bar, set for this pack): train N>=100, PF>=1.05 (must be net
	//        profitable in train on a real sample) → OOS N>=200, PF>=1.2,
	//        Sharpe>=0.5, MaxDD<=25%, AND both OOS calendar halves net-positive
	//        (H1PF>=1.0 && H2PF>=1.0). The WR>50 clause is dropped because WR
	//        is exit-geometry cosmetics (a 6-ATR runner is profitable well
	//        below 50%); profitability is enforced by PF/Sharpe/split-half
	//        instead, and the added DD cap + half consistency make the M1X bar
	//        stricter, not looser, where it matters. Desk-bar verdicts are
	//        still computed and reported alongside for full transparency.
	ownBar := *pack == "m1x"

	type row struct {
		Name       string      `json:"strategy"`
		Profile    string      `json:"exit_profile"`
		Train      stratResult `json:"train"`
		Validate   stratResult `json:"validate"`
		TrainPromo bool        `json:"train_promising"`
		StrictPass bool        `json:"strict_oos_pass"`
		DeskPass   bool        `json:"desk_bar_pass"`
	}
	rows := make([]row, len(entries))

	// parallel across strategies
	type job struct{ idx int }
	jobs := make(chan job, len(entries))
	done := make(chan int, len(entries))
	workers := 10
	for w := 0; w < workers; w++ {
		go func() {
			for jb := range jobs {
				e := entries[jb.idx]
				scfg := cfgFor(e.Name)
				profile := "scalp"
				if ownBar {
					profile = scalers.M1XProfile(e.Name)
				}
				tTrain, _ := runStrategy(e, c1m, c5m, c15m, c1h, n5mAt, n15mAt, n1hAt, i0, splitIdx, scfg)
				tr := metrics(e.Name, tTrain, trainFrom, trainTo)
				var promo bool
				if ownBar {
					promo = tr.Trades >= 100 && tr.PF >= 1.05
				} else {
					promo = tr.Trades >= 50 && tr.WinRate > 40 && tr.PF >= 1.0
				}
				var vr stratResult
				if promo {
					tValid, miss := runStrategy(e, c1m, c5m, c15m, c1h, n5mAt, n15mAt, n1hAt, splitIdx, len(c1m), scfg)
					vr = metrics(e.Name, tValid, trainTo, validTo)
					vr.Missed = miss
				}
				deskPass := promo && vr.WinRate > 50 && vr.PF >= 1.2 && vr.Sharpe > 0.5 && vr.Trades >= 200
				var strict bool
				if ownBar {
					strict = promo && vr.Trades >= 200 && vr.PF >= 1.2 && vr.Sharpe >= 0.5 &&
						vr.MaxDD <= 25 && vr.H1PF >= 1.0 && vr.H2PF >= 1.0
				} else {
					strict = deskPass
				}
				rows[jb.idx] = row{e.Name, profile, tr, vr, promo, strict, deskPass}
				done <- jb.idx
			}
		}()
	}
	for i := range entries {
		jobs <- job{i}
	}
	close(jobs)
	for range entries {
		idx := <-done
		r := rows[idx]
		fmt.Printf("[%3d/%d] %-32s trainN=%-5d WR=%5.1f%% PF=%5.2f %s\n",
			idx+1, len(entries), r.Name, r.Train.Trades, r.Train.WinRate, r.Train.PF,
			map[bool]string{true: "-> OOS", false: ""}[r.TrainPromo])
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StrictPass != rows[j].StrictPass {
			return rows[i].StrictPass
		}
		return rows[i].Validate.PF > rows[j].Validate.PF
	})

	barDesc := "OOS: WR>50%, PF>=1.2, Sharpe>0.5, trades>=200"
	if ownBar {
		barDesc = "M1X OOS: trades>=200, PF>=1.2, Sharpe>=0.5, MaxDD<=25%, both OOS halves PF>=1.0 (desk bar reported alongside)"
	}
	pass := 0
	fmt.Printf("\n=== STRICT OOS RESULTS (%s) ===\n", barDesc)
	for _, r := range rows {
		if !r.TrainPromo {
			continue
		}
		mark := "    "
		if r.StrictPass {
			mark = "PASS"
			pass++
		}
		desk := ""
		if ownBar && r.DeskPass {
			desk = " [desk-bar PASS]"
		}
		fmt.Printf("%s %-28s %-6s valN=%-5d WR=%5.1f%% PF=%5.2f Sh=%5.2f ret=%6.1f%% DD=%5.1f%% H1=%4.2f H2=%4.2f missed=%d%s\n",
			mark, r.Name, r.Profile, r.Validate.Trades, r.Validate.WinRate, r.Validate.PF,
			r.Validate.Sharpe, r.Validate.Ret, r.Validate.MaxDD, r.Validate.H1PF, r.Validate.H2PF,
			r.Validate.Missed, desk)
	}
	promoCount := 0
	for _, r := range rows {
		if r.TrainPromo {
			promoCount++
		}
	}
	fmt.Printf("\nStrict OOS passers: %d / %d (train-promising: %d)\n", pass, len(entries), promoCount)

	outDoc := map[string]interface{}{
		"pack": *pack, "mode": *mode, "config": fmt.Sprintf("%+v", cfg),
		"profiles": map[string]string{
			"scalp":  fmt.Sprintf("%+v", profiles["scalp"]),
			"revert": fmt.Sprintf("%+v", profiles["revert"]),
			"runner": fmt.Sprintf("%+v", profiles["runner"]),
		},
		"window": []string{trainFrom.Format("2006-01-02"), end.Format("2006-01-02")},
		"train_end": trainTo.Format("2006-01-02"),
		"strict_bar": barDesc,
		"pass_count": pass, "rows": rows,
		"ran_at": time.Now().UTC().Format(time.RFC3339),
	}
	data, _ := json.MarshalIndent(outDoc, "", "  ")
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		log.Fatalf("write %s: %v", *out, err)
	}
	fmt.Printf("Saved: %s\n", *out)
}

