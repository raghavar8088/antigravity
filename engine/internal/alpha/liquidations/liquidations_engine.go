package liquidations

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type Event struct {
	Symbol    string
	Side      alpha.Action
	Price     float64
	Quantity  float64
	Notional  float64
	Timestamp time.Time
}

type Cluster struct {
	Symbol      string
	Price       float64
	Notional    float64
	Count       int
	StartedAt   time.Time
	LastEventAt time.Time
}

type Engine struct {
	events []Event
	limit  int
}

func NewEngine(limit int) *Engine {
	if limit < 100 {
		limit = 5000
	}
	return &Engine{limit: limit}
}

func (e *Engine) Add(event Event) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if event.Notional <= 0 {
		event.Notional = event.Price * event.Quantity
	}
	e.events = append(e.events, event)
	if len(e.events) > e.limit {
		e.events = e.events[len(e.events)-e.limit:]
	}
}

func (e *Engine) Imbalance(window time.Duration) float64 {
	cutoff := time.Now().UTC().Add(-window)
	longLiq, shortLiq := 0.0, 0.0
	for _, row := range e.events {
		if row.Timestamp.Before(cutoff) {
			continue
		}
		if row.Side == alpha.ActionSell {
			longLiq += row.Notional
		} else {
			shortLiq += row.Notional
		}
	}
	total := longLiq + shortLiq
	if total == 0 {
		return 0
	}
	return (shortLiq - longLiq) / total
}

func (e *Engine) Signal(symbol string) alpha.Signal {
	imb := e.Imbalance(5 * time.Minute)
	if imb > 0.45 {
		return alpha.Signal{Source: "LiquidationCascade", Symbol: symbol, Action: alpha.ActionSell, Confidence: alpha.Clamp(0.70+imb*0.20, 0.70, 0.94), Reason: "short liquidation imbalance exhaustion", StopLossPct: 0.40, TakeProfitPct: 0.90, Timestamp: time.Now().UTC()}
	}
	if imb < -0.45 {
		return alpha.Signal{Source: "LiquidationCascade", Symbol: symbol, Action: alpha.ActionBuy, Confidence: alpha.Clamp(0.70-imb*0.20, 0.70, 0.94), Reason: "long liquidation imbalance exhaustion", StopLossPct: 0.40, TakeProfitPct: 0.90, Timestamp: time.Now().UTC()}
	}
	return alpha.Hold("LiquidationCascade", symbol)
}
