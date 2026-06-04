# Scheduler Certification

Generated for Phase 16A clone certification on 2026-06-02.

## Certification Verdict

Status: not certified for a true 100% clone.

The repository contains multiple active schedulers and several defined schedulers that are not proven to be wired into the active runtime. Clone correctness depends on stopping duplicate writers, rewriting original endpoints, and establishing a canonical scheduler manifest before enabling the clone.

## Active Cron Jobs

Vercel cron configuration:

- Source: `vercel.json`
- Active jobs:
  - `/api/cron/rank-strategies` at `0 0 * * *`
  - `/api/cron/policy-snapshot` at `5 0 * * *`
- `client/vercel.json` is empty.

GitHub Actions scheduled job:

- Source: `.github/workflows/keep-alive.yml`
- Schedule: `*/5 * * * *`
- Target: `https://antigravity-x7he.onrender.com/health`
- Clone risk: hard-coded original Render endpoint.

Other workflow schedules:

- `.github/workflows/deploy.yml` deploys engine on push to `main` under `engine/**`.
- `.github/workflows/replay-snapshot.yml` is PR-triggered, not scheduled.

## Cron Endpoints

Discovered Next.js cron endpoints:

- `client/src/app/api/cron/rank-strategies/route.ts`
  - Scheduled nightly by `vercel.json`.
  - Reads Mongo `paper_trades`, writes `policy_snapshots`, and writes a ranking fixture.
- `client/src/app/api/cron/policy-snapshot/route.ts`
  - Scheduled by `vercel.json` at `5 0 * * *`.
  - Route comments and code references should be treated as secondary to `vercel.json`.
- `client/src/app/api/cron/paper-desk-tick/route.ts`
  - Route comment says it runs every 1 minute via Vercel cron.
  - It is not present in active `vercel.json`.
  - This is a documented-but-not-scheduled backup writer.

Security finding:

- Cron routes enforce `CRON_SECRET` only when the environment variable is set.
- A clone without `CRON_SECRET` may expose cron execution.

## Background Workers

BTC futures paper worker:

- PM2 source: `client/ecosystem.worker.config.cjs`
- Worker source: `client/scripts/btc-ft-paper-worker.ts`
- Process name: `btc-ft-worker`
- Instances: 1
- Restart delay: 5000 ms
- Max memory restart: 256M
- Default poll cadence: `POLL_MS = 4000`, minimum 2000 ms
- Required env: `DESK_WORKER_ACCOUNT_KEY`, `MONGODB_URI`, and desk policy env vars.
- Writes: `paper_state`, `paper_trades`, `desk_worker_events`, `verification_track_events`, `paper_oms_orders`.
- Lease: `WORKER_LEASE_TTL_MS = 45000` in `client/src/lib/paperDeskWorker/workerLease.ts`.

Duplicate-writer risk:

- The PM2 worker, browser BTC futures engine, and `paper-desk-tick` route can all mutate the same account if lease/account/env isolation fails.
- The cron route comments say fresh heartbeat is less than 3 minutes, but the actual active helper uses 45 seconds.

## Browser-Side Timer Loops

Browser engines discovered as stateful schedulers:

- `client/src/hooks/useBTCFuturesScalperEngine.ts`
  - Main poll around 4 seconds.
  - Worker-state monitor around 4 seconds.
  - Mongo save around 30 seconds.
  - Daily reset check around 60 seconds.
  - Kline cadence: 1m around 4 seconds, 5m/15m around 15 seconds, 4h around 60 seconds, 1d around 300 seconds.
- `client/src/hooks/useBTCSpotScalperEngine.ts`
  - Main poll around 2 seconds.
  - Save around 30 seconds.
- `client/src/hooks/useNiftyBeesEngine.ts`
  - LTP poll around 4 seconds.
  - Persist around 30 seconds.
- `client/src/hooks/useNiftyStocksEngine.ts`
  - LTP poll around 3 seconds.
  - DB save around 60 seconds.
- `client/src/hooks/useNiftyOptionsEngine.ts`
  - Tick around 1 second.
  - DB save around 60 seconds.
- `client/src/hooks/useNiftyOptionsSellingEngine.ts`
  - Tick around 1 second.
  - DB save around 60 seconds.
- `client/src/hooks/useMCXEngine.ts`
  - LTP poll around 5 seconds.
  - Engine tick around 5 seconds.
  - DB save around 60 seconds.
- `client/src/lib/writeQueue.ts`
  - Retry backoff: 100 ms, 200 ms, 400 ms.
- `client/src/lib/backupManager.ts`
  - Defines 15 minute browser backup cadence.
  - No active `backupManager.start()` call was certified.

Clone risk: open browser tabs are live schedulers. A true clone cutover must close or isolate original operator tabs before enabling clone writers.

## Go Runtime Timer Loops

Active or directly wired from `engine/cmd/antigravity/main.go`:

- Coinbase websocket connect and reconnect logic in `engine/internal/marketdata/coinbase.go`.
- Trading orchestrator loops in `engine/internal/trading/loop.go`.
  - Close-event worker consumes close events.
  - 1m/5m candle workers consume candle channels.
  - 15m strategies run every third 5m candle.
  - 1h strategies run every twelfth 5m candle.
  - Strategy cooldown checker around 30 seconds.
  - Auto fallback monitor around 10 seconds.
  - Bridge offline threshold around 15 seconds.
  - Pending signal fallback threshold around 45 seconds.
- State saver in `engine/internal/persistence/saver.go`.
  - Actual ticker: 15 seconds.
  - Comment says 30 seconds.
- Options BTC price feed in `engine/cmd/antigravity/main.go`.
  - Loop wakes every 1 second.
  - Delta/Binance REST refresh around every 10 seconds.
- NIFTY feed in `engine/cmd/antigravity/main.go`.
  - 15 seconds during NSE session.
  - 60 seconds outside NSE session.
- Options engine in `engine/internal/options/engine.go`.
  - Default tick interval around 10 seconds.
- Delta live bridge monitor in `engine/internal/delta/live_bridge.go`.
  - Around 5 minutes when enabled.
- Engine self-ping keepalive in `engine/cmd/antigravity/main.go`.
  - Actual cadence around 2 minutes.
  - Some comments/docs mention 10 minutes.
- Vault rotation engine in `engine/internal/security/vault/rotation.go`.
  - Policies include Binance/Delta 30 days, auth/admin 7 days, internal API 14 days, OpenAI/GROQ 90 days, and manual DB/Redis/Mongo rotation.

## Reconciliation and Recovery Schedules

Defined scheduler:

- Source: `engine/internal/reconciliationv2/scheduler.go`
- Default intervals:
  - Orders: 2 seconds
  - Positions: 5 seconds
  - Balances: 10 seconds
  - Exposure: 10 seconds
  - Full audit: 60 seconds

Defined backup schedule:

- Source: `engine/backup/backup_manager.go`
- Defaults:
  - Ledger backup: 1 minute
  - DB backup: 1 hour
  - Full infrastructure backup: 24 hours
  - Backup age metrics: 30 seconds

Defined HA schedules:

- `engine/internal/ha/heartbeat.go`: heartbeat around 2 seconds, timeout around 8 seconds.
- `engine/internal/ha/leader_election.go`: renew/campaign around 3 seconds, campaign timeout around 5 seconds.
- `engine/internal/ha/ledger_replication.go`: replication poll around 500 ms.
- `engine/internal/ha/ledger_integrity.go`: integrity check around 30 seconds.
- `engine/internal/ha/database_failover.go`: DB health check around 3 seconds.
- `engine/internal/ha/redis_failover.go`: Redis health check around 2 seconds.
- `engine/internal/ha/exchange_failover.go`: exchange health check around 5 seconds.
- `engine/internal/riskv3/portfolio_engine.go`: portfolio snapshot around 5 minutes.
- `engine/internal/ha/oms_ha.go`: replication lag measurement around 5 seconds.

Certification gap:

- Several institutional schedulers exist as packages but were not certified as launched by `engine/cmd/antigravity/main.go`.
- A clone cannot assume these schedules are active unless startup wiring or runtime logs prove it.

## Expected Schedules vs Actual Schedules

Drift found:

- Vercel has exactly two active cron jobs. The paper desk fallback cron route exists but is not scheduled.
- Workspace rules warn that Vercel Hobby plan allows only two cron jobs total; active config already uses two.
- `paper-desk-tick` comments say 1 minute schedule and less-than-3-minute fresh heartbeat; active config has no schedule and helper TTL is 45 seconds.
- `policy-snapshot` comments and actual `vercel.json` schedule differ.
- `StateSaver` comment says 30 seconds; actual ticker is 15 seconds.
- Engine keepalive comments mention 10 minutes; actual observed code cadence is 2 minutes.
- Reconciliation, backup, HA, and risk snapshot schedules are defined but not all proven active in main runtime.
- GitHub keepalive points to original Render, not a clone-safe endpoint.

## Dry-Run Scheduler Validation

Executed focused source tests:

```bash
go test -mod=mod ./internal/strategy ./internal/ledger ./internal/omsv3 ./internal/reconciliationv2
```

Result: passed.

The default command without `-mod=mod` failed due vendor drift before tests ran.

No end-to-end scheduler parity test was found that compares:

- `vercel.json`
- route comments
- PM2 config
- browser timer loops
- Go tickers
- HA/reconciliation/backup schedule defaults
- deployed runtime logs

## Certification Blockers

1. Create a canonical scheduler manifest committed to source.
2. Resolve `paper-desk-tick`: either schedule it by replacing one Vercel cron or document it as manual/disabled.
3. Do not exceed the two-job Vercel cron limit unless the hosting plan changes.
4. Require `CRON_SECRET` in production and clone environments.
5. Rewrite or disable GitHub keepalive before clone activation.
6. Prove only one BTC futures writer is active per account: PM2 worker, cron fallback, or browser, never uncontrolled overlap.
7. Close or isolate original browser tabs before source export and clone cutover.
8. Prove reconciliation, HA, backup, ledger integrity, and portfolio snapshot loops are actually wired in the deployed runtime or mark them as inactive.
9. Fix docs/comments that disagree with actual intervals.
10. Add a scheduler certification test that fails on schedule drift.

## Certification Decision

Schedulers are cloneable with manual controls and runtime verification. They are not fully reproducible from source because active deployment schedules, browser schedulers, and defined-but-not-wired timers are not represented by one authoritative manifest.
