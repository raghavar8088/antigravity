package main

import (
	"encoding/json"
	"net/http"
	"sync"

	"antigravity-engine/internal/marketdata"
)

type niftyMarketPoint struct {
	Time  int64   `json:"time"`
	Price float64 `json:"price"`
}

type niftyMarketSnapshot struct {
	Price         float64            `json:"price"`
	Change        float64            `json:"change"`
	PercentChange float64            `json:"percentChange"`
	ExchangeTime  string             `json:"exchangeTime"`
	UpdatedAt     int64              `json:"updatedAt"`
	Series        []niftyMarketPoint `json:"series"`
}

type NiftyMarketCache struct {
	mu        sync.RWMutex
	latest    marketdata.NSEIndexQuote
	series    []niftyMarketPoint
	maxPoints int
}

func NewNiftyMarketCache(maxPoints int) *NiftyMarketCache {
	if maxPoints <= 0 {
		maxPoints = 180
	}
	return &NiftyMarketCache{maxPoints: maxPoints}
}

func (c *NiftyMarketCache) Update(quote marketdata.NSEIndexQuote) {
	if quote.Price <= 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.latest = quote
	point := niftyMarketPoint{
		Time:  quote.FetchedAt.UnixMilli(),
		Price: quote.Price,
	}

	if len(c.series) > 0 && c.series[len(c.series)-1].Time == point.Time {
		c.series[len(c.series)-1] = point
	} else {
		c.series = append(c.series, point)
		if len(c.series) > c.maxPoints {
			c.series = c.series[len(c.series)-c.maxPoints:]
		}
	}
}

func (c *NiftyMarketCache) Snapshot() niftyMarketSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	series := make([]niftyMarketPoint, len(c.series))
	copy(series, c.series)

	return niftyMarketSnapshot{
		Price:         c.latest.Price,
		Change:        c.latest.Change,
		PercentChange: c.latest.PercentChange,
		ExchangeTime:  c.latest.ExchangeTime,
		UpdatedAt:     c.latest.FetchedAt.UnixMilli(),
		Series:        series,
	}
}

func (c *NiftyMarketCache) HandleQuote(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		return
	}
	json.NewEncoder(w).Encode(c.Snapshot())
}
