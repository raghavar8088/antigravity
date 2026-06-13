# Architecture Overview

Purpose: BTC futures/crypto paper trading platform with a Next.js operator console and Go execution engine.

## Protected Modules

Do not delete, move, or refactor without explicit review:

- BTC Futures Trading UI, hooks, policy, math, signals, replay, worker, and tests under `client/src`.
- Go engine trading, execution, risk, OMS, ledger, reconciliation, persistence, market data, Delta BTC integration, and kill switch packages under `engine/`.
- Graphify code and generated knowledge graph folders, including any `graphify-out/` directories.

## High-Level Flow

```text
BTC market data
  -> strategy/signals
  -> policy and risk gates
  -> paper/live execution request
  -> OMS / position / ledger updates
  -> persistence
  -> Next.js APIs
  -> terminal UI
```

## Runtime Boundaries

- `client/`: Next.js 16 + React 19 application, API routes, dashboards, paper trading UI, Mongo/Postgres-backed server utilities.
- `engine/`: Go engine for strategy execution, risk, persistence, reconciliation, observability, and engine HTTP endpoints.
- `data/`: runtime persistence volume for engine state.
- `grafana/`, `nginx/`, `scripts/`: deployment and operational support.
- `docs/`: AI-optimized repository documentation generated for fast context loading.

## Primary Entry Points

- Client dev/build: `client/package.json`.
- Client app shell: `client/src/app/page.tsx`, `client/src/app/terminal/page.tsx`, `client/src/app/mock-trading/page.tsx`.
- BTC futures UI route: `client/src/app/btc-future-trading/page.tsx`.
- Engine: `engine/cmd/antigravity/main.go`.
- Engine support commands: `engine/cmd/backtest/main.go`, `engine/cmd/perfbench/main.go`, `engine/cmd/seed_db/main.go`.

## External Services

- BTC/crypto market data: Coinbase/Binance/Delta paths.
- Broker/API integration: Delta Exchange BTC options/live bridge.
- Persistence: MongoDB, PostgreSQL, SQLite file snapshots.
- Observability: Prometheus metrics, Grafana support, engine health endpoints.

## Deployment

- Frontend: Vercel via `vercel.json` and `client/package.json`.
- Engine: Docker/Lightsail-style deployment via `docker-compose.prod.yml`.
- Runtime env: `.env.example`, `.env.production.example`, Vercel env vars, engine `.env`.
