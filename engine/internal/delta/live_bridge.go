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

// IsConfigured returns whether API keys are present.
func (b *Bridge) IsConfigured() bool { return b.configured }

// IsEnabled returns whether live order mirroring is active.
func (b *Bridge) IsEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enabled
}

// SetBuyingMode switches between buying mode (BUY options, costs ~$0.50/lot) and
// selling mode (SELL options, requires ~$800+ margin). Buying mode is the only viable
// mode when account balance is under ~$50.
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

// IsBuyingMode returns true when the bridge is configured to buy options.
func (b *Bridge) IsBuyingMode() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.buyingMode
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

		strike := sig.Strike
		expiry := sig.ExpiryTime

		if buying {
			// For buying: use 0.7% OTM for better delta and responsiveness.
			// Paper strategies use 1.0-1.8% OTM which is good for selling but
			// gives low delta (~0.20) when buying — override to 0.7% OTM (~0.35 delta).
			if optType == "CALL" {
				strike = roundBTCStrike(sig.BTCPrice * 1.007)
			} else {
				strike = roundBTCStrike(sig.BTCPrice * 0.993)
			}
			// Use next weekly Friday at least 5 days out.
			// Paper signals use 2-3.5h expiry which causes crushing theta decay
			// for buyers — weekly expiry dramatically reduces theta risk.
			expiry = nextWeeklyFriday(time.Now().UTC(), 5)
		}

		info, err := b.client.FindOptionProduct(ctx, strike, expiry, optType)
		if err != nil {
			b.updateTrade(id, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = fmt.Sprintf("product lookup failed: %v", err)
			})
			log.Printf("[DELTA BRIDGE] ❌ OPEN FAILED %s — %v", id, err)
			return
		}

		// Position sizing:
		// Selling: $100/contract minimum (large margin required anyway)
		// Buying: risk 15% of available balance, min 1 lot, max 5 lots
		contracts := max1(int(sig.PremiumUSD / 100))
		if buying {
			if bal, err2 := b.client.GetWallet(ctx); err2 == nil && bal > 0 && sig.PremiumUSD > 0 {
				riskAmt := bal * 0.15
				contracts = max1(int(riskAmt / sig.PremiumUSD))
				if contracts > 5 {
					contracts = 5
				}
			} else {
				contracts = 1
			}
		}

		orderSide := SideSell
		if buying {
			orderSide = SideBuy
		}

		result, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
			ProductID: info.ProductID,
			Size:      contracts,
			Side:      orderSide,
			OrderType: TypeMarket,
			Leverage:  10,
		})
		if err != nil {
			b.updateTrade(id, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = fmt.Sprintf("order placement failed: %v", err)
			})
			log.Printf("[DELTA BRIDGE] ❌ ORDER FAILED %s — %v", id, err)
			return
		}

		b.updateTrade(id, func(t *LiveTrade) {
			t.DeltaOrderID = result.OrderID
			t.DeltaSymbol = result.Symbol
			t.ProductID = info.ProductID
			t.Contracts = contracts
			t.FillPrice = result.Price
			t.Strike = strike
			t.ExpiryTime = expiry
			t.Status = "OPEN"
		})
		b.mu.Lock()
		b.openByPaperID[sig.PaperTradeID] = b.indexOfID(id)
		b.mu.Unlock()

		action := "SELL"
		if buying {
			action = "BUY"
		}
		log.Printf("[DELTA BRIDGE] ✅ %s ORDER PLACED %s | %s | Strike: $%.0f | Contracts: %d | Fill: $%.4f",
			action, id, result.Symbol, strike, contracts, result.Price)
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

	buying := b.buyingMode

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Close side is the opposite of open side:
		// Selling mode opened SHORT → close with BUY
		// Buying mode opened LONG → close with SELL + ReduceOnly
		closeSide := SideBuy
		if buying {
			closeSide = SideSell
		}

		result, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
			ProductID:            trade.ProductID,
			Size:                 trade.Contracts,
			Side:                 closeSide,
			OrderType:            TypeMarket,
			Leverage:             10,
			ReduceOnly:           true,
			CancelOrdersAccepted: "true",
		})
		now := time.Now().UTC()
		if err != nil {
			b.updateTrade(trade.ID, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = fmt.Sprintf("close order failed: %v", err)
				t.ClosedAt = &now
			})
			log.Printf("[DELTA BRIDGE] ❌ CLOSE FAILED %s — %v", trade.ID, err)
			return
		}

		b.updateTrade(trade.ID, func(t *LiveTrade) {
			t.Status = "CLOSED"
			t.CloseOrderID = result.OrderID
			t.CloseFillPrice = result.Price
			t.ClosedAt = &now
			// PnL for a short option: sell premium - buy-back premium
			t.RealizedPnl = (t.FillPrice - result.Price) * float64(t.Contracts)
		})
		log.Printf("[DELTA BRIDGE] ✅ CLOSE ORDER PLACED %s | %s | Fill: $%.4f | Exit: %s",
			trade.ID, trade.DeltaSymbol, result.Price, sig.ExitReason)
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
	BuyingMode  bool    `json:"buyingMode"`  // true = buying options, false = selling
	TotalTrades int     `json:"totalTrades"`
	OpenTrades  int     `json:"openTrades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	TotalPnl    float64 `json:"totalPnl"`
	WalletUSDT  float64 `json:"walletUsdt"`
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

// PlaceManualOrder places a real order for any product by symbol. Useful for manual/test trades.
func (b *Bridge) PlaceManualOrder(ctx context.Context, symbol string, side OrderSide, size int) (PlaceOrderResult, error) {
	if !b.configured || b.client == nil {
		return PlaceOrderResult{}, fmt.Errorf("bridge not configured")
	}
	info, err := b.client.FindProductBySymbol(ctx, symbol)
	if err != nil {
		return PlaceOrderResult{}, fmt.Errorf("symbol lookup: %w", err)
	}
	return b.client.PlaceOrder(ctx, PlaceOrderRequest{
		ProductID: info.ProductID,
		Size:      size,
		Side:      side,
		OrderType: TypeMarket,
		Leverage:  10,
	})
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
