package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var klineSourceGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "marketdata",
	Name:      "kline_source",
	Help:      "1 = live Binance kline WebSocket feed active, 0 = synthetic 5m aggregation fallback.",
}, []string{"interval"})

const (
	binanceKlineWSURL = "wss://stream.binance.com:9443/stream"
	klineReconnectDelay = 5 * time.Second
)

// BinanceKlineClient subscribes to Binance kline WebSocket streams for 15m and 1h candles.
// On each closed kline it calls the supplied callback. If the stream disconnects it
// falls back to 5m synthesis (managed by the Orchestrator) and reconnects automatically.
type BinanceKlineClient struct {
	intervals  []string
	on15m      func(Candle)
	on1h       func(Candle)
	active15m  *atomicBool
	active1h   *atomicBool
}

// NewBinanceKlineClient creates a client for the given intervals (e.g. ["15m","1h"]).
func NewBinanceKlineClient(intervals []string) *BinanceKlineClient {
	return &BinanceKlineClient{
		intervals: intervals,
		active15m: &atomicBool{},
		active1h:  &atomicBool{},
	}
}

// Start subscribes to Binance kline streams and calls on15m/on1h on each closed candle.
// Reconnects automatically on disconnect. Blocks until ctx is cancelled.
func (b *BinanceKlineClient) Start(ctx context.Context, on15m func(Candle), on1h func(Candle)) error {
	b.on15m = on15m
	b.on1h = on1h

	for _, iv := range b.intervals {
		klineSourceGauge.WithLabelValues(iv).Set(0)
	}

	streams := ""
	for i, iv := range b.intervals {
		if i > 0 {
			streams += "/"
		}
		streams += "btcusdt@kline_" + iv
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.runSession(ctx, streams)

		// Mark feeds as inactive — fallback synthesis takes over.
		b.setActive("15m", false)
		b.setActive("1h", false)
		log.Println("[KLINES] Binance kline feed disconnected — falling back to 5m synthesis")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(klineReconnectDelay):
		}
	}
}

// IsActive15m returns true while the 15m Binance kline stream is connected and healthy.
func (b *BinanceKlineClient) IsActive15m() bool { return b.active15m.Load() }

// IsActive1h returns true while the 1h Binance kline stream is connected and healthy.
func (b *BinanceKlineClient) IsActive1h() bool { return b.active1h.Load() }

func (b *BinanceKlineClient) runSession(ctx context.Context, streams string) {
	url := fmt.Sprintf("%s?streams=%s", binanceKlineWSURL, streams)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		log.Printf("[KLINES] WebSocket dial failed: %v", err)
		return
	}
	defer conn.Close()
	log.Printf("[KLINES] Connected to Binance kline stream: %s", streams)

	// Mark feeds active once connected.
	for _, iv := range b.intervals {
		b.setActive(iv, true)
	}

	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[KLINES] Read error: %v", err)
			return
		}
		b.handleMessage(msg)
	}
}

// binanceStreamMsg is the combined stream message wrapper.
type binanceStreamMsg struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// binanceKlineEvent is the kline event payload.
type binanceKlineEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		StartTime            int64  `json:"t"`
		CloseTime            int64  `json:"T"`
		Symbol               string `json:"s"`
		Interval             string `json:"i"`
		Open                 string `json:"o"`
		Close                string `json:"c"`
		High                 string `json:"h"`
		Low                  string `json:"l"`
		Volume               string `json:"v"`
		NumberOfTrades       int    `json:"n"`
		IsFinal              bool   `json:"x"` // true = candle closed
	} `json:"k"`
}

func (b *BinanceKlineClient) handleMessage(msg []byte) {
	var wrapper binanceStreamMsg
	if err := json.Unmarshal(msg, &wrapper); err != nil {
		return
	}
	var evt binanceKlineEvent
	if err := json.Unmarshal(wrapper.Data, &evt); err != nil {
		return
	}
	k := evt.Kline
	if !k.IsFinal {
		return // candle not yet closed
	}

	candle := Candle{
		Symbol:    "BTCUSDT",
		Open:      parseF(k.Open),
		High:      parseF(k.High),
		Low:       parseF(k.Low),
		Close:     parseF(k.Close),
		Volume:    parseF(k.Volume),
		Trades:    k.NumberOfTrades,
		OpenTime:  time.UnixMilli(k.StartTime).UTC(),
		CloseTime: time.UnixMilli(k.CloseTime).UTC(),
	}

	switch k.Interval {
	case "15m":
		log.Printf("[KLINES] 15m candle closed: O=%.2f H=%.2f L=%.2f C=%.2f V=%.4f",
			candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
		klineSourceGauge.WithLabelValues("15m").Set(1)
		if b.on15m != nil {
			b.on15m(candle)
		}
	case "1h":
		log.Printf("[KLINES] 1h candle closed: O=%.2f H=%.2f L=%.2f C=%.2f V=%.4f",
			candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
		klineSourceGauge.WithLabelValues("1h").Set(1)
		if b.on1h != nil {
			b.on1h(candle)
		}
	}
}

func (b *BinanceKlineClient) setActive(interval string, active bool) {
	v := 0.0
	if active {
		v = 1.0
	}
	klineSourceGauge.WithLabelValues(interval).Set(v)
	switch interval {
	case "15m":
		b.active15m.Store(active)
	case "1h":
		b.active1h.Store(active)
	}
}

func parseF(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// atomicBool is a simple thread-safe boolean backed by sync/atomic.
type atomicBool struct {
	v int32
}

func (a *atomicBool) Store(b bool) {
	if b {
		atomic.StoreInt32(&a.v, 1)
	} else {
		atomic.StoreInt32(&a.v, 0)
	}
}

func (a *atomicBool) Load() bool { return atomic.LoadInt32(&a.v) != 0 }
