package performance

import (
	"context"
	"runtime"
	"sync"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

// EvaluationMode controls when a ScheduledStrategy is evaluated.
type EvaluationMode int

const (
	EvaluateOnTick   EvaluationMode = iota // evaluate on every tick
	EvaluateOnCandle                        // evaluate only on candle close ticks
)

// ScheduledStrategy pairs a strategy with its scheduling metadata.
type ScheduledStrategy struct {
	ID       string
	Mode     EvaluationMode
	Strategy strategy.Strategy
	Priority int
}

// EvaluationResult holds the signals produced by one strategy evaluation.
type EvaluationResult struct {
	StrategyID string
	Signals    []strategy.Signal
}

// StrategyScheduler evaluates a batch of strategies against a tick in parallel
// using a worker pool. It is the benchmark harness for measuring throughput
// and per-tick latency at scale (500+ strategies, 10k+ ticks/s).
type StrategyScheduler struct {
	workers int
}

// NewStrategyScheduler creates a scheduler with the given worker count.
// Pass 0 to use GOMAXPROCS (number of logical CPUs).
func NewStrategyScheduler(workers int) *StrategyScheduler {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	return &StrategyScheduler{workers: workers}
}

// Evaluate runs all strategies whose Mode matches the given mode against tick,
// returning one EvaluationResult per strategy that produced signals.
func (s *StrategyScheduler) Evaluate(ctx context.Context, tick marketdata.Tick, mode EvaluationMode, strategies []ScheduledStrategy) []EvaluationResult {
	type work struct {
		idx int
		ss  ScheduledStrategy
	}

	jobs := make(chan work, len(strategies))
	results := make([]EvaluationResult, len(strategies))

	var wg sync.WaitGroup
	for w := 0; w < s.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					continue
				}
				if j.ss.Mode != mode {
					continue
				}
				sigs := j.ss.Strategy.OnTick(tick)
				results[j.idx] = EvaluationResult{
					StrategyID: j.ss.ID,
					Signals:    sigs,
				}
			}
		}()
	}

	for i, ss := range strategies {
		jobs <- work{idx: i, ss: ss}
	}
	close(jobs)
	wg.Wait()

	// Compact: return only results that have signals
	out := make([]EvaluationResult, 0, len(strategies))
	for _, r := range results {
		if len(r.Signals) > 0 {
			out = append(out, r)
		}
	}
	return out
}
