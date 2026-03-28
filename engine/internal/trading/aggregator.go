package trading

import (
	"log"
	"sync"
	"time"

	"antigravity-engine/internal/strategy"
)

// AggregatedSignal wraps a raw signal with the originating strategy name.
type AggregatedSignal struct {
	Signal       strategy.Signal
	StrategyName string
	Category     string
	FiredAt      time.Time
}

// SignalAggregator collects signals from all strategies on each tick,
// applies cooldown filters, and emits deduplicated actionable signals.
type SignalAggregator struct {
	mu sync.Mutex

	// Cooldown: minimum seconds between signals from the same strategy
	cooldownSec int
	lastSignal  map[string]time.Time // strategyName -> last signal time

	// Stats tracking for logging
	totalSignals   int64
	filteredSignals int64
}

func NewSignalAggregator(cooldownSeconds int) *SignalAggregator {
	return &SignalAggregator{
		cooldownSec: cooldownSeconds,
		lastSignal:  make(map[string]time.Time),
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
