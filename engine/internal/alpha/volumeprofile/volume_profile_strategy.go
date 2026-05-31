package volumeprofile

import (
	"fmt"
	"time"

	"antigravity-engine/internal/alpha"
)

type Strategy struct{}

func NewStrategy() *Strategy { return &Strategy{} }

func (s *Strategy) Evaluate(candles []alpha.Candle) alpha.Signal {
	if len(candles) < 20 {
		return alpha.Hold("POCBounce", "")
	}
	last := candles[len(candles)-1]
	profile := Build(alpha.TailCandles(candles, 120), 48)
	if profile.POC <= 0 {
		return alpha.Hold("POCBounce", last.Symbol)
	}
	distPct := (last.Close - profile.POC) / profile.POC * 100
	if abs(distPct) <= 0.05 && last.Close > last.Open {
		return alpha.Signal{Source: "POCBounce", Symbol: last.Symbol, Action: alpha.ActionBuy, Confidence: 0.72, Reason: fmt.Sprintf("POC bounce %.2f", profile.POC), StopLossPct: 0.25, TakeProfitPct: 0.55, Timestamp: time.Now().UTC()}
	}
	if abs(distPct) <= 0.05 && last.Close < last.Open {
		return alpha.Signal{Source: "POCBounce", Symbol: last.Symbol, Action: alpha.ActionSell, Confidence: 0.72, Reason: fmt.Sprintf("POC rejection %.2f", profile.POC), StopLossPct: 0.25, TakeProfitPct: 0.55, Timestamp: time.Now().UTC()}
	}
	if last.Close > profile.VAH {
		return alpha.Signal{Source: "LVNBreakout", Symbol: last.Symbol, Action: alpha.ActionBuy, Confidence: 0.68, Reason: "value area high breakout", StopLossPct: 0.35, TakeProfitPct: 0.75, Timestamp: time.Now().UTC()}
	}
	if last.Close < profile.VAL {
		return alpha.Signal{Source: "LVNBreakout", Symbol: last.Symbol, Action: alpha.ActionSell, Confidence: 0.68, Reason: "value area low breakdown", StopLossPct: 0.35, TakeProfitPct: 0.75, Timestamp: time.Now().UTC()}
	}
	return alpha.Hold("POCBounce", last.Symbol)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
