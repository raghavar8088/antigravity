package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/performance"
	"antigravity-engine/internal/strategy"
)

type benchStrategy struct {
	name string
}

func (b benchStrategy) Name() string { return b.name }
func (b benchStrategy) OnTick(t marketdata.Tick) []strategy.Signal {
	return []strategy.Signal{{Symbol: t.Symbol, Action: strategy.ActionHold, Confidence: 0.5}}
}
func (b benchStrategy) OnCandle(t marketdata.Tick) []strategy.Signal { return b.OnTick(t) }

func main() {
	strategyCount := flag.Int("strategies", 500, "number of strategies to evaluate")
	tickCount := flag.Int("ticks", 10000, "number of ticks to simulate")
	workers := flag.Int("workers", 0, "strategy scheduler workers; defaults to CPU count")
	flag.Parse()

	scheduler := performance.NewStrategyScheduler(*workers)
	strategies := make([]performance.ScheduledStrategy, *strategyCount)
	for i := range strategies {
		strategies[i] = performance.ScheduledStrategy{
			ID:       fmt.Sprintf("strategy-%04d", i),
			Mode:     performance.EvaluateOnTick,
			Strategy: benchStrategy{name: fmt.Sprintf("strategy-%04d", i)},
			Priority: i,
		}
	}

	start := time.Now()
	maxLatency := time.Duration(0)
	totalSignals := 0
	for i := 0; i < *tickCount; i++ {
		tick := marketdata.Tick{Symbol: "BTCUSD", Price: 100000 + float64(i%100), Quantity: 1, TradeID: int64(i), TimeMs: time.Now().UnixMilli()}
		iterStart := time.Now()
		results := scheduler.Evaluate(context.Background(), tick, performance.EvaluateOnTick, strategies)
		latency := time.Since(iterStart)
		if latency > maxLatency {
			maxLatency = latency
		}
		for _, result := range results {
			totalSignals += len(result.Signals)
		}
	}
	elapsed := time.Since(start)
	fmt.Printf("strategies=%d ticks=%d total_evaluations=%d total_signals=%d elapsed_ms=%d max_tick_eval_ms=%.3f evals_per_sec=%.0f\n",
		*strategyCount,
		*tickCount,
		*strategyCount**tickCount,
		totalSignals,
		elapsed.Milliseconds(),
		float64(maxLatency.Microseconds())/1000,
		float64(*strategyCount**tickCount)/elapsed.Seconds(),
	)
}
