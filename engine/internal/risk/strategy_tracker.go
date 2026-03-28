package risk

import (
	"log"
	"sync"
	"time"
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
	Allocation        float64   `json:"allocation"` // USD budget for this strategy
	SignalCount       int64     `json:"signalCount"`
	LastSignalTime    time.Time `json:"lastSignalTime"`
	Status            string    `json:"status"` // "RUNNING", "DISABLED", "COOLDOWN"
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

	// Weighted allocation by category
	categoryWeights := map[string]float64{
		"Trend":          1.5,
		"Mean Reversion": 1.3,
		"Breakout":       1.2,
		"Momentum":       1.1,
		"Microstructure": 1.0,
		"Velocity":       0.9,
		"Statistical":    1.2,
		"Volatility":     1.0,
		"Smart Money":    1.1,
		"Price Action":   1.0,
		"Adaptive":       1.3,
	}

	// Calculate total weight
	totalWeight := 0.0
	for i, name := range strategyNames {
		cat := "Unknown"
		if i < len(categories) {
			cat = categories[i]
		}
		w, ok := categoryWeights[cat]
		if !ok {
			w = 1.0
		}
		totalWeight += w
		_ = name
	}

	// Distribute capital proportionally
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
		allocation := (w / totalWeight) * totalCapital

		stats[name] = &StrategyStats{
			Name:       name,
			Category:   cat,
			Timeframe:  tf,
			Allocation: allocation,
			Status:     "RUNNING",
		}
	}

	log.Printf("[STRATEGY TRACKER] Initialized %d strategies with $%.2f total capital (weighted allocation)", len(strategyNames), totalCapital)

	return &StrategyTracker{
		stats:                stats,
		maxConsecutiveLosses: 5,
		cooldownDuration:     10 * time.Minute,
		dailyLossLimit:       totalCapital / float64(len(strategyNames)) * 0.5, // 50% of allocation as daily loss limit
	}
}

// IsEnabled checks if a strategy is currently allowed to trade.
func (t *StrategyTracker) IsEnabled(strategyName string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[strategyName]
	if !ok {
		return true // Unknown strategy, allow
	}

	// Check cooldown expiry
	if s.Disabled && time.Now().After(s.DisabledUntil) {
		return false // Still need write lock to re-enable
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

	// Check consecutive loss threshold
	if s.ConsecutiveLosses >= t.maxConsecutiveLosses {
		s.Disabled = true
		s.DisabledUntil = time.Now().Add(t.cooldownDuration)
		s.Status = "COOLDOWN"
		log.Printf("[STRATEGY TRACKER] ⚠️ DISABLED strategy %s after %d consecutive losses. Cooldown until %s",
			s.Name, s.ConsecutiveLosses, s.DisabledUntil.Format("15:04:05"))
	}

	// Check daily loss limit
	if s.DailyPnL < -t.dailyLossLimit {
		s.Disabled = true
		s.DisabledUntil = time.Now().Add(t.cooldownDuration)
		s.Status = "DAILY_LIMIT"
		log.Printf("[STRATEGY TRACKER] 🛑 DISABLED strategy %s: daily loss $%.2f exceeds limit $%.2f",
			s.Name, s.DailyPnL, t.dailyLossLimit)
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
		return 0.5 // Default 50% for new strategies
	}
	return float64(s.Wins) / float64(s.TotalTrades)
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
