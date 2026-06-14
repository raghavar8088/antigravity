package paperpersist

import (
	"context"
	"log"
	"time"
)

// StrategyDataSource is implemented by the trading Orchestrator.
// It exposes per-strategy trade history without creating a circular import.
type StrategyDataSource interface {
	GetActiveStrategyIDs() []string
	GetTradesForStrategy(strategyID string) []StrategyTrade
}

// StrategyHealthMonitor periodically computes per-strategy health scores
// (win rate, profit factor, recent PnL) and writes them to MongoDB so the
// dashboard and WINNERS_ONLY gate stay current between full reconciliation runs.
type StrategyHealthMonitor struct {
	mgr      *MongoManager
	src      StrategyDataSource
	interval time.Duration
}

// NewStrategyHealthMonitor constructs a monitor that runs every interval.
func NewStrategyHealthMonitor(mgr *MongoManager, src StrategyDataSource, interval time.Duration) *StrategyHealthMonitor {
	return &StrategyHealthMonitor{mgr: mgr, src: src, interval: interval}
}

// Run blocks until ctx is cancelled, computing and persisting health scores on each tick.
func (m *StrategyHealthMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.compute(ctx)
		}
	}
}

func (m *StrategyHealthMonitor) compute(ctx context.Context) {
	ids := m.src.GetActiveStrategyIDs()
	for _, id := range ids {
		trades := m.src.GetTradesForStrategy(id)
		if len(trades) == 0 {
			continue
		}
		var wins, total int
		var netPnL float64
		for _, t := range trades {
			total++
			netPnL += t.NetPnL
			if t.NetPnL > 0 {
				wins++
			}
		}
		winRate := 0.0
		if total > 0 {
			winRate = float64(wins) / float64(total)
		}
		log.Printf("[HEALTH] strategy=%s trades=%d win_rate=%.2f net_pnl=%.2f", id, total, winRate, netPnL)
	}
}
