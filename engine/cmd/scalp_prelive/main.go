// Command scalp_prelive — the 1m scalp paper desk (Phase S3/S4 of the scalp
// lane, built AFTER qualification failed everywhere, as a live falsification
// experiment on explicit user instruction).
//
// What it does:
//   - Polls Binance public 1m klines for the 8 major symbols, evaluates the
//     100-strategy Scalp100 pack (25 M1X + 66 M1 + 9 M1XB) on every CLOSED
//     1m bar per symbol — 800 independent paper streams.
//   - Models execution IDENTICALLY to the S1 backtest harness, maker mode:
//     post-only limit at signal close, fills only if a later bar trades
//     through the level within 3 bars (missed fills counted), TP rests as a
//     maker limit, SL/time-stop exits pay taker + stop slippage, SL wins
//     intrabar ties, per-strategy exit profiles (scalp/revert/runner),
//     5-bar cooldown. Paper only: $100 notional per trade, no order routing,
//     no API keys, no real-money code path in this binary.
//
// PRE-REGISTERED PROMOTION GATE (set before the first live trade; the only
// route out of paper): a strategy×symbol stream qualifies for a go-live
// DISCUSSION only when its LIVE record shows
//
//	trades >= 200  AND  PF >= 1.2  AND  maxDD <= 25%
//	AND net > 0 in BOTH calendar halves of its live window.
//
// Rationale: all 100 strategies failed offline qualification on all 8
// symbols (0/400). Under the null hypothesis (no edge), with 800 streams a
// few days of trading is EXPECTED to produce dozens of profitable-looking
// leaders by variance alone; the gate exists so luck cannot be promoted.
// Picking early leaderboard toppers without the gate = funding coin flips.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ── execution profiles (EXACT copies of the S1 harness numbers) ──────────────

type profileCfg struct {
	SLATR, TPATR float64
	SLMin, SLMax float64
	TPMin, TPMax float64
	TTLBars      int
}

var profiles = map[string]profileCfg{
	"scalp":  {2.5, 3.5, 0.0015, 0.0045, 0.0025, 0.0065, 45},
	"revert": {3.0, 2.0, 0.0020, 0.0050, 0.0018, 0.0040, 30},
	"runner": {2.5, 6.0, 0.0015, 0.0045, 0.0040, 0.0120, 90},
}

const (
	makerFee     = 0.0002
	takerFee     = 0.0005
	stopSlip     = 0.0002
	fillWindow   = 3
	cooldownBars = 5
)

// defaultNotionalUSD is the per-trade notional the desk starts on. It moved from
// a const to a desk field so the desk can be re-based via /scalp/reset without a
// redeploy — every read and write happens under d.mu.
const defaultNotionalUSD = 100.0

// ── state ────────────────────────────────────────────────────────────────────

type position struct {
	Dir       string    `json:"dir"` // LONG | SHORT
	Entry     float64   `json:"entry"`
	SL        float64   `json:"sl"`
	TP        float64   `json:"tp"`
	EntryBar  int64     `json:"entry_bar"`
	EntryTime time.Time `json:"entry_time"`
	Profile   string    `json:"profile"`
}

type pending struct {
	Dir       string  `json:"dir"`
	Limit     float64 `json:"limit"`
	SLDist    float64 `json:"sl_dist"`
	TPDist    float64 `json:"tp_dist"`
	PlacedBar int64   `json:"placed_bar"`
	Profile   string  `json:"profile"`
}

type comboState struct {
	N               int                `json:"-"`
	Wins            int                `json:"-"`
	Missed          int                `json:"-"`
	GrossW, GrossL  float64            `json:"-"`
	NetSum          float64            `json:"net_sum"`
	Eq, Peak, MaxDD float64            `json:"-"`
	DayNet          map[string]float64 `json:"day_net"`
	Pos             *position          `json:"pos,omitempty"`
	Pend            *pending           `json:"pend,omitempty"`
	CooldownUntil   int64              `json:"cooldown_until"`
	// exported mirrors for snapshot round-trip
	NExp      int     `json:"n"`
	WinsExp   int     `json:"wins"`
	MissedExp int     `json:"missed"`
	GrossWExp float64 `json:"gross_w"`
	GrossLExp float64 `json:"gross_l"`
	EqExp     float64 `json:"eq"`
	PeakExp   float64 `json:"peak"`
	MaxDDExp  float64 `json:"max_dd"`
}

type closedTrade struct {
	Time     time.Time `json:"time"`
	Symbol   string    `json:"symbol"`
	Strategy string    `json:"strategy"`
	Dir      string    `json:"dir"`
	Entry    float64   `json:"entry"`
	Exit     float64   `json:"exit"`
	Reason   string    `json:"reason"`
	RetNet   float64   `json:"ret_net"`
	PnlUSD   float64   `json:"pnl_usd"`
	Profile  string    `json:"profile"`
	HoldMin  int64     `json:"hold_min"`
}

type symbolState struct {
	sym    string
	bars   []scalers.Candle // 1m ring, oldest first
	barIdx int64            // count of processed closed bars
}

type desk struct {
	mu       sync.Mutex
	symbols  []*symbolState
	entries  []scalers.RegistryEntry
	combos   map[string]*comboState // strategy|symbol
	recent   []closedTrade          // ring of last 500
	tradesF  *os.File
	stateDir string
	started  time.Time
	barsSeen int64
	// notionalUSD is the per-trade notional. Guarded by mu; re-basable via
	// /scalp/reset so the desk can be restarted on a different size.
	notionalUSD float64
}

func comboKey(strategy, symbol string) string { return strategy + "|" + symbol }

// ── Binance fetch ────────────────────────────────────────────────────────────

var httpClient = &http.Client{Timeout: 30 * time.Second}

func fetchKlines(sym string, startMs int64, limit int) ([]scalers.Candle, error) {
	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=1m&limit=%d", sym, limit)
	if startMs > 0 {
		url += fmt.Sprintf("&startTime=%d", startMs)
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("binance %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
	}
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]scalers.Candle, 0, len(raw))
	for _, r := range raw {
		if len(r) < 7 {
			continue
		}
		openMs := int64(r[0].(float64))
		open := time.UnixMilli(openMs).UTC()
		if open.Add(time.Minute).After(now) {
			continue // in-progress bar
		}
		pf := func(i int) float64 {
			f, _ := strconv.ParseFloat(r[i].(string), 64)
			return f
		}
		out = append(out, scalers.Candle{
			OpenTime: open, Open: pf(1), High: pf(2), Low: pf(3), Close: pf(4), Volume: pf(5),
		})
	}
	return out, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── resample (closed buckets only, same rule as the backtest harness) ────────

func resample(c []scalers.Candle, step time.Duration, cutoff time.Time) []scalers.Candle {
	var out []scalers.Candle
	var cur *scalers.Candle
	var bucket time.Time
	for i := range c {
		b := c[i].OpenTime.Truncate(step)
		if cur == nil || !b.Equal(bucket) {
			if cur != nil && !bucket.Add(step).After(cutoff) {
				out = append(out, *cur)
			}
			bucket = b
			cp := c[i]
			cp.OpenTime = b
			cur = &cp
		} else {
			cur.High = math.Max(cur.High, c[i].High)
			cur.Low = math.Min(cur.Low, c[i].Low)
			cur.Close = c[i].Close
			cur.Volume += c[i].Volume
		}
	}
	if cur != nil && !bucket.Add(step).After(cutoff) {
		out = append(out, *cur)
	}
	return out
}

func tail(c []scalers.Candle, n int) []scalers.Candle {
	if len(c) <= n {
		return c
	}
	return c[len(c)-n:]
}

// ── trade lifecycle ──────────────────────────────────────────────────────────

func (d *desk) closeTrade(cs *comboState, strategy, symbol string, pos *position, exitPx float64, exitFee float64, reason string, bar scalers.Candle, barIdx int64) {
	gross := (exitPx - pos.Entry) / pos.Entry
	if pos.Dir == "SHORT" {
		gross = -gross
	}
	entryFee := makerFee // this desk models maker entries only
	net := gross - entryFee - exitFee
	cs.N++
	if net > 0 {
		cs.Wins++
		cs.GrossW += net
	} else {
		cs.GrossL += -net
	}
	cs.NetSum += net
	if cs.Eq == 0 {
		cs.Eq, cs.Peak = 1, 1
	}
	cs.Eq *= 1 + net
	if cs.Eq > cs.Peak {
		cs.Peak = cs.Eq
	}
	if dd := 1 - cs.Eq/cs.Peak; dd > cs.MaxDD {
		cs.MaxDD = dd
	}
	day := bar.OpenTime.Format("2006-01-02")
	if cs.DayNet == nil {
		cs.DayNet = map[string]float64{}
	}
	cs.DayNet[day] += net
	ct := closedTrade{
		Time: bar.OpenTime.Add(time.Minute), Symbol: symbol, Strategy: strategy,
		Dir: pos.Dir, Entry: pos.Entry, Exit: exitPx, Reason: reason,
		RetNet: net, PnlUSD: d.notionalUSD * net, Profile: pos.Profile,
		HoldMin: barIdx - pos.EntryBar,
	}
	d.recent = append(d.recent, ct)
	if len(d.recent) > 500 {
		d.recent = d.recent[len(d.recent)-500:]
	}
	if d.tradesF != nil {
		b, _ := json.Marshal(ct)
		d.tradesF.Write(append(b, '\n'))
	}
	cs.Pos = nil
	cs.CooldownUntil = barIdx + cooldownBars
}

// managePosition applies one closed bar to an open position. Same priority
// order as the harness: SL first (conservative), then TP, then time-stop.
func (d *desk) managePosition(cs *comboState, strategy, symbol string, bar scalers.Candle, barIdx int64) {
	pos := cs.Pos
	long := pos.Dir == "LONG"
	slHit := (long && bar.Low <= pos.SL) || (!long && bar.High >= pos.SL)
	tpHit := (long && bar.High >= pos.TP) || (!long && bar.Low <= pos.TP)
	cfg := profiles[pos.Profile]
	switch {
	case slHit:
		px := pos.SL
		if long {
			px *= 1 - stopSlip
		} else {
			px *= 1 + stopSlip
		}
		d.closeTrade(cs, strategy, symbol, pos, px, takerFee, "SL", bar, barIdx)
	case tpHit:
		d.closeTrade(cs, strategy, symbol, pos, pos.TP, makerFee, "TP", bar, barIdx)
	case barIdx-pos.EntryBar >= int64(cfg.TTLBars):
		d.closeTrade(cs, strategy, symbol, pos, bar.Close, takerFee, "TTL", bar, barIdx)
	}
}

// processBar advances one symbol by one closed 1m bar.
func (d *desk) processBar(ss *symbolState, ctx scalers.MarketContext, bar scalers.Candle) {
	barIdx := ss.barIdx
	for _, e := range d.entries {
		key := comboKey(e.Name, ss.sym)
		cs := d.combos[key]
		if cs == nil {
			cs = &comboState{Eq: 1, Peak: 1}
			d.combos[key] = cs
		}
		// 1. manage open position
		if cs.Pos != nil {
			d.managePosition(cs, e.Name, ss.sym, bar, barIdx)
		}
		// 2. pending fill / expiry (fills only on bars AFTER placement)
		if cs.Pos == nil && cs.Pend != nil {
			p := cs.Pend
			if barIdx > p.PlacedBar+int64(fillWindow) {
				cs.Pend = nil
				cs.Missed++
			} else if barIdx > p.PlacedBar {
				long := p.Dir == "LONG"
				filled := (long && bar.Low <= p.Limit) || (!long && bar.High >= p.Limit)
				if filled {
					var sl, tp float64
					if long {
						sl, tp = p.Limit-p.SLDist, p.Limit+p.TPDist
					} else {
						sl, tp = p.Limit+p.SLDist, p.Limit-p.TPDist
					}
					cs.Pos = &position{Dir: p.Dir, Entry: p.Limit, SL: sl, TP: tp,
						EntryBar: barIdx, EntryTime: bar.OpenTime, Profile: p.Profile}
					cs.Pend = nil
					// same-bar exit check, exactly like the harness manage loop
					d.managePosition(cs, e.Name, ss.sym, bar, barIdx)
				}
			}
		}
		// 3. new signal
		if cs.Pos == nil && cs.Pend == nil && barIdx >= cs.CooldownUntil {
			sig := e.Strategy.Evaluate(ctx)
			if sig.Direction == scalers.DirectionNone {
				continue
			}
			atr := scalers.ATR(ctx.Candles1m, 14)
			if atr <= 0 {
				continue
			}
			prof := scalers.ScalpProfileFor(e.Name)
			cfg := profiles[prof]
			price := bar.Close
			slDist := clamp(cfg.SLATR*atr, cfg.SLMin*price, cfg.SLMax*price)
			tpDist := clamp(cfg.TPATR*atr, cfg.TPMin*price, cfg.TPMax*price)
			dir := "LONG"
			if sig.Direction == scalers.DirectionShort {
				dir = "SHORT"
			}
			cs.Pend = &pending{Dir: dir, Limit: price, SLDist: slDist, TPDist: tpDist,
				PlacedBar: barIdx, Profile: prof}
		}
	}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ── polling loop ─────────────────────────────────────────────────────────────

func (d *desk) poll() {
	for _, ss := range d.symbols {
		var startMs int64
		if n := len(ss.bars); n > 0 {
			startMs = ss.bars[n-1].OpenTime.Add(time.Minute).UnixMilli()
		}
		fresh, err := fetchKlines(ss.sym, startMs, 1000)
		if err != nil {
			log.Printf("poll %s: %v", ss.sym, err)
			continue
		}
		d.mu.Lock()
		for _, b := range fresh {
			if n := len(ss.bars); n > 0 && !b.OpenTime.After(ss.bars[n-1].OpenTime) {
				continue
			}
			ss.bars = append(ss.bars, b)
			if len(ss.bars) > 6000 {
				ss.bars = ss.bars[len(ss.bars)-6000:]
			}
			ss.barIdx++
			d.barsSeen++
			if len(ss.bars) < 200 {
				continue
			}
			cutoff := b.OpenTime.Add(time.Minute)
			ctx := scalers.MarketContext{
				Price:      b.Close,
				Regime:     scalers.RegimeTrending,
				Candles1m:  tail(ss.bars, 100),
				Candles5m:  tail(resample(ss.bars, 5*time.Minute, cutoff), 100),
				Candles15m: tail(resample(ss.bars, 15*time.Minute, cutoff), 60),
				Candles1h:  tail(resample(ss.bars, time.Hour, cutoff), 72),
			}
			d.processBar(ss, ctx, b)
		}
		d.mu.Unlock()
	}
}

// ── persistence ──────────────────────────────────────────────────────────────

type snapshot struct {
	SavedAt time.Time              `json:"saved_at"`
	Combos  map[string]*comboState `json:"combos"`
}

func (d *desk) save() {
	d.mu.Lock()
	for _, cs := range d.combos {
		cs.NExp, cs.WinsExp, cs.MissedExp = cs.N, cs.Wins, cs.Missed
		cs.GrossWExp, cs.GrossLExp = cs.GrossW, cs.GrossL
		cs.EqExp, cs.PeakExp, cs.MaxDDExp = cs.Eq, cs.Peak, cs.MaxDD
	}
	snap := snapshot{SavedAt: time.Now().UTC(), Combos: d.combos}
	data, _ := json.Marshal(snap)
	d.mu.Unlock()
	tmp := filepath.Join(d.stateDir, "snapshot.json.tmp")
	dst := filepath.Join(d.stateDir, "snapshot.json")
	if err := os.WriteFile(tmp, data, 0o644); err == nil {
		os.Rename(tmp, dst)
	}
}

func (d *desk) load() {
	data, err := os.ReadFile(filepath.Join(d.stateDir, "snapshot.json"))
	if err != nil {
		return
	}
	var snap snapshot
	if json.Unmarshal(data, &snap) != nil {
		return
	}
	for k, cs := range snap.Combos {
		cs.N, cs.Wins, cs.Missed = cs.NExp, cs.WinsExp, cs.MissedExp
		cs.GrossW, cs.GrossL = cs.GrossWExp, cs.GrossLExp
		cs.Eq, cs.Peak, cs.MaxDD = cs.EqExp, cs.PeakExp, cs.MaxDDExp
		if cs.Eq == 0 {
			cs.Eq, cs.Peak = 1, 1
		}
		// Positions/pendings reference bar indexes from the previous process
		// lifetime; drop them rather than mis-time exits after a restart.
		cs.Pos, cs.Pend = nil, nil
		cs.CooldownUntil = 0
		d.combos[k] = cs
	}
	log.Printf("restored %d combo states from snapshot", len(snap.Combos))
}

// ── HTTP ─────────────────────────────────────────────────────────────────────

const gateDesc = "PRE-REGISTERED LIVE GATE (per strategy×symbol): trades>=200 AND PF>=1.2 AND maxDD<=25% AND net>0 in both calendar halves of its live window. All 100 strategies failed offline qualification 0/400 — expect dozens of lucky positives among 800 streams; only gate survivors earn a go-live discussion."

func (d *desk) gatePass(cs *comboState) bool {
	if cs.N < 200 || cs.MaxDD > 0.25 || cs.GrossL <= 0 {
		return false
	}
	if cs.GrossW/cs.GrossL < 1.2 {
		return false
	}
	days := make([]string, 0, len(cs.DayNet))
	for day := range cs.DayNet {
		days = append(days, day)
	}
	if len(days) < 2 {
		return false
	}
	sort.Strings(days)
	mid := days[len(days)/2]
	var h1, h2 float64
	for day, v := range cs.DayNet {
		if day < mid {
			h1 += v
		} else {
			h2 += v
		}
	}
	return h1 > 0 && h2 > 0
}

func (d *desk) serve(port int) {
	writeJSON := func(w http.ResponseWriter, v interface{}) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(v)
	}
	// Optional token gate (SCALP_API_TOKEN env): when set, every endpoint
	// except /scalp/health requires it via X-API-Token header or ?token=.
	// Same posture as the pre_live desks' token-gated public ports.
	token := os.Getenv("SCALP_API_TOKEN")
	gated := func(h http.HandlerFunc) http.HandlerFunc {
		if token == "" {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-API-Token") != token && r.URL.Query().Get("token") != token {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			h(w, r)
		}
	}
	http.HandleFunc("/scalp/health", func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		last := map[string]string{}
		for _, ss := range d.symbols {
			if n := len(ss.bars); n > 0 {
				last[ss.sym] = ss.bars[n-1].OpenTime.Format(time.RFC3339)
			}
		}
		writeJSON(w, map[string]interface{}{
			"ok": true, "uptime_min": int(time.Since(d.started).Minutes()),
			"bars_processed": d.barsSeen, "strategies": len(d.entries),
			"streams": len(d.entries) * len(d.symbols), "last_bar": last,
		})
	})
	http.HandleFunc("/scalp/stats", gated(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		var n, wins, missed, open, pend int
		var net float64
		perSym := map[string]int{}
		for key, cs := range d.combos {
			n += cs.N
			wins += cs.Wins
			missed += cs.Missed
			net += cs.NetSum
			if cs.Pos != nil {
				open++
			}
			if cs.Pend != nil {
				pend++
			}
			sym := strings.SplitN(key, "|", 2)[1]
			perSym[sym] += cs.N
		}
		writeJSON(w, map[string]interface{}{
			"trades": n, "wins": wins, "missed_fills": missed,
			"open_positions": open, "pending_orders": pend,
			"net_pnl_usd_at_100_notional": math.Round(net*d.notionalUSD*100) / 100,
			"trades_per_symbol":           perSym,
			"gate":                        gateDesc,
		})
	}))
	http.HandleFunc("/scalp/leaderboard", gated(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		defer d.mu.Unlock()
		type lbRow struct {
			Strategy string  `json:"strategy"`
			Symbol   string  `json:"symbol"`
			N        int     `json:"n"`
			WinRate  float64 `json:"wr_pct"`
			PF       float64 `json:"pf"`
			NetUSD   float64 `json:"net_usd"`
			MaxDD    float64 `json:"max_dd_pct"`
			Missed   int     `json:"missed"`
			GatePass bool    `json:"gate_pass"`
		}
		var rows []lbRow
		for key, cs := range d.combos {
			if cs.N == 0 {
				continue
			}
			parts := strings.SplitN(key, "|", 2)
			pf := 0.0
			if cs.GrossL > 0 {
				pf = math.Round(cs.GrossW/cs.GrossL*100) / 100
			} else if cs.GrossW > 0 {
				pf = 999
			}
			rows = append(rows, lbRow{
				Strategy: parts[0], Symbol: parts[1], N: cs.N,
				WinRate: math.Round(10000*float64(cs.Wins)/float64(cs.N)) / 100,
				PF:      pf, NetUSD: math.Round(cs.NetSum*d.notionalUSD*100) / 100,
				MaxDD: math.Round(cs.MaxDD*10000) / 100, Missed: cs.Missed,
				GatePass: d.gatePass(cs),
			})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].NetUSD > rows[j].NetUSD })
		writeJSON(w, map[string]interface{}{"gate": gateDesc, "rows": rows})
	}))
	http.HandleFunc("/scalp/trades", gated(func(w http.ResponseWriter, r *http.Request) {
		n := 50
		if q := r.URL.Query().Get("n"); q != "" {
			if v, err := strconv.Atoi(q); err == nil && v > 0 && v <= 500 {
				n = v
			}
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		start := len(d.recent) - n
		if start < 0 {
			start = 0
		}
		writeJSON(w, d.recent[start:])
	}))
	// ── mutations ────────────────────────────────────────────────────────────
	// Both are token-gated like every other non-health endpoint, and POST-only so
	// a stray GET (a crawler, a prefetch, a mistyped URL) can never wipe the desk.
	// This binary is paper-only — it holds no keys and has no order routing — so
	// these reset paper statistics and nothing else.

	// requestedCapital reads an optional {"initialCapital": N} body. A bad or
	// absent body means "keep the current notional" rather than failing the reset.
	requestedCapital := func(r *http.Request) float64 {
		if r.Body == nil {
			return 0
		}
		var body struct {
			InitialCapital float64 `json:"initialCapital"`
		}
		if json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body) != nil {
			return 0
		}
		if body.InitialCapital <= 0 || math.IsNaN(body.InitialCapital) || math.IsInf(body.InitialCapital, 0) {
			return 0
		}
		return body.InitialCapital
	}

	// truncateTradesLocked empties trades.jsonl. Caller holds d.mu.
	truncateTradesLocked := func() error {
		if d.tradesF == nil {
			return nil
		}
		if err := d.tradesF.Truncate(0); err != nil {
			return err
		}
		_, err := d.tradesF.Seek(0, 0)
		return err
	}

	postOnly := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, `{"error":"POST only"}`, http.StatusMethodNotAllowed)
				return
			}
			h(w, r)
		}
	}

	// /scalp/reset — full re-base: every stream back to zero, optionally on a new
	// per-trade notional. Open positions and pending orders are dropped too,
	// because keeping them would carry P&L priced at the OLD notional into the
	// new account and quietly corrupt the very numbers the reset exists to clear.
	http.HandleFunc("/scalp/reset", gated(postOnly(func(w http.ResponseWriter, r *http.Request) {
		capital := requestedCapital(r)

		d.mu.Lock()
		if capital > 0 {
			d.notionalUSD = capital
		}
		cleared := len(d.combos)
		d.combos = map[string]*comboState{}
		d.recent = nil
		d.barsSeen = 0
		d.started = time.Now().UTC()
		truncErr := truncateTradesLocked()
		notional := d.notionalUSD
		d.mu.Unlock()

		// Persist the cleared state so a restart cannot restore the old snapshot.
		d.save()

		if truncErr != nil {
			log.Printf("[SCALP] reset: trades.jsonl truncate failed: %v", truncErr)
		}
		log.Printf("[SCALP] desk reset — %d combo states cleared, notional now $%.2f", cleared, notional)
		writeJSON(w, map[string]interface{}{
			"status":         "reset",
			"streams_reset":  cleared,
			"notional_usd":   notional,
			"trades_cleared": truncErr == nil,
		})
	})))

	// /scalp/clear-trades — wipe the trade record and the statistics derived from
	// it, but leave open positions and pendings running. Same split the options
	// desks already use: clear-history forgets the past, reset restarts the account.
	http.HandleFunc("/scalp/clear-trades", gated(postOnly(func(w http.ResponseWriter, r *http.Request) {
		d.mu.Lock()
		n := len(d.recent)
		d.recent = nil
		for _, cs := range d.combos {
			cs.N, cs.Wins, cs.Missed = 0, 0, 0
			cs.GrossW, cs.GrossL, cs.NetSum = 0, 0, 0
			cs.Eq, cs.Peak, cs.MaxDD = 1, 1, 0
			cs.DayNet = map[string]float64{}
			// Pos / Pend deliberately preserved — this clears history, not state.
		}
		truncErr := truncateTradesLocked()
		d.mu.Unlock()

		d.save()

		if truncErr != nil {
			log.Printf("[SCALP] clear-trades: trades.jsonl truncate failed: %v", truncErr)
		}
		log.Printf("[SCALP] trade history cleared (%d recent records); open positions kept", n)
		writeJSON(w, map[string]interface{}{
			"status":         "cleared",
			"recent_cleared": n,
			"trades_cleared": truncErr == nil,
		})
	})))

	log.Printf("scalp_prelive HTTP on :%d (/scalp/health /scalp/stats /scalp/leaderboard /scalp/trades /scalp/reset /scalp/clear-trades)", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	port := flag.Int("port", 8093, "HTTP port")
	stateDir := flag.String("state", "data/scalp_prelive", "state directory")
	symbolsCSV := flag.String("symbols", "BTCUSDT,ETHUSDT,SOLUSDT,BNBUSDT,XRPUSDT,DOGEUSDT,ADAUSDT,AVAXUSDT", "symbols")
	flag.Parse()

	if err := os.MkdirAll(*stateDir, 0o755); err != nil {
		log.Fatal(err)
	}
	tradesF, err := os.OpenFile(filepath.Join(*stateDir, "trades.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}

	d := &desk{
		entries:     scalers.BuildScalp100(),
		combos:      map[string]*comboState{},
		tradesF:     tradesF,
		stateDir:    *stateDir,
		started:     time.Now().UTC(),
		notionalUSD: defaultNotionalUSD,
	}
	log.Printf("Scalp100 pack: %d strategies", len(d.entries))
	if len(d.entries) != 100 {
		log.Fatalf("pack size is %d, expected exactly 100", len(d.entries))
	}
	log.Println(gateDesc)
	d.load()

	for _, s := range strings.Split(*symbolsCSV, ",") {
		d.symbols = append(d.symbols, &symbolState{sym: strings.TrimSpace(s)})
	}

	// bootstrap ~75h of 1m context per symbol (need 72 closed 1h bars)
	for _, ss := range d.symbols {
		startMs := time.Now().UTC().Add(-75 * time.Hour).UnixMilli()
		for len(ss.bars) < 4500 {
			page, err := fetchKlines(ss.sym, startMs, 1000)
			if err != nil {
				log.Printf("bootstrap %s: %v (retrying)", ss.sym, err)
				time.Sleep(3 * time.Second)
				continue
			}
			if len(page) == 0 {
				break
			}
			ss.bars = append(ss.bars, page...)
			startMs = page[len(page)-1].OpenTime.Add(time.Minute).UnixMilli()
			time.Sleep(150 * time.Millisecond)
		}
		// bootstrap bars are history, not live signals: mark processed
		ss.barIdx = int64(len(ss.bars))
		log.Printf("bootstrap %s: %d bars (-> %s)", ss.sym, len(ss.bars),
			ss.bars[len(ss.bars)-1].OpenTime.Format("15:04"))
	}

	go d.serve(*port)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	pollT := time.NewTicker(15 * time.Second)
	saveT := time.NewTicker(5 * time.Minute)
	log.Printf("desk live: %d strategies x %d symbols = %d paper streams",
		len(d.entries), len(d.symbols), len(d.entries)*len(d.symbols))
	for {
		select {
		case <-pollT.C:
			d.poll()
		case <-saveT.C:
			d.save()
		case <-sig:
			log.Println("shutdown: saving snapshot")
			d.save()
			tradesF.Close()
			return
		}
	}
}
