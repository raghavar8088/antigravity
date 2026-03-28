package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
)

func getBinanceWSBase() string {
	url := os.Getenv("BINANCE_WS_URL")
	if url == "" {
		return "wss://stream.binance.com:9443/ws"
	}
	return url
}

type BinanceClient struct {
	conn *websocket.Conn
	ch   chan Tick
}

func NewBinanceClient() *BinanceClient {
	return &BinanceClient{
		ch: make(chan Tick, 10000), // Buffer to handle high-velocity ticks
	}
}

func (b *BinanceClient) Connect(ctx context.Context, symbols []string) error {
	// Construct the stream URL: /ws/btcusdt@trade
	streams := []string{}
	for _, s := range symbols {
		streams = append(streams, fmt.Sprintf("%s@trade", strings.ToLower(s)))
	}
	url := fmt.Sprintf("%s/%s", getBinanceWSBase(), strings.Join(streams, "/"))

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("failed to dial binance: %w", err)
	}
	b.conn = conn

	log.Printf("Connected to Binance stream: %s", url)

	go b.listen(ctx)
	return nil
}

func (b *BinanceClient) listen(ctx context.Context) {
	defer b.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := b.conn.ReadMessage()
			if err != nil {
				log.Println("Binance read error:", err)
				return // Typically we trigger a reconnect here
			}

			var payload struct {
				E int64  `json:"E"` // Event time
				S string `json:"s"` // Symbol
				P string `json:"p"` // Price
				Q string `json:"q"` // Quantity
				T int64  `json:"t"` // Trade ID
				M bool   `json:"m"` // Is the buyer the market maker? (True means SELL market order, False means BUY market order)
			}

			if err := json.Unmarshal(message, &payload); err == nil {
				price, _ := strconv.ParseFloat(payload.P, 64)
				qty, _ := strconv.ParseFloat(payload.Q, 64)
				side := "BUY"
				if payload.M {
					side = "SELL"
				}

				b.ch <- Tick{
					Symbol:   payload.S,
					Price:    price,
					Quantity: qty,
					Side:     side,
					TradeID:  payload.T,
					TimeMs:   payload.E,
				}
			}
		}
	}
}

func (b *BinanceClient) GetTickChannel() <-chan Tick {
	return b.ch
}

func (b *BinanceClient) Close() error {
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}
