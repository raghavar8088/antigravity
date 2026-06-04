# Ledger Compatibility Report

Generated for Phase 16A clone certification on 2026-06-02.

## Certification Verdict

Status: not certified for a true 100% clone.

Classification impact: ledger state is cloneable only with manual schema mapping, replay validation, and live count/hash manifests. The repository has strong in-memory replay coverage, but durable event-store schemas are fragmented and not yet a single canonical replay contract.

## Ledger Schema Versions

Canonical Go ledger schema:

- `engine/internal/ledger/event.go` defines the canonical event envelope.
- `engine/internal/ledger/event_migrations.go` defines `CurrentSchemaVersion = 2`.
- New events created through `ledger.NewEvent()` are stamped with schema version 2.
- New events appended through `ledger.NewMigratingStore()` are also stamped with schema version 2.

Discovered durable schema variants:

- `engine/internal/ledger/postgres_store.go`
  - Tables: `ledger_aggregate_sequences`, `ledger_events`.
  - Event table defaults `schema_version INT NOT NULL DEFAULT 1`.
  - Runtime inserts use `event.SchemaVersion`, so Go-created events should be v2, but ad hoc SQL inserts can still default to v1.
- `infrastructure/database/event_store.sql`
  - Tables: `ledger_aggregate_sequences`, `ledger_events`, `ledger_snapshots`.
  - Event table defaults `schema_version INT DEFAULT 1`.
  - Snapshot table has `version INT DEFAULT 1`.
- `infrastructure/database/phase14_timescale_schema.sql`
  - Table: `trading.event_store`.
  - Uses `sequence_no BIGSERIAL PRIMARY KEY` and `schema_version INTEGER DEFAULT 1`.
  - This is not the same physical schema as the Go `PostgresStore` table.
- `client/supabase/migrations/009_institutional_database_foundation.sql`
  - Table: `audit.event_store`.
  - Uses `event_version INTEGER DEFAULT 1`, not `schema_version`.
- `engine/internal/ha/ledger_replication.go`
  - Defines another replica-side `ledger_events` shape using `sequence_no` and binary payload concepts, not the current Go store shape.

Certification finding: schema version 2 exists only as the current Go event envelope. SQL DDL still defaults to version 1 in multiple places, and one legacy schema uses `event_version` rather than `schema_version`.

## Event Versions

Canonical event version:

- Version 1: implied legacy envelope and payloads. The migration code treats events below current version as upgrade candidates.
- Version 2: current canonical envelope. The migration adds or backfills schema metadata in payload JSON and source metadata when missing.

Migration registry:

- `engine/internal/ledger/event_migrations.go` contains one registered migration: v1 to v2, global across all event types.
- `MigrateEvent()` applies registered migrations until `CurrentSchemaVersion`.
- If no migration is found for an older event, the code currently sets the event schema to current. This is a blocker because incompatible historical payloads can be silently treated as compatible.

Canonical event types:

- Order lifecycle: `ORDER_CREATED`, `ORDER_VALIDATED`, `ORDER_ACCEPTED`, `ORDER_SUBMITTED`, `ORDER_ACKED`, `ORDER_PARTIAL`, `ORDER_FILLED`, `ORDER_CANCELLED`, `ORDER_REJECTED`, `ORDER_EXPIRED`, `ORDER_REPLACE_REQUESTED`, `ORDER_REPLACED`.
- Position lifecycle: `POSITION_OPENED`, `POSITION_SCALED`, `POSITION_CHANGED`, `POSITION_REDUCED`, `POSITION_CLOSED`, `POSITION_LIQUIDATED`, `POSITION_TRANSFERRED`.
- Position management: `POSITION_SL_MOVED`, `POSITION_BREAKEVEN_ACTIVATED`, `POSITION_TRAILING_ACTIVATED`, `POSITION_TP_MOVED`, `POSITION_RISK_ADJUSTED`.
- Risk: `RISK_CHECK_STARTED`, `RISK_APPROVED`, `RISK_BLOCKED`, `RISK_VIOLATION`, `RISK_TRIGGERED`, `EXPOSURE_LIMIT_EXCEEDED`, `PORTFOLIO_HEAT_EXCEEDED`, `VAR_BREACH`, `CVAR_BREACH`, `MAX_DRAWDOWN_BREACHED`, `FUNDING_EXPOSURE_EXCEEDED`, `RISK_DAILY_LOSS_LIMIT_EXCEEDED`, `RISK_MARGIN_VIOLATION`, `RISK_LEVERAGE_VIOLATION`, `RISK_CONCENTRATION_VIOLATION`, `RISK_CORRELATION_VIOLATION`.
- Kill switch: `KILLSWITCH_TRIGGERED`, `KILLSWITCH_RELEASED`.
- Strategy: `STRATEGY_REGISTERED`, `STRATEGY_ENABLED`, `STRATEGY_DISABLED`, `STRATEGY_PAUSED`, `STRATEGY_RESUMED`, `STRATEGY_PROMOTED`, `STRATEGY_DEMOTED`, `STRATEGY_ALLOCATION_CHANGED`.
- Exchange and market data: `EXCHANGE_CONNECTED`, `EXCHANGE_DISCONNECTED`, `EXCHANGE_RECONNECTED`, `EXCHANGE_ORDER_REJECTED`, `EXCHANGE_LATENCY_SPIKE`, `EXCHANGE_RATE_LIMIT_HIT`, `EXCHANGE_DATA_GAP_DETECTED`, `EXCHANGE_OUTAGE`, `MARKET_DATA_STALE`, `MARKET_DATA_RECOVERED`.
- System: `ENGINE_STARTING`, `ENGINE_STARTED`, `ENGINE_STOPPING`, `ENGINE_STOPPED`, `REPLAY_STARTED`, `REPLAY_COMPLETED`, `PROJECTION_REBUILT`, `SNAPSHOT_CREATED`, `SNAPSHOT_RESTORED`.
- Reconciliation: `RECONCILIATION_STARTED`, `RECONCILIATION_MISMATCH`, `RECONCILIATION_ALERT`, `RECONCILIATION_CORRECTED`, `RECONCILIATION_RESOLVED`.

Legacy SQL event type enum:

- `client/supabase/migrations/009_institutional_database_foundation.sql` defines `audit.event_type` values such as `OrderCreated`, `OrderSubmitted`, `OrderFilled`, `PositionOpened`, `RiskViolation`, and `KillSwitchTriggered`.
- These names do not match the canonical Go uppercase event constants.

## Aggregate Versions

Canonical aggregate types:

- `ORDER`
- `POSITION`
- `RISK`
- `ACCOUNT`
- `MARKET_DATA`
- `RECONCILIATION`
- `STRATEGY`
- `EXCHANGE`
- `SYSTEM`

Aggregate state versioning:

- `OrderAggregate.Version`, `PositionAggregate.Version`, `RiskAggregate`, `StrategyAggregate`, and `SystemAggregate` are replay-derived state counters or domain state, not persisted schema-version contracts.
- No explicit aggregate schema version registry was found.
- Snapshot schema versioning is inconsistent. SQL snapshots have `version INT DEFAULT 1`; Go snapshots do not provide a fully materialized, versioned aggregate-state contract for all aggregate types.

Certification finding: aggregate state is replayable from canonical events, but aggregate state schemas themselves are not independently versioned.

## Projection Versions

Projection builders found:

- `omsv3.BuildOrderProjections()`
- `omsv3.BuildPositionProjections()`
- `omsv3.BuildOpenPositionProjections()`
- `omsv3.BuildPnLProjection()`
- `omsv3.BuildExposureProjection()`
- `omsv3.BuildRiskProjection()`
- `omsv3.BuildStrategyProjections()`
- `omsv3.BuildExchangeProjections()`
- `omsv3.BuildSystemProjection()`
- `omsv3.BuildDashboardProjection()`
- Reconciliation v2 projection and audit builders under `engine/internal/reconciliationv2/`.

Projection versioning:

- No explicit projection schema version registry was found.
- SQL projection tables in `infrastructure/database/phase14_timescale_schema.sql` are rebuildable read models, not authoritative event state.
- Some projections intentionally cap history for dashboards or repair display, making them unsuitable as exact clone authorities.

Certification finding: projections are rebuildable, but projection equality cannot prove a 100% clone without ledger count/hash equality.

## Replay Compatibility

Replay paths found:

- `ledger.ReplayEverything()` replays all account events and partitions by aggregate type.
- `ledger.ReplayOrders()`, `ReplayPositions()`, `ReplayRisk()`, `ReplayStrategies()`, `ReplaySystem()`, and `ReplayAggregate()` provide scoped replay helpers.
- `omsv3.ReplayAll()` rebuilds order, position, strategy, and system aggregate state from ledger events only.
- `omsv3.ReplayOpenOrders()`, `ReplayOpenPositions()`, `ReplayActiveStrategies()`, `ReplayPnL()`, `ReplayExposure()`, and `ReplayDashboard()` rebuild operational projections.
- `ledger.VerifySequence()` detects per-aggregate sequence gaps.
- `ledger.DetectOutOfOrder()` and `ledger.DeduplicateEvents()` support replay hygiene.

Compatibility controls found:

- Payload hashes are validated on append.
- Idempotency keys are indexed for deduplication.
- Per-aggregate sequence assignment is transactional in `PostgresStore`.
- `MigratingStore` migrates events on replay reads when explicitly used.

Compatibility gaps:

- `PostgresStore.Replay()` and `PostgresStore.ReplayAccount()` do not automatically migrate unless wrapped by `NewMigratingStore()`.
- Unknown old versions can be silently marked current by `MigrateEvent()`.
- Durable stores and SQL migrations use multiple incompatible event-store shapes.
- Snapshot/global-sequence semantics are inconsistent between the Go store and SQL snapshot docs.
- HA and backup code references do not all match the current `ledger_events` column names.

## Executed Dry-Run Tests

Initial command:

```bash
go test ./internal/strategy ./internal/ledger ./internal/omsv3 ./internal/reconciliationv2
```

Result: failed before running tests because `engine/vendor` is inconsistent with `go.mod`.

Second command:

```bash
go test -mod=mod ./internal/strategy ./internal/ledger ./internal/omsv3 ./internal/reconciliationv2
```

Result: passed for strategy, ledger, OMS v3, and reconciliation v2 packages.

Interpretation:

- Source-level replay and aggregate tests are currently passing when vendor drift is bypassed.
- Vendor drift is itself a reproducibility blocker because a default Go test/build uses the inconsistent vendor directory.

## Certification Blockers

1. Unify or explicitly map all durable event-store schemas before restore.
2. Make replay migration mandatory for all durable reads, not dependent on caller wrapping.
3. Replace silent unknown-version promotion with a hard compatibility failure or explicit migration policy.
4. Add a canonical aggregate schema version registry or state snapshot version contract.
5. Add a canonical projection version registry if projections are to be compared during clone certification.
6. Resolve `schema_version` vs `event_version` and uppercase Go event names vs legacy SQL enum names.
7. Fix HA/backup/recovery SQL to match the canonical durable ledger schema.
8. Produce live source and clone manifests for event count, snapshot count, aggregate sequence count, max global sequence, per-aggregate gaps, and payload hash totals.
9. Resolve Go vendor drift so default repo builds/tests are reproducible without `-mod=mod`.

## Required Clone Validation

Before claiming a true clone, collect and compare these values on source and clone:

```sql
select count(*) from ledger_events;
select count(*) from ledger_snapshots;
select count(*) from ledger_aggregate_sequences;
select max(global_sequence) from ledger_events;
select aggregate_type, aggregate_id, max(aggregate_sequence), count(*)
from ledger_events
group by aggregate_type, aggregate_id
order by aggregate_type, aggregate_id;
```

Also run:

```bash
go test -mod=mod ./internal/ledger ./internal/omsv3 ./internal/reconciliationv2
```

Certification remains blocked until source and clone manifests match and replay rebuilds OMS state without drift.
