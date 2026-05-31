package delta

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type DeltaDivergenceEvent struct {
	Direction  alpha.Action
	Strength   float64
	Confidence float64
	Timestamp  time.Time
}

type Engine struct {
	prices []float64
	deltas []float64
	limit  int
}

func NewEngine(limit int) *Engine {
	if limit < 20 {
		limit = 1000
	}
	return &Engine{limit: limit}
}

func (e *Engine) Add(price, delta float64) {
	e.prices = append(e.prices, price)
	e.deltas = append(e.deltas, delta)
	if len(e.prices) > e.limit {
		e.prices = e.prices[len(e.prices)-e.limit:]
		e.deltas = e.deltas[len(e.deltas)-e.limit:]
	}
}

func (e *Engine) Detect() DeltaDivergenceEvent {
	n := len(e.prices)
	if n < 20 || len(e.deltas) < 20 {
		return DeltaDivergenceEvent{Direction: alpha.ActionHold}
	}
	prices := e.prices[n-20:]
	deltas := e.deltas[len(e.deltas)-20:]
	priceMove := prices[len(prices)-1] - prices[0]
	deltaSum := 0.0
	for _, d := range deltas {
		deltaSum += d
	}
	strength := 0.0
	if prices[0] != 0 {
		strength = alpha.Clamp(abs(priceMove)/prices[0]*100+abs(deltaSum)/100, 0, 1)
	}
	if priceMove > 0 && deltaSum < 0 {
		return DeltaDivergenceEvent{Direction: alpha.ActionSell, Strength: strength, Confidence: alpha.Clamp(0.70+strength*0.20, 0.70, 0.95), Timestamp: time.Now().UTC()}
	}
	if priceMove < 0 && deltaSum > 0 {
		return DeltaDivergenceEvent{Direction: alpha.ActionBuy, Strength: strength, Confidence: alpha.Clamp(0.70+strength*0.20, 0.70, 0.95), Timestamp: time.Now().UTC()}
	}
	return DeltaDivergenceEvent{Direction: alpha.ActionHold}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
