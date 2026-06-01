# Phase 15C — Disaster Recovery Runbook

**RPO target:** < 5 minutes  
**RTO target:** < 15 minutes  
**Date:** 2026-06-01

---

## Architecture Overview

```
PostgreSQL (Neon)
  ├── ledger_events         — immutable append-only event log (PRIMARY source of truth)
  ├── ledger_snapshots      — periodic aggregate state snapshots (accelerates replay)
  └── ledger_aggregate_sequences — per-aggregate sequence counters

SQLite (local fallback)
  └── engine_state          — legacy balance/position snapshot (Phase 14 and earlier)

MemoryStore (runtime)
  └── in-process cache      — drained on restart, rebuilt from PostgreSQL
```

---

## Recovery Scenarios

### Scenario 1 — Render Restart (Normal)

**Cause:** Free-tier Render instance restart, deploy, or OOM kill.  
**Impact:** In-memory event store lost.  
**Recovery:** Automatic.

```
Boot sequence:
  1. Engine starts
  2. PostgresStore.Connect(DATABASE_URL)
  3. PostgresStore.CreateSchema() — idempotent, no-op if already exists
  4. Bootstrap(store, snapshotStore, {AccountID: "btc-paper-1"})
     a. Load latest snapshot from ledger_snapshots
     b. ReplayAccount() → all events for account
     c. Filter delta events (SequenceNo > snapshot.GlobalSequence)
     d. Return BootstrapResult{DeltaEvents, OpenPositionSnapshots}
  5. positions.Manager.RestorePositions(BootstrapPositions(result))
  6. PaperClient.RestoreBalance(pnlProjection)
  7. Orchestrator resumes — strategies warm up from historical candles
```

**Verification:**
```bash
# Check bootstrap log
grep "BOOTSTRAP" engine.log | tail -5
# Expected:
# [BOOTSTRAP] ✅ Snapshot loaded | account=btc-paper-1 | lastSeq=12345 | ...
# [BOOTSTRAP] ✅ Ready | snapshotUsed=true | deltaEvents=42 | totalInLedger=12387
```

---

### Scenario 2 — PostgreSQL Unavailable (Neon outage)

**Cause:** Neon maintenance window, network partition.  
**Impact:** Ledger events cannot be written; OMS v3 events fail silently.  
**Recovery:** Engine continues with MemoryStore fallback; reconnects automatically.

```
Fallback sequence:
  1. PostgresStore.Connect() fails
  2. Engine falls back to MemoryStore (warn in log)
  3. All events appended to MemoryStore (in-process only)
  4. When PostgreSQL recovers:
     a. Re-connect via connection pool retry (pgxpool auto-retries)
     b. Flush pending MemoryStore events to PostgreSQL via AppendBatch()
     c. Reconcile sequences to prevent gaps
```

**Manual recovery steps:**
```bash
# 1. Verify PostgreSQL is accessible
psql $DATABASE_URL -c "SELECT COUNT(*) FROM ledger_events;"

# 2. Check for sequence gaps
psql $DATABASE_URL -c "
  SELECT aggregate_id, COUNT(*) AS event_count, MAX(aggregate_sequence) AS last_seq
  FROM ledger_events
  WHERE account_id = 'btc-paper-1'
  GROUP BY aggregate_id
  ORDER BY last_seq DESC
  LIMIT 20;
"

# 3. Verify no duplicate events
psql $DATABASE_URL -c "
  SELECT event_id, COUNT(*) AS cnt
  FROM ledger_events
  GROUP BY event_id
  HAVING COUNT(*) > 1;
"
```

---

### Scenario 3 — Partial Write (Engine Crash Mid-Trade)

**Cause:** Engine killed between `EventOrderFilled` and `EventPositionOpened`.  
**Impact:** Order marked as filled in ledger but no corresponding position event.  
**Detection:** Reconciliation service detects FILLED order with no open position.

```
Recovery sequence:
  1. Engine restarts → Bootstrap replays events
  2. BootstrapPositions() finds no position for the filled order
  3. Reconciliation.Check() emits AlertGhostOrder for the orphaned order
  4. Operator investigates → determines if position was actually opened
  5. Manual repair: append EventPositionOpened with correct parameters
     OR append EventPositionClosed if position was never opened
```

**Automated detection:**
```bash
# Find orders that are FILLED but have no corresponding POSITION_OPENED event
psql $DATABASE_URL -c "
  SELECT e.aggregate_id AS order_id, e.symbol
  FROM ledger_events e
  WHERE e.event_type = 'ORDER_FILLED'
    AND e.account_id = 'btc-paper-1'
    AND e.correlation_id NOT IN (
      SELECT DISTINCT correlation_id
      FROM ledger_events
      WHERE event_type = 'POSITION_OPENED'
        AND account_id = 'btc-paper-1'
    );
"
```

---

### Scenario 4 — Ledger Corruption (Payload Hash Mismatch)

**Cause:** Storage bit-rot, manual database modification.  
**Impact:** Bootstrap rejects corrupted events (with `FailOnCorruption: true`).  
**Detection:** `[BOOTSTRAP] ⚠️ Event hash mismatch` in logs.

```
Recovery sequence:
  1. Set FailOnCorruption: false in BootstrapConfig (skip corrupted events)
  2. Engine starts in degraded mode (missing corrupted events)
  3. Identify corrupted events:
     SELECT event_id, aggregate_id, payload_hash
     FROM ledger_events
     WHERE length(payload_hash) != 64;  -- SHA-256 is always 64 chars
  4. Restore from snapshot that predates the corruption
  5. Re-run reconciliation to detect any position drift
```

**Verification query:**
```sql
-- Find events with invalid hash format
SELECT event_id, aggregate_id, event_type, created_at
FROM ledger_events
WHERE payload_hash !~ '^[0-9a-f]{64}$'
ORDER BY created_at DESC;

-- Cross-check a specific event's hash
SELECT
  event_id,
  payload_hash AS stored_hash,
  encode(sha256(payload::text::bytea), 'hex') AS recomputed_hash
FROM ledger_events
WHERE event_id = 'YOUR_EVENT_ID';
```

---

### Scenario 5 — Kill Switch Recovery (Full Flatten)

**Cause:** Daily loss limit hit, operator trigger, exchange outage.  
**Recovery:** Automatic when positions are flattened.

```
Kill switch sequence:
  1. KillSwitch.Trigger(TriggerDailyLoss/TriggerManualOperator)
     → EventKillSwitchTriggered appended to ledger (AggregateRisk)
  2. Executor.FlattenPositions()
     → positions.Manager.CloseAllPositions(currentPrice)
     → CloseEvents emitted for each position
  3. processCloseEvents() receives each CloseEvent:
     → EventPositionClosed appended to ledger per position
  4. Executor.CancelOpenOrders() — if applicable
  5. PreTradeRiskPipeline blocks all new orders (KillSwitch.IsActive() = true)
  
Reset sequence (after manual review):
  1. Operator confirms issue is resolved
  2. POST /api/admin/kill {"action": "release"}  — resets kill switch
  3. EventKillSwitchReleased appended to ledger
  4. New orders may resume
```

**Verification:**
```sql
-- Confirm all positions closed after kill switch
SELECT COUNT(*) AS open_positions
FROM (
  SELECT aggregate_id
  FROM ledger_events
  WHERE event_type = 'POSITION_OPENED'
    AND account_id = 'btc-paper-1'
  EXCEPT
  SELECT aggregate_id
  FROM ledger_events
  WHERE event_type = 'POSITION_CLOSED'
    AND account_id = 'btc-paper-1'
) t;
-- Expected: 0
```

---

## Point-In-Time Recovery

To restore the engine state to any historical point-in-time:

```go
// Example: restore to state as of 2026-06-01T10:30:00Z
cfg := ledger.BootstrapConfig{
    AccountID:         "btc-paper-1",
    SnapshotThreshold: 999999, // don't auto-snapshot during recovery
    FailOnCorruption:  false,
}

// 1. Load snapshot closest to (but before) the target time
snap, found, _ := snapStore.LoadAccountSnapshot(ctx, "btc-paper-1")

// 2. Replay only events up to the target time
allEvents, _ := store.ReplayAccount(ctx, "btc-paper-1")
deltaUntil := filterEventsUntil(allEvents, snap.GlobalSequence, targetTime)

// 3. Build projections from filtered events
positions := omsv3.BuildOpenPositionProjections(deltaUntil)
pnl := omsv3.BuildPnLProjection(deltaUntil)
```

---

## Automated Recovery Test

Run this test before every production deployment to verify replay correctness:

```bash
cd engine && go test -mod=mod -run TestBootstrap ./internal/ledger/... -v
```

Manual verification sequence:
```bash
# 1. Start engine normally and let it trade for a few minutes
# 2. Note the current balance and open positions from the dashboard
# 3. Kill the engine (SIGKILL to simulate crash)
# 4. Restart the engine
# 5. Compare positions.Manager state to dashboard display
# 6. Expected: identical open positions, no orphaned aggregates
```

---

## Backup Schedule

| Asset | Schedule | Retention | Tool |
|-------|----------|-----------|------|
| `ledger_events` table | Continuous (Neon PITR) | 7 days | Neon built-in |
| `ledger_snapshots` table | Continuous (Neon PITR) | 7 days | Neon built-in |
| Snapshot trigger | Every 10,000 events | indefinite | Bootstrap auto |
| Manual snapshot | Pre-deploy / post-incident | indefinite | `TakeSnapshot()` |

---

## Schema Rollback

If a schema migration needs to be rolled back:

```sql
-- Rollback: remove Phase 15C tables
-- WARNING: This destroys all event history. Only run if you have a backup.
-- DROP TABLE IF EXISTS ledger_events CASCADE;
-- DROP TABLE IF EXISTS ledger_snapshots CASCADE;
-- DROP TABLE IF EXISTS ledger_aggregate_sequences CASCADE;
-- DROP VIEW IF EXISTS v_open_positions;
-- DROP VIEW IF EXISTS v_closed_positions;
-- DROP VIEW IF EXISTS v_daily_pnl;

-- Safe rollback (rename tables, keep data):
-- ALTER TABLE ledger_events RENAME TO ledger_events_backup_20260601;
-- ALTER TABLE ledger_snapshots RENAME TO ledger_snapshots_backup_20260601;
```

---

## Remaining Blockers (Phase 15D)

| Blocker | Impact | Owner |
|---------|--------|-------|
| `PostgresStore` not yet wired in `main.go` | ENGINE uses MemoryStore on production | Architecture |
| MemoryStore → PostgresStore flush on reconnect | Events lost during Neon outage window | Backend |
| Snapshot auto-trigger in orchestrator | Manual snapshot only currently | Backend |
| `positions.Manager` not yet reading from OMS v3 on boot | Positions not restored from ledger | Architecture |
| Kill switch `FlattenPositions` executor not wired | KillSwitch.Trigger has no real Executor | Backend |
