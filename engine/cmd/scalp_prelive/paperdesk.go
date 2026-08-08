package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"antigravity-engine/internal/delta"
)

// LIVE ENGINE PAPER DESK
//
// The live-routed strategies, traded on a $100 paper account against real Delta
// prices, with Delta's real taker fee on both legs. Only the money is simulated.
//
// It exists because the two records that mattered disagreed and neither could
// explain the other. The scalp desk's leaderboard said 79.7% wins and +$37
// gross; the same streams with real money returned 33.3% and -$13.91. But the
// scalp desk runs 66,000 streams on a different fee model and different levels,
// so it was never answering "how are the strategies I actually promoted doing,
// on the terms they actually trade on".
//
// This is that place. Same allow-list as the venue, same fee, same $100, same
// prices, same levels. When it disagrees with the live bridge, the difference is
// EXECUTION - slippage, latency, partial fills - and nothing else, because every
// other variable is held equal by construction.
type paperTrade struct {
	Strategy  string    `json:"strategy"`
	Symbol    string    `json:"symbol"`
	Dir       string    `json:"dir"`
	Entry     float64   `json:"entry"`
	Exit      float64   `json:"exit"`
	Stop      float64   `json:"stop"`
	Target    float64   `json:"target"`
	Contracts float64   `json:"contracts"`
	Reason    string    `json:"reason"`
	GrossUSD  float64   `json:"grossUsd"`
	FeesUSD   float64   `json:"feesUsd"`
	NetUSD    float64   `json:"netUsd"`
	OpenedAt  time.Time `json:"openedAt"`
	ClosedAt  time.Time `json:"closedAt"`
	HoldMin   int64     `json:"holdMin"`
}

type paperPos struct {
	Strategy  string    `json:"strategy"`
	Symbol    string    `json:"symbol"`
	Dir       string    `json:"dir"`
	Entry     float64   `json:"entry"`
	Stop      float64   `json:"stop"`
	Target    float64   `json:"target"`
	Contracts float64   `json:"contracts"`
	OpenedAt  time.Time `json:"openedAt"`
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

// paperAccount is one strategy's own $100.
//
// Per strategy, not one shared pot: promotion is a per-strategy decision, and a
// shared balance would let one strategy's win fund another's loss and hide both.
type paperAccount struct {
	Strategy string  `json:"strategy"`
	Equity   float64 `json:"equityUsd"`
	Trades   int     `json:"trades"`
	Wins     int     `json:"wins"`
	GrossUSD float64 `json:"grossUsd"`
	FeesUSD  float64 `json:"feesUsd"`
	NetUSD   float64 `json:"netUsd"`
}

// livePaperDesk mirrors every live-routed signal onto paper.
type livePaperDesk struct {
	mu       sync.Mutex
	accounts map[string]*paperAccount
	open     map[string]*paperPos
	closed   []paperTrade
	started  time.Time
}

// livePaperStartingEquity is what each strategy begins with - the same $100 the
// live desk runs, so a result here transfers without rescaling.
const livePaperStartingEquity = 100.0

// livePaperNotional is the position size: 3x equity, the live risk config's
// aggregate cap, and one strategy holds one position at a time.
const livePaperNotional = livePaperStartingEquity * 3

func newLivePaperDesk() *livePaperDesk {
	return &livePaperDesk{
		accounts: map[string]*paperAccount{},
		open:     map[string]*paperPos{},
		started:  time.Now().UTC(),
	}
}

func paperKey(strategy, symbol string) string { return strategy + "|" + symbol }

// onSignal opens a paper position for a live-routed stream.
//
// Called from the same hook that feeds the real bridge, so the two see an
// identical signal at an identical moment. Anything that reaches one reaches the
// other, which is what makes a later disagreement mean something.
func (d *livePaperDesk) onSignal(strategy, symbol, dir string, entry, stop, target float64, ttl time.Duration) {
	if d == nil || entry <= 0 || stop <= 0 || target <= 0 {
		return
	}
	k := paperKey(strategy, symbol)

	d.mu.Lock()
	defer d.mu.Unlock()
	if _, held := d.open[k]; held {
		// One position per stream, matching the live bridge. Stacking would give
		// the paper desk leverage the real one is not allowed to take.
		return
	}
	if _, ok := d.accounts[strategy]; !ok {
		d.accounts[strategy] = &paperAccount{Strategy: strategy, Equity: livePaperStartingEquity}
	}
	p := &paperPos{
		Strategy: strategy, Symbol: symbol, Dir: dir,
		Entry: entry, Stop: stop, Target: target,
		Contracts: livePaperNotional / entry,
		OpenedAt:  time.Now().UTC(),
	}
	if ttl > 0 {
		p.ExpiresAt = p.OpenedAt.Add(ttl)
	}
	d.open[k] = p
}

// onBar advances every open position against a real Delta bar.
//
// Takes the bar's RANGE, not just its close. Checking the close alone misses
// every intrabar touch: a spike through the stop that closes back inside would
// never trigger here, while the venue's resting bracket would have filled it.
// That difference only ever runs one way - it deletes losses - which is exactly
// the optimism that made the old paper record fail to transfer.
//
// Exit precedence is STOP first. When one bar's range covers both levels there
// is no way to know which was touched first, and assuming the target flatters
// the result.
func (d *livePaperDesk) onBar(symbol string, high, low, close float64) {
	if d == nil || high <= 0 || low <= 0 {
		return
	}
	now := time.Now().UTC()

	d.mu.Lock()
	defer d.mu.Unlock()
	for k, p := range d.open {
		if p.Symbol != symbol {
			continue
		}
		long := p.Dir == "LONG"
		// The extreme that can hurt, and the one that can help.
		adverse, favourable := low, high
		if !long {
			adverse, favourable = high, low
		}

		reason := ""
		exit := close
		switch {
		case long && adverse <= p.Stop, !long && adverse >= p.Stop:
			reason, exit = "SL", p.Stop
		case long && favourable >= p.Target, !long && favourable <= p.Target:
			reason, exit = "TP", p.Target
		case !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt):
			reason = "TTL"
		}
		if reason == "" {
			continue
		}
		d.closeLocked(k, p, exit, reason, now)
	}
}

func (d *livePaperDesk) closeLocked(k string, p *paperPos, exit float64, reason string, now time.Time) {
	dir := 1.0
	if p.Dir == "SHORT" {
		dir = -1.0
	}
	gross := (exit - p.Entry) * dir * p.Contracts
	// Delta taker, BOTH legs - the same rate the live bridge pays. Charged on
	// entry notional and exit notional separately, because the fee is a
	// percentage of each fill and the two prices differ.
	fees := (p.Entry + exit) * p.Contracts * delta.PerpTakerFeeRate
	net := gross - fees

	acct := d.accounts[p.Strategy]
	if acct == nil {
		acct = &paperAccount{Strategy: p.Strategy, Equity: livePaperStartingEquity}
		d.accounts[p.Strategy] = acct
	}
	acct.Trades++
	if net > 0 {
		acct.Wins++
	}
	acct.GrossUSD += gross
	acct.FeesUSD += fees
	acct.NetUSD += net
	acct.Equity = livePaperStartingEquity + acct.NetUSD

	d.closed = append(d.closed, paperTrade{
		Strategy: p.Strategy, Symbol: p.Symbol, Dir: p.Dir,
		Entry: p.Entry, Exit: exit, Stop: p.Stop, Target: p.Target,
		Contracts: p.Contracts, Reason: reason,
		GrossUSD: gross, FeesUSD: fees, NetUSD: net,
		OpenedAt: p.OpenedAt, ClosedAt: now,
		HoldMin: int64(now.Sub(p.OpenedAt).Minutes()),
	})
	// Bounded: this is a live-decision surface, not an archive.
	if len(d.closed) > 2000 {
		d.closed = d.closed[len(d.closed)-2000:]
	}
	delete(d.open, k)

	log.Printf("[LIVE PAPER] %s %s %s CLOSE @ %.6f | %s | gross $%+.4f fees $%.4f net $%+.4f | equity $%.2f",
		p.Strategy, p.Symbol, p.Dir, exit, reason, gross, fees, net, acct.Equity)
}

// reset clears every account and trade.
//
// Used when the rules change underneath the record. A history recorded under two
// different rule sets is worse than no history, because it still looks complete.
func (d *livePaperDesk) reset() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.closed)
	d.accounts = map[string]*paperAccount{}
	d.open = map[string]*paperPos{}
	d.closed = nil
	d.started = time.Now().UTC()
	log.Printf("[LIVE PAPER] reset - %d closed trades and every account cleared", n)
	return n
}

func (d *livePaperDesk) snapshot() map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()

	accts := make([]paperAccount, 0, len(d.accounts))
	for _, a := range d.accounts {
		c := *a
		c.Equity = math.Round(c.Equity*100) / 100
		c.NetUSD = math.Round(c.NetUSD*100) / 100
		c.GrossUSD = math.Round(c.GrossUSD*100) / 100
		c.FeesUSD = math.Round(c.FeesUSD*100) / 100
		accts = append(accts, c)
	}
	sort.Slice(accts, func(i, j int) bool { return accts[i].NetUSD > accts[j].NetUSD })

	open := make([]paperPos, 0, len(d.open))
	for _, p := range d.open {
		open = append(open, *p)
	}
	sort.Slice(open, func(i, j int) bool { return open[i].OpenedAt.Before(open[j].OpenedAt) })

	// Newest first.
	recent := d.closed
	if len(recent) > 200 {
		recent = recent[len(recent)-200:]
	}
	out := make([]paperTrade, len(recent))
	for i := range recent {
		out[len(recent)-1-i] = recent[i]
	}

	return map[string]any{
		"startingEquityUsd": livePaperStartingEquity,
		"notionalUsd":       livePaperNotional,
		"feeRatePerSide":    delta.PerpTakerFeeRate,
		"accounts":          accts,
		"openPositions":     open,
		"recentTrades":      out,
		"uptimeMin":         int64(time.Since(d.started).Minutes()),
	}
}

func (d *livePaperDesk) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(d.snapshot())
}
