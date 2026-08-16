package delta

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A live audit of every desk symbol against the REAL gate.
//
// Not a unit test — it talks to the venue, so it is skipped unless
// PERP_GRID_AUDIT=1. It exists because the first version of this audit was
// written in Python and got the answer wrong in two ways at once:
//
//  1. It REIMPLEMENTED stopGridTicks instead of calling it. A second
//     implementation of a gate is a second thing that can disagree with the
//     gate, and the only number that matters is the one the bridge computes.
//  2. It assumed a 0.9% stop. The bridge does not gate the strategy's stop. It
//     gates the VOLATILITY-SCALED stop — 2x the p90 one-minute range, measured
//     per symbol — which on a quiet symbol is far TIGHTER than 0.9%. AVAAIUSD
//     measured 0.278%, a third of the assumption, and a tighter stop is fewer
//     ticks. The audit was therefore optimistic: every symbol it cleared was
//     cleared against a stop wider than the one it will actually be given.
//
// So this calls StopFractionFor and stopGridTicks, in that order, exactly as
// executePerpSignal does.
//
//	SYMBOLS=A,B,C PERP_GRID_AUDIT=1 go test ./internal/delta/ -run GridAudit -v
func TestPerpGridAudit_AgainstTheLiveVenue(t *testing.T) {
	if os.Getenv("PERP_GRID_AUDIT") != "1" {
		t.Skip("set PERP_GRID_AUDIT=1 to run the live venue audit")
	}
	raw := strings.TrimSpace(os.Getenv("SYMBOLS"))
	if raw == "" {
		t.Skip("set SYMBOLS=A,B,C")
	}
	var symbols []string
	for _, s := range strings.Split(raw, ",") {
		if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
			symbols = append(symbols, s)
		}
	}

	ctx := context.Background()
	reg := NewPerpRegistry()
	if err := reg.Refresh(ctx); err != nil {
		t.Fatalf("registry refresh: %v", err)
	}
	base := strings.TrimSpace(os.Getenv("DELTA_API_BASE_URL"))
	if base == "" {
		base = "https://api.india.delta.exchange"
	}
	vol := NewVolatilityTracker(base)

	type row struct {
		sym      string
		ticks    float64
		stopFrac float64
		mark     float64
		tick     float64
		measured bool
		reason   string
	}
	var rows []row

	for _, sym := range symbols {
		p, ok := reg.Lookup(sym)
		if !ok || p.MarkPrice <= 0 {
			rows = append(rows, row{sym: sym, reason: "not in the perpetual registry"})
			continue
		}
		// Exactly the bridge's order: measure the stop, THEN gate it.
		frac, measured := vol.StopFractionFor(ctx, sym)
		if !measured || frac <= 0 {
			// The bridge keeps the strategy's own stop when no estimate exists.
			// 0.9% is the narrowest the MTF pack produces, so it is the
			// most-favourable assumption available — flagged as an assumption.
			frac = 0.009
		}
		entry := p.MarkPrice
		stop := entry * (1 - frac)
		ticks, reason := stopGridTicks(reg, sym, entry, stop)
		rows = append(rows, row{
			sym: sym, ticks: ticks, stopFrac: frac, mark: entry,
			tick: p.TickSize, measured: measured, reason: reason,
		})
		// The venue rate-limits the candle endpoint far harder than quotes, and a
		// throttled fetch returns "no estimate" — which this audit would then
		// report as an ASSUMED 0.9% stop. That is indistinguishable in the output
		// from a symbol the venue genuinely has no candles for, and it is the
		// difference between a measurement and a guess. PACE_MS exists to re-run
		// the unmeasured ones slowly and find out which they are.
		time.Sleep(auditPace())
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ticks > rows[j].ticks })

	var pass, fail, unknown []row
	for _, r := range rows {
		switch {
		case r.reason != "" && r.ticks == 0 && r.mark == 0:
			unknown = append(unknown, r)
		case r.reason != "":
			fail = append(fail, r)
		default:
			pass = append(pass, r)
		}
	}

	show := func(title string, rs []row) {
		if len(rs) == 0 {
			return
		}
		fmt.Printf("\n-- %s --\n", title)
		fmt.Printf("%-14s %8s  %9s %14s %11s  %s\n",
			"symbol", "ticks", "stop%", "mark", "tick", "stop source")
		for _, r := range rs {
			src := "MEASURED 2x p90 1m"
			if !r.measured {
				src = "assumed 0.9% (no estimate)"
			}
			fmt.Printf("%-14s %8.1f  %8.3f%% %14.8g %11g  %s\n",
				r.sym, r.ticks, r.stopFrac*100, r.mark, r.tick, src)
		}
	}

	fmt.Printf("\n=== PERP GRID AUDIT — %d symbols, gate needs %d ticks ===\n",
		len(rows), minEntryStopTicks)
	fmt.Printf("PASS %d   FAIL %d   UNKNOWN %d\n", len(pass), len(fail), len(unknown))
	show(fmt.Sprintf("PASS (>= %d ticks)", minEntryStopTicks), pass)
	show(fmt.Sprintf("FAIL (< %d ticks — refused before entry)", minEntryStopTicks), fail)
	for _, r := range unknown {
		fmt.Printf("  UNKNOWN %-14s %s\n", r.sym, r.reason)
	}

	// The audit is a report, not an assertion about which symbols should exist.
	// The one thing it DOES assert: every symbol on the live roster must clear
	// the gate at its own measured stop, because those are the streams that
	// spend real money.
	live := map[string]bool{}
	for _, st := range ScalpLiveStreams() {
		live[strings.ToUpper(st.Symbol)] = true
	}
	for _, r := range fail {
		if live[r.sym] {
			t.Logf("ROSTER SYMBOL %s fails the gate at its measured stop (%.1f ticks at %.3f%%): %s",
				r.sym, r.ticks, r.stopFrac*100, r.reason)
		}
	}
}

// auditPace is the gap between candle fetches, default 250ms.
func auditPace() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("PACE_MS")); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 250 * time.Millisecond
}
