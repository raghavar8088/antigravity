package mss

import (
	"fmt"
	"math"
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

// Engine detects institutional market structure shifts with:
//   - Multi-timeframe structure break (short 8-bar + medium 20-bar lookback)
//   - Trend filter via ADX (ADX > 20 required)
//   - Liquidity sweep confirmation (wick beyond previous extreme before close)
//   - Candle close strength filter (close must be in top/bottom 30% of range)
type Engine struct {
	lastTrend     alpha.Action
	lastBreakTime time.Time
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
	return alpha.Signal{
		Source:        "MSSContinuation",
		Symbol:        last.Symbol,
		Action:        event.Direction,
		Confidence:    event.Confidence,
		Reason:        fmt.Sprintf("%s through %.2f", event.Type, event.Level),
		StopLossPct:   0.35,
		TakeProfitPct: 0.90,
		Timestamp:     time.Now().UTC(),
	}
}

func (e *Engine) Detect(candles []alpha.Candle) StructureEvent {
	if len(candles) < 22 {
		return StructureEvent{Direction: alpha.ActionHold}
	}

	// ── Trend filter: ADX must be > 20 for structure-based trades ──────────
	adx := alpha.ADX(candles, 14)
	if adx > 0 && adx < 20 {
		return StructureEvent{Direction: alpha.ActionHold}
	}

	highs := make([]float64, len(candles)-1)
	lows := make([]float64, len(candles)-1)
	for i, c := range candles[:len(candles)-1] {
		highs[i] = c.High
		lows[i] = c.Low
	}

	last := candles[len(candles)-1]

	// ── Multi-timeframe structure: both short (8-bar) and medium (20-bar) ──
	prevHighShort := alpha.Highest(alpha.Tail(highs, 8))
	prevLowShort := alpha.Lowest(alpha.Tail(lows, 8))
	prevHighMed := alpha.Highest(alpha.Tail(highs, 20))
	prevLowMed := alpha.Lowest(alpha.Tail(lows, 20))

	direction := alpha.ActionHold
	level := 0.0

	if last.Close > prevHighShort && last.Close > prevHighMed {
		direction = alpha.ActionBuy
		level = prevHighShort
	} else if last.Close < prevLowShort && last.Close < prevLowMed {
		direction = alpha.ActionSell
		level = prevLowShort
	}

	if direction == alpha.ActionHold {
		return StructureEvent{Direction: alpha.ActionHold}
	}

	// ── Liquidity sweep confirmation ─────────────────────────────────────
	// Require the candle's wick to have swept beyond the level before closing
	// through it — this confirms stop-hunt behaviour before continuation.
	sweptLiquidity := false
	if direction == alpha.ActionBuy {
		// Wick must have poked above prevHighShort, and close is also above.
		sweptLiquidity = last.High > prevHighShort && last.Close > prevHighShort
	} else {
		sweptLiquidity = last.Low < prevLowShort && last.Close < prevLowShort
	}
	if !sweptLiquidity {
		return StructureEvent{Direction: alpha.ActionHold}
	}

	// ── Candle close strength ─────────────────────────────────────────────
	// For a bullish BOS: close must be in the top 30% of the candle range.
	// For a bearish BOS: close must be in the bottom 30% of the candle range.
	candleRange := last.High - last.Low
	if candleRange > 0 {
		closePos := (last.Close - last.Low) / candleRange
		if direction == alpha.ActionBuy && closePos < 0.70 {
			return StructureEvent{Direction: alpha.ActionHold}
		}
		if direction == alpha.ActionSell && closePos > 0.30 {
			return StructureEvent{Direction: alpha.ActionHold}
		}
	}

	// ── Prevent repeat signals on consecutive candles ─────────────────────
	if !e.lastBreakTime.IsZero() && last.Timestamp.Sub(e.lastBreakTime) < 3*time.Minute {
		return StructureEvent{Direction: alpha.ActionHold}
	}

	eventType := EventBOS
	if e.lastTrend != alpha.ActionHold && e.lastTrend != direction {
		eventType = EventMSS
	}

	e.lastTrend = direction
	e.lastBreakTime = last.Timestamp

	// ── Confidence: distance through level + ADX boost ─────────────────────
	breakMagnitude := math.Abs(last.Close-level) / last.Close * 100
	adxBoost := math.Min(adx/100*0.10, 0.10)
	conf := alpha.Clamp(0.60+breakMagnitude*0.20+adxBoost, 0.60, 0.95)

	return StructureEvent{
		Type:       eventType,
		Direction:  direction,
		Level:      level,
		Confidence: conf,
		Timestamp:  last.Timestamp,
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
