package delta

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Live perpetual execution for scalp strategies.
//
// The paper scalp desk decides everything — which strategy, which symbol, which
// direction, where the entry limit, stop and target sit. This bridge does one
// job: when an allow-listed paper stream opens a position, place the same trade
// with real money on Delta, then own it until it is closed.
//
// # Custody is the point
//
// The options bridge learned this the expensive way: a live position whose exits
// are managed by the paper desk is not managed at all. The paper desk closes a
// paper position; the real one keeps running. So this bridge holds its own
// stop and target for every live position and closes them itself, on its own
// monitor loop, whether or not the paper side agrees.
//
// # Everything fails closed
//
// Disarmed by default. An unset allow-list permits nothing. A stale product
// registry refuses to size. An unreachable venue disarms rather than assumes.
// The account is $100 and the whole point of it is to find out whether a scalp
// strategy survives a real book — losing it to an infrastructure failure answers
// nothing.

// PerpLiveTrade is one real position this bridge opened and owns.
type PerpLiveTrade struct {
	ID        string    `json:"id"`
	Strategy  string    `json:"strategy"`
	Symbol    string    `json:"symbol"`
	ProductID int       `json:"productId"`
	Side      OrderSide `json:"side"`
	Contracts int       `json:"contracts"`

	// The exit plan this bridge enforces. Copied from the paper signal at open
	// and never re-read from it: the paper desk may close its own position for
	// reasons that do not apply to a real fill.
	StopPrice   float64 `json:"stopPrice"`
	TargetPrice float64 `json:"targetPrice"`
	// ExpiresAt is the TIME STOP, and it is not a detail.
	//
	// Over 500 measured paper trades the scalp desk exited 456 of them (91.2%)
	// on time, 30 on the stop and 14 on the target. A live bridge with only a
	// stop and a target therefore reproduces under 9% of the desk's behaviour
	// and holds the other 91% indefinitely — positions the paper record shows as
	// closed hours earlier, still open and still funded.
	ExpiresAt time.Time `json:"expiresAt"`

	EntryPrice float64 `json:"entryPrice"`
	// MarkPrice and UnrealizedPnL are refreshed by the custody loop, which
	// already reads the venue's marks to decide exits. Publishing them costs
	// nothing and is the difference between a page that shows a real number and
	// one that shows a placeholder.
	MarkPrice     float64   `json:"markPrice"`
	UnrealizedPnL float64   `json:"unrealizedPnl"`
	NotionalUSD   float64   `json:"notionalUsd"`
	RiskUSD       float64   `json:"riskUsd"`
	OpenedAt      time.Time `json:"openedAt"`
	OrderID       string    `json:"orderId"`

	ClosedAt   *time.Time `json:"closedAt,omitempty"`
	ExitPrice  float64    `json:"exitPrice,omitempty"`
	ExitReason string     `json:"exitReason,omitempty"`
	// Gross, fees and net are reported SEPARATELY. Collapsing them into one
	// number is how a desk looks profitable gross while shrinking net — and
	// fees here are comparable to the entire per-trade edge.
	GrossPnL    float64 `json:"grossPnl,omitempty"`
	FeesUSD     float64 `json:"feesUsd,omitempty"`
	RealisedPnL float64 `json:"realisedPnl,omitempty"`
	Status      string  `json:"status"` // OPEN | CLOSED | REJECTED
	Failure     string  `json:"failure,omitempty"`
}

func (t PerpLiveTrade) long() bool { return t.Side == "buy" }

// PerpBridge places and owns real perpetual positions for scalp signals.
type PerpBridge struct {
	client *Client
	reg    *PerpRegistry
	allow  *PerpAllowList

	mu        sync.RWMutex
	cfg       PerpRiskConfig
	open      map[string]*PerpLiveTrade // key: strategy|symbol
	history   []PerpLiveTrade
	lastError string

	armed     atomic.Bool
	killCheck func() bool

	submitted atomic.Int64
	rejected  atomic.Int64
	closes    atomic.Int64

	// stateDir is where the open book is persisted. Empty = memory only, which
	// strands every open position on restart. See perp_persistence.go.
	stateDir string
}

// NewPerpBridge builds a bridge. It starts DISARMED and permits nothing until
// both an allow-list and an arm are provided.
func NewPerpBridge(client *Client, reg *PerpRegistry, equityUSD float64) *PerpBridge {
	return &PerpBridge{
		client: client,
		reg:    reg,
		allow:  NewPerpAllowList(),
		cfg:    DefaultPerpRiskConfig(equityUSD),
		open:   map[string]*PerpLiveTrade{},
	}
}

// AllowList exposes the gate so the operator can set it.
func (b *PerpBridge) AllowList() *PerpAllowList { return b.allow }

// SetKillSwitch wires a predicate that, while true, blocks every new order.
func (b *PerpBridge) SetKillSwitch(fn func() bool) {
	b.mu.Lock()
	b.killCheck = fn
	b.mu.Unlock()
}

// SetEquity re-bases the account size. Sizing is derived from it, so a stale
// value silently mis-sizes every subsequent order.
func (b *PerpBridge) SetEquity(usd float64) {
	b.mu.Lock()
	b.cfg = DefaultPerpRiskConfig(usd)
	b.mu.Unlock()
	log.Printf("[PERP LIVE] account equity set to $%.2f (risk $%.2f/trade, max %.0fx, %d concurrent)",
		usd, usd*b.cfg.RiskPerTradeFraction, b.cfg.MaxLeverage, b.cfg.MaxConcurrentPositions)
}

// Config returns the current risk posture.
func (b *PerpBridge) Config() PerpRiskConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cfg
}

// Arm enables live order placement. Arming is always an explicit human action;
// nothing in this package arms itself.
func (b *PerpBridge) Arm(actor, reason string) error {
	if b.client == nil {
		return fmt.Errorf("perp bridge: no Delta client — cannot arm")
	}
	if b.allow.Count() == 0 {
		return fmt.Errorf("perp bridge: allow-list is empty — arming would permit nothing")
	}
	if b.reg.Count() == 0 {
		return fmt.Errorf("perp bridge: product registry is empty — cannot size an order")
	}
	b.armed.Store(true)
	log.Printf("[PERP LIVE] ✅ ARMED by %s (%s) — %d strategies, $%.2f equity",
		actor, reason, b.allow.Count(), b.Config().EquityUSD)
	return nil
}

// Disarm stops new orders. Open positions are still monitored to their exits:
// abandoning a funded position is worse than holding it.
func (b *PerpBridge) Disarm(actor, reason string) {
	if b.armed.Swap(false) {
		log.Printf("[PERP LIVE] ⏸️  DISARMED by %s (%s) — open positions remain under custody", actor, reason)
	}
}

// IsArmed reports whether new orders may be placed.
func (b *PerpBridge) IsArmed() bool { return b.armed.Load() }

func perpKey(strategy, symbol string) string { return strategy + "|" + symbol }

// OnPaperOpen mirrors a paper scalp entry into a real order.
//
// Returns nil when the signal was deliberately not traded (not allow-listed,
// disarmed, capped) — those are the normal case, not errors.
func (b *PerpBridge) OnPaperOpen(ctx context.Context, strategy, symbol string, long bool, entry, stop, target float64, ttl time.Duration) *PerpLiveTrade {
	if !b.IsArmed() || b.client == nil {
		return nil
	}
	if !b.allow.Allowed(strategy, symbol) {
		return nil
	}
	b.mu.RLock()
	kill := b.killCheck
	cfg := b.cfg
	_, alreadyOpen := b.open[perpKey(strategy, symbol)]
	openCount := len(b.open)
	// Notional already committed. The aggregate cap is measured against this, so
	// a book at its ceiling stops adding rather than stacking per-order caps.
	openNotional := 0.0
	for _, t := range b.open {
		openNotional += t.NotionalUSD
	}
	b.mu.RUnlock()

	if alreadyOpen {
		// One live position per stream, exactly as the paper desk holds one.
		return nil
	}
	if kill != nil && kill() {
		log.Printf("[PERP LIVE] kill switch active — refusing %s %s", strategy, symbol)
		return nil
	}

	plan, err := PlanPerpOrder(b.reg, cfg, symbol, long, entry, stop, target, openCount, openNotional)
	if err != nil {
		// Capacity and sub-contract refusals are routine; log them quietly once
		// rather than treating a normal skip as a failure.
		b.noteError(err.Error())
		return nil
	}

	res, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: plan.ProductID,
		Size:      plan.Contracts,
		Side:      plan.Side,
		OrderType: TypeMarket,
		// Explicit leverage so the margin this position consumes is predictable
		// rather than whatever the account happens to be set to.
		Leverage: cfg.LeverageForOrder,
		// Market entry, deliberately. The paper desk models a post-only maker
		// fill and only counts trades that actually filled; a resting limit here
		// would leave the live account holding orders the paper desk has already
		// considered filled, and the two records would diverge silently.
		TimeInForce:          "ioc",
		CancelOrdersAccepted: "true",
	})
	if err != nil {
		b.rejected.Add(1)
		b.noteError(err.Error())
		log.Printf("[PERP LIVE] ❌ REJECTED %s %s %d contracts: %v", strategy, plan.Symbol, plan.Contracts, err)
		return nil
	}

	fill := res.Price
	if fill <= 0 {
		fill = plan.LimitPrice
	}
	t := &PerpLiveTrade{
		ID:          fmt.Sprintf("perp-%d-%s", time.Now().UnixNano(), plan.Symbol),
		Strategy:    strategy,
		Symbol:      plan.Symbol,
		ProductID:   plan.ProductID,
		Side:        plan.Side,
		Contracts:   plan.Contracts,
		StopPrice:   plan.StopPrice,
		TargetPrice: plan.TargetPrice,
		EntryPrice:  fill,
		NotionalUSD: plan.NotionalUSD,
		RiskUSD:     plan.RiskUSD,
		OpenedAt:    time.Now().UTC(),
		OrderID:     res.OrderID,
		Status:      "OPEN",
	}
	if ttl > 0 {
		t.ExpiresAt = t.OpenedAt.Add(ttl)
	}

	// The entry fee is charged now and is part of this trade's cost whatever
	// happens next. Recording it at open means an UNRECONCILED close still
	// reports the fee that was definitely paid.
	if p, ok := b.reg.Lookup(plan.Symbol); ok {
		t.FeesUSD = PerpFeeUSD(fill, plan.Contracts, p.ContractValue)
	}

	b.mu.Lock()
	b.open[perpKey(strategy, symbol)] = t
	// Persist BEFORE returning: a crash between the fill and the next write
	// leaves the position funded and unknown to the next process.
	b.persistLocked()
	b.mu.Unlock()
	b.submitted.Add(1)

	ttlLabel := "none"
	if !t.ExpiresAt.IsZero() {
		ttlLabel = ttl.String()
	}
	log.Printf("[PERP LIVE] ✅ %s %s %s %d contracts @ %.6f | stop %.6f target %.6f ttl %s | $%.2f notional, $%.2f at risk",
		plan.Side, plan.Symbol, strategy, plan.Contracts, fill, plan.StopPrice, plan.TargetPrice,
		ttlLabel, plan.NotionalUSD, plan.RiskUSD)
	return t
}

// Monitor owns every open position until it exits. Blocks until ctx is done.
//
// This is the custody loop. It runs whether or not the bridge is armed, because
// disarming must not orphan a funded position.
func (b *PerpBridge) Monitor(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 15 * time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			b.checkExits(ctx)
		}
	}
}

// checkExits marks every open position against the venue and closes those that
// have reached their stop or target.
func (b *PerpBridge) checkExits(ctx context.Context) {
	if b.client == nil {
		return
	}
	positions, err := b.client.GetPositions(ctx)
	if err != nil {
		// A venue we cannot see is a venue we cannot manage risk on. Disarm so
		// no NEW exposure is added while blind; existing positions stay under
		// custody and will be re-checked when the feed returns.
		b.noteError(err.Error())
		if b.IsArmed() {
			b.Disarm("auto", "delta positions unreachable: "+err.Error())
		}
		return
	}

	marks := make(map[int]float64, len(positions))
	live := make(map[int]bool, len(positions))
	for _, p := range positions {
		if p.Size != 0 {
			marks[p.ProductID] = p.MarkPrice
			live[p.ProductID] = true
		}
	}

	b.mu.RLock()
	snapshot := make([]*PerpLiveTrade, 0, len(b.open))
	for _, t := range b.open {
		snapshot = append(snapshot, t)
	}
	b.mu.RUnlock()

	for _, t := range snapshot {
		mark, ok := marks[t.ProductID]
		if ok && mark > 0 {
			// Mark to the venue's own price before deciding anything. The same
			// figure drives the exit test below, so what the page shows is
			// exactly what custody is acting on.
			b.markLive(t, mark)
		}
		if !ok || mark <= 0 {
			// The venue no longer reports this position. It was closed by
			// something other than this bridge — liquidation, a manual close, or
			// an exchange action. Reconcile rather than keep managing a ghost.
			b.finish(t, t.EntryPrice, "CLOSED_EXTERNALLY")
			continue
		}
		if reason := perpExitReason(t, mark, time.Now().UTC()); reason != "" {
			if err := b.closePosition(ctx, t, mark, reason); err != nil {
				log.Printf("[PERP LIVE] close failed for %s %s: %v", t.Strategy, t.Symbol, err)
			}
		}
	}
}

// markLive refreshes a position's mark and unrealised P&L from the venue.
func (b *PerpBridge) markLive(t *PerpLiveTrade, mark float64) {
	cv := 0.0
	if p, ok := b.reg.Lookup(t.Symbol); ok {
		cv = p.ContractValue
	}
	dir := 1.0
	if !t.long() {
		dir = -1.0
	}
	b.mu.Lock()
	t.MarkPrice = mark
	// Contract value, not a BTC constant: on ADAUSD (1.0) a BTC assumption would
	// understate the P&L a thousandfold, the same trap as sizing.
	t.UnrealizedPnL = (mark - t.EntryPrice) * dir * float64(t.Contracts) * cv
	b.mu.Unlock()
}

// perpExitReason decides whether a position has reached an exit.
//
// Stop is checked BEFORE target, the same conservative precedence the paper desk
// uses: when a single mark could satisfy both, assume the adverse one happened.
func perpExitReason(t *PerpLiveTrade, mark float64, now time.Time) string {
	// Stop and target first: a position that reached a real level exited for
	// that reason, not for running out of time on the same tick.
	if r := perpPriceExit(t, mark); r != "" {
		return r
	}
	// The time stop. Checked last so it never masks a price exit, but checked —
	// it is how 9 in 10 of these trades end on the paper desk.
	if !t.ExpiresAt.IsZero() && !now.Before(t.ExpiresAt) {
		return "TTL"
	}
	return ""
}

// perpPriceExit reports a stop or target hit at the given mark.
func perpPriceExit(t *PerpLiveTrade, mark float64) string {
	if t.long() {
		switch {
		case t.StopPrice > 0 && mark <= t.StopPrice:
			return "SL"
		case t.TargetPrice > 0 && mark >= t.TargetPrice:
			return "TP"
		}
		return ""
	}
	switch {
	case t.StopPrice > 0 && mark >= t.StopPrice:
		return "SL"
	case t.TargetPrice > 0 && mark <= t.TargetPrice:
		return "TP"
	}
	return ""
}

// closePosition sends a reduce-only market order in the opposite direction.
func (b *PerpBridge) closePosition(ctx context.Context, t *PerpLiveTrade, mark float64, reason string) error {
	side := OrderSide("sell")
	if !t.long() {
		side = "buy"
	}
	res, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: t.ProductID,
		Size:      t.Contracts,
		Side:      side,
		OrderType: TypeMarket,
		// Reduce-only so a close can never accidentally open the opposite
		// position — which is what happens if the size is stale and larger than
		// what is actually held.
		ReduceOnly:           true,
		TimeInForce:          "ioc",
		CancelOrdersAccepted: "true",
	})
	if err != nil {
		b.noteError(err.Error())
		return err
	}

	// Book the FILL, not the mark that triggered this.
	//
	// The order is market/ioc and fills wherever the book is; `mark` is only the
	// price that made the exit decision. Booking the mark recorded a
	// 1,099-contract ADAUSD close at 0.17263 that actually filled at 0.17290 —
	// turning a booked +$0.0751 into a real -$0.1395. A sign flip, not a
	// rounding error.
	fill := res.Price
	if fill <= 0 {
		// The venue accepted the order but did not report a price. Marking the
		// trade rather than silently substituting `mark` keeps the uncertainty
		// visible; the reconciler will correct it against the venue.
		fill = mark
		reason = reason + "_" + ExitReasonUnreconciled
	}
	b.finish(t, fill, reason)
	return nil
}

// finish books a closed position.
func (b *PerpBridge) finish(t *PerpLiveTrade, exit float64, reason string) {
	now := time.Now().UTC()
	dir := 1.0
	if !t.long() {
		dir = -1.0
	}
	cv := 0.0
	if p, ok := b.reg.Lookup(t.Symbol); ok {
		cv = p.ContractValue
	}
	// Gross, both fee legs, and net — computed from the prices actually filled.
	// The previous version reported gross and called it realised, hiding
	// $1.9086 of fees in a single day on a $110 account.
	res := ComputePerpResult(t.EntryPrice, exit, t.Contracts, cv, t.long())
	pnl := res.Net
	_ = dir

	b.mu.Lock()
	delete(b.open, perpKey(t.Strategy, t.Symbol))
	t.ClosedAt = &now
	t.ExitPrice = exit
	t.ExitReason = reason
	t.GrossPnL = res.Gross
	t.FeesUSD = res.EntryFee + res.ExitFee
	t.RealisedPnL = pnl
	t.Status = "CLOSED"
	b.history = append(b.history, *t)
	if len(b.history) > 500 {
		b.history = b.history[len(b.history)-500:]
	}
	b.persistLocked()
	b.mu.Unlock()
	b.closes.Add(1)

	symbol := "✅"
	if pnl < 0 {
		symbol = "❌"
	}
	log.Printf("[PERP LIVE] %s CLOSE %s %s %s @ %.6f | %s | gross $%+.4f fees $%.4f net $%+.4f",
		symbol, t.Side, t.Symbol, t.Strategy, exit, reason, res.Gross, res.EntryFee+res.ExitFee, pnl)
}

// CloseAll flattens every position this bridge owns.
func (b *PerpBridge) CloseAll(ctx context.Context) (int, error) {
	b.Disarm("auto", "close-all requested")
	b.mu.RLock()
	snapshot := make([]*PerpLiveTrade, 0, len(b.open))
	for _, t := range b.open {
		snapshot = append(snapshot, t)
	}
	b.mu.RUnlock()

	closed := 0
	var firstErr error
	for _, t := range snapshot {
		if err := b.closePosition(ctx, t, t.EntryPrice, "CLOSE_ALL"); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		closed++
	}
	return closed, firstErr
}

// Reconcile compares this bridge's view against the venue's.
//
// A mismatch means one of them is wrong about real money, so it is surfaced
// rather than silently corrected.
func (b *PerpBridge) Reconcile(ctx context.Context) (engineOpen, venueOpen int, err error) {
	b.mu.RLock()
	engineOpen = len(b.open)
	mine := make(map[int]bool, len(b.open))
	for _, t := range b.open {
		mine[t.ProductID] = true
	}
	b.mu.RUnlock()

	if b.client == nil {
		return engineOpen, 0, fmt.Errorf("perp bridge: no Delta client")
	}
	positions, err := b.client.GetPositions(ctx)
	if err != nil {
		return engineOpen, 0, err
	}
	for _, p := range positions {
		if p.Size != 0 && mine[p.ProductID] {
			venueOpen++
		}
	}
	if engineOpen != venueOpen {
		log.Printf("[PERP LIVE] ⚠️  reconciliation mismatch: bridge holds %d, Delta reports %d",
			engineOpen, venueOpen)
	}
	return engineOpen, venueOpen, nil
}

func (b *PerpBridge) noteError(msg string) {
	b.mu.Lock()
	b.lastError = msg
	b.mu.Unlock()
}

// PerpBridgeStats is the operator view.
type PerpBridgeStats struct {
	Armed         bool            `json:"armed"`
	EquityUSD     float64         `json:"equityUsd"`
	RiskPerTrade  float64         `json:"riskPerTradeUsd"`
	MaxLeverage   float64         `json:"maxLeverage"`
	MaxConcurrent int             `json:"maxConcurrent"`
	Strategies    []string        `json:"strategies"`
	OpenPositions []PerpLiveTrade `json:"openPositions"`
	Submitted     int64           `json:"submitted"`
	Rejected      int64           `json:"rejected"`
	Closed        int64           `json:"closed"`
	RealisedPnL   float64         `json:"realisedPnlUsd"`
	LastError     string          `json:"lastError,omitempty"`
	ProductsKnown int             `json:"productsKnown"`
}

// Stats snapshots the bridge.
func (b *PerpBridge) Stats() PerpBridgeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	open := make([]PerpLiveTrade, 0, len(b.open))
	for _, t := range b.open {
		open = append(open, *t)
	}
	sort.Slice(open, func(i, j int) bool { return open[i].OpenedAt.Before(open[j].OpenedAt) })

	pnl := 0.0
	for _, t := range b.history {
		pnl += t.RealisedPnL
	}
	strategies := b.allow.Strategies()
	sort.Strings(strategies)

	return PerpBridgeStats{
		Armed:         b.armed.Load(),
		EquityUSD:     b.cfg.EquityUSD,
		RiskPerTrade:  b.cfg.EquityUSD * b.cfg.RiskPerTradeFraction,
		MaxLeverage:   b.cfg.MaxLeverage,
		MaxConcurrent: b.cfg.MaxConcurrentPositions,
		Strategies:    strategies,
		OpenPositions: open,
		Submitted:     b.submitted.Load(),
		Rejected:      b.rejected.Load(),
		Closed:        b.closes.Load(),
		RealisedPnL:   pnl,
		LastError:     b.lastError,
		ProductsKnown: b.reg.Count(),
	}
}

// History returns closed live trades, newest last.
func (b *PerpBridge) History() []PerpLiveTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]PerpLiveTrade, len(b.history))
	copy(out, b.history)
	return out
}
