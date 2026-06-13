package paperpersist

// portfolio_ledger.go — in-memory mirror of MongoDB closed-trade accounting.
// Updated atomically on every trade close. Journal reads from this cache;
// MongoDB remains the authoritative store for dashboards.

import (
	"context"
	"sync"
	"time"

	"antigravity-engine/internal/kelly"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// pnlHistoryMax is the ring-buffer capacity for Kelly sizing.
// 60 trades gives enough history for half-Kelly to stabilise.
const pnlHistoryMax = 60

// PortfolioLedger tracks cumulative accounting derived from persisted closes.
// It also implements kelly.LedgerInterface so it can drive Kelly position sizing.
type PortfolioLedger struct {
	mu sync.RWMutex

	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	RealizedPnL   float64
	GrossPnL      float64
	TotalFees     float64
	EntryFees     float64
	ExitFees      float64

	PeakEquity      float64
	CurrentDrawdown float64
	MaxDrawdown     float64

	StartingBalance float64
	LastUpdated     time.Time

	// pnlHistory is a ring buffer of the last pnlHistoryMax per-trade PnL
	// as a fraction of entry notional (positive = win, negative = loss).
	// Used by ClosedTrades() to feed Kelly sizing.
	pnlHistory []float64
}

// NewPortfolioLedger creates a ledger with the given starting balance.
func NewPortfolioLedger(startingBalance float64) *PortfolioLedger {
	return &PortfolioLedger{
		StartingBalance: startingBalance,
		PeakEquity:      startingBalance,
	}
}

// RecordClose updates ledger from a canonical closed trade.
// entryNotional is entryPrice×size in USD; used to compute PnL% for Kelly.
// Pass 0 if unknown — the PnL% entry is skipped in that case.
func (l *PortfolioLedger) RecordClose(grossPnL, entryFee, exitFee, netPnL float64, balanceAfter float64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.TotalTrades++
	if netPnL > 0 {
		l.WinningTrades++
	} else {
		l.LosingTrades++
	}
	l.RealizedPnL += netPnL
	l.GrossPnL += grossPnL
	l.EntryFees += entryFee
	l.ExitFees += exitFee
	l.TotalFees += entryFee + exitFee
	l.LastUpdated = time.Now()

	equity := balanceAfter
	if equity > l.PeakEquity {
		l.PeakEquity = equity
	}
	if l.PeakEquity > 0 {
		dd := (l.PeakEquity - equity) / l.PeakEquity
		if dd < 0 {
			dd = 0
		}
		l.CurrentDrawdown = dd
		if dd > l.MaxDrawdown {
			l.MaxDrawdown = dd
		}
	}
}

// Snapshot returns a point-in-time copy (mutex excluded).
func (l *PortfolioLedger) Snapshot() PortfolioLedger {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return PortfolioLedger{
		TotalTrades:     l.TotalTrades,
		WinningTrades:   l.WinningTrades,
		LosingTrades:    l.LosingTrades,
		RealizedPnL:     l.RealizedPnL,
		GrossPnL:        l.GrossPnL,
		TotalFees:       l.TotalFees,
		EntryFees:       l.EntryFees,
		ExitFees:        l.ExitFees,
		PeakEquity:      l.PeakEquity,
		CurrentDrawdown: l.CurrentDrawdown,
		MaxDrawdown:     l.MaxDrawdown,
		StartingBalance: l.StartingBalance,
		LastUpdated:     l.LastUpdated,
	}
}

// WinRate returns 0–1 win rate.
func (l *PortfolioLedger) WinRate() float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.TotalTrades == 0 {
		return 0
	}
	return float64(l.WinningTrades) / float64(l.TotalTrades)
}

// RecordTradePnLPct stores one trade's PnL% (netPnL / entryNotional) in the ring buffer.
// Must be called with the ledger mutex NOT held.
func (l *PortfolioLedger) RecordTradePnLPct(pnlPct float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pnlHistory = append(l.pnlHistory, pnlPct)
	if len(l.pnlHistory) > pnlHistoryMax {
		// Drop oldest entry — keep only the last pnlHistoryMax.
		l.pnlHistory = l.pnlHistory[len(l.pnlHistory)-pnlHistoryMax:]
	}
}

// ClosedTrades implements kelly.LedgerInterface.
// Returns the last min(n, len(history)) trades as Kelly-compatible records.
// accountID is accepted for interface compatibility but ignored — the ledger
// is single-account (btc-paper-1).
func (l *PortfolioLedger) ClosedTrades(accountID string, n int) ([]kelly.ClosedTrade, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	src := l.pnlHistory
	if n > 0 && n < len(src) {
		src = src[len(src)-n:]
	}
	out := make([]kelly.ClosedTrade, len(src))
	for i, pct := range src {
		out[i] = kelly.ClosedTrade{PnLPct: pct}
	}
	return out, nil
}

// BootstrapPortfolioLedgerFromMongo hydrates the ledger from paper_trades aggregation.
func BootstrapPortfolioLedgerFromMongo(ctx context.Context, mgr *MongoManager, ledger *PortfolioLedger, balance float64) error {
	if mgr == nil || !mgr.IsConnected() || ledger == nil {
		return nil
	}
	col := mgr.Col(ColPaperTrades)
	if col == nil {
		return nil
	}

	pipeline := []bson.M{
		{"$match": bson.M{"account_key": AccountKey()}},
		{"$group": bson.M{
			"_id":            nil,
			"total_trades":   bson.M{"$sum": 1},
			"winning_trades": bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$gt": bson.A{"$net_pnl", 0}}, 1, 0}}},
			"losing_trades":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$lte": bson.A{"$net_pnl", 0}}, 1, 0}}},
			"realized_pnl":   bson.M{"$sum": bson.M{"$ifNull": bson.A{"$net_pnl", 0}}},
			"gross_pnl":      bson.M{"$sum": bson.M{"$ifNull": bson.A{"$gross_pnl", 0}}},
			"total_fees":     bson.M{"$sum": bson.M{"$ifNull": bson.A{"$total_fee", bson.M{"$ifNull": bson.A{"$fees", 0}}}}},
			"entry_fees":     bson.M{"$sum": bson.M{"$ifNull": bson.A{"$entry_fee", 0}}},
			"exit_fees":      bson.M{"$sum": bson.M{"$ifNull": bson.A{"$exit_fee", 0}}},
		}},
	}

	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		return nil
	}
	var row bson.M
	if err := cursor.Decode(&row); err != nil {
		return err
	}

	ledger.mu.Lock()
	defer ledger.mu.Unlock()

	ledger.TotalTrades = int(getFloat(row, "total_trades"))
	ledger.WinningTrades = int(getFloat(row, "winning_trades"))
	ledger.LosingTrades = int(getFloat(row, "losing_trades"))
	ledger.RealizedPnL = getFloat(row, "realized_pnl")
	ledger.GrossPnL = getFloat(row, "gross_pnl")
	ledger.TotalFees = getFloat(row, "total_fees")
	ledger.EntryFees = getFloat(row, "entry_fees")
	ledger.ExitFees = getFloat(row, "exit_fees")
	ledger.LastUpdated = time.Now()

	equity := balance
	if equity > ledger.PeakEquity {
		ledger.PeakEquity = equity
	}
	return nil
}
