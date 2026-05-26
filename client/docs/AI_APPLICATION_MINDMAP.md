# AI Application Mind Map — BTC Paper Futures Desk

> **For AI agents:** This document describes the runtime topology, critical data flows, hard constraints, and debug procedures for this trading application. Read this before modifying any file under `client/`. The JSON twin is at `client/docs/ai-application-mindmap.json`.

---

## 1. What This App Is

A **paper-trading futures desk** for BTC/USD perpetuals on Delta Exchange India. It runs 24/7 signal evaluation against a roster of ~24 pre-configured strategies, opens and closes simulated positions, tracks PnL, and provides a live browser UI. No real money moves. Live order placement on Delta Exchange is **not implemented** — all trades are synthetic, stored only in MongoDB Atlas.

**Stack:** Next.js 16.2.1 (App Router) · React 19 · MongoDB 7.2 · TypeScript · Vercel Hobby plan · AWS Lightsail (Go engine on `:8080`) · pm2 worker on VPS.

---

## 2. Runtime Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Vercel (Next.js 16, App Router)                            │
│  ├─ Browser UI   — BTCFuturesScalper.tsx + hooks            │
│  ├─ API Routes   — /api/desk-entry-funnel, /api/cron/*      │
│  └─ Cron fallback — /api/cron/paper-desk-tick (every 60s)   │
└───────────────────────┬─────────────────────────────────────┘
                        │ MongoDB Atlas M0
                        │ Collections: paper_trades, paper_state,
                        │   desk_worker_events, paper_research
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  VPS (pm2) — btc-ft-paper-worker.ts                         │
│  ├─ 4s poll, worker lease TTL=45s                           │
│  ├─ Acquires lease → evaluates strategies → opens/closes    │
│  └─ Logs funnel line every tick                             │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  Delta Exchange India REST API (read-only)                  │
│  GET /v2/history/candles  — klines only, no order placement │
└─────────────────────────────────────────────────────────────┘
```

**Browser monitor mode**: When `NEXT_PUBLIC_DESK_WORKER_ENABLED=1` and the VPS worker heartbeat is fresh (age < 45s), the browser hook (`useBTCFuturesScalperEngine.ts`) runs in **read-only** mode — it fetches market data and evaluates signals for display only; all writes go through the VPS worker. When the heartbeat is stale, the browser falls back to write mode.

**Vercel cron fallback**: `/api/cron/paper-desk-tick` fires every 60s on Vercel. It skips its tick if the VPS worker heartbeat is fresh, preventing double-execution.

---

## 3. Critical BTC Desk Flow (Single Poll Cycle)

1. **Market data fetch** — `fetchBTCKlines()` pulls 200 1-min candles from Delta Exchange kline API.
2. **Strategy evaluation** — Each active strategy in `FUTURES_STRAT_DEFS` (IDs 91–152 CORE, 500–503 premium) is scored. Research-only IDs (600–759) are excluded from the worker.
3. **Entry gates** (ordered, each can block independently):
   - `noData` — no valid candle data
   - `noStrategies` — roster empty or all IDs invalid
   - `maxOpen` / `margin` — position count or balance cap
   - `session` — trading session window gate
   - `signal` — strategy score < threshold (default 26)
   - `regime` — market regime check (trend/chop/volatile)
   - `confirm` — MTF confirmation gate
   - `atrFees` — ATR-vs-fee min-move hurdle
   - `cooldown` — post-exit lockout
   - `rotation` — rotation score gate (suspended strategies blocked)
   - `spread` — bid/ask spread too wide
   - `category` / `sameSide` — portfolio concentration caps
4. **Paper open** — if all gates pass, inserts a new doc into `paper_trades` with synthetic fill price.
5. **Paper close** — on TP/SL/time exit, updates the trade doc with `closedAt`, `netPnl`, `exitReason`.
6. **MongoDB persistence** — `upsertAccountState()` writes `paper_state` including `entry_funnel_snapshot`.
7. **UI monitor** — `BTCFuturesScalper.tsx` polls MongoDB, renders `EntryFunnelCard` (always visible) showing the dominant gate and recommendation.

---

## 4. Key Files Map

| Purpose | File |
|---|---|
| Browser engine hook | `client/src/hooks/useBTCFuturesScalperEngine.ts` |
| Browser UI root | `client/src/components/BTCFuturesScalper.tsx` |
| VPS worker main loop | `client/scripts/btc-ft-paper-worker.ts` |
| Worker poll logic | `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` |
| Strategy definitions | `client/src/lib/futuresStrategyDefs.ts` |
| Entry gate logic | `client/src/lib/futuresDeskPolicy.ts` |
| Funnel diagnostics (pure) | `client/src/lib/deskEntryFunnelSnapshot.ts` |
| Session metrics + probe exclusion | `client/src/lib/futuresSessionMetrics.ts` |
| MongoDB client | `client/src/lib/mongoTradesClient.ts` |
| Worker lease (server-only) | `client/src/lib/paperDeskWorker/workerLease.ts` |
| Rotation system | `client/src/lib/futuresStrategyRotation.ts` |
| Edge candidates panel | `client/src/components/btcFutures/EdgeCandidatesPanel.tsx` |
| Funnel API route | `client/src/app/api/desk-entry-funnel/route.ts` |
| Cron fallback route | `client/src/app/api/cron/paper-desk-tick/route.ts` |
| Desk UI primitives | `client/src/components/desk/ui.tsx` |
| Tests — funnel | `client/src/lib/tests/deskEntryFunnel.test.ts` |

---

## 5. State Ownership

| State | Owner | Location |
|---|---|---|
| Open positions | Worker → Mongo | `paper_trades` collection, `status: "open"` |
| Closed trade history | Worker/Browser → Mongo | `paper_trades` collection, `status: "closed"` |
| Account balance, equity | Worker → Mongo | `paper_state.balance`, `paper_state.equity` |
| Entry funnel snapshot | Browser + Worker → Mongo | `paper_state.entry_funnel_snapshot` |
| Worker heartbeat | Worker → Mongo | `paper_state.worker_last_poll_at`, `paper_state.worker_id` |
| Rotation report | Computed in browser/worker | `paper_state.rotation_report` |
| Worker event log | Worker → Mongo | `desk_worker_events` collection |
| Research trades | Research mode only | `paper_research` collection |
| localStorage fallback | Browser (degraded mode) | `btc_future_trading_v4` namespace |

---

## 6. Important Environment Variables

| Variable | Default | Effect |
|---|---|---|
| `NEXT_PUBLIC_DESK_WORKER_ENABLED` | `0` | `1` = VPS worker mode active; browser switches to monitor-only when heartbeat fresh |
| `NEXT_PUBLIC_BTC_FT_SIGNAL_THRESHOLD` | `26` | Minimum strategy score to pass signal gate |
| `NEXT_PUBLIC_BTC_FT_MIN_MOVE_K_MUL` | `1.0` | ATR-vs-fee safety multiplier; raise to tighten fee gate |
| `NEXT_PUBLIC_BTC_FT_WINNERS_ONLY` | `0` | `1` = restrict roster to known-winning strategy IDs only |
| `NEXT_PUBLIC_DESK_ENTRY_CANARY` | `0` | `1` = fire diagnostic canary trade (PAPER_ENTRY_CANARY) after 5-min idle; **off by default** |
| `NEXT_PUBLIC_DESK_ENTRY_DEBUG` | `0` | `1` = expose full entry debug panel in UI |
| `DESK_WORKER_STRATEGY_IDS` | all CORE | Comma-separated strategy IDs for VPS worker (validated against `FUTURES_STRAT_DEFS`) |
| `NEXT_PUBLIC_BTC_FT_MAX_OPEN` | `3` | Max concurrent open paper positions |
| `NEXT_PUBLIC_DESK_WORKER_ENABLED` | `0` | Controls whether VPS worker runs or browser is primary |
| `MONGODB_URI` | *(required)* | Atlas M0 connection string (server-side only — never exposed to browser) |
| `NEXT_PUBLIC_RESEARCH_MODE` | `0` | `1` = firehose all strategies for data collection; not for production |

---

## 7. Hard Constraints

These constraints are invariants — do not violate them regardless of how you interpret a task description:

1. **Paper only.** No real order placement on Delta Exchange or any other exchange. `openPosition()` and `closePosition()` are local MongoDB writes only.
2. **Never lower the signal threshold to force trades.** If no trades are opening, diagnose the gate (use the funnel diagnostic). Do not reduce `threshold` or `minMoveKMul` below fee-break-even.
3. **Never bypass gates.** Do not skip fee/ATR, regime, MTF, quality, rotation, session, or spread gates. Each gate exists to prevent guaranteed-loss entries.
4. **Never delete historical trades.** `paper_trades` docs are append-only. Close a trade by updating `status: "closed"` — never delete the document.
5. **Canary must be OFF by default.** `NEXT_PUBLIC_DESK_ENTRY_CANARY` must not be set to `"1"` in any production environment without explicit operator opt-in. The canary trade `PAPER_ENTRY_CANARY` must always be excluded from all production metrics by `isProbeOrBootstrapTrade()`.
6. **MongoDB credentials server-side only.** `MONGODB_URI` must never appear in any `NEXT_PUBLIC_*` variable or be imported in browser-side code. `workerLease.ts` and `mongoTradesClient.ts` are server-only modules.
7. **No live engine changes without explicit approval.** The Go engine on AWS Lightsail (`:8080`) handles live trading. Do not modify `engine/` files without explicit user authorization.
8. **Vercel Hobby plan limits:** Max 2 crons, minimum 1-minute frequency. Do not add additional cron routes.

---

## 8. Common Failure Modes

### No trades opening (most common)

| Symptom | Likely cause | Check |
|---|---|---|
| `dominantBlocker: signal` | Score < threshold (26) | Market may be choppy; wait or check regime |
| `dominantBlocker: regime` | Chop/volatile regime | Market structure; not a bug |
| `dominantBlocker: atrFees` | ATR too small vs fee | `minMoveKMul` correct at 1.0? |
| `dominantBlocker: noStrategies` | `DESK_WORKER_STRATEGY_IDS` has invalid IDs | Check against `FUTURES_STRAT_DEFS` |
| `dominantBlocker: rotation` | All strategies suspended by rotation | Check `rotation_report` in Mongo |
| `dominantBlocker: maxOpen` | Already at max positions | Check `paper_trades` for open docs |
| `dominantBlocker: cooldown` | Recent exit locked out re-entry | Wait cooldown period |

### Worker stale / not running

Signs: `EntryFunnelCard` shows "STALE" banner; `worker_last_poll_at` age > 60s. Fix: `pm2 restart btc-ft-paper-worker` on VPS, or wait for Vercel cron fallback.

### Balance drift / corruption

Caused by bootstrap probe trades mutating balance. Fixed: `isProbeOrBootstrapTrade()` excludes BOOTSTRAP, PROBE, DEV_FORCE, CANARY from all metrics. Use `computeSessionEquityFromProduction()` for equity display.

### Old build deployed

Signs: UI behavior doesn't match code. Fix: check Vercel deployment status. Force redeploy via `git commit --allow-empty && git push`.

### Roster empty in worker

Signs: `noStrategies` dominant blocker in worker logs within the first tick. Fix: validate `DESK_WORKER_STRATEGY_IDS` env var contains valid IDs from `FUTURES_STRAT_DEFS`.

### Canary fires unexpectedly

Signs: `PAPER_ENTRY_CANARY` trades appearing in Mongo. Cause: `NEXT_PUBLIC_DESK_ENTRY_CANARY=1` was set. Fix: remove the env var. The canary must be excluded from all metrics by `isProbeOrBootstrapTrade()` — verify this is working.

---

## 9. How to Debug Quickly

**Step 1 — Check the Entry Funnel Card in UI.** The card is always visible when the desk is mounted and ready. Read `dominantBlocker` and `recommendation` fields.

**Step 2 — Check worker logs.** Every tick logs: `funnel=[strats=N sig=N cand=N open=N blocker=X]`. After 30 consecutive zero-open ticks, a `NO-OPEN-WATCH` warning appears.

**Step 3 — Query `/api/desk-entry-funnel`** (GET). Returns `{ ok, snapshot, ageSeconds, healthy }`. `healthy: false` means snapshot is stale (>30s).

**Step 4 — Check MongoDB `paper_state`** for `entry_funnel_snapshot`, `worker_last_poll_at`, and `rotation_report`.

**Step 5 — Check `desk_worker_events`** collection for `worker_tick_error` events — these appear after 30 consecutive zero-open ticks.

**Step 6 — Run TypeScript check:** `cd client && npx tsc --noEmit` — must pass zero errors.

**Step 7 — Run tests:** `cd client && npx vitest run src/lib/tests/` — 46+ tests in `deskEntryFunnel.test.ts` must all pass.

**Step 8 — Enable entry debug panel:** Set `NEXT_PUBLIC_DESK_ENTRY_DEBUG=1` locally and reload. The full `DeskEntryPollDebug` object is visible in the UI.

---

## 10. Last Updated

```
Date:    2026-05-26
Author:  AutoBot / Claude (claude-sonnet-4-6)
Commits: 11f8db8 (canary exclusion + EntryFunnelCard 10 fields + 46 tests)
         a6bcfbf (EdgeCandidatesPanel build fix)
         925388d (FIX 1-8: entry funnel diagnostics)
Session: PR-AI-APP-TRACKER
```

This document was auto-generated and should be updated whenever:
- New entry gates are added or removed
- New env vars are introduced
- Worker or browser architecture changes
- New collections are added to MongoDB
- Hard constraints change
