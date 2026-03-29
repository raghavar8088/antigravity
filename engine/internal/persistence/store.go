package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EngineState is the complete snapshot persisted to the database.
type EngineState struct {
	Balance       float64         `json:"balance"`
	PositionBTC   float64         `json:"positionBtc"`
	TotalFees     float64         `json:"totalFees"`
	Positions     json.RawMessage `json:"positions"`
	Trades        json.RawMessage `json:"trades"`
	TotalTrades   int             `json:"totalTrades"`
	TotalWins     int             `json:"totalWins"`
	TotalLosses   int             `json:"totalLosses"`
	TotalPnL      float64         `json:"totalPnl"`
	SavedAt       time.Time       `json:"savedAt"`
}

// Store handles all database persistence operations.
type Store struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
}

// NewStore connects to the database and creates the state table if needed.
func NewStore(ctx context.Context) (*Store, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	log.Println("[DB] ✅ Connected to Neon PostgreSQL")

	// Create state table
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS engine_state (
			id INTEGER PRIMARY KEY DEFAULT 1,
			balance DOUBLE PRECISION NOT NULL DEFAULT 100000,
			position_btc DOUBLE PRECISION NOT NULL DEFAULT 0,
			total_fees DOUBLE PRECISION NOT NULL DEFAULT 0,
			positions_json TEXT NOT NULL DEFAULT '[]',
			trades_json TEXT NOT NULL DEFAULT '[]',
			total_trades INTEGER NOT NULL DEFAULT 0,
			total_wins INTEGER NOT NULL DEFAULT 0,
			total_losses INTEGER NOT NULL DEFAULT 0,
			total_pnl DOUBLE PRECISION NOT NULL DEFAULT 0,
			saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (id = 1)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create state table: %w", err)
	}

	// Ensure a row exists
	_, err = pool.Exec(ctx, `
		INSERT INTO engine_state (id) VALUES (1) ON CONFLICT (id) DO NOTHING
	`)
	if err != nil {
		log.Printf("[DB] Warning: could not seed state row: %v", err)
	}

	log.Println("[DB] ✅ State table ready")
	return &Store{pool: pool}, nil
}

// LoadState retrieves the last saved engine state from the database.
func (s *Store) LoadState(ctx context.Context) (*EngineState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state EngineState
	var posJSON, tradesJSON string

	err := s.pool.QueryRow(ctx, `
		SELECT balance, position_btc, total_fees,
		       positions_json, trades_json,
		       total_trades, total_wins, total_losses, total_pnl,
		       saved_at
		FROM engine_state WHERE id = 1
	`).Scan(
		&state.Balance, &state.PositionBTC, &state.TotalFees,
		&posJSON, &tradesJSON,
		&state.TotalTrades, &state.TotalWins, &state.TotalLosses, &state.TotalPnL,
		&state.SavedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	state.Positions = json.RawMessage(posJSON)
	state.Trades = json.RawMessage(tradesJSON)
	return &state, nil
}

// SaveState persists the current engine state to the database.
func (s *Store) SaveState(ctx context.Context, state *EngineState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	posJSON := string(state.Positions)
	if posJSON == "" {
		posJSON = "[]"
	}
	tradesJSON := string(state.Trades)
	if tradesJSON == "" {
		tradesJSON = "[]"
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE engine_state SET
			balance = $1, position_btc = $2, total_fees = $3,
			positions_json = $4, trades_json = $5,
			total_trades = $6, total_wins = $7, total_losses = $8, total_pnl = $9,
			saved_at = NOW()
		WHERE id = 1
	`,
		state.Balance, state.PositionBTC, state.TotalFees,
		posJSON, tradesJSON,
		state.TotalTrades, state.TotalWins, state.TotalLosses, state.TotalPnL,
	)
	return err
}

// ResetState writes a clean default state to the database, effectively
// zeroing out the account so the next engine restart starts fresh.
func (s *Store) ResetState(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.pool.Exec(ctx, `
		UPDATE engine_state SET
			balance = 100000,
			position_btc = 0,
			total_fees = 0,
			positions_json = '[]',
			trades_json = '[]',
			total_trades = 0,
			total_wins = 0,
			total_losses = 0,
			total_pnl = 0,
			saved_at = NOW()
		WHERE id = 1
	`)
	if err != nil {
		return fmt.Errorf("failed to reset state in database: %w", err)
	}
	log.Println("[DB] 🔄 Account state reset to factory defaults in database")
	return nil
}

// Close shuts down the database connection pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
