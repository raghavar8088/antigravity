package orderblock

import (
	"fmt"
	"time"

	"antigravity-engine/internal/alpha"
)

type OrderBlock struct {
	Symbol        string
	Direction     alpha.Action
	Low           float64
	High          float64
	Strength      float64
	VolumeScore   float64
	ReactionCount int
	Age           int
	CreatedAt     time.Time
}

type Engine struct {
	blocks []OrderBlock
	limit  int
}

func NewEngine() *Engine { return &Engine{limit: 100} }

func (e *Engine) Detect(candles []alpha.Candle) []OrderBlock {
	if len(candles) < 4 {
		return nil
	}
	avgVol := 0.0
	for _, c := range candles[:len(candles)-1] {
		avgVol += c.Volume
	}
	avgVol /= float64(len(candles) - 1)
	i := len(candles) - 2
	prev := candles[i]
	next := candles[i+1]
	body := abs(next.Close - next.Open)
	rangeSize := next.High - next.Low
	if rangeSize <= 0 || body/rangeSize < 0.55 || avgVol <= 0 || next.Volume < avgVol*1.2 {
		return e.blocks
	}
	volumeScore := next.Volume / avgVol
	if prev.Close < prev.Open && next.Close > next.Open && next.Close > prev.High {
		e.add(OrderBlock{Symbol: next.Symbol, Direction: alpha.ActionBuy, Low: prev.Low, High: prev.Open, Strength: body / rangeSize, VolumeScore: volumeScore, CreatedAt: next.Timestamp})
	}
	if prev.Close > prev.Open && next.Close < next.Open && next.Close < prev.Low {
		e.add(OrderBlock{Symbol: next.Symbol, Direction: alpha.ActionSell, Low: prev.Open, High: prev.High, Strength: body / rangeSize, VolumeScore: volumeScore, CreatedAt: next.Timestamp})
	}
	return e.blocks
}

func (e *Engine) Evaluate(candles []alpha.Candle) alpha.Signal {
	blocks := e.Detect(candles)
	if len(candles) == 0 {
		return alpha.Hold("OrderBlockRetest", "")
	}
	last := candles[len(candles)-1]
	for i := range blocks {
		blocks[i].Age++
		if last.Low <= blocks[i].High && last.High >= blocks[i].Low {
			blocks[i].ReactionCount++
			if blocks[i].VolumeScore >= 1.2 && blocks[i].Age <= 80 {
				conf := alpha.Clamp(0.65+blocks[i].Strength*0.15+blocks[i].VolumeScore*0.04+float64(blocks[i].ReactionCount)*0.03, 0.65, 0.92)
				return alpha.Signal{Source: "OrderBlockRetest", Symbol: last.Symbol, Action: blocks[i].Direction, Confidence: conf, Reason: fmt.Sprintf("order block retest strength=%.2f volume=%.2f", blocks[i].Strength, blocks[i].VolumeScore), StopLossPct: 0.35, TakeProfitPct: 0.85, Timestamp: time.Now().UTC()}
			}
		}
	}
	return alpha.Hold("OrderBlockRetest", last.Symbol)
}

func (e *Engine) add(block OrderBlock) {
	e.blocks = append(e.blocks, block)
	if len(e.blocks) > e.limit {
		e.blocks = e.blocks[len(e.blocks)-e.limit:]
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
