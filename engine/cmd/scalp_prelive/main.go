// Command scalp_prelive — the 1m scalp paper desk (Phase S3/S4 of the scalp
// lane, built AFTER qualification failed everywhere, as a live falsification
// experiment on explicit user instruction).
//
// What it does:
//   - Reads 1m bars for the 8 major symbols from the SHARED market-data feed
//     (Delta Exchange primary, Binance fallback) and evaluates the
//     100-strategy Scalp100 pack (25 M1X + 66 M1 + 9 M1XB) on every CLOSED
//     1m bar per symbol — 800 independent paper streams.
//   - Delta is primary because the Live Engine executes on Delta: a strategy
//     scored on another venue's prices is scored on a book it will never
//     trade. The feed polls once per symbol and serves all 800 streams, so
//     upstream request volume tracks instruments, not strategies.
//   - Models execution IDENTICALLY to the S1 backtest harness, maker mode:
//     post-only limit at signal close, fills only if a later bar trades
//     through the level within 3 bars (missed fills counted), TP rests as a
//     maker limit, SL/time-stop exits pay taker + stop slippage, SL wins
//     intrabar ties, per-strategy exit profiles (scalp/revert/runner),
//     5-bar cooldown. Paper only: $1,000 notional per trade, no order
//     routing, no API keys, no real-money code path in this binary.
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
	"context"
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

	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/marketdata/sharedfeed"
	scalers "antigravity-engine/internal/strategy/scalpers"
)

// ── execution profiles ────────────────────────────────────────────────────────
//
// SL/TP are ATR-scaled distances clamped into a [Min,Max] band expressed as a
// fraction of price (see clamp() below) — the floor guarantees a minimum
// stop/target width regardless of how quiet the market is; the ceiling lets a
// genuinely volatile bar widen it further. At defaultNotionalUSD, the floor
// of each band is the dollar risk/reward actually requested for this desk:
// SL floor ≈ -$10 on every profile (never tighter — the old 0.15%-0.20% floor
// let normal 1m noise stop trades out before the setup had room to work), TP
// floor scales with the profile's holding-period ambition for a real 1:2 /
// 1:3 / 1:5 reward-to-risk instead of the old ~1:1.4-2.6 on pocket-change
// distances: scalp $20, revert $30, runner $50.
type profileCfg struct {
	SLATR, TPATR float64
	SLMin, SLMax float64
	TPMin, TPMax float64
	TTLBars      int
}

// Bands are sized so the dollar stop/target are meaningful AND the price move
// is actually reachable inside the holding period. Those two constraints pull
// against each other, and getting the balance wrong is how a scalp desk turns
// into a time-stop machine.
//
// Measured over 500 live trades on the previous 1.00%/2.00% bands:
//
//	TTL  456 (91.2%)  avg  -$0.33   <- 9 in 10 trades never reached either level
//	SL    30 ( 6.0%)  avg -$10.91
//	TP    14 ( 2.8%)  avg +$23.89
//	abs move: median 0.288%, p90 1.090%
//
// The stop sat at 1.00% while the median move was 0.288%: only about a tenth of
// trades travelled far enough to resolve, so the desk's P&L was decided by the
// time-stop rather than by its own risk levels. Widening further would make that
// worse, and simply tightening the percentage would drop the dollar stop below
// the $10 the desk is meant to risk.
//
// So the bands come DOWN to where price actually goes, and the notional goes UP
// to keep the dollars where they belong: SL ~0.35% (inside the median-to-p90
// range, so it resolves often) at $3,000 notional is a ~$10.50 stop, with
// targets at a genuine 1:2 / 1:3 / 1:5 against it.
var profiles = map[string]profileCfg{
	// name     SLATR TPATR  SLMin   SLMax   TPMin   TPMax   TTLBars
	"scalp":  {2.5, 3.5, 0.0035, 0.0060, 0.0070, 0.0120, 60},  // SL $1.05-1.80, TP $2.10-3.60 (1:2)
	"revert": {3.0, 2.0, 0.0035, 0.0060, 0.0105, 0.0180, 45},  // SL $1.05-1.80, TP $3.15-5.40 (1:3)
	"runner": {2.5, 6.0, 0.0035, 0.0060, 0.0175, 0.0300, 120}, // SL $1.05-1.80, TP $5.25-9.00 (1:5)
}

const (
	makerFee     = 0.0002
	takerFee     = 0.0005
	stopSlip     = 0.0002
	fillWindow   = 3
	cooldownBars = 5
)

// defaultNotionalUSD is the per-trade notional the desk starts on.
//
// EVERY strategy runs its own separate $100 account, and this is what $100 can
// put behind one trade: 3x, the aggregate leverage cap the live perpetual desk
// enforces. One strategy holds one position at a time, so 3x is the whole of it.
//
// It was 3,000 — a number chosen so the profile SL/TP bands landed on
// comfortable-looking dollar figures ($10 stops, $20-50 targets). That made the
// desk report tens of dollars per trade on an account that does not exist. The
// live desk has $100. A strategy earning "+$43.53" here would have made $3.65.
//
// Re-basing costs nothing in behaviour: the SL/TP bands above are PRICE
// FRACTIONS (0.0035, 0.0060, ...), not dollars, so every stop, target and exit
// fires at exactly the same price it did before. Only the dollars change, from
// a fictional account to the real one. At $300 the same 0.35% stop is $1.05.
//
// Re-basable at runtime via /scalp/reset — every read and write is under d.mu.
const defaultNotionalUSD = liveSimNotionalUSD

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
	FeeSum          float64            `json:"fee_sum"`
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

	// feed is the shared market-data store. The desk never fetches candles
	// itself: one poller per (symbol, resolution) serves all 800 streams, so
	// upstream request volume tracks instruments rather than strategies.
	feed *sharedfeed.Feed

	// dataSource records which venue the last accepted bars came from. Guarded
	// by mu. Surfaced on /scalp/health because a desk silently running on
	// fallback prices is measuring a book it will not execute on.
	dataSource sharedfeed.Source

	// mirrorOpens counts inverse positions opened from an original's fill;
	// mirrorSkips counts the times one could not be opened because the mirror was
	// still holding the previous trade. Guarded by mu.
	//
	// mirrorSkips is the honesty check on this whole mechanism: it should stay at
	// zero, and a non-zero value means some pairs are no longer exact inverses, so
	// their combined P&L stops being readable as "-2x fees". Cheaper to count than
	// to rediscover from the trade log, which is how the previous, broken mirrors
	// went unnoticed.
	mirrorOpens int64
	mirrorSkips int64

	// live is the optional real-money arm. nil means paper-only, which is the
	// default and the state this desk shipped in for its whole life.
	live *liveDesk
}

// noteSource records the venue that produced the bars just consumed, and logs
// the transition when it changes so a fallback is visible in the log, not just
// in an API field nobody reads.
func (d *desk) noteSource(s sharedfeed.Source) {
	if s == "" {
		return
	}
	d.mu.Lock()
	prev := d.dataSource
	d.dataSource = s
	d.mu.Unlock()
	if prev != "" && prev != s {
		log.Printf("[SCALP] ⚠️  market data source changed %s -> %s", prev, s)
	}
}

func comboKey(strategy, symbol string) string { return strategy + "|" + symbol }

// ── helpers ──────────────────────────────────────────────────────────────────

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

// ── $100 live-account simulation ─────────────────────────────────────────────
//
// The leaderboard's headline P&L is stated at $1,000 notional per trade with
// MAKER fees, because that is how the paper desk fills. Neither matches the
// money: the live perpetual desk runs a $100 account and takes liquidity on
// both legs.
//
// That gap is not cosmetic. These strategies target a few basis points, and a
// taker round trip costs 0.118% of notional — frequently more than the entire
// move. A strategy can rank near the top of the paper board and still be
// structurally unable to pay its own costs, which is exactly what the first
// real fills showed. So the promotion question is answered here, on live terms,
// rather than inferred from a board built on different ones.

// liveSimEquityUSD is the account each strategy is simulated on — the same $100
// the live desk actually runs.
const liveSimEquityUSD = 100.0

// liveSimNotionalUSD is the position size per trade.
//
// The live risk config caps aggregate exposure at 3x equity, and a single
// strategy holds one position at a time, so 3x is what one strategy can reach.
const liveSimNotionalUSD = liveSimEquityUSD * 3

// liveSimTakerRoundTrip is both legs at Delta's taker rate including GST
// (0.059% per side), taken from the venue's own order log rather than docs.
const liveSimTakerRoundTrip = 0.00059 * 2

// liveSimResult is one strategy's record restated on live terms.
type liveSimResult struct {
	GrossUSD float64
	FeesUSD  float64
	NetUSD   float64
	ROIPct   float64
	// FeeDragPct is fees as a share of gross PROFIT. Above 100 means the
	// strategy earns less than it pays to trade.
	FeeDragPct float64
}

// simulateLiveAccount restates a stream's paper record as a $100 taker account.
func simulateLiveAccount(cs *comboState) liveSimResult {
	if cs == nil || cs.N == 0 {
		return liveSimResult{}
	}
	// Recover gross by adding back the maker fees the desk charged, then charge
	// taker fees instead. Both legs, every trade.
	grossFrac := cs.NetSum + cs.FeeSum
	gross := grossFrac * liveSimNotionalUSD
	fees := float64(cs.N) * liveSimNotionalUSD * liveSimTakerRoundTrip
	net := gross - fees
	drag := 0.0
	if gross > 0 {
		drag = fees / gross * 100
	} else if fees > 0 {
		// No gross profit to pay from. Reported as fully drag rather than as a
		// divide-by-zero blank, which would read as "no fee problem".
		drag = 100
	}
	return liveSimResult{
		GrossUSD:   math.Round(gross*100) / 100,
		FeesUSD:    math.Round(fees*100) / 100,
		NetUSD:     math.Round(net*100) / 100,
		ROIPct:     math.Round(net/liveSimEquityUSD*10000) / 100,
		FeeDragPct: math.Round(drag*10) / 10,
	}
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
	// The fees THIS desk charged, kept so gross is recoverable.
	//
	// Without it only the net return survives, and the desk's net is a MAKER
	// net — it models post-only entries. The live path is taker on both legs.
	// Comparing a maker-fee paper result against a taker-fee live result is the
	// single biggest reason a paper leader disappoints with real money, and
	// nothing on this desk could measure the gap.
	cs.FeeSum += entryFee + exitFee
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
		// 1b. manage this strategy's mirror, if it holds one. Mirrors never
		// evaluate signals or place pendings — they exist only as the inverse of
		// a fill the original already took — so they are advanced here rather
		// than in their own pass over d.entries.
		d.manageMirror(e.Name, ss.sym, bar, barIdx)
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

					// Open the mirror on the SAME fill, before the exit check.
					//
					// The mirror inherits this fill rather than posting an order
					// of its own. Posting its own is what broke the previous
					// attempt: a post-only limit on the opposite side fills on the
					// opposite price condition, so the pair almost never both
					// traded and the mirror only ever filled when price moved its
					// way. Inheriting the fill makes the pair exact by
					// construction — same bar, same entry, opposite side, stop and
					// target distances swapped.
					d.openMirror(e.Name, ss.sym, cs.Pos, barIdx, bar)

					// Real money, if and only if this stream is allow-listed and
					// the bridge is armed. Paper accounting above is unaffected
					// either way, so the leaderboard stays a clean measurement
					// rather than a mixture of paper and live outcomes.
					d.live.onPaperFill(e.Name, ss.sym, cs.Pos)

					// same-bar exit check, exactly like the harness manage loop.
					// The mirror gets the same treatment on the same bar: giving
					// the original a same-bar exit and not the mirror would let a
					// pair that opened together close a bar apart, at prices that
					// no longer negate.
					d.managePosition(cs, e.Name, ss.sym, bar, barIdx)
					d.manageMirror(e.Name, ss.sym, bar, barIdx)
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
		// Read from the shared feed instead of fetching per symbol.
		//
		// The desk runs 800 streams (100 strategies x 8 symbols). Fetching here
		// would tie request volume to the desk's own loop; the shared feed polls
		// each (symbol, resolution) once and serves every reader from stored
		// bars, so upstream load tracks INSTRUMENTS, not strategies. Delta is the
		// source — the Live Engine executes on Delta, so a strategy scored on
		// another venue's prices is scored on a book it will never trade — with
		// Binance as fallback when Delta rate-limits or fails.
		snap := d.feed.Get(ss.sym, "1m")
		if len(snap.Bars) == 0 {
			if snap.LastErr != "" {
				log.Printf("poll %s: feed has no bars (%s)", ss.sym, snap.LastErr)
			}
			continue
		}
		if snap.Stale {
			log.Printf("poll %s: feed STALE (last update %s) — skipping, not trading a frozen book",
				ss.sym, snap.UpdatedAt.Format(time.RFC3339))
			continue
		}
		d.noteSource(snap.Source)

		fresh := make([]scalers.Candle, 0, len(snap.Bars))
		for _, b := range snap.Bars {
			fresh = append(fresh, scalers.Candle{
				OpenTime: b.OpenTime, Open: b.Open, High: b.High,
				Low: b.Low, Close: b.Close, Volume: b.Volume,
			})
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
	// DataVenue records which market the stored record was earned on. A record
	// built on Binance prices is not comparable to one built on Delta prices —
	// different book, different spread, different fills — so a venue change
	// discards the old statistics instead of silently averaging two experiments
	// into one leaderboard. Empty means "pre-versioning", i.e. Binance era.
	DataVenue string `json:"data_venue,omitempty"`
	// MirrorModel records how the ANTI_ records were produced. Empty means the
	// original signal-inverting mirrors, whose fills were selection-biased.
	MirrorModel string `json:"mirror_model,omitempty"`
}

// currentDataVenue is the venue this build measures on. Changing it invalidates
// every persisted combo record by design.
const currentDataVenue = "delta"

// currentMirrorModel names how ANTI_ records were produced. Changing it discards
// the stored mirror records — and only those.
//
// "signal" mirrors were separate registry entries that inverted the signal and
// posted their own post-only limit. They filled on the opposite price condition
// from their original, so they rarely traded as a pair and the ones that did
// trade were selected for having started in their favour. Their statistics
// describe that selection, not the originals they are named after.
//
// The originals' records are NOT affected: under the old scheme the mirrors were
// independent streams and never changed how an original filled. So a model
// change drops the ANTI_ combos and leaves everything else standing, rather than
// resetting the whole desk and throwing away progress toward the 200-trade gate.
const currentMirrorModel = "fill-inherited"

func (d *desk) save() {
	d.mu.Lock()
	for _, cs := range d.combos {
		cs.NExp, cs.WinsExp, cs.MissedExp = cs.N, cs.Wins, cs.Missed
		cs.GrossWExp, cs.GrossLExp = cs.GrossW, cs.GrossL
		cs.EqExp, cs.PeakExp, cs.MaxDDExp = cs.Eq, cs.Peak, cs.MaxDD
	}
	snap := snapshot{SavedAt: time.Now().UTC(), Combos: d.combos,
		DataVenue: currentDataVenue, MirrorModel: currentMirrorModel}
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

	// Venue cutover: a record earned on Binance prices cannot be carried into a
	// Delta-priced experiment. Different book, different spread, different
	// fills — merging them would produce a leaderboard where a stream's
	// statistics come from two different markets, and no amount of later
	// trading would separate them again. Discard and start clean.
	if snap.DataVenue != currentDataVenue {
		was := snap.DataVenue
		if was == "" {
			was = "binance (pre-versioning)"
		}
		log.Printf("[SCALP] venue changed %s -> %s: discarding %d stored combo record(s); the live record restarts on %s prices",
			was, currentDataVenue, len(snap.Combos), currentDataVenue)
		return
	}

	// Mirror-model cutover: drop the ANTI_ records built by the old
	// signal-inverting mirrors, and only those. Their trades came from a fill
	// model that could not pair them with their originals, so keeping them would
	// mix two incompatible mechanisms under one name. The originals were never
	// touched by that bug, so their records — and their progress toward the
	// 200-trade gate — survive the change.
	dropMirrors := snap.MirrorModel != currentMirrorModel
	dropped := 0

	for k, cs := range snap.Combos {
		if dropMirrors && scalers.IsAntiStrategy(strings.SplitN(k, "|", 2)[0]) {
			dropped++
			continue
		}
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
	if dropped > 0 {
		was := snap.MirrorModel
		if was == "" {
			was = "signal-inverted (unpaired fills)"
		}
		log.Printf("[SCALP] mirror model changed %s -> %s: discarded %d ANTI_ record(s); mirrors restart, originals keep theirs",
			was, currentMirrorModel, dropped)
	}
	log.Printf("restored %d combo states from snapshot", len(snap.Combos)-dropped)
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
		// Feed health is read outside d.mu — the feed has its own lock, and
		// calling into it while holding d.mu invites the lock-ordering problem
		// that already deadlocked one desk in this codebase.
		src := string(d.dataSource)
		d.mu.Unlock()
		feedHealth := []map[string]interface{}{}
		stale := 0
		if d.feed != nil {
			for _, h := range d.feed.Health() {
				if h.Stale {
					stale++
				}
				feedHealth = append(feedHealth, map[string]interface{}{
					"symbol": h.Symbol, "source": string(h.Source), "bars": len(h.Bars),
					"stale": h.Stale, "updated_at": h.UpdatedAt, "last_error": h.LastErr,
				})
			}
		}
		d.mu.Lock() // re-taken so the deferred Unlock stays balanced

		writeJSON(w, map[string]interface{}{
			"ok": true, "uptime_min": int(time.Since(d.started).Minutes()),
			"bars_processed": d.barsSeen, "strategies": len(d.streamNames()),
			"streams": len(d.streamNames()) * len(d.symbols), "last_bar": last,
			// Signal-emitting strategies, excluding the mirrors that ride their
			// fills. strategies-minus-originals is the mirror count.
			"originals": len(d.entries),
			// Which venue the record is actually being earned on. If this reads
			// "binance" the desk is measuring a book the Live Engine does not
			// execute on, and the numbers are not promotion evidence.
			"data_venue":  src,
			"stale_pairs": stale,
			"feed":        feedHealth,
			// Mirror pairing. mirror_skips must stay 0: any other value means a
			// pair drifted out of step and its two halves are no longer exact
			// inverses of each other.
			"mirror_opens": d.mirrorOpens,
			"mirror_skips": d.mirrorSkips,
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
			// Kept under its historical key so existing consumers do not break, but the
			// basis is now the $100 account. capital_usd below is the honest name.
			"net_pnl_usd_at_1000_notional": math.Round(net*d.notionalUSD*100) / 100,
			"notional_usd":                 d.notionalUSD,
			"account_equity_usd":           liveSimEquityUSD,
			"trades_per_symbol":            perSym,
			"gate":                         gateDesc,
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
			// The same record restated on the live desk's terms: a $100 account,
			// 3x notional, taker fees both legs. This is the number a go-live
			// decision should be made on; NetUSD above is $1,000 notional with
			// maker fees and flatters every strategy on this board.
			// CapitalUSD is what this strategy's own $100 account is worth now.
			CapitalUSD  float64 `json:"capital_usd"`
			LiveNetUSD  float64 `json:"live_net_usd"`
			LiveROIPct  float64 `json:"live_roi_pct"`
			LiveFeesUSD float64 `json:"live_fees_usd"`
			LiveDragPct float64 `json:"live_fee_drag_pct"`
			NetUSD      float64 `json:"net_usd"`
			MaxDD       float64 `json:"max_dd_pct"`
			Missed      int     `json:"missed"`
			GatePass    bool    `json:"gate_pass"`
		}
		// EVERY stream appears, including ones that have not traded yet.
		//
		// Building the board only from combos with trades hid the vast majority
		// of the desk: 2,416 streams run, but a combo state exists only once a
		// stream has closed something, so minutes after a restart the board
		// showed seven rows and looked like a seven-strategy desk. A strategy
		// waiting for its first setup is a strategy an operator needs to see.
		// Mirrors are not registry entries — they have no Evaluate() and never
		// place an order — so enumerating d.entries alone would leave half the
		// desk off its own leaderboard. streamNames() is the roster of what
		// actually runs.
		names := d.streamNames()
		rows := make([]lbRow, 0, len(names)*len(d.symbols))
		for _, name := range names {
			for _, ss := range d.symbols {
				key := comboKey(name, ss.sym)
				cs, traded := d.combos[key]
				if !traded || cs.N == 0 {
					// Funded and running, no closed trade yet. Zeroed rather
					// than omitted, so the row count matches the stream count.
					rows = append(rows, lbRow{Strategy: name, Symbol: ss.sym})
					continue
				}
				parts := []string{name, ss.sym}
				pf := 0.0
				if cs.GrossL > 0 {
					pf = math.Round(cs.GrossW/cs.GrossL*100) / 100
				} else if cs.GrossW > 0 {
					pf = 999
				}
				sim := simulateLiveAccount(cs)
				rows = append(rows, lbRow{
					Strategy: parts[0], Symbol: parts[1], N: cs.N,
					WinRate: math.Round(10000*float64(cs.Wins)/float64(cs.N)) / 100,
					PF:      pf, NetUSD: math.Round(cs.NetSum*d.notionalUSD*100) / 100,
					MaxDD: math.Round(cs.MaxDD*10000) / 100, Missed: cs.Missed,
					GatePass:    d.gatePass(cs),
					CapitalUSD:  math.Round((liveSimEquityUSD+sim.NetUSD)*100) / 100,
					LiveNetUSD:  sim.NetUSD,
					LiveROIPct:  sim.ROIPct,
					LiveFeesUSD: sim.FeesUSD,
					LiveDragPct: sim.FeeDragPct,
				})
			}
		}
		// Traded streams first, best net at the top; untraded ones fall to the
		// bottom rather than being scattered through the ranking at zero.
		sort.Slice(rows, func(i, j int) bool {
			if (rows[i].N > 0) != (rows[j].N > 0) {
				return rows[i].N > 0
			}
			return rows[i].NetUSD > rows[j].NetUSD
		})
		writeJSON(w, map[string]interface{}{"gate": gateDesc, "rows": rows})
	}))
	// /scalp/positions — open PAPER positions, with the live-enabled ones marked.
	//
	// The desk runs 2,416 streams and holds a few hundred positions at any
	// moment. Almost none of them matter to an operator watching real money:
	// only the streams on the live allow-list can reach the venue, and those are
	// the ones whose paper position predicts a live one.
	//
	// So each row carries `live`, and the UI shows the live-enabled ones by
	// default. The rest are still returned rather than dropped — a filter the
	// caller can widen is honest; a server that silently omits data is not.
	http.HandleFunc("/scalp/positions", gated(func(w http.ResponseWriter, r *http.Request) {
		liveNames := map[string]bool{}
		for _, n := range delta.ScalpLiveStrategies() {
			liveNames[n] = true
		}
		liveSyms := map[string]bool{}
		for _, sy := range scalpLiveSymbols() {
			liveSyms[strings.ToUpper(sy)] = true
		}

		type row struct {
			Strategy string    `json:"strategy"`
			Symbol   string    `json:"symbol"`
			Dir      string    `json:"dir"`
			Entry    float64   `json:"entry"`
			SL       float64   `json:"sl"`
			TP       float64   `json:"tp"`
			Profile  string    `json:"profile"`
			OpenedAt time.Time `json:"openedAt"`
			HeldMin  int64     `json:"heldMin"`
			// Mark is the last closed 1m bar for the symbol, and PnL is stated at
			// the desk's standard $1,000 notional — the same basis as
			// net_pnl_usd_at_1000_notional on /scalp/stats, so an open position and
			// a closed one can be read on one scale.
			Mark      float64 `json:"mark"`
			PnLPct    float64 `json:"pnlPct"`
			PnLAt1000 float64 `json:"pnlAt1000"`
			// Live is true when this exact (strategy, symbol) stream is on the
			// live allow-list — i.e. a fill here would also place a real order.
			Live bool `json:"live"`
		}

		d.mu.Lock()
		defer d.mu.Unlock()

		// Last closed bar per symbol. A position with no mark reports 0 rather
		// than a stale entry-equals-mark zero, which would read as flat.
		marks := make(map[string]float64, len(d.symbols))
		for _, ss := range d.symbols {
			if n := len(ss.bars); n > 0 {
				marks[ss.sym] = ss.bars[n-1].Close
			}
		}

		out := make([]row, 0, 64)
		liveOpen := 0
		for key, cs := range d.combos {
			if cs.Pos == nil {
				continue
			}
			parts := strings.SplitN(key, "|", 2)
			if len(parts) != 2 {
				continue
			}
			strat, sym := parts[0], parts[1]
			isLive := liveNames[strat] && (len(liveSyms) == 0 || liveSyms[strings.ToUpper(sym)])
			if isLive {
				liveOpen++
			}
			held := int64(0)
			if !cs.Pos.EntryTime.IsZero() {
				held = int64(time.Since(cs.Pos.EntryTime).Minutes())
			}
			mark := marks[sym]
			pnlPct, pnl1000 := 0.0, 0.0
			if mark > 0 && cs.Pos.Entry > 0 {
				dir := 1.0
				if cs.Pos.Dir == "SHORT" {
					dir = -1.0
				}
				pnlPct = (mark - cs.Pos.Entry) / cs.Pos.Entry * dir * 100
				pnl1000 = pnlPct / 100 * 1000
			}
			out = append(out, row{
				Strategy: strat, Symbol: sym, Dir: cs.Pos.Dir,
				Entry: cs.Pos.Entry, SL: cs.Pos.SL, TP: cs.Pos.TP,
				Profile: cs.Pos.Profile, OpenedAt: cs.Pos.EntryTime,
				HeldMin: held, Live: isLive,
				Mark: mark, PnLPct: pnlPct, PnLAt1000: pnl1000,
			})
		}
		// Live-enabled first, then longest held — the ones closest to their time
		// stop are the ones about to resolve.
		sort.Slice(out, func(i, j int) bool {
			if out[i].Live != out[j].Live {
				return out[i].Live
			}
			return out[i].HeldMin > out[j].HeldMin
		})

		// The live ROSTER, not just the strategies that happen to hold a position
		// right now. A leaderboard built from open positions alone would silently
		// omit every live strategy that is currently flat — which is most of them,
		// most of the time.
		roster := make([]string, 0, len(liveNames))
		for n := range liveNames {
			roster = append(roster, n)
		}
		sort.Strings(roster)

		writeJSON(w, map[string]interface{}{
			"open":            len(out),
			"live_open":       liveOpen,
			"live_strategies": roster,
			"rows":            out,
		})
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

	d.live.registerHTTP(gated, postOnly, writeJSON)
	log.Printf("scalp_prelive HTTP on :%d (/scalp/health /scalp/stats /scalp/leaderboard /scalp/trades /scalp/reset /scalp/clear-trades)", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	port := flag.Int("port", 8093, "HTTP port")
	stateDir := flag.String("state", "data/scalp_prelive", "state directory")
	// Delta symbol naming (BTCUSD, not BTCUSDT). Delta is the venue the Live
	// Engine executes on, so the desk measures strategies on Delta's own book;
	// the shared feed translates to Binance's USDT naming only when it has to
	// fall back. All eight are confirmed live perpetuals on Delta India.
	symbolsCSV := flag.String("symbols", "BTCUSD,ETHUSD,SOLUSD,BNBUSD,XRPUSD,DOGEUSD,ADAUSD,AVAXUSD", "symbols")
	flag.Parse()

	if err := os.MkdirAll(*stateDir, 0o755); err != nil {
		log.Fatal(err)
	}
	tradesF, err := os.OpenFile(filepath.Join(*stateDir, "trades.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}

	d := &desk{
		// Every registered scalp strategy, not just the Scalp100 pack. 51 of the
		// 151 that exist here (17 Delta20 + 34 Curated) were built and never
		// run, so the hunt could not have found them however good they were.
		entries:     scalers.BuildHuntPack(),
		combos:      map[string]*comboState{},
		tradesF:     tradesF,
		stateDir:    *stateDir,
		started:     time.Now().UTC(),
		notionalUSD: defaultNotionalUSD,
	}
	log.Printf("hunt pack: %d signal strategies + %d mirrors = %d streams/symbol",
		len(d.entries), len(d.streamNames())-len(d.entries), len(d.streamNames()))
	// Guard against an empty or accidentally-truncated pack rather than pinning
	// an exact count: the pack is meant to grow as strategies are registered,
	// and a hard 100 would fail the build every time one was added.
	if len(d.entries) < 100 {
		log.Fatalf("hunt pack has only %d strategies; expected at least the 100-strategy Scalp100 baseline", len(d.entries))
	}
	log.Println(gateDesc)
	d.load()

	for _, s := range strings.Split(*symbolsCSV, ",") {
		d.symbols = append(d.symbols, &symbolState{sym: strings.TrimSpace(s)})
	}

	// One shared feed for the whole desk: 8 pollers serve all 800 streams.
	// Backfill covers the ~75h of 1m context the strategies need (72 closed 1h
	// bars), so the feed itself does the bootstrap that used to be a per-symbol
	// paginated fetch loop here.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Optional live arm. Returns nil (paper-only) unless SCALP_LIVE_ENABLED is
	// true AND Delta credentials are present AND the product registry loaded.
	// Built here rather than at desk construction because it needs the process
	// context: its monitor and product-refresh loops must stop with the desk.
	d.live = newLiveDesk(ctx)
	d.live.reportUnknown(d.streamNames())

	d.feed = sharedfeed.New(sharedfeed.Config{
		Poll:       time.Minute,
		Backfill:   75 * time.Hour,
		MaxBars:    6000,
		StaleAfter: 5 * time.Minute,
		Primary:    sharedfeed.DeltaFetcher,
		Fallback:   sharedfeed.BinanceFetcher,
	})
	pairs := make([]sharedfeed.Pair, 0, len(d.symbols))
	for _, ss := range d.symbols {
		pairs = append(pairs, sharedfeed.Pair{Symbol: ss.sym, Resolution: "1m"})
	}
	d.feed.Start(ctx, pairs)

	// Wait for the backfill before evaluating anything. Trading a strategy on a
	// half-filled indicator window produces garbage that then pollutes its
	// permanent live record.
	log.Printf("waiting for shared feed backfill on %d symbol(s)...", len(pairs))
	for _, ss := range d.symbols {
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			if snap := d.feed.Get(ss.sym, "1m"); len(snap.Bars) >= 200 {
				break
			}
			time.Sleep(2 * time.Second)
		}

		// Seed the symbol's own window from the feed. This has to happen here:
		// poll() only appends bars NEWER than the last one it holds, so without
		// a seed the desk would start from an empty window and take three days
		// to accumulate the 1h context its strategies need.
		snap := d.feed.Get(ss.sym, "1m")
		for _, b := range snap.Bars {
			ss.bars = append(ss.bars, scalers.Candle{
				OpenTime: b.OpenTime, Open: b.Open, High: b.High,
				Low: b.Low, Close: b.Close, Volume: b.Volume,
			})
		}
		if len(ss.bars) == 0 {
			log.Printf("bootstrap %s: NO BARS — feed unavailable (%s); this symbol will idle until data arrives",
				ss.sym, snap.LastErr)
			continue
		}
		// Bootstrap bars are history, not live signals: mark them processed so
		// the desk does not fire 4,500 backdated entries on its first tick.
		ss.barIdx = int64(len(ss.bars))
		d.noteSource(snap.Source)
		log.Printf("bootstrap %s: %d bars from %s (-> %s)", ss.sym, len(ss.bars), snap.Source,
			ss.bars[len(ss.bars)-1].OpenTime.Format("15:04"))
	}

	go d.serve(*port)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	pollT := time.NewTicker(15 * time.Second)
	saveT := time.NewTicker(5 * time.Minute)
	log.Printf("desk live: %d strategies (%d signal + %d mirror) x %d symbols = %d paper streams",
		len(d.streamNames()), len(d.entries), len(d.streamNames())-len(d.entries),
		len(d.symbols), len(d.streamNames())*len(d.symbols))
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

// ── fill-time mirroring ──────────────────────────────────────────────────────
//
// An anti-strategy is the exact inverse of its original's REALISED trade, not a
// separate strategy that happens to trade the other way.
//
// The distinction is not cosmetic. The first attempt inverted each strategy's
// signal and let the mirror post its own post-only limit. Under this desk's fill
// rule —
//
//	filled := (long && bar.Low <= limit) || (!long && bar.High >= limit)
//
// — a buy limit and a sell limit at the same price fill on opposite conditions.
// Price cannot satisfy both on one bar, so the two halves almost never traded
// together (35 of 53 traded streams had no partner) and the mirror filled only
// when price moved toward its limit. That is a selection bias, and it produced
// four 100%-win-rate rows at the top of the leaderboard that meant nothing.
//
// Inheriting the fill removes the choice entirely: if the original traded, the
// mirror traded, on the same bar at the same price. Their P&L then sums to
// exactly minus the fees both paid, which is the property that makes the pair
// worth reading.

// mirrorOf returns the mirror's strategy name, or "" if the given name is itself
// a mirror (mirrors are not mirrored).
func mirrorOf(name string) string {
	if scalers.IsAntiStrategy(name) {
		return ""
	}
	return scalers.AntiPrefix + name
}

// openMirror creates the inverse position for a fill the original just took.
//
// Caller holds d.mu and has already set orig.
func (d *desk) openMirror(strategy, symbol string, orig *position, barIdx int64, bar scalers.Candle) {
	if !antiEnabled || orig == nil {
		return
	}
	mk := mirrorOf(strategy)
	if mk == "" {
		return
	}

	key := comboKey(mk, symbol)
	mcs := d.combos[key]
	if mcs == nil {
		mcs = &comboState{Eq: 1, Peak: 1}
		d.combos[key] = mcs
	}
	if mcs.Pos != nil {
		// Should not happen: a pair opens on one bar with mirrored exits, so it
		// closes on one bar too, and manageMirror runs before this. Skipping keeps
		// the one-position-per-stream invariant the original obeys — stacking
		// would give the mirror leverage its source never had — but a skip means
		// the pair has drifted out of step, so it is counted and surfaced on
		// /scalp/health rather than swallowed.
		d.mirrorSkips++
		return
	}

	// Distances are measured from the SHARED entry, then swapped: the mirror's
	// target sits where the original's stop is, and vice versa. That is what
	// makes every outcome of one the negation of the other.
	slDist := math.Abs(orig.Entry - orig.SL)
	tpDist := math.Abs(orig.TP - orig.Entry)

	dir := "SHORT"
	if orig.Dir == "SHORT" {
		dir = "LONG"
	}

	var sl, tp float64
	if dir == "LONG" {
		// Mirror stop where the original targets; mirror target where it stops.
		sl, tp = orig.Entry-tpDist, orig.Entry+slDist
	} else {
		sl, tp = orig.Entry+tpDist, orig.Entry-slDist
	}

	mcs.Pos = &position{
		Dir: dir, Entry: orig.Entry, SL: sl, TP: tp,
		EntryBar: barIdx, EntryTime: bar.OpenTime, Profile: orig.Profile,
	}
	mcs.Pend = nil
	d.mirrorOpens++

	// The mirror is a tradeable stream in its own right, so it must reach the
	// live bridge too.
	//
	// Missing this made six of the eight allow-listed live strategies
	// structurally unable to place an order: they are all ANTI_ mirrors, and the
	// only live hook was on the ORIGINAL's fill path. The bridge was armed, the
	// allow-list resolved, the desk traded — and those six could never have
	// produced a live order however long they ran. Nothing would have errored.
	d.live.onPaperFill(mk, symbol, mcs.Pos)
}

// manageMirror advances a strategy's mirror by one bar, if it holds a position.
//
// Caller holds d.mu.
func (d *desk) manageMirror(strategy, symbol string, bar scalers.Candle, barIdx int64) {
	mk := mirrorOf(strategy)
	if mk == "" {
		return
	}
	if mcs := d.combos[comboKey(mk, symbol)]; mcs != nil && mcs.Pos != nil {
		d.managePosition(mcs, mk, symbol, bar, barIdx)
	}
}

// antiEnabled is resolved once at boot. Reading the environment on every fill
// would let a mid-session change desynchronise a pair — mirroring some fills and
// not others, which is worse than mirroring none.
var antiEnabled = func() bool {
	raw := strings.TrimSpace(os.Getenv("ANTI_STRATEGIES"))
	if raw == "" {
		return true
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return true
	}
	return v
}()

// streamNames is the roster of strategy names the desk keeps accounts for:
// every registry entry, each followed by its mirror.
//
// Mirrors are not registry entries. They have no Evaluate(), place no orders and
// exist only as the inverse of a fill an original already took, so anything that
// enumerates the desk by walking d.entries — the leaderboard, the stream count —
// would report half of it. This is the single place that knows the difference.
//
// Caller holds d.mu (or is running before serve()).
func (d *desk) streamNames() []string {
	out := make([]string, 0, len(d.entries)*2)
	for _, e := range d.entries {
		out = append(out, e.Name)
		if mk := mirrorOf(e.Name); mk != "" && antiEnabled {
			out = append(out, mk)
		}
	}
	return out
}
