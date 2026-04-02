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
	Symbol        string  `json:"symbol"`
	Action        Action  `json:"action"`
	TargetSize    float64 `json:"targetSize"`
	Confidence    float64 `json:"confidence"`
	StopLossPct   float64 `json:"stopLossPct"`
	TakeProfitPct float64 `json:"takeProfitPct"`
	
	// AI Attribution fields — populated by the Supreme Court
	AIDecisionID string `json:"aiDecisionId"` // Which AI approved this? (openai, groq, etc)
	AIReasoning  string `json:"aiReasoning"`  // Short snippet of the AI's reason
}

// Strategy represents the absolute core of RAIG intelligence.
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
