# Infrastructure Clone Plan

Generated from repository discovery on 2026-06-02.

## Infrastructure Assets

| Asset | Location | Purpose | State / Volume |
| --- | --- | --- | --- |
| Engine production Compose | `docker-compose.prod.yml` | Backend-only engine deployment | `./data:/app/data` |
| Local infra Compose | `infrastructure/docker-compose.yml` | TimescaleDB, Redis, Prometheus, Grafana | `pgdata` volume; Redis no named volume in this file |
| Observability Compose | `grafana/docker-compose.yml` | Prometheus, AlertManager, Grafana, Loki, Promtail | `prometheus_data`, `alertmanager_data`, `grafana_data`, `loki_data` |
| Engine Dockerfile | `engine/Dockerfile` | Go engine image | Image layers |
| Client Dockerfile | `client/Dockerfile` | Next.js client image | Image layers |
| Engine Kubernetes | `infrastructure/kubernetes/trading-engine.yaml` | HA engine StatefulSet | `engine-data` PVC, 20Gi |
| Timescale Kubernetes | `infrastructure/kubernetes/timescale-ha.yaml` | Primary/replica Timescale | `timescale-primary-data`, `timescale-replica-data`, 100Gi each |
| Redis Kubernetes | `infrastructure/kubernetes/redis-ha.yaml` | Redis primary/replica/sentinel | Redis PVC templates, 5Gi |
| Nginx | `nginx/nginx.conf` | Reverse proxy worker config | Config only |
| CI/CD | `.github/workflows/` | Deploy, keepalive, replay gate | GitHub secrets/actions logs |
| VM deploy script | `scripts/deploy.sh` | Oracle/Linux VM deployment by archive/scp/ssh and `docker-compose.prod.yml` | Temporary archive under `/tmp`, remote Docker state |

No Terraform assets were found by glob search.

No declarative Render config such as `render.yaml` was found. Render appears through health keepalive and documentation/code references, so clone deployment must recreate Render settings manually if Render remains a target.

## Network And Ports

| Service | Port | Location |
| --- | --- | --- |
| Next.js client | 3000 | `client/package.json`, README |
| Go engine local | 8080 | Go engine, Docker, K8s |
| Go engine Render | 10000 | Prometheus config |
| Python AI gRPC | 50051 | AI docs/model server |
| Timescale/Postgres | 5432 | Compose/K8s |
| Redis | 6379 | Compose/K8s |
| Redis Sentinel | 26379 | K8s |
| Prometheus | 9090 | Observability compose |
| AlertManager | 9093 | Observability compose |
| Grafana | 3001 host -> 3000 container | Compose |
| Loki | 3100 | Observability compose |
| Promtail | 9080 | Promtail config |

## Docker Clone Steps

1. Create clone host directories:
   - `data/`
   - `grafana/` volumes or Docker named volumes
   - backup directory
2. Restore databases before enabling writers.
3. Build/pull images:

```bash
docker compose -f infrastructure/docker-compose.yml up -d timescaledb redis
docker compose -f grafana/docker-compose.yml up -d
docker compose -f docker-compose.prod.yml up -d --build
```

4. Replace original endpoints in:
   - `INTERNAL_API_URL`
   - Prometheus scrape targets
   - GitHub keep-alive target
   - Alert routing secrets
   - any VM deploy target in `scripts/deploy.sh` usage
5. Confirm engine health and metrics.

## Kubernetes Clone Steps

1. Create clone namespace, for example `trading-clone`.
2. Create clone secrets:
   - `engine-secrets`
   - `timescale-secrets`
3. Provision storage classes and PVCs:
   - Engine: 20Gi fast SSD
   - Timescale primary: 100Gi
   - Timescale replica: 100Gi
   - Redis primary/replica: 5Gi each
4. Apply manifests after editing namespace/storage class/image:

```bash
kubectl apply -f infrastructure/kubernetes/timescale-ha.yaml
kubectl apply -f infrastructure/kubernetes/redis-ha.yaml
kubectl apply -f infrastructure/kubernetes/trading-engine.yaml
```

5. Run DB migrations only if restoring into an empty database. For cloned databases, restore first and run migrations only after schema diff review.
6. Keep engine replicas at zero or kill switch active until validation is complete.

## CI/CD Clone Steps

1. Clone repository with full history.
2. Create new GitHub repository or clone-specific environment.
3. Recreate secrets:
   - `LIGHTSAIL_HOST`
   - `LIGHTSAIL_SSH_KEY`
   - default `GITHUB_TOKEN` is provided by GitHub.
4. Change deploy target from original Lightsail host to clone host.
5. Change keep-alive URL from original Render endpoint to clone endpoint or disable.
6. Keep replay-snapshot gate unchanged.

## Volumes To Preserve

| Volume / Path | Why |
| --- | --- |
| `./data` / `/app/data` | SQLite, local audit, engine snapshots |
| `pgdata` | Local Timescale/Postgres data when not using managed DB |
| `prometheus_data` | Metrics TSDB |
| `alertmanager_data` | Silences and notification state |
| `grafana_data` | Grafana users, settings, dashboard edits not provisioned |
| `loki_data` | Log chunks and index |
| Redis `/data` | RDB/AOF if exact cache clone is required |
| Runtime `/var/log/trading` | Logs collected by Promtail |

## Isolation Controls

- Use clone-specific database URIs.
- Use clone-specific account keys.
- Use clone-specific broker credentials, preferably testnet.
- Disable or rewrite all cron schedules until validation.
- Ensure `ALLOWED_ORIGINS`, admin CIDR, and service secrets point to clone network.
- Confirm no clone scrape target points at original engine.

## Infrastructure Validation

```bash
docker ps
docker volume ls
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
curl -fsS http://localhost:8080/metrics
curl -fsS http://localhost:9090/-/ready
curl -fsS http://localhost:3100/ready
```

Kubernetes:

```bash
kubectl -n trading-clone get pods,pvc,svc
kubectl -n trading-clone rollout status statefulset/trading-engine
kubectl -n trading-clone logs statefulset/trading-engine --tail=100
```

## Readiness Status

Infrastructure is planned but not clone-ready until:

- Managed/external DB exports are available.
- Clone secrets are created.
- Volumes are copied or intentionally rebuilt.
- Original URLs and broker endpoints are replaced.
- Writers are frozen during state export.
