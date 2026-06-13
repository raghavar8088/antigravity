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

const (
	depthWSURL       = "wss://stream.binance.com:9443/ws/btcusdt@depth20@1000ms"
	maxReconnects    = 5
	reconnectBase    = 1 * time.Second
	analyzeInterval  = 2 * time.Minute
)

// DepthSubscriber connects to the Binance depth WebSocket and maintains a
// live top-20 order book. It recomputes analysis every 2 minutes and on demand.
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

type depthMessage struct {
	Bids [][]string `json:"bids"` // [["price", "qty"], ...]
	Asks [][]string `json:"asks"`
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

	bids := parseLevels(dm.Bids)
	asks := parseLevels(dm.Asks)

	// Sort bids descending, asks ascending.
	sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })
	sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })

	d.mu.Lock()
	d.book = OrderBook{
		Symbol:    "BTCUSDT",
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

func parseLevels(raw [][]string) []PriceLevel {
	levels := make([]PriceLevel, 0, len(raw))
	for _, row := range raw {
		if len(row) < 2 {
			continue
		}
		p, err1 := strconv.ParseFloat(row[0], 64)
		q, err2 := strconv.ParseFloat(row[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		levels = append(levels, PriceLevel{Price: p, Quantity: q})
	}
	return levels
}
