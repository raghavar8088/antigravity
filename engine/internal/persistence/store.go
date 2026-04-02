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
	
	// Create trades table — for UNLIMITED trade history
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS trades (
			id TEXT PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL,
			strategy_name TEXT NOT NULL,
			category TEXT NOT NULL,
			side TEXT NOT NULL,
			entry_price DOUBLE PRECISION NOT NULL,
			exit_price DOUBLE PRECISION NOT NULL,
			size DOUBLE PRECISION NOT NULL,
			gross_pnl DOUBLE PRECISION NOT NULL,
			fees DOUBLE PRECISION NOT NULL,
			net_pnl DOUBLE PRECISION NOT NULL,
			reason TEXT NOT NULL,
			entry_time TIMESTAMPTZ NOT NULL,
			exit_time TIMESTAMPTZ NOT NULL,
			duration_ms BIGINT NOT NULL,
			ai_decision_id TEXT,
			ai_provider TEXT,
			ai_reasoning TEXT,
			ai_confidence DOUBLE PRECISION,
			ai_bull_thesis TEXT,
			ai_bear_thesis TEXT
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create trades table: %w", err)
	}
	log.Println("[DB] ✅ Trades table ready (Unlimited Mode)")

	// Create ai_audit_logs table — for AI Performance Tracking & History
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS ai_audit_logs (
			id TEXT PRIMARY KEY,
			timestamp TIMESTAMPTZ NOT NULL,
			strategy_name TEXT NOT NULL,
			action TEXT NOT NULL,
			approved BOOLEAN NOT NULL,
			reason TEXT NOT NULL,
			confidence DOUBLE PRECISION NOT NULL,
			provider TEXT NOT NULL
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create ai_audit_logs table: %w", err)
	}
	log.Println("[DB] ✅ AI Audit log table ready")

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

// SaveAuditLog persists a single AI vetting decision to the database.
func (s *Store) SaveAuditLog(ctx context.Context, id, strategy, action string, approved bool, reason string, confidence float64, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO ai_audit_logs (id, timestamp, strategy_name, action, approved, reason, confidence, provider)
		VALUES ($1, NOW(), $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, id, strategy, action, approved, reason, confidence, provider)
	return err
}

// SaveTrade persists a completed trade to the relational trades table.
func (s *Store) SaveTrade(ctx context.Context, trade map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert duration to MS
	dur, _ := trade["duration"].(time.Duration)
	
	_, err := s.pool.Exec(ctx, `
		INSERT INTO trades (
			id, timestamp, strategy_name, category, side,
			entry_price, exit_price, size, gross_pnl, fees, net_pnl,
			reason, entry_time, exit_time, duration_ms,
			ai_decision_id, ai_provider, ai_reasoning, ai_confidence, ai_bull_thesis, ai_bear_thesis
		) VALUES ($1, NOW(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (id) DO NOTHING
	`,
		trade["id"], trade["strategyName"], trade["category"], trade["side"],
		trade["entryPrice"], trade["exitPrice"], trade["size"], trade["grossPnl"],
		trade["fees"], trade["netPnl"], trade["reason"],
		trade["entryTime"], trade["exitTime"], dur.Milliseconds(),
		trade["aiDecisionId"], trade["aiProvider"], trade["aiReasoning"],
		trade["aiConfidence"], trade["aiBullThesis"], trade["aiBearThesis"],
	)
	return err
}

// GetTrades retrieves the latest N trades from the database.
func (s *Store) GetTrades(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.pool.Query(ctx, `
		SELECT id, entry_time, exit_time, strategy_name, category, side,
		       entry_price, exit_price, size, gross_pnl, fees, net_pnl,
		       reason, duration_ms, ai_decision_id, ai_provider, ai_reasoning,
		       ai_confidence, ai_bull_thesis, ai_bear_thesis
		FROM trades
		ORDER BY exit_time DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer rows.Close()

	var trades []map[string]interface{}
	for rows.Next() {
		var id, strategy, category, side, reason, aiID, aiProvider, aiReason, aiBull, aiBear string
		var entryP, exitP, size, grossP, fees, netP, aiConf float64
		var durMS int64
		var entryT, exitT time.Time
		
		err := rows.Scan(
			&id, &entryT, &exitT, &strategy, &category, &side,
			&entryP, &exitP, &size, &grossP, &fees, &netP,
			&reason, &durMS, &aiID, &aiProvider, &aiReason,
			&aiConf, &aiBull, &aiBear,
		)
		if err != nil {
			return nil, err
		}
		
		trades = append(trades, map[string]interface{}{
			"id":           id,
			"strategyName": strategy,
			"category":     category,
			"side":         side,
			"entryPrice":   entryP,
			"exitPrice":    exitP,
			"size":         size,
			"grossPnl":     grossP,
			"fees":         fees,
			"netPnl":       netP,
			"reason":       reason,
			"entryTime":    entryT,
			"exitTime":     exitT,
			"duration":     time.Duration(durMS) * time.Millisecond,
			"time":         exitT.Format("15:04:05"), // Friendly string for legacy UI
			"aiDecisionId": aiID,
			"aiProvider":   aiProvider,
			"aiReasoning":  aiReason,
			"aiConfidence": aiConf,
			"aiBullThesis": aiBull,
			"aiBearThesis": aiBear,
		})
	}
	return trades, nil
}

// LoadAuditLogs retrieves the latest N AI decisions from the database.
func (s *Store) LoadAuditLogs(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.pool.Query(ctx, `
		SELECT id, timestamp, strategy_name, action, approved, reason, confidence, provider
		FROM ai_audit_logs
		ORDER BY timestamp DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id, strategy, action, reason, provider string
		var approved bool
		var confidence float64
		var timestamp time.Time
		if err := rows.Scan(&id, &timestamp, &strategy, &action, &approved, &reason, &confidence, &provider); err != nil {
			return nil, err
		}
		logs = append(logs, map[string]interface{}{
			"id":           id,
			"timestamp":    timestamp,
			"strategyName": strategy,
			"action":       action,
			"approved":     approved,
			"reason":       reason,
			"confidence":   confidence,
			"provider":     provider,
		})
	}
	return logs, nil
}

// Close shuts down the database connection pool.
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
