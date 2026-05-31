package session

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type Name string

const (
	Asia    Name = "ASIA"
	London  Name = "LONDON"
	NewYork Name = "NEW_YORK"
)

type Snapshot struct {
	Name        Name
	RangePct    float64
	Volume      float64
	Volatility  float64
	BreakoutPct float64
	Bias        alpha.Action
	Momentum    float64
	Expansion   bool
}

type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

func CurrentSession(t time.Time) Name {
	h := t.UTC().Hour()
	switch {
	case h < 8:
		return Asia
	case h < 13:
		return London
	default:
		return NewYork
	}
}

func (e *Engine) Analyze(candles []alpha.Candle) Snapshot {
	if len(candles) == 0 {
		return Snapshot{}
	}
	last := candles[len(candles)-1]
	name := CurrentSession(last.Timestamp)
	window := alpha.TailCandles(candlesToRangeSession(candles, name), 120)
	if len(window) == 0 {
		window = alpha.TailCandles(candles, 30)
	}
	high, low, volume := window[0].High, window[0].Low, 0.0
	closes := make([]float64, 0, len(window))
	for _, c := range window {
		if c.High > high {
			high = c.High
		}
		if c.Low < low {
			low = c.Low
		}
		volume += c.Volume
		closes = append(closes, c.Close)
	}
	rangePct := 0.0
	if last.Close > 0 {
		rangePct = (high - low) / last.Close * 100
	}
	mom := 0.0
	if len(closes) >= 2 && closes[0] != 0 {
		mom = (closes[len(closes)-1] - closes[0]) / closes[0] * 100
	}
	bias := alpha.ActionHold
	if mom > 0.15 {
		bias = alpha.ActionBuy
	} else if mom < -0.15 {
		bias = alpha.ActionSell
	}
	breakout := 0.0
	if high > low {
		breakout = alpha.Clamp((last.Close-low)/(high-low)*100, 0, 100)
	}
	return Snapshot{Name: name, RangePct: rangePct, Volume: volume, Volatility: alpha.StdDev(closes), BreakoutPct: breakout, Bias: bias, Momentum: mom, Expansion: rangePct > 0.6 && (name == London || name == NewYork)}
}

func candlesToRangeSession(candles []alpha.Candle, name Name) []alpha.Candle {
	out := make([]alpha.Candle, 0, len(candles))
	for _, c := range candles {
		if CurrentSession(c.Timestamp) == name {
			out = append(out, c)
		}
	}
	return out
}
