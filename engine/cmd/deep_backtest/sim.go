package main

import (
	"fmt"
	"math"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// The corrected clock ("fixed" mode).
//
// A decision is taken at every 15m close T, exactly when the live desk runs its
// cycle. The context holds only candles that have CLOSED by T on every
// timeframe, Price is the last 1m close, Now is T, and CVD is the live desk's
// cumulative taker flow (CVDPrev = its value one cycle earlier, CVDHistory =
// the five previous cycle snapshots, oldest first). An entry fills at the open
// of the 1m bar that starts at T; the position is then walked minute by minute.
// Within a minute that touches both levels the stop is taken first. A stop or
// target that the next minute OPENS beyond fills at that open (a gap).
//
// Everything else follows the engine's DefaultRunConfig: one position at a
// time, at most 5 entries per strategy per UTC day, a 7-day time stop, the
// strategy's declared regime pinned, SL/TP distances taken as absolute values
// (the v3 adapter's abs()), and 0.35% / 0.70% when a signal carries none.
// Returns are recorded GROSS (entry open to exit level) with funding kept
// separate, so the cost model is applied once, in the analysis.

type simCfg struct {
	maxHold      int64 // seconds
	maxPerDay    int
	defSL, defTP float64 // percent
}

type decision struct {
	T                   int64
	n1, n5, n15, nh, n4 int
	cvd                 [6]float64 // cumulative CVD at T-75m, T-60m, ..., T
}

func buildDecisions(ds *dataset, from, to int64) ([]decision, error) {
	var out []decision
	for k, t := range ds.m15.t {
		T := t + 900
		if T < from || T > to {
			continue
		}
		d := decision{T: T, n1: ds.m1.closedBy(T), n5: ds.m5.closedBy(T), n15: k + 1,
			nh: ds.h1.closedBy(T), n4: ds.h4.closedBy(T)}
		for q := 0; q < 6; q++ {
			d.cvd[q] = ds.cvdCum[ds.m1.closedBy(T-int64(5-q)*900)]
		}
		// No-look-ahead self check: the newest bar of every timeframe closed by T.
		for _, c := range []struct {
			s *series
			n int
		}{{ds.m1, d.n1}, {ds.m5, d.n5}, {ds.m15, d.n15}, {ds.h1, d.nh}, {ds.h4, d.n4}} {
			if c.n == 0 || c.s.t[c.n-1]+c.s.step > T || (c.n < len(c.s.t) && c.s.t[c.n]+c.s.step <= T) {
				return nil, fmt.Errorf("context at %d is not exactly the bars closed by T (step %d)", T, c.s.step)
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func tailS(s []scalers.Candle, n int) []scalers.Candle {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

func (ds *dataset) contextAt(d *decision, regime scalers.Regime) scalers.MarketContext {
	fr, fh := ds.fundingAt(d.T)
	return scalers.MarketContext{
		Regime:         regime,
		Price:          ds.m1.c[d.n1-1].Close,
		Candles1m:      tailS(ds.m1.c[:d.n1], 100),
		Candles5m:      tailS(ds.m5.c[:d.n5], 100),
		Candles15m:     tailS(ds.m15.c[:d.n15], 60),
		Candles1h:      tailS(ds.h1.c[:d.nh], 72),
		Candles4h:      tailS(ds.h4.c[:d.n4], 100),
		CVD:            d.cvd[5],
		CVDPrev:        d.cvd[4],
		CVDHistory:     []float64{d.cvd[0], d.cvd[1], d.cvd[2], d.cvd[3], d.cvd[4]},
		FundingRate:    fr,
		FundingHistory: fh,
		Now:            time.Unix(d.T, 0).UTC(),
	}
}

type tradeRec struct {
	Strategy     string
	Long         bool
	Decision     int64
	Entry, Exit  int64
	EntryPx      float64
	ExitPx       float64
	Reason       string
	SLPct, TPPct float64
	Gross        float64 // side-adjusted fraction of entry notional
	Funding      float64 // signed funding as a fraction of notional, + = paid
	SynthFunding int     // settlements charged at the synthetic 0.01%
	Confidence   float64
	Defaulted    bool // SL or TP missing, engine default used
	Flipped      bool // SL or TP on the wrong side of price, abs() distance used
	NetUSD       float64
	Size         float64
}

type runStats struct {
	Name        string   `json:"strategy"`
	Evals       int      `json:"evaluations"`
	Signals     int      `json:"signals"`
	InPosition  int      `json:"signals_while_in_position"`
	Capped      int      `json:"signals_over_daily_cap"`
	GapSkips    int      `json:"signals_at_data_gap"`
	Entries     int      `json:"entries"`
	Defaulted   int      `json:"entries_with_default_sl_tp"`
	Flipped     int      `json:"entries_with_wrong_side_level"`
	Panic       string   `json:"panic,omitempty"`
	Seconds     float64  `json:"seconds"`
	Description string   `json:"description"`
	Regime      string   `json:"pinned_regime"`
	Timeframes  []string `json:"timeframes"`
}

type position struct {
	tradeRec
	sl, tp float64
	j      int
}

func runFixed(e scalers.RegistryEntry, ds *dataset, decs []decision, cfg simCfg) (trades []tradeRec, st runStats) {
	st.Name = e.Name
	regime := scalers.RegimeTrending
	if len(e.Regimes) > 0 {
		regime = e.Regimes[0]
	}
	var pos *position
	day, dayCount := int64(-1), 0
	for i := range decs {
		d := &decs[i]
		if pos != nil && advance(pos, ds, d.T, cfg) {
			trades = append(trades, finish(pos, ds))
			pos = nil
		}
		sig := e.Strategy.Evaluate(ds.contextAt(d, regime))
		st.Evals++
		if sig.Direction != scalers.DirectionLong && sig.Direction != scalers.DirectionShort {
			continue
		}
		st.Signals++
		if pos != nil {
			st.InPosition++
			continue
		}
		j := d.n1 // first 1m bar not closed by T: the one opening at T
		if j >= len(ds.m1.t) || ds.m1.t[j] != d.T {
			st.GapSkips++
			continue
		}
		if dd := d.T / 86400; dd != day {
			day, dayCount = dd, 0
		}
		if dayCount >= cfg.maxPerDay {
			st.Capped++
			continue
		}
		dayCount++

		long := sig.Direction == scalers.DirectionLong
		ref := ds.m1.c[d.n1-1].Close
		p := &position{j: j}
		p.Strategy, p.Long, p.Decision, p.Entry, p.Confidence = e.Name, long, d.T, d.T, sig.Confidence
		p.EntryPx = ds.m1.c[j].Open
		if sig.StopLoss > 0 {
			p.SLPct = math.Abs(ref-sig.StopLoss) / ref * 100
			p.Flipped = p.Flipped || (long && sig.StopLoss >= ref) || (!long && sig.StopLoss <= ref)
		}
		if sig.TakeProfit > 0 {
			p.TPPct = math.Abs(sig.TakeProfit-ref) / ref * 100
			p.Flipped = p.Flipped || (long && sig.TakeProfit <= ref) || (!long && sig.TakeProfit >= ref)
		}
		if p.SLPct <= 0 {
			p.SLPct, p.Defaulted = cfg.defSL, true
		}
		if p.TPPct <= 0 {
			p.TPPct, p.Defaulted = cfg.defTP, true
		}
		if long {
			p.sl, p.tp = p.EntryPx*(1-p.SLPct/100), p.EntryPx*(1+p.TPPct/100)
		} else {
			p.sl, p.tp = p.EntryPx*(1+p.SLPct/100), p.EntryPx*(1-p.TPPct/100)
		}
		if p.Defaulted {
			st.Defaulted++
		}
		if p.Flipped {
			st.Flipped++
		}
		st.Entries++
		pos = p
	}
	if pos != nil {
		if !advance(pos, ds, math.MaxInt64, cfg) {
			last := len(ds.m1.c) - 1
			pos.Exit, pos.ExitPx, pos.Reason = ds.m1.t[last]+60, ds.m1.c[last].Close, "END"
		}
		trades = append(trades, finish(pos, ds))
	}
	return trades, st
}

// advance walks the open position through the 1m bars that open before upTo and
// reports whether it closed.
func advance(p *position, ds *dataset, upTo int64, cfg simCfg) bool {
	for ; p.j < len(ds.m1.t) && ds.m1.t[p.j] < upTo; p.j++ {
		bt, b := ds.m1.t[p.j], ds.m1.c[p.j]
		if bt >= p.Entry+cfg.maxHold {
			p.Exit, p.ExitPx, p.Reason = bt, b.Open, "TIME"
			return true
		}
		if bt != p.Entry { // the entry bar opens AT the entry price
			if (p.Long && b.Open <= p.sl) || (!p.Long && b.Open >= p.sl) {
				p.Exit, p.ExitPx, p.Reason = bt, b.Open, "SL"
				return true
			}
			if (p.Long && b.Open >= p.tp) || (!p.Long && b.Open <= p.tp) {
				p.Exit, p.ExitPx, p.Reason = bt, b.Open, "TP"
				return true
			}
		}
		hitSL := (p.Long && b.Low <= p.sl) || (!p.Long && b.High >= p.sl)
		hitTP := (p.Long && b.High >= p.tp) || (!p.Long && b.Low <= p.tp)
		if hitSL { // both inside one minute: the stop is assumed first
			p.Exit, p.ExitPx, p.Reason = bt+60, p.sl, "SL"
			return true
		}
		if hitTP {
			p.Exit, p.ExitPx, p.Reason = bt+60, p.tp, "TP"
			return true
		}
	}
	return false
}

func finish(p *position, ds *dataset) tradeRec {
	t := p.tradeRec
	if t.Long {
		t.Gross = (t.ExitPx - t.EntryPx) / t.EntryPx
	} else {
		t.Gross = (t.EntryPx - t.ExitPx) / t.EntryPx
	}
	sign := 1.0
	if !t.Long {
		sign = -1
	}
	for s := (t.Entry/28800 + 1) * 28800; s <= t.Exit; s += 28800 {
		r, real := ds.settlementRate(s)
		t.Funding += sign * r
		if !real {
			t.SynthFunding++
		}
	}
	return t
}
