package performance

import (
	"sync"
	"time"
)

type IndicatorSnapshot struct {
	Symbol    string
	Timeframe string
	EMAFast   float64
	EMASlow   float64
	VWAP      float64
	RSI       float64
	ADX       float64
	ATR       float64
	Volume    float64
	UpdatedAt time.Time
}

type IndicatorCache struct {
	mu      sync.RWMutex
	entries map[string]IndicatorSnapshot
	maxAge  time.Duration
}

func NewIndicatorCache(maxAge time.Duration) *IndicatorCache {
	if maxAge <= 0 {
		maxAge = 2 * time.Second
	}
	return &IndicatorCache{entries: make(map[string]IndicatorSnapshot), maxAge: maxAge}
}

func (c *IndicatorCache) Set(snapshot IndicatorSnapshot) {
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now()
	}
	c.mu.Lock()
	c.entries[indicatorKey(snapshot.Symbol, snapshot.Timeframe)] = snapshot
	c.mu.Unlock()
}

func (c *IndicatorCache) Get(symbol, timeframe string) (IndicatorSnapshot, bool) {
	c.mu.RLock()
	item, ok := c.entries[indicatorKey(symbol, timeframe)]
	c.mu.RUnlock()
	if !ok || time.Since(item.UpdatedAt) > c.maxAge {
		return IndicatorSnapshot{}, false
	}
	return item, true
}

func indicatorKey(symbol, timeframe string) string {
	return symbol + ":" + timeframe
}
