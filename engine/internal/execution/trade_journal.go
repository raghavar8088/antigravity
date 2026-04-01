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
	Reason       string        `json:"reason"` // STOP_LOSS, TAKE_PROFIT, TRAILING_STOP, BREAK_EVEN, MANUAL
	EntryTime    time.Time     `json:"entryTime"`
	ExitTime     time.Time     `json:"exitTime"`
	Duration     time.Duration `json:"duration"`

	// AI fields — populated when the trade was initiated by the AI agent system
	AIDecisionID  string  `json:"aiDecisionId,omitempty"`
	AIProvider    string  `json:"aiProvider,omitempty"` // Tracking which AI approved this trade (MVP Tracking)
	AIReasoning   string  `json:"aiReasoning,omitempty"`
	AIConfidence  float64 `json:"aiConfidence,omitempty"`
	AIBullThesis  string  `json:"aiBullThesis,omitempty"`
	AIBearThesis  string  `json:"aiBearThesis,omitempty"`
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

func calculateFees(entryPrice, exitPrice, size float64) float64 {
	return 0
}

// CalculateNetPnL returns realized PnL in zero-fee mode.
func CalculateNetPnL(grossPnL, entryPrice, exitPrice, size float64) float64 {
	return grossPnL
}

// RecordTrade adds a completed trade to the journal.
func (j *TradeJournal) RecordTrade(entry JournalEntry) {
	j.mu.Lock()
	defer j.mu.Unlock()

	entry.Fees = calculateFees(entry.EntryPrice, entry.ExitPrice, entry.Size)
	entry.NetPnL = entry.GrossPnL
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
	TotalTrades   int     `json:"totalTrades"`
	TotalWins     int     `json:"totalWins"`
	TotalLosses   int     `json:"totalLosses"`
	WinRate       float64 `json:"winRate"`
	TotalPnL      float64 `json:"totalPnl"`
	BestTrade     float64 `json:"bestTrade"`
	WorstTrade    float64 `json:"worstTrade"`
	ProfitFactor  float64 `json:"profitFactor"`
	AvgWin        float64 `json:"avgWin"`
	AvgLoss       float64 `json:"avgLoss"`
	MaxDrawdown   float64 `json:"maxDrawdown"`
	AvgDurationMs int64   `json:"avgDurationMs"`
}

// GetAggregateStats returns summary statistics.
func (j *TradeJournal) GetAggregateStats() AggregateStats {
	j.mu.RLock()
	defer j.mu.RUnlock()

	winRate := 0.0
	if j.totalTrades > 0 {
		winRate = float64(j.totalWins) / float64(j.totalTrades) * 100
	}

	// Calculate profit factor, avg win/loss, max drawdown, avg duration
	grossProfit := 0.0
	grossLoss := 0.0
	winCount := 0
	lossCount := 0
	var totalDurationMs int64

	peak := 0.0
	cumPnL := 0.0
	maxDrawdown := 0.0

	for _, e := range j.entries {
		if e.NetPnL >= 0 {
			grossProfit += e.NetPnL
			winCount++
		} else {
			grossLoss += -e.NetPnL
			lossCount++
		}
		totalDurationMs += e.Duration.Milliseconds()

		cumPnL += e.NetPnL
		if cumPnL > peak {
			peak = cumPnL
		}
		if drawdown := peak - cumPnL; drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	profitFactor := 0.0
	if grossLoss > 0 {
		profitFactor = grossProfit / grossLoss
	}

	avgWin := 0.0
	if winCount > 0 {
		avgWin = grossProfit / float64(winCount)
	}

	avgLoss := 0.0
	if lossCount > 0 {
		avgLoss = grossLoss / float64(lossCount)
	}

	avgDurationMs := int64(0)
	if len(j.entries) > 0 {
		avgDurationMs = totalDurationMs / int64(len(j.entries))
	}

	return AggregateStats{
		TotalTrades:   j.totalTrades,
		TotalWins:     j.totalWins,
		TotalLosses:   j.totalLosses,
		WinRate:       winRate,
		TotalPnL:      j.totalPnL,
		BestTrade:     j.bestTrade,
		WorstTrade:    j.worstTrade,
		ProfitFactor:  profitFactor,
		AvgWin:        avgWin,
		AvgLoss:       avgLoss,
		MaxDrawdown:   maxDrawdown,
		AvgDurationMs: avgDurationMs,
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

// Reset wipes all trade history and aggregate counters from memory.
// Used by the account reset flow so the dashboard reflects a clean slate.
func (j *TradeJournal) Reset() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = make([]JournalEntry, 0)
	j.totalTrades = 0
	j.totalWins = 0
	j.totalLosses = 0
	j.totalPnL = 0
	j.bestTrade = 0
	j.worstTrade = 0
	log.Println("[TRADE JOURNAL] 🔄 All trades cleared for account reset")
}
