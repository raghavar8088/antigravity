package delta

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
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
	// BracketsAttached is true when Delta holds a resting stop and target for
	// this position.
	//
	// When it does, the 15-second monitor must NOT close on price. Both were
	// running and the monitor kept winning: it polls MARK every 15s and closes
	// at market, while the bracket waits for LAST TRADED to cross its trigger.
	// Measured on TSTUSD — bracket limit 0.016308, monitor exited at 0.016330,
	// a price a limit order could not have filled. The mechanism built to stop
	// the overshoot was being front-run by the one causing it.
	BracketsAttached bool `json:"bracketsAttached"`
	// IfTargetUSD and IfStopUSD are what this position pays or costs if it
	// reaches its target or its stop, NET of the round-trip taker fee.
	//
	// Net rather than gross, for the same reason every other figure on this desk
	// is: a target 0.3% away on a 0.118% round trip keeps less than half of what
	// the gross move suggests, and an operator reading the gross would think the
	// trade was worth twice what it is.
	IfTargetUSD float64 `json:"ifTargetUsd"`
	IfStopUSD   float64 `json:"ifStopUsd"`

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
	GrossPnL float64 `json:"grossPnl,omitempty"`
	FeesUSD  float64 `json:"feesUsd,omitempty"`

	// StopOvershoot is realised risk divided by planned risk on a stop-out.
	//
	// 1.00 means the stop closed exactly where it was placed. Above 1.00 is the
	// stop costing more than it was meant to, which is the failure this desk
	// has now hit five separate ways: params on the wrong endpoint, off-grid
	// prices, the monitor front-running the venue, a limit that could not fill,
	// and a limit so wide it filled at the worst permitted price.
	//
	// Recorded because each of those fixes looked complete when shipped. A
	// single clean stop-out cannot distinguish a fix from a quiet market, so
	// the ratio is stored per trade and can be read across many.
	StopOvershoot float64 `json:"stopOvershoot,omitempty"`
	RealisedPnL   float64 `json:"realisedPnl,omitempty"`
	Status        string  `json:"status"` // OPEN | CLOSED | REJECTED
	Failure       string  `json:"failure,omitempty"`
}

func (t PerpLiveTrade) long() bool { return t.Side == "buy" }

// PerpBridge places and owns real perpetual positions for scalp signals.
type PerpBridge struct {
	client *Client
	reg    *PerpRegistry
	allow  *PerpAllowList

	mu      sync.RWMutex
	cfg     PerpRiskConfig
	open    map[string]*PerpLiveTrade // key: strategy|symbol
	history []PerpLiveTrade

	// strategyOff names strategies the owner has switched off from the desk.
	//
	// Stored as the set of DISABLED names, not enabled ones, so the default for
	// an unknown name is "trades". A strategy added to the roster later must
	// behave as the roster says rather than silently sitting out because it was
	// missing from a saved enable-list — a switch whose failure mode is a
	// desk that quietly stops trading is worse than one that keeps going.
	strategyOff map[string]bool

	// gridBlocked records streams the pre-trade grid gate has refused, keyed by
	// stream. Kept so the board can show a stream that is switched ON and still
	// unable to trade — otherwise 19 of 31 streams sit at "on", never fill, and
	// look like strategies that simply are not signalling.
	gridBlocked map[string]gridBlock

	// vol measures per-symbol noise so the stop can be set outside it. Nil
	// leaves every strategy's own stop untouched.
	vol       *VolatilityTracker
	lastError string

	armed atomic.Bool
	// fundingUSD is the venue's settled funding total, refreshed from the
	// ledger by the custody loop. atomic.Value so Stats() need not take a lock
	// against the poller.
	fundingUSD atomic.Value
	killCheck  func() bool

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
	b := &PerpBridge{
		client: client,
		reg:    reg,
		allow:  NewPerpAllowList(),
		cfg:    DefaultPerpRiskConfig(equityUSD),
		open:   map[string]*PerpLiveTrade{},
	}
	// Seeded so Stats() never type-asserts a nil before the first refresh — an
	// unmeasured funding total must read as 0 through a defined path, not panic.
	b.fundingUSD.Store(0.0)
	return b
}

// RefreshFunding reads settled funding from the venue ledger.
//
// Called by the custody loop, which runs whether or not the bridge is armed:
// funding accrues on any position that exists, and a disarmed bridge can still
// be holding one.
func (b *PerpBridge) RefreshFunding(ctx context.Context) error {
	if b.client == nil {
		return fmt.Errorf("perp bridge: no Delta client")
	}
	entries, err := b.client.GetLedger(ctx, 200)
	if err != nil {
		return err
	}
	total := 0.0
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Type), "funding") {
			total += e.Amount
		}
	}
	b.fundingUSD.Store(total)
	return nil
}

// perpMaintenanceMarginPct is Delta's maintenance requirement on these
// perpetuals, as a percentage of notional. Read from the product definition
// (ADAUSD: maintenance_margin 0.5), and the number that put the liquidation
// price 0.5% from entry at the account's default leverage.
const perpMaintenanceMarginPct = 0.5

// EnsureLeverage sets the account's per-product leverage for every symbol this
// bridge may trade.
//
// This must happen BEFORE arming. Delta ignores the leverage field on an order
// and uses the account setting, which ships at 100x — putting the liquidation
// price inside every one of this desk's stops. Failing to set it is not a
// degraded mode; it is a desk whose risk management the venue overrides.
func (b *PerpBridge) EnsureLeverage(ctx context.Context, symbols []string) error {
	if b.client == nil {
		return fmt.Errorf("perp bridge: no client")
	}
	var firstErr error
	for _, sym := range symbols {
		p, ok := b.reg.Lookup(sym)
		if !ok {
			continue
		}
		if err := b.client.SetProductLeverage(ctx, p.ProductID, PerpLeverage); err != nil {
			log.Printf("[PERP LIVE] ⚠️  could not set %s leverage to %dx: %v — its stops may sit beyond the liquidation distance",
				sym, PerpLeverage, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		// Report THIS contract's liquidation distance. The old line printed a
		// constant 9.5% for every symbol, which was right for 13 of 92 and
		// wrong-but-reassuring for the rest.
		symMaint := perpMaintenanceMarginPct
		if pr, ok := b.reg.Lookup(sym); ok && pr.MaintenanceMarginPct > 0 {
			symMaint = pr.MaintenanceMarginPct
		}
		log.Printf("[PERP LIVE] %s leverage set to %dx (maint %.2f%% — liquidation ~%.1f%% adverse; stops are 0.35-1%%)",
			sym, PerpLeverage, symMaint, LiquidationDistanceFraction(PerpLeverage, symMaint)*100)
	}
	return firstErr
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
	// Positions already held in THIS instrument. Counted here under the same
	// lock as the rest of the snapshot so the check cannot race a concurrent
	// open — two signals arriving in the same tick is exactly the case this
	// guards.
	sameSymbol := 0
	for _, t := range b.open {
		openNotional += t.NotionalUSD
		if strings.EqualFold(t.Symbol, symbol) {
			sameSymbol++
		}
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
	if !b.StrategyEnabled(strategy, symbol) {
		// Switched off by the owner. Checked here, on the engine's open path,
		// because a toggle enforced only in the UI is decoration: the signal
		// loop does not read the browser.
		return nil
	}
	if cap := cfg.MaxPositionsPerSymbol; cap > 0 && sameSymbol >= cap {
		// Refused loudly. A silently skipped signal here looks identical to a
		// strategy that never fired, and the whole point of this cap is that
		// concentration was invisible until it had already cost three stop-outs.
		log.Printf("[PERP LIVE] %s %s: refused — %d position(s) already open on this symbol (cap %d)",
			strategy, symbol, sameSymbol, cap)
		return nil
	}

	// Volatility-scaled levels, BEFORE the grid gate.
	//
	// Ordered this way deliberately: the gate asks whether the stop can survive
	// the price grid, and it must judge the stop that will actually be sent. A
	// wider stop clears the grid more easily, so gating the strategy's original
	// 0.6% would refuse symbols that are perfectly tradable once the stop is
	// measured properly.
	if b.vol != nil {
		if frac, ok := b.vol.StopFractionFor(ctx, symbol); ok {
			newStop, newTarget := volScaledLevels(entry, stop, target, frac, long)
			if newStop > 0 && newTarget > 0 {
				// The time stop scales with the distance. A stop three times
				// wider takes far longer to resolve, and leaving the TTL at one
				// hour would close most positions on time before either level
				// was reached — replacing "stopped out by noise" with "timed
				// out before the signal could be judged", which answers the
				// question no better.
				if origRisk := math.Abs(entry - stop); origRisk > 0 {
					if scale := math.Abs(entry-newStop) / origRisk; scale > 1 {
						ttl = time.Duration(float64(ttl) * scale)
					}
				}
				stop, target = newStop, newTarget
			}
		}
	}

	// PRE-TRADE protectability gate.
	//
	// This check already existed, but it ran when brackets were ATTACHED —
	// after the position was funded. By then the only options were bad ones:
	// the engine kept the position and fell back to a 15s monitor, which closed
	// one 1.5x past its stop. A position you cannot protect is not a position
	// with weaker protection; it is a position that should not have been
	// opened.
	//
	// Refusing a signal costs nothing. Opening one that cannot be bracketed
	// costs whatever the market does next.
	if ticks, reason := stopGridTicks(b.reg, symbol, entry, stop); reason != "" {
		log.Printf("[PERP LIVE] %s %s: refused before entry — %s", strategy, symbol, reason)
		b.rejected.Add(1)
		b.mu.Lock()
		if b.gridBlocked == nil {
			b.gridBlocked = make(map[string]gridBlock)
		}
		k := perpStreamKey(strategy, symbol)
		gb := b.gridBlocked[k]
		gb.Refusals++
		gb.Consecutive++
		gb.LastStopTicks = ticks
		b.gridBlocked[k] = gb
		dead := gb.Consecutive >= gridAutoDisableAfter && !b.strategyOff[k]
		b.mu.Unlock()

		// A stream the grid keeps refusing is switched off, so it stops
		// occupying a roster slot it cannot use.
		//
		// On CONSECUTIVE refusals, not the first. Stops are volatility-scaled,
		// so one stream clears the tick grid when the market moves and fails
		// when it is quiet — measured live, both streams that had ever been
		// refused had also traded, and one of them was net positive. Switching
		// off at the first refusal would silence working strategies.
		//
		// Reversible: the row toggle turns it back on, and any fill resets the
		// counter.
		if dead {
			log.Printf("[PERP LIVE] %s on %s switched OFF automatically — %d consecutive grid refusals, last stop %.1f ticks",
				strategy, symbol, gb.Consecutive, ticks)
			b.SetStrategyEnabled(strategy, symbol, false)
		}
		return nil
	}

	// The stream reached sizing, so the grid is not blocking it. Clear the
	// streak here rather than on fill: an order refused for capacity is still
	// evidence the grid was passable.
	b.mu.Lock()
	if gb, ok := b.gridBlocked[perpStreamKey(strategy, symbol)]; ok && gb.Consecutive > 0 {
		gb.Consecutive = 0
		b.gridBlocked[perpStreamKey(strategy, symbol)] = gb
	}
	b.mu.Unlock()

	plan, err := PlanPerpOrder(b.reg, cfg, symbol, long, entry, stop, target, openCount, openNotional)
	if err != nil {
		// A sub-contract refusal is stated OUT LOUD, once per signal.
		//
		// It used to go only to noteError, which records a single last-error
		// string. That is fine for a transient capacity skip and wrong for this
		// one: a stream whose every signal rounds to zero contracts presents as
		// a strategy that never fires, and the desk offers no way to tell the
		// two apart. ETHUSD is deliberately on the roster in exactly that state
		// — $23.08 a contract against $10.85 of risk-sized notional — so the
		// reason it is quiet has to be legible without reading the sizing code.
		//
		// Capacity refusals stay quiet, because those are genuinely routine and
		// self-clearing.
		if errors.Is(err, ErrRiskTooSmall) {
			log.Printf("[PERP LIVE] %s %s: refused before entry — %v (risk $%.4f at a %.3f%% stop "+
				"buys less than one contract; it will size once the stop tightens or equity rises)",
				strategy, symbol, err, cfg.EquityUSD*cfg.RiskPerTradeFraction,
				math.Abs(entry-stop)/entry*100)
		}
		b.noteError(err.Error())
		return nil
	}

	// A stop the venue would pre-empt is not a stop.
	//
	// At the account's default 100x leverage, ADAUSD liquidates 0.5% adverse,
	// while these strategies stop at 0.35%-0.98%. Two positions were force-closed
	// at exactly 0.500% before their own stops were reached. Refusing here means
	// the STRATEGY decides the exit; taking the trade anyway produces a record of
	// liquidations dressed as stop-outs.
	// This contract's own maintenance margin, not a constant. 56 of the 92
	// perpetuals require 2.5%, where liquidation sits at 7.50% rather than the
	// 9.50% the old constant assumed.
	maint := perpMaintenanceMarginPct
	if pr, ok := b.reg.Lookup(plan.Symbol); ok && pr.MaintenanceMarginPct > 0 {
		maint = pr.MaintenanceMarginPct
	}
	if !StopIsReachable(plan.LimitPrice, plan.StopPrice, PerpLeverage, maint) {
		b.noteError(ErrStopBeyondLiquidation.Error())
		log.Printf("[PERP LIVE] ⏭️  skip %s %s — stop %.6f is beyond the liquidation distance at %dx",
			strategy, plan.Symbol, plan.StopPrice, PerpLeverage)
		return nil
	}

	res, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: plan.ProductID,
		Size:      plan.Contracts,
		Side:      plan.Side,
		OrderType: TypeMarket,
		// Explicit leverage so the margin this position consumes is predictable
		// rather than whatever the account happens to be set to.
		// Delta IGNORES leverage on the order; the per-product account setting
		// governs margin and is applied by EnsureLeverage at startup. Sent
		// anyway for the audit trail, not relied upon.
		Leverage: PerpLeverage,
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

	// Levels are recomputed from the ACTUAL FILL, not from the paper entry the
	// plan was built on.
	//
	// The plan's stop and target are distances measured from the paper desk's
	// entry. The live order is a market order and fills wherever the book is, so
	// inheriting those absolute prices leaves the distances wrong from the first
	// second — and wrong asymmetrically, since a fill that slipped against the
	// position moves the target closer and the stop further away. It produced
	// take-profits at a 0.05% move on a 0.350% target: five "wins" that were
	// losses after fees.
	// Named distinctly from the `stop`/`target` PARAMETERS, which are the paper
	// desk's levels. Shadowing them here would make the two indistinguishable at
	// a glance, and the whole point is that they are different numbers.
	fillStop, fillTarget := perpLevelsFromFill(fill, plan)
	bracketed := true
	if err := b.attachBrackets(ctx, plan, fillStop, fillTarget); err != nil {
		bracketed = false
		// The position is open and unprotected. Log loudly rather than pretend:
		// the Monitor still manages it, which is the behaviour that produced the
		// overshoot, so this is a degraded state and must read as one.
		log.Printf("[PERP LIVE] ⚠️  %s %s: venue brackets NOT attached (%v) — falling back to the 15s monitor",
			strategy, plan.Symbol, err)
		b.noteError("brackets: " + err.Error())
	}

	t := &PerpLiveTrade{
		ID:               fmt.Sprintf("perp-%d-%s", time.Now().UnixNano(), plan.Symbol),
		Strategy:         strategy,
		Symbol:           plan.Symbol,
		ProductID:        plan.ProductID,
		Side:             plan.Side,
		Contracts:        plan.Contracts,
		StopPrice:        fillStop,
		TargetPrice:      fillTarget,
		BracketsAttached: bracketed,
		EntryPrice:       fill,
		NotionalUSD:      plan.NotionalUSD,
		RiskUSD:          plan.RiskUSD,
		OpenedAt:         time.Now().UTC(),
		OrderID:          res.OrderID,
		Status:           "OPEN",
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
	// fillStop/fillTarget, NOT plan.StopPrice/plan.TargetPrice.
	//
	// The plan's levels are distances from the PAPER entry; the ones above are
	// what the venue actually holds. Printing the plan's made every entry line
	// read as though the position were opened at the fill with the paper stop,
	// and on a long that filled below its own paper stop the line printed a stop
	// ABOVE the entry — a sign error that was not happening. Diagnosing this
	// desk from its log is the normal path, so a log that disagrees with the
	// venue is worse than no log.
	log.Printf("[PERP LIVE] ✅ %s %s %s %d contracts @ %.6f | stop %.6f target %.6f ttl %s | $%.2f notional, $%.2f at risk",
		plan.Side, plan.Symbol, strategy, plan.Contracts, fill, fillStop, fillTarget,
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
	// Funding settles every 8h, so polling it on the exit cadence would be
	// wasteful; once a minute is far more than enough to keep the figure honest.
	fund := time.NewTicker(60 * time.Second)
	defer fund.Stop()
	if err := b.RefreshFunding(ctx); err != nil {
		log.Printf("[PERP LIVE] funding read failed at start: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-fund.C:
			// A failure leaves the LAST known total in place rather than
			// resetting to zero. Zero is a claim that no funding was paid; a
			// stale figure is at least a measured one.
			if err := b.RefreshFunding(ctx); err != nil {
				b.noteError("funding read: " + err.Error())
			}
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
			// The venue no longer reports this position — and with brackets
			// live this is now the NORMAL exit, not an exception: Delta filled
			// the stop or the target and the position is gone.
			//
			// This booked it at t.EntryPrice, which records exactly $0.00 for a
			// trade that really made or lost money. That was tolerable when the
			// branch only caught rare external closes; with the venue doing
			// every price exit it would silently zero most of the record.
			//
			// So ask the venue what it actually filled at. Failing that, mark it
			// UNRECONCILED rather than inventing a flat result — an unknown that
			// is visible can be corrected, a loss disguised as zero cannot.
			exit, reason := b.venueExitFor(ctx, t)
			b.finish(t, exit, reason)
			// Any surviving leg of the bracket must go. One side filled; the
			// other is still resting, and a resting reduce-only order against a
			// position that no longer exists can be left dangling or, worse,
			// re-arm on the next position in the same symbol.
			b.cancelBracketFor(ctx, t)
			continue
		}
		if reason := perpExitReason(t, mark, time.Now().UTC()); reason != "" {
			if err := b.closePosition(ctx, t, mark, reason); err != nil {
				log.Printf("[PERP LIVE] close failed for %s %s: %v", t.Strategy, t.Symbol, err)
				continue
			}
			// The position is flat, so its bracket must not outlive it. A
			// leftover trigger would sit on the venue and could arm against a
			// LATER position in the same symbol — opening exposure nobody asked
			// for, at a price chosen for a trade that already closed.
			b.cancelBracketFor(ctx, t)
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
	// PRICE EXITS BELONG TO THE VENUE when it is holding a bracket.
	//
	// Both mechanisms were live and the monitor kept winning, because it polls
	// mark every 15s and closes at market while the bracket waits for last
	// traded to cross. That is strictly worse: a market close pays whatever the
	// book offers after a 15-second delay, which is the overshoot the bracket
	// exists to remove. Standing down here is the fix — not adding a third
	// mechanism, but letting the correct one act.
	//
	// The TIME STOP still belongs to this process: Delta has no concept of it.
	if !t.BracketsAttached {
		// No venue protection, so the monitor is the only thing between this
		// position and an unbounded loss. Worse than a bracket, better than
		// nothing.
		if r := perpPriceExit(t, mark); r != "" {
			return r
		}
	} else if perpStopBreachedBadly(t, mark) {
		// BACKSTOP: bracketed, but price is far past the stop and the position
		// is still open. The venue leg either has not triggered or triggered and
		// could not fill.
		//
		// Both happen. LABUSD triggered at 0.1243 into a market at 0.1253; the
		// leg converted to a buy limit below the market and rested unfilled
		// while the loss ran from a planned -$1.78 to -$3.22, with the monitor
		// standing down because a bracket was "attached". Trusting the bracket
		// absolutely turned a capped loss into an open-ended one.
		//
		// So the stand-down is conditional, not total: the venue owns the normal
		// exit, and this process still owns the case where the venue demonstrably
		// has not acted.
		return "SL_BACKSTOP"
	}
	// The time stop. Checked last so it never masks a price exit, but checked —
	// it is how 9 in 10 of these trades end on the paper desk.
	if !t.ExpiresAt.IsZero() && !now.Before(t.ExpiresAt) {
		return "TTL"
	}
	return ""
}

// perpStopBackstopFraction is how far past the stop price must go before the
// monitor overrides a bracket.
//
// Wide enough that ordinary trigger latency does not cause a double-close — the
// bracket usually fills within a tick or two — and tight enough that a stop
// which cannot fill is caught long before liquidation at ~7.5%.
const perpStopBackstopFraction = 0.004

// perpStopBreachedBadly reports a position sitting well beyond its stop.
//
// The signal that a venue bracket has not done its job: not that price touched
// the stop, but that it went through and STAYED through.
func perpStopBreachedBadly(t *PerpLiveTrade, mark float64) bool {
	if t == nil || t.StopPrice <= 0 || mark <= 0 {
		return false
	}
	limit := t.StopPrice * perpStopBackstopFraction
	if t.long() {
		return mark <= t.StopPrice-limit
	}
	return mark >= t.StopPrice+limit
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
	t.StopOvershoot = perpStopOvershoot(t, exit)
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
// perpNetByProduct sums the bridge's SIGNED contracts per product.
//
// Pure so it can be tested without a venue: the netting arithmetic is the part
// that was wrong, not the HTTP call.
func perpNetByProduct(open map[string]*PerpLiveTrade) map[int]int {
	out := make(map[int]int, len(open))
	for _, t := range open {
		n := t.Contracts
		if !t.long() {
			n = -n
		}
		out[t.ProductID] += n
	}
	return out
}

// perpNetMismatches returns the products where the two books disagree.
//
// Checks the UNION of both sides on purpose: a position the bridge has
// forgotten must be as loud as one it invented, and iterating only the bridge's
// own products would miss the first — which is the dangerous one.
func perpNetMismatches(mine, venue map[int]int) []int {
	products := make(map[int]bool, len(mine)+len(venue))
	for id := range mine {
		products[id] = true
	}
	for id := range venue {
		products[id] = true
	}
	var bad []int
	for id := range products {
		if mine[id] != venue[id] {
			bad = append(bad, id)
		}
	}
	sort.Ints(bad)
	return bad
}

// Reconcile compares the bridge's book against the venue by NET SIZE PER
// SYMBOL, not by position count.
//
// Counting positions is invalid here and produced a permanent false alarm.
// Delta NETS every order on a symbol into one position; the bridge tracks a
// position per strategy. Two strategies on ADAUSD - one long 1,511 contracts,
// one short 75 - are two rows here and ONE row of 1,436 there. Both are
// correct. The old check called that a mismatch and logged a warning every
// cycle.
//
// That is worse than no check. A control that fires during normal operation
// teaches its operator to ignore it, and this is the control meant to catch the
// case that actually matters: real contracts on the venue that this process has
// forgotten about. During the 2026-08-01 audit it fired 38 times in 20 minutes
// and every one was noise.
//
// Net size is the invariant that survives netting: whatever the bridge believes
// it is holding on a symbol must equal what Delta holds on that symbol.
func (b *PerpBridge) Reconcile(ctx context.Context) (engineOpen, venueOpen int, matched bool, err error) {
	b.mu.RLock()
	engineOpen = len(b.open)
	mineNet := perpNetByProduct(b.open)
	b.mu.RUnlock()

	if b.client == nil {
		return engineOpen, 0, false, fmt.Errorf("perp bridge: no Delta client")
	}
	positions, err := b.client.GetPositions(ctx)
	if err != nil {
		return engineOpen, 0, false, err
	}

	venueNet := make(map[int]int, len(positions))
	for _, p := range positions {
		if p.Size != 0 {
			venueNet[p.ProductID] += int(p.Size)
		}
		if p.Size != 0 && mineNet[p.ProductID] != 0 {
			venueOpen++
		}
	}

	bad := perpNetMismatches(mineNet, venueNet)
	for _, id := range bad {
		log.Printf("[PERP LIVE] ⚠️  reconciliation mismatch on product %d: bridge net %d contracts, Delta net %d",
			id, mineNet[id], venueNet[id])
	}
	return engineOpen, venueOpen, len(bad) == 0, nil
}

func (b *PerpBridge) noteError(msg string) {
	b.mu.Lock()
	b.lastError = msg
	b.mu.Unlock()
}

// PerpBridgeStats is the operator view.
type PerpBridgeStats struct {
	Armed         bool     `json:"armed"`
	EquityUSD     float64  `json:"equityUsd"`
	RiskPerTrade  float64  `json:"riskPerTradeUsd"`
	MaxLeverage   float64  `json:"maxLeverage"`
	MaxConcurrent int      `json:"maxConcurrent"`
	Strategies    []string `json:"strategies"`
	// LiveStreams is the roster at its real granularity: (strategy, symbol)
	// pairs. One strategy commonly runs on several symbols as independent
	// positions, so a board keyed on strategy alone merges records that belong
	// to different instruments.
	LiveStreams []PerpStreamView `json:"liveStreams"`
	// Strategies switched off by the owner. Reported so the board can render
	// the switch from engine truth rather than from browser state, which would
	// drift the moment anything changed it from elsewhere or a restart happened.
	DisabledStrategies []string        `json:"disabledStrategies"`
	OpenPositions      []PerpLiveTrade `json:"openPositions"`
	Submitted          int64           `json:"submitted"`
	Rejected           int64           `json:"rejected"`
	Closed             int64           `json:"closed"`
	RealisedPnL        float64         `json:"realisedPnlUsd"`
	// FundingUSD is perpetual funding settled on this account, taken from the
	// venue ledger.
	//
	// It appears in NO other source — not in fills, not in trade P&L — because
	// funding is charged against the POSITION every eight hours, not against a
	// trade. A desk that reports only realised trade P&L is silently omitting a
	// recurring cash flow on every position held across a window, and reporting
	// zero for a number that is simply unmeasured is the same failure the
	// 2026-08-01 audit was about.
	//
	// Account-level, not per-strategy: Delta nets positions by symbol, so
	// funding on a symbol cannot be attributed to one of several strategies
	// holding it. Stated at the level where it is true.
	FundingUSD      float64 `json:"fundingUsd"`
	NetAfterFunding float64 `json:"netAfterFundingUsd"`
	LastError       string  `json:"lastError,omitempty"`
	ProductsKnown   int     `json:"productsKnown"`
}

// Stats snapshots the bridge.
func (b *PerpBridge) Stats() PerpBridgeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	open := make([]PerpLiveTrade, 0, len(b.open))
	for _, t := range b.open {
		c := *t
		// Computed on READ, not only at open.
		//
		// Computing at open left every position restored from custody — and
		// every position that predates a deploy — reporting $0.00 for both
		// outcomes, which reads as "this trade risks nothing and wins nothing"
		// rather than "not calculated". Derived from fields the trade already
		// carries, so there is no state to keep in sync.
		if pr, ok := b.reg.Lookup(c.Symbol); ok {
			c.IfTargetUSD, c.IfStopUSD = perpOutcomeUSD(&c, pr.ContractValue)
		}
		open = append(open, c)
	}
	sort.Slice(open, func(i, j int) bool { return open[i].OpenedAt.Before(open[j].OpenedAt) })

	pnl := 0.0
	for _, t := range b.history {
		pnl += t.RealisedPnL
	}
	strategies := b.allow.Strategies()
	sort.Strings(strategies)

	funding := b.fundingUSD.Load().(float64)

	return PerpBridgeStats{
		Armed:         b.armed.Load(),
		EquityUSD:     b.cfg.EquityUSD,
		RiskPerTrade:  b.cfg.EquityUSD * b.cfg.RiskPerTradeFraction,
		MaxLeverage:   b.cfg.MaxLeverage,
		MaxConcurrent: b.cfg.MaxConcurrentPositions,
		DisabledStrategies: func() []string {
			out := make([]string, 0, len(b.strategyOff))
			for n := range b.strategyOff {
				out = append(out, n)
			}
			sort.Strings(out)
			return out
		}(),
		Strategies: strategies,
		LiveStreams: func() []PerpStreamView {
			pairs := b.allow.Pairs()
			out := make([]PerpStreamView, 0, len(pairs))
			for _, st := range pairs {
				k := perpStreamKey(st.Strategy, st.Symbol)
				gb := b.gridBlocked[k]
				out = append(out, PerpStreamView{
					Strategy:      st.Strategy,
					Symbol:        st.Symbol,
					Enabled:       !b.strategyOff[k],
					GridRefusals:  gb.Refusals,
					LastStopTicks: gb.LastStopTicks,
				})
			}
			return out
		}(),
		OpenPositions:   open,
		Submitted:       b.submitted.Load(),
		Rejected:        b.rejected.Load(),
		Closed:          b.closes.Load(),
		RealisedPnL:     pnl,
		FundingUSD:      funding,
		NetAfterFunding: pnl + funding,
		LastError:       b.lastError,
		ProductsKnown:   b.reg.Count(),
	}
}

// ClearHistory drops the closed-trade record and returns how many rows went.
//
// Only the CLOSED record. Open positions live in b.open and are untouched: they
// are real money on the venue, and the bridge's own book is what lets it manage
// their stop, target and expiry. Erasing an open row would leave the position
// on Delta with nothing tracking it.
//
// This is destructive in a way the leaderboard makes easy to underestimate. The
// closed record is what the leaderboard ranks and therefore what the promotion
// gate reads, so clearing it resets the evidence every capital decision rests
// on — a stream with 200 fills behind it goes back to zero and has to earn the
// sample again.
//
// Persisted immediately, so the clear survives a restart rather than reappearing.
func (b *PerpBridge) ClearHistory() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	removed := len(b.history)
	b.history = nil
	b.persistLocked()
	return removed
}

// History returns closed live trades, newest last.
func (b *PerpBridge) History() []PerpLiveTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]PerpLiveTrade, len(b.history))
	copy(out, b.history)
	return out
}
