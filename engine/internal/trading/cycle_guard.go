package trading

import (
	"log/slog"
	"sync/atomic"
)

// CycleGuard prevents evaluation cycles from overlapping. If the previous
// 15-minute cycle has not finished by the time the next tick fires, the new
// cycle is skipped and a metric is incremented.
//
// Usage:
//
//	if !guard.TryStart(cycleID) {
//	    return // skip overlapping cycle
//	}
//	defer guard.Finish(cycleID)
type CycleGuard struct {
	running     atomic.Bool
	lastCycleID atomic.Value // stores string
}

// TryStart attempts to mark cycleID as the active cycle.
// Returns true if the cycle should proceed, false if a cycle is already running.
func (g *CycleGuard) TryStart(cycleID string) bool {
	if !g.running.CompareAndSwap(false, true) {
		last := ""
		if v := g.lastCycleID.Load(); v != nil {
			last, _ = v.(string)
		}
		slog.Warn("[trading] cycle overlap detected — skipping cycle",
			"skipped_cycle", cycleID,
			"running_cycle", last,
		)
		return false
	}
	g.lastCycleID.Store(cycleID)
	return true
}

// Finish marks the cycle as complete. Must be called via defer at the top
// of every cycle execution to ensure the guard is released even on panic.
func (g *CycleGuard) Finish(cycleID string) {
	g.running.Store(false)
	slog.Debug("[trading] cycle finished", "cycle_id", cycleID)
}
