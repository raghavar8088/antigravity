package cvd

import (
	"strings"
	"time"

	"antigravity-engine/internal/alpha"
)

type TickDelta struct {
	Symbol     string
	BuyVolume  float64
	SellVolume float64
	Delta      float64
	CVD        float64
	Timestamp  time.Time
}

type Engine struct {
	cvdBySymbol map[string]float64
}

func NewEngine() *Engine {
	return &Engine{cvdBySymbol: make(map[string]float64)}
}

func (e *Engine) AddTick(t alpha.Tick) TickDelta {
	buy, sell := 0.0, 0.0
	side := strings.ToUpper(t.Side)
	if side == "BUY" || side == "BID" || side == "TAKER_BUY" {
		buy = t.Quantity
	} else if side == "SELL" || side == "ASK" || side == "TAKER_SELL" {
		sell = t.Quantity
	} else {
		// If side is unavailable, classify upticks as buyer aggression by the
		// caller's pre-aggregated sign convention: positive quantity buys.
		if t.Quantity >= 0 {
			buy = t.Quantity
		} else {
			sell = -t.Quantity
		}
	}
	delta := buy - sell
	e.cvdBySymbol[t.Symbol] += delta
	return TickDelta{
		Symbol: t.Symbol, BuyVolume: buy, SellVolume: sell, Delta: delta,
		CVD: e.cvdBySymbol[t.Symbol], Timestamp: t.Timestamp,
	}
}
