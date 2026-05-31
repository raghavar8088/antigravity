package delta

import (
	"time"

	"antigravity-engine/internal/alpha"
)

type Strategy struct {
	engine *Engine
}

func NewStrategy(engine *Engine) *Strategy {
	return &Strategy{engine: engine}
}

func (s *Strategy) Evaluate(symbol string) alpha.Signal {
	if s.engine == nil {
		return alpha.Hold("DeltaAbsorption", symbol)
	}
	event := s.engine.Detect()
	if event.Direction == alpha.ActionHold {
		return alpha.Hold("DeltaAbsorption", symbol)
	}
	reason := "accumulation: price down while delta rises"
	if event.Direction == alpha.ActionSell {
		reason = "absorption: price up while delta falls"
	}
	return alpha.Signal{
		Source: "DeltaAbsorption", Symbol: symbol, Action: event.Direction,
		Confidence: event.Confidence, Reason: reason,
		StopLossPct: 0.25, TakeProfitPct: 0.65, Timestamp: time.Now().UTC(),
	}
}
