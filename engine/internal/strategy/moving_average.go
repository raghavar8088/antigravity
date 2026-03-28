package strategy

import (
	"fmt"
	
	"antigravity-engine/internal/marketdata"
)

// MovingAverageCrossover is a sample rule-based trend-following strategy.
// Internally tracks the closing price in small arrays to generate signals.
type MovingAverageCrossover struct {
	fastPeriod int
	slowPeriod int
	
	prices []float64 // Simple slice acting as an in-memory queue.
}

// NewMovingAverageCrossover initializes the strategy with structural constraints.
func NewMovingAverageCrossover(fast, slow int) *MovingAverageCrossover {
	return &MovingAverageCrossover{
		fastPeriod: fast,
		slowPeriod: slow,
		prices:     make([]float64, 0),
	}
}

func (m *MovingAverageCrossover) Name() string {
	return fmt.Sprintf("MACross_%d_%d", m.fastPeriod, m.slowPeriod)
}

func (m *MovingAverageCrossover) OnTick(tick marketdata.Tick) []Signal {
	// A standard crossover operates exclusively on closed candles. High-frequency ticks are ignored.
	return nil 
}

func (m *MovingAverageCrossover) OnCandle(candle marketdata.Tick) []Signal {
	// Insert new period closing price
	m.prices = append(m.prices, candle.Price)
	
	// Prevent unbounded memory growth if the bot runs for months.
	if len(m.prices) > m.slowPeriod*2 {
		m.prices = m.prices[1:]
	}
	
	// Require enough data to form the slower moving average
	if len(m.prices) < m.slowPeriod {
		return []Signal{{Action: ActionHold}}
	}

	fastMA := calculateSMA(m.prices, m.fastPeriod)
	slowMA := calculateSMA(m.prices, m.slowPeriod)

	// A very basic edge trigger rule: Buy when Fast > Slow crossover
	if fastMA > slowMA {
		return []Signal{{
			Symbol:     candle.Symbol,
			Action:     ActionBuy,
			TargetSize: 0.1, // Fixed fractional position size
			Confidence: 1.0, 
		}}
	} else if fastMA < slowMA {
		return []Signal{{
			Symbol:     candle.Symbol,
			Action:     ActionSell,
			TargetSize: 0.1, 
			Confidence: 1.0, 
		}}
	}
	
	return []Signal{{Action: ActionHold}}
}

// calculateSMA determines the Simple Moving Average given a slice frame.
func calculateSMA(data []float64, period int) float64 {
	sum := 0.0
	startIdx := len(data) - period
	for _, val := range data[startIdx:] {
		sum += val
	}
	return sum / float64(period)
}
