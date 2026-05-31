package cvd

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type Strategy struct {
	cache *Cache
}

func NewStrategy(cache *Cache) *Strategy {
	return &Strategy{cache: cache}
}

func (s *Strategy) Evaluate(symbol string, prices []float64) alpha.Signal {
	if s.cache == nil {
		return alpha.Hold("CVDDivergence", symbol)
	}
	div := DetectDivergence(prices, s.cache.CVDSeries(1000))
	if div.Direction == alpha.ActionHold {
		return alpha.Hold("CVDDivergence", symbol)
	}
	return alpha.Signal{
		Source: "CVDDivergence", Symbol: symbol, Action: div.Direction,
		Confidence: div.Confidence, Reason: div.Reason,
		StopLossPct: 0.30, TakeProfitPct: 0.75, Timestamp: time.Now().UTC(),
	}
}
