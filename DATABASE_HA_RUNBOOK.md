# Database High Availability Runbook
**Version:** 1.0 | **Date:** 2026-06-02 | **Severity:** P1 on primary failure

---

## Architecture

```
Trading Engine ──▶ TimescaleDB Primary (R/W)
                         │
                         ▼ Streaming Replication
                  TimescaleDB Replica (R)
                         │
                   (auto-promotion on primary failure)
```

The `DatabaseFailover` component (`engine/internal/ha/database_failover.go`) monitors both endpoints every **3 seconds** and switches automatically after **3 consecutive failures**.

---

## Automatic Failover

No human action required. The engine detects primary failure and switches the connection pool to the replica within **9–15 seconds**.

Failover sequence:
1. Health check fails 3× (9 seconds)
2. `OnFailover` callbacks invoked — reconciliation re-routes
3. Engine writes to replica (promoted automatically or via pg_promote())
4. Alert fires: `DBPrimaryFailover` → Slack/PagerDuty
5. Replication lag metric resets to 0 (single node)

---

## Manual Primary Promotion

If automatic promotion does not occur within 30 seconds:

```bash
# 1. Confirm primary is down
psql "$REPLICA_URL" -c "SELECT pg_is_in_recovery();"
# Returns 't' if in standby mode

# 2. Promote replica to primary
psql "$REPLICA_URL" -c "SELECT pg_promote();"
# Returns 't' on success

# 3. Update DATABASE_URL in secrets
kubectl -n trading edit secret engine-secrets
# Change DATABASE_URL to replica connection string

# 4. Restart engine pods to pick up new URL
kubectl -n trading rollout restart statefulset/trading-engine
```

---

## Replication Lag Monitoring

```bash
# On primary: check streaming replication status
psql "$PRIMARY_URL" -c "
  SELECT client_addr, state, sent_lsn, write_lsn, replay_lsn,
         (sent_lsn - replay_lsn) AS replication_lag_bytes
  FROM pg_stat_replication;
"

# Prometheus metric
ha_db_replication_lag_seconds > 5  # Alert threshold
```

---

## Scenario: Primary OOM Kill

1. Kubernetes pod is OOM-killed
2. Kubernetes automatically restarts pod (restartPolicy: Always)
3. PostgreSQL WAL-based recovery on restart
4. Replica continues serving reads during primary restart
5. Engine `DatabaseFailover` switches to replica within 9–15s
6. Primary recovers, becomes replica of the new primary (manual reconfiguration needed)

**Expected total trading downtime: 0** (writes route to promoted replica)

---

## Scenario: Storage Full

```bash
# Identify bloat
psql "$PRIMARY_URL" -c "
  SELECT schemaname, tablename,
         pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
  FROM pg_tables
  ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC
  LIMIT 20;
"

# Archive old ledger events (events > 90 days not needed for live trading)
psql "$PRIMARY_URL" -c "
  DELETE FROM ledger_events
  WHERE created_at < NOW() - INTERVAL '90 days'
    AND aggregate_type NOT IN ('ACCOUNT', 'POSITION'); -- keep account/position history
"

# Vacuum to reclaim space
psql "$PRIMARY_URL" -c "VACUUM FULL ANALYZE ledger_events;"
```

---

## Backup Restore Procedure

If both primary and replica are lost (catastrophic):

```bash
# 1. Identify latest backup
ls -lt /data/backups/ | grep db_full | head -5

# 2. Start fresh PostgreSQL instance
docker run -d --name pg-restore \
  -e POSTGRES_PASSWORD=secret \
  -v /data/backups:/backups \
  timescale/timescaledb:latest-pg15

# 3. Restore using RestoreManager
go run ./cmd/restore/main.go \
  --backup-path /backups/trading_node1_db_full_20260602T120000Z.bak \
  --target ledger \
  --node-id trading_node1

# 4. Verify event count
psql "$NEW_DB_URL" -c "SELECT COUNT(*) FROM ledger_events;"

# 5. Run recovery engine to rebuild OMS
go run ./cmd/antigravity/main.go --recovery-only
```

---

## Health Check Commands

```bash
# Primary health
psql "$PRIMARY_URL" -c "SELECT 1;"

# Replica health and lag
psql "$REPLICA_URL" -c "
  SELECT pg_is_in_recovery() AS is_replica,
         pg_last_xact_replay_timestamp() AS last_replayed,
         NOW() - pg_last_xact_replay_timestamp() AS lag;
"

# Engine DB pool health
curl http://engine:8080/health/db
```

---

## RPO/RTO for DB Failures

| Scenario | RPO | RTO |
|----------|-----|-----|
| Primary crash, replica healthy | 0 seconds (ledger replay) | 9–15 seconds |
| Primary + replica crash | < 60 seconds (last backup) | 15–30 minutes (restore) |
| Storage corruption | < 60 seconds (last backup) | 15–30 minutes |
