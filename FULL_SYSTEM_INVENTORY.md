# Full System Inventory

Generated from repository discovery on 2026-06-02. This report inventories observed applications, services, workers, schedules, and state-bearing components. Secret values were not inspected or copied.

## Discovery Boundary

- Repository root: `c:\Trading apllication`
- Git branch: `main`
- HEAD: `0baaef180177987d3dc39e7c07bd0384bef960b7`
- Tracked files: 10,770
- Git object store: 64.64 MiB packed, 24.73 MiB loose
- Current untracked state observed: `RESEARCH DOCS/`, `data/`, `engine/c`, `engine/cover_v3.out`, `engine/internal/certification/`
- Live external databases were not queried in this pass. Reports include exact count/export commands required to complete clone certification.

## Applications And Services

| Component | Location | Purpose | Dependencies | Runtime State | Clone Requirements |
| --- | --- | --- | --- | --- | --- |
| Next.js trading dashboard and API | `client/` | React 19 / Next 16 UI, API routes, auth, paper desk, mock trading, storage APIs, engine proxy | Node >=20, `client/package-lock.json`, MongoDB, Postgres/Supabase, Delta, AngelOne, Go engine | Mongo documents, browser `localStorage`, filesystem `data/`, API route caches, cookies | Clone git history, install from lockfile, migrate env names, export/import Mongo/Postgres/local files/browser state if exact operator clone is required |
| Go execution engine | `engine/cmd/antigravity/main.go` | Main trading engine, strategy registry, risk, OMS v3, ledger, reconciliation, execution, market data, health/metrics | Go 1.25, SQLite, pgx/Postgres, Prometheus, exchange APIs | SQLite `engine.db`, event ledger, snapshots, in-memory positions, file snapshots, metrics | Clone source, vendor/dependencies, DBs, mounted `data`, env, health/metrics wiring |
| Go backtest command | `engine/cmd/backtest/main.go` | Offline strategy backtesting | Go modules, strategy/backtest packages | Backtest output/logs, input market data | Clone source and any historical market/research datasets |
| Go perf benchmark | `engine/cmd/perfbench/main.go` | Engine performance benchmarking | Go modules | Benchmark outputs if retained | Clone outputs only if needed for forensic parity |
| Go DB seed command | `engine/cmd/seed_db/main.go` | Database initialization | Postgres/Timescale | Seed side effects in DB | Preserve migrations and rerun only in new clone after restore decision |
| Python AI gRPC stub | `infrastructure/ai/model_server.py` | Stub Python listener for external AI strategy on port 50051 | Python requirements, grpcio, torch | In-memory only in current file; future model state external | Clone code, Python env, any model artifacts if added |
| Python strategy FastAPI service | `infrastructure/ai/strategy_service/` | Strategy evaluation, framework cycle, backtest, journal, Monte Carlo | FastAPI, pandas, torch, config yaml | In-process `ApexScalpFramework`, trade journal, event layer | Clone code/config; persist any future journal/model files explicitly |
| Root Python FastAPI entrypoint | `main.py` | FastAPI entrypoint for the strategy service on port 8000 | Root Python requirements and `infrastructure/ai/strategy_service` | In-memory service state unless journal/model persistence is added | Clone code and launch config |
| TS browser automation bridge | `bridge/` | ChatGPT/Claude web handoff automation | Node, axios, puppeteer-core | `bridge-decisions.jsonl`, `autonomous-handoff.log.jsonl`, browser/session artifacts | Clone JSONL logs and browser profile/session only if workflow continuity is required |
| Legacy React app | `Trading appication02/` | Older standalone app | Node package lock | Browser/local app state | Clone if historical UI parity is required |
| Autonomous repo bot | `autonomous_repo_bot.py`, `autonomous_ai_bot.py`, `output/autonomous_bot/` | Repo audit/automation workflow | Python, Anthropic key | `output/autonomous_bot/*` JSON/results | Clone output directory and bot configs |

## API Surface

Observed `client/src/app/api` route files: 97. Main domains:

- Auth/session: `auth/session`, `auth/signin`, `auth/signout`
- Engine proxy: `engine/[...path]`
- Paper desk: `paper-state`, `paper-trades`, `paper-oms`, `paper-replay`, `strategy-signal-trace`, `desk-entry-funnel`, `desk-worker-events`
- Mock trading/research: `mock-trading/*`, `research-edge-report`, `replay-walkforward`
- Market data: `btc/*`, `crypto/*`, `nifty/*`, `nifty-bees/*`, `mcx/*`, `stocks/*`
- Brokers: `delta/*`, `angelone/*`
- Storage: `storage/health`, `storage/backup`, `storage/restore`
- Observability/health: `health/storage`, `health/desk-worker`
- Cron: `cron/rank-strategies`, `cron/policy-snapshot`, `cron/paper-desk-tick`

## Workers, Schedulers, And Background Processes

| Process | Location | Trigger | State Written | Clone Requirements |
| --- | --- | --- | --- | --- |
| BTC futures paper worker | `client/scripts/btc-ft-paper-worker.ts` | PM2 / long-running loop | Mongo `paper_state`, `paper_trades`, `desk_worker_events`, `verification_track_events`, `paper_oms_orders`; process memory for cooldowns/peak equity | Stop original before live clone or use new account/env; migrate Mongo state and worker env |
| PM2 worker config | `client/ecosystem.worker.config.cjs` | `pm2 start` | PM2 process metadata, `client/logs/btc-ft-worker-*.log` when present | Recreate PM2 with independent env and logs |
| Vercel cron rank strategies | `vercel.json`, `client/src/app/api/cron/rank-strategies/route.ts` | `0 0 * * *` | Mongo `policy_snapshots`; writes `client/fixtures/replay/btc_ft_strategy_rankings.json` | Preserve only two Vercel cron limit; isolate `CRON_SECRET` and Mongo target |
| Vercel cron policy snapshot | `vercel.json`, `client/src/app/api/cron/policy-snapshot/route.ts` | `5 0 * * *` | Mongo `policy_snapshots` | Same as above |
| Paper desk fallback cron route | `client/src/app/api/cron/paper-desk-tick/route.ts` | Route exists, but not scheduled in current root `vercel.json` | Mongo paper state/trades if invoked | Decide explicitly whether clone schedules it; respect two-cron Vercel limit |
| GitHub keep-alive | `.github/workflows/keep-alive.yml` | Every 5 minutes | GitHub Actions logs; pings Render health | Update target URL for clone or disable |
| GitHub engine deploy | `.github/workflows/deploy.yml` | Push to `main` under `engine/**` | GHCR image, Lightsail host | Use clone-specific GHCR repo, SSH host, `.env` path |
| GitHub replay gate | `.github/workflows/replay-snapshot.yml` | PR path filters | CI logs | Preserve for source validation |
| Go state saver | `engine/internal/persistence/saver.go` | Periodic engine runtime | SQLite/Postgres engine snapshots | Preserve DB/file target and timing |
| Backup manager | `engine/backup/` | Scheduled/explicit backup flows | Local backup catalog/files, metrics | Clone backup directory, encryption key reference, verification workflow |
| Reconciliation services | `engine/internal/reconciliation*` | Engine runtime and replay tests | Ledger events, repair events, projections | Clone event store and replay from source of truth |
| HA services | `engine/internal/ha/` | Engine runtime | HA heartbeat, replication checkpoint, integrity progress tables | Clone HA tables and isolate node IDs |
| In-process event buses / queues | `engine/internal/events/bus.go`, `client/src/internal/events/index.ts`, `client/src/lib/writeQueue.ts` | In-process event fanout and async client writes | Runtime memory; downstream DB/local writes | Rebuild at startup; do not treat as durable queue |

## Websocket And Streaming Services

| Service | Location | Purpose | State |
| --- | --- | --- | --- |
| Coinbase websocket market data | `engine/internal/marketdata/coinbase.go` | Public BTC price feed | In-memory ticks, downstream ledger/metrics |
| Go websocket dependency | `engine/go.mod` | Engine websocket support | Runtime connections |
| NIFTY stream API | `client/src/app/api/nifty/stream/route.ts` | Client market stream route | Request lifecycle only unless persisted by callers |
| Terminal websocket URL | `client/src/lib/terminal/terminalStore.tsx` | UI terminal connection via `NEXT_PUBLIC_TERMINAL_WS_URL` | Browser/UI state |

## Monitoring Services

| Component | Location | State | Clone Requirements |
| --- | --- | --- | --- |
| Prometheus | `grafana/docker-compose.yml`, `grafana/prometheus/prometheus.yml` | `prometheus_data` volume, 30d/10GB TSDB | Snapshot or remote-read/export TSDB; update scrape targets |
| Grafana | `grafana/dashboards/`, `grafana/provisioning/` | `grafana_data` volume plus provisioned dashboards | Clone dashboards and Grafana DB if users/settings must match |
| Loki | `infrastructure/loki/loki-config.yaml` | `loki_data` volume, 30d retention | Clone chunks/index or accept rebuilt logs only |
| Promtail | `infrastructure/loki/promtail-config.yaml` | Positions file `/tmp/positions.yaml` | Reset positions for clone or copy only for exact log ingest continuity |
| AlertManager | `grafana/alertmanager/alertmanager.yml` | `alertmanager_data` silences/notification log | Export silences and clone routing secrets |

## Clone Risk Notes

- A bit-for-bit independent clone is not certifiable from source alone. It requires live exports of MongoDB, Postgres/Timescale, Redis if persistence is enabled, SQLite/data volumes, Prometheus/Grafana/Loki/AlertManager volumes, browser `localStorage`, PM2 logs, and external platform metadata.
- Clone must use new broker API keys or explicit paper/testnet mode to avoid impacting original markets.
- Cron and worker duplication can create duplicate trading/policy writes. Disable original or use clone-specific account keys, database URIs, and `CRON_SECRET` during rehearsal.
