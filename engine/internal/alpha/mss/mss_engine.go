package mss

import (
	"fmt"
	"time"

	"antigravity-engine/internal/alpha"
)

type StructureEventType string

const (
	EventBOS   StructureEventType = "BOS"
	EventCHOCH StructureEventType = "CHOCH"
	EventMSS   StructureEventType = "MSS"
)

type StructureEvent struct {
	Type       StructureEventType
	Direction  alpha.Action
	Level      float64
	Confidence float64
	Timestamp  time.Time
}

type Engine struct {
	lastTrend alpha.Action
}

func NewEngine() *Engine { return &Engine{lastTrend: alpha.ActionHold} }

func (e *Engine) Evaluate(candles []alpha.Candle) alpha.Signal {
	event := e.Detect(candles)
	if event.Direction == alpha.ActionHold {
		if len(candles) == 0 {
			return alpha.Hold("MSSContinuation", "")
		}
		return alpha.Hold("MSSContinuation", candles[len(candles)-1].Symbol)
	}
	last := candles[len(candles)-1]
	return alpha.Signal{Source: "MSSContinuation", Symbol: last.Symbol, Action: event.Direction, Confidence: event.Confidence, Reason: fmt.Sprintf("%s through %.2f", event.Type, event.Level), StopLossPct: 0.35, TakeProfitPct: 0.90, Timestamp: time.Now().UTC()}
}

func (e *Engine) Detect(candles []alpha.Candle) StructureEvent {
	if len(candles) < 9 {
		return StructureEvent{Direction: alpha.ActionHold}
	}
	highs := make([]float64, len(candles)-1)
	lows := make([]float64, len(candles)-1)
	for i, c := range candles[:len(candles)-1] {
		highs[i] = c.High
		lows[i] = c.Low
	}
	last := candles[len(candles)-1]
	prevHigh := alpha.Highest(alpha.Tail(highs, 8))
	prevLow := alpha.Lowest(alpha.Tail(lows, 8))
	direction := alpha.ActionHold
	level := 0.0
	if last.Close > prevHigh {
		direction = alpha.ActionBuy
		level = prevHigh
	} else if last.Close < prevLow {
		direction = alpha.ActionSell
		level = prevLow
	}
	if direction == alpha.ActionHold {
		return StructureEvent{Direction: alpha.ActionHold}
	}
	eventType := EventBOS
	if e.lastTrend != alpha.ActionHold && e.lastTrend != direction {
		eventType = EventCHOCH
	}
	if eventType == EventCHOCH {
		eventType = EventMSS
	}
	e.lastTrend = direction
	return StructureEvent{Type: eventType, Direction: direction, Level: level, Confidence: alpha.Clamp(0.60+abs(last.Close-level)/last.Close*20, 0.60, 0.95), Timestamp: last.Timestamp}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
