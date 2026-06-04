# Infrastructure Reconstruction Guide

Generated for Phase 16A clone certification on 2026-06-02.

## Certification Verdict

Status: cloneable with manual steps, not fully reproducible.

The repository contains enough source/configuration to rebuild local Docker-shaped and partially Kubernetes-shaped environments. It does not contain complete Infrastructure as Code for Render, Vercel, AWS Lightsail, DNS, SSL, managed databases, firewall rules, volumes, secrets, or external broker credentials.

## External Infrastructure Dependencies

Frontend and serverless:

- Vercel project for the Next.js client.
- Root `vercel.json` defines build settings and two cron jobs.
- `client/vercel.json` is empty.
- Required Vercel env includes `INTERNAL_API_URL`, `ENGINE_URL`, Mongo/auth secrets, cron secret, broker keys where used by routes, and `NEXT_PUBLIC_*` policy flags.

Engine hosting:

- Render references exist through hard-coded `https://antigravity-x7he.onrender.com` in GitHub keepalive, bridge code, and agent config.
- No `render.yaml` was found.
- AWS Lightsail/VM deployment exists through `.github/workflows/deploy.yml`.
- `docker-compose.prod.yml` runs the engine on host port 80 mapped to container port 8080 and mounts `./data:/app/data`.

Containers and orchestration:

- `engine/Dockerfile`
- `client/Dockerfile`
- `docker-compose.prod.yml`
- `infrastructure/docker-compose.yml`
- `grafana/docker-compose.yml`
- `infrastructure/kubernetes/trading-engine.yaml`
- `infrastructure/kubernetes/timescale-ha.yaml`
- `infrastructure/kubernetes/redis-ha.yaml`

Managed data services:

- MongoDB Atlas via `MONGODB_URI`, `MONGODB_DB`, or `MONGODB_DB_NAME`.
- PostgreSQL/Timescale via `DATABASE_URL`.
- Redis via `REDIS_URL`.
- SQLite via `SQLITE_PATH`.
- Local filesystem via `LOCAL_DATA_DIR`, `ENGINE_DATA_DIR`, `data/`, `.engine-data`, and runtime host mounts.

Broker and market data APIs:

- Delta Exchange.
- Binance REST/WS/FAPI.
- Coinbase public websocket.
- AngelOne.
- NSE.
- Yahoo Finance.
- Optional Bybit/Hyperliquid funding collectors in research/performance surfaces.

Observability:

- Prometheus.
- Grafana.
- Loki.
- Promtail.
- AlertManager.
- Slack webhook.
- PagerDuty integration key.

Secrets and policy:

- HashiCorp Vault support exists in `engine/internal/security/vault/*` and `VAULT_MIGRATION_GUIDE.md`.
- Runtime still supports environment-variable fallback.

## Render Reconstruction

Render is referenced but not declared.

Required manual reconstruction if Render remains a target:

1. Create Render service manually.
2. Set repo, branch, build command, and start command.
3. Set `PORT=10000` if using Render convention.
4. Configure `/health` health check.
5. Add all engine secrets and database URLs.
6. Add persistent disk if `ENGINE_DATA_DIR` or SQLite state is used.
7. Rewrite keepalive and agent/bridge references to the clone Render service.
8. Prove clone service does not point at original Mongo, Postgres, Redis, brokers, or observability targets.

Blocker: no `render.yaml` or declarative Render service export exists.

## AWS Lightsail Reconstruction

Observed deployment:

- `.github/workflows/deploy.yml` builds and pushes `ghcr.io/${{ github.repository_owner }}/antigravity-engine:latest`.
- It SSHes to `${{ secrets.LIGHTSAIL_HOST }}` as `ubuntu`.
- It runs container `antigravity_engine`.
- It maps `80:8080`.
- It uses `/home/ubuntu/antigravity/.env`.
- It mounts `/home/ubuntu/antigravity/data:/app/data`.

Required manual reconstruction:

1. Create Lightsail instance or compatible VM.
2. Attach static IP.
3. Install Docker.
4. Create `/home/ubuntu/antigravity`.
5. Restore `.env` with clone-specific secrets.
6. Restore `/home/ubuntu/antigravity/data` including SQLite database, WAL/SHM files, engine snapshots, logs, and backups if used.
7. Recreate GitHub secrets `LIGHTSAIL_HOST` and `LIGHTSAIL_SSH_KEY` for the clone.
8. Allow inbound HTTP 80 or HTTPS 443 according to chosen endpoint.
9. Restrict SSH 22 to operator/GitHub deployment IPs as appropriate.
10. Ensure outbound 443 to exchanges, managed databases, Redis, MongoDB Atlas, and observability endpoints.

Blocker: no Lightsail instance size, static IP allocation, firewall, disk, snapshot, or SSH key IaC was found.

## Vercel Reconstruction

Required manual reconstruction:

1. Create clone Vercel project rooted at the correct Next.js client directory or root config according to deployment setup.
2. Set build command from `vercel.json`: `npm run build`.
3. Set install command from `vercel.json`: `npm install --legacy-peer-deps --ignore-scripts`.
4. Configure the two active cron jobs:
   - `/api/cron/rank-strategies` at `0 0 * * *`
   - `/api/cron/policy-snapshot` at `5 0 * * *`
5. Set `CRON_SECRET`.
6. Set clone-specific `INTERNAL_API_URL` or `ENGINE_URL`.
7. Set Mongo/auth/database env vars.
8. Set all `NEXT_PUBLIC_*` policy and feature flags.
9. Attach clone domains only after validation.
10. Keep cron disabled or pointed at clone-only data during dry run.

Blocker: no declarative Vercel project, domain, environment, or secret export was found.

## DNS, Domains, and SSL

Observed:

- `engine/internal/security/policies.go` defaults allowed origins to `https://antigravity.vercel.app` and `http://localhost:3000`.
- `nginx/nginx.conf` is HTTP-only.
- Route 53 is mentioned in disaster recovery documentation, but no hosted-zone or DNS change-batch IaC was found.
- Vercel can manage frontend SSL when domains are attached.
- Render provides HTTPS for Render domain.
- Lightsail compose exposes HTTP port 80; custom HTTPS requires a reverse proxy, certificate manager, or load balancer not declared in source.

Required manual reconstruction:

1. Inventory original domains and subdomains.
2. Decide clone domain names.
3. Attach frontend domain to Vercel clone.
4. Provision engine HTTPS if clone engine is public.
5. Update `ALLOWED_ORIGINS`.
6. Update OAuth/auth redirect settings if applicable.
7. Update broker/IP allowlists that depend on domain or static IP.
8. Verify no DNS record points clone traffic at original services.

Blocker: no DNS/SSL IaC exists.

## Firewall and Network Reconstruction

Required rules:

- Engine public endpoint: allow only intended HTTP/HTTPS.
- Engine admin endpoints: restrict with `ENGINE_ADMIN_CIDR` and `ENGINE_ADMIN_SECRET`.
- SSH: restrict to operator and deployment systems.
- MongoDB Atlas: allow clone runtime IPs or private endpoint.
- Postgres/Timescale/Neon/Supabase: restrict to clone runtime IPs where supported.
- Redis: private access only; enable TLS/auth if managed.
- Broker APIs: outbound 443 from engine/client workers.
- AngelOne/NSE/MCX paths: preserve stable whitelisted IP when required.
- Observability ports: keep Prometheus, Grafana, Loki, AlertManager private unless explicitly protected.

Blocker: no firewall/security group IaC was found.

## Volumes and Persistent State

Must preserve for exact clone:

- Git repository including full history and untracked Phase 16 files if accepted as artifacts.
- `data/` and runtime `/app/data`.
- SQLite `engine.db` plus WAL/SHM files.
- `.engine-data` if used.
- MongoDB dump and index metadata.
- Postgres/Timescale dump or managed snapshot with hypertable metadata.
- Redis RDB/AOF if exact cache clone is required.
- `prometheus_data`.
- `grafana_data`.
- `loki_data`.
- `alertmanager_data`.
- Promtail positions if preserving ingestion state.
- PM2 worker logs under `client/logs`.
- Bridge logs such as `bridge/bridge-decisions.jsonl`.
- Local audit logs under `data/audit`.
- Research PDFs and local fixtures.

Can be rebuilt if declared non-authoritative:

- `node_modules`.
- Go build cache.
- Python virtual environments.
- Prometheus/Loki history if historical observability equality is not required.
- Redis cache if declared rebuildable.

## Required Secrets

Critical secrets with no safe source default:

- `MONGODB_URI`
- `DATABASE_URL`
- `REDIS_URL`
- `AUTH_JWT_SECRET`
- `CRON_SECRET`
- `ENGINE_ADMIN_SECRET`
- `INTERNAL_API_SECRET`
- `SERVICE_SECRETS`
- `BINANCE_API_KEY`
- `BINANCE_API_SECRET`
- `DELTA_API_KEY`
- `DELTA_API_SECRET`
- `ANGELONE_API_KEY`
- `ANGELONE_CLIENT_CODE`
- `ANGELONE_PIN`
- `ANGELONE_TOTP_SECRET` or `ANGELONE_TOTP_CODE`
- `LIGHTSAIL_HOST`
- `LIGHTSAIL_SSH_KEY`
- `LIGHTSAIL_ENGINE_URL`
- `GRAFANA_ADMIN_PASSWORD`
- `SLACK_WEBHOOK_URL`
- `PAGERDUTY_INTEGRATION_KEY`

Defaults that must be reviewed:

- `MONGODB_DB` or `MONGODB_DB_NAME` defaults to `loop_trades`.
- Engine port defaults to 8080.
- SQLite defaults to `./data/engine.db`.
- `ENGINE_DATA_DIR` defaults to `.engine-data`.
- `INTERNAL_API_URL` and `ENGINE_URL` can default to localhost in some routes.
- Bridge code defaults `ENGINE_URL` to the original Render cloud endpoint.
- Security policy default allowed origin includes original Vercel production origin.

## Clone Dry Run

Executed source-level test:

```bash
go test -mod=mod ./internal/strategy ./internal/ledger ./internal/omsv3 ./internal/reconciliationv2
```

Result: passed.

Default source-level test without `-mod=mod` failed because `engine/vendor` is inconsistent with `go.mod`. This is a reproducibility blocker for default Go builds.

Validation status:

- Replay validation: source-level tests passed with `-mod=mod`; live replay against restored source/clone databases not executed.
- Database consistency: not executable without live source and clone DB credentials.
- Strategy counts: current committed strategy test expects and passes 589 strategies; source comments still claim 600, and expansion pack comments target a 600-strategy pack. This documentation/test drift must be resolved before final clone signoff.
- Order counts: not executable without live Postgres/Mongo exports.
- Position counts: not executable without live Postgres/Mongo exports.
- Ledger counts: not executable without live `ledger_events`, snapshots, and aggregate sequence manifests.

Required dry-run sequence:

1. Freeze all writers.
2. Disable or isolate Vercel cron, GitHub keepalive, PM2 worker, browser tabs, engine services, and broker bridges.
3. Capture source code and untracked artifact manifest.
4. Export Postgres/Timescale.
5. Export MongoDB with indexes.
6. Copy SQLite/WAL/SHM and runtime filesystem data.
7. Dump or explicitly reset Redis.
8. Copy or reset observability volumes according to clone policy.
9. Restore into clone infrastructure with writers disabled.
10. Compare schema, row, collection, index, and count manifests.
11. Run replay validation.
12. Compare strategy, order, position, trade, PnL, and ledger counts.
13. Start clone in paper/testnet mode with kill switch active.
14. Verify observability and alerts against clone-only targets.
15. Archive validation bundle.

## Final Clone Readiness Score

Classification: CLONEABLE WITH MANUAL STEPS.

Score: 63/100.

Rationale:

- Source inventory and runbooks are strong.
- Focused source replay tests pass when vendor drift is bypassed.
- Database and ledger schemas are fragmented.
- Runtime-created tables and implicit Mongo collections are not a committed canonical manifest.
- Scheduler state is not centralized and includes original-target jobs.
- Observability config exists but active metric/rule/log wiring is not fully certified.
- Infrastructure is not declared through complete IaC.
- Live state counts, hashes, exports, and restored clone validation were not available.

This is not FULLY REPRODUCIBLE because source alone cannot recreate platform state, managed-service settings, secrets, DNS, SSL, firewall rules, volumes, live databases, runtime-created tables, observability history, or exact scheduler state.

It is not NOT CLONEABLE because a careful manual clone is feasible if all blockers below are handled.

## Blockers Preventing a True 100% Clone

1. No complete IaC for Vercel, Render, Lightsail/AWS, DNS, SSL, firewalls, managed databases, Redis, or observability volumes.
2. No live platform export for Render/Vercel/Lightsail settings, GitHub secrets, domain records, firewall rules, or managed DB settings.
3. Ledger schemas are fragmented across `ledger_events`, `trading.event_store`, `audit.event_store`, and HA replica tables.
4. Event versioning is inconsistent: Go current schema is v2, while SQL defaults remain v1 and legacy audit uses `event_version`.
5. Runtime-created Postgres tables are not part of a canonical migration manifest.
6. Mongo collections and indexes are mostly implicit and must be exported from live Atlas.
7. SQLite has no migration/version table and requires file-level copy plus integrity checks.
8. Redis exact clone policy is unresolved.
9. Scheduler truth is split across Vercel cron, GitHub Actions, PM2, browser tabs, Go tickers, and package-defined schedulers.
10. Original endpoint references remain, including Render URL defaults and GitHub keepalive target.
11. Cron routes can be unauthenticated if `CRON_SECRET` is absent.
12. Duplicate writer fencing is not certified across PM2 worker, browser, cron fallback, and engine loops.
13. Observability metrics, dashboards, alert rules, exporters, log paths, and AlertManager templates are not fully wired/proven.
14. Historical observability data requires volume copy or must be declared intentionally non-identical.
15. Go vendor directory is inconsistent with `go.mod`; default Go test/build is not reproducible.
16. Strategy count documentation disagrees with the current passing test expectation.
17. No live row/count/hash validation bundle exists for source or clone.
18. No proof exists that clone secrets, broker credentials, account keys, and database URLs are isolated from original production.

## Reconstruction Decision

A true 100% clone is possible only as a controlled manual reconstruction with live exports, frozen writers, clone-specific secrets, schema mapping, and validation manifests. It is not currently possible as a fully automated, source-only reproduction.
