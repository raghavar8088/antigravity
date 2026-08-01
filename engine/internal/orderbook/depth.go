package orderbook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// L2 depth comes from Delta, the venue this engine executes on.
//
// It was Binance BTCUSDT@depth20. Of every feed in this application, the order
// book is the one where substituting another exchange makes least sense: a
// bid wall, a spread, a depth imbalance are facts about the resting orders in
// ONE book. Binance's walls are not the walls this account's orders will hit,
// and the liquidity conclusions drawn from them did not describe the market
// being traded at all.
//
// DELTA QUOTES SIZE IN CONTRACTS, like everywhere else in its API. One BTCUSD
// contract is 0.001 BTC, so an unscaled book reports a thousandfold more
// liquidity than exists — and every wall-detection and depth-imbalance
// threshold tuned on BTC quantities would stop firing entirely.
const (
	// Delta India public market-data socket. No auth: depth is public, and this
	// subscriber never sends credentials.
	depthWSURL  = "wss://socket.india.delta.exchange"
	depthSymbol = "BTCUSD"
	// BTC per contract. Delta publishes contract_value on its ticker; this is
	// BTCUSD's listed value and matches marketdata.DeltaContractValueBTC.
	depthContractValueBTC = 0.001
	maxReconnects         = 5
	reconnectBase         = 1 * time.Second
	analyzeInterval       = 2 * time.Minute
)

// DepthSubscriber connects to Delta's l2_orderbook WebSocket and maintains a
// live order book. It recomputes analysis every 2 minutes and on demand.
type DepthSubscriber struct {
	conn          *websocket.Conn
	book          OrderBook
	analysis      *OrderBookAnalysis
	spreadHistory []float64
	mu            sync.RWMutex
	reconnectMax  int
	reconnectWait time.Duration
	lastPrice     float64
}

// NewDepthSubscriber creates a DepthSubscriber with production defaults.
func NewDepthSubscriber() *DepthSubscriber {
	return &DepthSubscriber{
		reconnectMax:  maxReconnects,
		reconnectWait: reconnectBase,
	}
}

// Connect establishes the WebSocket connection and starts processing messages.
// Reconnects with exponential backoff on disconnect. Blocks until ctx is done.
func (d *DepthSubscriber) Connect(ctx context.Context) error {
	analyseTicker := time.NewTicker(analyzeInterval)
	defer analyseTicker.Stop()

	attempts := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, depthWSURL, nil)
		if err == nil {
			// Delta requires an explicit subscribe; Binance encoded the stream
			// in the URL. Without this the socket connects and stays silent.
			sub := map[string]any{
				"type": "subscribe",
				"payload": map[string]any{
					"channels": []map[string]any{
						{"name": "l2_orderbook", "symbols": []string{depthSymbol}},
					},
				},
			}
			if werr := conn.WriteJSON(sub); werr != nil {
				conn.Close()
				err = fmt.Errorf("subscribe: %w", werr)
			}
		}
		if err != nil {
			attempts++
			if attempts > d.reconnectMax {
				return fmt.Errorf("max reconnect attempts (%d) exceeded: %w", d.reconnectMax, err)
			}
			wait := d.reconnectWait * time.Duration(attempts*attempts)
			slog.Warn("[orderbook] WebSocket connect failed, retrying",
				"attempt", attempts,
				"wait_ms", wait.Milliseconds(),
				"err", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		attempts = 0
		d.mu.Lock()
		d.conn = conn
		d.mu.Unlock()
		slog.Info("[orderbook] WebSocket connected", "url", depthWSURL)

		if err := d.readLoop(ctx, conn, analyseTicker); err != nil {
			slog.Warn("[orderbook] WebSocket disconnected", "err", err)
			conn.Close()
			// Loop will reconnect on next iteration.
		}
	}
}

// GetLatestAnalysis returns the most recent order book analysis. Thread-safe.
// Returns nil if no analysis has been computed yet.
func (d *DepthSubscriber) GetLatestAnalysis() *OrderBookAnalysis {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.analysis == nil {
		return &OrderBookAnalysis{SpreadNormal: true, AnalysedAt: time.Now().UTC()}
	}
	a := *d.analysis
	return &a
}

// SetCurrentPrice updates the price used for analysis. Called by the market
// data layer on every tick.
func (d *DepthSubscriber) SetCurrentPrice(price float64) {
	d.mu.Lock()
	d.lastPrice = price
	d.mu.Unlock()
}

// ── internals ─────────────────────────────────────────────────────────────────

// deltaDepthLevel is one side of Delta's l2_orderbook. limit_price is a quoted
// string; size is a NUMBER OF CONTRACTS. "depth" is the cumulative total and is
// deliberately ignored — the analysis wants per-level quantity.
type deltaDepthLevel struct {
	LimitPrice string  `json:"limit_price"`
	Size       float64 `json:"size"`
}

// depthMessage is Delta's l2_orderbook payload. Note "buy"/"sell", not
// "bids"/"asks": decoding the Binance names against this schema yields two empty
// slices and an order book that is silently, permanently flat.
type depthMessage struct {
	Type string            `json:"type"`
	Buy  []deltaDepthLevel `json:"buy"`
	Sell []deltaDepthLevel `json:"sell"`
}

func (d *DepthSubscriber) readLoop(ctx context.Context, conn *websocket.Conn, analyseTicker *time.Ticker) error {
	msgCh := make(chan []byte, 64)
	errCh := make(chan error, 1)

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			select {
			case msgCh <- msg:
			default:
				// Drop if buffer full — latest message is more important than backlog.
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case msg := <-msgCh:
			d.processMessage(msg)
		case <-analyseTicker.C:
			d.runAnalysis()
		}
	}
}

func (d *DepthSubscriber) processMessage(msg []byte) {
	var dm depthMessage
	if err := json.Unmarshal(msg, &dm); err != nil {
		slog.Debug("[orderbook] parse error", "err", err)
		return
	}

	// Ignore the subscribe acknowledgement and anything else on the socket.
	if dm.Type != "" && dm.Type != "l2_orderbook" && dm.Type != "l2_updates" {
		return
	}
	if len(dm.Buy) == 0 && len(dm.Sell) == 0 {
		return
	}

	bids := parseDeltaLevels(dm.Buy)
	asks := parseDeltaLevels(dm.Sell)

	// Sort bids descending, asks ascending.
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })

	d.mu.Lock()
	d.book = OrderBook{
		Symbol:    depthSymbol,
		Bids:      bids,
		Asks:      asks,
		UpdatedAt: time.Now().UTC(),
	}
	d.mu.Unlock()
}

func (d *DepthSubscriber) runAnalysis() {
	d.mu.Lock()
	book := d.book
	price := d.lastPrice
	hist := make([]float64, len(d.spreadHistory))
	copy(hist, d.spreadHistory)
	d.mu.Unlock()

	if len(book.Bids) == 0 {
		return
	}
	if price == 0 && len(book.Bids) > 0 {
		price = book.Bids[0].Price
	}

	a := Analyse(book, price, hist)

	d.mu.Lock()
	d.analysis = &a
	// Append spread to history.
	d.spreadHistory = append(d.spreadHistory, a.SpreadBps)
	if len(d.spreadHistory) > spreadHistoryLen {
		d.spreadHistory = d.spreadHistory[len(d.spreadHistory)-spreadHistoryLen:]
	}
	d.mu.Unlock()

	slog.Debug("[orderbook] analysis updated",
		"imbalance", a.DepthImbalance,
		"spread_bps", a.SpreadBps,
		"score", a.Score,
	)
}

// parseDeltaLevels converts Delta's depth levels into the engine's PriceLevel,
// scaling size from contracts to BTC.
//
// The scaling is the whole point: unscaled, a 4,433-contract bid reads as 4,433
// BTC rather than 4.433, so every wall threshold and depth-imbalance bound
// calibrated on BTC quantities becomes meaningless.
func parseDeltaLevels(levels []deltaDepthLevel) []PriceLevel {
	out := make([]PriceLevel, 0, len(levels))
	for _, l := range levels {
		price, err := strconv.ParseFloat(l.LimitPrice, 64)
		if err != nil || price <= 0 || l.Size <= 0 {
			continue
		}
		out = append(out, PriceLevel{Price: price, Quantity: l.Size * depthContractValueBTC})
	}
	return out
}
