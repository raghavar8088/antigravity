package performance

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"antigravity-engine/internal/marketdata"
	"antigravity-engine/internal/strategy"
)

type EvaluationMode string

const (
	EvaluateOnTick        EvaluationMode = "ON_TICK"
	EvaluateOnCandleClose EvaluationMode = "ON_CANDLE_CLOSE"
	EvaluateEventDriven   EvaluationMode = "EVENT_DRIVEN"
)

type ScheduledStrategy struct {
	ID       string
	Mode     EvaluationMode
	Strategy strategy.Strategy
	Priority int
}

type EvaluationResult struct {
	StrategyID string
	Signals    []strategy.Signal
	Latency    time.Duration
	Error      error
}

type StrategyScheduler struct {
	workers int
}

func NewStrategyScheduler(workers int) *StrategyScheduler {
	if workers <= 0 {
		workers = maxInt(2, runtime.NumCPU())
	}
	return &StrategyScheduler{workers: workers}
}

func (s *StrategyScheduler) Evaluate(ctx context.Context, tick marketdata.Tick, mode EvaluationMode, strategies []ScheduledStrategy) []EvaluationResult {
	selected := make([]ScheduledStrategy, 0, len(strategies))
	for _, item := range strategies {
		if item.Strategy == nil {
			continue
		}
		if item.Mode == mode || item.Mode == EvaluateEventDriven {
			selected = append(selected, item)
		}
	}
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Priority > selected[j].Priority })
	jobs := make(chan ScheduledStrategy)
	results := make(chan EvaluationResult, len(selected))
	var wg sync.WaitGroup
	workerCount := minInt(s.workers, maxInt(1, len(selected)))
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				start := time.Now()
				var signals []strategy.Signal
				if mode == EvaluateOnTick {
					signals = item.Strategy.OnTick(tick)
				} else {
					signals = item.Strategy.OnCandle(tick)
				}
				results <- EvaluationResult{StrategyID: item.ID, Signals: signals, Latency: time.Since(start)}
			}
		}()
	}
	for _, item := range selected {
		select {
		case <-ctx.Done():
			results <- EvaluationResult{StrategyID: item.ID, Error: ctx.Err()}
		case jobs <- item:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)
	out := make([]EvaluationResult, 0, len(selected))
	for result := range results {
		out = append(out, result)
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
