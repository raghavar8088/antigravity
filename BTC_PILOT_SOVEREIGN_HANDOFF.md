# BTC-PILOT SOVEREIGN — Full System Handoff Document
**Generated:** 2026-06-14  
**Purpose:** Complete system snapshot for rebuilding or upgrading this application in a new AI session.

---

## 1. Application Identity

| Field | Value |
|---|---|
| **Name** | BTC-PILOT SOVEREIGN (codebase name: `antigravity`) |
| **Version** | v1.1 |
| **Purpose** | Algorithmic BTC paper trading system with multi-signal AI decision layer, institutional-grade risk controls, and a full operator dashboard |
| **Paper Balance** | $1,000,000 USD starting capital |
| **Root Path** | `D:\APPLICATION\antigravity-main\antigravity-main\` |

---

## 2. Current Architecture (What Exists and Works)

### 2.1 Folder Structure

```
antigravity-main/
├── engine/               # Go 1.25 trading engine (40+ internal packages)
│   ├── cmd/antigravity/  # main.go — engine entrypoint
│   └── internal/
│       ├── ai/           # Multi-LLM clients (Claude, OpenAI, Gemini, Groq, Mistral, etc.)
│       ├── aiscoring/    # Async AI signal scorer + rule-based fallback
│       ├── alpha/        # CVD, funding, FVG, liquidations, orderblocks, MSS, volume profile
│       ├── backtest/     # Backtesting engine
│       ├── calibration/  # Signal weight calibration
│       ├── dominance/    # BTC dominance signal
│       ├── etf/          # Spot ETF flow signal
│       ├── execution/    # Binance + Delta adapters, paper OMS
│       ├── kelly/        # Half-Kelly position sizing
│       ├── killswitch/   # Emergency halt
│       ├── macro/        # Macro signal
│       ├── marketdata/   # Candles, ticks (Binance WebSocket + Coinbase)
│       ├── ml/           # ML model registry
│       ├── montecarlo/   # Monte Carlo simulation
│       ├── options/      # BTC options chain (Delta Exchange)
│       ├── orderbook/    # Order book depth analysis
│       ├── paperpersist/ # MongoDB trade persistence with retry/dead-letter queue
│       ├── pilot/        # Pilot aggregate — stage manager
│       ├── regime/       # Market regime classifier
│       ├── risk/         # Full risk engine (CVaR, VaR, drawdown, circuit breakers, etc.)
│       ├── sentiment/    # Sentiment score
│       ├── strategy/     # Strategy registry (CURRENTLY EMPTY — see Critical Bug)
│       └── trading/      # Signal aggregator, trading loop, paper persist hooks
├── client/               # Next.js 16 frontend
│   └── src/
│       ├── app/terminal/ # Main operator dashboard (30 sub-pages)
│       └── components/   # 60+ components
├── data/                 # Audit event logs (NDJSON)
├── runtime/              # Candles, logs, backtests, snapshots (git-ignored)
├── scripts/              # Build, deploy, validation scripts
├── grafana/              # 7 Prometheus/Grafana dashboards
└── nginx/                # Reverse proxy config
```

### 2.2 Engine Modules (All Built)

| Module | Status | Notes |
|---|---|---|
| Market Data Feed | Working | Binance WebSocket + Coinbase stream |
| Candle Aggregator | Working | 1m + 5m candles |
| Signal Aggregator | Working | 15s cooldown per strategy |
| AI Scoring (Async) | Working | 3 workers, 60s cache TTL |
| Rule-based Fallback | Working | Instant RSI/MACD/EMA/BB scorer |
| Kelly Position Sizer | Working | Half-Kelly + regime + data quality |
| Risk Engine | Working | CVaR, VaR, drawdown, circuit breakers |
| Paper OMS | Working | Full order lifecycle, MongoDB persistence |
| Paper Persist | Working | Retry queue + dead-letter queue |
| Kill Switch | Working | Hard halt wired in all execution paths |
| Options Engine | Working | Delta Exchange BTC options chain |
| Options Selling | Working | Separate $1M paper account |
| Regime Classifier | Working | Trending / Ranging / Volatile / Unknown |
| Funding Rate | Working | Perpetual swap funding collector |
| CVD Engine | Working | Cumulative Volume Delta + divergence |
| Liquidations | Working | Liquidation signal engine |
| BTC Dominance | Working | Dominance score |
| Spot ETF Flows | Working | ETF flow score |
| Sentiment | Working | Sentiment fetcher + score |
| Macro | Working | Macro fetcher + score |
| Order Book | Working | Depth + imbalance analysis |
| Backtesting | Working | Replay + walk-forward |
| Monte Carlo | Working | Risk of ruin simulation |
| Observability | Working | Prometheus metrics |
| Grafana Dashboards | Working | 7 dashboards (executive, OMS, risk, exchange, reconciliation, infra, security) |
| Reconciliation v2 | Working | Trade reconciliation |

### 2.3 Frontend Dashboard (All Built)

**Main hub:** `/terminal` — Operator Command Center

**Command Center tabs (8):**
- **Today** — Daily PnL, open positions, session KPIs
- **Health** — Engine health, storage health, data quality
- **Gates** — Go-live gates checklist
- **Signal Trace** — Signal flow, AI confidence, reasoning
- **Edge Lab** — Research edge metrics
- **OMS** — Order management, order blotter
- **Replay Lab** — Walk-forward replay
- **Advanced** — Admin, kill switch, settings

**Sub-pages under `/terminal`:**
- `/analytics`, `/backtest`, `/diagnostics`, `/events`, `/execution`
- `/grade-1` through `/grade-5` (certification quality levels)
- `/health`, `/journal`, `/main-engine`, `/mock-engine`
- `/observability`, `/portfolio`, `/portfolio-intelligence`
- `/research`, `/risk`, `/settings`, `/trading`

**Legacy:** `/mock-trading` — BTC Research Strategy Lab (60 research strategy entries, regime classification, equity curve, daily PnL history, Lightweight Charts visualizations)

**UI Stack:** Radix UI + TailwindCSS v4, TradingView Lightweight Charts v5

### 2.4 Signals & Indicators Computed

| Category | Signals |
|---|---|
| **Technical** | RSI-14 (1h), MACD histogram, EMA-9, EMA-21, ATR-14, Bollinger Band position (0–1) |
| **Alpha** | CVD, CVD Divergence, Funding Rate, FVG (Fair Value Gap), Liquidations, Liquidity, Microstructure, MSS (Market Structure Shift), Order Blocks, Volume Profile, Session (Asia/London/NY) |
| **Market Structure** | BTC Dominance, Spot ETF Flows, Options Chain (Delta Exchange), Order Book Depth, Open Interest Trend |
| **Macro & Sentiment** | Macro Score, Sentiment Score |
| **Regime** | Market regime (Trending / Ranging / Volatile / Unknown) |

### 2.5 AI Decision Layer

**Architecture:** Multi-agent LLM system
- **Bull Agent** — argues for long positions
- **Bear Agent** — argues for short positions  
- **Risk Agent** — Constitutional AI arbiter enforcing 17 inviolable rules

**Providers (multi-provider routing):** Claude (Anthropic), OpenAI, Gemini, Groq, Mistral, HuggingFace, OpenRouter

**Scoring flow:**
1. Signal generated by strategy
2. Submitted async to AI scorer (non-blocking, 256-item queue)
3. 3 worker goroutines score in background, populate 60s TTL cache
4. Execution path reads cache instantly (`GetCachedScore`) — zero blocking
5. If no cache hit: instant rule-based fallback (RSI/MACD/EMA/BB → confidence 0–100)

**Trading Constitution (17 rules enforced by Risk Agent):**
- Max 2% portfolio risk per trade
- Max 5% daily loss (veto all new trades if breached)
- Max 10% total open exposure
- Min 0.70 confidence to execute
- Min 10-word reasoning (quality gate)
- Min 2:1 reward-to-risk ratio
- Max 5 simultaneous open positions
- No trading in UNKNOWN regime
- Reduce size 50% in VOLATILE regime
- Kill switch overrides all AI decisions immediately

### 2.6 Paper Trading Logic

| Parameter | Value |
|---|---|
| **Starting capital** | $1,000,000 USD |
| **Position sizing** | Half-Kelly criterion with regime × data-quality multipliers |
| **Kelly floor** | Flat 5% sizing until 30 closed trades (insufficient history guard) |
| **Hard ceiling** | 10% of portfolio per position |
| **Max position size** | 0.50 BTC per trade |
| **Stop-loss range** | 0.10%–0.80% of entry price |
| **Take-profit** | Min 2× SL distance (2:1 R:R enforced) |
| **Max open positions** | 5 simultaneous |
| **Auto-execute** | `PAPER_TRADING_AUTO_EXECUTE=true` bypasses manual approval |
| **Exit types** | TP / SL / TIME / TRAIL / BREAKEVEN |
| **P&L formula** | `NetPnL = GrossPnL − EntryFee − ExitFee` |
| **Persistence** | MongoDB with retry queue + dead-letter queue (no silent drops) |

---

## 3. Critical Bug — Why Trades Are Not Placing

### Root Cause

On **2026-05-01**, a "Winners Only Gate" was activated. All strategy implementations were **deleted from the registry**. The functions that supply strategies to the trading loop now return empty lists:

```go
// engine/internal/strategy/curated_registry.go
func BuildCuratedScalpers() []RegistryEntry { return nil }
func FilterWinnersOnly(_ []RegistryEntry) []RegistryEntry { return nil }

// engine/internal/strategy/registry.go
func BuildAllScalpers() []RegistryEntry { return nil }

// engine/internal/strategy/curated_expansion_pack.go
func buildExpansionPack() []RegistryEntry { return nil }
```

In `main.go`:
```go
allStrategies := strategy.BuildCuratedScalpers()  // returns nil → empty
```

**Result:** The engine boots, connects to market data, and the signal aggregator runs — but `allStrategies` is empty so no signals are ever generated and no trades fire.

**Engine last working:** March 29, 2026 (logs show live paper trades executing normally — 0.01 BTC positions, TP/SL working, balance tracking correct).

**Fix required:** Re-implement and register concrete strategy implementations in the strategy package, then wire them back through `BuildCuratedScalpers()`.

### Other Issues

| Issue | Detail |
|---|---|
| Engine crash on start | `exit status 0xffffffff` in smoke/conservative logs — likely caused by missing `.env` or DB connection |
| No audit events today | `data/audit/2026-06-14-events.ndjson` does not exist — engine hasn't run today |
| `brain/` and `bridge/` directories | Referenced in docs but absent in this checkout |

---

## 4. Tech Stack Snapshot

### Backend Engine
| Component | Version/Detail |
|---|---|
| Language | Go 1.25 |
| HTTP | Standard `net/http` + gorilla/websocket |
| Databases | PostgreSQL (pgx/v5), MongoDB Atlas (mongo-driver/v2), SQLite (modernc.org/sqlite) |
| AWS | AWS SDK v2 (Secrets Manager) |
| Metrics | Prometheus client_golang |
| Deployment | Docker on AWS (600MB RAM limit, 1.5 CPU) |

### Frontend
| Component | Version/Detail |
|---|---|
| Framework | Next.js 16.2.1 |
| React | 19.2.4 |
| CSS | TailwindCSS v4 |
| UI Components | Radix UI |
| Charts | TradingView Lightweight Charts v5 |
| Auth | Supabase SSR + JWT (raig_session cookie) |
| Database client | MongoDB (paper trades), PostgreSQL (pg) |
| Deployment | Vercel |
| Node | ≥20.0.0 |

### Python Layer
| Component | Version/Detail |
|---|---|
| API | FastAPI 0.111.1 + Uvicorn 0.23.2 |
| AI SDK | anthropic ≥0.25.0 |
| Indicators | `ta` 0.11.0 (RSI, MACD, EMA, ATR, BB) |
| Exchange | `ccxt` 3.0.89, `python-binance` 1.0.17 |
| Data | numpy 1.26.4, pandas 2.2.3, scipy 1.11.4 |

### AI/LLM Providers
- Claude (Anthropic) — Bull/Bear agents, Risk agent
- OpenAI (gpt-4o-mini for Bull/Bear, gpt-4o for Risk arbitration)
- Gemini, Groq, Mistral, HuggingFace, OpenRouter (multi-provider fallback routing)

### Exchange Integrations
- **Binance** — Primary (API key + secret, WebSocket feed)
- **Coinbase** — Market data feed (WebSocket)
- **Delta Exchange India** — BTC options chain (live + paper)

### Monitoring
- Prometheus + Grafana (7 dashboards: executive, OMS, risk, exchange, reconciliation, infrastructure, security)
- Alertmanager configured

---

## 5. Current Gaps & Bugs

### P0 — Blocking (No Trades Firing)
1. **All strategies deleted from registry** — `BuildCuratedScalpers()` returns `nil`. Must re-implement concrete strategy logic (previously included: `TripleFilter_Alpha_Scalp`, `ADX_Trend_Scalp`, `AdaptiveRSI_Dynamic_Scalp`, and others visible in March 2026 logs).
2. **Engine crash on start** — `exit status 0xffffffff` suggests missing environment variables or DB connection failure. The `.env` file must be present with valid `DATABASE_URL`, `MONGODB_URI`, `BINANCE_API_KEY`.

### P1 — Important
3. **No live session since June 13** — Last audit log shows only `SYSTEM_START` events with no trades since early June. Engine may not be staying up.
4. **Winners Only Gate blocks all strategy registration** — Even if new strategies are added, `FilterWinnersOnly()` must be updated or the gate logic must be properly re-implemented to actually filter by performance rather than returning nil.

### P2 — Known Limitations
5. `brain/` and `bridge/` directories referenced in docs but missing from this checkout.
6. Graphify `merge-graphs` has a NetworkX compatibility issue (workaround: `merge_graphs.py`).

---

## 6. Upgrade Goals

The application owner is **open to all new features and improvements** in the rebuild. Based on the codebase state, the highest-value upgrades are:

### Immediate (Fix Core Functionality)
1. **Restore strategy implementations** — Re-implement at minimum 3–5 proven scalping strategies (ADX trend, RSI adaptive, triple filter alpha) and wire them back into the registry
2. **Fix Winners Only Gate** — Replace the null-returning gate with a real performance filter that keeps strategies scoring above a win-rate/Sharpe threshold
3. **Fix engine startup** — Ensure `.env` validation on boot with clear error messages for missing required vars

### Short-Term Improvements
4. **Telegram / Discord alerts** — Trade entry/exit notifications, daily PnL summary, kill switch triggers
5. **Live trading mode toggle** — Safe path from paper → live Binance execution (infrastructure already exists in `execution/binance_live.go`)
6. **Strategy performance dashboard** — Real-time win rate, Sharpe, max drawdown per strategy visible in the frontend
7. **Smarter AI provider routing** — Auto-fallback chain: Groq (fastest) → OpenAI → Claude, with latency tracking

### Longer-Term
8. **Walk-forward strategy validation** — Automated promotion/demotion of strategies based on live performance
9. **Multi-asset expansion** — ETH, SOL paper trading alongside BTC
10. **Mobile dashboard** — `/mobile` route already exists in the frontend; needs completion

---

## 7. How to Start a New Session

Read these files first (in order):
1. This document (`BTC_PILOT_SOVEREIGN_HANDOFF.md`)
2. `.ai-context/README_FOR_AI.md`
3. `.ai-context/session/current-work.md`
4. `.ai-context/architecture.md`

Then go directly to the critical bug fix:
- **File:** `engine/internal/strategy/curated_registry.go`
- **File:** `engine/internal/strategy/registry.go`
- **File:** `engine/internal/strategy/curated_expansion_pack.go`
- **Reference:** `engine/cmd/antigravity/main.go` line with `allStrategies := strategy.BuildCuratedScalpers()`
- **Goal:** Re-populate `BuildCuratedScalpers()` with concrete strategy implementations

The last known-working strategies (from March 29, 2026 logs):
- `TripleFilter_Alpha_Scalp`
- `ADX_Trend_Scalp`
- `AdaptiveRSI_Dynamic_Scalp`
