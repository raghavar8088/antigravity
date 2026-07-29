package delta

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LiveTrade records a real Delta Exchange order that was mirrored from a paper signal.
type LiveTrade struct {
	ID             string     `json:"id"`
	PaperTradeID   string     `json:"paperTradeId"`
	StrategyID     int        `json:"strategyId"`
	StrategyName   string     `json:"strategyName"`
	OptionType     string     `json:"optionType"` // "CALL" or "PUT"
	Strike         float64    `json:"strike"`
	ExpiryTime     time.Time  `json:"expiryTime"`
	Side           string     `json:"side"` // "sell" (we sell options) or "buy" (to close)
	DeltaOrderID   string     `json:"deltaOrderId"`
	DeltaSymbol    string     `json:"deltaSymbol"`
	ProductID      int        `json:"productId"`
	Contracts      int        `json:"contracts"`
	FillPrice      float64    `json:"fillPrice"`
	PremiumUSD     float64    `json:"premiumUsd"`
	Status         string     `json:"status"` // "OPEN","CLOSED","FAILED","CANCELLED"
	OpenedAt       time.Time  `json:"openedAt"`
	ClosedAt       *time.Time `json:"closedAt,omitempty"`
	CloseOrderID   string     `json:"closeOrderId,omitempty"`
	CloseFillPrice float64    `json:"closeFillPrice,omitempty"`
	RealizedPnl    float64    `json:"realizedPnl,omitempty"`
	FailureReason  string     `json:"failureReason,omitempty"`
	// EntryBTCPrice is spot at the moment the signal fired. Fees are charged on
	// underlying notional, so pricing a round trip needs it; without it the fee
	// falls back to the 10%-of-premium cap alone.
	EntryBTCPrice float64 `json:"entryBtcPrice,omitempty"`
	// EntryFeeUSD / ExitFeeUSD are the real Delta fees for each leg, and GrossPnl
	// is the pre-fee result. RealizedPnl is NET of both legs.
	//
	// These exist because the live path previously modelled no fee whatsoever:
	// RealizedPnl was (exit-entry)×size, so every strategy statistic the desk
	// promoted on — expectancy, profit factor, win contribution — was computed
	// gross while real capital paid ~28% of premium per round trip.
	EntryFeeUSD float64 `json:"entryFeeUsd,omitempty"`
	ExitFeeUSD  float64 `json:"exitFeeUsd,omitempty"`
	GrossPnl    float64 `json:"grossPnl,omitempty"`
	// LastMarkPrice is the most recent mark seen by the monitor. Used to value an
	// expiry close, which has no fill price of its own — without it an expired
	// position reported $0 P&L even though the premium was really lost or won.
	LastMarkPrice float64 `json:"lastMarkPrice,omitempty"`
	// PeakMarkPrice is the highest mark seen since entry. It drives the trailing
	// exit that replaced the fixed take-profit: a long option's only structural
	// edge is its unbounded upside, and a hard +80% cap amputated exactly that.
	PeakMarkPrice float64 `json:"peakMarkPrice,omitempty"`
	// ExitReason records WHY the position closed (take_profit_80pct,
	// stop_loss_50pct, near_expiry_30min, CLOSE_ALL, ...). Previously the reason
	// was only logged, so a closed position could not show what triggered it.
	ExitReason string `json:"exitReason,omitempty"`
}

// OpenSignal is the event fired when a paper position opens.
type OpenSignal struct {
	PaperTradeID string
	StrategyID   int
	StrategyName string
	OptionType   string // "CALL" or "PUT"
	Strike       float64
	ExpiryTime   time.Time
	// PremiumUSD is the paper leg's cash cost basis; PremiumPerBTC is the quoted
	// premium (USD per BTC) for one BTC of underlying. Fee economics need the
	// quote — read against BTCPrice — because the fee is a share of notional
	// capped at a share of premium, and only their ratio decides which binds.
	PremiumUSD    float64
	PremiumPerBTC float64
	BTCPrice      float64
}

// CloseSignal is the event fired when a paper position closes.
type CloseSignal struct {
	PaperTradeID string
	StrategyID   int
	OptionType   string
	Strike       float64
	ExitReason   string
	ExitBTCPrice float64
}

// Bridge mirrors paper BTC option signals to Delta Exchange live orders.
// In selling mode (default) it sells options — requires large margin.
// In buying mode it buys options — costs only the premium (~$0.50/lot).
type Bridge struct {
	mu         sync.RWMutex
	client     *Client
	enabled    bool
	configured bool
	buyingMode bool // when true: BUY options instead of SELL
	trades     []LiveTrade
	seq        int

	// openByPaperID maps paper position ID → live trade ID (NOT a slice index).
	// It previously stored an index, but trades are PREPENDED on every open and
	// adoption, which shifted every element and invalidated every stored index —
	// closes then silently no-opped (leaving real positions unmanaged) or could
	// resolve to the wrong trade. Trade IDs are stable, so they cannot go stale.
	openByPaperID map[string]string

	// institutionalOpen, when set, replaces direct broker placement in OnOpen.
	institutionalOpen func(context.Context, OpenSignal, string) error

	// institutionalClose routes OnClose through the engine institutional execution stack.
	institutionalClose func(context.Context, CloseSignal, LiveTrade) error

	// killCheck blocks broker submission when the institutional kill switch is active.
	killCheck func(context.Context) error

	// submitResultHook, when set, is notified of every opening-order outcome so
	// the Live Engine can track consecutive broker rejects.
	submitResultHook func(err error)

	// liveEligibility answers whether a strategy has earned real capital, injected
	// from the wiring layer so the bridge does not import the roster (and the
	// roster stays free to evolve its gates independently). Nil means "unknown",
	// which is treated as not-eligible so a missing wiring cannot silently open
	// the gate. enforceGate decides whether a failing verdict blocks the order or
	// is only reported — see LIVE_ENGINE_ENFORCE_GATE.
	liveEligibility func(strategyName string) (bool, string)
	enforceGate     bool

	// nativeBuy is true when the signal source is already long-side (the option
	// BUYING engine), so OnOpen must NOT invert the option type. The legacy path
	// (mirroring the selling engine) inverts; a native buy source does not.
	nativeBuy bool

	// liveAllow, when non-nil, restricts live mirroring to exactly these strategy
	// names — the Live Engine's per-strategy allow-list. nil = allow all (legacy);
	// non-nil (even empty) = only the listed strategies may place live orders.
	liveAllow map[string]bool
}

// NewBridge creates a Bridge. If Delta keys are not set it starts in a disabled/unconfigured state.
func NewBridge() *Bridge {
	b := &Bridge{
		openByPaperID: make(map[string]string),
	}

	client, err := NewClient()
	if err != nil {
		log.Printf("[DELTA BRIDGE] Not configured: %v", err)
		b.configured = false
	} else {
		b.client = client
		b.configured = true
		b.enabled = false // starts disabled until user explicitly enables
		log.Printf("[DELTA BRIDGE] Configured (testnet=%v) — disabled until enabled via API", IsTestnet())
	}
	return b
}

// IsBuyingMode reports whether the bridge is in option-buy mode.
func (b *Bridge) IsBuyingMode() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.buyingMode
}

func (b *Bridge) SetKillCheck(fn func(context.Context) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.killCheck = fn
}

// SetSubmitResultHook registers a callback invoked after every opening-order
// submission with its outcome (nil on success). The Live Engine uses it to track
// consecutive broker rejects for its auto-disarm trigger.
func (b *Bridge) SetSubmitResultHook(fn func(err error)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.submitResultHook = fn
}

// SetInstitutionalOpenHandler routes OnOpen through the engine institutional execution stack.
func (b *Bridge) SetInstitutionalOpenHandler(fn func(context.Context, OpenSignal, string) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.institutionalOpen = fn
}

// SetInstitutionalCloseHandler routes OnClose through the engine institutional execution stack.
func (b *Bridge) SetInstitutionalCloseHandler(fn func(context.Context, CloseSignal, LiveTrade) error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.institutionalClose = fn
}

// SubmitOrder places an order on Delta. Structurally callable only from the
// institutional fill path: it rejects any context lacking risk-gate provenance
// and re-checks the kill switch before touching the network.
func (b *Bridge) SubmitOrder(ctx context.Context, productID int, side OrderSide, contracts int) (PlaceOrderResult, error) {
	if err := b.guardEffector(ctx, true); err != nil {
		return PlaceOrderResult{}, err
	}
	if b.client == nil {
		return PlaceOrderResult{}, fmt.Errorf("delta client not configured")
	}
	res, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: productID,
		Size:      contracts,
		Side:      side,
		OrderType: TypeMarket,
		Leverage:  10,
	})
	b.mu.RLock()
	hook := b.submitResultHook
	b.mu.RUnlock()
	if hook != nil {
		hook(err)
	}
	return res, err
}

// SubmitReduceOnlyOrder closes a live position. Same structural guard as
// SubmitOrder: risk-gate provenance + kill-switch recheck before any network call.
func (b *Bridge) SubmitReduceOnlyOrder(ctx context.Context, productID int, side OrderSide, contracts int) (PlaceOrderResult, error) {
	if err := b.guardEffector(ctx, false); err != nil {
		return PlaceOrderResult{}, err
	}
	if b.client == nil {
		return PlaceOrderResult{}, fmt.Errorf("delta client not configured")
	}
	return b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID:            productID,
		Size:                 contracts,
		Side:                 side,
		OrderType:            TypeMarket,
		Leverage:             10,
		ReduceOnly:           true,
		CancelOrdersAccepted: "true",
	})
}

// UpdateTradeAfterClose updates live trade state after an institutional close fill.
func (b *Bridge) UpdateTradeAfterClose(tradeID string, result PlaceOrderResult, buying bool) {
	now := time.Now().UTC()
	b.updateTrade(tradeID, func(t *LiveTrade) {
		t.Status = "CLOSED"
		t.CloseOrderID = result.OrderID
		t.CloseFillPrice = result.Price
		t.ClosedAt = &now
		// Premiums are quoted USD per BTC and a contract is 0.001 BTC, so realised
		// PnL must include the contract size. Without it this overstated every
		// close by 1000x (a $0.05 result would have been reported as $50).
		btc := float64(t.Contracts) * OptionContractSizeBTC
		if buying {
			t.GrossPnl = (result.Price - t.FillPrice) * btc
		} else {
			t.GrossPnl = (t.FillPrice - result.Price) * btc
		}
		// Net of both fee legs. The entry fee was booked at fill; the exit leg is
		// priced at the closing premium, which is why a winning close costs more
		// in fees than the entry did.
		t.ExitFeeUSD = OptionFeeUSD(result.Price, t.EntryBTCPrice, t.Contracts)
		t.RealizedPnl = t.GrossPnl - t.EntryFeeUSD - t.ExitFeeUSD
	})
}

// UpdateTradeAfterFill updates live trade state after institutional fill.
func (b *Bridge) UpdateTradeAfterFill(tradeID string, result PlaceOrderResult, productID int, contracts int, strike float64, expiry time.Time) {
	b.updateTrade(tradeID, func(t *LiveTrade) {
		// Never resurrect a trade the broker rejected. This ran unconditionally
		// after the institutional path, stamping OPEN over a FAILED status, so a
		// rejected order (e.g. invalid_contract) counted as an open live trade —
		// producing a permanent engine-vs-Delta position mismatch that
		// auto-disarmed the Live Engine on every arm.
		if t.Status == "FAILED" || t.Status == "CANCELLED" {
			return
		}
		// A genuine fill has a broker order id; without one there is nothing open.
		if result.OrderID == "" {
			t.Status = "FAILED"
			if t.FailureReason == "" {
				t.FailureReason = "no broker order id returned — treated as not filled"
			}
			return
		}
		t.DeltaOrderID = result.OrderID
		t.DeltaSymbol = result.Symbol
		t.ProductID = productID
		t.Contracts = contracts
		t.FillPrice = result.Price
		t.Strike = strike
		t.ExpiryTime = expiry
		t.Status = "OPEN"
		// Book the entry fee against the real fill, not the signal's estimate, and
		// seed the trailing-exit peak so a position that only ever falls still has
		// a sane reference.
		t.EntryFeeUSD = OptionFeeUSD(result.Price, t.EntryBTCPrice, contracts)
		t.PeakMarkPrice = result.Price
	})
}

func (b *Bridge) RegisterOpenMapping(paperTradeID, tradeID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if idx := b.indexOfID(tradeID); idx >= 0 {
		// Only track trades that are actually open; a rejected order must not be
		// registered as an open live position.
		if b.trades[idx].Status != "OPEN" {
			return
		}
		b.openByPaperID[paperTradeID] = tradeID
	}
}

// openIndexForPaperID resolves a paper position ID to the index of its OPEN live
// trade. It prefers the mapping, then falls back to a scan by PaperTradeID so a
// restored or adopted trade with a missing mapping can still be closed — a real
// position must never become uncloseable because a lookup failed.
// Caller holds b.mu.
// openLiveTradeForStrategyLocked returns the ID of an already-open live trade for
// this strategy, or "" if the strategy has no live position. Callers hold b.mu.
func (b *Bridge) openLiveTradeForStrategyLocked(strategyID int) string {
	for i := range b.trades {
		if b.trades[i].StrategyID == strategyID && b.trades[i].Status == "OPEN" {
			return b.trades[i].ID
		}
	}
	return ""
}

func (b *Bridge) openIndexForPaperID(paperID string) int {
	if tradeID, ok := b.openByPaperID[paperID]; ok {
		if idx := b.indexOfID(tradeID); idx >= 0 && b.trades[idx].Status == "OPEN" {
			return idx
		}
	}
	for i := range b.trades {
		if b.trades[i].PaperTradeID == paperID && b.trades[i].Status == "OPEN" {
			return i
		}
	}
	return -1
}

// Client exposes the underlying Delta REST client for institutional fill planning.
func (b *Bridge) Client() *Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.client
}

// IsConfigured returns whether API keys are present.
func (b *Bridge) IsConfigured() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.configured
}

// IsEnabled returns whether live order mirroring is active.
func (b *Bridge) IsEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enabled
}

// SetBuyingMode switches between buying mode (BUY options, wallet-tiered sizing) and
// selling mode (SELL options, requires large margin). Use buying mode for small real balances;
// sizing uses TieredBuySizing / BuyingContractsFromWallet (see bridge_buy_sizing.go).
func (b *Bridge) SetBuyingMode(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buyingMode = v
	if v {
		log.Printf("[DELTA BRIDGE] 📈 BUYING MODE — will BUY calls/puts on signals (~$0.50/lot)")
	} else {
		log.Printf("[DELTA BRIDGE] 📉 SELLING MODE — will SELL options on signals")
	}
}

// SetNativeBuyMode marks the signal source as already long-side (the buying
// engine). When set, OnOpen does not invert the option type.
func (b *Bridge) SetNativeBuyMode(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nativeBuy = v
}

// SetLiveAllowList restricts live mirroring to exactly these strategy names.
// Passing a non-nil slice (even empty) enables the allow-list; only listed
// strategies may place live orders. This is the Live Engine's reversible
// per-strategy enable/disable.
func (b *Bridge) SetLiveAllowList(names []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := make(map[string]bool, len(names))
	for _, n := range names {
		if n != "" {
			m[n] = true
		}
	}
	b.liveAllow = m
	log.Printf("[DELTA BRIDGE] live allow-list set to %d strateg(ies): %v", len(m), names)
}

// LiveAllowList returns the current allow-list (sorted). Empty slice when the
// allow-list is active but empty; nil-vs-set is not distinguished here.
func (b *Bridge) LiveAllowList() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]string, 0, len(b.liveAllow))
	for n := range b.liveAllow {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// SetLiveEligibility injects the go-live evidence test. The bridge calls it for
// every prospective open; returning false with a reason means the strategy has
// not earned real capital yet.
func (b *Bridge) SetLiveEligibility(fn func(strategyName string) (bool, string)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.liveEligibility = fn
}

// SetGateEnforcement decides whether a failing go-live verdict BLOCKS the order
// (true) or is merely logged (false).
//
// It defaults to false deliberately. Enforcing immediately would halt the desk
// outright, because the gate requires a real-fill record that a desk trading on
// unenforced gates never accumulated. Report-only lets the record build under
// honest accounting; flip this on once the numbers exist to gate on.
func (b *Bridge) SetGateEnforcement(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enforceGate = v
	if v {
		log.Printf("[DELTA BRIDGE] 🔒 go-live gate ENFORCED — strategies without a qualifying record cannot open")
	} else {
		log.Printf("[DELTA BRIDGE] 🔓 go-live gate REPORT-ONLY — failing strategies are logged, not blocked")
	}
}

// GateEnforced reports whether the go-live gate currently blocks orders.
func (b *Bridge) GateEnforced() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enforceGate
}

// NOTE: there is deliberately no `liveEligibilityLocked` helper. The injected
// eligibility test calls back into the bridge (LiveStrategyRecord takes
// b.mu.RLock), so it must NEVER be invoked while holding b.mu — Go's RWMutex is
// not reentrant and doing so deadlocks the bridge permanently. OnOpen reads the
// callback under a read lock, releases, and only then calls it.

// StrategyRecord is the real-fill track record for one strategy, computed from
// closed live trades only. This is what the go-live gate is supposed to read;
// it previously read hardcoded zeros, so no strategy could ever qualify and the
// verdict was decorative.
type StrategyRecord struct {
	Strategy        string
	Fills           int
	Days            int
	Expectancy      float64 // mean NET pnl per closed trade, USD
	ProfitFactor    float64
	FeePctOfPremium float64 // round-trip fees as a share of premium deployed
	NetPnl          float64
	GrossPnl        float64
	Fees            float64
	Wins            int
	Losses          int
}

// LiveStrategyRecord computes the real track record for one strategy from closed
// live trades. Expectancy and profit factor are NET of fees — a gross profit
// factor is exactly the number that let this desk believe it had an edge.
func (b *Bridge) LiveStrategyRecord(name string) StrategyRecord {
	b.mu.RLock()
	trades := make([]LiveTrade, len(b.trades))
	copy(trades, b.trades)
	b.mu.RUnlock()

	rec := StrategyRecord{Strategy: name}
	var grossWins, grossLosses float64
	var first, last time.Time
	var premiumDeployed float64

	for _, t := range trades {
		if t.StrategyName != name || t.Status != "CLOSED" {
			continue
		}
		rec.Fills++
		rec.NetPnl += t.RealizedPnl
		rec.GrossPnl += t.GrossPnl
		rec.Fees += t.EntryFeeUSD + t.ExitFeeUSD
		premiumDeployed += OptionPremiumUSD(t.FillPrice, t.Contracts)

		if t.RealizedPnl >= 0 {
			rec.Wins++
			grossWins += t.RealizedPnl
		} else {
			rec.Losses++
			grossLosses += -t.RealizedPnl
		}
		if first.IsZero() || t.OpenedAt.Before(first) {
			first = t.OpenedAt
		}
		if t.ClosedAt != nil && t.ClosedAt.After(last) {
			last = *t.ClosedAt
		}
	}

	if rec.Fills == 0 {
		return rec
	}
	rec.Expectancy = rec.NetPnl / float64(rec.Fills)
	if !first.IsZero() && !last.IsZero() && last.After(first) {
		rec.Days = int(last.Sub(first).Hours()/24) + 1
	} else {
		rec.Days = 1
	}
	switch {
	case grossLosses > 0:
		rec.ProfitFactor = grossWins / grossLosses
	case grossWins > 0:
		rec.ProfitFactor = 999 // no losing trade yet — unbounded, reported as very high
	}
	if premiumDeployed > 0 {
		rec.FeePctOfPremium = rec.Fees / premiumDeployed * 100
	}
	return rec
}

// strategyAllowedLocked reports whether a strategy may place a live order. nil
// allow-list = allow all (legacy); non-nil = only listed. Caller holds b.mu.
func (b *Bridge) strategyAllowedLocked(name string) bool {
	if b.liveAllow == nil {
		return true
	}
	return b.liveAllow[name]
}

// SetEnabled enables or disables live order mirroring.
func (b *Bridge) SetEnabled(v bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if v && !b.configured {
		log.Printf("[DELTA BRIDGE] Cannot enable — not configured (missing API keys)")
		return
	}
	b.enabled = v
	if v {
		log.Printf("[DELTA BRIDGE] ✅ LIVE ORDER MIRRORING ENABLED")
	} else {
		log.Printf("[DELTA BRIDGE] ⏸️  Live order mirroring disabled")
	}
}

// monitorInterval is how often open live positions are checked against their
// SL/TP levels. Default 30s; override with LIVE_MONITOR_INTERVAL_SECONDS.
func monitorInterval() time.Duration {
	if v := strings.TrimSpace(os.Getenv("LIVE_MONITOR_INTERVAL_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 5 {
			return time.Duration(n) * time.Second
		}
	}
	return 30 * time.Second
}

// resolveLiveOptionSide decides the option type and side for a live order.
//   - Sell mode: sell the signal's exact type (legacy naked-short path).
//   - Buy mode, native source (buying engine): BUY the signal's exact type —
//     the signal is already long-side, so never invert.
//   - Buy mode, legacy source (mirroring the selling engine): invert the type
//     (paper sells a PUT on a bull view → we BUY a CALL to hold the same view).
//
// Getting this wrong buys the opposite option, so it is unit-tested directly.
func resolveLiveOptionSide(sigType string, buying, nativeBuy bool) (optType, side string) {
	optType = sigType
	side = "sell"
	if buying {
		side = "buy"
		if !nativeBuy {
			if sigType == "PUT" {
				optType = "CALL"
			} else {
				optType = "PUT"
			}
		}
	}
	return optType, side
}

// OnOpen is called when the paper engine opens a new position.
// In selling mode: sells the option (requires large margin).
// In buying mode: buys the opposite option type (cheap premium, fits small balance).
func (b *Bridge) OnOpen(sig OpenSignal) {
	// Pre-flight under a READ lock, released before the injected eligibility test
	// runs. That test reads the bridge's own trade history via LiveStrategyRecord,
	// which takes b.mu.RLock — and Go's RWMutex is not reentrant. Running it while
	// holding b.mu.Lock deadlocked the bridge permanently on the first live
	// signal: every later read (positions, account, state) blocked forever, so the
	// control plane returned 502 while /health still answered in milliseconds.
	//
	// Nothing here mutates, so a read lock is sufficient. The gap between this
	// snapshot and the write lock below is benign: an order that races a disarm is
	// still stopped downstream by the risk gate and the kill switch, which are
	// checked at submission, not here.
	b.mu.RLock()
	enabled, configured := b.enabled, b.configured
	allowed := b.strategyAllowedLocked(sig.StrategyName)
	enforceGate := b.enforceGate
	eligibilityFn := b.liveEligibility
	b.mu.RUnlock()

	if !enabled || !configured {
		return
	}

	// Live allow-list: only explicitly enabled strategies may place live orders.
	if !allowed {
		log.Printf("[DELTA BRIDGE] ⏭️  skip live open — strategy %q not in live allow-list", sig.StrategyName)
		return
	}

	// Go-live gate. The allow-list is an operator switch; this is the evidence
	// test (real fills, real days, positive expectancy, PF, fee bound). The two
	// were never connected: the roster computed a verdict for the UI while the
	// allow-list alone decided what traded, so strategies with a zero-fill record
	// placed real orders. Enforcement is off by default — a desk with no real
	// track record would halt entirely — and is flipped on via
	// LIVE_ENGINE_ENFORCE_GATE once a record exists to gate on.
	//
	// A nil test means "unknown", which is treated as not eligible so a missing
	// wiring cannot silently open the gate.
	eligible, reason := false, "no eligibility test wired"
	if eligibilityFn != nil {
		eligible, reason = eligibilityFn(sig.StrategyName)
	}
	if !eligible {
		if enforceGate {
			log.Printf("[DELTA BRIDGE] ⛔ BLOCK live open %q — go-live gate: %s", sig.StrategyName, reason)
			return
		}
		log.Printf("[DELTA BRIDGE] ⚠️  gate REPORT-ONLY: %q would be blocked — %s", sig.StrategyName, reason)
	}

	// Entry economics. Delta caps its option fee at 10% of premium per side, so
	// an option cheap enough for the cap to bind costs a flat ~28% of premium to
	// round-trip — a tax that demands a ~56% win rate from a long-option book.
	// Direction is irrelevant if the instrument cannot pay for itself.
	// The fee percentage is scale-invariant — premium and notional both scale with
	// contracts, so the ratio cancels — which is why one contract prices it.
	if econ := EvaluateEntryEconomics(sig.PremiumPerBTC, sig.BTCPrice, 1, LiveTakeProfitPct); !econ.Acceptable {
		log.Printf("[DELTA BRIDGE] ⏭️  skip live open %q — %s", sig.StrategyName, econ.Reason)
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Re-check the effector under the write lock: the pre-flight snapshot above is
	// deliberately lock-free, so a disarm in between must still be honoured here.
	if !b.enabled || !b.configured {
		return
	}

	// Expiry floor: a position opened inside this window cannot reach the +80%
	// target before the monitor force-closes it near expiry, so it is a losing
	// trade by construction rather than by outcome. Paper still takes the signal.
	if !sig.ExpiryTime.IsZero() {
		if ttl := time.Until(sig.ExpiryTime); ttl < MinTimeToExpiryForNewEntry {
			log.Printf("[DELTA BRIDGE] ⏭️  skip live open %q — %.0fm to expiry, under the %.0fm floor",
				sig.StrategyName, ttl.Minutes(), MinTimeToExpiryForNewEntry.Minutes())
			return
		}
	}

	// One live position per strategy. Paper closes its leg on a profit-take that
	// the live side now declines (see OnClose), so the strategy can legitimately
	// open a fresh paper position while the live leg is still running. Without
	// this guard that would stack a second real position on the same strategy.
	if openID := b.openLiveTradeForStrategyLocked(sig.StrategyID); openID != "" {
		log.Printf("[DELTA BRIDGE] ⏭️  skip live open %q — %s already open for this strategy",
			sig.StrategyName, openID)
		return
	}

	b.seq++
	id := fmt.Sprintf("DLT-%04d", b.seq)

	optType, side := resolveLiveOptionSide(sig.OptionType, b.buyingMode, b.nativeBuy)

	trade := LiveTrade{
		ID:            id,
		PaperTradeID:  sig.PaperTradeID,
		StrategyID:    sig.StrategyID,
		StrategyName:  sig.StrategyName,
		OptionType:    optType,
		Strike:        sig.Strike,
		ExpiryTime:    sig.ExpiryTime,
		Side:          side,
		PremiumUSD:    sig.PremiumUSD,
		EntryBTCPrice: sig.BTCPrice,
		Status:        "OPEN",
		OpenedAt:      time.Now().UTC(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		openHandler := b.institutionalOpen
		if openHandler == nil {
			b.updateTrade(id, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = "institutional open handler not wired — broker execution rejected"
			})
			log.Printf("[DELTA BRIDGE] ❌ INSTITUTIONAL OPEN REJECTED %s — handler not wired", id)
			return
		}
		if err := openHandler(ctx, sig, id); err != nil {
			b.updateTrade(id, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = err.Error()
			})
			log.Printf("[DELTA BRIDGE] ❌ INSTITUTIONAL OPEN FAILED %s — %v", id, err)
		}
	}()

	b.trades = append([]LiveTrade{trade}, b.trades...)
	if len(b.trades) > 500 {
		b.trades = b.trades[:500]
	}
	b.persistTradesLocked()
}

// OnClose is called when the paper engine closes a position.
func (b *Bridge) OnClose(sig CloseSignal) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Closing is risk-reducing and must work while DISARMED — otherwise an open
	// real position could never be taken off after a disarm/auto-disarm. Only
	// broker configuration is required here; new opens remain gated by `enabled`.
	if !b.configured {
		return
	}

	idx := b.openIndexForPaperID(sig.PaperTradeID)
	if idx < 0 {
		// Loud, not silent: this used to return quietly, so a close that could not
		// find its trade left a REAL position open with nothing managing it while
		// the monitor retried forever. If this ever fires, the position needs
		// manual attention.
		log.Printf("[DELTA BRIDGE] ⚠️  close requested for unknown/!open paper trade %q (reason=%s) — no live trade matched; check for an unmanaged position",
			sig.PaperTradeID, sig.ExitReason)
		return
	}

	// Custody owns the upside. A paper profit-take is a signal about the synthetic
	// chain, not about this real position, and it fires well below the live +80%
	// target — so honouring it capped every live winner. Decline it and leave the
	// position under the monitor, which still holds both the +80% take-profit and
	// the -50% stop. The paper leg closes either way; only the live leg runs on.
	//
	// The mapping and ExitReason are intentionally left untouched: this position is
	// still open, and the monitor closes it later via this same path with a custody
	// reason (take_profit_80pct / stop_loss_50pct / near_expiry_30min).
	if IsStrategyProfitCapExit(sig.ExitReason) {
		log.Printf("[DELTA BRIDGE] 🎯 holding %s through paper %s — live position runs to +%.0f%% TP / -%.0f%% SL",
			b.trades[idx].ID, sig.ExitReason, LiveTakeProfitPct*100, LiveStopLossPct*100)
		return
	}

	trade := b.trades[idx]
	delete(b.openByPaperID, sig.PaperTradeID)

	// Record why this position is closing so the closed list can show it.
	if sig.ExitReason != "" {
		b.trades[idx].ExitReason = sig.ExitReason
		trade.ExitReason = sig.ExitReason
	}

	tradeCopy := trade

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		closeHandler := b.institutionalClose
		if closeHandler == nil {
			now := time.Now().UTC()
			b.updateTrade(tradeCopy.ID, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = "institutional close handler not wired — broker execution rejected"
				t.ClosedAt = &now
			})
			log.Printf("[DELTA BRIDGE] ❌ INSTITUTIONAL CLOSE REJECTED %s — handler not wired", tradeCopy.ID)
			return
		}
		if err := closeHandler(ctx, sig, tradeCopy); err != nil {
			now := time.Now().UTC()
			b.updateTrade(tradeCopy.ID, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = err.Error()
				t.ClosedAt = &now
			})
			log.Printf("[DELTA BRIDGE] ❌ INSTITUTIONAL CLOSE FAILED %s — %v", tradeCopy.ID, err)
			return
		}
		log.Printf("[DELTA BRIDGE] ✅ INSTITUTIONAL CLOSE %s | %s | Exit: %s",
			tradeCopy.ID, tradeCopy.DeltaSymbol, sig.ExitReason)
	}()
}

// CloseAll flattens every open live trade, driving each through the institutional
// close path (risk gate + reduce-only). It runs synchronously and independently
// of the strategy loop, so the panic button works even if the loop is wedged.
// Reduce-only closes are permitted while the kill switch is active.
func (b *Bridge) CloseAll(ctx context.Context) (map[string]any, error) {
	b.mu.RLock()
	handler := b.institutionalClose
	open := make([]LiveTrade, 0)
	for _, t := range b.trades {
		if t.Status == "OPEN" {
			open = append(open, t)
		}
	}
	b.mu.RUnlock()

	if handler == nil {
		return nil, fmt.Errorf("institutional close handler not wired — cannot close-all")
	}

	closed, failed := 0, 0
	errs := make([]string, 0)
	for _, t := range open {
		sig := CloseSignal{
			PaperTradeID: t.PaperTradeID,
			StrategyID:   t.StrategyID,
			OptionType:   t.OptionType,
			Strike:       t.Strike,
			ExitReason:   "CLOSE_ALL",
		}
		if err := handler(ctx, sig, t); err != nil {
			failed++
			errs = append(errs, fmt.Sprintf("%s: %v", t.ID, err))
			continue
		}
		now := time.Now().UTC()
		b.updateTrade(t.ID, func(x *LiveTrade) {
			x.Status = "CLOSED"
			x.ClosedAt = &now
		})
		b.mu.Lock()
		delete(b.openByPaperID, t.PaperTradeID)
		b.mu.Unlock()
		closed++
	}

	res := map[string]any{"attempted": len(open), "closed": closed, "failed": failed}
	if len(errs) > 0 {
		res["errors"] = errs
	}
	return res, nil
}

// Trades returns a snapshot of all live trades (most recent first).
func (b *Bridge) Trades() []LiveTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LiveTrade, len(b.trades))
	copy(out, b.trades)
	return out
}

// OpenTrades returns only trades with Status == "OPEN".
func (b *Bridge) OpenTrades() []LiveTrade {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var out []LiveTrade
	for _, t := range b.trades {
		if t.Status == "OPEN" {
			out = append(out, t)
		}
	}
	return out
}

// AccountInfo bundles all live account data from Delta Exchange.
type AccountInfo struct {
	Wallets    []WalletEntry  `json:"wallets"`
	Positions  []LivePosition `json:"positions"`
	OpenOrders []OpenOrder    `json:"openOrders"`
	FetchedAt  string         `json:"fetchedAt"`
	Error      string         `json:"error,omitempty"`
}

// Stats returns aggregate stats.
type BridgeStats struct {
	Configured  bool    `json:"configured"`
	Testnet     bool    `json:"testnet"`
	Enabled     bool    `json:"enabled"`
	BuyingMode  bool    `json:"buyingMode"` // true = buying options, false = selling
	TotalTrades int     `json:"totalTrades"`
	OpenTrades  int     `json:"openTrades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	TotalPnl    float64 `json:"totalPnl"`
	WalletUSDT  float64 `json:"walletUsdt"`
	// Low-balance live BUY profile (buyingMode + wallet snapshot).
	LowBalanceProfile bool    `json:"lowBalanceProfile,omitempty"`
	BuyRiskPct        float64 `json:"buyRiskPct,omitempty"`
	BuyMaxContracts   int     `json:"buyMaxContracts,omitempty"`
	MinWalletUSD      float64 `json:"minWalletUsd,omitempty"`
	BuyEstPremiumUSD  float64 `json:"buyEstPremiumUsd,omitempty"`
	// Live account data
	Account *AccountInfo `json:"account,omitempty"`
}

func (b *Bridge) Stats(ctx context.Context) BridgeStats {
	b.mu.RLock()
	enabled := b.enabled
	configured := b.configured
	buyingMode := b.buyingMode
	trades := b.trades
	b.mu.RUnlock()

	s := BridgeStats{
		Configured:  configured,
		Testnet:     IsTestnet(),
		Enabled:     enabled,
		BuyingMode:  buyingMode,
		TotalTrades: len(trades),
	}
	for _, t := range trades {
		if t.Status == "OPEN" {
			s.OpenTrades++
		} else if t.Status == "CLOSED" {
			s.TotalPnl += t.RealizedPnl
			if t.RealizedPnl >= 0 {
				s.Wins++
			} else {
				s.Losses++
			}
		}
	}

	if configured && b.client != nil {
		info := &AccountInfo{FetchedAt: time.Now().UTC().Format(time.RFC3339)}
		wallets, err := b.client.GetWalletAll(ctx)
		if err != nil {
			info.Error = err.Error()
		} else {
			info.Wallets = wallets
			for _, w := range wallets {
				if w.Asset == "USDT" || w.Asset == "USD" {
					s.WalletUSDT = w.AvailableBalance
				}
			}
		}
		if buyingMode && s.WalletUSDT > 0 {
			risk, maxC, minW := TieredBuySizing(s.WalletUSDT)
			s.BuyRiskPct = risk
			s.BuyMaxContracts = maxC
			s.MinWalletUSD = minW
			s.BuyEstPremiumUSD = parseEnvFloat("DELTA_BUY_EST_PREMIUM_USD", 35)
			if s.BuyEstPremiumUSD < 5 {
				s.BuyEstPremiumUSD = 5
			}
			s.LowBalanceProfile = s.WalletUSDT < 100
		}
		if positions, err := b.client.GetPositions(ctx); err == nil {
			info.Positions = positions
		}
		if orders, err := b.client.GetOpenOrders(ctx); err == nil {
			info.OpenOrders = orders
		}
		s.Account = info
	}
	return s
}

// StartMonitor launches a background goroutine that polls live positions every 5 minutes
// and auto-closes bought positions at profit target (+80%), stop loss (-50%), or 30 min before expiry.
// Only active in buying mode. Call once after bridge is created.
func (b *Bridge) StartMonitor(ctx context.Context) {
	go func() {
		// SL/TP is enforced by polling, so the interval bounds how far price can
		// run past a stop before anything reacts. 5 minutes was far too slack for
		// a -50% stop on a volatile option; 30s is the practical floor without
		// hammering Delta's rate limits. Tune with LIVE_MONITOR_INTERVAL_SECONDS.
		ticker := time.NewTicker(monitorInterval())
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.monitorPositions(ctx)
			}
		}
	}()
}

func (b *Bridge) monitorPositions(ctx context.Context) {
	b.mu.RLock()
	// Custody rule: a position this app opened must be managed to SL/TP even when
	// the engine is DISARMED. Disarming stops NEW orders; it must never abandon an
	// open real position. Previously this returned early when !enabled, so
	// disarming (or an auto-disarm, or a restart) left real money unmanaged.
	if !b.configured || !b.buyingMode {
		b.mu.RUnlock()
		return
	}
	var openTrades []LiveTrade
	for _, t := range b.trades {
		if t.Status == "OPEN" {
			openTrades = append(openTrades, t)
		}
	}
	b.mu.RUnlock()

	if len(openTrades) == 0 {
		return
	}

	positions, err := b.client.GetPositions(ctx)
	if err != nil {
		log.Printf("[DELTA BRIDGE] Monitor: position fetch failed: %v", err)
		return
	}

	posMap := make(map[int]LivePosition)
	for _, p := range positions {
		posMap[p.ProductID] = p
	}

	now := time.Now().UTC()
	for _, trade := range openTrades {
		pos, ok := posMap[trade.ProductID]
		if !ok {
			if !trade.ExpiryTime.IsZero() && trade.ExpiryTime.Before(now) {
				b.updateTrade(trade.ID, func(t *LiveTrade) {
					t.Status = "CLOSED"
					nowCopy := now
					t.ClosedAt = &nowCopy
					// Value the expiry from the last mark we saw rather than
					// leaving it 0, which reported every expiry as flat P&L.
					t.CloseFillPrice = t.LastMarkPrice
					t.GrossPnl = (t.LastMarkPrice - t.FillPrice) * float64(t.Contracts) * OptionContractSizeBTC
					// An expiry has no closing trade, so there is no exit fee — but
					// the entry fee was still paid. That is why a worthless expiry
					// loses MORE than the premium: -100% of premium, minus the fee
					// that bought it.
					t.ExitFeeUSD = 0
					t.RealizedPnl = t.GrossPnl - t.EntryFeeUSD
					t.ExitReason = "expired"
					t.FailureReason = "expired"
				})
			}
			continue
		}

		if trade.FillPrice <= 0 {
			continue
		}
		// Remember the latest mark so an expiry close can be valued honestly, and
		// track the running peak the trailing exit measures against.
		peak := trade.PeakMarkPrice
		if pos.MarkPrice > peak {
			peak = pos.MarkPrice
		}
		peakCopy := peak
		b.updateTrade(trade.ID, func(t *LiveTrade) {
			t.LastMarkPrice = pos.MarkPrice
			t.PeakMarkPrice = peakCopy
		})

		ex := EvaluateExit(trade.FillPrice, pos.MarkPrice, peak)

		reason := ""
		switch {
		case ex.Reason != "":
			reason = ex.Reason
		case !trade.ExpiryTime.IsZero() && now.After(trade.ExpiryTime.Add(-30*time.Minute)):
			reason = "near_expiry_30min"
		}
		pnlPct := ex.GainPct

		if reason != "" {
			log.Printf("[DELTA BRIDGE] 🔔 Auto-close %s | %s | PnL: %.1f%%", trade.ID, reason, pnlPct*100)
			b.OnClose(CloseSignal{
				PaperTradeID: trade.PaperTradeID,
				ExitReason:   reason,
			})
		}
	}
}

// PlaceManualOrder is disabled — all manual orders must use POST /api/execution/request.
func (b *Bridge) PlaceManualOrder(ctx context.Context, symbol string, side OrderSide, size int) (PlaceOrderResult, error) {
	_ = ctx
	_ = symbol
	_ = side
	_ = size
	return PlaceOrderResult{}, fmt.Errorf("direct PlaceManualOrder disabled — use POST /api/execution/request with venue=delta")
}

// ---- helpers ----

func (b *Bridge) updateTrade(id string, fn func(*LiveTrade)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.trades {
		if b.trades[i].ID == id {
			fn(&b.trades[i])
			// Custody must survive a restart: persist on every state change so a
			// position opened moments before a crash is still managed afterwards.
			b.persistTradesLocked()
			return
		}
	}
}

func (b *Bridge) indexOfID(id string) int {
	for i, t := range b.trades {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// nextWeeklyFriday returns the nearest Friday at least minDays from now at 08:00 UTC.
// Weekly Friday expiries on Delta Exchange have the most liquidity.
func nextWeeklyFriday(from time.Time, minDays int) time.Time {
	t := from.Add(time.Duration(minDays) * 24 * time.Hour)
	for t.Weekday() != time.Friday {
		t = t.Add(24 * time.Hour)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 8, 0, 0, 0, time.UTC)
}

// roundBTCStrike rounds a price to the nearest $500 strike increment used by Delta Exchange.
func roundBTCStrike(price float64) float64 {
	return math.Round(price/500) * 500
}
