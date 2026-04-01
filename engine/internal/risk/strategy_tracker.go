package risk

import (
	"log"
	"math"
	"sync"
	"time"
)

const (
	poorPerformanceMinTrades  = 6
	poorPerformanceMinWinRate = 0.35

	defaultSizingMultiplier      = 1.0
	coldStartSizingMultiplier    = 0.85
	minSizingMultiplier          = 0.35
	maxSizingMultiplier          = 1.60
	maxEarlyBoostMultiplier      = 1.05
	lossStreakPenaltyPerLoss     = 0.15
	strongAvgPnLThresholdUSD     = 4.0
	mildAvgPnLThresholdUSD       = 1.0
	strongAvgPnLPenaltyThreshold = -4.0
	mildAvgPnLPenaltyThreshold   = -1.0

	defaultExecutionWeight      = 1.0
	coldStartExecutionWeight    = 1.10 // Boosted: give new strategies a head start (was 0.90)
	minExecutionWeight          = 0.20
	maxExecutionWeight          = 1.35
	matureExecutionMinTrades    = 8
	matureExecutionMinWinRate   = 0.42
	strongExecutionWinRate      = 0.60
	executionLossPenaltyPerLoss = 0.12
)

// StrategyStats tracks live performance metrics for a single strategy.
type StrategyStats struct {
	Name              string    `json:"name"`
	Category          string    `json:"category"`
	Timeframe         string    `json:"timeframe"`
	TotalTrades       int       `json:"totalTrades"`
	Wins              int       `json:"wins"`
	Losses            int       `json:"losses"`
	ConsecutiveLosses int       `json:"consecutiveLosses"`
	DailyPnL          float64   `json:"dailyPnl"`
	TotalPnL          float64   `json:"totalPnl"`
	Disabled          bool      `json:"disabled"`
	DisabledUntil     time.Time `json:"disabledUntil"`
	Allocation        float64   `json:"allocation"`
	SignalCount       int64     `json:"signalCount"`
	LastSignalTime    time.Time `json:"lastSignalTime"`
	Status            string    `json:"status"`
}

// StrategyTracker maintains per-strategy performance state.
type StrategyTracker struct {
	mu    sync.RWMutex
	stats map[string]*StrategyStats

	// Global config
	maxConsecutiveLosses int
	cooldownDuration     time.Duration
	dailyLossLimit       float64 // per-strategy daily loss limit in USD
}

// NewStrategyTracker initializes tracking for all given strategies.
func NewStrategyTracker(strategyNames []string, categories []string, timeframes []string, totalCapital float64) *StrategyTracker {
	stats := make(map[string]*StrategyStats)

	categoryWeights := map[string]float64{
		"Trend":              1.5,
		"Mean Reversion":     1.3,
		"Mean Rev Elite":     1.25,
		"Breakout":           1.2,
		"Breakout Elite":     1.2,
		"Momentum":           1.1,
		"Momentum Elite":     1.1,
		"Microstructure":     1.0,
		"Velocity":           0.9,
		"Statistical":        1.1,
		"Volatility":         0.95,
		"Time-of-Day":        0.95,
		"Smart Money":        1.0,
		"Price Action":       1.0,
		"Price Action Elite": 1.0,
		"Adaptive":           1.15,
		"Adaptive Elite":     1.15,
		"Multi-Signal":       1.3,
	}

	totalWeight := 0.0
	for i := range strategyNames {
		cat := "Unknown"
		if i < len(categories) {
			cat = categories[i]
		}
		w, ok := categoryWeights[cat]
		if !ok {
			w = 1.0
		}
		totalWeight += w
	}

	for i, name := range strategyNames {
		cat := "Unknown"
		tf := "1m"
		if i < len(categories) {
			cat = categories[i]
		}
		if i < len(timeframes) {
			tf = timeframes[i]
		}
		w, ok := categoryWeights[cat]
		if !ok {
			w = 1.0
		}
		allocation := 0.0
		if totalWeight > 0 {
			allocation = (w / totalWeight) * totalCapital
		}

		stats[name] = &StrategyStats{
			Name:       name,
			Category:   cat,
			Timeframe:  tf,
			Allocation: allocation,
			Status:     "RUNNING",
		}
	}

	perStrategyCapital := totalCapital
	if len(strategyNames) > 0 {
		perStrategyCapital = totalCapital / float64(len(strategyNames))
	}

	log.Printf("[STRATEGY TRACKER] Initialized %d strategies with $%.2f total capital (weighted allocation)", len(strategyNames), totalCapital)

	return &StrategyTracker{
		stats:                stats,
		maxConsecutiveLosses: 5,                  // Raised: need 5 losses in a row to disable (was 3)
		cooldownDuration:     10 * time.Minute,   // Shorter cooldown: recover faster (was 20 min)
		dailyLossLimit:       perStrategyCapital * 0.05, // Raised: 5% daily loss limit per strategy (was 2%)
	}
}

// IsEnabled checks if a strategy is currently allowed to trade.
func (t *StrategyTracker) IsEnabled(strategyName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[strategyName]
	if !ok {
		return true
	}

	if s.Disabled && time.Now().After(s.DisabledUntil) {
		return true
	}

	return !s.Disabled
}

// ReEnableExpired re-enables strategies whose cooldown has expired.
func (t *StrategyTracker) ReEnableExpired() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for _, s := range t.stats {
		if s.Disabled && now.After(s.DisabledUntil) {
			s.Disabled = false
			s.ConsecutiveLosses = 0
			s.Status = "RUNNING"
			log.Printf("[STRATEGY TRACKER] Re-enabled strategy: %s after cooldown", s.Name)
		}
	}
}

// RecordSignal increments the signal counter for a strategy.
func (t *StrategyTracker) RecordSignal(strategyName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if s, ok := t.stats[strategyName]; ok {
		s.SignalCount++
		s.LastSignalTime = time.Now()
	}
}

func (t *StrategyTracker) disableStrategy(s *StrategyStats, status, reason string) {
	s.Disabled = true
	s.DisabledUntil = time.Now().Add(t.cooldownDuration)
	s.Status = status
	log.Printf("[STRATEGY TRACKER] Disabled strategy %s: %s. Cooldown until %s", s.Name, reason, s.DisabledUntil.Format("15:04:05"))
}

// RecordTradeResult updates a strategy's stats after a trade closes.
func (t *StrategyTracker) RecordTradeResult(strategyName string, pnl float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.stats[strategyName]
	if !ok {
		return
	}

	s.TotalTrades++
	s.DailyPnL += pnl
	s.TotalPnL += pnl

	if pnl >= 0 {
		s.Wins++
		s.ConsecutiveLosses = 0
	} else {
		s.Losses++
		s.ConsecutiveLosses++
	}

	if s.ConsecutiveLosses >= t.maxConsecutiveLosses {
		t.disableStrategy(s, "COOLDOWN", "hit consecutive loss limit")
		return
	}

	if s.DailyPnL < -t.dailyLossLimit {
		t.disableStrategy(s, "DAILY_LIMIT", "exceeded daily loss limit")
		return
	}

	if s.TotalTrades >= poorPerformanceMinTrades {
		winRate := float64(s.Wins) / float64(s.TotalTrades)
		if s.TotalPnL < 0 && winRate < poorPerformanceMinWinRate {
			t.disableStrategy(s, "UNDERPERFORMING", "poor live win rate and negative PnL")
			return
		}
	}
}

// GetAllStats returns a snapshot of all strategy stats.
func (t *StrategyTracker) GetAllStats() []StrategyStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]StrategyStats, 0, len(t.stats))
	for _, s := range t.stats {
		result = append(result, *s)
	}
	return result
}

// GetStats returns stats for a single strategy.
func (t *StrategyTracker) GetStats(name string) (StrategyStats, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[name]
	if !ok {
		return StrategyStats{}, false
	}
	return *s, true
}

// GetWinRate returns the win rate for a strategy (0-1).
func (t *StrategyTracker) GetWinRate(name string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[name]
	if !ok || s.TotalTrades == 0 {
		return 0.5
	}
	return float64(s.Wins) / float64(s.TotalTrades)
}

// GetSizingMultiplier returns a dynamic position-size multiplier for a strategy.
// It scales up consistent winners and scales down weak or unstable performers.
func (t *StrategyTracker) GetSizingMultiplier(name string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[name]
	if !ok {
		return defaultSizingMultiplier
	}
	if s.Disabled {
		return minSizingMultiplier
	}
	if s.TotalTrades == 0 {
		return coldStartSizingMultiplier
	}

	winRate := float64(s.Wins) / float64(s.TotalTrades)
	avgPnL := s.TotalPnL / float64(s.TotalTrades)

	multiplier := defaultSizingMultiplier

	// Win-rate contribution centered at 50%.
	multiplier += (winRate - 0.5)

	switch {
	case avgPnL >= strongAvgPnLThresholdUSD:
		multiplier += 0.15
	case avgPnL >= mildAvgPnLThresholdUSD:
		multiplier += 0.05
	case avgPnL <= strongAvgPnLPenaltyThreshold:
		multiplier -= 0.15
	case avgPnL <= mildAvgPnLPenaltyThreshold:
		multiplier -= 0.05
	}

	multiplier -= float64(s.ConsecutiveLosses) * lossStreakPenaltyPerLoss

	// Avoid over-boosting while sample size is still small.
	if s.TotalTrades < poorPerformanceMinTrades && multiplier > maxEarlyBoostMultiplier {
		multiplier = maxEarlyBoostMultiplier
	}

	return math.Max(minSizingMultiplier, math.Min(maxSizingMultiplier, multiplier))
}

// GetExecutionWeight returns a quality weight used by the execution layer.
// Unlike sizing multipliers, this can aggressively de-prioritize weak strategies.
func (t *StrategyTracker) GetExecutionWeight(name string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[name]
	if !ok {
		return defaultExecutionWeight
	}
	if s.Disabled {
		return minExecutionWeight
	}
	if s.TotalTrades == 0 {
		return coldStartExecutionWeight
	}

	winRate := float64(s.Wins) / float64(s.TotalTrades)
	avgPnL := s.TotalPnL / float64(s.TotalTrades)
	weight := defaultExecutionWeight

	// Harder quality checks once we have meaningful sample size.
	if s.TotalTrades >= matureExecutionMinTrades {
		if winRate < matureExecutionMinWinRate && s.TotalPnL < 0 {
			weight -= 0.25
		}
		if winRate >= strongExecutionWinRate && avgPnL > 0 {
			weight += 0.20
		}
	}

	if avgPnL < 0 {
		weight -= 0.10
	}
	weight -= float64(s.ConsecutiveLosses) * executionLossPenaltyPerLoss

	if s.Allocation > 0 && s.DailyPnL < -(s.Allocation*0.004) {
		weight -= 0.20
	}

	return math.Max(minExecutionWeight, math.Min(maxExecutionWeight, weight))
}

// ResetDaily resets daily counters (call at midnight UTC).
func (t *StrategyTracker) ResetDaily() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, s := range t.stats {
		s.DailyPnL = 0
		if s.Disabled {
			s.Disabled = false
			s.Status = "RUNNING"
			s.ConsecutiveLosses = 0
		}
	}
	log.Println("[STRATEGY TRACKER] Daily stats reset completed")
}

// Reset clears all strategy performance state while preserving static metadata.
func (t *StrategyTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, s := range t.stats {
		s.TotalTrades = 0
		s.Wins = 0
		s.Losses = 0
		s.ConsecutiveLosses = 0
		s.DailyPnL = 0
		s.TotalPnL = 0
		s.Disabled = false
		s.DisabledUntil = time.Time{}
		s.SignalCount = 0
		s.LastSignalTime = time.Time{}
		s.Status = "RUNNING"
	}

	log.Println("[STRATEGY TRACKER] Full state reset")
}
