package risk

import (
	"sync"

	riskv2 "antigravity-engine/internal/risk/v2"
)

// StrategyStats holds per-strategy lifetime performance counters.
type StrategyStats struct {
	TotalTrades  int
	Wins         int
	Losses       int
	TotalPnL     float64
	Category     string
	SignalsFired int
}

// StrategyTracker tracks per-strategy signals, trade results, and execution
// quality so the trading loop can gate and size positions intelligently.
type StrategyTracker struct {
	mu sync.RWMutex

	// Per-strategy state
	enabled         map[string]bool
	executionWeight map[string]float64
	stats           map[string]*StrategyStats

	// Metadata (informational; used for logging / HTTP endpoints)
	categories map[string]string
	timeframes map[string]string
}

// NewStrategyTracker initialises a tracker for the given strategy roster.
// Every strategy starts enabled with a default execution weight of 1.0.
func NewStrategyTracker(names, categories, timeframes []string, _ float64) *StrategyTracker {
	t := &StrategyTracker{
		enabled:         make(map[string]bool, len(names)),
		executionWeight: make(map[string]float64, len(names)),
		stats:           make(map[string]*StrategyStats, len(names)),
		categories:      make(map[string]string, len(names)),
		timeframes:      make(map[string]string, len(names)),
	}
	for i, name := range names {
		t.enabled[name] = true
		t.executionWeight[name] = 1.0
		t.stats[name] = &StrategyStats{}
		if i < len(categories) {
			t.categories[name] = categories[i]
		}
		if i < len(timeframes) {
			t.timeframes[name] = timeframes[i]
		}
	}
	return t
}

// IsEnabled returns true if the strategy is active (not cooled down / disabled).
// Unknown strategies are enabled by default so new scalers can trade immediately.
func (t *StrategyTracker) IsEnabled(name string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.enabled[name]
	if !ok {
		return true // new / unregistered strategy — allow by default
	}
	return v
}

// Disable temporarily disables a strategy (e.g. during cooldown).
func (t *StrategyTracker) Disable(name string) {
	t.mu.Lock()
	t.enabled[name] = false
	t.mu.Unlock()
}

// Enable re-enables a strategy after cooldown.
func (t *StrategyTracker) Enable(name string) {
	t.mu.Lock()
	t.enabled[name] = true
	t.mu.Unlock()
}

// GetExecutionWeight returns the quality-adjusted execution weight (0–1).
// Returns 1.0 for unknown strategies (new scalers start at full weight).
func (t *StrategyTracker) GetExecutionWeight(name string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	w, ok := t.executionWeight[name]
	if !ok {
		return 1.0 // default for unregistered strategies
	}
	return w
}

// SetExecutionWeight updates the quality-based execution weight for a strategy.
func (t *StrategyTracker) SetExecutionWeight(name string, w float64) {
	t.mu.Lock()
	t.executionWeight[name] = w
	t.mu.Unlock()
}

// GetStats returns lifetime performance stats for a strategy.
// Returns (zero, false) if the strategy is not tracked yet.
func (t *StrategyTracker) GetStats(name string) (StrategyStats, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.stats[name]
	if !ok {
		return StrategyStats{}, false
	}
	out := *s
	out.Category = t.categories[name]
	return out, true
}

// ReEnableExpired re-enables any strategies that were temporarily disabled and
// whose cooldown period has elapsed. This is called periodically from the main loop.
// The base tracker has no time-based cooldown; the method is a no-op stub here
// since cooldown logic lives in the aggregator.
func (t *StrategyTracker) ReEnableExpired() {}

// ResetDaily resets per-day counters for all strategies.
// Called at midnight UTC by the daily reset goroutine.
// The base tracker keeps lifetime stats; this is a no-op unless day-scoped
// fields are added later.
func (t *StrategyTracker) ResetDaily() {}

// GetAllStats returns a copy of all per-strategy stats keyed by strategy name.
// Safe for concurrent use; called from the /api/strategies HTTP handler.
func (t *StrategyTracker) GetAllStats() map[string]StrategyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]StrategyStats, len(t.stats))
	for name, s := range t.stats {
		cp := *s
		cp.Category = t.categories[name]
		out[name] = cp
	}
	return out
}

// RecordSignal records that a strategy generated a signal this cycle.
func (t *StrategyTracker) RecordSignal(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.stats[name]; ok {
		s.SignalsFired++
	}
}

// Reset clears all per-strategy state so the tracker starts fresh after a full account reset.
func (t *StrategyTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for name := range t.stats {
		t.stats[name] = &StrategyStats{}
		t.enabled[name] = true
		t.executionWeight[name] = 1.0
	}
}

// RecordTradeResult records the net PnL of a closed trade for the strategy.
// This keeps the tracker's win-rate and PnL counters current.
func (t *StrategyTracker) RecordTradeResult(name string, netPnL float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s, ok := t.stats[name]
	if !ok {
		s = &StrategyStats{}
		t.stats[name] = s
	}
	s.TotalTrades++
	s.TotalPnL += netPnL
	if netPnL > 0 {
		s.Wins++
	} else {
		s.Losses++
	}

	// Update execution weight from live win rate.
	// Below 40% win rate: reduce weight. Above 55%: boost slightly.
	if s.TotalTrades >= 10 {
		wr := float64(s.Wins) / float64(s.TotalTrades)
		switch {
		case wr < 0.40:
			t.executionWeight[name] = 0.60
		case wr < 0.45:
			t.executionWeight[name] = 0.80
		case wr > 0.55:
			t.executionWeight[name] = 1.10
		default:
			t.executionWeight[name] = 1.0
		}
	}
}

// BuildRiskMetrics constructs a riskv2.StrategyMetrics snapshot from current tracker state.
// Used by PreTradeRiskPipeline to feed per-strategy performance into the V2 risk engine.
func (t *StrategyTracker) BuildRiskMetrics(name string) riskv2.StrategyMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()
	m := riskv2.StrategyMetrics{Strategy: name}
	s, ok := t.stats[name]
	if !ok {
		return m
	}
	m.TotalTrades = s.TotalTrades
	if s.TotalTrades > 0 {
		m.WinRate = float64(s.Wins) / float64(s.TotalTrades)
	}
	if s.TotalTrades >= 10 && s.Losses > 0 {
		m.ProfitFactor = float64(s.Wins) / float64(s.Losses)
	}
	return m
}
