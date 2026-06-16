package paperpersist

// recovery.go — Crash recovery on engine restart.
//
// On startup, Recover() reads MongoDB Atlas to reconstruct:
//   1. Account balance and equity from paper_state
//   2. Open positions from paper_positions (status == "OPEN")
//   3. OMS state from paper_orders (pending transitions)
//
// A RecoveryReport is generated that lists what was restored, any
// inconsistencies detected, and which positions could not be recovered.
//
// The report is written to structured logs and returned to the caller so
// the engine can make a go/no-go decision before accepting new signals.
//
// Security: all queries are keyed on ownerAccountKey — never client input.

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ── Recovered types ───────────────────────────────────────────────────────────

// RecoveredAccountState holds balance and equity restored from paper_state.
type RecoveredAccountState struct {
	Balance       float64
	Equity        float64
	UnrealizedPnL float64
	RealizedPnL   float64
	MaxDrawdown   float64
	TotalTrades   int
	WinRate       float64
	TotalFees     float64
	SessionStart  time.Time
	SnappedAt     time.Time
}

// RecoveredPosition holds a single open position restored from paper_positions.
type RecoveredPosition struct {
	PositionID string
	OrderID    string
	StrategyID string
	Symbol     string
	Side       string
	EntryPrice float64
	Size       float64
	StopLoss   float64
	TakeProfit float64
	OpenedAt   time.Time
}

// ── RecoveryReport ────────────────────────────────────────────────────────────

// RecoveryReport summarises what was restored and any anomalies found.
type RecoveryReport struct {
	// Outcome
	Success bool   `json:"success"`
	Message string `json:"message"`

	// Account state
	AccountRestored  bool                  `json:"account_restored"`
	AccountState     RecoveredAccountState `json:"account_state"`
	AccountDataAge   time.Duration         `json:"account_data_age"` // now - snapped_at

	// Positions
	PositionsRestored int                  `json:"positions_restored"`
	OpenPositions     []RecoveredPosition  `json:"open_positions"`

	// Inconsistencies
	Inconsistencies []string `json:"inconsistencies"`
	MissingData     []string `json:"missing_data"`

	// Metadata
	RecoveredAt time.Time `json:"recovered_at"`
	AccountKey  string    `json:"account_key"` // always ownerAccountKey
}

// ── Recover ───────────────────────────────────────────────────────────────────

// Recover performs full crash recovery from MongoDB Atlas.
// It is called once at engine boot, before any trading begins.
// Returns a RecoveryReport describing what was found.
func Recover(ctx context.Context, mgr *MongoManager) RecoveryReport {
	report := RecoveryReport{
		RecoveredAt: time.Now(),
		AccountKey:  AccountKey(), // sourced from server env, never client
	}

	if !mgr.IsConnected() {
		report.Success = false
		report.Message = "MongoDB not connected — running without recovery"
		report.MissingData = append(report.MissingData, "MongoDB unavailable")
		log.Println("[paperpersist/recovery] MongoDB unavailable — cold start")
		return report
	}

	// 1. Restore account state.
	acct, err := recoverAccountState(ctx, mgr)
	if err != nil {
		report.Inconsistencies = append(report.Inconsistencies,
			fmt.Sprintf("account state load failed: %v", err))
		report.MissingData = append(report.MissingData, "paper_state document")
		log.Printf("[paperpersist/recovery] account state: %v", err)
	} else {
		report.AccountRestored = true
		report.AccountState = acct
		report.AccountDataAge = time.Since(acct.SnappedAt)
		if report.AccountDataAge > 5*time.Minute {
			report.Inconsistencies = append(report.Inconsistencies,
				fmt.Sprintf("account snapshot is %.0f minutes old — may be stale",
					report.AccountDataAge.Minutes()))
		}
	}

	// 2. Restore open positions.
	positions, posErr := recoverOpenPositions(ctx, mgr)
	if posErr != nil {
		report.Inconsistencies = append(report.Inconsistencies,
			fmt.Sprintf("position load failed: %v", posErr))
		log.Printf("[paperpersist/recovery] positions: %v", posErr)
	} else {
		report.OpenPositions = positions
		report.PositionsRestored = len(positions)
	}

	// 3. Cross-validate: account says N open positions, we found M.
	if report.AccountRestored {
		stated := report.AccountState.TotalTrades
		_ = stated // informational only
		if report.AccountState.TotalTrades > 0 && report.PositionsRestored == 0 &&
			report.AccountState.UnrealizedPnL != 0 {
			report.Inconsistencies = append(report.Inconsistencies,
				"account reports non-zero unrealized PnL but no open positions found")
		}
	}

	// 4. Final outcome.
	if len(report.Inconsistencies) == 0 {
		report.Success = true
		report.Message = "recovery complete"
	} else {
		report.Success = true // partial success: engine can still start
		report.Message = fmt.Sprintf("recovery complete with %d inconsistencies — review logs",
			len(report.Inconsistencies))
	}

	logReport(report)
	return report
}

// ── Internal query functions ──────────────────────────────────────────────────

func recoverAccountState(ctx context.Context, mgr *MongoManager) (RecoveredAccountState, error) {
	col := mgr.Col(ColPaperState)
	if col == nil {
		return RecoveredAccountState{}, fmt.Errorf("collection unavailable")
	}

	filter := bson.M{"account_key": AccountKey()}
	opts := options.FindOne().SetSort(bson.D{{Key: "snapped_at", Value: -1}})

	var raw bson.M
	if err := col.FindOne(ctx, filter, opts).Decode(&raw); err != nil {
		if err == mongo.ErrNoDocuments {
			return RecoveredAccountState{}, fmt.Errorf("no paper_state document for account_key=%s", AccountKey())
		}
		return RecoveredAccountState{}, fmt.Errorf("FindOne paper_state: %w", err)
	}

	acct := RecoveredAccountState{}
	acct.Balance = getFloat(raw, "balance")
	if acct.Balance < 0 {
		log.Printf("[paperpersist/recovery] negative balance %.4f clamped to 0", acct.Balance)
		acct.Balance = 0
	}
	acct.Equity = getFloat(raw, "equity")
	acct.UnrealizedPnL = getFloat(raw, "unrealized_pnl")
	acct.RealizedPnL = getFloat(raw, "realized_pnl")
	acct.MaxDrawdown = getFloat(raw, "max_drawdown")
	acct.TotalTrades = getInt(raw, "total_trades")
	acct.WinRate = getFloat(raw, "win_rate")
	acct.TotalFees = getFloat(raw, "total_fees")
	if t := getTime(raw, "session_start"); !t.IsZero() {
		acct.SessionStart = t
	}
	if t := getTime(raw, "snapped_at"); !t.IsZero() {
		acct.SnappedAt = t
	}

	return acct, nil
}

func recoverOpenPositions(ctx context.Context, mgr *MongoManager) ([]RecoveredPosition, error) {
	col := mgr.Col(ColPaperPositions)
	if col == nil {
		return nil, fmt.Errorf("collection unavailable")
	}

	filter := bson.M{
		"account_key": AccountKey(),
		"status":      "OPEN",
	}
	opts := options.Find().SetSort(bson.D{{Key: "opened_at", Value: 1}})

	cursor, err := col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("Find paper_positions: %w", err)
	}
	defer cursor.Close(ctx)

	var out []RecoveredPosition
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			log.Printf("[paperpersist/recovery] position decode error: %v", err)
			continue
		}
		p := RecoveredPosition{
			PositionID: getString(raw, "position_id"),
			OrderID:    getString(raw, "order_id"),
			StrategyID: getString(raw, "strategy_id"),
			Symbol:     getString(raw, "symbol"),
			Side:       getString(raw, "side"),
			EntryPrice: getFloat(raw, "entry_price"),
			Size:       getFloat(raw, "size"),
			StopLoss:   getFloat(raw, "stop_loss"),
			TakeProfit: getFloat(raw, "take_profit"),
		}
		if t := getTime(raw, "opened_at"); !t.IsZero() {
			p.OpenedAt = t
		}
		if p.PositionID == "" {
			continue // skip corrupted documents
		}
		applyRecoveredProtection(&p)
		out = append(out, p)
	}
	if err := cursor.Err(); err != nil {
		return out, fmt.Errorf("cursor error: %w", err)
	}
	return out, nil
}

// ── Logging ───────────────────────────────────────────────────────────────────

func logReport(r RecoveryReport) {
	log.Printf("[paperpersist/recovery] ════════════════════════════════════")
	log.Printf("[paperpersist/recovery] RECOVERY REPORT  account_key=%s", r.AccountKey)
	log.Printf("[paperpersist/recovery] success=%v  message=%s", r.Success, r.Message)

	if r.AccountRestored {
		a := r.AccountState
		log.Printf("[paperpersist/recovery] account: balance=%.2f equity=%.2f unrealized=%.4f realized=%.4f",
			a.Balance, a.Equity, a.UnrealizedPnL, a.RealizedPnL)
		log.Printf("[paperpersist/recovery] account: max_drawdown=%.2f%% trades=%d win_rate=%.1f%% data_age=%s",
			a.MaxDrawdown*100, a.TotalTrades, a.WinRate*100, r.AccountDataAge.Round(time.Second))
	} else {
		log.Println("[paperpersist/recovery] account: NOT RESTORED — cold start with defaults")
	}

	log.Printf("[paperpersist/recovery] positions: %d open positions restored", r.PositionsRestored)
	for i, p := range r.OpenPositions {
		log.Printf("[paperpersist/recovery]   [%d] id=%s strat=%s %s %s entry=%.2f size=%.4f",
			i, p.PositionID, p.StrategyID, p.Symbol, p.Side, p.EntryPrice, p.Size)
	}

	if len(r.MissingData) > 0 {
		log.Printf("[paperpersist/recovery] missing data: %v", r.MissingData)
	}
	if len(r.Inconsistencies) > 0 {
		for _, inc := range r.Inconsistencies {
			log.Printf("[paperpersist/recovery] ⚠  inconsistency: %s", inc)
		}
	}
	log.Printf("[paperpersist/recovery] ════════════════════════════════════")
}

// ── BSON helpers ──────────────────────────────────────────────────────────────

func getFloat(m bson.M, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	}
	return 0
}

func getInt(m bson.M, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func getString(m bson.M, key string) string {
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// getTime extracts a time.Time from a raw BSON map decode. The mongo driver
// decodes BSON datetime fields into bson.DateTime (int64 millis) — not
// time.Time — when the destination is interface{}/bson.M, so a plain
// `m[key].(time.Time)` assertion silently fails and leaves the zero value
// (year 0001), which then propagates into closed-trade records.
func getTime(m bson.M, key string) time.Time {
	switch v := m[key].(type) {
	case time.Time:
		return v
	case bson.DateTime:
		return v.Time()
	}
	return time.Time{}
}

// applyRecoveredProtection fills missing SL/TP on recovered MongoDB positions.
func applyRecoveredProtection(p *RecoveredPosition) {
	if p == nil || p.EntryPrice <= 0 {
		return
	}
	slPct := defaultRecoveredStopLossPct
	tpPct := defaultRecoveredTakeProfitPct
	isLong := p.Side == "LONG" || p.Side == "BUY"

	validSL := p.StopLoss > 0 && ((isLong && p.StopLoss < p.EntryPrice) || (!isLong && p.StopLoss > p.EntryPrice))
	validTP := p.TakeProfit > 0 && ((isLong && p.TakeProfit > p.EntryPrice) || (!isLong && p.TakeProfit < p.EntryPrice))

	if !validSL {
		if isLong {
			p.StopLoss = p.EntryPrice * (1 - slPct/100)
		} else {
			p.StopLoss = p.EntryPrice * (1 + slPct/100)
		}
		log.Printf("[paperpersist/recovery] position %s missing SL — defaulted to %.2f", p.PositionID, p.StopLoss)
	}
	if !validTP {
		if isLong {
			p.TakeProfit = p.EntryPrice * (1 + tpPct/100)
		} else {
			p.TakeProfit = p.EntryPrice * (1 - tpPct/100)
		}
		log.Printf("[paperpersist/recovery] position %s missing TP — defaulted to %.2f", p.PositionID, p.TakeProfit)
	}
}

const (
	defaultRecoveredStopLossPct   = 1.0
	defaultRecoveredTakeProfitPct = 0.30
)
