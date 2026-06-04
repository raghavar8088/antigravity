# Observability Clone Plan

Generated from repository discovery on 2026-06-02.

## Observability Inventory

| Component | Location | Purpose | State |
| --- | --- | --- | --- |
| Prometheus | `grafana/docker-compose.yml`, `grafana/prometheus/prometheus.yml` | Metrics scrape, rules, TSDB | `prometheus_data` volume |
| AlertManager | `grafana/docker-compose.yml`, `grafana/alertmanager/alertmanager.yml` | Alert routing, inhibition, silences | `alertmanager_data` volume |
| Grafana | `grafana/docker-compose.yml`, `grafana/dashboards/`, `grafana/provisioning/` | Dashboards, datasources, alert UI | `grafana_data` volume plus provisioned files |
| Loki | `grafana/docker-compose.yml`, `infrastructure/loki/loki-config.yaml` | Log storage/query | `loki_data` volume |
| Promtail | `grafana/docker-compose.yml`, `infrastructure/loki/promtail-config.yaml` | Log shipping | positions file `/tmp/positions.yaml` |
| Engine alert rules | `engine/alerts/alert_rules.yml` | Trading/risk/reconciliation alerts | Config |
| DB alert rules | `infrastructure/database/monitoring/prometheus-database-alerts.yml` | DB/Timescale alerts | Config |
| Security alert rules | `infrastructure/security/prometheus-security-alerts.yml` | Security alerts | Config |
| Performance alert rules | `infrastructure/performance/prometheus-performance-alerts.yml` | Performance alerts | Config |
| DB Grafana dashboard | `infrastructure/database/monitoring/grafana-database-dashboard.json` | Database dashboard | Config |

Current compose wiring mounts the engine alert rules and database alert rules into Prometheus. Security and performance alert rule files exist, but are not mounted/referenced by `grafana/docker-compose.yml` and `grafana/prometheus/prometheus.yml`; wire them explicitly if they are required in the clone.

## Dashboards

Provisioned dashboards observed:

- `grafana/dashboards/01_executive.json`
- `grafana/dashboards/02_oms.json`
- `grafana/dashboards/03_risk.json`
- `grafana/dashboards/04_exchange.json`
- `grafana/dashboards/05_reconciliation.json`
- `grafana/dashboards/06_infrastructure.json`
- `grafana/dashboards/07_security.json`
- `infrastructure/database/monitoring/grafana-database-dashboard.json`

Provisioning:

- `grafana/provisioning/dashboards/all.yaml`
- `grafana/provisioning/datasources/prometheus.yaml`
- `grafana/provisioning/datasources/loki.yaml`

## Prometheus Configuration

- Scrape interval: 15s global
- Evaluation interval: 15s
- Rule files:
  - `/etc/prometheus/alert_rules.yml`
  - `/etc/prometheus/db_alerts.yml`
- AlertManager target: `alertmanager:9093`
- Scrape jobs:
  - `trading_engine` at `host.docker.internal:8080`, `/metrics`, 5s
  - `trading_engine_render` at `host.docker.internal:10000`, `/metrics`, 10s
  - `prometheus`
  - `alertmanager`
  - `grafana`
  - `loki`
- TSDB retention:
  - 30 days
  - 10GB

Clone action: replace original scrape targets with clone engine targets before starting Prometheus.

## AlertManager Configuration

Routes:

- `critical`: PagerDuty + Slack
- `high`: Slack
- `medium`: Slack ops
- `low`: silence/log only

Inhibition:

- Kill switch suppresses execution alerts.
- Exchange disconnected suppresses market data stale.
- Database unavailable suppresses reconciliation alerts.

Secret references:

- `SLACK_WEBHOOK_URL`
- `PAGERDUTY_INTEGRATION_KEY`

Clone action: use clone/on-call routing or a non-production test receiver during dry run.

## Loki Configuration

- Filesystem storage under `/loki`
- TSDB schema v13
- Index period: 24h
- Retention: 720h / 30 days
- Reject old samples older than 168h
- Ingestion limits configured
- Ruler points to AlertManager

Clone action: copy `loki_data` for log-identical clone or start empty for new environment.

## Promtail Configuration

Scrape sources:

- Trading engine logs: `/var/log/trading/engine*.log`
- Next.js logs: `/var/log/trading/nextjs*.log`
- Docker containers with label `com.trading.scrape=true`

Pipeline labels:

- `severity`
- `event_type`
- `trace_id`
- `exchange`
- `component`
- `account_id`
- `level`
- `status`
- `container`
- `service`

Clone action: update log paths/labels for clone and decide whether to copy or reset Promtail positions.

## Metrics State Clone

### Exact Clone

1. Stop Prometheus.
2. Copy `prometheus_data` volume.
3. Start clone Prometheus with rewritten scrape targets disabled or pointed to clone.
4. Validate `/api/v1/status/tsdb`.

### Rebuildable Clone

1. Start empty Prometheus.
2. Keep dashboard/rule files.
3. Accept missing historical metrics.
4. Validate new metrics ingestion.

## Grafana State Clone

Provisioned dashboards are in source and should load automatically. Copy `grafana_data` only if you need:

- Users
- Teams
- Starred dashboards
- UI-edited dashboards not in source
- Alert state
- API keys/service accounts
- Preferences

## Observability Validation

```bash
curl -fsS http://localhost:9090/-/ready
curl -fsS http://localhost:9090/api/v1/targets
curl -fsS http://localhost:9093/-/ready
curl -fsS http://localhost:3001/api/health
curl -fsS http://localhost:3100/ready
```

Dashboard checks:

- Executive dashboard loads.
- OMS dashboard has clone metrics only.
- Risk dashboard has clone metrics only.
- Reconciliation dashboard has no original target labels.
- Infrastructure dashboard shows clone services.
- Security dashboard alert rules load.

## Retention Summary

| Component | Retention |
| --- | --- |
| Prometheus | 30d or 10GB |
| Loki | 30d |
| Mongo worker events | 30d TTL |
| Mongo analytics/report TTLs | 30d or 90d depending collection |
| Timescale market ticks | 180d |
| Timescale candles | 5 years |
| Timescale portfolio snapshots | 10 years |
| Timescale AI decisions | 90d |

## Certification Blockers

- Historical Prometheus/Loki data cannot be claimed identical unless volumes are copied and hashes/queries match.
- Grafana users/preferences cannot be claimed identical unless `grafana_data` is copied.
- Alert routing cannot be used as-is in a clone unless the target on-call/webhooks are intentionally shared.
