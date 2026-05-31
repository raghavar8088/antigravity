package performance

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/marketdata"
)

type MarketEventType string

const (
	MarketEventTick      MarketEventType = "TICK"
	MarketEventHeartbeat MarketEventType = "HEARTBEAT"
	MarketEventReconnect MarketEventType = "RECONNECT"
)

type MarketEvent struct {
	Type      MarketEventType
	Exchange  string
	Tick      marketdata.Tick
	Timestamp time.Time
}

type MarketDataBus struct {
	buffer      chan MarketEvent
	subscribers map[string]chan MarketEvent
	seen        map[string]int64
	mu          sync.RWMutex
	received    atomic.Int64
	published   atomic.Int64
	dropped     atomic.Int64
	duplicates  atomic.Int64
}

type MarketDataBusStats struct {
	Received    int64
	Published   int64
	Dropped     int64
	Duplicates  int64
	QueueDepth  int
	Subscribers int
}

func NewMarketDataBus(bufferSize int) *MarketDataBus {
	if bufferSize <= 0 {
		bufferSize = 100_000
	}
	return &MarketDataBus{
		buffer:      make(chan MarketEvent, bufferSize),
		subscribers: make(map[string]chan MarketEvent),
		seen:        make(map[string]int64),
	}
}

func (b *MarketDataBus) Publish(event MarketEvent) bool {
	b.received.Add(1)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.Type == MarketEventTick && b.isDuplicate(event) {
		b.duplicates.Add(1)
		return false
	}
	select {
	case b.buffer <- event:
		return true
	default:
		b.dropped.Add(1)
		return false
	}
}

func (b *MarketDataBus) Subscribe(label string, bufferSize int) <-chan MarketEvent {
	if bufferSize <= 0 {
		bufferSize = 4096
	}
	ch := make(chan MarketEvent, bufferSize)
	b.mu.Lock()
	b.subscribers[label] = ch
	b.mu.Unlock()
	return ch
}

func (b *MarketDataBus) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-b.buffer:
			b.mu.RLock()
			for _, sub := range b.subscribers {
				select {
				case sub <- event:
					b.published.Add(1)
				default:
					b.dropped.Add(1)
				}
			}
			b.mu.RUnlock()
		}
	}
}

func (b *MarketDataBus) Stats() MarketDataBusStats {
	b.mu.RLock()
	subs := len(b.subscribers)
	b.mu.RUnlock()
	return MarketDataBusStats{
		Received:    b.received.Load(),
		Published:   b.published.Load(),
		Dropped:     b.dropped.Load(),
		Duplicates:  b.duplicates.Load(),
		QueueDepth:  len(b.buffer),
		Subscribers: subs,
	}
}

func (b *MarketDataBus) isDuplicate(event MarketEvent) bool {
	key := event.Exchange + ":" + event.Tick.Symbol + ":" + stringInt64(event.Tick.TradeID)
	b.mu.Lock()
	defer b.mu.Unlock()
	if last, ok := b.seen[key]; ok && last == event.Tick.TimeMs {
		return true
	}
	b.seen[key] = event.Tick.TimeMs
	if len(b.seen) > 250_000 {
		b.seen = make(map[string]int64)
	}
	return false
}

func stringInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
