# BTC-PILOT SOVEREIGN — Grafana Observability Stack

## Prerequisites
- Docker + Docker Compose
- BTC-PILOT engine running on port 8080

## Setup

### 1. Set environment variables
```bash
export GRAFANA_PASSWORD=your-secure-password
export ENGINE_HOST=your-lightsail-ip   # e.g., 13.233.x.x
```

### 2. Start the stack
```bash
cd grafana
docker-compose -f docker-compose.grafana.yml up -d
```

### 3. Access
- **Grafana**: http://localhost:3001 — admin / `${GRAFANA_PASSWORD}`
- **Prometheus**: http://localhost:9090
- **Jaeger**: http://localhost:16686

Dashboards auto-provision from `dashboards/`. Open **BTC-PILOT SOVEREIGN** folder.

### 4. Verify
- All panels should show data within 30 seconds of engine startup.
- Check **Explore → Prometheus** to confirm `up{job="btc-engine"}` == 1.
- After one 15-min evaluation cycle: check Jaeger for `btc.cycle` traces.

## Alert Rules
Alert rules in `dashboards/btc_pilot_alerts.json`. Import via:
1. Grafana UI → Alerting → Import
2. Or via API: `curl -X POST http://admin:${GRAFANA_PASSWORD}@localhost:3001/api/v1/provisioning/alert-rules -d @dashboards/btc_pilot_alerts.json`

## Triggered Alerts
| Alert | Condition | Severity |
|-------|-----------|----------|
| ENGINE_DOWN | Engine absent for 2m | Critical |
| KILL_SWITCH_ACTIVATED | Kill switch == 1 | Critical |
| DATA_QUALITY_CRITICAL | Quality score < 60 for 5m | Warning |
| HIGH_VOLATILITY_REGIME | Regime = HIGH_VOL for 10m | Warning |
| AI_FALLBACK_HIGH | Fallback rate > 50% | Warning |
| CYCLE_OVERLAP_SUSTAINED | Overlaps > 2/30m | Warning |
| DAILY_LOSS_LIMIT_APPROACHING | Approaching 4% drawdown | Warning |

## Metric Reference
All engine metrics use the `trading_` prefix. Key metrics:
- `trading_execution_kill_switch_active` — 0/1
- `trading_dataquality_score` — 0–100
- `trading_regime_active{regime="..."}` — 0/1 per regime
- `trading_derivatives_funding_rate` — raw funding rate
- `trading_aiscoring_cache_lookups_total{result="hit|miss|stale"}`
- `trading_cycleguard_blocks_total`
- `trading_kelly_size_ratio`
- `trading_reconciliation_restart_duration_ms`
