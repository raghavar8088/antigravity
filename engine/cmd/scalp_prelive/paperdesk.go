package main

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
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
	Live      bool      `json:"live"`
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
	// Mark is the last real Delta price seen for this symbol. Carried on the
	// position so an open trade can report what it is worth right now — an open
	// position with no P&L tells an operator nothing about whether it is
	// working.
	Mark float64 `json:"mark"`
	// UnrealisedUSD is the result if it closed at Mark, NET of the round-trip
	// taker fee. Gross would flatter it by exactly the amount that has decided
	// every result on this desk.
	UnrealisedUSD float64 `json:"unrealisedUsd"`
	UnrealisedPct float64 `json:"unrealisedPct"`
	// Live is true when this exact stream is ALSO on the venue allow-list.
	//
	// The desk trades the live roster plus candidates, and without this the two
	// are indistinguishable on the page — an operator would read a candidate's
	// result as evidence about real money.
	Live bool `json:"live"`
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
	Strategy string `json:"strategy"`
	// Symbol, because the unit being watched is the STREAM. The same strategy
	// runs on several symbols and they are different bets; collapsing them into
	// one row averages a winner with a loser and hides both.
	Symbol string `json:"symbol"`
	// Live is true when this stream is also on the venue allow-list.
	Live     bool    `json:"live"`
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
	// account is which book this is. Independent $100 books run side by side;
	// a winner in one must never fund a position in another, or the better
	// hypothesis subsidises the worse and hides it.
	account string
	mu      sync.Mutex
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

func newLivePaperDesk(account string) *livePaperDesk {
	d := &livePaperDesk{
		account:  account,
		equity:   livePaperStartingEquity,
		accounts: map[string]*paperAccount{},
		open:     map[string]*paperPos{},
		started:  time.Now().UTC(),
	}
	// Seed every watched stream at zero.
	//
	// Rows were created on first signal, so the board showed only what had
	// already traded — 1 row out of 19 watched, which reads as "nothing is
	// configured" rather than "nothing has fired yet". An operator cannot
	// confirm a promotion took effect from a board that hides idle streams.
	for _, st := range delta.ScalpPaperStreamsFor(account) {
		d.accounts[paperKey(st.Strategy, st.Symbol)] = &paperAccount{
			Strategy: st.Strategy,
			Symbol:   strings.ToUpper(st.Symbol),
			Live:     delta.PerpStreamPermitted(st.Strategy, st.Symbol),
		}
	}
	return d
}

// livePaperBooks are the accounts, keyed by id.
//
// Package level and built once, so a signal reaches every book that watches it
// in the same instant. Feeding them from separate call sites would let the two
// see slightly different prices, and then a difference between them would mean
// nothing.
var livePaperBooks = func() map[string]*livePaperDesk {
	m := map[string]*livePaperDesk{}
	for _, id := range delta.PaperAccountIDs() {
		m[id] = newLivePaperDesk(id)
	}
	return m
}()

// paperOnSignal offers a fill to every book that watches the stream.
func paperOnSignal(strategy, symbol, dir string, entry, stop, target float64, ttl time.Duration) {
	for id, d := range livePaperBooks {
		if delta.PerpStreamPaperPermittedFor(id, strategy, symbol) {
			d.onSignal(strategy, symbol, dir, entry, stop, target, ttl)
		}
	}
}

// paperOnBar advances every book on the same real Delta bar.
func paperOnBar(symbol string, high, low, close float64) {
	for _, d := range livePaperBooks {
		d.onBar(symbol, high, low, close)
	}
}

// paperSnapshotAll returns every book, in display order.
func paperSnapshotAll() map[string]any {
	books := make([]map[string]any, 0, len(livePaperBooks))
	for _, id := range delta.PaperAccountIDs() {
		if d := livePaperBooks[id]; d != nil {
			snap := d.snapshot()
			snap["account"] = id
			books = append(books, snap)
		}
	}
	return map[string]any{"accounts": books}
}

// paperResetAll clears every book.
func paperResetAll() int {
	n := 0
	for _, d := range livePaperBooks {
		n += d.reset()
	}
	return n
}

func paperKey(strategy, symbol string) string { return strategy + "|" + symbol }

// paperUnrealised is what a position is worth at `mark`, net of the round-trip
// taker fee it will pay to close.
//
// Net rather than gross on purpose. Fees are 0.118% of notional round trip and
// most of these strategies target moves smaller than that, so a gross
// unrealised figure would show a winner where the close books a loss — which is
// exactly the discrepancy this desk was built to stop reproducing.
func paperUnrealised(p *paperPos, mark float64) (usd, pct float64) {
	if p == nil || mark <= 0 || p.Entry <= 0 {
		return 0, 0
	}
	dir := 1.0
	if p.Dir == "SHORT" {
		dir = -1.0
	}
	gross := (mark - p.Entry) * dir * p.Contracts
	fees := (p.Entry + mark) * p.Contracts * delta.PerpTakerFeeRate
	usd = gross - fees
	pct = (mark - p.Entry) / p.Entry * dir * 100
	return usd, pct
}

// openUnrealisedLocked is the mark-to-market of every open position, net of
// the fee each will pay to close. Caller holds d.mu.
func (d *livePaperDesk) openUnrealisedLocked() float64 {
	total := 0.0
	for _, p := range d.open {
		total += p.UnrealisedUSD
	}
	return total
}

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
	if _, ok := d.accounts[k]; !ok {
		d.accounts[k] = &paperAccount{
			Strategy: strategy, Symbol: strings.ToUpper(symbol),
			Live: delta.PerpStreamPermitted(strategy, symbol),
		}
	}
	p := &paperPos{
		Live:     delta.PerpStreamPermitted(strategy, symbol),
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
		// Mark first, so a position that survives this bar still reports a
		// current value rather than a stale one from its entry.
		p.Mark = close
		p.UnrealisedUSD, p.UnrealisedPct = paperUnrealised(p, close)

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

	acct := d.accounts[paperKey(p.Strategy, p.Symbol)]
	if acct == nil {
		acct = &paperAccount{Strategy: p.Strategy, Symbol: p.Symbol, Live: p.Live}
		d.accounts[paperKey(p.Strategy, p.Symbol)] = acct
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
		Live:     p.Live,
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

	log.Printf("[LIVE PAPER %s] %s %s %s CLOSE @ %.6f | %s | gross $%+.4f fees $%.4f net $%+.4f | equity $%.2f",
		d.account, p.Strategy, p.Symbol, p.Dir, exit, reason, gross, fees, net, d.equity)
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
	// Re-seed rather than empty: a cleared desk still watches the same streams,
	// and an empty board would again read as "nothing configured".
	d.accounts = map[string]*paperAccount{}
	for _, st := range delta.ScalpPaperStreamsFor(d.account) {
		d.accounts[paperKey(st.Strategy, st.Symbol)] = &paperAccount{
			Strategy: st.Strategy,
			Symbol:   strings.ToUpper(st.Symbol),
			Live:     delta.PerpStreamPermitted(st.Strategy, st.Symbol),
		}
	}
	d.open = map[string]*paperPos{}
	d.closed = nil
	d.started = time.Now().UTC()
	log.Printf("[LIVE PAPER %s] reset - %d closed trades cleared", d.account, n)
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
	sort.Slice(accts, func(i, j int) bool {
		// Traded streams first, then the live-routed, then alphabetical. Sorting
		// purely by net would scatter 19 zero rows through the ranking.
		if (accts[i].Trades > 0) != (accts[j].Trades > 0) {
			return accts[i].Trades > 0
		}
		if accts[i].Trades > 0 && accts[i].NetUSD != accts[j].NetUSD {
			return accts[i].NetUSD > accts[j].NetUSD
		}
		if accts[i].Live != accts[j].Live {
			return accts[i].Live
		}
		if accts[i].Strategy != accts[j].Strategy {
			return accts[i].Strategy < accts[j].Strategy
		}
		return accts[i].Symbol < accts[j].Symbol
	})

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
		// Unrealised across every open position, NET of the exit fee each will
		// pay. Summed here rather than in the UI so the page and the API cannot
		// disagree about what is exposed.
		"openUnrealisedUsd": math.Round(d.openUnrealisedLocked()*100) / 100,
		"maxNotionalUsd":    math.Round(d.equity*livePaperMaxLeverage*100) / 100,
		"maxConcurrent":     livePaperMaxConcurrent,
		"maxLeverage":       livePaperMaxLeverage,
		// The VENUE-side settings, distinct from the size caps above. These are
		// what decide where Delta force-closes a position, and they are reported
		// so the page can show that the paper desk plays by the same margin
		// rules the real account does.
		"productLeverage":    livePaperProductLeverage,
		"liquidationDistPct": math.Round(delta.LiquidationDistanceFraction(livePaperProductLeverage, livePaperMaintenanceMarginPct)*10000) / 100,
		"feeRatePerSide":     delta.PerpTakerFeeRate,
		"accounts":           accts,
		"openPositions":      open,
		"recentTrades":       out,
		"uptimeMin":          int64(time.Since(d.started).Minutes()),
	}
}

func (d *livePaperDesk) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(d.snapshot())
}
