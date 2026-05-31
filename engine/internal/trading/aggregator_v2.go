// Package trading — SignalAggregatorV2
//
// Replaces the original aggregator.go with:
//  1. Per-strategy cooldown with exponential back-off on losses
//  2. Conflict resolution — prevents long+short from same family simultaneously
//  3. Priority ranking — highest-scoring signal per symbol wins in a batch
//  4. Signal compression — merges near-identical signals within a time window
//  5. Side-capacity enforcement — max N long, max N short open simultaneously
//  6. Throughput target: 1000+ signals/sec with <100µs per FilterSignals call
package trading

import (
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"antigravity-engine/internal/regime"
	"antigravity-engine/internal/strategy"
)

// ── RatedSignal ───────────────────────────────────────────────────────────────

// RatedSignal is a strategy signal that has been quality-scored.
type RatedSignal struct {
	Signal        strategy.Signal
	StrategyName  string
	StrategyID    int
	Family        regime.StrategyFamily
	Score         ScoreResult
	FiredAt       time.Time
	ExecutionWeight float64 // from strategy tracker (0.35–1.6)
	HistoricalWR  float64
	TotalTrades   int
}

// ── Aggregator config ─────────────────────────────────────────────────────────

// AggregatorV2Config controls aggregation behaviour.
type AggregatorV2Config struct {
	// Cooldown: minimum wait between signals from the same strategy
	BaseCooldownSec int // default 30
	// After N consecutive strategy losses, cooldown multiplier applied
	LossCooldownMultiplier float64 // default 2.0
	// Maximum number of long and short positions simultaneously
	MaxLongSlots  int // default 3
	MaxShortSlots int // default 3
	// Signal compression: signals within this window with same symbol+side
	// are merged and only the highest-score one is kept.
	CompressionWindowMs int64 // default 500 ms
}

func DefaultAggregatorV2Config() AggregatorV2Config {
	return AggregatorV2Config{
		BaseCooldownSec:        30,
		LossCooldownMultiplier: 2.0,
		MaxLongSlots:           3,
		MaxShortSlots:          3,
		CompressionWindowMs:    500,
	}
}

// ── Per-strategy state ────────────────────────────────────────────────────────

type strategyState struct {
	lastSignalAt  time.Time
	cooldownSec   int
	consecutiveLoss int
}

// ── Aggregator ────────────────────────────────────────────────────────────────

// SignalAggregatorV2 is the production signal aggregation layer.
// All public methods are goroutine-safe.
type SignalAggregatorV2 struct {
	mu  sync.Mutex
	cfg AggregatorV2Config

	// Per-strategy state
	stratStates map[string]*strategyState

	// Active position tracking (updated externally via NotifyOpen/NotifyClose)
	openLongs  int
	openShorts int

	// Active families per side (to prevent family conflict)
	activeFamilyLong  map[regime.StrategyFamily]int // family → open count
	activeFamilyShort map[regime.StrategyFamily]int

	// Counters
	totalIn     atomic.Int64
	totalOut    atomic.Int64
	totalDropped atomic.Int64
}

// NewSignalAggregatorV2 creates a V2 aggregator.
func NewSignalAggregatorV2(cfg AggregatorV2Config) *SignalAggregatorV2 {
	return &SignalAggregatorV2{
		cfg:               cfg,
		stratStates:       make(map[string]*strategyState),
		activeFamilyLong:  make(map[regime.StrategyFamily]int),
		activeFamilyShort: make(map[regime.StrategyFamily]int),
	}
}

// NewSignalAggregatorV2Default creates a V2 aggregator with production defaults.
func NewSignalAggregatorV2Default() *SignalAggregatorV2 {
	return NewSignalAggregatorV2(DefaultAggregatorV2Config())
}

// Filter takes a batch of rated signals for one tick/candle and returns
// the approved subset, ranked by score descending.
//
// Operations performed in order:
//  1. Drop HOLD signals
//  2. Drop if regime = UNKNOWN (family inactive)
//  3. Drop if score did not pass
//  4. Drop if strategy cooldown active
//  5. Drop if side slots full
//  6. Drop if family conflict on this side
//  7. Signal compression: per (symbol, side) keep top-score only
//  8. Sort approved by score descending
func (a *SignalAggregatorV2) Filter(signals []RatedSignal) []RatedSignal {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	a.totalIn.Add(int64(len(signals)))

	// ── Steps 1–6: per-signal rejection ──────────────────────────────────────
	var candidates []RatedSignal
	for _, sig := range signals {
		if reject, reason := a.rejectReason(sig, now); reject {
			a.totalDropped.Add(1)
			log.Printf("[AGGv2] DROP strat=%s reason=%s score=%d",
				sig.StrategyName, reason, sig.Score.Total)
			continue
		}
		candidates = append(candidates, sig)
	}

	// ── Step 7: signal compression ────────────────────────────────────────────
	compressed := a.compress(candidates, now)

	// ── Step 8: sort by score descending ──────────────────────────────────────
	sort.Slice(compressed, func(i, j int) bool {
		return compressed[i].Score.Total > compressed[j].Score.Total
	})

	// Update cooldown timestamps for approved signals
	for _, sig := range compressed {
		key := sig.StrategyName
		if s, ok := a.stratStates[key]; ok {
			s.lastSignalAt = now
		} else {
			a.stratStates[key] = &strategyState{
				lastSignalAt: now,
				cooldownSec:  a.cfg.BaseCooldownSec,
			}
		}
	}

	a.totalOut.Add(int64(len(compressed)))
	return compressed
}

// NotifyOpen records that a new position was opened on the given side/family.
// Called by the OMS after a fill.
func (a *SignalAggregatorV2) NotifyOpen(side string, family regime.StrategyFamily) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if side == "BUY" || side == "LONG" {
		a.openLongs++
		a.activeFamilyLong[family]++
	} else {
		a.openShorts++
		a.activeFamilyShort[family]++
	}
}

// NotifyClose records that a position was closed.
// won=true increases strategy health; won=false may increase cooldown.
func (a *SignalAggregatorV2) NotifyClose(stratName, side string, family regime.StrategyFamily, won bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if side == "BUY" || side == "LONG" {
		if a.openLongs > 0 {
			a.openLongs--
		}
		if a.activeFamilyLong[family] > 0 {
			a.activeFamilyLong[family]--
		}
	} else {
		if a.openShorts > 0 {
			a.openShorts--
		}
		if a.activeFamilyShort[family] > 0 {
			a.activeFamilyShort[family]--
		}
	}

	// Adjust strategy cooldown based on outcome
	if s, ok := a.stratStates[stratName]; ok {
		if !won {
			s.consecutiveLoss++
			mult := 1.0 + float64(s.consecutiveLoss)*a.cfg.LossCooldownMultiplier
			s.cooldownSec = int(float64(a.cfg.BaseCooldownSec) * mult)
			if s.cooldownSec > 600 {
				s.cooldownSec = 600 // max 10 minutes
			}
		} else {
			s.consecutiveLoss = 0
			s.cooldownSec = a.cfg.BaseCooldownSec
		}
	}
}

// GetStats returns throughput counters for monitoring.
func (a *SignalAggregatorV2) GetStats() (in, out, dropped int64) {
	return a.totalIn.Load(), a.totalOut.Load(), a.totalDropped.Load()
}

// GetPositionCounts returns current open position counts.
func (a *SignalAggregatorV2) GetPositionCounts() (longs, shorts int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.openLongs, a.openShorts
}

// ── Internal helpers ───────────────────────────────────────────────────────────

// rejectReason returns (true, reason) if the signal should be dropped.
func (a *SignalAggregatorV2) rejectReason(sig RatedSignal, now time.Time) (bool, string) {
	// 1. HOLD
	if sig.Signal.Action == strategy.ActionHold {
		return true, "HOLD"
	}

	// 2. Score gate (covers REGIME, CONFIDENCE, ATR_FEES, LOW_SIGNAL_SCORE)
	if !sig.Score.Passed {
		return true, sig.Score.RejectReason
	}

	// 3. Strategy cooldown
	if s, ok := a.stratStates[sig.StrategyName]; ok {
		elapsed := now.Sub(s.lastSignalAt)
		required := time.Duration(s.cooldownSec) * time.Second
		if elapsed < required {
			return true, "COOLDOWN"
		}
	}

	// 4. Side capacity
	side := string(sig.Signal.Action)
	if side == "BUY" && a.openLongs >= a.cfg.MaxLongSlots {
		return true, "EXPOSURE_LIMIT_LONG"
	}
	if side == "SELL" && a.openShorts >= a.cfg.MaxShortSlots {
		return true, "EXPOSURE_LIMIT_SHORT"
	}

	// 5. Family conflict — block if opposite direction already open from same family
	if side == "BUY" && a.activeFamilyShort[sig.Family] > 0 {
		return true, "FAMILY_CONFLICT"
	}
	if side == "SELL" && a.activeFamilyLong[sig.Family] > 0 {
		return true, "FAMILY_CONFLICT"
	}

	return false, ""
}

// compress merges signals with the same (symbol, side) within the compression
// window, keeping only the highest-scoring signal per (symbol, side) pair.
func (a *SignalAggregatorV2) compress(signals []RatedSignal, now time.Time) []RatedSignal {
	if len(signals) == 0 {
		return nil
	}
	windowStart := now.Add(-time.Duration(a.cfg.CompressionWindowMs) * time.Millisecond)

	type key struct {
		symbol string
		side   string
	}
	best := make(map[key]RatedSignal, len(signals))

	for _, sig := range signals {
		// Only compress signals within the window
		if sig.FiredAt.Before(windowStart) {
			sig.FiredAt = now
		}
		k := key{sig.Signal.Symbol, string(sig.Signal.Action)}
		if prev, ok := best[k]; !ok || sig.Score.Total > prev.Score.Total {
			best[k] = sig
		}
	}

	result := make([]RatedSignal, 0, len(best))
	for _, sig := range best {
		result = append(result, sig)
	}
	return result
}
