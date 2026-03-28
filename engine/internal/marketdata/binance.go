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
	conn    *websocket.Conn
	ch      chan Tick
	symbols []string
}

func NewBinanceClient() *BinanceClient {
	return &BinanceClient{
		ch: make(chan Tick, 10000), // Buffer to handle high-velocity ticks
	}
}

func (b *BinanceClient) Connect(ctx context.Context, symbols []string) error {
	b.symbols = symbols
	go b.keepConnected(ctx)
	return nil
}

func (b *BinanceClient) keepConnected(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := b.dial()
		if err != nil {
			log.Printf("Binance dial error: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		b.listen(ctx) // Blocks until the connection breaks

		log.Println("Binance stream disconnected. Reconnecting in 5s...")
		time.Sleep(5 * time.Second)
	}
}

func (b *BinanceClient) dial() error {
	streams := []string{}
	for _, s := range b.symbols {
		streams = append(streams, fmt.Sprintf("%s@trade", strings.ToLower(s)))
	}
	url := fmt.Sprintf("%s/%s", getBinanceWSBase(), strings.Join(streams, "/"))

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}
	b.conn = conn
	log.Printf("Connected to Binance stream: %s", url)
	return nil
}

func (b *BinanceClient) listen(ctx context.Context) {
	defer func() {
		if b.conn != nil {
			b.conn.Close()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := b.conn.ReadMessage()
			if err != nil {
				log.Printf("Binance read error (connection dropped): %v", err)
				return // Triggers auto-reconnect in keepConnected
			}

			var payload struct {
				E int64  `json:"E"`
				S string `json:"s"`
				P string `json:"p"`
				Q string `json:"q"`
				T int64  `json:"t"`
				M bool   `json:"m"`
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
