package main

// mongo.go — MongoDB persistence wiring for the Pre-Live Trade Engine.
//
// Connects to the same MongoDB Atlas instance as the main engine but writes
// under account_key="pre_live_engine" so all collections stay segregated
// from the main paper desk.
//
// Collections written:
//   paper_trades            — closed trade records
//   paper_positions         — open position state
//   paper_orders            — OMS lifecycle log
//   paper_state             — account snapshot (crash recovery)
//   equity_curve            — 1-minute equity snapshots
//   daily_pnl_history       — midnight daily PnL
//   strategy_scores         — per-strategy performance
//   strategy_health         — per-strategy health status
//   portfolio_metrics       — portfolio-level summary
//   pre_live_strategies     — backtest results + qualified strategy registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"antigravity-engine/internal/execution"
	"antigravity-engine/internal/mongopersist"
	"antigravity-engine/internal/paperpersist"
	"antigravity-engine/internal/trading"
)

// preLiveAccountKey segregates pre-live data from the main paper desk
// inside shared MongoDB collections (all docs are keyed by account_key).
const preLiveAccountKey = "pre_live_engine"

// applyPreLiveAccountKey forces OWNER_ACCOUNT_KEY to preLiveAccountKey so that
// all paperpersist reads/writes (journal bootstrap, crash recovery, trade writes)
// are scoped exclusively to pre-live data, even when the shared .env already
// has OWNER_ACCOUNT_KEY set to the main paper-desk key ("mock_trading_main").
// Must be called once, immediately after loadDotEnv().
func applyPreLiveAccountKey() {
	// PRE_LIVE_OWNER_ACCOUNT_KEY lets operators override the key explicitly.
	if v := os.Getenv("PRE_LIVE_OWNER_ACCOUNT_KEY"); v != "" {
		os.Setenv("OWNER_ACCOUNT_KEY", v) //nolint:errcheck
		log.Printf("[PRE-LIVE/mongo] account_key=%s (from PRE_LIVE_OWNER_ACCOUNT_KEY)", v)
		return
	}
	// Always override OWNER_ACCOUNT_KEY with the pre-live key — never inherit
	// the main engine's key from the shared .env file.
	if prev := os.Getenv("OWNER_ACCOUNT_KEY"); prev != "" && prev != preLiveAccountKey {
		log.Printf("[PRE-LIVE/mongo] overriding inherited OWNER_ACCOUNT_KEY=%q → %q", prev, preLiveAccountKey)
	}
	os.Setenv("OWNER_ACCOUNT_KEY", preLiveAccountKey) //nolint:errcheck
	log.Printf("[PRE-LIVE/mongo] account_key=%s", preLiveAccountKey)
}

// preLiveMongoBundle holds all live MongoDB handles for the pre-live engine.
type preLiveMongoBundle struct {
	mgr    *paperpersist.MongoManager
	bundle *trading.PaperPersistBundle
}

// IsConnected returns true when the MongoDB manager is live.
func (b *preLiveMongoBundle) IsConnected() bool {
	return b != nil && b.mgr != nil && b.mgr.IsConnected()
}

// WipePreLiveData deletes every document belonging to the pre-live account key
// from all persistence collections. Called during /api/admin/reset so the data
// does not come back on the next page refresh or engine restart.
// Safe to call when MongoDB is unavailable (no-op, returns nil).
func (b *preLiveMongoBundle) WipePreLiveData(ctx context.Context) error {
	if b == nil || b.mgr == nil || !b.mgr.IsConnected() {
		return nil
	}
	// Use the EFFECTIVE account key (set by applyPreLiveAccountKey, possibly
	// overridden via PRE_LIVE_OWNER_ACCOUNT_KEY), not the constant — otherwise a
	// second instance (e.g. the BTC Pre-Live Engine) resetting itself would wipe
	// the default instance's data instead of its own.
	key := os.Getenv("OWNER_ACCOUNT_KEY")
	if key == "" {
		key = preLiveAccountKey
	}
	filter := bson.M{"account_key": key}
	collections := []string{
		"paper_trades",
		"paper_positions",
		"paper_orders",
		"paper_state",
		"equity_curve",
		"daily_pnl_history",
		"strategy_scores",
		"strategy_health",
		"portfolio_metrics",
	}
	wipeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var errs []string
	for _, col := range collections {
		res, err := b.mgr.Col(col).DeleteMany(wipeCtx, filter)
		if err != nil {
			errs = append(errs, col+": "+err.Error())
			continue
		}
		log.Printf("[PRE-LIVE RESET] wiped %d docs from %s", res.DeletedCount, col)
	}
	if len(errs) > 0 {
		return fmt.Errorf("mongo wipe partial errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// initPreLiveMongo connects to MongoDB, runs crash recovery, wires persistence
// into the orchestrator, and starts all background sync goroutines.
// Returns nil when MongoDB is unavailable — the engine continues in-memory only.
func initPreLiveMongo(
	ctx context.Context,
	orch *trading.Orchestrator,
	journal *execution.TradeJournal,
	balance float64,
	qualifiedNames []string,
) *preLiveMongoBundle {
	envReport := paperpersist.CheckEnv()
	if !envReport.MongoReady {
		log.Printf("[PRE-LIVE/mongo] ⛔  MONGODB_URI not set — running in-memory only. Trades will NOT persist across restarts.")
		return nil
	}

	mongoMgr, err := paperpersist.NewMongoManager(ctx)
	if err != nil {
		log.Printf("[PRE-LIVE/mongo] ❌  connect failed (%v) — running in-memory only", err)
		return nil
	}

	// Ensure paperpersist indexes (TTL + compound).
	indexCtx, indexCancel := context.WithTimeout(ctx, 30*time.Second)
	if err := mongoMgr.EnsureIndexes(indexCtx); err != nil {
		log.Printf("[PRE-LIVE/mongo] warn: paperpersist indexes: %v", err)
	}
	indexCancel()

	// Ensure Phase 30 engine-state indexes (engine_positions, engine_risk, etc.).
	if db := mongoMgr.DB(); db != nil {
		p30Ctx, p30Cancel := context.WithTimeout(ctx, 30*time.Second)
		if err := mongopersist.EnsureIndexes(p30Ctx, db); err != nil {
			log.Printf("[PRE-LIVE/mongo] warn: phase30 indexes: %v", err)
		}
		p30Cancel()

		// Ensure the pre_live_strategies compound unique index.
		ensurePreLiveStrategiesIndex(ctx, db)
	}

	// Crash recovery: restore balance + open positions from the last paper_state
	// snapshot so a restarted engine continues from where it left off.
	recovery := paperpersist.Recover(ctx, mongoMgr)
	if recovery.AccountRestored {
		log.Printf("[PRE-LIVE/mongo] ✅  crash recovery: balance=%.2f trades=%d open_positions=%d",
			recovery.AccountState.Balance,
			recovery.AccountState.TotalTrades,
			recovery.PositionsRestored,
		)
	} else {
		log.Printf("[PRE-LIVE/mongo] fresh session — no prior state found (initial_balance=%.2f)", balance)
	}

	// Re-register recovered open positions so processCloseEvents emits correct
	// OMS transitions when those positions hit SL/TP after restart.
	if len(recovery.OpenPositions) > 0 {
		orch.RegisterRecoveredPositions(recovery.OpenPositions)
	}

	// Bootstrap in-memory trade journal from paper_trades so StrategyHealthMonitor
	// has history from prior sessions immediately, not only after new closes.
	if booted, err := paperpersist.BootstrapJournalFromMongo(ctx, mongoMgr, journal); err != nil {
		log.Printf("[PRE-LIVE/mongo] journal bootstrap warn: %v", err)
	} else if booted > 0 {
		log.Printf("[PRE-LIVE/mongo] bootstrapped %d trades from paper_trades into journal", booted)
	}

	// Bootstrap portfolio ledger (cumulative accounting / Kelly ring-buffer).
	if err := paperpersist.BootstrapPortfolioLedgerFromMongo(
		ctx, mongoMgr, orch.PortfolioLedger(), balance,
	); err != nil {
		log.Printf("[PRE-LIVE/mongo] portfolio ledger bootstrap warn: %v", err)
	}

	// Wire trade/order writers and attach the bundle to the orchestrator.
	tradeWriter := paperpersist.NewTradeWriter(mongoMgr)
	orderWriter := paperpersist.NewOrderWriter(mongoMgr)
	ppBundle := trading.NewPaperPersistBundle(mongoMgr, tradeWriter, orderWriter)
	orch.SetPaperPersist(ppBundle)

	// Ping monitor: reconnects MongoDB on transient Atlas failures.
	go mongoMgr.RunPingMonitor(ctx, 30*time.Second)

	// Write-pressure note (2026-07-10 outage): this Atlas M0 free-tier cluster
	// gets throttled hard when hammered (multi-second heartbeat RTTs, wedged
	// pools, ReplicaSetNoPrimary for hours). The pre-live harness trades a
	// handful of times per week — sub-minute telemetry granularity is pure
	// cost. Intervals below are deliberately calm.

	// State snapshotter → paper_state every 30 s (crash-recovery source of truth).
	snapshotter := paperpersist.NewStateSnapshotter(mongoMgr, orch, 30*time.Second)
	go snapshotter.Run(ctx)

	// Equity recorder → equity_curve every 5 m + midnight daily_pnl_history seal.
	equityRecorder := paperpersist.NewEquityRecorder(mongoMgr, orch, orch, 5*time.Minute)
	go equityRecorder.Run(ctx)

	// Strategy health monitor → strategy_health + strategy_scores every 30 m.
	healthMonitor := paperpersist.NewStrategyHealthMonitor(mongoMgr, orch, 30*time.Minute)
	go healthMonitor.Run(ctx)

	// Portfolio metrics → portfolio_metrics every 30 s.
	go runPortfolioMetrics(ctx, mongoMgr, orch)

	// Seed pre_live_strategies with backtest results + qualified strategy list.
	// Runs in background — never blocks the trading loop.
	go seedPreLiveStrategies(ctx, mongoMgr, qualifiedNames)

	log.Printf("[PRE-LIVE/mongo] ✅  MongoDB persistence active — account_key=%s db=%s",
		paperpersist.AccountKey(), envReport.DatabaseName)

	return &preLiveMongoBundle{mgr: mongoMgr, bundle: ppBundle}
}

// runPortfolioMetrics upserts portfolio-level metrics to portfolio_metrics every 5 m.
// Each write carries its own deadline: the 2026-07-10 outage showed that a write
// with the engine-lifetime ctx hangs forever on a throttled Atlas node, pinning a
// pool connection and starving pings/reconnects.
func runPortfolioMetrics(ctx context.Context, mgr *paperpersist.MongoManager, orch *trading.Orchestrator) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := orch.GetAccountSnapshot()
			opCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			_ = paperpersist.WritePortfolioMetrics(opCtx, mgr, paperpersist.PortfolioMetricsDoc{
				RealizedPnL:      snap.RealizedPnL,
				UnrealizedPnL:    snap.UnrealizedPnL,
				GrossPnL:         snap.RealizedPnL + snap.UnrealizedPnL,
				Equity:           snap.Equity,
				Balance:          snap.Balance,
				PeakEquity:       snap.PeakEquity,
				CurrentDrawdown:  snap.CurrentDrawdown,
				MaxDrawdown:      snap.MaxDrawdown,
				TotalTrades:      snap.TotalTrades,
				WinRate:          snap.WinRate,
				LongExposureBTC:  snap.LongExposureBTC,
				ShortExposureBTC: snap.ShortExposureBTC,
				GrossExposureBTC: snap.TotalExposureBTC,
				OpenPositions:    snap.OpenPositionCount,
			})
			cancel()
		}
	}
}

// ── Pre-live strategy registry seeder ────────────────────────────────────────

// backtestResult mirrors one entry in engine/data/backtest_results.json.
type backtestResult struct {
	Rank           int     `json:"rank"`
	Strategy       string  `json:"strategy"`
	TotalTrades    int     `json:"total_trades"`
	WinRatePct     float64 `json:"win_rate_pct"`
	Sharpe         float64 `json:"sharpe"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	ProfitFactor   float64 `json:"profit_factor"`
	TotalReturnPct float64 `json:"total_return_pct"`
	AvgWinPct      float64 `json:"avg_win_pct"`
	AvgLossPct     float64 `json:"avg_loss_pct"`
	MeetsPromotion bool    `json:"meets_promotion"`
	DemoteReason   string  `json:"demote_reason,omitempty"`
}

type backtestFile struct {
	ElapsedS   float64          `json:"elapsed_s"`
	From       string           `json:"from"`
	RanAt      string           `json:"ran_at"`
	Strategies []backtestResult `json:"strategies"`
}

// seedPreLiveStrategies upserts the backtest results and the qualified-strategy
// registry into the pre_live_strategies MongoDB collection.
// One document per strategy, keyed by (strategy_name, account_key).
// Non-fatal — seed failure never blocks the trading loop.
func seedPreLiveStrategies(ctx context.Context, mgr *paperpersist.MongoManager, qualifiedNames []string) {
	if mgr == nil || !mgr.IsConnected() {
		return
	}

	results, ranAt, err := loadBacktestResults()
	if err != nil {
		log.Printf("[PRE-LIVE/mongo] backtest seed: could not read backtest_results.json: %v", err)
		// Seed the qualified names without backtest metrics.
		results = nil
	}

	// Build a lookup map from backtest results.
	byName := make(map[string]backtestResult, len(results))
	for _, r := range results {
		byName[r.Strategy] = r
	}

	// Build a set of qualified names for fast lookup.
	qualifiedSet := make(map[string]bool, len(qualifiedNames))
	for _, n := range qualifiedNames {
		qualifiedSet[n] = true
	}

	col := mgr.Col("pre_live_strategies")
	if col == nil {
		log.Printf("[PRE-LIVE/mongo] backtest seed: pre_live_strategies collection unavailable")
		return
	}

	now := time.Now().UTC()
	accountKey := paperpersist.AccountKey()
	upserted := 0
	failed := 0

	upsertStrategy := func(name string, qualified bool, br *backtestResult) {
		doc := bson.M{
			"strategy_name":  name,
			"qualified":      qualified,
			"account_key":    accountKey,
			"seeded_at":      now,
			"schema_version": 1,
		}
		if br != nil {
			doc["rank"] = br.Rank
			doc["total_trades"] = br.TotalTrades
			doc["win_rate_pct"] = br.WinRatePct
			doc["sharpe"] = br.Sharpe
			doc["max_drawdown_pct"] = br.MaxDrawdownPct
			doc["profit_factor"] = br.ProfitFactor
			doc["total_return_pct"] = br.TotalReturnPct
			doc["avg_win_pct"] = br.AvgWinPct
			doc["avg_loss_pct"] = br.AvgLossPct
			doc["meets_promotion"] = br.MeetsPromotion
			doc["demote_reason"] = br.DemoteReason
			if ranAt != "" {
				doc["backtest_ran_at"] = ranAt
			}
		}
		filter := bson.M{"strategy_name": name, "account_key": accountKey}
		update := bson.M{
			"$set":         doc,
			"$setOnInsert": bson.M{"created_at": now},
		}
		opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if _, err := col.UpdateOne(opCtx, filter, update, options.UpdateOne().SetUpsert(true)); err != nil {
			log.Printf("[PRE-LIVE/mongo] backtest seed upsert %s: %v", name, err)
			failed++
		} else {
			upserted++
		}
	}

	// Upsert qualified strategies first (with backtest metrics where available).
	for _, name := range qualifiedNames {
		br, ok := byName[name]
		if ok {
			upsertStrategy(name, true, &br)
		} else {
			upsertStrategy(name, true, nil)
		}
	}

	// Also upsert non-qualified strategies from the backtest file so the full
	// history is queryable in MongoDB (qualified=false).
	for _, br := range results {
		if qualifiedSet[br.Strategy] {
			continue // already upserted above
		}
		brCopy := br
		upsertStrategy(br.Strategy, false, &brCopy)
	}

	log.Printf("[PRE-LIVE/mongo] pre_live_strategies seeded: %d upserted, %d failed", upserted, failed)
}

// ensurePreLiveStrategiesIndex creates a compound unique index on the
// pre_live_strategies collection so (strategy_name, account_key) is unique.
func ensurePreLiveStrategiesIndex(ctx context.Context, db *mongo.Database) {
	idxCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	col := db.Collection("pre_live_strategies")
	_, err := col.Indexes().CreateOne(idxCtx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "strategy_name", Value: 1},
			{Key: "account_key", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		// Duplicate index creation is harmless.
		log.Printf("[PRE-LIVE/mongo] pre_live_strategies index: %v", err)
	}
}

// loadBacktestResults reads engine/data/backtest_results.json.
// Returns empty slice on any error (the seeder degrades gracefully).
func loadBacktestResults() ([]backtestResult, string, error) {
	data, err := os.ReadFile(backtestResultsPath())
	if err != nil {
		return nil, "", err
	}
	var f backtestFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, "", err
	}
	return f.Strategies, f.RanAt, nil
}

// backtestResultsPath returns the absolute path to engine/data/backtest_results.json.
// Precedence: BACKTEST_RESULTS_PATH env → derived from source file location → relative fallback.
func backtestResultsPath() string {
	if v := os.Getenv("BACKTEST_RESULTS_PATH"); v != "" {
		return v
	}
	// This source file lives at engine/cmd/pre_live/mongo.go — go up 2 levels
	// from cmd/pre_live/ to reach engine/, then into data/.
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		return filepath.Join(filepath.Dir(thisFile), "..", "..", "data", "backtest_results.json")
	}
	return filepath.Join("..", "..", "data", "backtest_results.json")
}
