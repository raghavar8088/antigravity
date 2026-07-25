package delta

import (
	"context"
	"fmt"
	"log"
	"math"
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

	// openByPaperID maps paper position ID → live trade index for fast lookup on close.
	openByPaperID map[string]int

	// institutionalOpen, when set, replaces direct broker placement in OnOpen.
	institutionalOpen func(context.Context, OpenSignal, string) error

	// institutionalClose routes OnClose through the engine institutional execution stack.
	institutionalClose func(context.Context, CloseSignal, LiveTrade) error

	// killCheck blocks broker submission when the institutional kill switch is active.
	killCheck func(context.Context) error
}

// NewBridge creates a Bridge. If Delta keys are not set it starts in a disabled/unconfigured state.
func NewBridge() *Bridge {
	b := &Bridge{
		openByPaperID: make(map[string]int),
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
	return b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: productID,
		Size:      contracts,
		Side:      side,
		OrderType: TypeMarket,
		Leverage:  10,
	})
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
		if buying {
			t.RealizedPnl = (result.Price - t.FillPrice) * float64(t.Contracts)
		} else {
			t.RealizedPnl = (t.FillPrice - result.Price) * float64(t.Contracts)
		}
	})
}

// UpdateTradeAfterFill updates live trade state after institutional fill.
func (b *Bridge) UpdateTradeAfterFill(tradeID string, result PlaceOrderResult, productID int, contracts int, strike float64, expiry time.Time) {
	b.updateTrade(tradeID, func(t *LiveTrade) {
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
		b.openByPaperID[paperTradeID] = idx
	}
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

// OnOpen is called when the paper engine opens a new position.
// In selling mode: sells the option (requires large margin).
// In buying mode: buys the opposite option type (cheap premium, fits small balance).
func (b *Bridge) OnOpen(sig OpenSignal) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.enabled || !b.configured {
		return
	}

	b.seq++
	id := fmt.Sprintf("DLT-%04d", b.seq)
	buying := b.buyingMode

	// In buying mode, invert the option type:
	// Paper bull signal sells a PUT → we BUY a CALL (profit if BTC rises)
	// Paper bear signal sells a CALL → we BUY a PUT (profit if BTC falls)
	optType := sig.OptionType
	side := "sell"
	if buying {
		if sig.OptionType == "PUT" {
			optType = "CALL"
		} else {
			optType = "PUT"
		}
		side = "buy"
	}

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
}

// OnClose is called when the paper engine closes a position.
func (b *Bridge) OnClose(sig CloseSignal) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.enabled || !b.configured {
		return
	}

	idx, ok := b.openByPaperID[sig.PaperTradeID]
	if !ok || idx < 0 || idx >= len(b.trades) {
		return
	}
	trade := b.trades[idx]
	if trade.Status != "OPEN" {
		return
	}
	delete(b.openByPaperID, sig.PaperTradeID)

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
		ticker := time.NewTicker(5 * time.Minute)
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
	if !b.enabled || !b.configured || !b.buyingMode {
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
					t.FailureReason = "expired"
				})
			}
			continue
		}

		if trade.FillPrice <= 0 {
			continue
		}

		pnlPct := (pos.MarkPrice - trade.FillPrice) / trade.FillPrice

		reason := ""
		switch {
		case pnlPct >= 0.80:
			reason = "take_profit_80pct"
		case pnlPct <= -0.50:
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
