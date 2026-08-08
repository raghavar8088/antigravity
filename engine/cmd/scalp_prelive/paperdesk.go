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

// paperAccount is one strategy's CONTRIBUTION to the shared account.
//
// There is one $100, not $100 each. Per-strategy accounts made the desk look
// like several small portfolios and quietly multiplied the capital: ten
// strategies meant $1,000 deployed while reporting per-strategy returns as if
// each were the whole. The live bridge has one wallet, one aggregate leverage
// cap and one concurrency cap, so the paper mirror must too.
//
// NetUSD is what this strategy added to or took from the shared balance. It is
// still tracked per strategy because promotion is a per-strategy decision — but
// the balance those decisions spend from is single.
type paperAccount struct {
	Strategy string  `json:"strategy"`
	Trades   int     `json:"trades"`
	Wins     int     `json:"wins"`
	GrossUSD float64 `json:"grossUsd"`
	FeesUSD  float64 `json:"feesUsd"`
	NetUSD   float64 `json:"netUsd"`
	// ShareOfEquityPct is this strategy's net as a share of the STARTING
	// balance, so a row can be read against the $100 without implying it owned
	// $100 of its own.
	ShareOfEquityPct float64 `json:"shareOfEquityPct"`
}

// livePaperDesk mirrors every live-routed signal onto paper.
type livePaperDesk struct {
	mu sync.Mutex
	// equity is THE account. One balance, shared by every strategy, exactly as
	// the live bridge shares one Delta wallet.
	equity   float64
	accounts map[string]*paperAccount
	open     map[string]*paperPos
	closed   []paperTrade
	started  time.Time
}

// livePaperStartingEquity is the whole account - the same $100 the live desk
// runs, so a result here transfers without rescaling.
const livePaperStartingEquity = 100.0

// livePaperMaxLeverage is the AGGREGATE cap across all open positions, matching
// PerpRiskConfig.MaxAggregateLeverage on the live bridge.
const livePaperMaxLeverage = 3.0

// livePaperMaxConcurrent matches the bridge's MaxConcurrentPositions. Three
// positions sharing a 3x cap is ~1x notional each — which is what the live desk
// actually deploys, and materially less than the 3x a per-strategy account
// implied.
const livePaperMaxConcurrent = 3

// livePaperProductLeverage is the leverage SET ON THE PRODUCT at Delta, which
// decides how much margin the venue holds and therefore how far away it will
// force-close the position. Matches delta.PerpLeverage.
//
// Distinct from livePaperMaxLeverage, which limits SIZE. Ten times is about
// where the liquidation price sits; three times is about how much is bought.
const livePaperProductLeverage = delta.PerpLeverage

// livePaperMaintenanceMarginPct is Delta's maintenance requirement, the other
// half of the liquidation distance. At 10x this leaves ~9.5%.
const livePaperMaintenanceMarginPct = 0.5

func newLivePaperDesk() *livePaperDesk {
	return &livePaperDesk{
		equity:   livePaperStartingEquity,
		accounts: map[string]*paperAccount{},
		open:     map[string]*paperPos{},
		started:  time.Now().UTC(),
	}
}

func paperKey(strategy, symbol string) string { return strategy + "|" + symbol }

// openNotionalLocked is the capital already deployed. Caller holds d.mu.
func (d *livePaperDesk) openNotionalLocked() float64 {
	total := 0.0
	for _, p := range d.open {
		total += p.Entry * p.Contracts
	}
	return total
}

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
	// The bridge refuses a trade whose stop sits BEYOND the liquidation price -
	// the venue would close the position before the strategy's own risk
	// management could act. Without the same refusal here, the paper record
	// would contain trades the real desk declines, and the two would disagree
	// for a reason that has nothing to do with execution.
	if !delta.StopIsReachable(entry, stop, livePaperProductLeverage, livePaperMaintenanceMarginPct) {
		return
	}

	// Concurrency cap, matching the bridge. Refusing here is the same refusal
	// the live desk makes; without it the paper record would contain trades the
	// real desk would never have taken.
	if len(d.open) >= livePaperMaxConcurrent {
		return
	}
	// Aggregate leverage cap across everything already open. One wallet means
	// one budget: a fourth idea cannot be funded by pretending the first three
	// were free.
	budget := d.equity*livePaperMaxLeverage - d.openNotionalLocked()
	perPosition := d.equity * livePaperMaxLeverage / livePaperMaxConcurrent
	notional := math.Min(budget, perPosition)
	if notional <= 0 {
		return
	}
	if _, ok := d.accounts[strategy]; !ok {
		d.accounts[strategy] = &paperAccount{Strategy: strategy}
	}
	p := &paperPos{
		Strategy: strategy, Symbol: symbol, Dir: dir,
		Entry: entry, Stop: stop, Target: target,
		Contracts: notional / entry,
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

		// The venue force-closes before anything the strategy wants, so this is
		// checked FIRST. It should never fire while stops are 0.7% and
		// liquidation is ~9.5% away — but "should never" is exactly the
		// assumption that let two real positions get liquidated inside their own
		// stops, and a paper desk that cannot represent the event cannot warn
		// about it either.
		liqFrac := delta.LiquidationDistanceFraction(livePaperProductLeverage, livePaperMaintenanceMarginPct)
		liq := p.Entry * (1 - liqFrac)
		if !long {
			liq = p.Entry * (1 + liqFrac)
		}

		reason := ""
		exit := close
		switch {
		case long && adverse <= liq, !long && adverse >= liq:
			reason, exit = delta.ExitReasonLiquidated, liq
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
		acct = &paperAccount{Strategy: p.Strategy}
		d.accounts[p.Strategy] = acct
	}
	acct.Trades++
	if net > 0 {
		acct.Wins++
	}
	acct.GrossUSD += gross
	acct.FeesUSD += fees
	acct.NetUSD += net

	// The SHARED balance moves. Every strategy spends from and returns to this
	// one number, so a win here really does fund the next position elsewhere —
	// which is what the live wallet does.
	d.equity += net

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
		p.Strategy, p.Symbol, p.Dir, exit, reason, gross, fees, net, d.equity)
}

// reset clears every account and trade.
//
// Used when the rules change underneath the record. A history recorded under two
// different rule sets is worse than no history, because it still looks complete.
func (d *livePaperDesk) reset() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.closed)
	d.equity = livePaperStartingEquity
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
		c.NetUSD = math.Round(c.NetUSD*100) / 100
		c.GrossUSD = math.Round(c.GrossUSD*100) / 100
		c.FeesUSD = math.Round(c.FeesUSD*100) / 100
		c.ShareOfEquityPct = math.Round(c.NetUSD/livePaperStartingEquity*10000) / 100
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
		// ONE balance for the whole desk, not one per strategy.
		"equityUsd":       math.Round(d.equity*100) / 100,
		"netUsd":          math.Round((d.equity-livePaperStartingEquity)*100) / 100,
		"roiPct":          math.Round((d.equity-livePaperStartingEquity)/livePaperStartingEquity*10000) / 100,
		"openNotionalUsd": math.Round(d.openNotionalLocked()*100) / 100,
		"maxNotionalUsd":  math.Round(d.equity*livePaperMaxLeverage*100) / 100,
		"maxConcurrent":   livePaperMaxConcurrent,
		"maxLeverage":     livePaperMaxLeverage,
		"feeRatePerSide":  delta.PerpTakerFeeRate,
		"accounts":        accts,
		"openPositions":   open,
		"recentTrades":    out,
		"uptimeMin":       int64(time.Since(d.started).Minutes()),
	}
}

func (d *livePaperDesk) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(d.snapshot())
}
