# Multi-Region Disaster Recovery Plan
**Version:** 1.0 | **Date:** 2026-06-02

---

## Region Architecture

```
                    ┌─────────────────────────────────────┐
                    │         PRIMARY REGION               │
                    │    (AWS ap-south-1 / Mumbai)         │
                    │                                      │
                    │  Trading Engine × 3 (StatefulSet)   │
                    │  TimescaleDB Primary                 │
                    │  TimescaleDB Replica                 │
                    │  Redis Primary + Replica + Sentinel  │
                    │  Backup Storage (S3)                 │
                    └──────────────┬──────────────────────┘
                                   │
                              Replication
                              (streaming)
                                   │
                    ┌──────────────▼──────────────────────┐
                    │       SECONDARY REGION               │
                    │   (AWS ap-southeast-1 / Singapore)   │
                    │                                      │
                    │  Trading Engine × 1 (hot standby)   │
                    │  TimescaleDB Replica                 │
                    │  Redis Replica                       │
                    │  Backup Storage (S3 cross-region)    │
                    └──────────────┬──────────────────────┘
                                   │
                              Backup Sync
                                   │
                    ┌──────────────▼──────────────────────┐
                    │         DR REGION                    │
                    │      (AWS eu-west-1 / Ireland)       │
                    │                                      │
                    │  Cold standby (spin up on demand)    │
                    │  Backup Storage (S3 cross-region)    │
                    └─────────────────────────────────────┘
```

---

## Failover Tiers

### Tier 1: In-Region Failover (Automatic)
- **Trigger:** Primary engine pod fails
- **Action:** Kubernetes restarts pod; HA leader election promotes standby
- **RTO:** < 60 seconds
- **RPO:** 0 (ledger-based)

### Tier 2: Intra-Region DB Failover (Automatic)
- **Trigger:** TimescaleDB primary failure
- **Action:** `DatabaseFailover` promotes replica; engine reconnects
- **RTO:** < 2 minutes
- **RPO:** < 30 seconds

### Tier 3: Cross-Region Failover (Manual Trigger, Automated Execution)
- **Trigger:** Full primary region failure (AZ outage, cloud provider incident)
- **Action:** Promote secondary region; update DNS; engines restart with secondary DB
- **RTO:** < 15 minutes
- **RPO:** < 60 seconds (based on ledger replication lag)

---

## Cross-Region Replication Strategy

### Ledger Replication
The `LedgerReplicator` continuously streams ledger events from primary to secondary DB:
- Poll interval: 500ms
- Batch size: 500 events
- Checkpoint: persisted in secondary DB
- Lag alert threshold: 5 seconds

### Backup Replication
All backup artifacts are written to primary S3, then replicated to secondary and DR S3 buckets using S3 Cross-Region Replication (CRR) with 15-minute SLA.

---

## Cross-Region Failover Procedure

### Prerequisites
- [ ] Primary region confirmed unavailable (not just network partition)
- [ ] Secondary DB replication lag confirmed < 5 minutes
- [ ] On-call engineer approval

### Step 1: Promote Secondary DB (2 minutes)
```bash
# SSH to secondary region bastion
ssh bastion.secondary.trading.internal

# Promote TimescaleDB replica to primary
psql "$SECONDARY_DB_URL" -c "SELECT pg_promote();"

# Verify promotion
psql "$SECONDARY_DB_URL" -c "SELECT pg_is_in_recovery();"
# Must return 'f' (false = primary mode)
```

### Step 2: Update DNS (1 minute)
```bash
# Update Route 53 to point to secondary region
aws route53 change-resource-record-sets \
  --hosted-zone-id $ZONE_ID \
  --change-batch file://failover-dns-change.json
```

### Step 3: Update Secrets (1 minute)
```bash
# Update DATABASE_URL in secondary cluster secrets
kubectl -n trading --context=secondary \
  patch secret engine-secrets \
  --patch '{"data":{"DATABASE_URL":"'$(echo -n "$SECONDARY_DB_URL" | base64)'"}}'
```

### Step 4: Start Secondary Engines (3–5 minutes)
```bash
# Scale up secondary trading engine
kubectl -n trading --context=secondary scale statefulset trading-engine --replicas=3

# Monitor startup
kubectl -n trading --context=secondary rollout status statefulset trading-engine
```

### Step 5: Validate (2 minutes)
```bash
# Health check
curl https://engine.secondary.trading.internal/health

# OMS state check — open orders should be present
curl https://engine.secondary.trading.internal/api/oms/state

# Reconciliation — compare with exchange state
curl https://engine.secondary.trading.internal/api/reconciliation/status
```

### Step 6: Notify Stakeholders
- [ ] Update status page
- [ ] Notify risk team of failover
- [ ] Send P1 incident update

---

## Failback to Primary Region

After primary region is restored:

1. Sync all events generated during failover back to primary DB
2. Set primary DB as downstream replica of secondary
3. Verify replication lag < 1 second
4. Gradually shift traffic: 10% → 50% → 100%
5. Revert DNS
6. Scale down secondary engines

---

## Backup Retention Policy

| Backup Type | Retention | Location |
|-------------|-----------|----------|
| Ledger (1 min) | 7 days | Primary + Secondary S3 |
| DB Snapshot (1 hr) | 30 days | Primary + Secondary + DR S3 |
| Full Infra (24 hr) | 90 days | All 3 S3 regions |

---

## Recovery Point Analysis

| Scenario | Last Good State | RPO |
|----------|----------------|-----|
| Engine crash | Last ledger event | 0 |
| Region outage (replication current) | Last replicated event | < 5 seconds |
| Region outage (replication lagging) | Last replicated event | Up to 5 minutes |
| All regions lost | Last backup | Up to 1 minute |
