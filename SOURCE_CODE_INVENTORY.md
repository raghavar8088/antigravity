# Source Code Inventory

Generated from repository discovery on 2026-06-02.

## Repository Identity

- Root: `c:\Trading apllication`
- Git repo: yes
- Branch: `main`
- HEAD: `0baaef180177987d3dc39e7c07bd0384bef960b7`
- Recent commits:
  - `0baaef1 feat(infra): add ha dr and security hardening`
  - `50ef688 feat(observability): add institutional monitoring stack`
  - `f3f2766 feat(trading): integrate institutional go-live hardening`
  - `75053cd feat(alpha): add phase 11 microstructure strategies`
  - `c8181d9 feat(trading): add institutional terminal and engine modules`
- Tracked file count: 10,770
- Git storage: 64.64 MiB packed, 24.73 MiB loose

## Directory Structure

| Path | Type | Role | Clone Requirement |
| --- | --- | --- | --- |
| `client/` | TypeScript / Next.js | UI, API routes, paper desk, Mongo persistence, replay/research tools | Clone with `package-lock.json`; run install from this directory |
| `engine/` | Go | Trading engine, strategy registry, OMS v3, ledger, risk, reconciliation, HA, backup, observability | Clone with `go.mod`, `go.sum`, and untracked certification files if needed |
| `infrastructure/` | SQL, Docker, Kubernetes, Python, monitoring | Timescale/Redis/Prometheus local stack, K8s manifests, DB schema, AI service | Clone for infra parity |
| `grafana/` | Docker Compose, dashboards, provisioning | Prometheus, AlertManager, Grafana, Loki, Promtail stack | Clone dashboards/config plus volumes |
| `bridge/` | Node.js | ChatGPT/Claude bridge automation | Clone `package-lock.json` and JSONL state |
| `brain/` | Python scratch | Scratch encoding utilities | Include for bit-for-bit source parity |
| `Trading appication02/` | Legacy React app | Older app variant | Include if full historical app clone is required |
| `.github/workflows/` | CI/CD | Engine deploy, keep-alive, replay gate | Fork or rewire secrets and URLs |
| `data/` | Runtime data | Local audit and generated state root | Clone as state, not just source |
| `output/` | Runtime outputs | Autonomous bot outputs | Clone for forensic parity |
| `RESEARCH DOCS/` | Untracked PDFs | Research documents | Clone manually because currently untracked |

## Language And Service Inventory

### Go

- Module: `engine/go.mod`
- Module name: `antigravity-engine`
- Go version: `1.25.0`
- Direct dependencies: `github.com/gorilla/websocket`, `github.com/jackc/pgx/v5`, `github.com/prometheus/client_golang`, `modernc.org/sqlite`
- Entrypoints:
  - `engine/cmd/antigravity/main.go`
  - `engine/cmd/backtest/main.go`
  - `engine/cmd/perfbench/main.go`
  - `engine/cmd/seed_db/main.go`
  - `engine/cmd/antigravity/nifty_market.go`
- Strategy code: `engine/internal/strategy/` with 26 Go files; `BuildCuratedScalpers()` declares the 600-strategy live pack.
- Critical state packages: `engine/internal/ledger/`, `engine/internal/omsv3/`, `engine/internal/risk*`, `engine/internal/reconciliation*`, `engine/internal/persistence/`, `engine/internal/ha/`, `engine/backup/`.

### TypeScript / JavaScript

- Primary manifest: `client/package.json`
- Node version: `>=20.0.0`
- Framework: Next `16.2.1`, React `19.2.4`, TypeScript `^5`, Vitest `^3.2.4`
- Runtime dependencies include MongoDB `^7.2.0`, pg `^8.20.0`, Supabase, zod, undici, lightweight-charts.
- API routes observed: 97 route files under `client/src/app/api`.
- Worker: `client/scripts/btc-ft-paper-worker.ts`
- Replay/research scripts:
  - `client/scripts/replay-paper-desk.ts`
  - `client/scripts/replay-fetch.ts`
  - `client/scripts/replay-compare.ts`
  - `client/scripts/replay-walkforward.ts`
  - `client/scripts/rank-btc-ft-strategies.ts`
  - `client/scripts/research-rank-btc-ft.ts`
  - `client/scripts/calibrate-signal-weights.ts`
- Bridge manifest: `bridge/package.json`, Node automation with `axios` and `puppeteer-core`.
- Legacy manifest: `Trading appication02/package.json`.

### Python

- Root requirements: `requirements.txt`
  - FastAPI, uvicorn, anthropic, numpy, pandas, scipy, ta, yfinance, ccxt, python-binance, pydantic, requests, aiohttp, PyYAML.
- AI service requirements: `infrastructure/ai/requirements.txt`
  - grpcio, grpcio-tools, protobuf, FastAPI, torch, pandas, numpy.
- Entrypoints:
  - `main.py`
  - `infrastructure/ai/model_server.py`
  - `infrastructure/ai/strategy_service/api.py`
  - `autonomous_ai_bot.py`
  - `autonomous_repo_bot.py`
  - `scripts/fetch_history.py`

## Infrastructure Code

| Asset | Location | Purpose |
| --- | --- | --- |
| Production engine Dockerfile | `engine/Dockerfile` | Build Go engine container |
| Client Dockerfile | `client/Dockerfile` | Build Next.js client container |
| Production backend Compose | `docker-compose.prod.yml` | Run engine with mounted `./data` |
| Local infra Compose | `infrastructure/docker-compose.yml` | TimescaleDB, Redis, Prometheus, Grafana |
| Observability Compose | `grafana/docker-compose.yml` | Prometheus, AlertManager, Grafana, Loki, Promtail |
| Kubernetes engine | `infrastructure/kubernetes/trading-engine.yaml` | HA engine StatefulSet, service, config, secrets, PVC |
| Kubernetes Timescale HA | `infrastructure/kubernetes/timescale-ha.yaml` | Primary/replica TimescaleDB |
| Kubernetes Redis HA | `infrastructure/kubernetes/redis-ha.yaml` | Primary/replica/sentinel Redis |
| Database SQL | `infrastructure/database/*.sql`, `client/supabase/migrations/*.sql` | Postgres/Timescale/Supabase schema |
| CI/CD | `.github/workflows/*.yml` | Deploy, keep-alive, replay regression |
| VM deploy script | `scripts/deploy.sh` | Tar/scp/ssh deployment using `docker-compose.prod.yml` |

No Terraform files were observed by glob search.

## Build Commands

| Component | Command |
| --- | --- |
| Client install | `cd client && npm install --legacy-peer-deps --ignore-scripts` |
| Client build | `cd client && npm run build` |
| Client tests | `cd client && npm run test` |
| Go build | `cd engine && go build ./...` |
| Go tests | `cd engine && go test ./...` |
| Python AI service | `cd infrastructure/ai && pip install -r requirements.txt` |
| Local infra | `cd infrastructure && docker-compose up -d` |
| Observability | `cd grafana && docker-compose up -d` |

## Version Dependencies To Preserve

- `client/package-lock.json`
- `bridge/package-lock.json`
- `Trading appication02/package-lock.json`
- `engine/go.sum`
- `requirements.txt`
- `infrastructure/ai/requirements.txt`
- Docker image tags in compose/Kubernetes:
  - `timescale/timescaledb:latest-pg15`
  - `redis:7-alpine`
  - `prom/prometheus:v2.51.0`
  - `prom/alertmanager:v0.27.0`
  - `grafana/grafana:10.4.0`
  - `grafana/loki:3.0.0`
  - `grafana/promtail:3.0.0`

## Source Clone Requirements

1. Use `git clone --mirror` or `git bundle create` to preserve complete history, refs, tags, and branches.
2. Copy current untracked state separately or add it to a migration artifact:
   - `RESEARCH DOCS/`
   - `data/`
   - `engine/c`
   - `engine/cover_v3.out`
   - `engine/internal/certification/`
3. Do not copy `.env` values into docs or tickets. Export environment names and migrate values through the target secret manager.
4. After clone, verify:
   - `git rev-parse HEAD`
   - `git fsck --full`
   - `git status --short`
   - dependency lockfile checksums
