# Database Schema Certification

Generated for Phase 16A clone certification on 2026-06-02.

## Certification Verdict

Status: not certified for a true 100% clone.

The repository contains substantial schema definitions, but the expected schema is split across SQL migrations, Go boot-time DDL, Next.js route-time DDL, HA helpers, Mongo implicit collections, SQLite boot-time tables, and Redis cache key conventions. A deterministic clone cannot be certified from source alone until the live database schemas are dumped and compared against an explicit manifest.

## Schema Sources Discovered

PostgreSQL and TimescaleDB sources:

- `client/supabase/migrations/001_paper_trades.sql`
- `client/supabase/migrations/002_paper_trades_rls.sql`
- `client/supabase/migrations/003_migrate_legacy_account_key.sql`
- `client/supabase/migrations/004_delta_audit_log.sql`
- `client/supabase/migrations/005_shadow_trade_intents.sql`
- `client/supabase/migrations/006_strategy_promotions.sql`
- `client/supabase/migrations/007_paper_trades_module_key.sql`
- `client/supabase/migrations/008_institutional_risk_audit.sql`
- `client/supabase/migrations/009_institutional_database_foundation.sql`
- `client/supabase/migrations/010_event_replay_data_quality_and_security.sql`
- `infrastructure/init.sql`
- `infrastructure/database/event_store.sql`
- `infrastructure/database/phase14_timescale_schema.sql`
- `engine/internal/ledger/postgres_store.go`
- `engine/internal/ledger/postgres_snapshot_store.go`
- `engine/internal/ha/*.go`
- Next.js API route state endpoints under `client/src/app/api/**/route.ts`

SQLite source:

- `engine/internal/persistence/store.go`

MongoDB sources:

- `client/src/lib/mongoTradesClient.ts`
- `client/src/lib/paperOmsMongo.ts`
- `client/src/lib/mockTradingMongo.ts`
- `client/src/lib/randomTrader.ts`
- `client/src/lib/aiAppTracker/*`
- `client/src/lib/verificationTrack/*`
- `client/scripts/*`
- `infrastructure/database/mongo/analytics-indexes.js`

Redis sources:

- `engine/internal/performance/redis_cache.go`
- `engine/internal/ha/redis_failover.go`
- `infrastructure/REDIS_PHASE14_SCHEMA.md`
- `infrastructure/kubernetes/redis-ha.yaml`

## CREATE TABLE Inventory

Migration-defined PostgreSQL and TimescaleDB tables:

- `public.paper_trades`
- `public.delta_audit_log`
- `public.shadow_trade_intents`
- `public.strategy_promotions`
- `public.risk_audit_events`
- `market.market_ticks`
- `market.market_candles_1m`
- `market.market_candles_3m`
- `market.market_candles_5m`
- `market.market_candles_15m`
- `market.market_candles_30m`
- `market.market_candles_1h`
- `market.market_candles_4h`
- `market.market_candles_1d`
- `market.funding_rates`
- `market.liquidation_events`
- `market.orderbook_snapshots`
- `market.ticks`
- `core.users`
- `core.accounts`
- `core.strategies`
- `core.strategy_versions`
- `core.strategy_health`
- `core.research_tournaments`
- `core.orders`
- `core.executions`
- `core.positions`
- `core.closed_positions`
- `core.position_events`
- `core.risk_events`
- `core.portfolio_snapshots`
- `core.promotion_history`
- `core.system_config`
- `audit.event_store`
- `audit.ai_decisions`
- `research.backtests`
- `research.walk_forward_results`
- `research.monte_carlo_simulations`
- `research.parameter_sweeps`
- `research.strategy_comparisons`
- `research.optimization_results`
- `ops.data_quality_checks`
- `ops.data_repair_jobs`
- `ops.performance_targets`
- `trading.event_store`
- `trading.order_projection`
- `trading.fill_projection`
- `trading.position_projection`
- `ledger_aggregate_sequences`
- `ledger_events`
- `ledger_snapshots`
- `users`
- `exchange_accounts`
- `strategies`
- `orders`
- `positions`
- `market_ticks`

Materialized views and views:

- `market.cagg_ticks_1m`
- `market.cagg_ticks_3m`
- `market.cagg_ticks_5m`
- `market.cagg_ticks_15m`
- `market.cagg_ticks_30m`
- `market.cagg_ticks_1h`
- `market.cagg_ticks_4h`
- `market.cagg_ticks_1d`
- `market.candles_1m`
- `trading.pnl_1m`
- `ops.continuous_aggregate_health`
- `audit.event_replay_lag`
- `ops.data_quality_latest`
- `ops.database_health`
- `v_open_positions`
- `v_closed_positions`
- `v_daily_pnl`

Runtime-created PostgreSQL tables:

- `nifty_client_state` from `client/src/app/api/nifty/state/route.ts`
- `nifty_selling_state` from `client/src/app/api/nifty/selling-state/route.ts`
- `nifty_stocks_state` from `client/src/app/api/nifty/stocks-state/route.ts`
- `btc_spot_state` from `client/src/app/api/btc/spot-state/route.ts`
- `crypto_equity_state` from `client/src/app/api/crypto/equity-state/route.ts`
- `mcx_state` from `client/src/app/api/mcx/state/route.ts`
- `btc_options_paper_snapshot` from `client/src/app/api/options/paper-snapshot/route.ts`
- `trade_history_archive` from `client/src/lib/tradeHistoryService.ts`
- `ledger_aggregate_sequences` and `ledger_events` from `engine/internal/ledger/postgres_store.go`
- `ledger_snapshots` from `engine/internal/ledger/postgres_snapshot_store.go`
- `ha_heartbeats`
- `ha_integrity_progress`
- `ha_replication_checkpoint`
- `ha_recovery_checkpoint`
- replica-side `ledger_events` variant from `engine/internal/ha/ledger_replication.go`

SQLite runtime tables:

- `engine_state`
- `trades`
- `ai_audit_logs`

## Runtime-Generated Schema Risks

Route-generated state tables are created lazily on first API call. This means an unused route may not exist in a live database even though it exists in code, and a frequently used route may have a production table not present in any committed migration.

The route tables are operational state, not disposable cache. They persist balances, trades, strategies, snapshots, and UI state for BTC spot, BTC options, NIFTY, MCX, and crypto-equity flows.

Engine-created ledger and HA tables are also runtime-generated with `CREATE TABLE IF NOT EXISTS`. This can hide drift if an older table already exists with a different shape.

SQLite has no migration/version table. The schema is created at engine boot from `engine/internal/persistence/store.go`.

## MongoDB Collections

Discovered application collections:

- `paper_trades`
- `paper_state`
- `paper_research`
- `desk_worker_events`
- `desk_worker_lease`
- `paper_oms_orders`
- `auth_users`
- `policy_snapshots`
- `ai_app_tracker_reports`
- `verification_track_events`
- `mock_trades`
- `mock_account_snapshots`
- `mock_strategy_analytics`
- `mock_trade_logs`
- `mock_engine_config`
- `strategy_signals`
- `regime_snapshots`
- `strategy_scores`
- `strategy_score_history`
- `equity_curve`
- `daily_pnl_history`
- `random_trades`

Discovered analytics/cache collections:

- `dashboard_metrics`
- `strategy_leaderboards`
- `research_summaries`
- `analytics_views`
- `ai_summaries`
- `precomputed_reports`
- `logs`
- `account_snapshots`

Certification gap: Mongo collections are mostly implicit and created by first write. Index creation is best-effort in several modules. A true clone requires `mongodump`, `mongorestore`, and a collection/index/count manifest from the live source.

## Redis Keyspaces

Documented keyspaces include:

- `strategy_rankings`
- `research_results`
- `portfolio_metrics`
- `market_state`
- `ai_decisions`
- `risk_metrics`
- `dashboard_views`

Certification gap: no durable Redis schema migration or exact key manifest was found. Redis must be treated as rebuildable cache unless an RDB/AOF dump and key count/hash manifest are captured.

## Expected Schema vs Actual Schema

Expected schema:

- Supabase/Postgres migrations define the institutional database and legacy paper tables.
- `infrastructure/database/*.sql` defines Phase 14/15 event-store and Timescale assets.
- Docker and Kubernetes files define local/cluster Postgres and Redis topology.

Actual runtime schema:

- Next.js routes create additional Postgres tables at runtime.
- Go ledger code creates event tables at runtime.
- Go HA code creates heartbeat, replication, integrity, and recovery checkpoint tables at runtime.
- SQLite creates local engine tables at runtime.
- Mongo creates collections and indexes lazily through application writes.
- Redis keys are not declared as a complete manifest.

Certification finding: expected schema and actual schema are not yet a single source of truth.

## Dry-Run Database Validation

Live database validation was not executed because no live source and clone credentials were available in this session.

Required source and clone checks:

```sql
select table_schema, table_name
from information_schema.tables
where table_type in ('BASE TABLE', 'VIEW')
order by table_schema, table_name;

select schemaname, matviewname
from pg_matviews
order by schemaname, matviewname;

select * from timescaledb_information.hypertables;
select * from timescaledb_information.continuous_aggregates;
```

Critical counts:

```sql
select 'ledger_events' as name, count(*) from ledger_events
union all select 'ledger_snapshots', count(*) from ledger_snapshots
union all select 'ledger_aggregate_sequences', count(*) from ledger_aggregate_sequences
union all select 'trading.event_store', count(*) from trading.event_store
union all select 'audit.event_store', count(*) from audit.event_store
union all select 'core.orders', count(*) from core.orders
union all select 'core.executions', count(*) from core.executions
union all select 'core.positions', count(*) from core.positions
union all select 'core.closed_positions', count(*) from core.closed_positions
union all select 'public.paper_trades', count(*) from public.paper_trades;
```

Mongo validation:

```javascript
db.getCollectionNames().sort().forEach((name) => {
  printjson({
    collection: name,
    count: db.getCollection(name).countDocuments({}),
    indexes: db.getCollection(name).getIndexes()
  });
});
```

SQLite validation:

```sql
pragma integrity_check;
select 'engine_state' as name, count(*) from engine_state
union all select 'trades', count(*) from trades
union all select 'ai_audit_logs', count(*) from ai_audit_logs;
```

## Certification Blockers

1. Convert route-created Postgres tables into migrations or include them in a mandatory live schema dump manifest.
2. Pick a canonical event-store schema or provide deterministic mappings between `ledger_events`, `trading.event_store`, and `audit.event_store`.
3. Add schema version tracking for SQLite.
4. Capture live Postgres schema-only dump, row counts, hypertable metadata, continuous aggregate metadata, extension versions, and sequence values.
5. Capture live Mongo collection names, indexes, counts, and optional collection-name overrides.
6. Decide whether Redis is rebuildable or exact. If exact, capture RDB/AOF plus key manifest.
7. Add CI validation that compares migrations, route-created DDL, Go-created DDL, and actual live schema dumps.
8. Ensure clone environment variables cannot point at original `DATABASE_URL`, `MONGODB_URI`, or `REDIS_URL`.

## Certification Decision

Database state is cloneable with manual exports and validation, but it is not fully reproducible from source and migrations alone.
