# 15 — Observability Validation Report

**Stack:** CloudWatch + Prometheus + X-Ray + SNS (+ Grafana external)

---

## Component Status

| Component | Configured | Deployed | Validated |
|-----------|------------|----------|-----------|
| CloudWatch Logs | ✅ 90-day retention | ❌ | ❌ |
| CloudWatch Alarms | ✅ 10+ alarms | ❌ | ❌ |
| CloudWatch Dashboard | ✅ Trading dashboard | ❌ | ❌ |
| Container Insights | ✅ ECS cluster | ❌ | ❌ |
| Prometheus `/metrics` | ✅ Engine endpoint | ✅ Lightsail | ⚠️ |
| X-Ray | ✅ Sidecar in task def | ❌ | ❌ |
| SNS Alerts | ✅ Email subscription | ❌ | ❌ |
| Grafana | ⚠️ JSON dashboard exists | ❌ | ❌ |

---

## Required Alerts

| Alert | Source | Threshold | Status |
|-------|--------|-----------|--------|
| Exchange disconnect | `trading_exchange_connected` | = 0 for 60s | ⚠️ Metric exists, alarm TBD |
| Reconciliation drift | `trading_reconciliation_alerts_total` | > 0 in 5min | ⚠️ Add to monitoring.tf |
| Order failures | `trading_order_failures_total` | > 10 in 5min | ⚠️ Add to monitoring.tf |
| Risk violations | `trading_risk_rejections_total` | > 50 in 5min | ⚠️ Add to monitoring.tf |
| Database failures | `aurora-connections` + engine errors | Connections > 80 | ✅ TF alarm |
| ECS failures | `ecs-running-tasks` | < 2 | ✅ TF alarm |
| Secret rotation failures | AWS/SecretsManager | RotationFailed | ❌ Add alarm |
| Kill switch active | `trading_kill_switch_active` | = 1 | ⚠️ Add to monitoring.tf |
| ALB 5xx | `alb-5xx-rate` | > 10/min | ✅ TF alarm |
| ALB unhealthy | `alb-unhealthy-hosts` | >= 1 | ✅ TF alarm |

---

## Prometheus Metrics (Engine)

Reference: `infrastructure/observability/PHASE14_PROMETHEUS_METRICS.md`

Key metrics to validate post-deploy:

```bash
curl -s "$ENGINE_URL/metrics" | grep -E "trading_|ha_|kill_switch"
```

| Metric | Expected |
|--------|----------|
| `trading_strategy_active_count` | > 0 |
| `trading_signal_generated_total` | Increasing |
| `trading_kill_switch_active` | 0 (normal) |
| `ha_leader_is_leader` | 1 on exactly one task |
| `ha_cluster_healthy` | 1 |

---

## CloudWatch Dashboard

**URL:** Terraform output `cloudwatch_dashboard_url`

Widgets validated in `monitoring.tf`:
- Engine tasks running
- CPU / Memory
- ALB p95 latency
- Aurora CPU + connections

**Add (Phase 4):**
- Kill switch status panel
- Reconciliation alert count
- Exchange connectivity
- Daily PnL (from ledger)

---

## Grafana Integration

Existing: `infrastructure/database/monitoring/grafana-database-dashboard.json`

Setup:
1. Prometheus remote write from engine `/metrics`
2. Or CloudWatch data source for AWS metrics
3. Import dashboard JSON
4. Configure alert rules → PagerDuty

---

## Tracing (X-Ray)

ECS task includes `xray-daemon` sidecar (non-essential).

Validate:
```bash
# After deploy, check X-Ray console for traces on /api/admin/ks/block
aws xray get-trace-summaries --start-time $(date -u -d '1 hour ago' +%s) \
  --end-time $(date -u +%s) --filter-expression 'service("engine")'
```

---

## Alerting Validation Procedure

| Step | Action | Pass |
|------|--------|------|
| 1 | Confirm SNS email subscription | Clicked confirm link |
| 2 | Trigger test alarm | `aws cloudwatch set-alarm-state --state-value ALARM` |
| 3 | Verify email received | < 5 min |
| 4 | Trigger kill switch | Alert to ops channel |
| 5 | Stop ECS task | `ecs-running-tasks` alarm fires |

---

## Log Retention & Audit

| Log Group | Retention | Purpose |
|-----------|-----------|---------|
| `/ecs/antigravity-production/engine` | 90 days | Engine operations |
| `/aws/elasticache/.../redis` | 90 days | Redis slow log |
| Aurora PostgreSQL logs | CloudWatch export | Query audit |
| ALB access logs | S3 30d → IA → Glacier | Request audit |
| S3 audit bucket | 30–90 days | Backup verification |

---

## Sign-Off

**Observability Readiness:** 62/100 (engine metrics live, AWS stack not deployed)
