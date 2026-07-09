// Package livemirror is the LIVE ENGINE: it clones every trade the Pre-Live
// Trade Engine fires into real orders on Delta Exchange BTC perpetual futures.
//
// Flow:
//
//	pre-live positions.Manager OnOpen  ──► Mirror.OnPaperOpen  ──► queue ──► worker ──► Delta market order (entry)
//	pre-live positions.Manager OnClose ──► Mirror.OnPaperClose ──► queue ──► worker ──► Delta reduce-only order (exit)
//
// Design constraints:
//   - The pre-live tick path must never block on broker I/O: OnPaperOpen /
//     OnPaperClose only enqueue events; a single worker goroutine talks to Delta.
//   - Real money: the mirror starts DISABLED and must be armed explicitly via
//     the API (or LIVE_ENGINE_AUTO_ENABLE=true for headless restarts).
//   - Every order passes the kill-switch check first when one is wired.
//   - Per-order contract cap (LIVE_ENGINE_MAX_CONTRACTS) bounds worst-case size.
//
// Environment variables:
//
//	DELTA_API_KEY / DELTA_API_SECRET — broker credentials (required)
//	DELTA_TESTNET=true               — use Delta testnet
//	LIVE_ENGINE_SYMBOL               — perp symbol to trade (default "BTCUSD")
//	LIVE_ENGINE_AUTO_ENABLE=true     — arm mirroring on boot (default: disarmed)
//	LIVE_ENGINE_FIXED_CONTRACTS      — if >0, every order uses exactly this many contracts
//	LIVE_ENGINE_MAX_CONTRACTS        — per-order cap (default 5; BTCUSD perp contract = 0.001 BTC)
//	LIVE_ENGINE_LEVERAGE             — order leverage (default 10, matching the pre-live engine)
package livemirror

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"antigravity-engine/internal/delta"
	"antigravity-engine/internal/positions"
)

const (
	maxRecords     = 1000
	queueSize      = 512
	orderTimeout   = 20 * time.Second
	productCacheTTL = 30 * time.Minute
)

// TradeStatus is the lifecycle state of a mirrored trade.
type TradeStatus string

const (
	StatusPending TradeStatus = "PENDING" // queued, broker order not yet placed
	StatusOpen    TradeStatus = "OPEN"    // entry filled on Delta
	StatusClosed  TradeStatus = "CLOSED"  // exit filled on Delta
	StatusFailed  TradeStatus = "FAILED"  // broker rejected / error
)

// Trade is one pre-live position cloned onto Delta Exchange.
type Trade struct {
	ID              string      `json:"id"`
	PaperPositionID string      `json:"paperPositionId"`
	StrategyName    string      `json:"strategyName"`
	Side            string      `json:"side"` // "LONG" or "SHORT"
	Symbol          string      `json:"symbol"`
	ProductID       int         `json:"productId"`
	Contracts       int         `json:"contracts"`
	ContractValue   float64     `json:"contractValue"` // BTC per contract
	PaperSizeBTC    float64     `json:"paperSizeBtc"`
	PaperEntryPrice float64     `json:"paperEntryPrice"`
	PaperExitPrice  float64     `json:"paperExitPrice,omitempty"`
	EntryOrderID    string      `json:"entryOrderId,omitempty"`
	EntryPrice      float64     `json:"entryPrice,omitempty"` // Delta average fill
	ExitOrderID     string      `json:"exitOrderId,omitempty"`
	ExitPrice       float64     `json:"exitPrice,omitempty"`
	CloseReason     string      `json:"closeReason,omitempty"`
	Status          TradeStatus `json:"status"`
	FailureReason   string      `json:"failureReason,omitempty"`
	OpenedAt        time.Time   `json:"openedAt"`
	ClosedAt        *time.Time  `json:"closedAt,omitempty"`
	// RealizedPnlUSD is estimated from Delta fills: (exit-entry) × contracts ×
	// contractValue, sign-adjusted for side. Fees not included.
	RealizedPnlUSD float64 `json:"realizedPnlUsd,omitempty"`
}

// Config controls sizing and symbol selection.
type Config struct {
	Symbol         string
	FixedContracts int // >0 → constant order size
	MaxContracts   int // hard per-order cap
	Leverage       int
	AutoEnable     bool
}

// ConfigFromEnv builds the mirror config from LIVE_ENGINE_* environment variables.
func ConfigFromEnv() Config {
	cfg := Config{
		Symbol:       "BTCUSD",
		MaxContracts: 5,
		Leverage:     10,
	}
	if v := strings.TrimSpace(os.Getenv("LIVE_ENGINE_SYMBOL")); v != "" {
		cfg.Symbol = strings.ToUpper(v)
	}
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("LIVE_ENGINE_FIXED_CONTRACTS"))); err == nil && n > 0 {
		cfg.FixedContracts = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("LIVE_ENGINE_MAX_CONTRACTS"))); err == nil && n > 0 {
		cfg.MaxContracts = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv("LIVE_ENGINE_LEVERAGE"))); err == nil && n > 0 {
		cfg.Leverage = n
	}
	cfg.AutoEnable = strings.TrimSpace(os.Getenv("LIVE_ENGINE_AUTO_ENABLE")) == "true"
	return cfg
}

type eventKind int

const (
	evOpen eventKind = iota
	evClose
)

type event struct {
	kind      eventKind
	pos       positions.Position
	reason    string
	exitPrice float64
}

// Mirror clones pre-live paper positions to live Delta Exchange orders.
type Mirror struct {
	mu         sync.RWMutex
	cfg        Config
	client     *delta.Client
	configured bool
	enabled    bool
	trades     []Trade
	byPaperID  map[string]string // paper position ID → mirror trade ID
	seq        int
	killCheck  func(context.Context) error

	// product cache
	product      delta.PerpProductInfo
	productAt    time.Time

	// counters
	skippedOpens  int
	droppedEvents int

	events chan event
}

// New creates the mirror. Missing Delta keys → unconfigured (all events ignored).
func New(cfg Config) *Mirror {
	m := &Mirror{
		cfg:       cfg,
		byPaperID: make(map[string]string),
		events:    make(chan event, queueSize),
	}
	client, err := delta.NewClient()
	if err != nil {
		log.Printf("[LIVE ENGINE] not configured: %v — mirroring unavailable", err)
	} else {
		m.client = client
		m.configured = true
		m.enabled = cfg.AutoEnable
		state := "DISARMED — enable via POST /api/live/enable"
		if m.enabled {
			state = "ARMED via LIVE_ENGINE_AUTO_ENABLE"
		}
		log.Printf("[LIVE ENGINE] configured (testnet=%v symbol=%s maxContracts=%d leverage=%dx) — %s",
			delta.IsTestnet(), cfg.Symbol, cfg.MaxContracts, cfg.Leverage, state)
	}
	return m
}

// Start launches the order worker. Call once.
func (m *Mirror) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-m.events:
				switch ev.kind {
				case evOpen:
					m.handleOpen(ctx, ev)
				case evClose:
					m.handleClose(ctx, ev)
				}
			}
		}
	}()
}

// SetKillCheck wires the institutional kill switch. When fn returns an error,
// no broker order is placed.
func (m *Mirror) SetKillCheck(fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.killCheck = fn
}

// IsConfigured reports whether Delta API keys are present.
func (m *Mirror) IsConfigured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configured
}

// IsEnabled reports whether live cloning is armed.
func (m *Mirror) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled arms or disarms live cloning. Returns an error when arming without keys.
func (m *Mirror) SetEnabled(v bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v && !m.configured {
		return fmt.Errorf("cannot enable live engine: DELTA_API_KEY/DELTA_API_SECRET not configured")
	}
	m.enabled = v
	if v {
		log.Printf("[LIVE ENGINE] ✅ ARMED — pre-live trades will be cloned to Delta Exchange (%s)", m.cfg.Symbol)
	} else {
		log.Printf("[LIVE ENGINE] ⏸️  DISARMED — no new live orders (open live positions stay open)")
	}
	return nil
}

// Config returns the active mirror configuration.
func (m *Mirror) Config() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// OnPaperOpen enqueues a pre-live position open for live cloning.
// Called from the positions.Manager OnOpen hook — must return immediately.
func (m *Mirror) OnPaperOpen(pos positions.Position) {
	if !m.IsEnabled() {
		return
	}
	select {
	case m.events <- event{kind: evOpen, pos: pos}:
	default:
		m.mu.Lock()
		m.droppedEvents++
		m.mu.Unlock()
		log.Printf("[LIVE ENGINE] ⚠️ event queue full — OPEN %s (%s) NOT mirrored", pos.ID, pos.StrategyName)
	}
}

// OnPaperClose enqueues a pre-live position close. Closes are enqueued even
// while disarmed so positions opened during an armed period still get flattened.
func (m *Mirror) OnPaperClose(pos positions.Position, reason string, exitPrice float64) {
	if !m.IsConfigured() {
		return
	}
	// Fast path: nothing mirrored for this paper position → nothing to close.
	m.mu.RLock()
	_, tracked := m.byPaperID[pos.ID]
	m.mu.RUnlock()
	if !tracked {
		return
	}
	select {
	case m.events <- event{kind: evClose, pos: pos, reason: reason, exitPrice: exitPrice}:
	default:
		m.mu.Lock()
		m.droppedEvents++
		m.mu.Unlock()
		log.Printf("[LIVE ENGINE] ⚠️ event queue full — CLOSE %s NOT mirrored (position may stay open on Delta!)", pos.ID)
	}
}

// resolveProduct returns the cached perp product, refreshing every productCacheTTL.
func (m *Mirror) resolveProduct(ctx context.Context) (delta.PerpProductInfo, error) {
	m.mu.RLock()
	p, at := m.product, m.productAt
	m.mu.RUnlock()
	if p.ProductID != 0 && time.Since(at) < productCacheTTL {
		return p, nil
	}
	info, err := m.client.FindPerpProduct(ctx, m.cfg.Symbol)
	if err != nil {
		if p.ProductID != 0 {
			return p, nil // stale cache beats hard failure
		}
		return delta.PerpProductInfo{}, err
	}
	m.mu.Lock()
	m.product = info
	m.productAt = time.Now()
	m.mu.Unlock()
	log.Printf("[LIVE ENGINE] product resolved: %s id=%d contract=%g %s",
		info.Symbol, info.ProductID, info.ContractValue, info.ContractUnit)
	return info, nil
}

// contractsFor converts a paper size in BTC to Delta contracts, honouring the
// fixed-size override and the per-order cap.
func (m *Mirror) contractsFor(paperSizeBTC, contractValue float64) int {
	if m.cfg.FixedContracts > 0 {
		return minInt(m.cfg.FixedContracts, m.cfg.MaxContracts)
	}
	if contractValue <= 0 {
		return 0
	}
	n := int(math.Round(paperSizeBTC / contractValue))
	if n < 1 {
		n = 1
	}
	return minInt(n, m.cfg.MaxContracts)
}

func (m *Mirror) handleOpen(ctx context.Context, ev event) {
	if !m.IsEnabled() {
		return
	}
	if kc := m.killCheckFn(); kc != nil {
		if err := kc(ctx); err != nil {
			m.mu.Lock()
			m.skippedOpens++
			m.mu.Unlock()
			log.Printf("[LIVE ENGINE] open %s blocked by kill switch: %v", ev.pos.ID, err)
			return
		}
	}

	octx, cancel := context.WithTimeout(ctx, orderTimeout)
	defer cancel()

	product, err := m.resolveProduct(octx)
	if err != nil {
		log.Printf("[LIVE ENGINE] ❌ open %s: product resolution failed: %v", ev.pos.ID, err)
		m.recordFailedOpen(ev.pos, "product resolution failed: "+err.Error())
		return
	}

	contracts := m.contractsFor(ev.pos.Size, product.ContractValue)
	if contracts <= 0 {
		m.recordFailedOpen(ev.pos, "computed 0 contracts")
		return
	}

	side := delta.SideBuy
	dir := "LONG"
	if ev.pos.Side == "SELL" {
		side = delta.SideSell
		dir = "SHORT"
	}

	m.mu.Lock()
	m.seq++
	id := fmt.Sprintf("LIVE-%04d", m.seq)
	tr := Trade{
		ID:              id,
		PaperPositionID: ev.pos.ID,
		StrategyName:    ev.pos.StrategyName,
		Side:            dir,
		Symbol:          product.Symbol,
		ProductID:       product.ProductID,
		Contracts:       contracts,
		ContractValue:   product.ContractValue,
		PaperSizeBTC:    ev.pos.Size,
		PaperEntryPrice: ev.pos.EntryPrice,
		Status:          StatusPending,
		OpenedAt:        time.Now().UTC(),
	}
	m.prependTrade(tr)
	m.byPaperID[ev.pos.ID] = id
	m.mu.Unlock()

	res, err := m.client.PlaceOrder(octx, delta.PlaceOrderRequest{
		ProductID: product.ProductID,
		Size:      contracts,
		Side:      side,
		OrderType: delta.TypeMarket,
		Leverage:  m.cfg.Leverage,
	})
	if err != nil {
		m.updateTrade(id, func(t *Trade) {
			t.Status = StatusFailed
			t.FailureReason = err.Error()
		})
		m.mu.Lock()
		delete(m.byPaperID, ev.pos.ID)
		m.mu.Unlock()
		log.Printf("[LIVE ENGINE] ❌ OPEN FAILED %s (%s %d contracts %s): %v", id, dir, contracts, product.Symbol, err)
		return
	}
	m.updateTrade(id, func(t *Trade) {
		t.Status = StatusOpen
		t.EntryOrderID = res.OrderID
		t.EntryPrice = res.Price
	})
	log.Printf("[LIVE ENGINE] ✅ OPENED %s | %s %d×%s @ ~$%.2f | strategy=%s paper=%s",
		id, dir, contracts, product.Symbol, res.Price, ev.pos.StrategyName, ev.pos.ID)
}

func (m *Mirror) handleClose(ctx context.Context, ev event) {
	m.mu.Lock()
	id, ok := m.byPaperID[ev.pos.ID]
	if ok {
		delete(m.byPaperID, ev.pos.ID)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	tr, found := m.tradeByID(id)
	if !found || tr.Status != StatusOpen {
		return
	}

	octx, cancel := context.WithTimeout(ctx, orderTimeout)
	defer cancel()

	side := delta.SideSell // closing a LONG
	if tr.Side == "SHORT" {
		side = delta.SideBuy
	}
	res, err := m.client.PlaceOrder(octx, delta.PlaceOrderRequest{
		ProductID:            tr.ProductID,
		Size:                 tr.Contracts,
		Side:                 side,
		OrderType:            delta.TypeMarket,
		Leverage:             m.cfg.Leverage,
		ReduceOnly:           true,
		CancelOrdersAccepted: "true",
	})
	now := time.Now().UTC()
	if err != nil {
		m.updateTrade(id, func(t *Trade) {
			t.Status = StatusFailed
			t.FailureReason = "close failed: " + err.Error()
			t.CloseReason = ev.reason
			t.PaperExitPrice = ev.exitPrice
			t.ClosedAt = &now
		})
		log.Printf("[LIVE ENGINE] ❌ CLOSE FAILED %s (%s) — POSITION MAY STILL BE OPEN ON DELTA: %v", id, tr.Symbol, err)
		return
	}
	m.updateTrade(id, func(t *Trade) {
		t.Status = StatusClosed
		t.ExitOrderID = res.OrderID
		t.ExitPrice = res.Price
		t.CloseReason = ev.reason
		t.PaperExitPrice = ev.exitPrice
		t.ClosedAt = &now
		if t.EntryPrice > 0 && res.Price > 0 {
			pnl := (res.Price - t.EntryPrice) * float64(t.Contracts) * t.ContractValue
			if t.Side == "SHORT" {
				pnl = -pnl
			}
			t.RealizedPnlUSD = pnl
		}
	})
	log.Printf("[LIVE ENGINE] ✅ CLOSED %s | %s | reason=%s exit≈$%.2f", id, tr.Symbol, ev.reason, res.Price)
}

// CloseAll flattens every open mirrored trade AND any residual position on the
// configured symbol (covers orphans from restarts). Returns per-action results.
func (m *Mirror) CloseAll(ctx context.Context) map[string]interface{} {
	out := map[string]interface{}{"closedTrades": 0, "flattenedResidual": false}
	if !m.IsConfigured() {
		out["error"] = "not configured"
		return out
	}

	// 1) Close all tracked OPEN trades.
	m.mu.RLock()
	var open []Trade
	for _, t := range m.trades {
		if t.Status == StatusOpen {
			open = append(open, t)
		}
	}
	m.mu.RUnlock()

	closed := 0
	var errs []string
	for _, tr := range open {
		m.handleClose(ctx, event{
			kind:      evClose,
			pos:       positions.Position{ID: tr.PaperPositionID},
			reason:    "MANUAL_CLOSE_ALL",
			exitPrice: 0,
		})
		closed++
	}
	out["closedTrades"] = closed

	// 2) Flatten any residual net position on the symbol (orphans from restarts).
	product, err := m.resolveProduct(ctx)
	if err != nil {
		errs = append(errs, "product: "+err.Error())
	} else {
		positionsLive, err := m.client.GetPositions(ctx)
		if err != nil {
			errs = append(errs, "positions: "+err.Error())
		} else {
			for _, p := range positionsLive {
				if p.ProductID != product.ProductID || p.Size == 0 {
					continue
				}
				side := delta.SideSell
				size := int(math.Abs(p.Size))
				if p.Size < 0 {
					side = delta.SideBuy
				}
				if size == 0 {
					continue
				}
				_, err := m.client.PlaceOrder(ctx, delta.PlaceOrderRequest{
					ProductID:            product.ProductID,
					Size:                 size,
					Side:                 side,
					OrderType:            delta.TypeMarket,
					Leverage:             m.cfg.Leverage,
					ReduceOnly:           true,
					CancelOrdersAccepted: "true",
				})
				if err != nil {
					errs = append(errs, "flatten: "+err.Error())
				} else {
					out["flattenedResidual"] = true
					log.Printf("[LIVE ENGINE] 🧹 flattened residual %s position (%d contracts)", p.Symbol, size)
				}
			}
		}
	}
	if len(errs) > 0 {
		out["errors"] = errs
	}
	return out
}

// Trades returns a snapshot of mirrored trades, newest first.
func (m *Mirror) Trades() []Trade {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Trade, len(m.trades))
	copy(out, m.trades)
	return out
}

// Stats is the live engine status snapshot for the UI.
type Stats struct {
	Configured    bool    `json:"configured"`
	Testnet       bool    `json:"testnet"`
	Enabled       bool    `json:"enabled"`
	Symbol        string  `json:"symbol"`
	ProductID     int     `json:"productId,omitempty"`
	ContractValue float64 `json:"contractValue,omitempty"`
	MaxContracts  int     `json:"maxContracts"`
	FixedContracts int    `json:"fixedContracts,omitempty"`
	Leverage      int     `json:"leverage"`
	TotalTrades   int     `json:"totalTrades"`
	OpenTrades    int     `json:"openTrades"`
	ClosedTrades  int     `json:"closedTrades"`
	FailedTrades  int     `json:"failedTrades"`
	Wins          int     `json:"wins"`
	Losses        int     `json:"losses"`
	RealizedPnlUSD float64 `json:"realizedPnlUsd"`
	SkippedOpens  int     `json:"skippedOpens"`
	DroppedEvents int     `json:"droppedEvents"`
	// Live account data straight from Delta Exchange.
	WalletUSD     float64              `json:"walletUsd"`
	Wallets       []delta.WalletEntry  `json:"wallets,omitempty"`
	LivePositions []delta.LivePosition `json:"livePositions,omitempty"`
	OpenOrders    []delta.OpenOrder    `json:"openOrders,omitempty"`
	AccountError  string               `json:"accountError,omitempty"`
	FetchedAt     string               `json:"fetchedAt"`
}

// GetStats builds the status snapshot, including live wallet/positions when configured.
func (m *Mirror) GetStats(ctx context.Context) Stats {
	m.mu.RLock()
	s := Stats{
		Configured:     m.configured,
		Testnet:        delta.IsTestnet(),
		Enabled:        m.enabled,
		Symbol:         m.cfg.Symbol,
		ProductID:      m.product.ProductID,
		ContractValue:  m.product.ContractValue,
		MaxContracts:   m.cfg.MaxContracts,
		FixedContracts: m.cfg.FixedContracts,
		Leverage:       m.cfg.Leverage,
		SkippedOpens:   m.skippedOpens,
		DroppedEvents:  m.droppedEvents,
		FetchedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	for _, t := range m.trades {
		s.TotalTrades++
		switch t.Status {
		case StatusOpen, StatusPending:
			s.OpenTrades++
		case StatusClosed:
			s.ClosedTrades++
			s.RealizedPnlUSD += t.RealizedPnlUSD
			if t.RealizedPnlUSD >= 0 {
				s.Wins++
			} else {
				s.Losses++
			}
		case StatusFailed:
			s.FailedTrades++
		}
	}
	client, configured := m.client, m.configured
	m.mu.RUnlock()

	if configured && client != nil {
		if wallets, err := client.GetWalletAll(ctx); err != nil {
			s.AccountError = err.Error()
		} else {
			s.Wallets = wallets
			for _, w := range wallets {
				if w.Asset == "USDT" || w.Asset == "USD" {
					s.WalletUSD = w.AvailableBalance
				}
			}
		}
		if pos, err := client.GetPositions(ctx); err == nil {
			s.LivePositions = pos
		}
		if orders, err := client.GetOpenOrders(ctx); err == nil {
			s.OpenOrders = orders
		}
	}
	return s
}

// ---- helpers ----

func (m *Mirror) killCheckFn() func(context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.killCheck
}

func (m *Mirror) recordFailedOpen(pos positions.Position, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	m.prependTrade(Trade{
		ID:              fmt.Sprintf("LIVE-%04d", m.seq),
		PaperPositionID: pos.ID,
		StrategyName:    pos.StrategyName,
		Side:            dirOf(string(pos.Side)),
		Symbol:          m.cfg.Symbol,
		PaperSizeBTC:    pos.Size,
		PaperEntryPrice: pos.EntryPrice,
		Status:          StatusFailed,
		FailureReason:   reason,
		OpenedAt:        time.Now().UTC(),
	})
}

// prependTrade must be called with m.mu held.
func (m *Mirror) prependTrade(t Trade) {
	m.trades = append([]Trade{t}, m.trades...)
	if len(m.trades) > maxRecords {
		m.trades = m.trades[:maxRecords]
	}
}

func (m *Mirror) tradeByID(id string) (Trade, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, t := range m.trades {
		if t.ID == id {
			return t, true
		}
	}
	return Trade{}, false
}

func (m *Mirror) updateTrade(id string, fn func(*Trade)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.trades {
		if m.trades[i].ID == id {
			fn(&m.trades[i])
			return
		}
	}
}

func dirOf(side string) string {
	if side == "SELL" {
		return "SHORT"
	}
	return "LONG"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
