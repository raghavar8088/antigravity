# DR Observability Dashboard — Phase 15I
**Grafana Dashboard Specification**  
**Version:** 1.0 | **Date:** 2026-06-02

---

## Dashboard: `trading-dr-overview`

Import this dashboard into Grafana. It provides a real-time view of all DR and HA subsystems.

---

## Row 1: Cluster Health

### Panel 1.1 — Cluster Status
**Type:** Stat  
**Metric:**
```promql
ha_cluster_healthy{node_id=~"$node"}
```
**Thresholds:** `1` = green (Healthy), `0` = red (Degraded)

### Panel 1.2 — Active Leader
**Type:** Stat  
**Metric:**
```promql
ha_leader_is_leader{node_id=~"$node"} == 1
```

### Panel 1.3 — Cluster Quorum
**Type:** Stat  
**Metric:**
```promql
ha_cluster_quorum{node_id=~"$node"}
```

### Panel 1.4 — Nodes Alive / Dead
**Type:** Gauge  
**Metrics:**
```promql
ha_heartbeat_nodes_alive{node_id=~"$node"}
ha_heartbeat_nodes_dead{node_id=~"$node"}
```

---

## Row 2: Failover History

### Panel 2.1 — Leadership Failovers (24h)
**Type:** Time series  
**Metric:**
```promql
increase(ha_leader_failovers_total{node_id=~"$node"}[1h])
```

### Panel 2.2 — DB Failovers (24h)
**Type:** Stat  
**Metric:**
```promql
ha_db_failovers_total{node_id=~"$node"}
```
**Thresholds:** `0` = green, `> 0` = yellow, `> 3` = red

### Panel 2.3 — Redis Failovers
**Type:** Stat  
**Metric:**
```promql
ha_redis_failovers_total{node_id=~"$node"}
```

### Panel 2.4 — Exchange Failovers
**Type:** Stat  
**Metric:**
```promql
ha_exchange_failovers_total{node_id=~"$node"}
```

---

## Row 3: Recovery Performance

### Panel 3.1 — Last RPO
**Type:** Gauge  
**Metric:**
```promql
ha_recovery_rpo_seconds{node_id=~"$node"}
```
**Thresholds:** `0–30` = green, `30–60` = yellow, `> 60` = red  
**Target line:** 30 seconds

### Panel 3.2 — Last RTO
**Type:** Gauge  
**Metric:**
```promql
ha_recovery_rto_seconds{node_id=~"$node"}
```
**Thresholds:** `0–60` = green, `60–300` = yellow, `> 300` = red  
**Target line:** 300 seconds (5 min)

### Panel 3.3 — Recovery State
**Type:** Stat  
**Metric:**
```promql
ha_recovery_state{node_id=~"$node"}
```
**Value mappings:** `0=Idle`, `1=InProgress`, `2=Complete`, `3=Failed`

### Panel 3.4 — Events Replayed (last recovery)
**Type:** Stat  
**Metric:**
```promql
ha_recovery_events_replayed_total{node_id=~"$node"}
```

---

## Row 4: Replication Health

### Panel 4.1 — Ledger Replication Lag
**Type:** Time series  
**Metric:**
```promql
ha_ledger_replication_lag_seconds{node_id=~"$node"}
```
**Alert threshold:** 5 seconds  
**Unit:** seconds

### Panel 4.2 — Events Behind Primary
**Type:** Stat  
**Metric:**
```promql
ha_ledger_behind_primary_events{node_id=~"$node"}
```
**Thresholds:** `0–100` = green, `> 100` = yellow, `> 1000` = red

### Panel 4.3 — DB Replication Lag
**Type:** Time series  
**Metric:**
```promql
ha_db_replication_lag_seconds{node_id=~"$node"}
```

### Panel 4.4 — Ledger Integrity Violations
**Type:** Stat  
**Metric:**
```promql
ha_ledger_integrity_violations_total{node_id=~"$node"}
```
**Thresholds:** `0` = green, `> 0` = critical RED

---

## Row 5: Backup Status

### Panel 5.1 — Last Ledger Backup Age
**Type:** Gauge  
**Metric:**
```promql
backup_last_age_seconds{node_id=~"$node", type="ledger"}
```
**Thresholds:** `0–120` = green, `120–300` = yellow, `> 300` = red  
**Target:** < 120 seconds

### Panel 5.2 — Last DB Backup Age
**Type:** Gauge  
**Metric:**
```promql
backup_last_age_seconds{node_id=~"$node", type="db_full"}
```
**Thresholds:** `0–3600` = green, `> 3600` = yellow, `> 7200` = red

### Panel 5.3 — Backup Verification Pass Rate
**Type:** Stat  
**Metric:**
```promql
rate(backup_verification_valid_total{node_id=~"$node"}[1h]) /
(rate(backup_verification_valid_total{node_id=~"$node"}[1h]) +
 rate(backup_verification_invalid_total{node_id=~"$node"}[1h]))
```
**Thresholds:** `1.0` = green, `< 1.0` = red

### Panel 5.4 — Backup Errors
**Type:** Time series  
**Metric:**
```promql
increase(backup_errors_total{node_id=~"$node"}[1h])
```

---

## Row 6: Exchange Health

### Panel 6.1 — Exchange Status Grid
**Type:** Table  
**Metrics:**
```promql
ha_exchange_status{node_id=~"$node"}
```
**Value mappings:** `0=Healthy`, `1=Degraded`, `2=Down`

### Panel 6.2 — Active Exchange
**Type:** Stat  
**Metric:**
```promql
ha_exchange_active{node_id=~"$node"} == 1
```
**Label display:** `exchange`

### Panel 6.3 — Exchange Health Check Latency
**Type:** Time series  
**Metric:**
```promql
ha_exchange_health_check_latency_seconds{node_id=~"$node"}
```

---

## Row 7: DR Test Results

### Panel 7.1 — DR Tests Passed (7d)
**Type:** Stat  
**Metric:**
```promql
increase(dr_test_passed_total{node_id=~"$node"}[7d])
```

### Panel 7.2 — DR Tests Failed (7d)
**Type:** Stat  
**Metric:**
```promql
increase(dr_test_failed_total{node_id=~"$node"}[7d])
```
**Thresholds:** `0` = green, `> 0` = red

### Panel 7.3 — DR Test RPO by Scenario
**Type:** Bar chart  
**Metric:**
```promql
dr_test_rpo_seconds{node_id=~"$node"}
```

### Panel 7.4 — DR Test RTO by Scenario
**Type:** Bar chart  
**Metric:**
```promql
dr_test_rto_seconds{node_id=~"$node"}
```

---

## AlertManager Rules

```yaml
groups:
  - name: ha_dr_alerts
    rules:
      - alert: ClusterNoLeader
        expr: sum(ha_leader_is_leader) == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "No leader elected in trading cluster"

      - alert: LedgerIntegrityViolation
        expr: increase(ha_ledger_integrity_violations_total[5m]) > 0
        for: 0s
        labels:
          severity: critical
        annotations:
          summary: "Ledger hash chain violation detected"

      - alert: LedgerReplicationLagHigh
        expr: ha_ledger_replication_lag_seconds > 10
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Ledger replication lag exceeds 10 seconds"

      - alert: BackupStaleLedger
        expr: backup_last_age_seconds{type="ledger"} > 300
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Ledger backup is more than 5 minutes old"

      - alert: ExchangeDown
        expr: ha_exchange_status > 1
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Exchange {{ $labels.exchange }} is DOWN"

      - alert: RPOBreached
        expr: ha_recovery_rpo_seconds > 30
        for: 0s
        labels:
          severity: critical
        annotations:
          summary: "RPO target breached: {{ $value }}s > 30s"

      - alert: RTOBreached
        expr: ha_recovery_rto_seconds > 300
        for: 0s
        labels:
          severity: critical
        annotations:
          summary: "RTO target breached: {{ $value }}s > 300s"

      - alert: DRTestFailed
        expr: increase(dr_test_failed_total[1h]) > 0
        for: 0s
        labels:
          severity: warning
        annotations:
          summary: "DR test scenario failed: {{ $labels.scenario }}"
```

---

## Dashboard Variables

```
$node    : label_values(ha_leader_is_leader, node_id)
$exchange: label_values(ha_exchange_status, exchange)
$scenario: label_values(dr_test_passed_total, scenario)
```
