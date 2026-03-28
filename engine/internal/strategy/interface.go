package strategy

import "antigravity-engine/internal/marketdata"

// Action represents the signal output by our analytical models.
type Action string

const (
	ActionBuy  Action = "BUY"
	ActionSell Action = "SELL"
	ActionHold Action = "HOLD"
)

// Signal represents an intentional market order decision.
type Signal struct {
	Symbol        string
	Action        Action
	TargetSize    float64 // The amount of BTC to transact
	Confidence    float64 // Optional AI/ML probability metric
	StopLossPct   float64 // Auto stop-loss distance from entry (e.g. 0.5 = 0.5%)
	TakeProfitPct float64 // Auto take-profit distance from entry (e.g. 1.0 = 1.0%)
}

// Strategy represents the absolute core of Antigravity intelligence.
// Any algorithm (Rule-based, Mathematical indicator, or Deep Learning)
// MUST implement this interface to generate active trading Signals.
type Strategy interface {
	// Name defines the human-readable identifier of this strategy version.
	Name() string
	
	// OnTick accepts real-time high-speed market updates. Highly effective for market-making algorithms.
	OnTick(tick marketdata.Tick) []Signal

	// OnCandle accepts closed period updates (e.g., 1m, 1h, 1d) for trend-following or structural models.
	OnCandle(candle marketdata.Tick) []Signal // Note: In reality, we'd use a dedicated Candle struct representing OHLCV
}
