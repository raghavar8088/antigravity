# Onboarding Guide

## Local Setup

Frontend:

```bash
cd client
npm install
npm run dev
```

Engine:

```bash
cd engine
go build ./...
go run ./cmd/antigravity
```

## Required Context

- Frontend uses Next.js in `client/`.
- Engine uses Go in `engine/`.
- Vercel deploys the frontend.
- The Go engine runs separately and is reached by `INTERNAL_API_URL`.

## Environment Essentials

- `INTERNAL_API_URL`: Next.js to Go engine.
- `ENGINE_ADMIN_SECRET`: admin/kill-switch operations.
- `MONGODB_URI`: paper/mock trading and strategy state.
- `DATABASE_URL`: Postgres snapshots/history where enabled.
- `DELTA_API_KEY`, `DELTA_API_SECRET`: Delta live/testnet integrations.
- `BINANCE_API_KEY`, `BINANCE_API_SECRET`: Binance paths where configured.

## Debugging BTC Futures

Follow this order:

1. Signal generation: `client/src/lib/trading/futuresSignals.ts`
2. Gate/policy: `client/src/lib/trading/futuresDeskPolicy.ts`
3. Runtime/worker: `client/src/lib/trading/futuresDeskRuntime.ts`
4. PnL/math: `client/src/lib/trading/futuresPaperMath.ts`
5. Persistence: `client/src/lib/portfolio/*`, `client/src/lib/broker/*`
6. UI: `client/src/components/*`, terminal pages

## Debugging Engine Flow

1. `engine/cmd/antigravity/main.go`
2. `engine/internal/marketdata`
3. `engine/internal/strategy`
4. `engine/internal/risk*`
5. `engine/internal/execution*`
6. `engine/internal/omsv3`
7. `engine/internal/ledger`
8. `engine/internal/reconciliation*`
9. `engine/internal/killswitch`

## AI Assistant Rule

Do not load the whole repository. Start with `docs/` and follow only the relevant module paths. Skip `vendor`, `.venv`, build artifacts, and Graphify cache unless specifically needed.
