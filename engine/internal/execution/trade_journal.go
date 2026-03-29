package execution

import (
	"log"
	"sync"
	"time"
)

// JournalEntry records a complete trade lifecycle for analytics and dashboard display.
type JournalEntry struct {
	ID           string        `json:"id"`
	StrategyName string        `json:"strategyName"`
	Category     string        `json:"category"`
	Side         string        `json:"side"`
	EntryPrice   float64       `json:"entryPrice"`
	ExitPrice    float64       `json:"exitPrice"`
	Size         float64       `json:"size"`
	GrossPnL     float64       `json:"grossPnl"`
	Fees         float64       `json:"fees"`
	NetPnL       float64       `json:"netPnl"`
	Reason       string        `json:"reason"` // STOP_LOSS, TAKE_PROFIT, TRAILING_STOP, TIME_EXIT, BREAK_EVEN
	EntryTime    time.Time     `json:"entryTime"`
	ExitTime     time.Time     `json:"exitTime"`
	Duration     time.Duration `json:"duration"`
}

// TradeJournal maintains an in-memory log of all completed trades.
type TradeJournal struct {
	mu      sync.RWMutex
	entries []JournalEntry
	maxSize int

	// Aggregate counters
	totalTrades int
	totalWins   int
	totalLosses int
	totalPnL    float64
	bestTrade   float64
	worstTrade  float64
}

func NewTradeJournal(maxEntries int) *TradeJournal {
	return &TradeJournal{
		entries: make([]JournalEntry, 0),
		maxSize: maxEntries,
	}
}

// RecordTrade adds a completed trade to the journal.
func (j *TradeJournal) RecordTrade(entry JournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()

	// Calculate fees (simulated 0.1% taker fee each side = 0.2% round trip)
	feeRate := 0.001 // 0.1% per side
	entryFee := entry.EntryPrice * entry.Size * feeRate
	exitFee := entry.ExitPrice * entry.Size * feeRate
	entry.Fees = entryFee + exitFee
	entry.NetPnL = entry.GrossPnL - entry.Fees
	entry.Duration = entry.ExitTime.Sub(entry.EntryTime)

	j.entries = append(j.entries, entry)

	// Cap size
	if len(j.entries) > j.maxSize {
		j.entries = j.entries[1:]
	}

	// Update aggregates
	j.totalTrades++
	j.totalPnL += entry.NetPnL
	if entry.NetPnL >= 0 {
		j.totalWins++
	} else {
		j.totalLosses++
	}
	if entry.NetPnL > j.bestTrade {
		j.bestTrade = entry.NetPnL
	}
	if entry.NetPnL < j.worstTrade {
		j.worstTrade = entry.NetPnL
	}

	log.Printf("[TRADE JOURNAL] Recorded: %s | %s %s | PnL: $%.2f (net: $%.2f) | Reason: %s | Duration: %s",
		entry.StrategyName, entry.Side, entry.ID,
		entry.GrossPnL, entry.NetPnL, entry.Reason, entry.Duration)
}

// GetRecentTrades returns the most recent N trades.
func (j *TradeJournal) GetRecentTrades(n int) []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()

	if n > len(j.entries) {
		n = len(j.entries)
	}
	// Return newest first
	result := make([]JournalEntry, n)
	for i := 0; i < n; i++ {
		result[i] = j.entries[len(j.entries)-1-i]
	}
	return result
}

// GetAllTrades returns all journal entries.
func (j *TradeJournal) GetAllTrades() []JournalEntry {
	j.mu.RLock()
	defer j.mu.RUnlock()

	result := make([]JournalEntry, len(j.entries))
	copy(result, j.entries)
	return result
}

// AggregateStats holds overall performance numbers.
type AggregateStats struct {
	TotalTrades int     `json:"totalTrades"`
	TotalWins   int     `json:"totalWins"`
	TotalLosses int     `json:"totalLosses"`
	WinRate     float64 `json:"winRate"`
	TotalPnL    float64 `json:"totalPnl"`
	BestTrade   float64 `json:"bestTrade"`
	WorstTrade  float64 `json:"worstTrade"`
	ProfitFactor float64 `json:"profitFactor"`
}

// GetAggregateStats returns summary statistics.
func (j *TradeJournal) GetAggregateStats() AggregateStats {
	j.mu.RLock()
	defer j.mu.RUnlock()

	winRate := 0.0
	if j.totalTrades > 0 {
		winRate = float64(j.totalWins) / float64(j.totalTrades) * 100
	}

	// Calculate profit factor
	grossProfit := 0.0
	grossLoss := 0.0
	for _, e := range j.entries {
		if e.NetPnL >= 0 {
			grossProfit += e.NetPnL
		} else {
			grossLoss += -e.NetPnL
		}
	}
	profitFactor := 0.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	}

	return AggregateStats{
		TotalTrades:  j.totalTrades,
		TotalWins:    j.totalWins,
		TotalLosses:  j.totalLosses,
		WinRate:      winRate,
		TotalPnL:     j.totalPnL,
		BestTrade:    j.bestTrade,
		WorstTrade:   j.worstTrade,
		ProfitFactor: profitFactor,
	}
}

// RestoreTrades loads previously-saved trades back into the journal.
// This is called on engine boot to restore trade history from the database,
// so the dashboard shows correct lifetime stats even after restarts.
func (j *TradeJournal) RestoreTrades(trades []JournalEntry, totalTrades, totalWins, totalLosses int, totalPnL float64) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.entries = trades
	j.totalTrades = totalTrades
	j.totalWins = totalWins
	j.totalLosses = totalLosses
	j.totalPnL = totalPnL

	// Recalculate best/worst from entries
	for _, e := range trades {
		if e.NetPnL > j.bestTrade {
			j.bestTrade = e.NetPnL
		}
		if e.NetPnL < j.worstTrade {
			j.worstTrade = e.NetPnL
		}
	}

	log.Printf("[TRADE JOURNAL] ♻️  Restored %d trades (W/L: %d/%d, PnL: $%.2f)",
		len(trades), totalWins, totalLosses, totalPnL)
}
