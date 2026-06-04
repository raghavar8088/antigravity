# Trading Data Clone Plan

Generated from repository discovery on 2026-06-02. Live row counts require database access and are intentionally not estimated.

## Trading Data Locations

| Data Type | Primary Location | Secondary / Derived Location | Clone Priority |
| --- | --- | --- | --- |
| Orders | `core.orders`, `trading.order_projection`, `ledger_events`, `paper_oms_orders` | OMS v3 projections, Mongo paper OMS | Critical |
| Positions | `core.positions`, `core.closed_positions`, `trading.position_projection`, `paper_state.positions`, SQLite `engine_state.positions_json` | Ledger projections, browser state | Critical |
| Fills / executions | `core.executions`, `trading.fill_projection`, ledger position/order events | Broker APIs if live | Critical |
| PnL | `core.closed_positions`, `core.portfolio_snapshots`, `trading.pnl_1m`, Mongo `paper_trades`, SQLite `trades` | Dashboard projections | Critical |
| Strategies | `engine/internal/strategy/`, `core.strategies`, `core.strategy_versions`, Mongo `paper_research`, fixtures | Source registry and research state | Critical |
| Signals | Mongo `strategy_signals`, `paper_state.signal_trace_latest`, local `data/signals`, browser/localStorage | UI trace/projections | High |
| Research tournaments | `core.research_tournaments`, `research.strategy_comparisons`, Mongo `paper_research`, fixtures | Research docs/PDFs | High |
| Backtests | `research.backtests`, `research.walk_forward_results`, `research.monte_carlo_simulations`, `research.parameter_sweeps`, `research.optimization_results`, fixtures | Script outputs | High |
| Audit logs | `audit.event_store`, `audit.ai_decisions`, SQLite `ai_audit_logs`, `data/audit/*.ndjson`, Loki | JSONL/NDJSON logs | Critical |
| Market data | Timescale `market.*`, fixtures `client/fixtures/replay/*.json` | Exchange APIs | High |

## Observed Source Files

- Strategy registry: `engine/internal/strategy/curated_registry.go`
- BTC futures trade types: `client/src/lib/btcFuturesTrade.types.ts`
- Paper worker: `client/scripts/btc-ft-paper-worker.ts`
- Paper desk execution: `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts`
- Mongo paper persistence: `client/src/lib/mongoTradesClient.ts`
- Mock trading persistence: `client/src/lib/mockTradingMongo.ts`
- Paper OMS Mongo: `client/src/lib/paperOmsMongo.ts`
- Replay fixtures:
  - `client/fixtures/replay/btcusd_1m_sample.json`
  - `client/fixtures/replay/btcusd_1m_live.json`
  - `client/fixtures/replay/btc_ft_strategy_rankings.json`
- Research fixtures:
  - `client/fixtures/research/btc_ft_verdicts.json`
- Research PDFs:
  - `RESEARCH DOCS/BTC Algorithmic Trading Strategy Research.pdf`
  - `RESEARCH DOCS/BTC CLAUDE RESEARCH.pdf`
  - `RESEARCH DOCS/BTC Intraday Trading Strategy Families (Prioritized).pdf`

## Mongo Trading Collections

Clone all documents and indexes for:

- `paper_trades`
- `paper_state`
- `paper_research`
- `paper_oms_orders`
- `desk_worker_events`
- `desk_worker_lease`
- `policy_snapshots`
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

## SQL Trading Tables

Clone all rows and schema metadata for:

- `core.orders`
- `core.executions`
- `core.positions`
- `core.closed_positions`
- `core.position_events`
- `core.risk_events`
- `core.portfolio_snapshots`
- `core.strategies`
- `core.strategy_versions`
- `core.strategy_health`
- `core.research_tournaments`
- `core.promotion_history`
- `research.backtests`
- `research.walk_forward_results`
- `research.monte_carlo_simulations`
- `research.parameter_sweeps`
- `research.strategy_comparisons`
- `research.optimization_results`
- `public.paper_trades`
- `public.delta_audit_log`
- `public.shadow_trade_intents`
- `public.strategy_promotions`
- `public.risk_audit_events`
- `trading.order_projection`
- `trading.fill_projection`
- `trading.position_projection`
- `trading.pnl_1m`

## Row Count Queries

### MongoDB

```javascript
[
  "paper_trades",
  "paper_state",
  "paper_research",
  "paper_oms_orders",
  "desk_worker_events",
  "desk_worker_lease",
  "policy_snapshots",
  "verification_track_events",
  "mock_trades",
  "mock_account_snapshots",
  "mock_strategy_analytics",
  "mock_trade_logs",
  "mock_engine_config",
  "strategy_signals",
  "regime_snapshots",
  "strategy_scores",
  "strategy_score_history",
  "equity_curve",
  "daily_pnl_history",
  "random_trades"
].forEach((c) => print(`${c}: ${db.getCollection(c).countDocuments({})}`));
```

### PostgreSQL

```sql
select 'core.orders', count(*) from core.orders
union all select 'core.executions', count(*) from core.executions
union all select 'core.positions', count(*) from core.positions
union all select 'core.closed_positions', count(*) from core.closed_positions
union all select 'core.position_events', count(*) from core.position_events
union all select 'core.risk_events', count(*) from core.risk_events
union all select 'core.portfolio_snapshots', count(*) from core.portfolio_snapshots
union all select 'core.strategies', count(*) from core.strategies
union all select 'core.strategy_versions', count(*) from core.strategy_versions
union all select 'core.strategy_health', count(*) from core.strategy_health
union all select 'core.research_tournaments', count(*) from core.research_tournaments
union all select 'core.promotion_history', count(*) from core.promotion_history
union all select 'research.backtests', count(*) from research.backtests
union all select 'research.walk_forward_results', count(*) from research.walk_forward_results
union all select 'research.monte_carlo_simulations', count(*) from research.monte_carlo_simulations
union all select 'research.parameter_sweeps', count(*) from research.parameter_sweeps
union all select 'research.strategy_comparisons', count(*) from research.strategy_comparisons
union all select 'research.optimization_results', count(*) from research.optimization_results
union all select 'trading.order_projection', count(*) from trading.order_projection
union all select 'trading.fill_projection', count(*) from trading.fill_projection
union all select 'trading.position_projection', count(*) from trading.position_projection
union all select 'public.paper_trades', count(*) from public.paper_trades;
```

### SQLite

```sql
select 'trades', count(*) from trades
union all select 'ai_audit_logs', count(*) from ai_audit_logs
union all select 'engine_state', count(*) from engine_state;
```

## Storage Size Queries

### PostgreSQL

```sql
select schemaname, relname, pg_size_pretty(pg_total_relation_size(format('%I.%I', schemaname, relname))) as total_size
from pg_stat_user_tables
where schemaname in ('core','trading','market','audit','research','ops','public')
order by pg_total_relation_size(format('%I.%I', schemaname, relname)) desc;
```

### MongoDB

```javascript
db.getCollectionNames().sort().forEach((c) => {
  const s = db.getCollection(c).stats();
  printjson({ collection: c, count: s.count, size: s.size, storageSize: s.storageSize, totalIndexSize: s.totalIndexSize });
});
```

## Export Sequence

1. Announce freeze window.
2. Stop or disable writers:
   - Go engine
   - PM2 paper worker
   - Vercel cron
   - GitHub keep-alive/deploy flows if they can restart services
3. Export Postgres/Timescale.
4. Export MongoDB.
5. Copy SQLite and local data files.
6. Copy replay/research fixtures and untracked research PDFs.
7. Hash every artifact.
8. Restore into clone.
9. Run count, checksum, and replay validation.

## Dependency Notes

- `paper_state` depends on the same `account_key` used by browser session/worker env.
- Worker lease state must be reset or rewritten for clone to prevent stale ownership.
- `policy_snapshots` and rankings are audit history; clone them, but disable source cron before clone cron starts.
- `paper_oms_orders` and ledger events must be compared for duplicated/contradictory order state.
- Browser `localStorage` may contain active local paper/Delta state not present in Mongo if the operator used local-only mode.

## Certification Gate

Trading data is clone-ready only when:

- Original and clone row counts match for every listed table/collection.
- PnL aggregates match by account, day, strategy, symbol, and module.
- Open positions match by account/symbol/side/quantity/entry.
- Event replay rebuilds the same orders, positions, PnL, risk, reconciliation, and dashboard projections.
- No clone worker, cron, or broker credential points at the original environment.
