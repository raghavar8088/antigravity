package risk

import (
	"log"
	"math"
	"sync"
	"time"

	riskv2 "antigravity-engine/internal/risk/v2"
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
	GrossWinUSD       float64   `json:"grossWinUsd"`
	GrossLossUSD      float64   `json:"grossLossUsd"`  // stored as positive value
	PeakTotalPnL      float64   `json:"peakTotalPnl"`
	MaxDrawdownPct    float64   `json:"maxDrawdownPct"`
	recentReturns     []float64 // last 252 trade PnL values; unexported, not serialised
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
		"Trend Elite":        1.45,
		"Mean Reversion":     1.3,
		"Mean Rev Elite":     1.25,
		"Breakout":           1.2,
		"Breakout Elite":     1.2,
		"Momentum":           1.1,
		"Momentum Elite":     1.1,
		"Oscillator Elite":   1.05,
		"Volume Elite":       1.0,
		"Microstructure":     1.0,
		"Velocity":           0.9,
		"Statistical":        1.1,
		"Volatility":         0.95,
		"Volatility Elite":   1.0,
		"Time-of-Day":        0.95,
		"Smart Money":        1.0,
		"Price Action":       1.0,
		"Price Action Elite": 1.0,
		"Adaptive":           1.15,
		"Adaptive Elite":     1.15,
		"Multi-Signal":       1.3,
		"Intraday":           1.15,
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
		maxConsecutiveLosses: 5,                         // Raised: need 5 losses in a row to disable (was 3)
		cooldownDuration:     10 * time.Minute,          // Shorter cooldown: recover faster (was 20 min)
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
		s.GrossWinUSD += pnl
	} else {
		s.Losses++
		s.ConsecutiveLosses++
		s.GrossLossUSD += -pnl // store as positive
	}

	// Track drawdown from equity peak
	if s.TotalPnL > s.PeakTotalPnL {
		s.PeakTotalPnL = s.TotalPnL
	}
	denom := s.Allocation
	if denom <= 0 {
		denom = 1000
	}
	if dd := (s.PeakTotalPnL - s.TotalPnL) / denom * 100; dd > s.MaxDrawdownPct {
		s.MaxDrawdownPct = dd
	}

	// Rolling window of per-trade PnL for Sharpe calculation
	s.recentReturns = append(s.recentReturns, pnl)
	if len(s.recentReturns) > 252 {
		s.recentReturns = s.recentReturns[len(s.recentReturns)-252:]
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
		s.GrossWinUSD = 0
		s.GrossLossUSD = 0
		s.PeakTotalPnL = 0
		s.MaxDrawdownPct = 0
		s.recentReturns = s.recentReturns[:0]
		s.Disabled = false
		s.DisabledUntil = time.Time{}
		s.SignalCount = 0
		s.LastSignalTime = time.Time{}
		s.Status = "RUNNING"
	}

	log.Println("[STRATEGY TRACKER] Full state reset")
}

// BuildRiskMetrics converts a strategy's live StrategyStats into a riskv2.StrategyMetrics
// populated with real measured performance values. This is the bridge that connects the
// StrategyTracker (real performance data) to Risk V2 (Kelly sizing, dynamic sizing,
// allocation decisions). No hardcoded placeholders; cold-start strategies receive neutral
// values that the Kelly engine treats conservatively via its sample-size stability factor.
func (t *StrategyTracker) BuildRiskMetrics(name string) riskv2.StrategyMetrics {
	t.mu.RLock()
	defer t.mu.RUnlock()

	s, ok := t.stats[name]
	if !ok {
		return riskv2.StrategyMetrics{
			Strategy: name,
			Family:   riskv2.FamilyReserve,
		}
	}

	winRate := 0.5
	if s.TotalTrades > 0 {
		winRate = float64(s.Wins) / float64(s.TotalTrades)
	}

	profitFactor := 1.0
	switch {
	case s.GrossLossUSD > 0:
		profitFactor = s.GrossWinUSD / s.GrossLossUSD
	case s.GrossWinUSD > 0:
		profitFactor = 3.0 // all wins so far; Kelly will down-weight due to low TotalTrades
	}

	avgWin := 0.0
	if s.Wins > 0 {
		avgWin = s.GrossWinUSD / float64(s.Wins)
	}
	avgLoss := 0.0
	if s.Losses > 0 {
		avgLoss = s.GrossLossUSD / float64(s.Losses)
	}

	expectancy := winRate*avgWin - (1-winRate)*avgLoss

	return riskv2.StrategyMetrics{
		Strategy:       name,
		Family:         trackerFamilyFromCategory(s.Category),
		WinRate:        winRate,
		ProfitFactor:   profitFactor,
		Sharpe:         trackerAnnualizedSharpe(s.recentReturns),
		ExpectancyUSD:  expectancy,
		AverageWinUSD:  avgWin,
		AverageLossUSD: -avgLoss, // riskv2 expects negative for losses
		MaxDrawdownPct: s.MaxDrawdownPct,
		HealthScore:    trackerHealthScore(s),
		TotalTrades:    s.TotalTrades,
	}
}

// trackerAnnualizedSharpe computes an annualised Sharpe ratio from a slice of per-trade
// USD PnL values. Returns 0 when the sample is too small to be meaningful (< 5 trades).
func trackerAnnualizedSharpe(returns []float64) float64 {
	n := len(returns)
	if n < 5 {
		return 0
	}
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	m := sum / float64(n)
	var variance float64
	for _, r := range returns {
		d := r - m
		variance += d * d
	}
	variance /= float64(n - 1)
	if variance == 0 {
		return 0
	}
	// Annualise treating each trade as an independent period; sqrt(252) is the
	// standard daily-to-annual scaler and keeps our values on the same scale that
	// riskv2 thresholds (e.g. DynamicSize Sharpe < 1.0) were designed for.
	return m / math.Sqrt(variance) * math.Sqrt(252)
}

// trackerHealthScore derives a 0–100 health score from live performance statistics.
// A score ≥ 85 = ELITE, 70 = HEALTHY, 55 = WATCHLIST, 40 = RESTRICTED, < 40 = DISABLED.
func trackerHealthScore(s *StrategyStats) float64 {
	if s.TotalTrades == 0 {
		return 50.0 // neutral cold start — not penalised, not privileged
	}
	winRate := float64(s.Wins) / float64(s.TotalTrades)

	// Win-rate component: linear from 0 (0% WR) to 100 (100% WR)
	score := winRate * 100

	// Profit-factor bonus: PF 2.0 → +10, PF 3.0 → +20 (capped)
	if s.GrossLossUSD > 0 {
		pf := s.GrossWinUSD / s.GrossLossUSD
		score += math.Min(20, (pf-1)*10)
	} else if s.GrossWinUSD > 0 {
		score += 20 // all wins so far
	}

	// Drawdown penalty: 10% DD → -20 pts (capped at -30)
	score -= math.Min(30, s.MaxDrawdownPct*2)

	// Consecutive-loss stress penalty
	score -= math.Min(15, float64(s.ConsecutiveLosses)*3)

	// Small sample size reduces confidence
	switch {
	case s.TotalTrades < 10:
		score -= 10
	case s.TotalTrades < 30:
		score -= 5
	}

	return math.Max(0, math.Min(100, score))
}

// trackerFamilyFromCategory maps a strategy's textual category to the riskv2 family
// enum used for family-level risk budget and correlation grouping.
func trackerFamilyFromCategory(cat string) riskv2.StrategyFamily {
	switch cat {
	case "Trend", "Trend Elite", "Momentum", "Momentum Elite", "Intraday":
		return riskv2.FamilyTrend
	case "Mean Reversion", "Mean Rev Elite", "Oscillator Elite", "Statistical":
		return riskv2.FamilyMeanReversion
	case "Breakout", "Breakout Elite":
		return riskv2.FamilyBreakout
	case "Microstructure", "Velocity":
		return riskv2.FamilyOrderFlow
	case "Smart Money", "Price Action", "Price Action Elite":
		return riskv2.FamilySmartMoney
	case "Volume Elite":
		return riskv2.FamilyVolumeProfile
	case "Adaptive", "Adaptive Elite", "Multi-Signal":
		return riskv2.FamilyMSS
	default:
		return riskv2.FamilyReserve
	}
}
