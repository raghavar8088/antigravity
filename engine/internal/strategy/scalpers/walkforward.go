package scalpers

import (
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

const walkForwardWindow = 30

// StrategyStatus represents the lifecycle state of a strategy in the walk-forward validator.
type StrategyStatus string

const (
	StatusProbationary StrategyStatus = "PROBATIONARY"
	StatusActive       StrategyStatus = "ACTIVE"
	StatusDemoted      StrategyStatus = "DEMOTED"
)

// TradeResult holds the outcome of a single closed trade.
type TradeResult struct {
	PnLPct   float64
	ClosedAt time.Time
}

// WalkForwardSummary is the per-strategy snapshot returned by Summary().
type WalkForwardSummary struct {
	Status  StrategyStatus `json:"status"`
	Sharpe  float64        `json:"sharpe"`
	WinRate float64        `json:"win_rate"`
	Trades  int            `json:"trades"`
}

// WalkForwardValidator tracks rolling 30-trade performance and promotes or demotes
// strategies based on Sharpe ratio and win rate thresholds.
type WalkForwardValidator struct {
	mu      sync.RWMutex
	history map[string][]TradeResult
	status  map[string]StrategyStatus
}

// NewWalkForwardValidator creates a ready-to-use validator.
func NewWalkForwardValidator() *WalkForwardValidator {
	return &WalkForwardValidator{
		history: make(map[string][]TradeResult),
		status:  make(map[string]StrategyStatus),
	}
}

// RecordTrade records a closed trade outcome and re-evaluates the strategy status.
func (v *WalkForwardValidator) RecordTrade(strategyName string, pnlPct float64) {
	v.mu.Lock()
	defer v.mu.Unlock()

	hist := append(v.history[strategyName], TradeResult{PnLPct: pnlPct, ClosedAt: time.Now()})
	if len(hist) > walkForwardWindow {
		hist = hist[len(hist)-walkForwardWindow:]
	}
	v.history[strategyName] = hist

	if len(hist) < walkForwardWindow {
		v.status[strategyName] = StatusProbationary
		return
	}

	sharpe := wfComputeSharpe(hist)
	winRate := wfComputeWinRate(hist)

	current := v.status[strategyName]
	if current == "" {
		current = StatusProbationary
	}

	var next StrategyStatus
	switch {
	case sharpe >= 0.50 && winRate >= 0.45:
		next = StatusActive
	case sharpe < 0.30 || winRate < 0.35:
		next = StatusDemoted
	default:
		next = current
	}

	// Safety rule: never leave 0 active strategies.
	if next == StatusDemoted {
		activeCount := 0
		for name, s := range v.status {
			if name != strategyName && s == StatusActive {
				activeCount++
			}
		}
		if activeCount == 0 {
			sharpes := make(map[string]float64)
			for name, h := range v.history {
				if len(h) >= walkForwardWindow {
					sharpes[name] = wfComputeSharpe(h)
				}
			}
			sharpes[strategyName] = sharpe
			topTwo := wfTopNBySharpe(sharpes, 2)
			for _, n := range topTwo {
				if n == strategyName {
					next = StatusActive
					break
				}
			}
		}
	}

	old := v.status[strategyName]
	v.status[strategyName] = next

	if old != next {
		switch next {
		case StatusActive:
			log.Printf("[WALKFORWARD] Strategy %s promoted to ACTIVE (Sharpe=%.2f, WinRate=%.2f)",
				strategyName, sharpe, winRate)
		case StatusDemoted:
			log.Printf("[WALKFORWARD] Strategy %s DEMOTED (Sharpe=%.2f, WinRate=%.2f) — removed from live rotation",
				strategyName, sharpe, winRate)
		}
	}
}

// IsActive returns true for PROBATIONARY and ACTIVE strategies; false for DEMOTED.
func (v *WalkForwardValidator) IsActive(strategyName string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s := v.status[strategyName]
	return s == StatusActive || s == StatusProbationary || s == ""
}

// GetStatus returns the current status for a strategy.
func (v *WalkForwardValidator) GetStatus(strategyName string) StrategyStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()
	s := v.status[strategyName]
	if s == "" {
		return StatusProbationary
	}
	return s
}

// GetSharpe returns the rolling Sharpe ratio for a strategy (0 if fewer than 30 trades).
func (v *WalkForwardValidator) GetSharpe(strategyName string) float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	hist := v.history[strategyName]
	if len(hist) < walkForwardWindow {
		return 0
	}
	return wfComputeSharpe(hist)
}

// GetWinRate returns the rolling win rate for a strategy.
func (v *WalkForwardValidator) GetWinRate(strategyName string) float64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return wfComputeWinRate(v.history[strategyName])
}

// Summary returns a snapshot of all tracked strategies.
func (v *WalkForwardValidator) Summary() map[string]WalkForwardSummary {
	v.mu.RLock()
	defer v.mu.RUnlock()
	out := make(map[string]WalkForwardSummary, len(v.status))
	for name, s := range v.status {
		hist := v.history[name]
		sharpe := 0.0
		winRate := wfComputeWinRate(hist)
		if len(hist) >= walkForwardWindow {
			sharpe = wfComputeSharpe(hist)
		}
		out[name] = WalkForwardSummary{
			Status:  s,
			Sharpe:  sharpe,
			WinRate: winRate,
			Trades:  len(hist),
		}
	}
	return out
}

// ── internal helpers ──────────────────────────────────────────────────────────

func wfComputeSharpe(hist []TradeResult) float64 {
	if len(hist) == 0 {
		return 0
	}
	pnls := make([]float64, len(hist))
	for i, t := range hist {
		pnls[i] = t.PnLPct
	}
	mean := wfAvg(pnls)
	sd := wfStddev(pnls, mean)
	if sd == 0 {
		return 0
	}
	return (mean / sd) * math.Sqrt(252)
}

func wfComputeWinRate(hist []TradeResult) float64 {
	if len(hist) == 0 {
		return 0
	}
	wins := 0
	for _, t := range hist {
		if t.PnLPct > 0 {
			wins++
		}
	}
	return float64(wins) / float64(len(hist))
}

func wfAvg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func wfStddev(vals []float64, mean float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(vals)))
}

func wfTopNBySharpe(sharpes map[string]float64, n int) []string {
	type kv struct {
		name   string
		sharpe float64
	}
	kvs := make([]kv, 0, len(sharpes))
	for k, v := range sharpes {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].sharpe > kvs[j].sharpe })
	out := make([]string, 0, n)
	for i, item := range kvs {
		if i >= n {
			break
		}
		out = append(out, item.name)
	}
	return out
}
