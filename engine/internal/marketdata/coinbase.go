package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

func getCoinbaseWSBase() string {
	url := os.Getenv("COINBASE_WS_URL")
	if url == "" {
		return "wss://ws-feed.exchange.coinbase.com"
	}
	return url
}

type CoinbaseClient struct {
	conn    *websocket.Conn
	ch      chan Tick
	symbols []string
}

func NewCoinbaseClient() *CoinbaseClient {
	return &CoinbaseClient{
		ch: make(chan Tick, 10000), // Buffer to handle high-velocity ticks
	}
}

func (c *CoinbaseClient) Connect(ctx context.Context, symbols []string) error {
	c.symbols = symbols
	go c.keepConnected(ctx)
	return nil
}

func (c *CoinbaseClient) keepConnected(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := c.dial()
		if err != nil {
			log.Printf("Coinbase dial error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		c.listen(ctx) // Blocks until the connection breaks

		log.Println("Coinbase stream disconnected. Reconnecting in 5s...")
		time.Sleep(5 * time.Second)
	}
}

func (c *CoinbaseClient) dial() error {
	url := getCoinbaseWSBase()

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	log.Printf("Connected to Coinbase stream: %s", url)

	// Subscribe to ticker/matches
	subMsg := map[string]interface{}{
		"type":        "subscribe",
		"product_ids": c.symbols,
		"channels":    []string{"matches"},
	}

	err = c.conn.WriteJSON(subMsg)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to send subscribe message: %w", err)
	}

	return nil
}

func (c *CoinbaseClient) listen(ctx context.Context) {
	defer func() {
		if c.conn != nil {
			c.conn.Close()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("Coinbase read error (connection dropped): %v", err)
				return // Triggers auto-reconnect in keepConnected
			}

			var payload struct {
				Type      string `json:"type"`
				ProductID string `json:"product_id"`
				Price     string `json:"price"`
				Size      string `json:"size"`
				Side      string `json:"side"` // "buy" or "sell"
				TradeID   int64  `json:"trade_id"`
				TimeStr   string `json:"time"`
			}

			if err := json.Unmarshal(message, &payload); err == nil {
				if payload.Type != "match" && payload.Type != "last_match" {
					continue
				}

				price, _ := strconv.ParseFloat(payload.Price, 64)
				qty, _ := strconv.ParseFloat(payload.Size, 64)
				
				// Coinbase outputs "buy" or "sell", Binance used "BUY" or "SELL".
				side := "BUY"
				if payload.Side == "sell" {
					side = "SELL"
				}

				// Parse RFC3339 time to Unix Milliseconds
				t, err := time.Parse(time.RFC3339Nano, payload.TimeStr)
				var timeMs int64
				if err == nil {
					timeMs = t.UnixMilli()
				} else {
					timeMs = time.Now().UnixMilli() // Fallback
				}

				c.ch <- Tick{
					Symbol:   payload.ProductID,
					Price:    price,
					Quantity: qty,
					Side:     side,
					TradeID:  payload.TradeID,
					TimeMs:   timeMs,
				}
			}
		}
	}
}

func (c *CoinbaseClient) GetTickChannel() <-chan Tick {
	return c.ch
}

func (c *CoinbaseClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
