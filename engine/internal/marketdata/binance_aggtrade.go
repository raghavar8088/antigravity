package marketdata

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const binanceAggTradeWSBase = "wss://stream.binance.com:9443/ws/"

// AggTrade is a single Binance aggregated trade with real taker-side
// classification — unlike a price-direction proxy, IsBuyerMaker tells us
// definitively which side was the aggressor.
type AggTrade struct {
	Price        float64
	Quantity     float64
	IsBuyerMaker bool // true = seller was aggressor (sell delta); false = buyer was aggressor (buy delta)
	Timestamp    time.Time
}

// BinanceAggTradeClient streams real-time aggregated trades from Binance so
// CVD can be computed from actual taker-side classification instead of the
// price-direction proxy used when only quote ticks are available. Reconnects
// automatically with exponential backoff (1s, 2s, 4s, ... capped at 30s).
type BinanceAggTradeClient struct {
	symbol  string
	onTrade func(AggTrade)

	mu   sync.RWMutex
	conn *websocket.Conn
}

// NewBinanceAggTradeClient creates a client for the given symbol (e.g. "btcusdt").
// onTrade is invoked synchronously for every aggregated trade event.
func NewBinanceAggTradeClient(symbol string, onTrade func(AggTrade)) *BinanceAggTradeClient {
	return &BinanceAggTradeClient{symbol: symbol, onTrade: onTrade}
}

// Start connects to the Binance aggTrade stream and invokes onTrade for every
// event, reconnecting automatically until ctx is cancelled. Blocks until ctx
// is done, returning ctx.Err().
func (c *BinanceAggTradeClient) Start(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		url := binanceAggTradeWSBase + c.symbol + "@aggTrade"
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			log.Printf("[AGGTRADE] dial failed: %v (retrying in %s)", err, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()
		log.Printf("[AGGTRADE] Connected to Binance aggTrade stream: %s", url)
		backoff = time.Second // reset backoff after a successful connection

		c.readLoop(ctx, conn)
		conn.Close()
		log.Println("[AGGTRADE] stream disconnected — reconnecting")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Stop closes the active WebSocket connection, if any. Start's read loop
// will then return and either reconnect or exit if ctx is already done.
func (c *BinanceAggTradeClient) Stop() {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn != nil {
		conn.Close()
	}
}

func (c *BinanceAggTradeClient) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[AGGTRADE] read error: %v", err)
			return
		}
		c.handleMessage(msg)
	}
}

// binanceAggTradeEvent mirrors the raw Binance aggTrade WebSocket payload:
// {"e":"aggTrade","E":evtTime,"s":"BTCUSDT","p":"price","q":"qty","m":isBuyerMaker,"T":tradeTime}
type binanceAggTradeEvent struct {
	EventType    string `json:"e"`
	Price        string `json:"p"`
	Quantity     string `json:"q"`
	IsBuyerMaker bool   `json:"m"`
	TradeTime    int64  `json:"T"`
}

func (c *BinanceAggTradeClient) handleMessage(msg []byte) {
	var evt binanceAggTradeEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		return
	}
	if evt.EventType != "" && evt.EventType != "aggTrade" {
		return
	}
	price, err := strconv.ParseFloat(evt.Price, 64)
	if err != nil {
		return
	}
	qty, err := strconv.ParseFloat(evt.Quantity, 64)
	if err != nil {
		return
	}
	ts := time.Now().UTC()
	if evt.TradeTime > 0 {
		ts = time.UnixMilli(evt.TradeTime).UTC()
	}
	if c.onTrade != nil {
		c.onTrade(AggTrade{
			Price:        price,
			Quantity:     qty,
			IsBuyerMaker: evt.IsBuyerMaker,
			Timestamp:    ts,
		})
	}
}
