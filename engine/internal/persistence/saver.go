package persistence

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/positions"
)

// StateSaver periodically snapshots the engine state to the database.
// This runs in the background and saves every 30 seconds.
type StateSaver struct {
	store   *Store
	paper   *execution.PaperClient
	posMgr  *positions.Manager
	journal *execution.TradeJournal
}

// NewStateSaver creates a state saver that periodically persists engine state.
func NewStateSaver(
	store *Store,
	paper *execution.PaperClient,
	posMgr *positions.Manager,
	journal *execution.TradeJournal,
) *StateSaver {
	return &StateSaver{
		store:   store,
		paper:   paper,
		posMgr:  posMgr,
		journal: journal,
	}
}

// Run starts the periodic save loop. Call this in a goroutine.
func (s *StateSaver) Run(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	log.Println("[STATE SAVER] Saving engine state every 15s to Neon PostgreSQL")

	for {
		select {
		case <-ctx.Done():
			// Final save on shutdown
			s.save(context.Background())
			log.Println("[STATE SAVER] Final state saved on shutdown")
			return
		case <-ticker.C:
			s.save(ctx)
		}
	}
}

func (s *StateSaver) save(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[STATE SAVER] Panic during save (recovered): %v", r)
		}
	}()

	// Serialize open positions
	openPos := s.posMgr.GetOpenPositions()
	posJSON, err := json.Marshal(openPos)
	if err != nil {
		log.Printf("[STATE SAVER] Failed to marshal positions: %v", err)
		posJSON = []byte("[]")
	}

	// Serialize trade journal
	trades := s.journal.GetAllTrades()
	tradesJSON, err := json.Marshal(trades)
	if err != nil {
		log.Printf("[STATE SAVER] Failed to marshal trades: %v", err)
		tradesJSON = []byte("[]")
	}

	// Get aggregate stats
	stats := s.journal.GetAggregateStats()

	state := &EngineState{
		Balance:     s.paper.GetBalanceUSD(),
		PositionBTC: 0, // Tracked via positions
		TotalFees:   s.paper.GetTotalFees(),
		Positions:   posJSON,
		Trades:      tradesJSON,
		TotalTrades: stats.TotalTrades,
		TotalWins:   stats.TotalWins,
		TotalLosses: stats.TotalLosses,
		TotalPnL:    stats.TotalPnL,
	}

	err = s.store.SaveState(ctx, state)
	if err != nil {
		log.Printf("[STATE SAVER] ⚠️  Save failed: %v", err)
	} else {
		log.Printf("[STATE SAVER] ✅ Saved: Balance=$%.2f | Positions=%d | Trades=%d",
			state.Balance, len(openPos), stats.TotalTrades)
	}
}
