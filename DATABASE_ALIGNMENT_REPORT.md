# Database Alignment Report — Phase 13
# Generated: 2026-06-11 | Auditor: Claude Code Forensic Audit

---

## 1. Database Inventory

### DB-1: PostgreSQL (Neon) — `DATABASE_URL`
**Used by:** Go engine only (`engine/internal/persistence/store.go`)
**What it stores:**
- `engine_state` table: balance, positionBTC, totalFees, positions (JSON blob), trades (JSON blob), totalTrades/Wins/Losses, totalPnL, savedAt — full BTC futures paper state snapshot
- `options_state` table: BTC options buy engine state (balance, priceHist, minuteBars, trades, strategies JSON blobs)
- `nifty_options_state` table: NIFTY options buy engine state
- `options_selling_state` table: BTC options selling engine state
- `nifty_options_selling_state` table: NIFTY options selling engine state
- `trades` table: individual BTC futures paper trades (relational, with ON CONFLICT upsert) — `engine/cmd/antigravity/main.go:574`
- `kill_switch_ledger` table: kill switch activation events (`engine/internal/ledger/postgres_store.go`)
- `pms_ledger` table: portfolio management system events
**Written by:** Go engine persistence layer + kill switch + PMS
**Read by:** Go engine on startup (state restore), Next.js NEVER reads PostgreSQL directly
**Evidence:** `engine/internal/persistence/store.go:1-15` (imports `modernc.org/sqlite` but also uses PostgreSQL via `pgx/v5`). Wait — actually `store.go:14` imports `modernc.org/sqlite` suggesting SQLite, not Postgres. The postgres connection is in `engine/internal/ledger/postgres_store.go`.

**CORRECTION based on evidence:** `engine/internal/persistence/store.go` opens with `sql.Open` — checking driver import `_ "modernc.org/sqlite"` at line 14. This is SQLite, not PostgreSQL. The `Store.NewStore()` connects to SQLite. PostgreSQL (`DATABASE_URL`) is used only by the `ledger.PostgresStore` for kill switch events and PMS. The `persistence.NewStore(ctx)` that loads `DATABASE_URL` is separate — confirmed at `main.go:544`.

**Actual breakdown:**
- `persistence.NewStore(ctx)` → `engine/internal/persistence/store.go` → Uses `DATABASE_URL` (pgx) for the main engine state + trades table
- `ledger.NewPostgresStore(ctx, dbURL)` → `engine/internal/ledger/postgres_store.go` → Uses `DATABASE_URL` for kill switch ledger

### DB-2: SQLite — `SQLITE_PATH` / `./data/engine.db`
**Used by:** Go engine as fallback when `DATABASE_URL` unavailable
**What it stores:** Same schema as PostgreSQL for engine_state (when Postgres unavailable) — FileSnapshotStore at `engine/internal/persistence/file_snapshot.go`
**Written by:** Go engine
**Read by:** Go engine only
**Evidence:** `engine/cmd/antigravity/main.go:987-995` — if `dbStore == nil` (PostgreSQL unavailable), tries `persistence.NewFileSnapshotStore()` for BTC options persistence
**Note:** SQLite is a fallback, not primary. File snapshot store writes to `ENGINE_DATA_DIR`.

### DB-3: MongoDB Atlas — `MONGODB_URI` (db: `loop_trades`)
**Used by:** Go engine (paperpersist package) AND Next.js client
**Collections:**
- `paper_trades` — individual paper trade records (BTC futures paper desk)
- `paper_state` — per-account state snapshots (balance, equity, positions, drawdown)
- `paper_positions` / `open_positions` — open position records
- `paper_oms_orders` — paper OMS order records (TTL 30 days)
- `equity_curve` — 1-minute equity snapshots (from EquityRecorder)
- `daily_pnl_history` — daily PnL seal records
- `strategy_health` — per-strategy health metrics (15-min updates)
- `strategy_scores` — strategy performance scores
- `strategy_score_history` — historical strategy scores
- `strategy_signals` — per-strategy signal snapshots
- `mock_trades` — mock trading engine trades
- `mock_account` — mock trading account state
- `desk_worker_events` — worker heartbeat/event log (TTL 30 days)
- `portfolio_metrics` — 30-min portfolio metrics snapshots
- `regime_snapshots` — market regime snapshots
- `verification_events` — verification track events (TTL 30 days)
- `ai_tracker_reports` — AI app tracker reports (TTL 30 days)
- `shadow_trade_intents` — shadow trade intent records
- `signal_trace` — strategy signal trace (latest snapshot)
- `paper_entry_funnel` — entry funnel snapshot

**Written by:** Go engine (paperpersist: TradeWriter, OrderWriter, StateSnapshotter, EquityRecorder, StrategyHealthMonitor, PortfolioMetricsWriter) AND Next.js API routes (paper-trades POST, paper-state POST, cron tick)
**Read by:** Next.js API routes (all paper-desk/*, paper-trades/*, mock-trading/*)
**Evidence:** `engine/internal/paperpersist/mongo.go`, `engine/internal/paperpersist/writer.go`, `client/src/lib/mongoTradesClient.ts`

### DB-4: Redis — `REDIS_URL`
**Used by:** Not found in Next.js client source. One reference in `client/src/lib/mockPortfolioOptimizer.ts` but that appears to be a mock/test file.
**Go engine:** Not found in engine Go files during grep (no `REDIS_URL` in non-vendor Go files)
**Status:** CONFIGURED IN ENV but evidence of active use is UNVERIFIED from code audit. Redis is mentioned in CLAUDE.md as "Indicator cache, performance cache" but no active client code was found.

---

## 2. Entity Write/Read Matrix

| Entity | Write Sources | Read Sources | Source of Truth | Conflict Risk |
|---|---|---|---|---|
| BTC futures paper trades | Go engine → MongoDB (TradeWriter) + PostgreSQL (SaveTrade) | Next.js → MongoDB | **MongoDB** (primary) | YES — dual write to Mongo + Postgres |
| BTC paper account state | Go engine → MongoDB (StateSnapshotter 10s) + PostgreSQL (StateSaver) | Next.js → MongoDB | **MongoDB** (engine is authority) | YES — dual write |
| BTC paper open positions | Go engine → MongoDB (open_positions) | Next.js → MongoDB | **Go engine in-memory + MongoDB** | MODERATE |
| BTC options state | Go engine → PostgreSQL/FileSnapshot | Next.js → Go engine proxy | **PostgreSQL** | LOW — single write path |
| NIFTY options state | Go engine → PostgreSQL | Next.js → Go engine proxy | **PostgreSQL** | LOW |
| Kill switch state | Go engine → PostgreSQL (ledger) | Go engine in-memory | **PostgreSQL** | LOW |
| Strategy performance | Go engine → MongoDB (StrategyHealthMonitor) | Next.js → MongoDB | **MongoDB** | LOW |
| Equity curve | Go engine → MongoDB (EquityRecorder 1m) | Next.js → MongoDB | **MongoDB** | LOW |
| Mock trading trades | Next.js → MongoDB (mock_trades) | Next.js → MongoDB | **MongoDB** | LOW |
| Auth sessions | Next.js → MongoDB (sessions) or JWT only | Next.js JWT verify | **JWT (stateless)** | LOW |
| Strategy signals | Next.js → MongoDB (strategy_signals) | Next.js → MongoDB | **MongoDB** | LOW |
| AngelOne JWT token | In-memory module-scope cache (`angelAuth.ts`) | AngelOne API | **In-memory** | HIGH — lost on process restart |

---

## 3. Dual-State Conflicts

### CONFLICT-1: BTC futures paper trades in both PostgreSQL and MongoDB (CRITICAL)
The Go engine writes every closed BTC futures trade to BOTH:
- MongoDB via `TradeWriter` (paperpersist) — `engine/internal/paperpersist/writer.go`
- PostgreSQL via `journal.OnTrade` hook — `engine/cmd/antigravity/main.go:574`

The Next.js frontend reads ONLY from MongoDB. PostgreSQL contains a potentially more complete or differently-formatted copy. If the MongoDB write fails (Atlas connectivity) but PostgreSQL write succeeds, the frontend shows 0 trades while PostgreSQL has the full history.

**Risk:** HIGH. MongoDB Atlas M0 (free tier) has connection limits and occasional maintenance windows. During those windows, PostgreSQL is the only copy but the UI reads from MongoDB.

### CONFLICT-2: Account state written by both Go engine and legacy Next.js worker
If `ENGINE_EXECUTION_AUTHORITY` is not set or set to "1", the Go engine writes `paper_state` every 10 seconds. If it's "0" or accidentally cleared, the legacy Next.js paper-desk-tick cron also writes `paper_state`. Both use `upsertAccountState` which is a full document replace, not a merge. Whichever writes last wins.

**Risk:** HIGH when `ENGINE_EXECUTION_AUTHORITY` is misconfigured or unclear.

### CONFLICT-3: Balance in Go engine memory vs MongoDB vs PostgreSQL
The Go engine maintains `paperExecute.GetBalanceUSD()` in RAM. On restart it restores from MongoDB first (Phase31B recovery at `main.go:659-668`), then may override with PostgreSQL if MongoDB is newer. Both paths exist:
- `main.go:583`: `paperExecute.RestoreBalance(state.Balance, ...)` from PostgreSQL `LoadState`
- `main.go:667`: `paperExecute.RestoreBalance(restoredBalance, ...)` from MongoDB (overrides PostgreSQL)

This means balance restore order is: PostgreSQL first, MongoDB overrides. If MongoDB has stale balance (not updated in 10 min) but PostgreSQL is current, the balance will be set incorrectly.

**Risk:** MODERATE — documented as intentional (MongoDB is "authoritative" per Phase31B) but the logic is fragile.

---

## 4. UI Read Source vs Engine Write Source

| UI Display | Reads From | Engine Writes To | Match? |
|---|---|---|---|
| Paper Desk balance/equity | MongoDB `paper_state` | MongoDB (10s snapshots) | YES |
| Paper Desk open positions | MongoDB `open_positions` | MongoDB (paperpersist hooks) | YES |
| Paper Desk trades table | MongoDB `paper_trades` | MongoDB (TradeWriter) | YES |
| Paper Desk equity curve | MongoDB `equity_curve` | MongoDB (EquityRecorder 1m) | YES |
| Paper Desk diagnostics | Go engine direct | Go engine internal | YES (via proxy) |
| Options positions/trades | Go engine direct | PostgreSQL + in-memory | YES (via proxy) |
| NIFTY options | Go engine direct | PostgreSQL + in-memory | YES (via proxy) |
| Strategy rankings | File: `fixtures/replay/btc_ft_strategy_rankings.json` | MongoDB → file (nightly cron) | YES with 24h lag |
| Kill switch status | Go engine direct | PostgreSQL ledger | YES (via proxy) |

---

## 5. Missing Indexes / Performance Risks

1. **`paper_trades.strategy_id` index**: Only `by_account_strat_closed` index exists (`client/src/lib/mongoTradesClient.ts:89`). Queries filtering only by `strategy_id` without `account_key` require a collection scan.

2. **`paper_state` collection**: No index definition found in codebase. Queries by `account_key` may be slow on large collections.

3. **PostgreSQL `trades` table**: Uses `ON CONFLICT` upsert by trade ID but no index discovery in code — relying on pgx/v5 to manage.

4. **MongoDB `strategy_health` collection**: No explicit index found beyond those created by `EnsureIndexes` in paperpersist — exact index set needs `engine/internal/paperpersist/mongo.go` verification.

5. **`equity_curve` collection**: 1-minute inserts over months = ~500K+ documents. No TTL found for this collection. **RISK: unbounded growth.**

---

## 6. Verdict: Is Database State Consistent?

**PARTIALLY.** 

The system has a clear primary authority model (MongoDB for the paper desk, PostgreSQL for kill switch/PMS) with the Go engine as sole writer when `ENGINE_EXECUTION_AUTHORITY=1`. However:

1. **Dual writes to MongoDB + PostgreSQL for trades** create potential for divergence if one write fails.
2. **Balance restore order** (PostgreSQL then MongoDB override) is fragile and could set incorrect initial balance.
3. **Redis** — declared in CLAUDE.md but no active usage found in code. Its role as "indicator cache" appears unused or removed.
4. **Equity curve has no TTL** — unbounded MongoDB collection growth risk.
5. **Strategy rankings file** — written by nightly cron from MongoDB. 24-hour lag between MongoDB data and file reflects a real stale window for strategy ranking decisions.
