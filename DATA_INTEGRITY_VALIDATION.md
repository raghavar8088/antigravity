# Data Integrity Validation

Generated from repository discovery on 2026-06-02.

## Validation Principle

The clone is valid only when source and clone produce identical counts, hashes, and deterministic projections from the same source-of-truth data. Missing live database access means validation is not complete yet.

## Source Code Validation

```bash
git rev-parse HEAD
git fsck --full
git status --short
git log --oneline -5
```

Expected:

- HEAD equals `0baaef180177987d3dc39e7c07bd0384bef960b7` unless clone intentionally advances.
- Untracked runtime files are accounted for in a separate artifact manifest.

## File Checksum Validation

```bash
sha256sum \
  data/**/* \
  output/**/* \
  bridge/*.jsonl \
  client/fixtures/replay/*.json \
  client/fixtures/research/*.json \
  > local_state.sha256
```

Validate clone:

```bash
sha256sum -c local_state.sha256
```

Windows alternative:

```powershell
Get-ChildItem -Recurse data,output,bridge,client\fixtures |
  Where-Object { -not $_.PSIsContainer } |
  Get-FileHash -Algorithm SHA256
```

## PostgreSQL Row Count Validation

Run on original and clone and diff output:

```sql
select 'ledger_events', count(*) from ledger_events
union all select 'ledger_snapshots', count(*) from ledger_snapshots
union all select 'ledger_aggregate_sequences', count(*) from ledger_aggregate_sequences
union all select 'trading.event_store', count(*) from trading.event_store
union all select 'trading.order_projection', count(*) from trading.order_projection
union all select 'trading.fill_projection', count(*) from trading.fill_projection
union all select 'trading.position_projection', count(*) from trading.position_projection
union all select 'core.orders', count(*) from core.orders
union all select 'core.executions', count(*) from core.executions
union all select 'core.positions', count(*) from core.positions
union all select 'core.closed_positions', count(*) from core.closed_positions
union all select 'core.position_events', count(*) from core.position_events
union all select 'core.risk_events', count(*) from core.risk_events
union all select 'core.portfolio_snapshots', count(*) from core.portfolio_snapshots
union all select 'core.strategies', count(*) from core.strategies
union all select 'core.strategy_versions', count(*) from core.strategy_versions
union all select 'research.backtests', count(*) from research.backtests
union all select 'research.walk_forward_results', count(*) from research.walk_forward_results
union all select 'research.monte_carlo_simulations', count(*) from research.monte_carlo_simulations
union all select 'research.parameter_sweeps', count(*) from research.parameter_sweeps
union all select 'research.strategy_comparisons', count(*) from research.strategy_comparisons
union all select 'research.optimization_results', count(*) from research.optimization_results
union all select 'public.paper_trades', count(*) from public.paper_trades;
```

## PostgreSQL Hash Validation

Ledger manifest:

```sql
select
  count(*) as event_count,
  min(global_sequence) as min_seq,
  max(global_sequence) as max_seq,
  md5(string_agg(event_id || ':' || payload_hash, ',' order by global_sequence)) as manifest_hash
from ledger_events;
```

Trading projections:

```sql
select md5(string_agg(client_order_id || ':' || state || ':' || coalesce(updated_at::text,''), ',' order by client_order_id)) from trading.order_projection;
select md5(string_agg(account_id || ':' || symbol || ':' || side || ':' || quantity::text, ',' order by account_id, symbol, side)) from trading.position_projection;
```

PnL:

```sql
select account_id, sum(net_pnl) as net_pnl from core.closed_positions group by account_id order by account_id;
select account_key, sum(net_pnl) as net_pnl from public.paper_trades group by account_key order by account_key;
```

## MongoDB Count Validation

```javascript
db.getCollectionNames().sort().forEach((name) => {
  print(`${name}: ${db.getCollection(name).countDocuments({})}`);
});
```

Required named collections:

- `paper_trades`
- `paper_state`
- `paper_research`
- `paper_oms_orders`
- `desk_worker_events`
- `desk_worker_lease`
- `policy_snapshots`
- `verification_track_events`
- `ai_app_tracker_reports`
- `auth_users`
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

## MongoDB Hash Validation

For each collection, export canonical extended JSON and hash:

```bash
mongoexport --uri "$MONGODB_URI" --db "$MONGODB_DB" --collection paper_trades --jsonArray --out paper_trades.json
sha256sum paper_trades.json
```

For large collections, hash stable identity fields:

```javascript
db.paper_trades.find({}, {client_trade_id:1, account_key:1, closed_at:1, net_pnl:1, _id:0})
  .sort({client_trade_id:1})
```

## SQLite Validation

```sql
PRAGMA integrity_check;
select count(*) from engine_state;
select count(*) from trades;
select count(*) from ai_audit_logs;
select total_pnl, total_trades, total_wins, total_losses from engine_state where id = 1;
```

## Trading Validation

Required comparisons:

- Order count by account, symbol, state.
- Position count by account, symbol, side, status.
- Fill/execution count by account, symbol, day.
- PnL totals by account, day, strategy, symbol.
- Strategy counts from source registry and DB strategy tables.
- Backtest counts and total net/gross PnL by strategy version.
- Research tournament counts and promotion history.
- Paper trade count and PnL by module.

Example:

```sql
select account_id, symbol, status, count(*) from core.positions group by account_id, symbol, status;
select account_id, symbol, date_trunc('day', closed_at), sum(net_pnl) from core.closed_positions group by account_id, symbol, date_trunc('day', closed_at);
```

Mongo:

```javascript
db.paper_trades.aggregate([
  {$group: {_id: {account_key:"$account_key", strategy_id:"$strategy_id"}, trades: {$sum:1}, net: {$sum:"$net_pnl"}}},
  {$sort: {"_id.account_key":1, "_id.strategy_id":1}}
]);
```

## Replay Consistency Validation

1. Run full account replay from `ledger_events`.
2. Build OMS order projections.
3. Build open position projections.
4. Build PnL projection.
5. Build risk projection.
6. Compare to restored projection tables and dashboard API outputs.

Pass criteria:

- Zero aggregate sequence gaps.
- Zero duplicate event ids.
- Zero duplicate idempotency keys.
- Open position projections match DB and UI.
- PnL projection matches closed trades and portfolio snapshots.
- Reconciliation drift equals zero or documented accepted exceptions.

## Dashboard Metrics Validation

Prometheus:

```bash
curl -G http://localhost:9090/api/v1/query --data-urlencode 'query=up'
curl -G http://localhost:9090/api/v1/query --data-urlencode 'query=trading_execution_kill_switch_active'
```

Grafana:

- Dashboards load without datasource errors.
- Clone labels do not include original hostnames.
- Alert rules evaluate.

## Certification Output

Create a validation bundle containing:

- `source_counts.txt`
- `clone_counts.txt`
- `source_hashes.txt`
- `clone_hashes.txt`
- `replay_report.json`
- `reconciliation_report.json`
- `dashboard_validation.md`
- `config_redacted_manifest.txt`

The final clone is not certified until these artifacts match or all deviations are approved.
