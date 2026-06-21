package marketdata

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Binance USD-M futures liquidation order stream (public, no auth required).
// Verified via web search (2026): wss://fstream.binance.com/ws/!forceOrder@arr
// streams ALL force-liquidation orders across all symbols on Binance Futures.
// Each event has the shape:
//
//	{"e":"forceOrder","E":<eventTime>,"o":{
//	   "s":"BTCUSDT","S":"SELL","o":"LIMIT","f":"IOC","q":"0.014",
//	   "p":"9910","ap":"9910","X":"FILLED","l":"0.014","z":"0.014","T":<tradeTime>}}
//
// The "o" (order) object's S field is the SIDE OF THE LIQUIDATION ORDER itself,
// which is the opposite of the liquidated trader's position:
//
//	S="SELL" forceOrder => exchange is selling to close a LONG position that got
//	  liquidated => this is a LONG liquidation.
//	S="BUY"  forceOrder => exchange is buying to close a SHORT position that got
//	  liquidated => this is a SHORT liquidation.
//
// This matches Binance API docs ("Liquidation Order Streams") and is consistent
// with community references (e.g. binance-futures-connector-python forceOrder
// examples, and public liquidation-heatmap dashboards like coinglass.com which
// derive long/short liquidation totals the same way).
const (
	binanceLiquidationWSURL = "wss://fstream.binance.com/ws/!forceOrder@arr"
	liqWindow5m             = 5 * time.Minute
	liqWindow15m            = 15 * time.Minute
	liqWindow1h             = 1 * time.Hour // used for the cascade-spike trailing average baseline
	liqSymbolFilter         = "BTCUSDT"
)

// liqEvent is a single recorded liquidation fill, in USD notional, tagged by
// the side of the underlying liquidated position (long or short).
type liqEvent struct {
	at      time.Time
	usd     float64
	wasLong bool // true = a LONG position was liquidated (forceOrder side SELL)
}

// BinanceLiquidationHolder tracks a rolling window of Binance BTCUSDT perpetual
// futures forced-liquidation volume, split by side, over trailing 5m/15m/1h
// windows. Safe for concurrent use. Degrades gracefully on disconnect: keeps
// serving the last computed rolling values (which will naturally decay toward
// zero as old events roll out of the window) and reports IsHealthy()=false so
// callers/strategies can decide whether to trust a stale read.
type BinanceLiquidationHolder struct {
	mu sync.RWMutex

	events []liqEvent // ring of recent liquidation events, pruned to the longest window (1h)

	connected   bool
	lastEventAt time.Time
	lastErr     error
}

// NewBinanceLiquidationHolder constructs a holder ready to be started with StartStreaming.
func NewBinanceLiquidationHolder() *BinanceLiquidationHolder {
	return &BinanceLiquidationHolder{}
}

// StartStreaming launches a background goroutine that connects to the Binance
// forceOrder WS stream and reconnects with exponential backoff (1s..30s) until
// ctx is cancelled.
func (h *BinanceLiquidationHolder) StartStreaming(ctx context.Context) {
	go h.run(ctx)
}

func (h *BinanceLiquidationHolder) run(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, binanceLiquidationWSURL, nil)
		if err != nil {
			h.mu.Lock()
			h.connected = false
			h.lastErr = err
			h.mu.Unlock()
			log.Printf("[LIQUIDATIONS] dial failed: %v (retrying in %s)", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		h.mu.Lock()
		h.connected = true
		h.lastErr = nil
		h.mu.Unlock()
		log.Printf("[LIQUIDATIONS] Connected to Binance forceOrder stream: %s", binanceLiquidationWSURL)
		backoff = time.Second

		h.readLoop(ctx, conn)
		conn.Close()

		h.mu.Lock()
		h.connected = false
		h.mu.Unlock()
		log.Println("[LIQUIDATIONS] stream disconnected — reconnecting (graceful degradation: last rolling values retained)")

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (h *BinanceLiquidationHolder) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			h.mu.Lock()
			h.lastErr = err
			h.mu.Unlock()
			log.Printf("[LIQUIDATIONS] read error: %v", err)
			return
		}
		h.handleMessage(msg)
	}
}

// binanceForceOrderEvent mirrors the raw !forceOrder@arr payload.
type binanceForceOrderEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Order     struct {
		Symbol string `json:"s"`
		Side   string `json:"S"`
		Price  string `json:"p"`
		Qty    string `json:"q"`
	} `json:"o"`
}

func (h *BinanceLiquidationHolder) handleMessage(msg []byte) {
	var evt binanceForceOrderEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		return
	}
	if evt.EventType != "" && evt.EventType != "forceOrder" {
		return
	}
	if !strings.EqualFold(evt.Order.Symbol, liqSymbolFilter) {
		return
	}
	price, err := strconv.ParseFloat(evt.Order.Price, 64)
	if err != nil || price <= 0 {
		return
	}
	qty, err := strconv.ParseFloat(evt.Order.Qty, 64)
	if err != nil || qty <= 0 {
		return
	}
	usd := price * qty

	// S="SELL" => exchange selling to close a liquidated LONG.
	// S="BUY"  => exchange buying to close a liquidated SHORT.
	wasLong := strings.EqualFold(evt.Order.Side, "SELL")

	ts := time.Now().UTC()
	if evt.EventTime > 0 {
		ts = time.UnixMilli(evt.EventTime).UTC()
	}

	h.mu.Lock()
	h.events = append(h.events, liqEvent{at: ts, usd: usd, wasLong: wasLong})
	h.lastEventAt = ts
	h.pruneLocked(ts)
	h.mu.Unlock()
}

// pruneLocked drops events older than the longest tracked window (1h).
// Caller must hold h.mu (write lock).
func (h *BinanceLiquidationHolder) pruneLocked(now time.Time) {
	cutoff := now.Add(-liqWindow1h)
	i := 0
	for i < len(h.events) && h.events[i].at.Before(cutoff) {
		i++
	}
	if i > 0 {
		h.events = h.events[i:]
	}
}

// sumWindowLocked sums USD liquidation notional by side over the trailing window.
// Caller must hold at least a read lock.
func (h *BinanceLiquidationHolder) sumWindowLocked(window time.Duration) (longUSD, shortUSD float64) {
	if len(h.events) == 0 {
		return 0, 0
	}
	cutoff := time.Now().UTC().Add(-window)
	for _, e := range h.events {
		if e.at.Before(cutoff) {
			continue
		}
		if e.wasLong {
			longUSD += e.usd
		} else {
			shortUSD += e.usd
		}
	}
	return longUSD, shortUSD
}

// Rolling5m returns trailing 5-minute long/short liquidation USD notional.
func (h *BinanceLiquidationHolder) Rolling5m() (longUSD, shortUSD float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sumWindowLocked(liqWindow5m)
}

// Rolling15m returns trailing 15-minute long/short liquidation USD notional.
func (h *BinanceLiquidationHolder) Rolling15m() (longUSD, shortUSD float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sumWindowLocked(liqWindow15m)
}

// Rolling1mRate returns the most recent 1-minute liquidation rate (USD/min),
// by side — used by S14 to detect cascade-velocity deceleration without
// needing dedicated per-strategy state (see s_liquidation_cascade_fade.go).
func (h *BinanceLiquidationHolder) Rolling1mRate() (longUSD, shortUSD float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sumWindowLocked(time.Minute)
}

// Rolling1hAvgPerMin returns the trailing 1-hour average liquidation rate
// (USD/min) by side — the cascade-spike baseline for S14 (">3x trailing 1h avg").
func (h *BinanceLiquidationHolder) Rolling1hAvgPerMin() (longUSD, shortUSD float64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	longTotal, shortTotal := h.sumWindowLocked(liqWindow1h)
	return longTotal / 60.0, shortTotal / 60.0
}

// IsHealthy returns true if the WS connection is currently established.
func (h *BinanceLiquidationHolder) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.connected
}

// IsPopulated returns true once at least one liquidation event has ever been recorded.
func (h *BinanceLiquidationHolder) IsPopulated() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastEventAt.Unix() > 0
}

// LastError returns the most recent connection/read error, if any.
func (h *BinanceLiquidationHolder) LastError() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastErr
}
