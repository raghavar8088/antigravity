package scalpers

import (
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"
)

// walkForwardWindow is the size of one evaluation window, in closed trades.
// Configurable via WALKFORWARD_MIN_TRADES (default 30).
var walkForwardWindow = wfEnvInt("WALKFORWARD_MIN_TRADES", 30)

// Promotion/demotion thresholds, configurable via env vars. Promotion now
// additionally requires walkForwardConsecutiveWindows passing windows in a
// row (not just one) before a strategy reaches ACTIVE — a single lucky
// 30-trade window is too small a sample to trust on its own.
var (
	walkForwardWinRateThreshold    = wfEnvFloat("WALKFORWARD_WIN_RATE_THRESHOLD", 0.48)
	walkForwardSharpeThreshold     = wfEnvFloat("WALKFORWARD_SHARPE_THRESHOLD", 0.60)
	walkForwardMaxDrawdown         = wfEnvFloat("WALKFORWARD_MAX_DRAWDOWN", 0.20) // reserved: this validator does not yet track drawdown
	walkForwardConsecutiveWindows  = wfEnvInt("WALKFORWARD_CONSECUTIVE_WINDOWS", 2)
	walkForwardFastDemoteSharpe    = 0.20
	walkForwardFastDemoteWinRate   = 0.30
)

func wfEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func wfEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

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
	Status                StrategyStatus `json:"status"`
	Sharpe                float64        `json:"sharpe"`
	WinRate               float64        `json:"win_rate"`
	Trades                int            `json:"trades"`
	AvgWinPct             float64        `json:"avg_win_pct"`  // mean PnLPct of winning trades
	AvgLossPct            float64        `json:"avg_loss_pct"` // mean abs(PnLPct) of losing trades
	WindowCount           int            `json:"window_count"`             // completed 30-trade windows
	ConsecutivePassCount  int            `json:"consecutive_pass_count"`   // consecutive windows clearing the promotion bar
	TradesInCurrentWindow int            `json:"trades_in_current_window"` // progress toward the next window boundary
}

// WalkForwardValidator tracks rolling 30-trade performance and promotes or demotes
// strategies based on Sharpe ratio and win rate thresholds. Promotion requires
// walkForwardConsecutiveWindows consecutive passing 30-trade windows (not a
// single window) to avoid promoting on a lucky streak.
type WalkForwardValidator struct {
	mu              sync.RWMutex
	history         map[string][]TradeResult
	status          map[string]StrategyStatus
	windowCount     map[string]int // completed (non-overlapping) 30-trade windows
	consecutivePass map[string]int // consecutive windows that cleared the promotion bar
	tradesInWindow  map[string]int // trades recorded since the last window boundary (0-29)
}

// NewWalkForwardValidator creates a ready-to-use validator.
func NewWalkForwardValidator() *WalkForwardValidator {
	return &WalkForwardValidator{
		history:         make(map[string][]TradeResult),
		status:          make(map[string]StrategyStatus),
		windowCount:     make(map[string]int),
		consecutivePass: make(map[string]int),
		tradesInWindow:  make(map[string]int),
	}
}

// RecordTrade records a closed trade outcome. Status is only re-evaluated at
// 30-trade window boundaries (not on every trade) — see the window-boundary
// comment below for why.
func (v *WalkForwardValidator) RecordTrade(strategyName string, pnlPct float64) {
	v.mu.Lock()
	defer v.mu.Unlock()

	hist := append(v.history[strategyName], TradeResult{PnLPct: pnlPct, ClosedAt: time.Now()})
	if len(hist) > walkForwardWindow {
		hist = hist[len(hist)-walkForwardWindow:]
	}
	v.history[strategyName] = hist
	v.tradesInWindow[strategyName]++

	if len(hist) < walkForwardWindow {
		if v.status[strategyName] == "" {
			v.status[strategyName] = StatusProbationary
		}
		return
	}

	// hist is a rolling 30-trade buffer, but window evaluation only fires
	// once per walkForwardWindow trades (a non-overlapping window boundary).
	// Without this gate, every single trade would re-evaluate the same
	// sliding window and "consecutive passing windows" would be meaningless
	// (every trade would count as one more "window").
	if v.tradesInWindow[strategyName] < walkForwardWindow {
		return
	}
	v.tradesInWindow[strategyName] = 0
	v.windowCount[strategyName]++

	sharpe := wfComputeSharpe(hist)
	winRate := wfComputeWinRate(hist)

	current := v.status[strategyName]
	if current == "" {
		current = StatusProbationary
	}

	next := current
	switch {
	case sharpe < walkForwardFastDemoteSharpe || winRate < walkForwardFastDemoteWinRate:
		// Fast demotion: any strategy whose window craters drops immediately —
		// it does not get to "spend" a consecutive-pass streak recovering from
		// a result this bad. Applies regardless of current status: an
		// already-DEMOTED or still-PROBATIONARY strategy this bad should not
		// keep accumulating trade history in live rotation either.
		next = StatusDemoted
		v.consecutivePass[strategyName] = 0
	case sharpe >= walkForwardSharpeThreshold && winRate >= walkForwardWinRateThreshold:
		v.consecutivePass[strategyName]++
		if v.consecutivePass[strategyName] >= walkForwardConsecutiveWindows {
			next = StatusActive
		}
	default:
		// Mediocre window: neither a clear pass nor a crater. Resets the
		// promotion streak. A DEMOTED strategy stays DEMOTED on a merely
		// so-so window — only a fresh consecutive-pass streak recovers it.
		v.consecutivePass[strategyName] = 0
		if current != StatusDemoted {
			next = StatusProbationary
		}
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
			log.Printf("[WALKFORWARD] Strategy %s promoted to ACTIVE after %d consecutive passing windows (Sharpe=%.2f, WinRate=%.2f)",
				strategyName, v.consecutivePass[strategyName], sharpe, winRate)
		case StatusDemoted:
			if sharpe < walkForwardFastDemoteSharpe || winRate < walkForwardFastDemoteWinRate {
				log.Printf("[WALKFORWARD] Strategy %s fast-demoted: Sharpe=%.2f WinRate=%.2f", strategyName, sharpe, winRate)
			} else {
				log.Printf("[WALKFORWARD] Strategy %s DEMOTED (Sharpe=%.2f, WinRate=%.2f) — removed from live rotation",
					strategyName, sharpe, winRate)
			}
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
		avgWin, avgLoss := wfComputeAvgWinLoss(hist)
		if len(hist) >= walkForwardWindow {
			sharpe = wfComputeSharpe(hist)
		}
		out[name] = WalkForwardSummary{
			Status:                s,
			Sharpe:                sharpe,
			WinRate:               winRate,
			Trades:                len(hist),
			AvgWinPct:             avgWin,
			AvgLossPct:            avgLoss,
			WindowCount:           v.windowCount[name],
			ConsecutivePassCount:  v.consecutivePass[name],
			TradesInCurrentWindow: v.tradesInWindow[name],
		}
	}
	return out
}

// GetSummary returns the current snapshot for a single strategy and whether
// any trade history has been recorded for it yet. Used by Kelly sizing to
// decide between live trade statistics and a conservative probationary
// fallback (see evalAndExecuteScalers in the trading package).
func (v *WalkForwardValidator) GetSummary(strategyName string) (WalkForwardSummary, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	hist, ok := v.history[strategyName]
	if !ok || len(hist) == 0 {
		return WalkForwardSummary{}, false
	}
	s := v.status[strategyName]
	if s == "" {
		s = StatusProbationary
	}
	sharpe := 0.0
	if len(hist) >= walkForwardWindow {
		sharpe = wfComputeSharpe(hist)
	}
	avgWin, avgLoss := wfComputeAvgWinLoss(hist)
	return WalkForwardSummary{
		Status:                s,
		Sharpe:                sharpe,
		WinRate:               wfComputeWinRate(hist),
		Trades:                len(hist),
		AvgWinPct:             avgWin,
		AvgLossPct:            avgLoss,
		WindowCount:           v.windowCount[strategyName],
		ConsecutivePassCount:  v.consecutivePass[strategyName],
		TradesInCurrentWindow: v.tradesInWindow[strategyName],
	}, true
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

// wfComputeAvgWinLoss returns the mean PnLPct of winning trades and the mean
// absolute PnLPct of losing trades (always >= 0). Either is 0 if there are
// no trades of that kind in hist.
func wfComputeAvgWinLoss(hist []TradeResult) (avgWin, avgLoss float64) {
	var winSum, lossSum float64
	var winN, lossN int
	for _, t := range hist {
		if t.PnLPct > 0 {
			winSum += t.PnLPct
			winN++
		} else if t.PnLPct < 0 {
			lossSum += -t.PnLPct
			lossN++
		}
	}
	if winN > 0 {
		avgWin = winSum / float64(winN)
	}
	if lossN > 0 {
		avgLoss = lossSum / float64(lossN)
	}
	return avgWin, avgLoss
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
	if len(vals) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		d := v - mean
		sum += d * d
	}
	// Bessel's correction: divide by N-1 (sample stddev) not N (population).
	// For N=30 this reduces Sharpe overestimation from ~1.7% to 0%.
	return math.Sqrt(sum / float64(len(vals)-1))
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
