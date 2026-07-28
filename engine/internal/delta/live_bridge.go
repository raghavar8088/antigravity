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
	ID            string    `json:"id"`
	PaperTradeID  string    `json:"paperTradeId"`
	StrategyID    int       `json:"strategyId"`
	StrategyName  string    `json:"strategyName"`
	OptionType    string    `json:"optionType"` // "CALL" or "PUT"
	Strike        float64   `json:"strike"`
	ExpiryTime    time.Time `json:"expiryTime"`
	Side          string    `json:"side"`    // "sell" (we sell options) or "buy" (to close)
	DeltaOrderID  string    `json:"deltaOrderId"`
	DeltaSymbol   string    `json:"deltaSymbol"`
	ProductID     int       `json:"productId"`
	Contracts     int       `json:"contracts"`
	FillPrice     float64   `json:"fillPrice"`
	PremiumUSD    float64   `json:"premiumUsd"`
	Status        string    `json:"status"` // "OPEN","CLOSED","FAILED","CANCELLED"
	OpenedAt      time.Time `json:"openedAt"`
	ClosedAt      *time.Time `json:"closedAt,omitempty"`
	CloseOrderID  string    `json:"closeOrderId,omitempty"`
	CloseFillPrice float64  `json:"closeFillPrice,omitempty"`
	RealizedPnl   float64   `json:"realizedPnl,omitempty"`
	FailureReason string    `json:"failureReason,omitempty"`
	// LastMarkPrice is the most recent mark seen by the monitor. Used to value an
	// expiry close, which has no fill price of its own — without it an expired
	// position reported $0 P&L even though the premium was really lost or won.
	LastMarkPrice float64 `json:"lastMarkPrice,omitempty"`
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
	OptionType   string  // "CALL" or "PUT"
	Strike       float64
	ExpiryTime   time.Time
	PremiumUSD   float64
	BTCPrice     float64
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
	mu          sync.RWMutex
	client      *Client
	enabled     bool
	configured  bool
	buyingMode  bool // when true: BUY options instead of SELL
	trades      []LiveTrade
	seq         int

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
			t.RealizedPnl = (result.Price - t.FillPrice) * btc
		} else {
			t.RealizedPnl = (t.FillPrice - result.Price) * btc
		}
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
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.enabled || !b.configured {
		return
	}

	// Live allow-list: only explicitly enabled strategies may place live orders.
	if !b.strategyAllowedLocked(sig.StrategyName) {
		log.Printf("[DELTA BRIDGE] ⏭️  skip live open — strategy %q not in live allow-list", sig.StrategyName)
		return
	}

	b.seq++
	id := fmt.Sprintf("DLT-%04d", b.seq)

	optType, side := resolveLiveOptionSide(sig.OptionType, b.buyingMode, b.nativeBuy)

	trade := LiveTrade{
		ID:           id,
		PaperTradeID: sig.PaperTradeID,
		StrategyID:   sig.StrategyID,
		StrategyName: sig.StrategyName,
		OptionType:   optType,
		Strike:       sig.Strike,
		ExpiryTime:   sig.ExpiryTime,
		Side:         side,
		PremiumUSD:   sig.PremiumUSD,
		Status:       "OPEN",
		OpenedAt:     time.Now().UTC(),
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
	LowBalanceProfile   bool    `json:"lowBalanceProfile,omitempty"`
	BuyRiskPct          float64 `json:"buyRiskPct,omitempty"`
	BuyMaxContracts     int     `json:"buyMaxContracts,omitempty"`
	MinWalletUSD        float64 `json:"minWalletUsd,omitempty"`
	BuyEstPremiumUSD    float64 `json:"buyEstPremiumUsd,omitempty"`
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
					t.RealizedPnl = (t.LastMarkPrice - t.FillPrice) * float64(t.Contracts) * OptionContractSizeBTC
					t.ExitReason = "expired"
					t.FailureReason = "expired"
				})
			}
			continue
		}

		if trade.FillPrice <= 0 {
			continue
		}
		// Remember the latest mark so an expiry close can be valued honestly.
		b.updateTrade(trade.ID, func(t *LiveTrade) { t.LastMarkPrice = pos.MarkPrice })

		pnlPct := (pos.MarkPrice - trade.FillPrice) / trade.FillPrice

		reason := ""
		switch {
		case pnlPct >= LiveTakeProfitPct:
			reason = "take_profit_80pct"
		case pnlPct <= -LiveStopLossPct:
			reason = "stop_loss_50pct"
		case !trade.ExpiryTime.IsZero() && now.After(trade.ExpiryTime.Add(-30*time.Minute)):
			reason = "near_expiry_30min"
		}

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
