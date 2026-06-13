package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// EngineState is the complete snapshot persisted to the database.
type EngineState struct {
	Balance     float64         `json:"balance"`
	PositionBTC float64         `json:"positionBtc"`
	TotalFees   float64         `json:"totalFees"`
	Positions   json.RawMessage `json:"positions"`
	Trades      json.RawMessage `json:"trades"`
	TotalTrades int             `json:"totalTrades"`
	TotalWins   int             `json:"totalWins"`
	TotalLosses int             `json:"totalLosses"`
	TotalPnL    float64         `json:"totalPnl"`
	SavedAt     time.Time       `json:"savedAt"`
}

// OptionsState is the persisted snapshot for the BTC options engine.
type OptionsState struct {
	Balance    float64         `json:"balance"`
	LastPrice  float64         `json:"lastPrice"`
	LastMinute int64           `json:"lastMinute"`
	TradeSeq   int             `json:"tradeSeq"`
	PriceHist  json.RawMessage `json:"priceHist"`
	MinuteBars json.RawMessage `json:"minuteBars"`
	Trades     json.RawMessage `json:"trades"`
	Strategies json.RawMessage `json:"strategies"`
	SavedAt    time.Time       `json:"savedAt"`
}

// OptionsSellingState is the persisted snapshot for the BTC options selling (writing) engine.
type OptionsSellingState struct {
	Balance         float64         `json:"balance"`
	DayStartBalance float64         `json:"dayStartBalance"`
	DayStartDate    int             `json:"dayStartDate"`
	LastPrice       float64         `json:"lastPrice"`
	LastMinute      int64           `json:"lastMinute"`
	TradeSeq        int             `json:"tradeSeq"`
	PriceHist       json.RawMessage `json:"priceHist"`
	MinuteBars      json.RawMessage `json:"minuteBars"`
	Trades          json.RawMessage `json:"trades"`
	Strategies      json.RawMessage `json:"strategies"`
	SavedAt         time.Time       `json:"savedAt"`
}

// Store handles all database persistence operations.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// ErrDisabled is returned by NewStore when SQLITE_ENABLED=false.
var ErrDisabled = fmt.Errorf("SQLite disabled via SQLITE_ENABLED=false")

// NewStore opens the SQLite database and creates all tables if needed.
// Set SQLITE_ENABLED=false to skip SQLite entirely (production default on AWS Lightsail,
// where MongoDB Atlas is the primary persistence layer).
func NewStore(ctx context.Context) (*Store, error) {
	if v := os.Getenv("SQLITE_ENABLED"); v == "false" {
		log.Printf("[DB] SQLite disabled (SQLITE_ENABLED=false) — skipping local store")
		return nil, ErrDisabled
	}

	dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "./data/engine.db"
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("sqlite ping failed: %w", err)
	}

	log.Printf("[DB] ✅ Connected to SQLite at %s", dbPath)

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS engine_state (
			id INTEGER PRIMARY KEY DEFAULT 1,
			balance REAL NOT NULL DEFAULT 1000000,
			position_btc REAL NOT NULL DEFAULT 0,
			total_fees REAL NOT NULL DEFAULT 0,
			positions_json TEXT NOT NULL DEFAULT '[]',
			trades_json TEXT NOT NULL DEFAULT '[]',
			total_trades INTEGER NOT NULL DEFAULT 0,
			total_wins INTEGER NOT NULL DEFAULT 0,
			total_losses INTEGER NOT NULL DEFAULT 0,
			total_pnl REAL NOT NULL DEFAULT 0,
			saved_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			options_balance REAL NOT NULL DEFAULT 1000000,
			options_last_price REAL NOT NULL DEFAULT 0,
			options_last_minute INTEGER NOT NULL DEFAULT 0,
			options_trade_seq INTEGER NOT NULL DEFAULT 0,
			options_price_hist_json TEXT NOT NULL DEFAULT '[]',
			options_minute_bars_json TEXT NOT NULL DEFAULT '[]',
			options_trades_json TEXT NOT NULL DEFAULT '[]',
			options_strategies_json TEXT NOT NULL DEFAULT '[]',
			options_saved_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			nifty_options_balance REAL NOT NULL DEFAULT 1000000,
			nifty_options_last_price REAL NOT NULL DEFAULT 0,
			nifty_options_last_minute INTEGER NOT NULL DEFAULT 0,
			nifty_options_trade_seq INTEGER NOT NULL DEFAULT 0,
			nifty_options_price_hist_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_minute_bars_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_trades_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_strategies_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_saved_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			options_selling_balance REAL NOT NULL DEFAULT 1000000,
			options_selling_last_price REAL NOT NULL DEFAULT 0,
			options_selling_last_minute INTEGER NOT NULL DEFAULT 0,
			options_selling_trade_seq INTEGER NOT NULL DEFAULT 0,
			options_selling_price_hist_json TEXT NOT NULL DEFAULT '[]',
			options_selling_minute_bars_json TEXT NOT NULL DEFAULT '[]',
			options_selling_trades_json TEXT NOT NULL DEFAULT '[]',
			options_selling_strategies_json TEXT NOT NULL DEFAULT '[]',
			options_selling_saved_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			nifty_options_selling_balance REAL NOT NULL DEFAULT 1000000,
			nifty_options_selling_last_price REAL NOT NULL DEFAULT 0,
			nifty_options_selling_last_minute INTEGER NOT NULL DEFAULT 0,
			nifty_options_selling_trade_seq INTEGER NOT NULL DEFAULT 0,
			nifty_options_selling_price_hist_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_selling_minute_bars_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_selling_trades_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_selling_strategies_json TEXT NOT NULL DEFAULT '[]',
			nifty_options_selling_saved_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			CHECK (id = 1)
		)
	`); err != nil {
		return nil, fmt.Errorf("failed to create engine_state table: %w", err)
	}

	if _, err := db.ExecContext(ctx, `INSERT INTO engine_state (id) VALUES (1) ON CONFLICT (id) DO NOTHING`); err != nil {
		log.Printf("[DB] Warning: could not seed state row: %v", err)
	}

	log.Println("[DB] ✅ State table ready")

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS trades (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			strategy_name TEXT NOT NULL,
			category TEXT NOT NULL,
			side TEXT NOT NULL,
			entry_price REAL NOT NULL,
			exit_price REAL NOT NULL,
			size REAL NOT NULL,
			gross_pnl REAL NOT NULL,
			fees REAL NOT NULL,
			net_pnl REAL NOT NULL,
			reason TEXT NOT NULL,
			entry_time TEXT NOT NULL,
			exit_time TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			ai_decision_id TEXT,
			ai_provider TEXT,
			ai_reasoning TEXT,
			ai_confidence REAL,
			ai_bull_thesis TEXT,
			ai_bear_thesis TEXT
		)
	`); err != nil {
		return nil, fmt.Errorf("failed to create trades table: %w", err)
	}
	log.Println("[DB] ✅ Trades table ready (Unlimited Mode)")

	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS ai_audit_logs (
			id TEXT PRIMARY KEY,
			timestamp TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			strategy_name TEXT NOT NULL,
			action TEXT NOT NULL,
			approved INTEGER NOT NULL,
			reason TEXT NOT NULL,
			confidence REAL NOT NULL,
			provider TEXT NOT NULL
		)
	`); err != nil {
		return nil, fmt.Errorf("failed to create ai_audit_logs table: %w", err)
	}
	log.Println("[DB] ✅ AI Audit log table ready")

	return &Store{db: db}, nil
}

// LoadState retrieves the last saved engine state from the database.
func (s *Store) LoadState(ctx context.Context) (*EngineState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state EngineState
	var posJSON, tradesJSON, savedAtStr string

	err := s.db.QueryRowContext(ctx, `
		SELECT balance, position_btc, total_fees,
		       positions_json, trades_json,
		       total_trades, total_wins, total_losses, total_pnl,
		       saved_at
		FROM engine_state WHERE id = 1
	`).Scan(
		&state.Balance, &state.PositionBTC, &state.TotalFees,
		&posJSON, &tradesJSON,
		&state.TotalTrades, &state.TotalWins, &state.TotalLosses, &state.TotalPnL,
		&savedAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	state.Positions = json.RawMessage(posJSON)
	state.Trades = json.RawMessage(tradesJSON)
	state.SavedAt, _ = time.Parse(time.RFC3339, savedAtStr)
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

	_, err := s.db.ExecContext(ctx, `
		UPDATE engine_state SET
			balance = ?, position_btc = ?, total_fees = ?,
			positions_json = ?, trades_json = ?,
			total_trades = ?, total_wins = ?, total_losses = ?, total_pnl = ?,
			saved_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id = 1
	`,
		state.Balance, state.PositionBTC, state.TotalFees,
		posJSON, tradesJSON,
		state.TotalTrades, state.TotalWins, state.TotalLosses, state.TotalPnL,
	)
	return err
}

// LoadOptionsState retrieves the last saved BTC options engine snapshot.
func (s *Store) LoadOptionsState(ctx context.Context) (*OptionsState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state OptionsState
	var priceHistJSON, minuteBarsJSON, tradesJSON, strategiesJSON, savedAtStr string

	err := s.db.QueryRowContext(ctx, `
		SELECT options_balance, options_last_price, options_last_minute, options_trade_seq,
		       options_price_hist_json, options_minute_bars_json,
		       options_trades_json, options_strategies_json, options_saved_at
		FROM engine_state WHERE id = 1
	`).Scan(
		&state.Balance, &state.LastPrice, &state.LastMinute, &state.TradeSeq,
		&priceHistJSON, &minuteBarsJSON,
		&tradesJSON, &strategiesJSON, &savedAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load options state: %w", err)
	}

	state.PriceHist = json.RawMessage(priceHistJSON)
	state.MinuteBars = json.RawMessage(minuteBarsJSON)
	state.Trades = json.RawMessage(tradesJSON)
	state.Strategies = json.RawMessage(strategiesJSON)
	state.SavedAt, _ = time.Parse(time.RFC3339, savedAtStr)
	return &state, nil
}

// SaveOptionsState persists the BTC options engine snapshot.
func (s *Store) SaveOptionsState(ctx context.Context, state *OptionsState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	priceHistJSON := string(state.PriceHist)
	if priceHistJSON == "" {
		priceHistJSON = "[]"
	}
	minuteBarsJSON := string(state.MinuteBars)
	if minuteBarsJSON == "" {
		minuteBarsJSON = "[]"
	}
	tradesJSON := string(state.Trades)
	if tradesJSON == "" {
		tradesJSON = "[]"
	}
	strategiesJSON := string(state.Strategies)
	if strategiesJSON == "" {
		strategiesJSON = "[]"
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE engine_state SET
			options_balance = ?,
			options_last_price = ?,
			options_last_minute = ?,
			options_trade_seq = ?,
			options_price_hist_json = ?,
			options_minute_bars_json = ?,
			options_trades_json = ?,
			options_strategies_json = ?,
			options_saved_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id = 1
	`,
		state.Balance, state.LastPrice, state.LastMinute, state.TradeSeq,
		priceHistJSON, minuteBarsJSON, tradesJSON, strategiesJSON,
	)
	return err
}

// LoadOptionsSellingState retrieves the last saved BTC options selling engine snapshot.
func (s *Store) LoadOptionsSellingState(ctx context.Context) (*OptionsSellingState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var state OptionsSellingState
	var priceHistJSON, minuteBarsJSON, tradesJSON, strategiesJSON, savedAtStr string

	err := s.db.QueryRowContext(ctx, `
		SELECT options_selling_balance, options_selling_last_price, options_selling_last_minute, options_selling_trade_seq,
		       options_selling_price_hist_json, options_selling_minute_bars_json,
		       options_selling_trades_json, options_selling_strategies_json, options_selling_saved_at
		FROM engine_state WHERE id = 1
	`).Scan(
		&state.Balance, &state.LastPrice, &state.LastMinute, &state.TradeSeq,
		&priceHistJSON, &minuteBarsJSON,
		&tradesJSON, &strategiesJSON, &savedAtStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load options selling state: %w", err)
	}

	state.PriceHist = json.RawMessage(priceHistJSON)
	state.MinuteBars = json.RawMessage(minuteBarsJSON)
	state.Trades = json.RawMessage(tradesJSON)
	state.Strategies = json.RawMessage(strategiesJSON)
	state.SavedAt, _ = time.Parse(time.RFC3339, savedAtStr)
	return &state, nil
}

// SaveOptionsSellingState persists the BTC options selling engine snapshot.
func (s *Store) SaveOptionsSellingState(ctx context.Context, state *OptionsSellingState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	priceHistJSON := string(state.PriceHist)
	if priceHistJSON == "" {
		priceHistJSON = "[]"
	}
	minuteBarsJSON := string(state.MinuteBars)
	if minuteBarsJSON == "" {
		minuteBarsJSON = "[]"
	}
	tradesJSON := string(state.Trades)
	if tradesJSON == "" {
		tradesJSON = "[]"
	}
	strategiesJSON := string(state.Strategies)
	if strategiesJSON == "" {
		strategiesJSON = "[]"
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE engine_state SET
			options_selling_balance = ?,
			options_selling_last_price = ?,
			options_selling_last_minute = ?,
			options_selling_trade_seq = ?,
			options_selling_price_hist_json = ?,
			options_selling_minute_bars_json = ?,
			options_selling_trades_json = ?,
			options_selling_strategies_json = ?,
			options_selling_saved_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id = 1
	`,
		state.Balance, state.LastPrice, state.LastMinute, state.TradeSeq,
		priceHistJSON, minuteBarsJSON, tradesJSON, strategiesJSON,
	)
	return err
}

// ResetState writes a clean default state to the database.
func (s *Store) ResetState(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		UPDATE engine_state SET
			balance = 1000000,
			position_btc = 0,
			total_fees = 0,
			positions_json = '[]',
			trades_json = '[]',
			total_trades = 0,
			total_wins = 0,
			total_losses = 0,
			total_pnl = 0,
			saved_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id = 1
	`)
	if err != nil {
		return fmt.Errorf("failed to reset state: %w", err)
	}
	log.Println("[DB] 🔄 Account state reset to factory defaults in database")
	return nil
}

// ClearTradeHistory removes persisted trade-history records while preserving balances and open positions.
func (s *Store) ClearTradeHistory(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.ExecContext(ctx, `DELETE FROM trades`); err != nil {
		return fmt.Errorf("failed to clear trades table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		UPDATE engine_state SET
			trades_json = '[]',
			total_trades = 0,
			total_wins = 0,
			total_losses = 0,
			total_pnl = 0,
			saved_at = strftime('%Y-%m-%dT%H:%M:%SZ','now')
		WHERE id = 1
	`); err != nil {
		return fmt.Errorf("failed to clear trade history from engine state: %w", err)
	}

	log.Println("[DB] 🧹 Trade history cleared from database")
	return nil
}

// SaveAuditLog persists a single AI vetting decision to the database.
func (s *Store) SaveAuditLog(ctx context.Context, id, strategy, action string, approved bool, reason string, confidence float64, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	approvedInt := 0
	if approved {
		approvedInt = 1
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_audit_logs (id, timestamp, strategy_name, action, approved, reason, confidence, provider)
		VALUES (?, strftime('%Y-%m-%dT%H:%M:%SZ','now'), ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO NOTHING
	`, id, strategy, action, approvedInt, reason, confidence, provider)
	return err
}

// SaveTrade persists a completed trade to the relational trades table.
func (s *Store) SaveTrade(ctx context.Context, trade map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dur, _ := trade["duration"].(time.Duration)

	entryTimeStr := ""
	if t, ok := trade["entryTime"].(time.Time); ok {
		entryTimeStr = t.Format(time.RFC3339)
	}
	exitTimeStr := ""
	if t, ok := trade["exitTime"].(time.Time); ok {
		exitTimeStr = t.Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO trades (
			id, timestamp, strategy_name, category, side,
			entry_price, exit_price, size, gross_pnl, fees, net_pnl,
			reason, entry_time, exit_time, duration_ms,
			ai_decision_id, ai_provider, ai_reasoning, ai_confidence, ai_bull_thesis, ai_bear_thesis
		) VALUES (?, strftime('%Y-%m-%dT%H:%M:%SZ','now'), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO NOTHING
	`,
		trade["id"], trade["strategyName"], trade["category"], trade["side"],
		trade["entryPrice"], trade["exitPrice"], trade["size"], trade["grossPnl"],
		trade["fees"], trade["netPnl"], trade["reason"],
		entryTimeStr, exitTimeStr, dur.Milliseconds(),
		trade["aiDecisionId"], trade["aiProvider"], trade["aiReasoning"],
		trade["aiConfidence"], trade["aiBullThesis"], trade["aiBearThesis"],
	)
	return err
}

// GetTrades retrieves the latest N trades from the database.
func (s *Store) GetTrades(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entry_time, exit_time, strategy_name, category, side,
		       entry_price, exit_price, size, gross_pnl, fees, net_pnl,
		       reason, duration_ms,
		       COALESCE(ai_decision_id, ''), COALESCE(ai_provider, ''), COALESCE(ai_reasoning, ''),
		       COALESCE(ai_confidence, 0), COALESCE(ai_bull_thesis, ''), COALESCE(ai_bear_thesis, '')
		FROM trades
		ORDER BY exit_time DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query trades: %w", err)
	}
	defer rows.Close()

	trades := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, strategy, category, side, reason, aiID, aiProvider, aiReason, aiBull, aiBear string
		var entryP, exitP, size, grossP, fees, netP, aiConf float64
		var durMS int64
		var entryTStr, exitTStr string

		err := rows.Scan(
			&id, &entryTStr, &exitTStr, &strategy, &category, &side,
			&entryP, &exitP, &size, &grossP, &fees, &netP,
			&reason, &durMS, &aiID, &aiProvider, &aiReason,
			&aiConf, &aiBull, &aiBear,
		)
		if err != nil {
			return nil, err
		}

		entryT, _ := time.Parse(time.RFC3339, entryTStr)
		exitT, _ := time.Parse(time.RFC3339, exitTStr)

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
			"entryTime":    entryT.Format(time.RFC3339),
			"exitTime":     exitT.Format(time.RFC3339),
			"duration":     time.Duration(durMS) * time.Millisecond,
			"time":         exitT.Format("15:04:05"),
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

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, timestamp, strategy_name, action, approved, reason, confidence, provider
		FROM ai_audit_logs
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var id, strategy, action, reason, provider, timestampStr string
		var approvedInt int
		var confidence float64
		if err := rows.Scan(&id, &timestampStr, &strategy, &action, &approvedInt, &reason, &confidence, &provider); err != nil {
			return nil, err
		}
		timestamp, _ := time.Parse(time.RFC3339, timestampStr)
		logs = append(logs, map[string]interface{}{
			"id":           id,
			"timestamp":    timestamp,
			"strategyName": strategy,
			"action":       action,
			"approved":     approvedInt != 0,
			"reason":       reason,
			"confidence":   confidence,
			"provider":     provider,
		})
	}
	return logs, nil
}

// Close shuts down the database connection.
func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}
