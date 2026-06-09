# 02 — ECS Readiness Report

**Service:** `antigravity-production-engine`  
**Cluster:** `antigravity-production-cluster`  
**Launch Type:** Fargate (awsvpc)

---

## Capacity Planning

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| CPU | 2048 (2 vCPU) | 600+ strategies, 1m candle evaluation |
| Memory | 4096 MiB | GOMEMLIMIT=3584MiB, headroom for GC |
| Desired count | 2 | Active-standby via leader election |
| Min capacity | 2 | Never below HA minimum |
| Max capacity | 6 | CPU auto-scale at 60% |
| Ephemeral storage | 21 GiB | Logs only — no SQLite in ECS |

### Load Profile

| Metric | Steady State | Peak (US session open) |
|--------|--------------|------------------------|
| CPU | 35–45% | 65–75% |
| Memory | 50–60% | 70–80% |
| Requests/min (ALB) | 50–200 | 500+ (dashboard polling) |
| Strategy evals/sec | ~10 | ~30 |

### Scaling Thresholds

```hcl
# ecs.tf — current configuration
target_value       = 60.0   # CPU %
scale_out_cooldown = 60     # 1 min — fast scale for trading spikes
scale_in_cooldown  = 300    # 5 min — avoid thrashing during volatility
min_capacity       = engine_desired_count  # 2
max_capacity       = 6
```

**Recommendation:** Add memory-based scaling policy at 75% as secondary trigger (Phase 4).

---

## Multi-AZ Deployment

```
┌──────────────── AZ-a ────────────────┐  ┌──────────────── AZ-b ────────────────┐
│  ECS Task (Fargate)                  │  │  ECS Task (Fargate)                  │
│  engine container :8080              │  │  engine container :8080              │
│  xray-daemon (non-essential)       │  │  xray-daemon (non-essential)       │
│  Role: LEADER or FOLLOWER          │  │  Role: LEADER or FOLLOWER          │
│  Private subnet + NAT egress       │  │  Private subnet + NAT egress       │
└────────────────────────────────────┘  └────────────────────────────────────┘
              │                                        │
              └──────────── ALB Target Group ──────────┘
```

ECS places tasks across private subnets (`aws_subnet.private[*].id`) spanning both AZs.

---

## Leader Election (CRITICAL — Pre-Deploy)

**Status:** Code exists (`engine/internal/ha/cluster.go`) — **NOT wired in `main.go`**

Before enabling `desired_count=2`:

1. Expose `PostgresStore.Pool()` or create shared pgxpool from `DATABASE_URL`
2. Wire `ha.NewCluster()` with node ID = ECS task ARN or hostname
3. Gate `orchestrator.Run()` on `cluster.IsLeader()`
4. Followers serve `/health` and `/metrics` only — no order submission

```go
// Required wiring pattern (main.go)
cluster := ha.NewCluster(ha.ClusterConfig{
    NodeID:     nodeID,
    EnginePort: port,
    Version:    buildVersion,
    Pool:       durableLedger.Pool(),
})
go cluster.Run(ctx)
cluster.WaitReady(ctx)
orchestrator.SetLeaderGate(func() bool { return cluster.IsLeader() })
```

**Without this:** Two tasks will both execute trades → capital risk.

---

## Health Checks

| Layer | Config | Pass Criteria |
|-------|--------|---------------|
| Container | `wget -qO- http://localhost:8080/health` | Exit 0, 30s interval |
| ALB | `GET /health`, matcher 200 | 2 healthy / 3 unhealthy |
| ECS deployment | Circuit breaker | Auto-rollback on failure |
| Start period | 60s | Allow warmup + DB connect |

### Health Endpoint Requirements

`/health` must return:
- Engine initialized
- MongoDB connected (or degraded mode documented)
- DATABASE_URL connected (production)
- Reconciliation goroutine started
- Leader role (leader/follower)

---

## Rolling Deployment

| Setting | Value | Effect |
|---------|-------|--------|
| `deployment_minimum_healthy_percent` | 50% | 1 of 2 tasks always serving |
| `deployment_maximum_percent` | 200% | New task starts before old drains |
| `deregistration_delay` | 30s | Fast failover for trading |
| Circuit breaker | enabled + rollback | Failed deploy auto-reverts |

### Graceful Shutdown Sequence

1. SIGTERM received → context cancelled
2. ALB deregistration (30s drain window)
3. Kill switch state persisted to Aurora (PostgresStore)
4. Open orders: OMS state in ledger (append-only)
5. Reconciliation goroutine exits cleanly
6. pgxpool connections closed

**Validation:** Deploy new image during paper trading; verify zero duplicate orders.

---

## CI/CD Pipeline

**Workflow:** `.github/workflows/deploy-ecs.yml`

| Step | Safety Control |
|------|----------------|
| OIDC auth | No long-lived AWS keys |
| Trivy scan | Blocks CRITICAL CVEs |
| `wait-for-service-stability` | Blocks until healthy |
| `runningCount >= 1` | Post-deploy verification |
| GitHub Environment `production` | Manual approval gate |

---

## ECS Validation Tests

| Test | Command | Pass |
|------|---------|------|
| Service stable | `aws ecs wait services-stable` | Exit 0 |
| 2 running tasks | `describe-services runningCount >= 2` | Yes |
| ALB healthy | `describe-target-health healthy >= 2` | Yes |
| Leader election | Kill leader task → new leader < 5s | Yes |
| Deploy rollback | Push broken image → circuit breaker reverts | Yes |
| Secret injection | ECS exec env check (no plaintext in task def) | Yes |

Run: `bash scripts/production-readiness/validate-infrastructure.sh`

---

## Sign-Off

| Item | Status |
|------|--------|
| Terraform ECS config | ✅ VALIDATED |
| Leader election wired | ❌ BLOCKER |
| ECS deployed | ❌ PENDING |
| 2-task HA verified | ❌ PENDING |
| Deploy pipeline tested | ❌ PENDING |

**ECS Readiness Score:** 72/100 (design complete, deployment + leader election pending)
