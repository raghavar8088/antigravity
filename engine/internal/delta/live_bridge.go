package delta

import (
	"context"
	"fmt"
	"log"
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

// Bridge mirrors paper BTC option selling trades to Delta Exchange live orders.
type Bridge struct {
	mu         sync.RWMutex
	client     *Client
	enabled    bool
	configured bool
	trades     []LiveTrade
	seq        int

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

// OnOpen is called when the paper engine opens a new sell position.
func (b *Bridge) OnOpen(sig OpenSignal) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.enabled || !b.configured {
		return
	}

	b.seq++
	id := fmt.Sprintf("DLT-%04d", b.seq)

	trade := LiveTrade{
		ID:           id,
		PaperTradeID: sig.PaperTradeID,
		StrategyID:   sig.StrategyID,
		StrategyName: sig.StrategyName,
		OptionType:   sig.OptionType,
		Strike:       sig.Strike,
		ExpiryTime:   sig.ExpiryTime,
		Side:         "sell",
		PremiumUSD:   sig.PremiumUSD,
		Status:       "OPEN",
		OpenedAt:     time.Now().UTC(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Find closest matching option contract on Delta Exchange
		info, err := b.client.FindOptionProduct(ctx, sig.Strike, sig.ExpiryTime, sig.OptionType)
		if err != nil {
			b.updateTrade(id, func(t *LiveTrade) {
				t.Status = "FAILED"
				t.FailureReason = fmt.Sprintf("product lookup failed: %v", err)
			})
			log.Printf("[DELTA BRIDGE] ❌ OPEN FAILED %s — %v", id, err)
			return
		}

		// Estimate contracts: $100 min per contract, use full PremiumUSD
		contracts := max1(int(sig.PremiumUSD / 100))

		result, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
			ProductID: info.ProductID,
			Size:      contracts,
			Side:      SideSell,
			OrderType: TypeMarket,
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
			t.Status = "OPEN"
		})
		b.mu.Lock()
		b.openByPaperID[sig.PaperTradeID] = b.indexOfID(id)
		b.mu.Unlock()

		log.Printf("[DELTA BRIDGE] ✅ SELL ORDER PLACED %s | %s | Strike: $%.0f | Contracts: %d | Fill: $%.4f",
			id, result.Symbol, sig.Strike, contracts, result.Price)
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

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// To close a short option sell position, we BUY to close
		result, err := b.client.PlaceOrder(ctx, PlaceOrderRequest{
			ProductID: trade.ProductID,
			Size:      trade.Contracts,
			Side:      SideBuy,
			OrderType: TypeMarket,
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

// Stats returns aggregate stats.
type BridgeStats struct {
	Configured  bool    `json:"configured"`
	Testnet     bool    `json:"testnet"`
	Enabled     bool    `json:"enabled"`
	TotalTrades int     `json:"totalTrades"`
	OpenTrades  int     `json:"openTrades"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	TotalPnl    float64 `json:"totalPnl"`
	WalletUSDT  float64 `json:"walletUsdt"`
}

func (b *Bridge) Stats(ctx context.Context) BridgeStats {
	b.mu.RLock()
	enabled := b.enabled
	configured := b.configured
	trades := b.trades
	b.mu.RUnlock()

	s := BridgeStats{
		Configured:  configured,
		Testnet:     IsTestnet(),
		Enabled:     enabled,
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

	if configured && enabled && b.client != nil {
		if wallet, err := b.client.GetWallet(ctx); err == nil {
			s.WalletUSDT = wallet
		}
	}
	return s
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
