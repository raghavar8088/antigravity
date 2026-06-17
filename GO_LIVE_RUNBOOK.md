# Mock Trading Executor — Go-Live Runbook

Backend-driven mock-trading execution. **PM2 on AWS Lightsail is primary** (5s cycles). Vercel cron (`/api/cron/mock-trading-tick`) runs every **1 minute** as fallback (Hobby plan: max 2 crons, no sub-minute).

---

## Pre-flight (local)

```bash
cd client
npm run test -- src/lib/tests/mockTradingExecutor.test.ts src/lib/tests/mockTradingExecutorDiagnostics.test.ts src/lib/tests/mockTradingExecutorFunnel.test.ts
npm run build
npm run mock-trading:fix-corrupt   # once, removes opened_at=0 rows
```

Required env on Lightsail and Vercel:

| Variable | Where | Purpose |
|----------|-------|---------|
| `MONGODB_URI` | Lightsail + Vercel | Trade persistence |
| `MONGODB_DB` | Lightsail + Vercel | Default `loop_trades` |
| `CRON_SECRET` | Vercel only | Protects cron endpoint |
| `DELTA_API_BASE_URL` | Lightsail | BTC klines (optional fallback chain) |

---

## Step 1 — Deploy code

Push latest `main` to Vercel (frontend + API routes). SSH to Lightsail and pull the same commit into `client/`.

---

## Step 2 — Start PM2 worker (Lightsail)

```bash
cd /path/to/antigravity-main/client

# Verify Mongo
grep MONGODB_URI .env.local

# Start executor (5s interval)
pm2 start ecosystem.config.js --only mock-trading-executor
pm2 save
pm2 status
```

Expected: `mock-trading-executor` status **online**.

---

## Step 3 — Verify logs

```bash
pm2 logs mock-trading-executor --lines 20
```

Look for lines like:

```
[mock-trading-executor] tick ok=true opened=0 closed=0 candidates=0 blocker=regime duration=...ms
```

`blocker=regime` with zero opens is **normal** in ranging markets — not a failure.

---

## Step 4 — API health checks

Replace `YOUR_APP` with your Vercel deployment URL.

```bash
# Executor health (no auth)
curl -s "https://YOUR_APP/api/mock-trading/executor-status?account_key=mock_trading_main" | jq '{healthy:.executor.healthy, age:.executor.ageSeconds, blocker:.executor.dominantBlocker, diagnosis:.no_trade_diagnosis.status, nextAction:.no_trade_diagnosis.nextAction}'

# Aggregated diagnostics
curl -s "https://YOUR_APP/api/diagnostics/health?account_key=mock_trading_main" | jq '{healthy, checks:.checks.executor}'

# Dry-run signal trace
curl -s "https://YOUR_APP/api/mock-trading/signal-trace?account_key=mock_trading_main" | jq '{regime, dominantBlocker, candidateCount, dataFresh}'

# Cron (requires CRON_SECRET)
curl -s -H "Authorization: Bearer $CRON_SECRET" "https://YOUR_APP/api/cron/mock-trading-tick?account_key=mock_trading_main" | jq '{ok, opened, closed, dominant_blocker}'
```

**Exit criteria:**

- `executor.healthy: true`
- `executor.ageSeconds` < 30 (after PM2 started)
- Trade Engine UI shows **Backend Live** + recommendation panel when gates block entries

---

## Step 5 — UI verification

1. Open `/terminal/trade-engine` (or Trade Engine route).
2. Confirm **Engine Status: Backend Live** (green path when executor healthy).
3. When no trades: see **Status: MARKET_WAIT** + **Recommendation** (not a red error for regime/ATR blocks).
4. Open positions and closed trades should update every ~2s from MongoDB (read-only).

---

## Step 6 — First 24h monitoring

| Check | Command / endpoint | Alert if |
|-------|-------------------|----------|
| Worker alive | `pm2 status` | not `online` |
| Recent tick | `/api/mock-trading/executor-status` | `healthy: false` or age > 30s |
| Errors | `pm2 logs mock-trading-executor --err` | MongoDB / kline failures |
| Trades | MongoDB `mock_trades` or UI table | zero opens for 24h **and** `EXECUTOR_STALE` (not `MARKET_WAIT`) |

Restart if stale:

```bash
pm2 restart mock-trading-executor
```

---

## Rollback

1. Stop worker: `pm2 stop mock-trading-executor`
2. Re-enable browser polling on legacy dashboard only (temporary): set `disablePolling: false` in `MockTradingDashboard.tsx`
3. Cron remains as slow fallback

---

## Architecture reference

```
PM2 worker (5s) ──┐
                  ├──► runMockTradingExecutorCycle()
Vercel cron (1m) ─┘         │
                            ├─ fetch klines
                            ├─ evaluate signals + gates
                            ├─ open/close trades → MongoDB
                            └─ persist mock_executor_state + mock_execution_logs

UI (read-only) ──► GET /api/mock-trading/executor-status (2s poll)
```

Key paths:

- `client/src/lib/mockTradingExecutor/runMockTradingExecutorCycle.ts`
- `client/scripts/mock-trading-executor-worker.ts`
- `client/ecosystem.config.js`
- `client/src/app/api/cron/mock-trading-tick/route.ts`
