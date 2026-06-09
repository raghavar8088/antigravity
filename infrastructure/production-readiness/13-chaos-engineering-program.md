# 13 — Chaos Engineering Program

**Philosophy:** Inject controlled failures in staging; validate detection, alerting, and recovery before production incidents.

---

## Program Structure

| Tier | Environment | Frequency | Blast Radius |
|------|-------------|-----------|--------------|
| L1 | Local/dev | Every PR (automated) | Single process |
| L2 | Staging ECS | Weekly | Full stack, paper only |
| L3 | Production | Quarterly | Read-only + failover only |
| L4 | Production | Annual | Full chaos (maintenance window) |

---

## Failure Injection Matrix

| ID | Failure | Injection Tool | Detection | Recovery | Pass |
|----|---------|----------------|-----------|----------|------|
| CHAOS-01 | Exchange outage | Block outbound to Delta/Binance SG | `trading_exchange_connected=0` | Fallback chain activates | < 30s |
| CHAOS-02 | Database outage | Aurora SG deny ECS | `aurora-connections` alarm | pgxpool reconnect on restore | < 60s |
| CHAOS-03 | Redis outage | Redis SG deny ECS | `redis-*` alarms | Degraded mode, no crash | < 30s |
| CHAOS-04 | ECS task failure | `aws ecs stop-task` | `ecs-running-tasks` alarm | Auto-restart | < 90s |
| CHAOS-05 | Network partition | iptables DROP broker IPs | Exchange disconnect metric | Trading pause + alert | < 15s |
| CHAOS-06 | High latency | `tc qdisc` 2000ms delay | `alb-latency` alarm | Timeouts handled gracefully | N/A |
| CHAOS-07 | Packet loss | `tc` 30% loss | Retry metrics increase | No duplicate orders | N/A |
| CHAOS-08 | Secrets retrieval failure | Revoke ECS IAM temporarily | Task fails to start | Fail-fast, no plaintext fallback | Immediate |
| CHAOS-09 | Leader split-brain attempt | Run 2 tasks without leader election | Dual write detection | MUST FAIL pre-go-live | Block deploy |
| CHAOS-10 | Kill switch under load | Flood signals + activate KS | All orders stop < 1 tick | Positions flatten | < 5s |

---

## Quarterly Chaos Schedule

### Q1 — Compute + Network

| Week | Experiment |
|------|------------|
| W1 | CHAOS-04 ECS task kill |
| W2 | CHAOS-05 Network partition |
| W3 | CHAOS-06 High latency |
| W4 | Report + remediation |

### Q2 — Data Layer

| Week | Experiment |
|------|------------|
| W1 | CHAOS-02 Database outage |
| W2 | CHAOS-03 Redis outage |
| W3 | CHAOS-07 Packet loss |
| W4 | Report + remediation |

### Q3 — Trading Path

| Week | Experiment |
|------|------------|
| W1 | CHAOS-01 Exchange outage |
| W2 | CHAOS-10 Kill switch under load |
| W3 | CHAOS-04 + CHAOS-02 combined |
| W4 | Full DR drill |

### Q4 — Security + Secrets

| Week | Experiment |
|------|------------|
| W1 | CHAOS-08 Secrets failure |
| W2 | Pen-test retest (see doc 14) |
| W3 | CHAOS-09 Leader election validation |
| W4 | Annual chaos report |

---

## Experiment Template

```markdown
# Chaos Experiment: CHAOS-XX
Date: YYYY-MM-DD
Environment: staging
Hypothesis: System detects X within Ys and recovers within Zs
Steady State: trading_reconciliation_last_check < 15s ago, 2 healthy ECS tasks
Injection: [command]
Observations: [metrics, logs, alerts]
Result: PASS/FAIL
Remediation: [if FAIL]
```

---

## Tooling

| Tool | Use |
|------|-----|
| AWS FIS | ECS task stop, AZ impairment |
| `tc` / `iptables` | Network chaos on staging |
| Custom Go chaos hooks | Exchange adapter mock failures |
| CloudWatch Synthetics | Post-chaos health verification |

Reference: `infrastructure/performance/failure-scenarios.yml`

---

## Abort Criteria

Stop experiment immediately if:
- Real capital at risk (live mode accidentally enabled)
- Kill switch fails to activate on drift
- Duplicate orders detected
- Data corruption in ledger

---

## Sign-Off

**Chaos Program Readiness:** 68/100 (program designed, L2+ not executed)
