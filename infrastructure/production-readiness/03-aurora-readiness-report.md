# 03 — Aurora Readiness Report

**Engine:** Aurora PostgreSQL 15.4 Serverless v2  
**Database:** `antigravity`  
**Role:** Event ledger write authority + kill switch + PMS state

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Aurora PostgreSQL Cluster                 │
│  ┌─────────────────┐         ┌─────────────────┐            │
│  │ Writer (AZ-a)   │ ◄─sync─►│ Reader (AZ-b)   │            │
│  │ db.serverless   │         │ db.serverless   │            │
│  │ 0.5–8 ACU       │         │ failover target │            │
│  └────────┬────────┘         └────────┬────────┘            │
│           │                           │                      │
│  ledger_events (append-only)         │ read replicas         │
│  ledger_aggregate_sequences          │ for dashboards        │
│  ledger_snapshots                    │                      │
└─────────────────────────────────────────────────────────────┘
         ▲                                    ▲
         │ writes (leader task only)          │ reads (projections)
    Go PostgresStore                    Vercel read models (future)
```

---

## Event Ledger Schema

**DDL:** `infrastructure/database/event_store.sql`  
**Runtime:** `ledger.PostgresStore.CreateSchema()` at boot

| Table | Purpose | Mutability |
|-------|---------|------------|
| `ledger_events` | Append-only event log | INSERT only |
| `ledger_aggregate_sequences` | Per-aggregate sequence counters | UPSERT |
| `ledger_snapshots` | Aggregate state checkpoints | UPSERT |

### Idempotency

```sql
CREATE UNIQUE INDEX idx_ledger_idempotency
    ON ledger_events (idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

Duplicate submissions return `ErrDuplicateEvent` — safe on replay.

### Event Types (Production)

| Category | Events |
|----------|--------|
| Orders | ORDER_CREATED, SUBMITTED, ACKED, FILLED, CANCELLED |
| Positions | POSITION_OPENED, MARK_UPDATE, CLOSED |
| Risk | RISK_APPROVED, RISK_REJECTED, DAILY_LOSS_BREACH |
| Kill Switch | KS_ACTIVATED, KS_RELEASED |
| Reconciliation | RECONCILIATION_ALERT, OMS_DESYNC_DETECTED |

---

## High Availability

| Feature | Config | RTO |
|---------|--------|-----|
| Multi-AZ | Writer + reader in separate AZs | ~30s failover |
| Auto-failover | Aurora promotes reader | ~30s |
| PITR | `backup_retention_period=30` days | Point-in-time restore |
| AWS Backup | Daily 01:00 UTC + weekly Sunday | Cross-snapshot recovery |
| Deletion protection | Enabled | N/A |
| Force SSL | `rds.force_ssl=1` | Connection rejected without TLS |

---

## Connection Pooling

**Engine (direct):** `pgxpool` in `PostgresStore`

```go
MaxConns = 10
MinConns = 2
MaxConnLifetime = 30 min
MaxConnIdleTime = 5 min
```

**Production recommendation:** Add PgBouncer sidecar or RDS Proxy when connection count exceeds 80.

Config reference: `infrastructure/database/pgbouncer/pgbouncer.ini`

| Pool Mode | Use Case |
|-----------|----------|
| Transaction | Engine ledger writes (recommended) |
| Session | Leader election advisory lock (dedicated conn) |

**Critical:** Leader election uses a **dedicated connection** (not pooled) per `ha/leader_election.go`.

---

## Event Replay Procedures

### Boot Recovery (Required Pre-Production)

```go
// engine/cmd/antigravity/main.go — wire at boot
result, err := omsv3.ReplayAll(ctx, durableLedger, "btc-paper-1")
// Restore: open orders, positions, risk state, PnL projections
```

### Manual Replay (Operations)

```bash
# 1. Verify ledger integrity
psql $DATABASE_URL -c "SELECT COUNT(*) FROM ledger_events;"
psql $DATABASE_URL -c "SELECT event_type, COUNT(*) FROM ledger_events GROUP BY 1 ORDER BY 2 DESC;"

# 2. Replay single aggregate
# Use engine admin endpoint or CLI tool (Phase 4)

# 3. PITR restore to test cluster
bash infrastructure/database/scripts/restore-pitr.sh \
  --cluster antigravity-production-aurora \
  --restore-time "2026-06-09T10:00:00Z" \
  --target-cluster antigravity-pitr-test
```

---

## Recovery Procedures

### Scenario: Writer Failure

1. Aurora auto-promotes reader (~30s)
2. Engine pgxpool reconnects on next operation
3. Verify: `aws rds describe-db-clusters --query 'DBClusters[0].Status'`
4. Check replication lag alarm clears

### Scenario: Corrupt Event (Hash Mismatch)

1. `PostgresStore` returns `ErrHashMismatch` on read
2. Halt trading via kill switch
3. Identify event: `SELECT * FROM ledger_events WHERE event_id = '...'`
4. Restore from PITR to point before corruption
5. Replay events forward

### Scenario: Connection Exhaustion

1. Alarm: `aurora-connections >= 80`
2. Scale ACU: increase `aurora_max_capacity`
3. Add PgBouncer/RDS Proxy
4. Audit connection leaks in engine

---

## Backup Validation

| Backup Type | Retention | Validation |
|-------------|-----------|------------|
| Aurora automated | 30 days | Quarterly PITR restore drill |
| AWS Backup daily | 30 days cold storage | Monthly snapshot verify |
| AWS Backup weekly | 90 days | Quarterly full restore |
| Cross-region | Phase 5 | ap-southeast-1 copy |

**PITR Drill Script:** `infrastructure/database/scripts/backup-pitr.sh`

---

## Sign-Off Criteria

| Test | Pass |
|------|------|
| `CreateSchema()` succeeds on Aurora | ✅ |
| Kill switch persists across restart | Requires DATABASE_URL |
| Event append + idempotency | Go test `ledger` package |
| PITR restore to test cluster | Quarterly |
| Failover < 60s | HA test plan §Database |
| Replication lag < 10s | CloudWatch alarm |

**Aurora Readiness Score:** 78/100 (schema + code ready, not deployed, boot replay not wired)
