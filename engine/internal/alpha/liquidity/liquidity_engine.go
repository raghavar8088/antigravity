package liquidity

import "antigravity-engine/internal/alpha"

type Engine struct {
	candles []alpha.Candle
	limit   int
}

func NewEngine(limit int) *Engine {
	if limit < 20 {
		limit = 200
	}
	return &Engine{limit: limit}
}

func (e *Engine) Add(c alpha.Candle) SweepEvent {
	e.candles = append(e.candles, c)
	if len(e.candles) > e.limit {
		e.candles = e.candles[len(e.candles)-e.limit:]
	}
	if len(e.candles) < 10 {
		return SweepEvent{Direction: alpha.ActionHold}
	}
	highs := make([]float64, 0, len(e.candles)-1)
	lows := make([]float64, 0, len(e.candles)-1)
	for _, row := range e.candles[:len(e.candles)-1] {
		highs = append(highs, row.High)
		lows = append(lows, row.Low)
	}
	levels := DetectLevels(c.Symbol, highs, lows, 0.06)
	return DetectSweep(e.candles, levels)
}
