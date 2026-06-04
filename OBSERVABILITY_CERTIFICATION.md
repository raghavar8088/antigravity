# Observability Certification

Generated for Phase 16A clone certification on 2026-06-02.

## Certification Verdict

Status: not certified for a true 100% clone.

The repository has a substantial Prometheus/Grafana/Loki/AlertManager scaffold, but active deployment wiring is incomplete or not proven. A true clone requires rewriting scrape targets, cloning or intentionally resetting observability volumes, and proving alert rules evaluate against metrics that are actually exposed by the running engine.

## Prometheus Wiring

Engine metrics endpoint:

- `engine/cmd/antigravity/main.go` registers `/metrics` with `promhttp.Handler()`.
- This exposes the default Prometheus gatherer in the running binary.

Institutional metrics package:

- `engine/internal/observability/metrics.go` defines many `promauto` trading metrics.
- `engine/internal/observability/registry.go` defines `MetricsHandler()` that combines `prometheus.DefaultGatherer` and a custom registry.
- The active engine route uses `promhttp.Handler()`, not `observability.MetricsHandler()`.
- The main engine entry point was not certified as importing `engine/internal/observability`, so package-level collectors in that package may not register in the production binary.

Reconciliation metrics:

- `engine/internal/reconciliationv2/metrics.go` uses a private registry.
- The comments state callers may expose it through `promhttp.HandlerFor(m.Registry, ...)`.
- No production wiring was certified that exposes this private registry through `/metrics`.

Prometheus config:

- Source: `grafana/prometheus/prometheus.yml`
- Global scrape interval: 15 seconds.
- Global evaluation interval: 15 seconds.
- Active rule files:
  - `/etc/prometheus/alert_rules.yml`
  - `/etc/prometheus/db_alerts.yml`
- AlertManager target: `alertmanager:9093`.
- Scrape jobs:
  - `trading_engine` at `host.docker.internal:8080`, `/metrics`, 5 seconds.
  - `trading_engine_render` at `host.docker.internal:10000`, `/metrics`, 10 seconds.
  - `prometheus`
  - `alertmanager`
  - `grafana`
  - `loki`

Certification finding: Prometheus is configured, but it is not certified that all dashboard and alert metrics are actually registered by the running engine.

## Grafana Dashboards

Provisioned stack:

- `grafana/docker-compose.yml`
- `grafana/provisioning/datasources/prometheus.yaml`
- `grafana/provisioning/datasources/loki.yaml`
- `grafana/provisioning/dashboards/all.yaml`

Provisioned dashboards:

- `grafana/dashboards/01_executive.json`
- `grafana/dashboards/02_oms.json`
- `grafana/dashboards/03_risk.json`
- `grafana/dashboards/04_exchange.json`
- `grafana/dashboards/05_reconciliation.json`
- `grafana/dashboards/06_infrastructure.json`
- `grafana/dashboards/07_security.json`

Additional dashboard:

- `infrastructure/database/monitoring/grafana-database-dashboard.json`

Certification gap:

- `grafana/docker-compose.yml` mounts `./dashboards` to `/etc/grafana/dashboards`.
- The database dashboard under `infrastructure/database/monitoring/` is not mounted by the observability compose file.
- Grafana UI state such as users, teams, API keys, preferences, alert state, and UI-edited dashboards lives in the `grafana_data` volume and is not reproducible from dashboard JSON alone.

## Loki Ingestion

Loki config:

- Source: `infrastructure/loki/loki-config.yaml`
- HTTP port: 3100.
- Storage: filesystem under `/loki`.
- Schema: TSDB v13.
- Retention: 720 hours / 30 days.
- Ruler points to `http://alertmanager:9093`.

Promtail config:

- Source: `infrastructure/loki/promtail-config.yaml`
- Positions file: `/tmp/positions.yaml`.
- Push target: `http://loki:3100/loki/api/v1/push`.
- Scrape sources:
  - `/var/log/trading/engine*.log`
  - `/var/log/trading/nextjs*.log`
  - Docker containers with label `com.trading.scrape=true`.

Certification gaps:

- Engine code primarily logs through `log.Printf` and stdout paths, not a certified `/var/log/trading/engine*.log` file.
- PM2 worker logs are configured under `client/logs/btc-ft-worker-*.log`, not `/var/log/trading/nextjs*.log`.
- Docker log scraping is configured only for containers with `com.trading.scrape=true`.
- `LOGGING_STANDARD.md` expects JSON logs, but the engine still emits many plain-text log lines.

## AlertManager Routing

AlertManager config:

- Source: `grafana/alertmanager/alertmanager.yml`
- Main route groups by `alertname`, `severity`, and `exchange`.
- Critical alerts route to PagerDuty and Slack.
- High alerts route to Slack.
- Medium alerts route to ops Slack.
- Low alerts route to `silence`.
- Inhibition rules suppress downstream alerts for kill switch, exchange disconnect, and database unavailable conditions.

Secret references:

- `SLACK_WEBHOOK_URL`
- `PAGERDUTY_INTEGRATION_KEY`

Certification gaps:

- AlertManager config references `/etc/alertmanager/templates/*.tmpl`, but no template mount was certified in `grafana/docker-compose.yml`.
- Environment-variable substitution in mounted AlertManager YAML is not proven by the compose file alone.
- A clone must use clone/test notification receivers during dry run to avoid paging the original production responders.

## Alert Rules

Active in Prometheus config:

- `engine/alerts/alert_rules.yml`
- `infrastructure/database/monitoring/prometheus-database-alerts.yml`

Present but not active in Prometheus config:

- `infrastructure/security/prometheus-security-alerts.yml`
- `infrastructure/performance/prometheus-performance-alerts.yml`

Database alert risk:

- DB rules reference Postgres, PgBouncer, or Timescale exporter metrics.
- No postgres exporter, PgBouncer exporter, or Timescale exporter scrape target was certified in `grafana/prometheus/prometheus.yml`.

Trading alert risk:

- Some alert expressions may reference metrics from the institutional observability package or private reconciliation registry.
- Those metrics are not certified as exported by the running engine.

## Observability Volumes

Docker volumes in `grafana/docker-compose.yml`:

- `prometheus_data`
- `alertmanager_data`
- `grafana_data`
- `loki_data`

Clone policy:

- Exact observability clone requires stopping services and copying all volumes plus Promtail positions.
- Rebuildable observability clone can start with empty volumes, but historical metrics/logs, silences, alert state, Grafana users, and UI edits will not match.

## Dry-Run Observability Validation

Not executed in this session:

- Prometheus target readiness.
- Alert rule evaluation.
- Loki ingestion.
- Grafana dashboard rendering.
- AlertManager notification route smoke tests.

Required checks after clone startup:

```bash
curl -fsS http://localhost:9090/-/ready
curl -fsS http://localhost:9090/api/v1/targets
curl -fsS http://localhost:9090/api/v1/rules
curl -fsS http://localhost:9093/-/ready
curl -fsS http://localhost:3001/api/health
curl -fsS http://localhost:3100/ready
```

Required Prometheus checks:

- `up{job="trading_engine"}` is 1 for clone engine only.
- No scrape target points at original Render or original host.
- Alert rules load without parse errors.
- Alert expressions reference metrics present in `/api/v1/label/__name__/values`.

Required Loki checks:

- Clone engine logs are queryable.
- Log labels contain clone environment identifiers.
- No original host/container labels are still being scraped.

## Certification Blockers

1. Replace `promhttp.Handler()` with the intended combined observability handler or explicitly import/register every metric required by dashboards and alerts.
2. Expose reconciliation v2 private registry or merge it into the served gatherer.
3. Mount and activate security and performance alert rules if they are part of the certified stack.
4. Add scrape targets for required database exporters or remove/disable DB alert rules that cannot evaluate.
5. Mount the database Grafana dashboard or document it as out-of-band.
6. Align Promtail log paths with actual engine, Next.js, PM2, and Docker log locations.
7. Make production logs JSON-compatible if log pipeline labels depend on JSON fields.
8. Clone or intentionally reset `prometheus_data`, `grafana_data`, `loki_data`, `alertmanager_data`, and Promtail positions.
9. Rewrite scrape targets and notification receivers for clone isolation.
10. Prove dashboard panels and alert rules work against live clone metrics.

## Certification Decision

Observability is source-present but not runtime-certified. It is cloneable with manual volume copy, scrape-target rewrites, and rule/metric validation, but it is not fully reproducible from source alone.
