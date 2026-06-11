# SYSTEM ARCHITECTURE REPORT — Forensic Audit
# Date: 2026-06-11 | Auditor: Claude Code

---

## 1. HIGH-LEVEL ARCHITECTURE (ASCII)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         TRADING PLATFORM OVERVIEW                           │
│                                                                             │
│  ┌──────────────┐        ┌──────────────────┐       ┌─────────────────┐   │
│  │   BROWSER    │◄──────►│  NEXT.JS (Vercel)│◄─────►│   Go Engine     │   │
│  │   (React)    │  HTTPS │  client/src/      │  HTTP │  (AWS Lightsail)│   │
│  └──────────────┘        │  Port :3000       │       │  Port :8080     │   │
│                          └──────────┬────────┘       └────────┬────────┘   │
│                                     │                          │            │
│                          ┌──────────▼──────────────────────────▼────────┐  │
│                          │              DATABASES                         │  │
│                          │  ┌────────────┐  ┌──────────┐  ┌──────────┐ │  │
│                          │  │ MongoDB    │  │ Postgres │  │ SQLite   │ │  │
│                          │  │ Atlas      │  │ (Neon)   │  │ (local)  │ │  │
│                          │  │ loop_trades│  │ ledger   │  │ engine.db│ │  │
│                          │  └────────────┘  └──────────┘  └──────────┘ │  │
│                          └───────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                    EXTERNAL MARKET DATA                               │  │
│  │  Coinbase WS    Binance REST    Delta Exchange    AngelOne    NSE     │  │
│  │  (BTC price)    (BTC fallback)  (options/live)   (NIFTY)     (index) │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. GO ENGINE BOOT SEQUENCE (engine/cmd/antigravity/main.go)

```
main()
 ├─ loadDotEnv()                         [line 399] — reads ../../.env
 ├─ production.RunBootGate()             [line 401] — pre-flight checks
 ├─ marketdata.NewCoinbaseClient()       [line 419] — WebSocket BTC-USD
 ├─ marketdata.NewNSEIndexClient()       [line 420]
 ├─ marketdata.NewDeltaTickerClient()    [line 422]
 ├─ marketdata.NewAngelOneClient()       [line 423]
 ├─ strategy.BuildCuratedScalpers()      [line 435] — loads 600+ strategies
 ├─ go FundingCollector()               [line 458] — 8h Binance funding rate
 ├─ risk.NewRiskEngine(profile)         [line 497] — $1M cap, 5% daily loss
 ├─ risk.NewStrategyTracker()           [line 507]
 ├─ execution.NewPaperClient($1M)       [line 512]
 ├─ execution.NewPaperOMS($1M)          [line 517]
 ├─ positions.NewManager()              [line 522]
 ├─ trading.NewSignalAggregator(15s)    [line 527]
 ├─ execution.NewTradeJournal(5000)     [line 533]
 ├─ marketdata.NewCandleAggregator()    [line 538]
 ├─ persistence.NewStore(ctx)           [line 544] — PostgreSQL (Neon) + SQLite
 ├─ paperpersist.NewMongoManager(ctx)   [line 648] — MongoDB Atlas
 ├─ trading.NewOrchestrator()           [line 723]
 ├─ killswitch.NewService()             [line 765]
 ├─ reconciliationv2.WireProduction()   [line 936]
 ├─ options.NewEngine()                 [line 954] — BTC options
 ├─ options_selling.NewEngine()         [line 955]
 ├─ options.NewNiftyEngine()            [line 956]
 ├─ options_selling.NewNiftyEngine()    [line 957]
 ├─ delta.NewBridge()                   [line 961]
 ├─ niftystocks.NewEngine()             [line 985]
 ├─ go OptionsPriceFeed()              [line 1146] — Delta/Binance BTC price
 ├─ go Nifty50PriceFeed()              [line 1245] — NSE/AngelOne NIFTY price
 ├─ http.HandleFunc() x 50+            [line 1380+] — register all HTTP handlers
 ├─ go keepAlive()                     [line 2157] — self-ping every 2 min
 └─ signal.Notify() → shutdown          [line 2161]
```

---

## 3. DATA FLOW DIAGRAM (ASCII)

```
MARKET DATA INGESTION
═════════════════════
Coinbase WS ──────────────────────┐
                                  │ BTC-USD tick
Binance REST ─────────────────────┤ (fallback every 10s)
                                  ▼
                          CandleAggregator
                          (1m + 5m bars)
                                  │
              ┌───────────────────┴──────────────────┐
              │                                       │
              ▼                                       ▼
    Tick-based Strategies                  Candle-based Strategies
    (600+ in parallel)                     (1m / 5m bars)
              │                                       │
              └────────────────┬──────────────────────┘
                               │ Signal{Side, Price, SL, TP, Confidence}
                               ▼
                    SignalAggregator (15s cooldown)
                               │
                               ▼
                  ┌─────────────────────────┐
                  │  PreTradeRiskPipeline    │
                  │  ├─ KillSwitch.IsActive │
                  │  ├─ PMSBudget gate      │
                  │  ├─ RiskEngine check    │
                  │  └─ ConfidenceThreshold │
                  └───────────┬─────────────┘
                              │ approved
                              ▼
                    PaperClient.Execute()
                              │
                              ▼
              ┌───────────────────────────────┐
              │       PositionManager         │
              │  TrailingStop / TakeProfit    │
              └───────────────┬───────────────┘
                              │ on close
                              ▼
                    TradeJournal.Record()
                              │
              ┌───────────────┴──────────────┐
              │                              │
              ▼                              ▼
    PostgreSQL (Neon)              MongoDB Atlas
    trades table                   paper_trades
    via dbStore.SaveTrade()        via TradeWriter
              │                              │
              └───────────────┬──────────────┘
                              │
                              ▼
                   Next.js API Routes
                   /api/paper-desk/*  ← polls every 5s from browser
```

---

## 4. CONTROL FLOW DIAGRAM (ASCII) — Kill Switch

```
Normal Trading Loop
        │
        ├──► KillSwitch.IsActive() ──YES──► BLOCK (return)
        │
        ▼ NO
   Execute Trade
        │
        ▼
ReconciliationV2 (runs every 5min)
        │
        ├─ drift > threshold? ──YES──► ksSvc.Trigger(OMS_DESYNC)
        │                               └──► ActionBlockNewOrders
        │
        ▼ NO
  Continue trading

KillSwitch Triggers:
  TriggerDailyLoss          DAILY_LOSS_BREACH
  TriggerExchangeOutage     EXCHANGE_OUTAGE
  TriggerDataFeedOutage     DATA_FEED_OUTAGE
  TriggerOMSDesync          OMS_DESYNC
  TriggerRiskServiceFailure RISK_SERVICE_FAILURE
  TriggerPositionDrift      LARGE_POSITION_DRIFT
  TriggerFundingShock       FUNDING_SHOCK
  TriggerLiquidationSpike   LIQUIDATION_EVENT_SPIKE
  TriggerManualOperator     MANUAL_OPERATOR_TRIGGER

Actions: CANCEL_OPEN_ORDERS | BLOCK_NEW_ORDERS | FLATTEN_POSITIONS | SEND_ALERTS
```

---

## 5. SERVICES INVENTORY

### Go Engine Services (engine/internal/)

| Package | Purpose | File |
|---------|---------|------|
| `trading` | Orchestrator — main event loop, strategy dispatch, signal routing | `loop.go` |
| `execution` | PaperClient (fills), PaperOMS (canonical), BinanceLive, TradeJournal | `paper.go`, `binance_live.go` |
| `strategy` | 600+ strategy implementations + registry | `curated_registry.go`, `elite_v2.go`, `elite_v3.go`, etc. |
| `risk` | RiskEngine, StrategyTracker, Heat/VaR/CVaR/DrawdownScaling | `heat.go`, `var.go`, `cvar.go` |
| `positions` | PositionManager (open positions, trailing stop, TP) | `manager.go` |
| `marketdata` | CoinbaseClient (WS), NSEIndexClient, DeltaTickerClient, AngelOneClient, CandleAggregator, warmup | `coinbase.go`, `nse.go`, `delta.go` |
| `options` | BTC options buy engine (50 strategies, $1M paper) | `engine.go` |
| `options_selling` | BTC options write/sell engine | `engine.go` |
| `killswitch` | Service — trigger/release/restore from ledger | `service.go` |
| `ledger` | Append-only event store (PostgreSQL + memory fallback) | `store.go`, `postgres_store.go` |
| `omsv3` | OMS v3 — event-sourced order+position aggregates | `aggregate.go`, `events.go` |
| `reconciliationv2` | Drift detection vs exchange + position manager, kill switch hook | `authority.go`, `wiring.go` |
| `paperpersist` | MongoDB Atlas writers — TradeWriter, OrderWriter, StateSnapshotter, EquityRecorder, StrategyHealthMonitor | multiple files |
| `persistence` | SQLite store (engine.db) + PostgreSQL state | `store.go` |
| `ai` | MultiAgent (OpenAI, Gemini, Groq, Mistral, HuggingFace, Cloudflare, OpenRouter) | `agents.go`, individual provider files |
| `delta` | Delta Exchange live bridge (mirrors options signals) | `client.go`, `bridge_buy_sizing.go` |
| `niftystocks` | NIFTY 50 stocks engine | `engine.go` |
| `pms` | Portfolio Management System (heat/VaR/drawdown budget) | part of `trading` package import |
| `regime` | Market regime engine (TREND/RANGE/VOLATILE/MIXED) | `engine.go` |
| `events` | Internal event bus | `bus.go` |
| `oms` | OMS v1/v2 (older) | `order.go`, `manager.go` |
| `observability` | Prometheus metrics | `metrics.go` |
| `security` | Zero Trust gate (authn/RBAC/audit) | loaded via `security.LoadPolicy()` |
| `admin` | KillSwitch HTTP handler | wired in main.go |
| `execintel` | Execution intelligence tracker (Phase 22D) | separate package |

### Background Goroutines (engine/cmd/antigravity/main.go)

| Name | Interval | Purpose |
|------|----------|---------|
| `Orchestrator` | tick-driven | Main trading loop |
| `FundingCollector` | 8h | Binance funding rate injection |
| `OptionsPriceFeed` | 1s | BTC price → Delta/Binance |
| `Nifty50PriceFeed` | 15s/60s | NIFTY price → NSE/AngelOne |
| `OptionsScalper` | minute-driven | BTC options buy engine |
| `OptionsSellingScalper` | minute-driven | BTC options sell engine |
| `NiftyOptionsScalper` | minute-driven | NIFTY options buy engine |
| `NiftyOptionsSellingScalper` | minute-driven | NIFTY options sell engine |
| `NiftyStocksEngine` | (price-driven) | NIFTY stocks scalper |
| `NiftyOptionsWarmup` | once at boot | Yahoo Finance warmup |
| `StateSaver` | periodic | PostgreSQL/SQLite snapshots |
| `DailyLossReset` | midnight UTC | Reset daily PnL counters |
| `StateSnapshotter` | 10s | MongoDB paper_state snapshot |
| `EquityRecorder` | 1m | MongoDB equity curve |
| `PortfolioMetricsWriter` | 30m | MongoDB portfolio metrics |
| `StrategyHealthMonitor` | 15m | MongoDB strategy health |
| `ExecutionWatchdog` | continuous | Detects stale market data |
| `keepAlive` | 2m | Self-ping /health |
| `mongoMgr.RunPingMonitor` | 30s | MongoDB reconnect monitor |

---

## 6. DATABASE INVENTORY

### MongoDB Atlas (`MONGODB_URI`, db: `loop_trades`)

| Collection | Written By | Read By | Purpose |
|-----------|-----------|--------|---------|
| `paper_trades` | Go engine TradeWriter (paperpersist) | Next.js `/api/paper-desk/trades` | Closed BTC paper trades |
| `paper_positions` | Go engine | Next.js `/api/paper-desk/positions` | Open positions |
| `paper_state` | StateSnapshotter (10s) | Next.js `/api/paper-desk/state` | Account balance, equity, drawdown |
| `paper_orders` | OrderWriter | Next.js `/api/paper-desk/orders` | OMS order events |
| `equity_curve` | EquityRecorder (1m) | Next.js `/api/paper-desk/equity` | 1m equity snapshots |
| `strategy_health` | StrategyHealthMonitor (15m) | Next.js `/api/paper-desk/strategy-health` | Per-strategy score, status |
| `portfolio_metrics` | PortfolioMetricsWriter (30m) | Next.js `/api/paper-desk/portfolio` | Aggregate risk metrics |
| `daily_pnl` | EquityRecorder | equity route | Daily P&L seal |
| `mock_trades` | Next.js `/api/mock-trading/*` | mock-trading routes | Frontend-only simulation trades |
| `mock_account_state` | Next.js `/api/mock-trading/account` | mock-trading routes | Frontend simulation state |
| `strategy_signals` | Next.js `/api/mock-trading/signals` POST | `/api/mock-trading/signals` GET | Research signal snapshots |
| `worker_events` | Next.js cron/VPS worker | desk-worker-events route | Worker heartbeat/error logs |
| `verification_track` | `/api/verification-track/events` POST | verification-track routes | Phase verification log |
| `ai_tracker` | `/api/ai-app-tracker/capture` POST | ai-app-tracker routes | AI app activity tracker |

### PostgreSQL / Neon (`DATABASE_URL`)

| Table | Written By | Purpose |
|-------|-----------|---------|
| `engine_state` | persistence.Store (StateSaver) | BTC paper account snapshot |
| `trades` | journal.OnTrade hook | Relational trade history |
| `options_state` | saveOptionsSnapshot | BTC options engine state |
| `nifty_options_state` | saveNiftyOptionsSnapshot | NIFTY options engine state |
| `options_selling_state` | saveOptionsSellingSnapshot | BTC options selling state |
| `nifty_options_selling_state` | saveNiftyOptionsSellingSnapshot | NIFTY options selling state |
| `kill_switch_events` | ledger.PostgresStore | Durable kill switch log |
| `oms_events` | ledger.PostgresStore | OMS event ledger |

### SQLite (`SQLITE_PATH=./data/engine.db`)

Used as fallback when PostgreSQL is unavailable. Same schema as Postgres state tables.
File: `engine/internal/persistence/store.go` lines 94–100.

### Redis (`REDIS_URL`)

Referenced in CLAUDE.md as "indicator cache, performance cache." No direct source evidence found in audited files — likely used by separate Python `brain/` service or older code path.

---

## 7. EXTERNAL DEPENDENCIES

| Service | Protocol | Used By | Auth |
|---------|---------|---------|------|
| Coinbase Advanced Trade | WebSocket | engine/internal/marketdata/coinbase.go | None (public) |
| Binance REST | HTTPS | engine/cmd/antigravity/main.go:fetchBinanceBTCSpot | BINANCE_API_KEY/SECRET |
| Delta Exchange | REST | engine/internal/marketdata/delta.go + delta/client.go | DELTA_API_KEY/SECRET |
| AngelOne Smart API | REST | engine/internal/marketdata/nse.go (AngelOneClient) | ANGELONE_* keys |
| NSE India (public) | REST | engine/internal/marketdata/nse.go | None |
| Yahoo Finance v8 | HTTPS | client/src/app/api/nifty/stream/route.ts | None |
| OpenAI | HTTPS | engine/internal/ai/openai.go | OPENAI_API_KEY |
| Gemini | HTTPS | engine/internal/ai/gemini.go | GEMINI_API_KEY |
| Groq | HTTPS | engine/internal/ai/groq.go | GROQ_API_KEY |
| OpenRouter | HTTPS | engine/internal/ai/openrouter.go | OPENROUTER_API_KEY |
| Mistral | HTTPS | engine/internal/ai/mistral.go | (key) |
| HuggingFace | HTTPS | engine/internal/ai/huggingface.go | (key) |
| Cloudflare Workers AI | HTTPS | engine/internal/ai/cloudflare.go | (key) |
| MongoDB Atlas | TCP/TLS | engine paperpersist + Next.js mongoTradesClient | MONGODB_URI |
| PostgreSQL Neon | TCP/TLS | engine ledger + persistence | DATABASE_URL |
| HashiCorp Vault | HTTPS | engine/internal/security/vault | VAULT_ADDR/TOKEN |

---

## 8. WEBSOCKET / SSE CONNECTIONS

| Type | Direction | Endpoint | File |
|------|----------|---------|------|
| WebSocket | Engine → Coinbase | wss://advanced-trade-ws.coinbase.com | engine/internal/marketdata/coinbase.go |
| SSE | Browser → Next.js | `/api/nifty/stream` | client/src/app/api/nifty/stream/route.ts |
| Polling | Browser → Next.js | `/api/paper-desk/snapshot` every 5s | client/src/hooks/usePaperDesk.ts |

Note: No SSE from Next.js to Go engine found. The engine uses polling architecture (per `usePaperDesk.ts` comment: "Vercel serverless kills SSE at platform timeout").

---

## 9. CRON JOBS

| Cron | Schedule | Defined In | Purpose |
|------|---------|-----------|---------|
| `/api/cron/rank-strategies` | `0 0 * * *` (midnight UTC) | vercel.json (root) | Nightly strategy ranking from MongoDB |
| `/api/cron/policy-snapshot` | `5 0 * * *` (12:05 UTC) | vercel.json (root) | Policy snapshot |
| `/api/cron/paper-desk-tick` | Every 1 min (backup only) | Code references vercel.json; NOT in root vercel.json crons array | Safety-net backup tick when VPS worker is stale |

FINDING: vercel.json root has only 2 crons (rank-strategies, policy-snapshot). The paper-desk-tick route EXISTS but is NOT in the current vercel.json cron configuration. It references being in vercel.json but is not listed.

---

## 10. DEPLOYMENT TOPOLOGY

```
Browser ──HTTPS──► Vercel (Next.js)
                        │
                        ├── INTERNAL_API_URL ──HTTP──► AWS Lightsail (Go Engine :8080)
                        │
                        ├── MONGODB_URI ──TLS──► MongoDB Atlas
                        │
                        └── DATABASE_URL ──TLS──► PostgreSQL Neon
                        
Go Engine ──WS──► Coinbase
           ──REST──► Binance / Delta / AngelOne / NSE / Yahoo Finance
           ──TLS──► MongoDB Atlas (paperpersist)
           ──TLS──► PostgreSQL Neon (ledger + persistence)
```
