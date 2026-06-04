# Database Clone Report

Generated from repository discovery on 2026-06-02. Live database contents were not queried. Row counts are therefore listed as "requires export/query" with exact commands to execute during clone rehearsal.

## Database Inventory

| Store | Observed Location / Reference | Purpose | Source Of Truth | Clone Priority |
| --- | --- | --- | --- | --- |
| PostgreSQL / TimescaleDB | `DATABASE_URL`, `infrastructure/database/*.sql`, `client/supabase/migrations/*.sql`, `engine/internal/ledger/postgres_store.go` | Ledger, market data, OMS projections, risk, research, audit, Supabase-era tables | Yes for event store / trading warehouse where enabled | Critical |
| MongoDB Atlas | `MONGODB_URI`, `MONGODB_DB`, `MONGODB_DB_NAME`, `client/src/lib/mongoTradesClient.ts` | Paper desk state, paper trades, mock trading, policy snapshots, worker events, auth | Yes for BTC paper desk and mock trading | Critical |
| Redis | `REDIS_URL`, `infrastructure/REDIS_PHASE14_SCHEMA.md`, `infrastructure/kubernetes/redis-ha.yaml` | Hot cache, idempotency/rate/health coordination | No, except AOF/RDB when preserving exact runtime cache | Medium |
| SQLite | `SQLITE_PATH`, `engine/internal/persistence/store.go` | Local engine state, trades, AI audit logs | Yes for local engine mode | Critical if local engine used |
| Filesystem JSON/NDJSON/CSV | `LOCAL_DATA_DIR`, `data/`, `.engine-data` | Local storage fallback, backups, audit logs | Yes for fallback and forensic logs | Critical |
| Supabase/Postgres legacy | `client/supabase/migrations/` and Supabase env vars | Legacy/mirror paper tables, auth integration | Mirror/legacy depending deployment | High |

## PostgreSQL / TimescaleDB Schema

Observed SQL assets:

- `infrastructure/database/phase14_timescale_schema.sql`
- `infrastructure/database/event_store.sql`
- `infrastructure/init.sql`
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

### Extensions

- `timescaledb`
- `pgcrypto`
- `citext`

### Schemas

- `market`
- `core`
- `audit`
- `research`
- `ops`
- `trading`
- `public`

### Core Tables And Projections

| Schema | Tables / Views |
| --- | --- |
| `trading` | `event_store`, `order_projection`, `fill_projection`, `position_projection`, materialized view `pnl_1m` |
| `market` | `ticks`, `market_ticks`, `market_candles_1m`, `market_candles_3m`, `market_candles_5m`, `market_candles_15m`, `market_candles_30m`, `market_candles_1h`, `market_candles_4h`, `market_candles_1d`, `funding_rates`, `liquidation_events`, `orderbook_snapshots`, continuous aggregates `candles_1m`, `cagg_ticks_1m`, `cagg_ticks_3m`, `cagg_ticks_5m`, `cagg_ticks_15m`, `cagg_ticks_30m`, `cagg_ticks_1h`, `cagg_ticks_4h`, `cagg_ticks_1d` |
| `core` | `users`, `accounts`, `strategies`, `strategy_versions`, `strategy_health`, `research_tournaments`, `orders`, `executions`, `positions`, `closed_positions`, `position_events`, `risk_events`, `portfolio_snapshots`, `promotion_history`, `system_config` |
| `audit` | `event_store`, `ai_decisions` |
| `research` | `backtests`, `walk_forward_results`, `monte_carlo_simulations`, `parameter_sweeps`, `strategy_comparisons`, `optimization_results` |
| `ops` | `data_quality_checks`, `data_repair_jobs`, `performance_targets` |
| `public` | `paper_trades`, `delta_audit_log`, `shadow_trade_intents`, `strategy_promotions`, `risk_audit_events` |
| standalone event schema | `ledger_aggregate_sequences`, `ledger_events`, `ledger_snapshots`, views including `v_open_positions` |
| local init schema | `users`, `exchange_accounts`, `strategies`, `orders`, `positions`, `market_ticks` |

### Route-Created PostgreSQL Tables

Some API routes create operational state tables at runtime rather than relying only on SQL migrations. These must be included in schema discovery and dump validation:

- Confirmed in route code:
  - `btc_spot_state` in `client/src/app/api/btc/spot-state/route.ts`
  - `mcx_state` in `client/src/app/api/mcx/state/route.ts`
- Additional route-backed state surfaces identified for migration verification:
  - `crypto_equity_state`
  - `btc_options_paper_snapshot`
  - `nifty_client_state`
  - `nifty_selling_state`
  - `nifty_stocks_state`
  - `trade_history_archive`

Before a clone is certified, query `information_schema.tables` in the live database and include every matching table in the row-count manifest.

### Hypertables And Retention

Observed hypertables:

- `trading.event_store` by `created_at`
- `market.ticks` by `time`
- `trading.fill_projection` by `filled_at`
- `market.market_ticks` by `time`, partitioned by `symbol`, retention 180 days
- `market.market_candles_*` by `time`, partitioned by `symbol`, retention 5 years
- `market.funding_rates` by `time`, retention 5 years
- `market.liquidation_events` by `time`
- `market.orderbook_snapshots` by `time`
- `core.portfolio_snapshots` by `snapshot_at`, retention 10 years
- `audit.ai_decisions` by `decided_at`, retention 90 days

Compression policies are present for market, portfolio, event, and AI decision tables. Clone must preserve Timescale metadata, not only rows.

### PostgreSQL Export Method

Preferred full logical dump:

```bash
pg_dump "$DATABASE_URL" \
  --format=custom \
  --blobs \
  --no-owner \
  --no-acl \
  --file=postgres_full.dump
```

For Timescale-heavy databases where exact continuous aggregate and hypertable metadata matter, use a database-level backup provider snapshot when available, then validate with `pg_dump --schema-only` diff.

### PostgreSQL Restore Method

```bash
createdb "$CLONE_DATABASE"
pg_restore --dbname="$CLONE_DATABASE_URL" --clean --if-exists --no-owner postgres_full.dump
```

Run extension checks after restore:

```sql
select extname, extversion from pg_extension where extname in ('timescaledb','pgcrypto','citext');
select * from timescaledb_information.hypertables;
select * from timescaledb_information.continuous_aggregates;
```

### PostgreSQL Count Queries

```sql
select 'ledger_events' as table_name, count(*) from ledger_events
union all select 'ledger_snapshots', count(*) from ledger_snapshots
union all select 'trading.event_store', count(*) from trading.event_store
union all select 'core.orders', count(*) from core.orders
union all select 'core.executions', count(*) from core.executions
union all select 'core.positions', count(*) from core.positions
union all select 'core.closed_positions', count(*) from core.closed_positions
union all select 'core.risk_events', count(*) from core.risk_events
union all select 'core.portfolio_snapshots', count(*) from core.portfolio_snapshots
union all select 'research.backtests', count(*) from research.backtests
union all select 'research.walk_forward_results', count(*) from research.walk_forward_results
union all select 'public.paper_trades', count(*) from public.paper_trades;
```

## MongoDB Discovery

### Database Names

- Default: `loop_trades`
- Env references: `MONGODB_DB`, `MONGODB_DB_NAME`

### Collections Observed

| Collection | Location | Purpose | Indexes Observed |
| --- | --- | --- | --- |
| `paper_trades` | `mongoTradesClient.ts`, cron routes, scripts | BTC paper trade history | `client_trade_id` unique, `account_key/closed_at`, `account_key/strategy_id/closed_at`, `account_key/module_key/closed_at`, analytics indexes |
| `paper_state` | `mongoTradesClient.ts` | Account balance, positions, disabled strategies, worker heartbeat, signal trace | `account_key` unique |
| `paper_research` | `mongoTradesClient.ts` | Winners/retired strategy state by namespace | `account_key/namespace` unique |
| `desk_worker_events` | `mongoTradesClient.ts` | Worker event log | TTL on `created_at`, 30 days |
| `desk_worker_lease` | `deskWorkerLease.ts` | Worker lease heartbeat | `_id` account key |
| `paper_oms_orders` | `paperOmsMongo.ts` | Paper OMS order projections | See module tests/source during live clone |
| `policy_snapshots` | cron routes | Daily policy/ranking audit | Insert-only snapshots |
| `ai_app_tracker_reports` | tracker constants/scripts | AI app tracker snapshots | Application-managed |
| `verification_track_events` | verification track module | Worker verification events | Application-managed |
| `auth_users` | `mongoAuthClient.ts` | App auth users | Application-managed |
| `mock_trades` | `mockTradingMongo.ts` | Mock trading trades | Application-managed |
| `mock_account_snapshots` | `mockTradingMongo.ts` | Mock account snapshots | Application-managed |
| `mock_strategy_analytics` | `mockTradingMongo.ts` | Mock analytics | Application-managed |
| `mock_trade_logs` | `mockTradingMongo.ts` | Mock trade logs | Application-managed |
| `mock_engine_config` | `mockTradingMongo.ts` | Mock engine config | Application-managed |
| `strategy_signals` | `mockTradingMongo.ts` | Mock strategy signals | Application-managed |
| `regime_snapshots` | `mockTradingMongo.ts` | Market regime snapshots | Application-managed |
| `strategy_scores` | `mockTradingMongo.ts` | Strategy score current state | Application-managed |
| `strategy_score_history` | `mockTradingMongo.ts` | Strategy score history | Application-managed |
| `equity_curve` | `mockTradingMongo.ts` | Equity curve | Application-managed |
| `daily_pnl_history` | `mockTradingMongo.ts` | Daily PnL | Application-managed |
| `random_trades` | `randomTrader.ts` | Random trader baseline | Application-managed |
| `dashboard_metrics`, `strategy_leaderboards`, `research_summaries`, `analytics_views`, `ai_summaries`, `precomputed_reports`, `logs`, `account_snapshots` | `infrastructure/database/mongo/analytics-indexes.js` | Analytics/reporting cache collections | TTL and compound indexes |

### MongoDB Export Method

```bash
mongodump --uri "$MONGODB_URI" --db "$MONGODB_DB" --out mongo_dump
```

For Atlas, also export index metadata:

```bash
mongosh "$MONGODB_URI/$MONGODB_DB" --eval 'db.getCollectionNames().forEach(c => printjson({collection:c,indexes:db.getCollection(c).getIndexes()}))'
```

### MongoDB Restore Method

```bash
mongorestore --uri "$CLONE_MONGODB_URI" --db "$CLONE_MONGODB_DB" --drop mongo_dump/$MONGODB_DB
```

### MongoDB Count Queries

```javascript
db.getCollectionNames().sort().forEach((name) => {
  print(`${name}: ${db.getCollection(name).countDocuments({})}`);
});
```

## SQLite Discovery

Default path: `./data/engine.db`, override `SQLITE_PATH`.

Tables created by `engine/internal/persistence/store.go`:

- `engine_state`
- `trades`
- `ai_audit_logs`

`engine_state` includes embedded JSON columns for positions, trades, BTC options buy/sell state, and NIFTY options buy/sell state.

### SQLite Export / Restore

Cold-copy method:

1. Stop engine.
2. Copy `engine.db`, `engine.db-wal`, and `engine.db-shm` as one set.
3. Verify with `PRAGMA integrity_check;`.

Logical method:

```bash
sqlite3 "$SQLITE_PATH" ".backup 'engine_clone.db'"
sqlite3 engine_clone.db "PRAGMA integrity_check;"
```

### SQLite Count Queries

```sql
select 'engine_state', count(*) from engine_state
union all select 'trades', count(*) from trades
union all select 'ai_audit_logs', count(*) from ai_audit_logs;
```

## Redis Discovery

Redis keys documented in `infrastructure/REDIS_PHASE14_SCHEMA.md`:

- `live:position:{account}:{symbol}`
- `live:pnl:{account}`
- `live:risk:{account}`
- `live:market:{exchange}:{symbol}`
- `live:strategy_rankings:{account}`
- `live:order:{account}:{client_order_id}`
- `dedupe:idempotency:{idempotency_key}`
- `rate:exchange:{exchange}:{endpoint}:{window}`
- `health:service:{service_name}`

Redis is documented as cache/coordination, not source of truth. K8s Redis enables RDB and AOF.

### Redis Export / Restore

For exact runtime clone:

```bash
redis-cli --rdb dump.rdb
redis-cli INFO keyspace
redis-cli --scan > redis_keys.txt
```

Restore by loading `dump.rdb` into an isolated Redis instance or using `redis-cli --pipe` with exported commands. For most clone rehearsals, rebuild Redis by replaying ledger/projections and warming market/risk caches.

## Certification Status

Not certified yet. Required live artifacts:

- PostgreSQL/Timescale dump or physical snapshot
- MongoDB dump including indexes
- SQLite DB and WAL files if present
- Redis RDB/AOF only if exact hot cache is required
- Row-count and checksum manifests from original and clone

Schema risk to resolve before relying on automated restore: the repository contains multiple ledger table definitions and callers. `engine/internal/ledger/postgres_store.go` uses `global_sequence` and `created_at`, while some backup/HA paths and SQL documents may expect different sequence/timestamp names. Validate the live schema against every restore/replication path before cutover.
