package funding

import (
	"fmt"
	"time"

	"antigravity-engine/internal/alpha"
)

type StrategyInput struct {
	Symbol        string
	Exchange      string
	Closes        []float64
	Support       float64
	Resistance    float64
	LatestFunding FundingSnapshot
	Metrics       Metrics
}

type Strategy struct {
	engine *Engine
}

func NewStrategy(engine *Engine) *Strategy {
	return &Strategy{engine: engine}
}

func (s *Strategy) Evaluate(input StrategyInput) alpha.Signal {
	if input.Symbol == "" {
		input.Symbol = input.LatestFunding.Symbol
	}
	metrics := input.Metrics
	if metrics.Regime == "" && s.engine != nil {
		metrics = s.engine.Metrics(input.Exchange, input.Symbol, 30*24*time.Hour)
	}
	rsi := alpha.RSI(input.Closes, 14)
	price := 0.0
	if len(input.Closes) > 0 {
		price = input.Closes[len(input.Closes)-1]
	}
	nearSupport := input.Support > 0 && price <= input.Support*1.003
	nearResistance := input.Resistance > 0 && price >= input.Resistance*0.997
	rate := input.LatestFunding.FundingRate

	if rate < -0.0005 && metrics.Percentile < 10 && rsi < 35 && nearSupport {
		conf := alpha.Clamp(0.60+(10-metrics.Percentile)/100+alpha.Clamp(-metrics.ZScore, 0, 3)*0.06, 0.60, 0.95)
		return alpha.Signal{
			Source: "FundingMeanReversion", Symbol: input.Symbol, Action: alpha.ActionBuy,
			Confidence: conf, Reason: fmt.Sprintf("negative funding extreme rate=%.5f pct=%.1f rsi=%.1f", rate, metrics.Percentile, rsi),
			StopLossPct: 0.35, TakeProfitPct: 0.85, Timestamp: time.Now().UTC(),
		}
	}
	if rate > 0.001 && metrics.Percentile > 90 && rsi > 65 && nearResistance {
		conf := alpha.Clamp(0.60+(metrics.Percentile-90)/100+alpha.Clamp(metrics.ZScore, 0, 3)*0.06, 0.60, 0.95)
		return alpha.Signal{
			Source: "FundingMeanReversion", Symbol: input.Symbol, Action: alpha.ActionSell,
			Confidence: conf, Reason: fmt.Sprintf("positive funding extreme rate=%.5f pct=%.1f rsi=%.1f", rate, metrics.Percentile, rsi),
			StopLossPct: 0.35, TakeProfitPct: 0.85, Timestamp: time.Now().UTC(),
		}
	}
	return alpha.Hold("FundingMeanReversion", input.Symbol)
}
