# SLO / SLA Definitions — Phase 15H

## Overview

Service Level Objectives (SLOs) define the quantitative targets the trading platform must meet.
Service Level Agreements (SLAs) are the contractual commitments based on those objectives.

All percentages are measured over rolling 30-day windows.

---

## Tier 1 — Critical (Capital-Impacting)

### Market Data Availability

| Metric | Target | Measurement |
|--------|--------|-------------|
| Feed uptime | 99.95% | `avg_over_time(trading_marketdata_exchange_connected[30d])` |
| Tick latency P99 | <5ms | `histogram_quantile(0.99, rate(trading_marketdata_tick_processing_latency_ms_bucket[30d]))` |
| Staleness | <5s | `max(trading_marketdata_staleness_seconds)` |
| Error budget | 21.9 min/month downtime | SLA breach triggers incident |

---

### OMS Availability

| Metric | Target | Measurement |
|--------|--------|-------------|
| OMS uptime | 99.99% | `trading_health_component_score{component="oms"}` |
| Order acceptance rate | ≥99% | `rate(orders_accepted) / rate(orders_submitted)` |
| Fill latency P99 | <100ms | `histogram_quantile(0.99, rate(trading_oms_fill_latency_ms_bucket[5m]))` |
| Error budget | 4.3 min/month downtime | SLA breach triggers critical incident |

---

### Reconciliation Authority Availability

| Metric | Target | Measurement |
|--------|--------|-------------|
| Reconciliation uptime | 99.99% | `trading_health_component_score{component="reconciliation"}` |
| Cycle completion | Every 60s | `increase(recon_v2_cycles_total[2m]) > 0` |
| Drift detection latency | <120s | Time from drift occurrence to detection |
| Ghost positions outstanding | 0 | `recon_v2_position_drift_count{type="ghost"} == 0` |
| Missing fills outstanding | 0 | `recon_v2_fill_drift_count{type="missing"} == 0` |

---

### Risk Engine Availability

| Metric | Target | Measurement |
|--------|--------|-------------|
| Risk engine uptime | 99.99% | `trading_health_component_score{component="risk"}` |
| Risk check latency P99 | <5ms | `histogram_quantile(0.99, rate(trading_latency_pipeline_stage_ms_bucket{stage="strategy_to_risk"}[5m]))` |
| False positive block rate | <5% | `rate(blocks) / rate(total_checks)` |

---

### Execution Pipeline

| Metric | Target | Measurement |
|--------|--------|-------------|
| Signal-to-fill latency P50 | <50ms | `histogram_quantile(0.50, rate(trading_latency_e2e_ms_bucket[5m]))` |
| Signal-to-fill latency P99 | **<150ms** | `histogram_quantile(0.99, rate(trading_latency_e2e_ms_bucket[5m]))` |
| SLO compliance (30d) | ≥99.5% of fills within 150ms | `1 - (breach_count / total_fills)` |

---

## Tier 2 — Important (Operational)

### Replay Engine Availability

| Metric | Target | Measurement |
|--------|--------|-------------|
| Replay engine uptime | 99.95% | Health check |
| Replay throughput | ≥10,000 events/sec | `trading_ledger_replay_throughput_eps` |
| Full replay time (RTO) | <15 minutes | `trading_dr_rto_estimate_seconds < 900` |

---

### Ledger (Event Store)

| Metric | Target | Measurement |
|--------|--------|-------------|
| Write availability | 99.99% | Health check |
| Write latency P99 | <25ms | `histogram_quantile(0.99, rate(trading_ledger_write_latency_ms_bucket[5m]))` |
| Snapshot age | <300s | `trading_dr_snapshot_age_seconds` |

---

### Database (PostgreSQL / TimescaleDB)

| Metric | Target | Measurement |
|--------|--------|-------------|
| Availability | 99.95% | `trading_db_available{db="postgres"}` |
| Query latency P99 | <100ms | `histogram_quantile(0.99, ...)` |
| WAL lag | <1MB | `trading_dr_wal_lag_bytes` |

---

### Redis

| Metric | Target | Measurement |
|--------|--------|-------------|
| Availability | 99.9% | `trading_redis_available` |
| Command latency P99 | <5ms | `histogram_quantile(0.99, rate(trading_redis_command_latency_ms_bucket[5m]))` |

---

## Tier 3 — DR Targets

| Objective | Target | Current Estimate |
|-----------|--------|-----------------|
| RPO (Recovery Point Objective) | **<5 minutes** | `trading_dr_rpo_estimate_seconds` |
| RTO (Recovery Time Objective) | **<15 minutes** | `trading_dr_rto_estimate_seconds` |
| Backup freshness | <1 hour | `trading_dr_backup_age_seconds` |
| DR drill frequency | Monthly | `trading_dr_last_drill_unix` |

---

## Error Budgets

| Service | Monthly Downtime Budget | Daily Budget |
|---------|------------------------|--------------|
| OMS (99.99%) | 4.3 minutes | 8.6 seconds |
| Risk Engine (99.99%) | 4.3 minutes | 8.6 seconds |
| Reconciliation (99.99%) | 4.3 minutes | 8.6 seconds |
| Market Data (99.95%) | 21.9 minutes | 43.8 seconds |
| Replay / Ledger (99.95%) | 21.9 minutes | 43.8 seconds |
| PostgreSQL (99.95%) | 21.9 minutes | 43.8 seconds |
| Redis (99.9%) | 43.8 minutes | 87.6 seconds |

---

## SLO Breach Response

| Severity | Condition | Action |
|----------|-----------|--------|
| **CRITICAL** | Any Tier 1 SLO breach | Trigger incident, PagerDuty alert |
| **HIGH** | Tier 2 SLO breach | Slack #trading-alerts |
| **MEDIUM** | Error budget >50% consumed in first 15 days | Review + remediation plan |
| **LOW** | Error budget >25% consumed | Monitoring only |

---

## Prometheus Recording Rules (add to prometheus.yml)

```yaml
groups:
  - name: slo_recording_rules
    interval: 60s
    rules:
      - record: slo:oms_availability:ratio_rate5m
        expr: avg_over_time(trading_health_component_score{component="oms"}[5m])

      - record: slo:execution_latency_slo_compliance:ratio_rate5m
        expr: |
          1 - (
            rate(trading_latency_slo_breaches_total[5m])
            /
            rate(trading_latency_e2e_ms_count[5m])
          )

      - record: slo:market_data_availability:ratio_rate5m
        expr: avg_over_time(trading_marketdata_exchange_connected[5m])

      - record: slo:reconciliation_cycle_health:ratio_rate5m
        expr: clamp_max(increase(recon_v2_cycles_total[2m]) > bool 0, 1)
```
