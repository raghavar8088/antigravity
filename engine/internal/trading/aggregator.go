package trading

import (
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/strategy"
)

// AggregatedSignal wraps a raw signal with the originating strategy name.
type AggregatedSignal struct {
	Signal          strategy.Signal
	StrategyName    string
	Category        string
	FiredAt         time.Time
	ExecutionWeight float64
	TotalTrades     int
	WinRate         float64
	TotalPnL        float64
}

// SignalAggregator collects signals from all strategies on each tick,
// applies cooldown filters, and emits deduplicated actionable signals.
type SignalAggregator struct {
	mu sync.Mutex

	// Cooldown: minimum seconds between signals from the same strategy
	cooldownSec int
	lastSignal  map[string]time.Time // strategyName -> last signal time

	// Stats tracking for logging
	totalSignals    int64
	filteredSignals int64
	flowMetrics     *SignalFlowMetrics
}

func NewSignalAggregator(cooldownSeconds int) *SignalAggregator {
	return &SignalAggregator{
		cooldownSec: cooldownSeconds,
		lastSignal:  make(map[string]time.Time),
		flowMetrics: NewSignalFlowMetrics(),
	}
}

// FilterSignals takes raw signals from all strategies for a given tick
// and returns only the ones that pass through cooldown and deduplication.
// In AGGRESSIVE mode: every individual strategy signal is allowed through
// (subject only to cooldown).
func (a *SignalAggregator) FilterSignals(rawSignals []AggregatedSignal) []AggregatedSignal {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	var approved []AggregatedSignal

	for _, sig := range rawSignals {
		a.totalSignals++

		// Skip HOLD signals
		if sig.Signal.Action == strategy.ActionHold {
			continue
		}

		// Cooldown check: has this strategy fired too recently?
		if lastFired, ok := a.lastSignal[sig.StrategyName]; ok {
			elapsed := now.Sub(lastFired)
			if elapsed < time.Duration(a.cooldownSec)*time.Second {
				a.filteredSignals++
				continue
			}
		}

		// Passed all filters - approve
		sig.FiredAt = now
		a.lastSignal[sig.StrategyName] = now
		approved = append(approved, sig)

		log.Printf("[AGGREGATOR] APPROVED: %s → %s %.4f %s",
			sig.StrategyName, sig.Signal.Action, sig.Signal.TargetSize, sig.Signal.Symbol)
	}

	return approved
}

// GetStats returns aggregator statistics.
func (a *SignalAggregator) GetStats() (total int64, filtered int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totalSignals, a.filteredSignals
}

func (a *SignalAggregator) RecordSignalFlowStage(stage string, input, output int) {
	if a == nil || a.flowMetrics == nil {
		return
	}
	a.flowMetrics.RecordStage(stage, input, output)
}

func (a *SignalAggregator) RecordSignalFlowRejection(stage, reason, category string) {
	if a == nil || a.flowMetrics == nil {
		return
	}
	a.flowMetrics.RecordRejection(stage, reason, category)
}

func (a *SignalAggregator) GetSignalFlowSnapshot() SignalFlowSnapshot {
	if a == nil || a.flowMetrics == nil {
		return SignalFlowSnapshot{}
	}
	return a.flowMetrics.Snapshot()
}

func (a *SignalAggregator) RecordStrategyApproval(strategyName, category string) {
	if a == nil || a.flowMetrics == nil {
		return
	}
	a.flowMetrics.RecordStrategyApproval(strategyName, category)
}

func (a *SignalAggregator) RecordStrategyExecution(strategyName string) {
	if a == nil || a.flowMetrics == nil {
		return
	}
	a.flowMetrics.RecordStrategyExecution(strategyName)
}

// GetSignalFlowDiagnostics returns the full Phase 22A diagnostics snapshot
// including per-strategy approval/execution counts and top bottleneck ranking.
func (a *SignalAggregator) GetSignalFlowDiagnostics() SignalFlowDiagnostics {
	if a == nil || a.flowMetrics == nil {
		return SignalFlowDiagnostics{}
	}
	return a.flowMetrics.Diagnostics()
}
