# Trading Application — Architecture

Overview of **antigravity** (monorepo): how pieces connect, where paper vs live trading runs, and where data is stored.

> **Last updated:** 2026-05-22 · Reflects production focus on **BTC Future Trading** (CORE 20 + premium strategies).

---

## 1. High-level system

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         USER (Browser)                                   │
│  TradingDashboard → BTC Future Trading desk (+ optional dev panels)        │
└───────────────────────────────┬─────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  CLIENT (Next.js 16 + React 19) — Vercel                                │
│  • UI (M3 desk, scalpers, panels)                                       │
│  • API routes (/api/btc, /api/paper-trades, /api/engine/*, …)           │
│  • Hooks = per-market “state machines” (paper trading logic)            │
│  • Lib = signals, desk policy, PnL math, Mongo sync                     │
└───────┬─────────────────────────────┬───────────────────────────────────┘
        │                             │
        │ REST / proxy                │ (optional live path)
        ▼                             ▼
┌───────────────────┐       ┌─────────────────────────────────────────┐
│  DATA STORES      │       │  ENGINE (Go 1.25) — AWS Lightsail :8080   │
│  • MongoDB Atlas  │       │  marketdata → strategy → execution       │
│  • Supabase/PG*   │       │  risk, positions, backtest, delta bridge   │
│  • TimescaleDB*   │       └─────────────────────────────────────────┘
│  • Redis*         │                     ▲
└───────────────────┘                     │
        * via infrastructure/              │
┌───────────────────┐       ┌─────────────┴───────────────────────────┐
│  AI / ML          │       │  BRIDGE — LLM routing (Claude, GPT…)     │
│  PyTorch server*  │◄──────│  AI handoff for signals / decisions      │
└───────────────────┘       └─────────────────────────────────────────┘
```

**Important split:** Most **BTC Future Trading** behavior today runs in the **browser** (React hook + `/api/btc/futures-klines`). The **Go engine** is the path for market feeds, backtest, engine state, and **live execution** when wired. Do not assume both paths share identical paper semantics.

---

## 2. Repository layout

```
Trading application/          (GitHub: raghavar8088/antigravity)
├── client/                   Next.js frontend + API routes (Vercel)
├── engine/                   Go trading engine (HTTP :8080)
├── bridge/                   AI routing between LLMs
├── infrastructure/           Docker (PG/Timescale, Redis, Grafana) + Python ML
├── scripts/                  Build / deploy / migrations
├── nginx/                    Reverse proxy
└── docker-compose.prod.yml   Production backend stack (engine only)
```

---

## 3. Client (`client/`)

### 3.1 Entry & routing

| Path | Purpose |
|------|---------|
| `src/app/page.tsx` | Home → `TradingDashboard` |
| `src/app/btc-future-trading/` | BTC futures paper module (dedicated route) |
| `src/app/sign-in/` | Magic-link / session sign-in |
| `src/app/api/btc/` | Klines / ticker (futures + spot) |
| `src/app/api/paper-trades/` | Trade CRUD — **MongoDB primary** |
| `src/app/api/delta/` | Delta Exchange (testnet + live probes) |
| `src/app/api/engine/[...path]/` | Proxy to Go engine |
| `src/app/api/auth/` | Session (JWT cookie) + sign-in |
| `src/app/api/strategy-rankings/` | Replay rankings JSON (optional winners gate) |
| `src/app/api/cron/` | Policy snapshot, strategy ranking jobs |

Legacy API routes for Nifty, MCX, crypto, etc. may still exist in the tree; the **shipped UI** is currently centered on BTC Future Trading.

### 3.2 UI layers

| Layer | Role |
|-------|------|
| `TradingDashboard.tsx` | Shell: workspace nav, settings, embeds BTC FT module |
| `DeskShell` / `DeskAppBar` | M3 layout (Google-style desk) |
| `BTCFuturesScalper.tsx` | Futures desk UI (positions, trades, leaderboard) |
| `BTCFutureTradingScalper.tsx` | BTC-only wrapper: roster, env modes, banners |
| `BTCFuturesDeskPanels.tsx` | Equity, profile, leaderboard, positions, trades |
| `EntryDebugPanel` | Entry gate diagnostics (`dominantBlocker`) |
| `ReplayBacktestPanel` | Offline replay UI (dev) |
| `PaperDeskAuthBar` | Sign-in / session |

### 3.3 Hooks (one per market)

Each module can own a large hook — a state machine for that market:

| Hook | Market |
|------|--------|
| `useBTCFuturesScalperEngine.ts` | **BTC futures paper (primary)** |
| `useBTCSpotScalperEngine.ts` | BTC spot |
| `useNiftyOptionsEngine.ts` | Nifty 50 options |
| `useMCXEngine.ts` | MCX commodities |
| `useCryptoEquityEngine.ts` | Altcoins |
| `useDeltaLive.ts` | Delta live bridge |
| … | Other legacy hooks |

**BTC Future Trading** uses the futures hook with:

- `storageNamespace`: e.g. `btc_future_trading_v4`
- `moduleKey`: `btc_future_trading`
- Poll ~4s, 1m bars, default **12** open positions (env-overridable)
- `cloudAccountKey`: Supabase user id, or anonymous device key when enabled

### 3.4 Business logic (`client/src/lib/`)

| Module | Responsibility |
|--------|----------------|
| `futuresStrategies.ts` | **CORE 20** + premium (500–503) definitions |
| `futuresSignals.ts` | `evalMinuteSignal`, HTF OHLCV, confirmation |
| `futuresDeskPolicy.ts` | TP widen, regime, spread cap, entry gates, env helpers |
| `futuresPaperMath.ts` | PnL, fees, funding, liquidation, slippage, exits |
| `futuresDeskRuntime.ts` | Shared exit step (trail, profit-lock, SL/TP/TIME) |
| `btcFtResearch.ts` | Research pool, rotation, winners, verdicts |
| `btcFtRoster.ts` | Active strategy ID resolution |
| `btcFtStrategyTemplates.ts` | Stub (extended pool removed from production) |
| `btcFtStrategyGenerator.ts` | Stub (generated 300–399 removed from production) |
| `paperTradesSync.ts` | POST trades, retry queue |
| `mongoTradesClient.ts` | MongoDB upsert/list |
| `anonAccountKey.ts` | Browser-persistent anonymous `account_key` |
| `futuresReplayEngine.ts` | Offline replay on fixtures |

### 3.5 Paper trade lifecycle (client)

```
Poll klines (REST) → buildSignalInputs → evalMinuteSignal
    → passesEntryConfirmation → gates (min-move, spread, regime, session, category cap)
    → entry priority (max open slots) → openPosition (slippage on entry)
    → mark + funding each poll → resolveFuturesExitStep
    → closePosition (slippage on exit) → POST /api/paper-trades
         → Mongo upsert (primary)
    → localStorage snapshot periodically
```

---

## 4. Go engine (`engine/`)

**Entry:** `engine/cmd/antigravity/main.go` — HTTP server on port **8080** (mapped to **80** in production Docker).

| Package | Role |
|---------|------|
| `marketdata/` | Binance, Coinbase, Delta, Angel, NSE feeds |
| `strategy/` | Strategy framework & lifecycle |
| `execution/` | Live orders + paper simulator |
| `positions/` | Open positions, PnL, margin |
| `risk/` | Limits, daily loss, circuit breakers |
| `options/` | Nifty options, Greeks, chain |
| `delta/` | Delta client + live bridge |
| `backtest/` | Historical simulation |
| `persistence/` | SQLite + PostgreSQL |
| `ai/` | Multi-LLM strategy generation |
| `admin/` | Kill switch |
| `telemetry/` | Prometheus metrics |

Client reaches engine via **`/api/engine/*`** proxy (set `INTERNAL_API_URL` / Lightsail host on Vercel).

**Deploy (AWS):**

```bash
cd ~/antigravity && git pull origin main \
  && docker-compose -f docker-compose.prod.yml build --no-cache engine \
  && docker stop antigravity_engine 2>/dev/null; docker rm -f antigravity_engine 2>/dev/null \
  && docker-compose -f docker-compose.prod.yml down --remove-orphans \
  && docker-compose -f docker-compose.prod.yml up -d --force-recreate \
  && sleep 5 && curl -sS http://127.0.0.1/health
```

---

## 5. Bridge & AI (`bridge/` + `infrastructure/`)

| Component | Role |
|-----------|------|
| `bridge/` | Routes AI requests (Claude, GPT, Gemini, Groq, Mistral) |
| `infrastructure/ai/model_server.py` | PyTorch model serving |
| `infrastructure/ai/strategy_service/` | REST wrapper for ML strategies |

Often **adjacent** to the desk (insights panel) — not always in the critical path for every paper entry.

---

## 6. Data architecture

| Store | Primary use | Notes |
|-------|-------------|-------|
| **MongoDB** (`loop_trades.paper_trades`) | Paper trade ledger | **Required** for cloud persistence; `MONGODB_URI` on Vercel |
| **Supabase** (PostgreSQL) | Auth, optional promotions API | Magic link; redirect URLs must match Vercel domain |
| **TimescaleDB** (Docker) | Market time-series | Engine / infra |
| **SQLite** | Local engine fallback | Dev / edge on AWS volume |
| **Redis** | Cache (optional) | Infra stack |
| **localStorage** | Per-browser paper wallet | Key: `{namespace}_paper_state` |

**Per-browser vs cloud:** Same URL + sign-in (or anon key) → Mongo sync. Different browser / cleared storage → local-only until trades POST successfully.

**Trade document (conceptual):** `client_trade_id`, `account_key`, `module_key`, prices, PnL, fees, funding, `strategy_name`, `exit_reason`, timestamps.

**Anonymous writes:** When `ALLOW_ANON_PAPER_TRADES=1` / `NEXT_PUBLIC_ALLOW_ANON_PAPER_TRADES=1`, the client generates `anon_<uuid>` and POSTs without sign-in.

---

## 7. Supported markets (modules)

| Market | Exchange (typical) | UI / hook |
|--------|-------------------|-----------|
| **BTC Futures** | Delta / Binance data | `BTCFutureTradingScalper` → `useBTCFuturesScalperEngine` |
| BTC Spot | Binance | `useBTCSpotScalperEngine` |
| Nifty 50 Options | NSE / Angel | `useNiftyOptionsEngine` |
| MCX | MCX | `useMCXEngine` |
| Altcoins | Binance | `useCryptoEquityEngine` |
| Delta derivatives | Delta Exchange | `useDeltaLive` |

**Current production UI** focuses on **BTC Future Trading** only; other hooks remain for future routes or research.

---

## 8. BTC Future Trading — module-specific architecture

```
┌─────────────────────────────────────────────────────────────┐
│ BTCFutureTradingScalper                                      │
│  • Roster: CORE 20 (+ premium 500–503)                       │
│  • Env: WINNERS_ONLY, STRATEGY_IDS, RESEARCH_MODE, …         │
│  • module_key: btc_future_trading                            │
│  • BUILD hash in tagline (NEXT_PUBLIC_VERCEL_GIT_COMMIT_SHA) │
└────────────────────────────┬────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ useBTCFuturesScalperEngine                                   │
│  • /api/btc/futures-klines (mark, funding, 1m candles)       │
│  • buildPaperDeskStrategies() → active defs                    │
│  • paperBootstrapProbe: force probe after 5m if zero trades    │
└────────────────────────────┬────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────┐
│ Gates: signal ≥ threshold, confirm, cooldown, min-move,      │
│ spread, regime, session UTC, category cap, open slots,       │
│ slippage (default 5 bps), drawdown pause                       │
└─────────────────────────────────────────────────────────────┘
```

### Roster resolution (`resolveBtcFtActiveStrategyIds`)

When `NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1`:

1. `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS` (explicit comma list)
2. Promoted winners (localStorage + optional Mongo promotions API)
3. Full **CORE** basket (never empty — desk always renders)

When research mode (`NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1`):

- Rotating batches (~30 active / 24h) from CORE-only pool (extended/generated pools removed from production registry).

---

## 9. Deployment topology

| Environment | Client | Engine | Secrets |
|-------------|--------|--------|---------|
| **Local** | `npm run dev` in `client/` | Optional Docker | `client/.env.local` |
| **Vercel** | Production UI | Proxied via `INTERNAL_API_URL` | `MONGODB_URI`, `MONGODB_DB`, auth, `ALLOW_ANON_PAPER_TRADES` |
| **AWS Lightsail** | — | `docker-compose.prod.yml` | `.env` on instance |

**Vercel root directory:** `client/`

**Production should not expose** without explicit env: testnet ops, replay UI, research firehose.

---

## 10. Modes & feature flags

| Env flag | Purpose |
|----------|---------|
| `NEXT_PUBLIC_BTC_FT_WINNERS_ONLY=1` | Production paper: env IDs → promoted → CORE fallback |
| `NEXT_PUBLIC_BTC_FT_STRATEGY_IDS` | Manual roster override (comma-separated CORE IDs) |
| `NEXT_PUBLIC_BTC_FT_RESEARCH_MODE=1` | Tournament / batch rotation |
| `NEXT_PUBLIC_BTC_FT_USE_RANKED=1` | Filter by `/api/strategy-rankings` |
| `NEXT_PUBLIC_ALLOW_ANON_PAPER_TRADES=1` | Mongo writes without sign-in |
| `NEXT_PUBLIC_DESK_ENTRY_DEBUG=1` | Entry debug panel |
| `NEXT_PUBLIC_BTC_FT_FIREHOSE=1` | High concurrency (research only) |

---

## 11. Strengths & risks

| Strengths | Risks |
|-----------|-------|
| Clear client / engine split | Two paper models (browser vs Go) can diverge |
| Rich desk policy & metrics | Large hooks — hard to maintain |
| Mongo persistence + anon mode | Build fails if types/imports drift (e.g. missing `FUTURES_STRAT_DEFS` import) |
| CORE 20 focus reduces fee bleed | Single symbol (BTC) — correlated entries |
| Research → winners promotion path | Expectancy after fees not guaranteed |

---

## 12. Profit path (how architecture supports it)

```
Research mode (optional) → paper trades → Mongo by strategy
       → rank by sumNet / expectancy (fee-aware)
       → promote WINNERS → WINNERS_ONLY production
       → optional: Go backtest / testnet / live
```

Architecture does **not** guarantee profit; a small winner set with positive expectancy after fees does.

---

## 13. Related docs in repo

| Doc | Topic |
|-----|--------|
| [PAPER_DESK_RUNBOOK.md](./PAPER_DESK_RUNBOOK.md) | Env vars, tuning |
| [DEPLOY.md](./DEPLOY.md) | Vercel, Supabase, Mongo |
| [LIVE_TRADING_PHASE.md](./LIVE_TRADING_PHASE.md) | Testnet → mainnet |
| [SHADOW_VS_PAPER.md](./SHADOW_VS_PAPER.md) | Reconciliation |
| [SCALPING_STRATEGY_RESEARCH.md](./SCALPING_STRATEGY_RESEARCH.md) | Strategy taxonomy |
| [ROOT_CAUSE.md](./ROOT_CAUSE.md) | Incident notes |

---

## 14. Quick verification checklist

| Check | Where |
|-------|--------|
| Latest client deploy | Tagline `BUILD <sha>` on desk |
| Engine healthy | `curl http://<aws-ip>/health` → `"status":"ok"` |
| Mongo writes | Closed trade → `loop_trades.paper_trades` in Atlas |
| Entry blocked? | Entry debug → `dominantBlocker` |
| Roster empty? | Vercel env: `BTC_FT_STRATEGY_IDS` or disable `WINNERS_ONLY` |
