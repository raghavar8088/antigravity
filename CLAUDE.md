# Trading Application — Claude Code Context

## What This Is
Institutional-grade algorithmic trading platform for Indian markets (NSE/BSE via AngelOne) and crypto (Binance, Delta Exchange). Supports live trading, paper trading ($1M paper accounts), and backtesting. 600+ active strategies. Currently at ~75–80/100 institutional readiness (Phase 13–14 active).

---

## Tech Stack
| Layer | Tech | Location |
|-------|------|----------|
| Frontend + API routes | Next.js (TypeScript) | `client/` |
| Execution Engine | Go | `engine/` |
| AI/Strategy Brain | Python | `brain/` |
| Broker Bridge | TS/Go | `bridge/` |
| Databases | MongoDB Atlas, PostgreSQL (Neon), Redis, SQLite | `infrastructure/` |
| Deploy | Vercel (client), AWS Lightsail (engine) | — |

---

## Port Map
| Service | Port |
|---------|------|
| Next.js dev | :3000 |
| Go engine (local) | :8080 |
| Go engine (Render) | :10000 |
| AI gRPC | :50051 |
| Prometheus metrics | /metrics on engine port |

---

## Go Engine Entry Points (`engine/cmd/`)
- **`antigravity/main.go`** — Main trading engine. Boots 600+ strategies, BTC + NIFTY paper accounts, multi-agent AI, risk engine, kill switch.
- **`backtest/main.go`** — Offline strategy backtesting.
- **`perfbench/main.go`** — Performance benchmarking.
- **`seed_db/main.go`** — Database initialization.

Engine HTTP server loads `.env` via `loadDotEnv()` at lines 92–115 of `engine/cmd/antigravity/main.go`. Self-pings `/health` every 2 min to prevent Render sleep.

---

## Execution Data Flow
```
Market Data (Coinbase WS / Binance REST / AngelOne)
  → Strategy Registry (600+ strategies in engine/internal/strategy/)
  → Risk Gate (engine/internal/risk/gate/)
  → OMS v3 (engine/internal/omsv3/)
  → Execution (engine/internal/trading/loop.go)
  → Fill → Position Update
  → Ledger (engine/internal/ledger/)
  → Reconciliation (engine/internal/reconciliation/)
  → Kill Switch check (engine/internal/killswitch/)
  → Persistence (SQLite + MongoDB)
  → Next.js API → Dashboard
```

---

## Active Brokers
| Broker | Market | Status | Auth Keys |
|--------|--------|--------|-----------|
| Coinbase WS | BTC-USD price feed | Live (primary) | None (public) |
| Binance REST | BTC spot fallback | Live | `BINANCE_API_KEY/SECRET` |
| Delta Exchange | BTC options ticks + orders | Live | `DELTA_API_KEY/SECRET` |
| AngelOne | NSE/BSE equity + NIFTY | Live | `ANGELONE_API_KEY`, `ANGELONE_CLIENT_CODE`, `ANGELONE_PIN`, `ANGELONE_TOTP_SECRET` |
| Yahoo Finance | NIFTY warmup bars | Live (public) | None |
| NSE REST | NIFTY index quotes | Live (fallback) | None |

Fallback chain: Delta → Binance, NSE → AngelOne, synthetic spot if both fail.

---

## API Routes (102 total — `client/src/app/api/`)
Key domains:
- **Auth**: `/api/auth/session|signin|signout`
- **BTC Paper Desk**: `/api/paper-*` (20+ routes: trades, state, replay, OMS, diagnostics)
- **BTC Options**: `/api/options/*` (positions, trades, strategies, stats, reset)
- **NIFTY 50**: `/api/nifty/*` (candles, option-chain, selling-state, VIX, stream)
- **NIFTY Options**: `/api/nifty-options/*` and `/api/nifty-options-selling/*`
- **NIFTY Stocks**: `/api/nifty-stocks/*`
- **Delta Live**: `/api/delta-live/*` (stats, trades, enable, mode, manual order)
- **Live Engine**: `/api/pre-live/api/live/*` (stats, trades, enable, close-all) — proxied to the pre-live process, which hosts the Delta Exchange trade-cloning mirror (`engine/internal/livemirror`)
- **AngelOne**: `/api/angelone/*` (order, cancel, funds) + `/api/angel-proxy`
- **Engine proxy**: `/api/engine/[...path]` → catches all and forwards to Go at `INTERNAL_API_URL`
- **Cron/Admin**: `/api/cron/paper-desk-tick`, `/api/admin/kill`, `/api/admin/reset`
- **AI Tracker**: `/api/ai-app-tracker/*`

---

## Database Architecture
| DB | Purpose | Key |
|----|---------|-----|
| MongoDB Atlas | Paper trades, auth sessions, audit logs | `MONGODB_URI`, `MONGODB_DB=loop_trades` |
| PostgreSQL (Neon) | TimescaleDB metrics, phase 14 schema | `DATABASE_URL` |
| Redis | Indicator cache, performance cache | `REDIS_URL` |
| SQLite | Engine state (local fallback) | `SQLITE_PATH=./data/engine.db` |

SQLite opened in `engine/internal/persistence/store.go` lines 87–100. Falls back to filesystem snapshots under `ENGINE_DATA_DIR` if MongoDB unavailable.

---

## Key Environment Variables
```
# Brokers
BINANCE_API_KEY / BINANCE_API_SECRET
DELTA_API_KEY / DELTA_API_SECRET
ANGELONE_API_KEY / ANGELONE_CLIENT_CODE / ANGELONE_PIN / ANGELONE_TOTP_SECRET

# AI
OPENAI_API_KEY
GEMINI_API_KEY
GROQ_API_KEY  ← free tier, used as fallback

# Databases
DATABASE_URL         ← PostgreSQL Neon
REDIS_URL
MONGODB_URI / MONGODB_DB

# Live Engine (Delta trade cloning inside cmd/pre_live)
LIVE_ENGINE_SYMBOL           ← perp symbol (default BTCUSD)
LIVE_ENGINE_AUTO_ENABLE      ← arm on boot (default false; arm via POST /api/live/enable)
LIVE_ENGINE_MAX_CONTRACTS    ← per-order cap (default 5; 1 contract = 0.001 BTC)
LIVE_ENGINE_FIXED_CONTRACTS  ← constant order size override
LIVE_ENGINE_LEVERAGE         ← default 10

# Engine
INTERNAL_API_URL     ← Next.js → Go engine calls
PORT                 ← engine port (default 8080, Render 10000)
SQLITE_PATH
MAX_POSITION_BTC
MAX_DAILY_LOSS_PCT

# Auth
AUTH_JWT_SECRET      ← MongoDB JWT, raig_session httpOnly cookie

# Cron
CRON_SECRET          ← 64-char hex for /api/cron/paper-desk-tick

# Paper desk feature flags (30+)
NEXT_PUBLIC_DESK_*   ← slippage, vol-sizing, risk%, session gate, correlation gates
```

---

## Strategy Registry
- **Location**: `engine/internal/strategy/curated_registry.go`
- **Count**: 600+ live strategies
- **Families**: EMA Cross (15 variants), RSI threshold (8), RSI slope (5), Bollinger Band (12), Funding/CVD, Delta absorption, Liquidity sweep, FVG retest, Order block, MSS continuation, Microstructure, Volume profile
- **Removal policy**: Losing strategies removed (WINNERS_ONLY gate active since May 2026)
- **Fee model and expectancy formula**: Updated May 2026 — see memory for details

---

## Central TypeScript Types
- `client/src/lib/btcFuturesTrade.types.ts` — `BTCFuturesTrade` interface (exitReason: TP|SL|TIME|TRAIL|BREAKEVEN|LIQUIDATION_RISK|PROFIT_LOCK|MOM_DECAY)
- `client/src/lib/aiAppTracker/types.ts`
- `client/src/lib/verificationTrack/types.ts`

---

## Active Phase Work
| Phase | File | Status |
|-------|------|--------|
| Phase 13 | `PHASE13_INSTITUTIONAL_TRANSFORMATION_BLUEPRINT.md` | ~80% — OMS v3, risk gate, event bus, observability |
| Phase 14 | `PHASE14_PRODUCTION_INTEGRATION_GO_LIVE.md` | Active — wiring event bus→strategy→risk→OMS→execution |
| Phase 14 Recovery | `PHASE14_RECOVERY_RUNBOOK.md` | RPO <5min, RTO <15min. Kill switch + event store replay |

New engine modules (untracked): `engine/internal/killswitch/`, `engine/internal/ledger/`, `engine/internal/omsv3/`, `engine/internal/risk/gate/`, `engine/internal/reconciliation/`

---

## Graphify Knowledge Graph
Graphify is installed to reduce AI token consumption by providing a structured graph of the codebase.

Always start broad codebase questions from:

1. `.ai-context/README_FOR_AI.md`
2. `.ai-context/system-manifest.json`
3. `.ai-context/repository-summary.md`
4. `.ai-context/dependency-map.md`
5. `.ai-context/domain-model.md`
6. `.ai-context/business-rules.md`
7. `.ai-context/glossary.md`

Never scan the entire repository by default. Open source files only when implementation details are required. Prefer maps, summaries, ADRs, ownership metadata, and Graphify scoped queries over raw source.

```bash
# First-time setup (after cloning)
pip install graphifyy
npm run graphify:rebuild
npm run ai-context:install-hooks

# After changing code files (fast, no API cost)
npm run graphify:update

# Query the graph with a small default budget
npm run graphify:query -- "how does X connect to Y?"
python scripts/graphify_workflow.py query --scope client "where is the paper desk UI wired?"
python scripts/graphify_workflow.py query --scope engine-internal "how does risk gate connect to OMS?"

# More targeted graph tools
graphify path "SymbolA" "SymbolB"
graphify explain "concept"
```

The Cursor rule at `.cursor/rules/graphify.mdc` instructs the AI to consult `AI_CONTEXT.md` and Graphify before exploring raw files. Use scoped graphs (`client`, `engine-internal`, `engine-cmd`) whenever the subsystem is known; this keeps graph answers smaller and cheaper.

---

## Common Commands
```bash
# Frontend dev
cd client && npm run dev

# TypeScript check
node "c:/Trading apllication/client/node_modules/typescript/bin/tsc" --noEmit --project "c:/Trading apllication/client/tsconfig.json"

# Go build
cd engine && go build ./...

# Go tests
cd engine && go test ./...

# Engine run
cd engine && go run ./cmd/antigravity/main.go

# Backtest
cd engine && go run ./cmd/backtest/main.go
```

---

## Deployment (ALWAYS remember)
- **Frontend** → **Vercel** (Next.js, `client/`)
- **Go Engine** → **AWS Lightsail** (NOT Render — never reference Render)
- **Databases** → MongoDB Atlas, PostgreSQL Neon, Redis (all cloud-managed, not on AWS Lightsail)

---

## Hard Rules (Never Violate)
1. **Vercel cron limit**: Max 2 cron jobs total (root + client combined). Hobby plan — excess silently breaks webhooks. Always count before adding.
2. **No DB mocking in engine tests** — use real test DBs. Past incident: mocked tests passed, prod migration failed.
3. **WINNERS_ONLY gate** is active — don't re-add losing strategies.
4. **NSE/BSE market hours** differ from crypto (24/7). Always gate NSE strategies by session.
5. **Kill switch** (`engine/internal/killswitch/`) must remain wired — never bypass in prod paths.
