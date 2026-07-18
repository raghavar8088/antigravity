// Command btc_signal_probe answers one question with evidence instead of
// theory: given the REAL market data of the last N hours, how many signals
// SHOULD the BTC pre-live whitelist have produced?
//
// It fetches fresh Delta Exchange candles, then replays every 15m evaluation
// cycle of the probe window through the exact same strategy code the live
// desk runs (BuildPreLiveStrategies + Evaluate), building the MarketContext
// with the same rolling caps as the live ScalerBundle (100/100/60/72/80).
//
// It evaluates each cycle TWICE with differently-built 4h candles:
//   - "utc4h"  — exchange-aligned 4h candles fetched from Delta directly
//                (what the qualification backtest consumed)
//   - "live4h" — 4h synthesized by merging consecutive 1h bars in groups of 4
//                from an arbitrary warmup anchor (what the live engine does)
//
// If utc4h fires signals but live4h doesn't, the live desk's 4h alignment is
// suppressing qualified setups (a real parity bug). If BOTH are ~zero, the
// desk is faithful and the market genuinely offered no setups.
//
// Analysis tool only — never deployed, never traded.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"antigravity-engine/internal/marketdata"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

const symbol = "BTCUSD"

func toScalerCandles(in []marketdata.HistoricalCandle) []scalers.Candle {
	out := make([]scalers.Candle, 0, len(in))
	for _, c := range in {
		out = append(out, scalers.Candle{
			OpenTime: c.OpenTime, Open: c.Open, High: c.High,
			Low: c.Low, Close: c.Close, Volume: c.Volume,
		})
	}
	return out
}

// lastN returns the trailing n elements (all if fewer).
func lastN(c []scalers.Candle, n int) []scalers.Candle {
	if len(c) <= n {
		return c
	}
	return c[len(c)-n:]
}

// upTo returns candles whose OpenTime is strictly before t.
func upTo(c []scalers.Candle, t time.Time) []scalers.Candle {
	i := len(c)
	for i > 0 && !c[i-1].OpenTime.Before(t) {
		i--
	}
	return c[:i]
}

// synthLive4h merges consecutive 1h candles into 4h buckets of exactly 4 bars,
// starting from the FIRST candle in the slice — mirroring the live engine's
// warmup-anchored synthesis (buckets are NOT aligned to UTC 00/04/08...).
func synthLive4h(h1 []scalers.Candle) []scalers.Candle {
	var out []scalers.Candle
	for i := 0; i+4 <= len(h1); i += 4 {
		g := h1[i : i+4]
		m := scalers.Candle{
			OpenTime: g[0].OpenTime, Open: g[0].Open, High: g[0].High,
			Low: g[0].Low, Close: g[3].Close,
		}
		for _, c := range g {
			if c.High > m.High {
				m.High = c.High
			}
			if c.Low < m.Low {
				m.Low = c.Low
			}
			m.Volume += c.Volume
		}
		out = append(out, m)
	}
	return out
}

func main() {
	probeHours := flag.Int("hours", 16, "probe window length in hours (stepped every 15m)")
	whitelist := flag.String("whitelist", "data/btc_prelive_whitelist.json", "whitelist JSON")
	flag.Parse()

	os.Setenv("PRE_LIVE_WHITELIST_FILE", *whitelist) //nolint:errcheck
	entries := scalers.BuildPreLiveStrategies()
	fmt.Printf("probe: %d whitelisted strategies loaded\n", len(entries))

	now := time.Now().UTC().Truncate(15 * time.Minute)
	probeStart := now.Add(-time.Duration(*probeHours) * time.Hour)

	// History depth: enough for 80×4h of context behind the earliest step.
	histStart := probeStart.Add(-(600 + 24) * time.Hour)
	fmt.Printf("fetching Delta %s candles %s -> %s ...\n", symbol, histStart.Format("01-02 15:04"), now.Format("01-02 15:04"))
	fetch := func(res string, from time.Time) []scalers.Candle {
		c, err := marketdata.FetchDeltaHistoricalCandles(symbol, res, from, now)
		if err != nil {
			log.Fatalf("fetch %s: %v", res, err)
		}
		fmt.Printf("  %-3s: %d candles (last %s)\n", res, len(c), c[len(c)-1].OpenTime.Format("01-02 15:04"))
		return toScalerCandles(c)
	}
	c1m := fetch("1m", probeStart.Add(-3*time.Hour))
	c5m := fetch("5m", probeStart.Add(-12*time.Hour))
	c15 := fetch("15m", probeStart.Add(-18*time.Hour))
	c1h := fetch("1h", histStart)
	c4hUTC := fetch("4h", histStart)

	type hit struct {
		when     time.Time
		strategy string
		dir      string
		conf     float64
		mode     string
	}
	var hits []hit
	evals := 0

	for step := probeStart; !step.After(now); step = step.Add(15 * time.Minute) {
		h1 := upTo(c1h, step)
		ctxBase := scalers.MarketContext{
			Regime:     scalers.RegimeTrending,
			Candles1m:  lastN(upTo(c1m, step), 100),
			Candles5m:  lastN(upTo(c5m, step), 100),
			Candles15m: lastN(upTo(c15, step), 60),
			Candles1h:  lastN(h1, 72),
		}
		if n := len(ctxBase.Candles1m); n > 0 {
			ctxBase.Price = ctxBase.Candles1m[n-1].Close
		}

		for _, mode := range []string{"utc4h", "live4h"} {
			ctx := ctxBase
			if mode == "utc4h" {
				ctx.Candles4h = lastN(upTo(c4hUTC, step), 80)
			} else {
				ctx.Candles4h = lastN(synthLive4h(h1), 80)
			}
			for _, e := range entries {
				evals++
				sig := e.Strategy.Evaluate(ctx)
				if sig.Direction != scalers.DirectionNone {
					hits = append(hits, hit{step, sig.Strategy, string(sig.Direction), sig.Confidence, mode})
				}
			}
		}
	}

	fmt.Printf("\nprobe window: %s -> %s UTC, %d evaluations\n",
		probeStart.Format("01-02 15:04"), now.Format("01-02 15:04"), evals)

	counts := map[string]int{}
	for _, h := range hits {
		counts[h.mode]++
	}
	fmt.Printf("signals with UTC-aligned 4h (backtest-style): %d\n", counts["utc4h"])
	fmt.Printf("signals with live-anchored 4h (engine-style) : %d\n", counts["live4h"])
	fmt.Println("\nfirst 25 signal hits:")
	for i, h := range hits {
		if i >= 25 {
			break
		}
		fmt.Printf("  [%s] %s %-40s %-5s conf=%.2f\n", h.mode, h.when.Format("01-02 15:04"), h.strategy, h.dir, h.conf)
	}
	if len(hits) == 0 {
		fmt.Println("  (none — the whitelist genuinely produced no signals on this window's real data)")
	}
}
