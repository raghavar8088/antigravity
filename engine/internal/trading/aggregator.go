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

// CooldownForTimeframe returns the recommended signal cooldown for a given candle timeframe.
// Higher-timeframe strategies need longer cooldowns to avoid thrashing on slow signals.
func CooldownForTimeframe(tf string) time.Duration {
	switch tf {
	case "1m":
		return 30 * time.Second
	case "5m":
		return 2 * time.Minute
	case "15m":
		return 5 * time.Minute
	case "1h":
		return 15 * time.Minute
	default:
		return 15 * time.Second
	}
}

// SignalAggregator collects signals from all strategies on each tick,
// applies cooldown filters, and emits deduplicated actionable signals.
type SignalAggregator struct {
	mu sync.Mutex

	// defaultCooldown applies when no per-strategy override is registered.
	defaultCooldown   time.Duration
	strategyCooldowns map[string]time.Duration // strategyName -> cooldown override
	lastSignal        map[string]time.Time     // strategyName -> last signal time

	// Stats tracking for logging
	totalSignals    int64
	filteredSignals int64
	flowMetrics     *SignalFlowMetrics
}

func NewSignalAggregator(defaultSeconds int) *SignalAggregator {
	return &SignalAggregator{
		defaultCooldown:   time.Duration(defaultSeconds) * time.Second,
		strategyCooldowns: make(map[string]time.Duration),
		lastSignal:        make(map[string]time.Time),
		flowMetrics:       NewSignalFlowMetrics(),
	}
}

// SetStrategyCooldown registers a per-strategy cooldown override.
// Call once per strategy after construction, using CooldownForTimeframe(entry.Timeframe).
func (a *SignalAggregator) SetStrategyCooldown(strategyName string, d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.strategyCooldowns[strategyName] = d
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
			cd := a.defaultCooldown
			if override, ok2 := a.strategyCooldowns[sig.StrategyName]; ok2 {
				cd = override
			}
			if now.Sub(lastFired) < cd {
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
