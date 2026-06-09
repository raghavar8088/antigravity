# 11 — High Availability Test Plan

**Frequency:** Pre-go-live + quarterly  
**Duration:** 4 hours (staging) / 2 hours (production smoke)  
**Regions:** ap-south-1

---

## Compute (ECS)

| Test | Method | Expected RTO | Pass Criteria |
|------|--------|------------|---------------|
| HA-ECS-01 Task failure | `aws ecs stop-task --task $ARN` | < 90s | New task healthy, ALB routes traffic |
| HA-ECS-02 AZ failure | Stop all tasks in AZ-a | < 120s | Tasks in AZ-b healthy, count ≥ 2 |
| HA-ECS-03 Leader failover | Stop leader task | < 5s | Follower becomes leader, trading resumes |
| HA-ECS-04 Rolling deploy | Push new image | < 180s | Zero 5xx spike > 10/min |
| HA-ECS-05 Circuit breaker | Deploy broken health check | < 300s | Auto-rollback to previous task def |

### Procedure: Leader Failover (HA-ECS-03)

```bash
# Identify leader
curl "$ENGINE_URL/metrics" | grep ha_leader_is_leader
# Stop leader task
aws ecs stop-task --cluster $CLUSTER --task $LEADER_ARN --reason "HA test"
# Verify follower promoted within 5s
watch -n1 'curl -s $ENGINE_URL/metrics | grep ha_leader'
```

---

## Database (Aurora)

| Test | Method | Expected RTO | Pass Criteria |
|------|--------|------------|---------------|
| HA-DB-01 Writer failure | `aws rds failover-db-cluster` | < 60s | Engine reconnects, writes resume |
| HA-DB-02 Replica promotion | Manual promote reader | < 60s | New writer accepts connections |
| HA-DB-03 PITR recovery | Restore to 1h ago test cluster | < 30 min | Ledger events intact |
| HA-DB-04 Connection storm | 100 concurrent connections | < 5s | Pool handles, no crash |

### Procedure: Aurora Failover (HA-DB-01)

```bash
aws rds failover-db-cluster --db-cluster-identifier antigravity-production-aurora
# Monitor engine logs for reconnect
aws logs tail /ecs/antigravity-production/engine --follow | grep LEDGER
# Verify kill switch state persisted
curl "$ENGINE_URL/api/admin/ks/status" -H "X-Engine-Admin-Secret: $SECRET"
```

---

## Cache (Redis)

| Test | Method | Expected RTO | Pass Criteria |
|------|--------|------------|---------------|
| HA-REDIS-01 Node failure | ElastiCache test failover API | < 30s | Engine reconnects |
| HA-REDIS-02 AZ failure | Primary AZ outage simulation | < 60s | Replica promoted |
| HA-REDIS-03 Cache miss | Redis unavailable 5 min | N/A | Trading continues (degraded) |

---

## End-to-End HA Sequence

```
T+0    Stop leader ECS task
T+5s   Verify new leader elected
T+10s  Trigger Aurora failover
T+40s  Verify engine reconnected to Aurora
T+45s  Trigger Redis failover
T+75s  Verify engine reconnected to Redis
T+90s  Submit test paper order — must succeed
T+120s Verify reconciliation clean
```

---

## Expected Recovery Times Summary

| Component | RTO Target | RTO Measured | RPO |
|-----------|------------|--------------|-----|
| ECS task | 90s | TBD | 0 (stateless) |
| Leader election | 5s | TBD | 0 |
| Aurora failover | 60s | TBD | < 5 min (PITR) |
| Redis failover | 30s | TBD | 0 (cache) |
| Full AZ loss | 15 min | TBD | < 5 min |

---

## Operational Runbooks

| Scenario | Runbook Section |
|----------|-----------------|
| ECS task unhealthy | [12-disaster-recovery-runbook.md](./12-disaster-recovery-runbook.md) §ECS |
| Aurora outage | §Aurora |
| Redis outage | §Redis |
| Multi-component failure | §Full Recovery |

---

## Sign-Off

Document results in `infrastructure/production-readiness/drills/YYYY-MM-DD-ha-test.md`

| Role | Sign-off |
|------|----------|
| SRE Lead | _____________ |
| Trading Ops | _____________ |
| Security | _____________ |

**HA Test Plan Readiness:** 70/100 (designed, not executed)
