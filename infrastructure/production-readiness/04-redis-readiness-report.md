# 04 — Redis Readiness Report

**Service:** ElastiCache Redis 7.1 Replication Group  
**Purpose:** Hot cache, indicator cache, idempotency dedup window (ephemeral)

---

## Configuration Validation

| Requirement | Terraform Setting | Status |
|-------------|-------------------|--------|
| TLS in transit | `transit_encryption_enabled=true`, mode=`required` | ✅ |
| Encryption at rest | `at_rest_encryption_enabled=true` | ✅ |
| Auth token | `random_password.redis_auth` (64 chars) | ✅ |
| Multi-AZ | `multi_az_enabled=true` | ✅ |
| Auto-failover | `automatic_failover_enabled=true` | ✅ |
| Node count | `redis_num_cache_nodes=2` (default) | ✅ |
| Node type | `cache.t4g.small` (1.37 GiB) | ✅ |
| Snapshot retention | 7 days | ✅ |
| Snapshot window | 03:00–04:00 UTC | ✅ |
| Slow log | CloudWatch JSON delivery | ✅ |
| Eviction policy | `allkeys-lru` | ✅ |
| Keyspace events | `notify-keyspace-events=KEA` | ✅ |

---

## Connection String Format

```
rediss://:AUTH_TOKEN@PRIMARY_ENDPOINT:6379/0
```

Stored in Secrets Manager key `REDIS_URL`. Engine injects via ECS task `secrets[]`.

**Validation:**
```bash
redis-cli -u "$REDIS_URL" --tls PING
# Expected: PONG
```

---

## Data Classification

| Key Pattern | Durability | Recovery |
|-------------|------------|----------|
| `live:position:{id}` | Ephemeral | Rebuild from Aurora ledger |
| `live:equity` | Ephemeral | Recompute from positions |
| `indicator:{symbol}:*` | Ephemeral | Recompute from candles |
| `dedupe:{key}` | Ephemeral (TTL window) | Safe to lose — idempotency in Postgres |

**Mandate:** Redis is NOT source of truth. Aurora ledger is authoritative.

---

## Failover Behavior

| Event | Detection | Recovery | RTO |
|-------|-----------|----------|-----|
| Primary node failure | ElastiCache auto-detect | Promote replica | 10–30s |
| AZ failure | Multi-AZ failover | New primary in other AZ | 30s |
| Auth token mismatch | Engine connection error | Fix Secrets Manager | Manual |
| Memory pressure | `redis-memory` alarm at 80% | Scale node type or evict | 5 min |

### Engine Reconnect Logic

Engine must:
1. Use Redis client with TLS + auth
2. Retry with exponential backoff on connection loss
3. Fall back to direct computation (no cache) during outage
4. Never block trading solely due to cache miss

---

## Snapshot Schedule

| Setting | Value |
|---------|-------|
| `snapshot_retention_limit` | 7 |
| `snapshot_window` | 03:00–04:00 UTC (08:30–09:30 IST) |
| `maintenance_window` | Sunday 05:00–06:00 UTC |

Snapshots are for operational recovery of hot cache — not trading state recovery.

---

## Monitoring Alarms

| Alarm | Threshold | Action |
|-------|-----------|--------|
| `redis-cpu` | CPU ≥ 80% for 2 min | Scale node type |
| `redis-memory` | Memory ≥ 80% for 2 min | Review key TTLs, scale |

---

## Validation Tests

| Test | Procedure | Pass |
|------|-----------|------|
| TLS required | Connect without TLS → rejected | Connection fails |
| Auth required | Connect without token → rejected | Connection fails |
| Write/read | SET/GET test key | Success |
| Failover | `aws elasticache test-failover` | Reconnect < 30s |
| Engine degraded mode | Block Redis SG temporarily | Trading continues |

Run: `bash scripts/production-readiness/validate-infrastructure.sh --redis`

---

## Phase 14 Schema Reference

Hot cache key schema: `infrastructure/REDIS_PHASE14_SCHEMA.md`

---

## Sign-Off

| Item | Status |
|------|--------|
| Terraform Redis config | ✅ VALIDATED |
| REDIS_URL in Secrets Manager | ❌ PENDING |
| Engine Redis client wired | ⚠️ PARTIAL (uses REDIS_URL if set) |
| Failover tested | ❌ PENDING |

**Redis Readiness Score:** 75/100
