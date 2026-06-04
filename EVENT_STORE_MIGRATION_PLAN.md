# Event Store Migration Plan

Generated from repository discovery on 2026-06-02.

## Event-Sourcing Components

| Component | Location | Role | Persistence |
| --- | --- | --- | --- |
| Ledger interface and memory store | `engine/internal/ledger/store.go` | Append/replay contract and in-memory implementation | In-memory unless wrapped by durable store |
| Postgres ledger store | `engine/internal/ledger/postgres_store.go` | Durable append-only event store with per-aggregate sequences and idempotency | PostgreSQL tables `ledger_events`, `ledger_aggregate_sequences` |
| Snapshot stores | `engine/internal/ledger/snapshots.go`, `engine/internal/ledger/postgres_snapshot_store.go` | Bootstrap acceleration and recovery snapshots | Memory or PostgreSQL `ledger_snapshots` |
| Replay logic | `engine/internal/ledger/replay.go` | Account replay grouped by aggregate type | Derived from event store |
| OMS v3 command bus | `engine/internal/omsv3/aggregate_invariants.go` | Validates commands and appends canonical events | Ledger first, projections derived |
| OMS v3 projections | `engine/internal/omsv3/projections.go`, `authority.go`, `replay_engine.go`, `snapshot_provider.go` | Orders, positions, PnL, dashboard projections | Rebuildable in memory / projection tables |
| Reconciliation v2 | `engine/internal/reconciliationv2/` | Drift detection and repair events | Ledger-derived repair/projection state |
| Risk v3 projections | `engine/internal/riskv3/portfolio_projection.go` | Dashboard CQRS risk projections | Rebuildable from risk events |
| HA replication/integrity | `engine/internal/ha/ledger_replication.go`, `ledger_integrity.go` | Standby replay, checkpoints, hash-chain verification | Postgres `ledger_events`, `ha_*` tables |
| Backup/restore | `engine/backup/` | Backup catalog, restore manager, verification | Local backup files and Postgres |

## Event Store Schemas

Two related schemas were observed:

1. Standalone durable ledger:
   - `ledger_aggregate_sequences`
   - `ledger_events`
   - `ledger_snapshots`
   - Views such as `v_open_positions`
   - Source: `infrastructure/database/event_store.sql` and `engine/internal/ledger/postgres_store.go`

2. Phase 14 Timescale schema:
   - `trading.event_store`
   - `trading.order_projection`
   - `trading.fill_projection`
   - `trading.position_projection`
   - `trading.pnl_1m`
   - Source: `infrastructure/database/phase14_timescale_schema.sql`

The migration must preserve both if both exist in the live database.

## Aggregates

Observed aggregate families from replay and invariant code:

- `ORDER`
- `POSITION`
- `RISK`
- `STRATEGY`
- `EXCHANGE`
- `SYSTEM`
- `RECONCILIATION`
- `MARKET_DATA`
- `ACCOUNT`

## Event Payload Families

Observed canonical payload types:

- Position opened
- Position closed/liquidated
- Stop-loss moved / breakeven / trailing stop
- Risk approved / blocked / violation
- Strategy registered/enabled/disabled/paused/resumed/promoted/demoted/allocation changed
- Exchange connected/disconnected/degraded/gap detected/recovered
- Engine started/stopped
- Replay started/completed
- Snapshot created/restored
- Projection rebuilt
- Kill switch released
- Reconciliation resolved

## Counts To Capture

Live counts were not queried. Capture before and after migration:

```sql
select count(*) as ledger_event_count from ledger_events;
select count(*) as ledger_snapshot_count from ledger_snapshots;
select count(*) as aggregate_count from ledger_aggregate_sequences;
select aggregate_type, count(*) as events from ledger_events group by aggregate_type order by aggregate_type;
select event_type, count(*) as events from ledger_events group by event_type order by event_type;
select account_id, count(*) as events from ledger_events group by account_id order by account_id;
select count(distinct aggregate_type || ':' || aggregate_id) as distinct_aggregates from ledger_events;
```

For Phase 14 schema:

```sql
select count(*) as trading_event_count from trading.event_store;
select aggregate_type, count(*) from trading.event_store group by aggregate_type order by aggregate_type;
select count(*) as order_projection_count from trading.order_projection;
select count(*) as fill_projection_count from trading.fill_projection;
select count(*) as position_projection_count from trading.position_projection;
```

## Export Plan

1. Freeze writers:
   - Disable Vercel cron routes.
   - Stop PM2 worker.
   - Stop Go engine or activate kill switch and block writes.
   - Confirm no active leader is appending events.
2. Record original high-water marks:
   - `max(global_sequence)` from `ledger_events`
   - `max(sequence_no)` from `trading.event_store`
   - `max(created_at)` from each event table
3. Export event store with schema:

```bash
pg_dump "$DATABASE_URL" --format=custom --table=ledger_aggregate_sequences --table=ledger_events --table=ledger_snapshots --file=ledger.dump
pg_dump "$DATABASE_URL" --format=custom --schema=trading --file=trading_event_schema.dump
```

4. Export checksum manifest:

```sql
select
  min(global_sequence) as min_seq,
  max(global_sequence) as max_seq,
  count(*) as events,
  encode(sha256(string_agg(event_id || ':' || payload_hash, ',' order by global_sequence)::bytea), 'hex') as event_manifest_hash
from ledger_events;
```

If `sha256` is not available, export ordered `event_id,payload_hash` and hash the file externally.

## Restore Plan

1. Restore into isolated clone database.
2. Disable clone writers until validation completes.
3. Restore SQL dump:

```bash
pg_restore --dbname "$CLONE_DATABASE_URL" --clean --if-exists --no-owner ledger.dump
pg_restore --dbname "$CLONE_DATABASE_URL" --clean --if-exists --no-owner trading_event_schema.dump
```

4. Rebuild derived projections from event source:
   - OMS v3 order projections
   - Position projections
   - PnL projection
   - Risk dashboard projections
   - Reconciliation projections
   - System history projection
5. Compare projection counts and aggregate checksums to the original.

## Replay Requirements

- Replay must sort by `created_at`, with `aggregate_sequence`/global sequence as tie-breaker where code requires it.
- Payload hashes must be verified on read.
- Idempotency keys must remain unique.
- Aggregate sequence continuity must hold:

```sql
with seqs as (
  select aggregate_type, aggregate_id, count(*) c, max(aggregate_sequence) m
  from ledger_events
  group by aggregate_type, aggregate_id
)
select * from seqs where c <> m;
```

Expected result: zero rows.

## Recovery Requirements

- Latest snapshots must be loadable for each account.
- Delta replay after snapshot must reproduce open positions and PnL.
- If snapshot checksum fails, fall back to full event replay.
- HA standby must warm projections before serving reads.
- Kill switch remains active until:
  - DB restore validated
  - Event replay validated
  - Reconciliation drift is zero or accepted
  - Broker/exchange endpoints are clone-safe

## Validation Queries

```sql
-- Event integrity
select count(*) from ledger_events where payload_hash is null or payload_hash = '';

-- Duplicate event ids
select event_id, count(*) from ledger_events group by event_id having count(*) > 1;

-- Duplicate idempotency keys
select idempotency_key, count(*) from ledger_events
where idempotency_key is not null
group by idempotency_key having count(*) > 1;

-- Snapshot coverage
select account_id, count(*) as snapshots, max(created_at) as latest
from ledger_snapshots
group by account_id;

-- Projection counts
select count(*) from trading.order_projection;
select count(*) from trading.position_projection;
select count(*) from trading.fill_projection;
```

## Open Items

- Live event counts: not available until database access is provided.
- Live aggregate counts: not available until database access is provided.
- Projection counts: not available until database access is provided.
- Snapshot counts: not available until database access is provided.

The clone cannot be certified as replay-identical until these counts and hashes match between original and clone.
