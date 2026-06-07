package paperpersist

// strategy_health.go — Strategy performance scoring and health computation.
//
// Every 15 minutes the StrategyHealthMonitor pulls trade history for each
// active strategy and computes:
//   - win rate
//   - expectancy (expected PnL per trade)
//   - profit factor (gross profits / |gross losses|)
//   - max drawdown
//   - sample size
//   - health status (HEALTHY | WARNING | CRITICAL | INSUFFICIENT_DATA)
//
// Results are written to two collections:
//   - strategy_scores — full numerical metrics (upserted, singleton per strategy)
//   - strategy_health — health status + brief summary (upserted)
//
// The caller provides a StrategyDataSource that returns closed trades. This
// keeps strategy_health.go free from any import cycles.

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ── Health thresholds ─────────────────────────────────────────────────────────

const (
	minSampleSize    = 20    // fewer trades → INSUFFICIENT_DATA
	warnWinRate      = 0.45  // below → WARNING
	critWinRate      = 0.35  // below → CRITICAL
	warnExpectancy   = 0.0   // below 0 → WARNING
	critProfitFactor = 0.90  // below → CRITICAL
	warnProfitFactor = 1.10  // below → WARNING
	maxAcceptableDD  = 0.20  // >20% max drawdown → WARNING
)

// ── Data types ────────────────────────────────────────────────────────────────

// StrategyTrade is a minimal trade record fed into the health computation.
type StrategyTrade struct {
	StrategyID string
	NetPnL     float64
	ClosedAt   time.Time
}

// StrategyScore holds full numerical metrics for one strategy.
type StrategyScore struct {
	StrategyID   string  `json:"strategy_id"`
	WinRate      float64 `json:"win_rate"`
	Expectancy   float64 `json:"expectancy"`    // avg NetPnL per trade
	ProfitFactor float64 `json:"profit_factor"` // gross_profit / |gross_loss|
	MaxDrawdown  float64 `json:"max_drawdown"`  // fraction of peak
	SampleSize   int     `json:"sample_size"`
	TotalPnL     float64 `json:"total_pnl"`
	AvgWin       float64 `json:"avg_win"`
	AvgLoss      float64 `json:"avg_loss"`
	ComputedAt   time.Time `json:"computed_at"`
}

// HealthStatus is the qualitative assessment of a strategy.
type HealthStatus string

const (
	HealthHealthy          HealthStatus = "HEALTHY"
	HealthWarning          HealthStatus = "WARNING"
	HealthCritical         HealthStatus = "CRITICAL"
	HealthInsufficientData HealthStatus = "INSUFFICIENT_DATA"
)

// StrategyHealthDoc is written to strategy_health.
type StrategyHealthDoc struct {
	StrategyID    string       `json:"strategy_id"`
	HealthStatus  HealthStatus `json:"health_status"`
	HealthReasons []string     `json:"health_reasons"`
	Score         StrategyScore `json:"score"`
	ComputedAt    time.Time    `json:"computed_at"`
}

// ── StrategyDataSource ────────────────────────────────────────────────────────

// StrategyDataSource is implemented by the engine to expose trade history.
type StrategyDataSource interface {
	// GetActiveStrategyIDs returns all strategy IDs that have traded.
	GetActiveStrategyIDs() []string
	// GetTradesForStrategy returns closed trades for a strategy ID.
	// The engine may limit to a rolling window (e.g., last 500 trades).
	GetTradesForStrategy(strategyID string) []StrategyTrade
}

// ── StrategyHealthMonitor ─────────────────────────────────────────────────────

// StrategyHealthMonitor computes and persists strategy health every 15 minutes.
type StrategyHealthMonitor struct {
	mgr      *MongoManager
	source   StrategyDataSource
	interval time.Duration
	runCount int64 // atomic
}

// NewStrategyHealthMonitor creates a monitor with the given compute interval.
// Recommended interval: 15 * time.Minute.
func NewStrategyHealthMonitor(mgr *MongoManager, source StrategyDataSource, interval time.Duration) *StrategyHealthMonitor {
	return &StrategyHealthMonitor{
		mgr:      mgr,
		source:   source,
		interval: interval,
	}
}

// Run starts the health monitor loop. Blocks until ctx is done.
func (m *StrategyHealthMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	log.Printf("[paperpersist/health] started interval=%s account_key=%s",
		m.interval, ownerAccountKey)

	// Run once immediately on start.
	m.computeAndPersistAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.computeAndPersistAll(ctx)
		}
	}
}

// ComputeNow triggers an immediate health computation for all strategies.
func (m *StrategyHealthMonitor) ComputeNow(ctx context.Context) {
	m.computeAndPersistAll(ctx)
}

func (m *StrategyHealthMonitor) computeAndPersistAll(ctx context.Context) {
	ids := m.source.GetActiveStrategyIDs()
	if len(ids) == 0 {
		return
	}

	var healthy, warning, critical, insufficient int
	for _, id := range ids {
		trades := m.source.GetTradesForStrategy(id)
		score := computeScore(id, trades)
		health := assessHealth(score)

		if err := m.persistScore(ctx, score); err != nil {
			log.Printf("[paperpersist/health] score persist %s: %v", id, err)
			M.RecordFailed()
		}
		if err := m.persistHealth(ctx, health); err != nil {
			log.Printf("[paperpersist/health] health persist %s: %v", id, err)
			M.RecordFailed()
		}

		switch health.HealthStatus {
		case HealthHealthy:
			healthy++
		case HealthWarning:
			warning++
		case HealthCritical:
			critical++
		case HealthInsufficientData:
			insufficient++
		}
	}

	atomic.AddInt64(&m.runCount, 1)
	log.Printf("[paperpersist/health] run #%d: %d strategies healthy=%d warn=%d critical=%d insufficient=%d",
		atomic.LoadInt64(&m.runCount), len(ids), healthy, warning, critical, insufficient)
}

// ── Computation ───────────────────────────────────────────────────────────────

func computeScore(strategyID string, trades []StrategyTrade) StrategyScore {
	s := StrategyScore{
		StrategyID:  strategyID,
		SampleSize:  len(trades),
		ComputedAt:  time.Now(),
	}
	if len(trades) == 0 {
		return s
	}

	var wins, losses int
	var grossProfit, grossLoss, totalPnL float64
	var runningPeak, runningBalance, maxDD float64

	for _, t := range trades {
		totalPnL += t.NetPnL
		runningBalance += t.NetPnL

		if runningBalance > runningPeak {
			runningPeak = runningBalance
		}
		dd := 0.0
		if runningPeak > 0 {
			dd = (runningPeak - runningBalance) / runningPeak
		}
		if dd > maxDD {
			maxDD = dd
		}

		if t.NetPnL > 0 {
			wins++
			grossProfit += t.NetPnL
		} else {
			losses++
			grossLoss += math.Abs(t.NetPnL)
		}
	}

	s.TotalPnL = totalPnL
	s.MaxDrawdown = maxDD
	s.WinRate = float64(wins) / float64(len(trades))
	s.Expectancy = totalPnL / float64(len(trades))

	if wins > 0 {
		s.AvgWin = grossProfit / float64(wins)
	}
	if losses > 0 {
		s.AvgLoss = grossLoss / float64(losses)
	}
	if grossLoss > 0 {
		s.ProfitFactor = grossProfit / grossLoss
	} else if grossProfit > 0 {
		s.ProfitFactor = 999.0 // no losses — perfect (cap at 999)
	}

	return s
}

func assessHealth(s StrategyScore) StrategyHealthDoc {
	h := StrategyHealthDoc{
		StrategyID: s.StrategyID,
		Score:      s,
		ComputedAt: s.ComputedAt,
	}

	if s.SampleSize < minSampleSize {
		h.HealthStatus = HealthInsufficientData
		h.HealthReasons = []string{
			fmt.Sprintf("only %d trades (need %d)", s.SampleSize, minSampleSize),
		}
		return h
	}

	var reasons []string
	worst := HealthHealthy

	if s.WinRate < critWinRate {
		reasons = append(reasons, fmt.Sprintf("win_rate %.1f%% < %.0f%% threshold", s.WinRate*100, critWinRate*100))
		worst = HealthCritical
	} else if s.WinRate < warnWinRate {
		reasons = append(reasons, fmt.Sprintf("win_rate %.1f%% below %.0f%% warning", s.WinRate*100, warnWinRate*100))
		if worst == HealthHealthy {
			worst = HealthWarning
		}
	}

	if s.ProfitFactor < critProfitFactor {
		reasons = append(reasons, fmt.Sprintf("profit_factor %.2f < %.2f", s.ProfitFactor, critProfitFactor))
		worst = HealthCritical
	} else if s.ProfitFactor < warnProfitFactor {
		reasons = append(reasons, fmt.Sprintf("profit_factor %.2f < %.2f warning", s.ProfitFactor, warnProfitFactor))
		if worst == HealthHealthy {
			worst = HealthWarning
		}
	}

	if s.Expectancy < warnExpectancy {
		reasons = append(reasons, fmt.Sprintf("negative expectancy %.4f", s.Expectancy))
		if worst == HealthHealthy {
			worst = HealthWarning
		}
	}

	if s.MaxDrawdown > maxAcceptableDD {
		reasons = append(reasons, fmt.Sprintf("max_drawdown %.1f%% > %.0f%%", s.MaxDrawdown*100, maxAcceptableDD*100))
		if worst == HealthHealthy {
			worst = HealthWarning
		}
	}

	h.HealthStatus = worst
	h.HealthReasons = reasons
	return h
}

// ── Persistence ───────────────────────────────────────────────────────────────

func (m *StrategyHealthMonitor) persistScore(ctx context.Context, s StrategyScore) error {
	col := m.mgr.Col(ColStrategyScores)
	if col == nil {
		return nil
	}

	now := s.ComputedAt
	doc := baseDoc(now)
	doc["strategy_id"] = s.StrategyID
	doc["win_rate"] = s.WinRate
	doc["expectancy"] = s.Expectancy
	doc["profit_factor"] = s.ProfitFactor
	doc["max_drawdown"] = s.MaxDrawdown
	doc["sample_size"] = s.SampleSize
	doc["total_pnl"] = s.TotalPnL
	doc["avg_win"] = s.AvgWin
	doc["avg_loss"] = s.AvgLoss
	doc["computed_at"] = s.ComputedAt
	doc["checksum"] = checksum(s)

	// source is included in the filter to match the compound unique index
	// (account_key, strategy_id, source) and prevent engine/browser collisions.
	filter := bson.M{"account_key": ownerAccountKey, "strategy_id": s.StrategyID, "source": "paperpersist-phase31a"}
	return upsertOne(ctx, col, filter, doc)
}

func (m *StrategyHealthMonitor) persistHealth(ctx context.Context, h StrategyHealthDoc) error {
	col := m.mgr.Col(ColStrategyHealth)
	if col == nil {
		return nil
	}

	now := h.ComputedAt
	doc := baseDoc(now)
	doc["strategy_id"] = h.StrategyID
	doc["health_status"] = string(h.HealthStatus)
	doc["health_reasons"] = h.HealthReasons
	doc["score"] = h.Score
	doc["computed_at"] = h.ComputedAt
	doc["checksum"] = checksum(h)

	filter := bson.M{"account_key": ownerAccountKey, "strategy_id": h.StrategyID, "source": "paperpersist-phase31a"}
	return upsertOne(ctx, col, filter, doc)
}

// Stats returns how many health compute runs have completed.
func (m *StrategyHealthMonitor) Stats() int64 {
	return atomic.LoadInt64(&m.runCount)
}
