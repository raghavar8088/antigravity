# RAIG AUTONOMOUS TRADING ENGINE — COMPLETE TECHNICAL KNOWLEDGE TRANSFER
**Classification:** Principal Engineer Handover Document  
**Date:** 2026-06-01  
**Engine Version:** v6.0 (Phase 14)  
**Basis:** Live codebase inspection — 257 Go files, 101 Next.js API routes, Python service, full DB schema  
**Author:** Lead Architect Review

---

## SECTION 1 — EXECUTIVE SUMMARY

### What the Application Does
A multi-market algorithmic trading platform that combines a high-performance Go execution engine, a Next.js dashboard, a Python strategy service, and a multi-model AI consensus council. It continuously evaluates 100+ trading strategies against live market data, routes approved signals through a multi-layer risk pipeline, and executes paper or live trades.

### Primary Business Purpose
Systematic alpha generation on BTC perpetual futures (primary), NIFTY 50 options (secondary), Delta Exchange BTC options (secondary), and NSE equities (tertiary). The platform's research tournament promotes the highest-edge strategies from paper trading into a live-eligible approved list using a statistically rigorous scoring system.

### Core Workflows
1. **Live Market Data** → tick ingestion from Binance WebSocket (BTC) or Coinbase/Delta/Angel One → strategy evaluation → signal generation
2. **Signal Pipeline** → cooldown filter → AI agent council (Bull/Bear/Macro/Risk) → pre-trade risk gate → paper OMS execution
3. **Research Tournament** → daily cron ranks closed paper trades by strategy → awards PROMOTE/KEEP/WATCH/DISABLE/INSUFFICIENT verdicts → approved list gates live fills
4. **Operations** → kill switch monitors 8 trigger conditions → reconciliation service compares OMS vs exchange state → telemetry publishes Prometheus metrics

### Main Capabilities
- 100+ price-based scalping strategies (EMA, RSI, Bollinger, VWAP, Ichimoku, Order Flow, etc.)
- 9 new Phase-11 microstructure alpha modules (CVD, Funding Rate, FVG, Liquidity Sweep, Liquidation Cascade, MSS, Order Block, Volume Profile, Delta Divergence)
- Multi-model AI consensus council (GPT-4o, GPT-4o-mini, Gemini Flash 2.0, Groq Llama-3-70b) with 6-provider fallback chain
- ML classifier (engine/internal/ml/) as LLM replacement path
- BTC options scalper (50 strategies, buy + sell engines)
- NIFTY 50 options scalper (session-gated)
- NSE equities feed (Angel One SmartAPI)
- Event-sourced order ledger (OMS v3) with full reconciliation
- Kill switch with 8 trigger types and 4 action types
- Research edge scoring with statistically rigorous verdict logic
- Full paper trading simulator (client-side + Go engine)
- MongoDB + SQLite + PostgreSQL/TimescaleDB + Redis persistence stack

### Trading Style
Scalping / high-frequency — 1-minute to 5-minute timeframes. Hold times 5–45 minutes. SL geometry 0.12–0.22%, TP geometry 0.20–0.75%. Target R:R ≥ 2.40.

### Markets Supported
- BTC-USD / BTCUSDT / BTC-USDT (Binance Futures, Coinbase, Delta Exchange India)
- NIFTY 50 options (Angel One / NSE)
- Delta Exchange BTC options (options buying + selling engines)
- MCX commodities (crude oil, gold — data feed only)
- NSE equities (Angel One — NIFTY 50 stocks)

### Live Trading Support
**Yes** — Binance live client (HMAC-SHA256 signed orders) and Delta Exchange live bridge (live_bridge.go) are implemented. Angel One equities orders are also routed live via SmartAPI. Live trading is mode-switched via environment flag.

### Paper Trading Support
**Yes** — Paper OMS (engine/internal/execution/paper_oms.go) with $1,000,000 virtual account. Client-side mock engine (client/src/lib/mockTradingEngine.ts) provides a second independent paper simulation. Both implement identical fee and slippage models.

### Current Maturity Level
**Advanced Prototype / Pre-Production.** All core execution paths are implemented and tested. The Phase 14 institutional infrastructure (OMS v3, ledger, kill switch, reconciliation, Redis cache, TimescaleDB schema) is complete. Integration between new modules and the main trading loop is partially verified. Security hardening is absent — blocking live deployment.

### Institutional Readiness Estimate
**61 / 100** (up from 47 at Phase 1). See Section 19 for full breakdown.

---

## SECTION 2 — COMPLETE REPOSITORY STRUCTURE

```
c:\Trading apllication\
├── engine/                          ← Go trading engine (primary backend)
│   ├── cmd/
│   │   ├── antigravity/
│   │   │   └── main.go              ← 1,881 lines — boot, HTTP server, all subsystems
│   │   └── backtest/
│   │       └── main.go              ← Standalone backtester entry point
│   ├── go.mod                       ← Go 1.25.0, 15 deps (pgx, gorilla/websocket, prometheus, sqlite)
│   ├── go.sum
│   └── internal/
│       ├── admin/                   ← Kill switch HTTP handler (delegates to killswitch/)
│       ├── ai/
│       │   ├── agents.go            ← Multi-agent orchestrator, 5 agent definitions + prompts
│       │   ├── constitution.go      ← Inviolable trading rules enforced at veto layer
│       │   └── strategy_library.go  ← AI strategy definitions (system prompts per strategy class)
│       ├── alpha/                   ← Phase 11 microstructure alpha engine
│       │   ├── types.go             ← Signal, Candle, Action types shared across alpha modules
│       │   ├── math.go              ← Shared math utilities (EMA, ATR, Clamp)
│       │   ├── alpha_engine_test.go ← Integration tests for alpha modules
│       │   ├── ai_training_data/
│       │   │   └── dataset.go       ← Training data pipeline for ML classifier
│       │   ├── cvd/
│       │   │   ├── cvd.go           ← CVD accumulation logic
│       │   │   ├── cvd_cache.go     ← Time-windowed CVD series cache
│       │   │   ├── cvd_divergence.go← Divergence detection algorithm
│       │   │   └── cvd_strategy.go  ← Signal generation (SL 0.30%, TP 0.75%)
│       │   ├── delta/
│       │   │   ├── delta_engine.go  ← Candlestick delta aggregation
│       │   │   └── delta_strategy.go← Delta divergence signal
│       │   ├── funding/
│       │   │   ├── funding_cache.go ← Rolling funding rate cache
│       │   │   ├── funding_collector.go ← Binance /fapi/v1/fundingRate poller
│       │   │   ├── funding_engine.go← Mean-reversion decision engine
│       │   │   └── funding_strategy.go ← Signal: short when >0.1%, long when <-0.05%
│       │   ├── fvg/
│       │   │   ├── fvg_detector.go  ← 3-candle imbalance zone detection
│       │   │   └── fvg_strategy.go  ← FVG fill signal generation
│       │   ├── liquidations/
│       │   │   └── liquidations_engine.go ← Liquidation cascade detection
│       │   ├── liquidity/
│       │   │   ├── liquidity_engine.go ← Liquidity pool mapping
│       │   │   ├── liquidity_levels.go ← Round-number + swing-high/low levels
│       │   │   └── sweep_detector.go   ← Stop-hunt → reversal detector
│       │   ├── microstructure/
│       │   │   ├── engine.go        ← Bid/ask imbalance, tick clustering engine
│       │   │   ├── strategy.go      ← Microstructure signal generation
│       │   │   ├── types.go         ← Microstructure-specific types
│       │   │   └── microstructure_test.go
│       │   ├── mss/
│       │   │   └── mss_engine.go    ← Break of Structure / Change of Character detection
│       │   ├── orderblock/
│       │   │   └── orderblock_engine.go ← Last candle before strong move detection
│       │   ├── quality/
│       │   │   └── quality_engine.go ← Pre-filter signal quality scoring
│       │   ├── session/
│       │   │   └── (session-aware activation — Asia/London/NY ranges)
│       │   └── volumeprofile/
│       │       └── (VPOC, HVN, LVN computation)
│       ├── backtest/
│       │   ├── engine.go            ← 91-line historical replay engine
│       │   └── metrics.go           ← ROI, win rate (40 lines)
│       ├── delta/
│       │   ├── live_bridge.go       ← Delta Exchange live options bridge (signed API calls)
│       │   └── ticker_client.go     ← Public price feed (no auth required)
│       ├── events/
│       │   ├── bus.go               ← Non-blocking pub/sub event bus (19 event types)
│       │   ├── router.go            ← Route events to subscribers by type
│       │   └── engine.go            ← Event engine lifecycle management
│       ├── exchange/                ← Exchange abstraction layer
│       ├── execution/
│       │   ├── paper.go             ← Virtual wallet (balance tracking)
│       │   ├── paper_oms.go         ← Paper OMS — 7 exit types, trailing/breakeven/liquidation
│       │   ├── routing.go           ← Route order by mode (MARKET/POST_ONLY/IOC) per category+regime
│       │   ├── binance_live.go      ← Binance Futures HMAC-SHA256 signed order placement
│       │   └── trade_journal.go     ← Local execution log (append-only)
│       ├── killswitch/
│       │   ├── service.go           ← Emergency halt (8 triggers, 4 actions)
│       │   └── service_test.go
│       ├── ledger/
│       │   ├── event.go             ← 50+ event types, AggregateType enum
│       │   ├── order_projection.go  ← Projects current order state from event stream
│       │   ├── store.go             ← Event store interface + implementation
│       │   └── store_test.go
│       ├── marketdata/
│       │   ├── client.go            ← MarketDataClient interface + Tick struct
│       │   ├── angelone.go          ← Angel One SmartAPI REST poller (NIFTY + stocks)
│       │   ├── candle_aggregator.go ← Build OHLCV 1m/5m from tick stream
│       │   └── (coinbase, delta, nse connectors)
│       ├── ml/
│       │   └── ml_classifier.go     ← Trained classifier to replace LLM council calls
│       ├── niftystocks/             ← NSE equity strategies (50 strategies)
│       ├── oms/                     ← Additional OMS utilities
│       ├── omsv3/
│       │   ├── aggregate.go         ← OrderAggregate state machine (9 states, 10 transitions)
│       │   ├── aggregate_test.go
│       │   ├── pipeline.go          ← OMS v3 processing pipeline
│       │   └── pipeline_test.go
│       ├── options/                 ← BTC options scalper (50 strategies, buy engine)
│       ├── options_selling/         ← BTC options writer engine
│       ├── performance/
│       │   ├── analytics.go         ← Performance analytics module
│       │   ├── indicator_cache.go   ← Computed indicator cache (avoids recomputation)
│       │   ├── market_data_bus.go   ← Decouples tick ingestion from strategy evaluation
│       │   ├── ml_classifier.go     ← Performance-layer ML integration
│       │   ├── redis_cache.go       ← Redis-backed cache (AI decisions, rankings)
│       │   ├── runtime.go           ← Runtime performance monitoring
│       │   └── strategy_scheduler.go← Batches strategy evaluation (not every tick)
│       ├── persistence/
│       │   ├── store.go             ← 784 lines — SQLite/Postgres state snapshots
│       │   ├── saver.go             ← 15-second periodic save goroutine
│       │   └── file_snapshot.go     ← Atomic JSON writes for options state
│       ├── positions/
│       │   └── manager.go           ← Position lifecycle + CloseEvent emission
│       ├── reconciliation/
│       │   ├── service.go           ← Periodic reconciliation check (orders + positions + balance)
│       │   ├── detectors.go         ← OrderMismatchDetector, PositionDriftDetector, BalanceDriftDetector
│       │   ├── detectors_test.go
│       │   └── sync.go              ← Reconciliation sync / correction actions
│       ├── regime/
│       │   └── engine.go            ← Market regime classifier (TRENDING/RANGING/HIGH_VOL/NO_TRADE)
│       ├── research_ai/             ← Research-grade signal generation
│       ├── risk/
│       │   ├── engine.go            ← 5-gate validation (symbol, size, capital, daily loss, confidence)
│       │   ├── strategy_tracker.go  ← Per-strategy stats, dynamic sizing, auto-disable
│       │   └── gate/
│       │       ├── aggregate.go     ← Aggregates risk signals across layers
│       │       ├── aggregate_test.go
│       │       ├── pipeline.go      ← PreTradeRiskPipeline.Check() — composable pre-trade gates
│       │       └── pipeline_test.go
│       ├── strategy/
│       │   ├── interface.go         ← Strategy interface + Signal + Action types
│       │   ├── registry.go          ← BuildAllScalpers() — 100+ strategies
│       │   ├── curated_registry.go  ← BuildCuratedBTCScalpers() — best 30 strategies
│       │   ├── scalpers.go          ← Original 40 strategies
│       │   ├── scalpers_elite.go    ← Elite set 1 (strategies 41–65)
│       │   ├── scalpers_elite2.go   ← Elite set 2 (strategies 66–80)
│       │   ├── scalpers_pro.go      ← Pro set (strategies 81–95)
│       │   ├── scalpers_pro2.go     ← Pro set 2 (strategies 96–100)
│       │   ├── elite_v2.go          ← ~95 struct-based strategy variants
│       │   ├── elite_v3.go          ← ~105 strategy variants
│       │   ├── research_scalpers.go ← Research-oriented scalpers
│       │   ├── chart_scalpers.go    ← Chart pattern strategies
│       │   ├── intraday_strategies.go← Intraday-specific strategies
│       │   ├── indicators.go        ← Shared technical indicator math
│       │   └── category_normalizer.go← Maps strategy names → canonical categories
│       ├── telemetry/
│       │   ├── metrics.go           ← Prometheus metric definitions
│       │   └── sync.go              ← Metric sync to Prometheus registry
│       └── trading/
│           ├── loop.go              ← Orchestrator struct — main trading loop
│           └── aggregator.go        ← Signal dedup + cooldown filter
│
├── client/                          ← Next.js 14 dashboard
│   ├── package.json                 ← React 19.2.4, Next.js 16.2.1, TypeScript
│   └── src/
│       ├── app/
│       │   ├── page.tsx             ← Root → <TerminalDashboard />
│       │   ├── layout.tsx           ← Root layout
│       │   ├── mock-trading/        ← Paper desk UI
│       │   ├── btc-future-trading/  ← BTC live/paper futures desk UI
│       │   └── api/                 ← 101 Next.js API route files
│       │       ├── ai-app-tracker/  ← AI activity tracking
│       │       ├── angelone/        ← Angel One order management
│       │       ├── auth/            ← Session, sign-in, sign-out
│       │       ├── btc/             ← Klines, spot state, option chain
│       │       ├── cron/            ← Rank strategies, paper desk tick, policy snapshot
│       │       ├── delta/           ← Mirror, probe, testnet, account
│       │       ├── engine/[...path] ← Proxy to Go engine (catch-all)
│       │       ├── health/          ← Health checks (desk worker, paper desk, storage)
│       │       ├── mcx/             ← MCX candles + LTP
│       │       ├── mock-trading/    ← Mock account, trades, analytics (10 routes)
│       │       ├── nifty/           ← Options chain, candles, stocks state
│       │       ├── paper-desk-smoke-test/
│       │       ├── paper-diagnostics/
│       │       ├── paper-oms/       ← Orders, summary
│       │       ├── paper-state/
│       │       ├── paper-trades/    ← Trade CRUD, analytics, leaderboard (9 routes)
│       │       ├── probe/           ← Delta BTC + Angel One NIFTY probes
│       │       ├── replay/          ← Paper replay, walk-forward comparison
│       │       ├── research-edge-report/
│       │       ├── stocks/ltp/
│       │       ├── storage/         ← Backup, restore, health
│       │       ├── strategy-attribution/[id]/
│       │       ├── strategy-rankings/
│       │       ├── strategy-signal-trace/
│       │       └── verification-track/ ← AI context, events, verification
│       ├── components/              ← UI components (TerminalDashboard, charts, panels)
│       ├── lib/
│       │   ├── mockTradingEngine.ts ← Client-side paper trading simulator (1,821 lines)
│       │   ├── researchEdgeScore.ts ← Strategy verdict logic (PROMOTE/KEEP/WATCH/DISABLE)
│       │   └── (other utilities)
│       └── types/                   ← TypeScript type definitions
│
├── infrastructure/
│   ├── ai/
│   │   └── strategy_service/
│   │       ├── api.py               ← FastAPI — 11 endpoints
│   │       ├── management.py        ← Monte Carlo, Trade Journal, News events
│   │       └── backtest.py          ← Historical simulation (96 lines)
│   ├── database/
│   │   ├── phase14_timescale_schema.sql ← 144 lines — 7 tables, 2 continuous views
│   │   └── monitoring/
│   │       └── prometheus-database-alerts.yml
│   ├── docker-compose.yml           ← 5 services: TimescaleDB, Redis, Prometheus, Grafana, engine
│   ├── REDIS_PHASE14_SCHEMA.md      ← Redis key patterns and TTL documentation
│   └── observability/
│       └── PHASE14_PROMETHEUS_METRICS.md ← Metric name catalogue
│
├── data/                            ← Historical market data (JSON flat files)
├── scripts/                         ← Deployment utility scripts
├── bridge/                          ← Node.js ChatGPT/Claude handoff scripts
├── nginx/                           ← Reverse proxy config
├── .env.example                     ← 60+ environment variable definitions
├── .ABOUT THIS APPLICATION/         ← Audit documents
│   ├── ABOUT_THIS_APPLICATION       ← Phase 1 audit (2026-05-31)
│   ├── FILE02                       ← Institutional review (updated 2026-06-01)
│   └── TECHNICAL_KNOWLEDGE_TRANSFER.md ← This document
└── PHASE13_INSTITUTIONAL_TRANSFORMATION_BLUEPRINT.md
    PHASE14_PRODUCTION_INTEGRATION_GO_LIVE.md
    PHASE14_RECOVERY_RUNBOOK.md
```

---

## SECTION 3 — SYSTEM ARCHITECTURE

### Frontend (Next.js 14 + React 19)
**Entry:** `client/src/app/page.tsx` → `<TerminalDashboard />`  
**Port:** 3000 (Vercel deployment)  
**Role:** Dashboard, paper trading UI, strategy research, options viewer  
**Key files:** `mockTradingEngine.ts` (1,821 lines), `researchEdgeScore.ts`, 101 API routes  
**Data sources:** Polls Go engine via `/api/engine/[...path]` proxy; direct MongoDB via Next.js API routes; Supabase for auth/RLS sync  

### Backend API Layer (Next.js API Routes)
101 `route.ts` files under `client/src/app/api/`. These act as:
- **Proxy** to Go engine (`/api/engine/[...path]` catch-all, 60s timeout)
- **Direct DB** access to MongoDB Atlas for paper trades, analytics, strategy rankings
- **Cron jobs** for strategy ranking, paper desk tick, policy snapshots (Vercel cron)
- **Exchange probes** for Delta and Angel One connectivity checks

### Go Engine (Primary Backend)
**Port:** 8080 (Render.com deployment)  
**Language:** Go 1.25.0  
**Entry:** `engine/cmd/antigravity/main.go` (1,881 lines)  
**Role:** Market data ingestion, strategy evaluation, AI council, risk gating, order execution, persistence  

### Data Flow (Complete)

```
┌─────────────────────────────────────────────────────────────────────┐
│  EXTERNAL MARKET DATA                                               │
│  Coinbase WS (BTC-USD) ──┐                                          │
│  Binance REST klines ────┼──→ marketdata.CandleAggregator           │
│  Delta Exchange WS ──────┤    (builds 1m + 5m OHLCV from ticks)    │
│  Angel One SmartAPI REST ┘                                          │
└───────────────────────────────────┬─────────────────────────────────┘
                                    │ Tick{Symbol,Price,Qty,Side,TimeMs}
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  PERFORMANCE / DATA BUS (engine/internal/performance/)              │
│  market_data_bus.go                                                 │
│    Decouples tick ingestion from strategy evaluation                │
│    indicator_cache.go caches computed EMA, RSI, ATR values         │
│    strategy_scheduler.go batches 1m strategies to candle close      │
└───────────────────────────────────┬─────────────────────────────────┘
                                    │
                          ┌─────────┴──────────────────┐
                          ▼                            ▼
┌──────────────────────────────┐    ┌──────────────────────────────┐
│  STRATEGY LAYER              │    │  ALPHA ENGINE LAYER          │
│  engine/internal/strategy/   │    │  engine/internal/alpha/      │
│  100+ strategies via         │    │  CVD, Funding, FVG,          │
│  Strategy.OnTick() /         │    │  Liquidity, Liquidations,    │
│  Strategy.OnCandle()         │    │  MSS, OrderBlock,            │
│  → []Signal                  │    │  VolumeProfile, Delta        │
└──────────────┬───────────────┘    └──────────────┬───────────────┘
               │                                   │
               └──────────────┬────────────────────┘
                              │ []Signal{Action,Confidence,SL%,TP%}
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  SIGNAL AGGREGATION (engine/internal/trading/aggregator.go)         │
│  1. Dedup by strategy ID + symbol                                   │
│  2. Cooldown filter (15-min lockout per strategy)                   │
│  3. HOLD signals dropped                                            │
│  4. Regime filter (NO_TRADE regime blocks all)                      │
│  → approved []Signal                                                │
└───────────────────────────────────┬─────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  AI AGENT COUNCIL (engine/internal/ai/agents.go)                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │ Bull Agent (GPT-4o-mini)  → {should_trade, confidence, SL%} │   │
│  │ Bear Agent (GPT-4o-mini)  → {should_trade, confidence, SL%} │   │
│  │ Macro Analyst (Gemini)    → {bias, should_trade, confidence} │   │
│  │ Risk Arbitrator (GPT-4o)  → {approved, veto_reason}          │   │
│  │ Signal Auditor (Groq)     → {approved, reason}               │   │
│  └─────────────────────────────────────────────────────────────┘   │
│  OR: ML Classifier (engine/internal/ml/) — same decision, <5ms     │
│  Min execution confidence required: 0.82                           │
└───────────────────────────────────┬─────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  PRE-TRADE RISK GATE (engine/internal/risk/gate/pipeline.go)        │
│  1. Context not cancelled                                           │
│  2. Kill switch not active (killswitch/service.go)                  │
│  3. Risk engine available                                           │
│  4. Symbol, EntryPrice, StopLoss, Size all non-zero                 │
│  5. RiskEngine.ValidateTrade() passes                               │
│  6. Recommended size > 0                                            │
└───────────────────────────────────┬─────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  RISK ENGINE (engine/internal/risk/engine.go)                       │
│  Gate 1: Symbol whitelist (BTCUSDT / BTC-USD / BTC-USDT only)       │
│  Gate 2: Position size ≤ MaxPositionBTC (2.0 BTC)                   │
│  Gate 3: Notional ≤ MaxCapitalUSD ($1M)                             │
│  Gate 4: Daily loss < MaxDailyLossPct (5% = $50k circuit breaker)   │
│  Gate 5: Confidence > 0.8 when exposure > 60%                       │
│  + STRATEGY TRACKER: per-strategy sizing multiplier (0.35x–1.60x)   │
│  + AI CONSTITUTION: inviolable rules (2% max risk, 5% daily halt)   │
└───────────────────────────────────┬─────────────────────────────────┘
                                    │ Approved order
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  EXECUTION ROUTER (engine/internal/execution/routing.go)            │
│  Selects order mode by strategy category + market regime:           │
│  MARKET / POST_ONLY / IOC                                           │
│  Routes to: PaperOMS (paper mode) OR BinanceLive (live mode)        │
└─────────────────────┬─────────────────────────────┬─────────────────┘
                      │                             │
                      ▼                             ▼
          ┌─────────────────────┐       ┌──────────────────────┐
          │  PAPER OMS          │       │  BINANCE LIVE CLIENT │
          │  paper_oms.go       │       │  binance_live.go     │
          │  $1M virtual acct   │       │  HMAC-SHA256 signed  │
          │  7 exit types       │       │  REST API calls      │
          │  SL/TP/TRAIL/BE/    │       │  Real fills          │
          │  TIME/LIQ/MANUAL    │       └──────────────────────┘
          └──────────┬──────────┘
                     │ Per-tick evaluation
                     ▼
          ┌─────────────────────┐
          │  OMS v3 AGGREGATE   │
          │  omsv3/aggregate.go │
          │  State machine:     │
          │  NEW→VALIDATED→     │
          │  RISK_APPROVED→     │
          │  SUBMITTED→FILLED   │
          └──────────┬──────────┘
                     │ CloseEvent
                     ▼
┌─────────────────────────────────────────────────────────────────────┐
│  EVENT BUS (engine/internal/events/bus.go)                          │
│  Non-blocking pub/sub (19 event types)                              │
│  Subscribers: telemetry, analytics, reconciliation                  │
└─────────┬────────────────────────┬───────────────────┬─────────────┘
          │                        │                   │
          ▼                        ▼                   ▼
┌─────────────────┐   ┌────────────────────┐  ┌──────────────────────┐
│  LEDGER         │   │  PERSISTENCE       │  │  TELEMETRY           │
│  ledger/store.go│   │  persistence/      │  │  telemetry/metrics.go│
│  Immutable      │   │  store.go          │  │  Prometheus /metrics │
│  event log      │   │  SQLite + Postgres │  │  Phase 14 metrics    │
│  SHA-256 hashed │   │  15s snapshots     │  └──────────────────────┘
└────────┬────────┘   └────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────┐
│  RECONCILIATION (engine/internal/reconciliation/service.go)         │
│  Periodic comparison: OMS state vs Exchange state vs Balance        │
│  OrderMismatchDetector (stale after 30s)                            │
│  PositionDriftDetector (tolerance: 1e-8 BTC)                        │
│  BalanceDriftDetector (tolerance: $1)                               │
│  Alerts → Kill Switch if drift exceeds threshold                    │
└─────────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────┐
│  MONGODB ATLAS                                                      │
│  paper_trades, account_snapshots, analytics, logs                   │
│                                                                     │
│  NEXT.JS API LAYER (101 routes)                                     │
│  Proxy → Go Engine + Direct MongoDB reads                           │
│                                                                     │
│  REACT DASHBOARD                                                    │
│  TerminalDashboard → MockTradingDashboard → BTC Futures Desk        │
│  Strategy Leaderboard, Research Center, Options Chain               │
└─────────────────────────────────────────────────────────────────────┘
```

---

## SECTION 4 — COMPLETE FEATURE INVENTORY

| # | Feature Name | Purpose | Status | Files | Known Limitations |
|---|---|---|---|---|---|
| 1 | BTC Paper Trading (Go engine) | Paper trade BTC perps with $1M virtual account | **Production** | execution/paper_oms.go, trading/loop.go | No funding rate applied to positions |
| 2 | BTC Paper Trading (client-side) | Browser-based paper simulator | **Production** | client/src/lib/mockTradingEngine.ts | Runs on main thread; O(n) analytics |
| 3 | BTC Live Trading (Binance) | Place real Binance Futures orders | **Beta** | execution/binance_live.go | No auth on engine endpoints — security blocker |
| 4 | BTC Live Trading (Delta India) | Place real Delta Exchange orders | **Beta** | delta/live_bridge.go | Credentials in .env — security blocker |
| 5 | Strategy Registry (100+ strategies) | Evaluate all scalpers on every tick/candle | **Production** | strategy/registry.go, scalpers*.go, elite_v*.go | TestScalper always returns BUY — not excluded from production |
| 6 | Alpha Engine — CVD Divergence | Highest-edge BTC signal | **Beta** | alpha/cvd/ | Integration with main registry not confirmed |
| 7 | Alpha Engine — Funding Rate | Mean-reversion at funding extremes | **Beta** | alpha/funding/ | PnL impact of funding not yet modeled in paper OMS |
| 8 | Alpha Engine — Liquidity Sweep | Stop-hunt → reversal detection | **Beta** | alpha/liquidity/ | Integration with main registry not confirmed |
| 9 | Alpha Engine — FVG | 3-candle imbalance fill signals | **Beta** | alpha/fvg/ | Integration not confirmed |
| 10 | Alpha Engine — MSS | Break of structure / CHoCH | **Beta** | alpha/mss/ | Integration not confirmed |
| 11 | Alpha Engine — Order Block | Institutional supply/demand zones | **Beta** | alpha/orderblock/ | Integration not confirmed |
| 12 | Alpha Engine — Volume Profile | VPOC/HVN/LVN dynamic S/R | **Beta** | alpha/volumeprofile/ | Integration not confirmed |
| 13 | Alpha Engine — Liquidation Cascade | Cascade reversal signals | **Beta** | alpha/liquidations/ | Integration not confirmed |
| 14 | Alpha Engine — Delta Divergence | Candlestick delta vs price | **Beta** | alpha/delta/ | Integration not confirmed |
| 15 | Multi-Agent AI Council | Bull/Bear/Macro/Risk LLM voting | **Production** | ai/agents.go | 4.2s free-tier throttle; latency 300ms–2000ms |
| 16 | ML Classifier | Replace LLM calls with trained model | **Prototype** | ml/ml_classifier.go, performance/ml_classifier.go | Training/deployment status unknown |
| 17 | Research Tournament | Strategy promotion via edge scoring | **Production** | client/src/lib/researchEdgeScore.ts, cron/rank-strategies | No OOS split — overfitting risk |
| 18 | Kill Switch | Emergency halt (8 triggers, 4 actions) | **Beta** | killswitch/service.go | Wiring to trading loop not confirmed |
| 19 | OMS v3 + Ledger | Event-sourced order management | **Beta** | omsv3/, ledger/ | Relationship to legacy paper_oms unclear |
| 20 | Reconciliation Service | OMS vs exchange state comparison | **Beta** | reconciliation/ | No exchange connector confirmed |
| 21 | Risk Gate Pipeline | Composable pre-trade gate checks | **Beta** | risk/gate/pipeline.go | Kill switch check is a stub if not wired |
| 22 | Strategy Tracker | Per-strategy adaptive sizing | **Production** | risk/strategy_tracker.go | |
| 23 | Event Bus | Non-blocking pub/sub | **Beta** | events/bus.go | |
| 24 | Persistence (SQLite/Postgres) | Engine state snapshots | **Production** | persistence/store.go | Single-row engine_state overwrite |
| 25 | Dual Persistence (MongoDB) | Paper trades to cloud | **Production** | client API routes | No compound indexes confirmed |
| 26 | Redis Cache Layer | AI decisions, rankings hot cache | **Beta** | performance/redis_cache.go | Redis AUTH not confirmed |
| 27 | Performance Layer | Indicator cache, data bus, scheduler | **Beta** | performance/ | Integration status uncertain |
| 28 | BTC Options Scalper (buy) | 50-strategy options buying engine | **Beta** | options/ | Synthetic fallback prices if exchange unavailable |
| 29 | BTC Options Scalper (sell) | Options writing engine | **Beta** | options_selling/ | |
| 30 | NIFTY 50 Options | Session-gated options trading | **Beta** | niftystocks/ | Session-gated 9:15–15:30 IST |
| 31 | Angel One Equities | NSE stock trading | **Beta** | marketdata/angelone.go | TOTP secret in env — security risk |
| 32 | MCX Commodities | Crude oil, gold price feed | **Prototype** | client/src/app/api/mcx/ | Data feed only, no trading |
| 33 | Python Strategy Service | Monte Carlo, backtest, scoring | **Beta** | infrastructure/ai/strategy_service/ | No test files confirmed |
| 34 | Prometheus Metrics | Full telemetry | **Beta** | telemetry/metrics.go, /metrics endpoint | Not all new modules confirmed instrumented |
| 35 | Grafana Dashboard | Visual monitoring | **Prototype** | infrastructure/docker-compose.yml | No dashboards provisioned |
| 36 | TimescaleDB Schema | Phase 14 time-series schema | **Prototype** | phase14_timescale_schema.sql | Not confirmed applied to running DB |
| 37 | Paper Replay | Walk-forward comparison | **Beta** | client/src/app/api/replay/ | |
| 38 | AI Training Data Pipeline | Dataset for ML classifier | **Prototype** | alpha/ai_training_data/dataset.go | No trained model confirmed |
| 39 | Auth (Supabase) | Session management | **Beta** | client/src/app/api/auth/ | sameSite: lax, 30-day session, insecure dev |
| 40 | Delta Exchange Probe | Connectivity health check | **Production** | client/src/app/api/probe/ | |

---

## SECTION 5 — DATABASE ARCHITECTURE

### 1. PostgreSQL / TimescaleDB
**Purpose:** Event store, tick data, order/fill/position projections, PnL materialized views  
**Location:** Local Docker (dev), AWS RDS Postgres (prod intent)  
**Schema:** `infrastructure/database/phase14_timescale_schema.sql`

**Tables:**

| Table | Key Columns | Type | Retention | Notes |
|---|---|---|---|---|
| trading.event_store | event_id, aggregate_type, aggregate_id, payload, created_at | Hypertable (by created_at) | Unlimited | SHA-256 payload hash; idempotency_key UNIQUE |
| market.ticks | time, exchange, symbol, price, quantity | Hypertable (by time) | 90 days (policy) | Compressed after 7 days |
| trading.order_projection | client_order_id, state, filled_quantity, fee_usd | Regular table | Indefinite | Updated on FILLED/CANCELLED events |
| trading.fill_projection | fill_id, fill_price, fill_quantity, filled_at | Hypertable | Indefinite | |
| trading.position_projection | (account_id, symbol, side) PK | Regular table | Indefinite | Upserted on position events |
| market.candles_1m | bucket, exchange, symbol, OHLCV | Continuous aggregate | Per source | From market.ticks |
| trading.pnl_1m | bucket, account_id, fills, fees | Continuous aggregate | Per source | From fill_projection |

**Indexes (confirmed in schema):**
- event_store: (aggregate_type, aggregate_id, sequence_no), (account_id, created_at), (symbol, created_at)
- ticks: (symbol, time), (exchange, symbol, time)
- fill_projection: (account_id, filled_at), (symbol, filled_at)

**Status:** Schema file present at `infrastructure/database/phase14_timescale_schema.sql`. Application to running database **NOT CONFIRMED**.

---

### 2. SQLite (Go Engine Primary State)
**Purpose:** Engine state snapshots, individual trade records, AI audit logs  
**File:** `./data/engine.db` (SQLITE_PATH env var, Docker volume `/app/data`)  
**Driver:** `modernc.org/sqlite` v1.29.10 (pure Go, no CGO)

**Tables:**

| Table | Purpose | Notes |
|---|---|---|
| engine_state | Single-row overwrite of full engine state | JSON blobs for positions + trades. No audit trail. |
| trades | Per-trade records (relational) | Migrated from BLOB on boot |
| ai_audit_logs | AI agent decision log | No TTL — grows unbounded |
| options_state | BTC options engine state | Per-engine JSON snapshot |
| nifty_options_state | NIFTY options engine state | |
| options_selling_state | Options writing engine state | |
| nifty_options_selling_state | NIFTY options writing state | |

**WAL mode:** Enabled. SQLite in Docker container — if volume not mounted, **ALL history lost on restart**.

---

### 3. MongoDB Atlas
**Purpose:** Paper trades, account snapshots, analytics, logs (cloud persistence)  
**Database name:** `loop_trades`  
**Connection:** MONGODB_URI env var

| Collection | Purpose | Est. Daily Growth | Issues |
|---|---|---|---|
| paper_trades | All executed paper trades | ~500/day | Missing compound index (strategyId, timestamp, status) |
| account_snapshots | Equity/position state snapshots | ~96/day | No TTL |
| analytics | Daily + strategy aggregates | ~30/day | Re-run duplication possible |
| logs | Execution trace events | High (thousands/day) | No TTL index — will bloat |

**Critical missing indexes:**
```javascript
db.paper_trades.createIndex({ strategyId: 1, timestamp: -1, status: 1 })
db.paper_trades.createIndex({ timestamp: -1 })
db.logs.createIndex({ timestamp: 1 }, { expireAfterSeconds: 2592000 })
db.account_snapshots.createIndex({ timestamp: 1 }, { expireAfterSeconds: 7776000 })
```

---

### 4. Redis
**Purpose:** Hot cache for live positions, PnL, risk state, AI decisions, strategy rankings  
**Connection:** REDIS_URL env var (default: redis://localhost:6379)  
**Schema:** `infrastructure/REDIS_PHASE14_SCHEMA.md`

| Key Pattern | TTL | Purpose |
|---|---|---|
| live:position:{account}:{symbol} | 10s | Live position state |
| live:pnl:{account} | 10s | Real-time PnL |
| live:risk:{account} | 10s | Risk state |
| live:market:{exchange}:{symbol} | 3s | Latest tick |
| live:strategy_rankings:{account} | 60s | Rankings cache |
| live:order:{account}:{client_order_id} | 24h | Order state |
| dedupe:idempotency:{key} | 24h | Prevent duplicate orders |
| rate:exchange:{exchange}:{endpoint}:{window} | window+5s | Rate limiting |
| health:service:{service} | 15s | Service heartbeat |

**Security issue:** No AUTH password confirmed. REDIS_URL in .env is `redis://localhost:6379`.

---

### 5. Browser localStorage
**Purpose:** Client-side mock engine state (account state, config)  
**Risk:** Not encrypted. Any XSS on the page can read position data.  
**Status:** NOT VERIFIED which specific keys are stored.

---

## SECTION 6 — API INVENTORY

### Go Engine Endpoints (port 8080)

| Method | Route | Purpose | Auth | Notes |
|---|---|---|---|---|
| GET | /health | Engine vitals | ❌ None | |
| GET | /api/strategies | Strategy performance stats | ❌ None | |
| GET | /api/positions | All open positions (BTC + NIFTY + Options) | ❌ None | |
| GET | /api/trades | Trade journal (last 5,000 from DB or 100 in-memory) | ❌ None | |
| GET | /api/stats | Aggregate PnL, balance, exposure | ❌ None | |
| GET | /api/logs | Ring buffer log lines (last 100) | ❌ None | |
| GET | /api/ai/insights | AI council last decisions | ❌ None | |
| GET | /api/ai/pending-signals | Signals awaiting manual review | ❌ None | |
| POST | /api/ai/approve/:id | Approve a pending signal | ❌ None | **CRITICAL** — no auth |
| POST | /api/ai/reject/:id | Reject a pending signal | ❌ None | **CRITICAL** — no auth |
| GET | /api/options/* | BTC options engine state | ❌ None | |
| GET | /api/nifty-options/* | NIFTY options state | ❌ None | |
| POST | /api/delta-live/enable | Enable Delta live bridge | ❌ None | **CRITICAL** — live trading |
| POST | /api/delta-live/disable | Disable Delta live bridge | ❌ None | |
| GET | /api/delta-live/status | Bridge status | ❌ None | |
| POST | /api/admin/killswitch | Trigger kill switch | ❌ None | **CRITICAL** — emergency stop |
| POST | /api/admin/reset | Reset paper account | ❌ None | **CRITICAL** — destroys state |
| DELETE | /api/admin/history | Clear trade history | ❌ None | **CRITICAL** |
| GET | /metrics | Prometheus scrape endpoint | ❌ None | Should be internal only |

**SECURITY FINDING:** Every single Go engine endpoint has no authentication. Anyone with network access to port 8080 can reset the account, trigger the kill switch, enable live trading, or approve AI signals.

### Next.js API Routes (port 3000, selected important routes)

| Method | Route | Purpose | Auth | Notes |
|---|---|---|---|---|
| GET/POST | /api/engine/[...path] | Proxy to Go engine | Session | 60s timeout |
| GET | /api/paper-trades | List paper trades | Session | MongoDB query |
| POST | /api/paper-trades | Create paper trade | Session | |
| GET | /api/paper-trades/analytics | Strategy analytics | Session | |
| GET | /api/paper-trades/leaderboard | Strategy leaderboard | Session | |
| GET | /api/strategy-rankings | Strategy rankings (60s Redis cache) | Session | |
| POST | /api/cron/rank-strategies | Daily strategy ranking cron | Vercel cron header | |
| POST | /api/cron/paper-desk-tick | Paper desk heartbeat cron | Vercel cron header | |
| POST | /api/mock-trading/trades | Record mock trade | Session | |
| GET | /api/mock-trading/account | Mock account state | Session | |
| GET | /api/btc/futures-klines | BTC klines from Binance | None | |
| GET | /api/health/paper-desk | Paper desk health check | None | |
| POST | /api/auth/signin | Create session | None | Rate limiting: NONE |
| POST | /api/auth/signout | Destroy session | Session | |
| POST | /api/storage/backup | Trigger state backup | Session | |
| GET | /api/research-edge-report | Full research report | Session | |

### Python FastAPI Endpoints (port 8001, if running)

| Method | Route | Purpose |
|---|---|---|
| GET | / | Root info |
| GET | /health | Health check |
| GET | /strategies | List all strategies |
| POST | /strategies/evaluate | Evaluate signal set |
| GET | /framework/config | Framework configuration |
| GET | /framework/status | Current status |
| POST | /framework/run-cycle | Execute one cycle |
| POST | /framework/backtest | Run historical backtest |
| POST | /framework/events | Add news event |
| GET | /framework/journal | Trade journal summary |
| POST | /framework/monte-carlo | Run 500 Monte Carlo simulations |

---

## SECTION 7 — EXECUTION ENGINE (Code Path)

### Order Creation (Paper Mode — Complete Code Path)

```
trading/loop.go: Orchestrator.processSignal(signal)
  │
  ├─ 1. Cooldown check: aggregator.IsCooledDown(strategyID)
  │     └─ Returns false if < 15 min since last fill for this strategy
  │
  ├─ 2. AI council vote: aiAgent.EvaluateSignal(ctx, signal, marketCtx)
  │     ├─ Bull Agent: POST to OpenAI /chat/completions (gpt-4o-mini)
  │     ├─ Bear Agent: POST to OpenAI /chat/completions (gpt-4o-mini)
  │     ├─ Macro Analyst: POST to Gemini Flash API (optional)
  │     └─ Risk Arbitrator: POST to OpenAI (gpt-4o) — final veto
  │     └─ Returns: VoteResult{approved bool, confidence float64, action, SL%, TP%}
  │     └─ If confidence < 0.82: park in pendingSignals (manual review)
  │
  ├─ 3. Pre-trade gate: riskGatePipeline.Check(ctx, input)
  │     ├─ Context alive?
  │     ├─ KillSwitch.IsActive() → block if true
  │     ├─ RiskEngine available?
  │     ├─ Symbol, EntryPrice, StopLoss, Size all > 0?
  │     └─ RiskEngine.ValidateTrade(symbol, size, notional, dailyLoss)
  │
  ├─ 4. Execution router: routing.RouteModeForCategory(category, regime)
  │     └─ Returns: MARKET / POST_ONLY / IOC based on strategy type + regime
  │
  ├─ 5. Paper OMS: oms.OpenPosition(OpenPositionRequest)
  │     ├─ Apply entry slippage: price × (1 ± 5bps)
  │     ├─ Calculate margin: notional / leverage
  │     ├─ Calculate liq price: entryPrice × (1 - (1/leverage - maintenanceMarginPct))
  │     ├─ Set initialSL: entryPrice × (1 - slPct) for LONG
  │     ├─ Set tpPrice: entryPrice × (1 + tpPct) for LONG
  │     ├─ Deduct entry fee: notional × 0.05%
  │     └─ Append to openPositions slice
  │
  ├─ 6. Ledger event: ledger.Store.Append(PositionOpenedEvent)
  │     └─ SHA-256 hash payload, write to DB
  │
  └─ 7. Event bus: bus.Publish(EventPositionOpened, position)
        └─ Telemetry, analytics subscribers notified
```

### Per-Tick Evaluation (paper_oms.go: evalPosition)

```
For each open position (in insertion order, mutex held):
  │
  ├─ LONG SL hit: currentPrice ≤ position.CurrentSL
  │    └─ close(ExitReasonSL)
  ├─ LONG TP hit: currentPrice ≥ position.TPPrice
  │    └─ close(ExitReasonTP)
  ├─ Time limit: age > HoldMinutes (45 min default)
  │    └─ close(ExitReasonTime)
  ├─ Liquidation risk: margin < maintenanceMarginPct (0.5%)
  │    └─ close(ExitReasonLiquidation)
  ├─ Breakeven trigger: progressToTP ≥ 0.40
  │    └─ move CurrentSL to entryPrice (protect capital)
  └─ Trailing stop: progressToTP ≥ 0.65
       └─ CurrentSL = peakPrice × (1 - trailGivebackShare × 0.35)
```

### Exit / Close (paper_oms.go: closePosition)

```
1. Apply exit slippage: exitPrice = price × (1 ∓ 5bps)
2. Calculate gross PnL: (exitPrice - entryPrice) × contracts × leverage (LONG)
3. Deduct exit fee: notional × 0.05%
4. Net PnL = grossPnL - totalFees
5. Remove from openPositions slice
6. Append to closedTrades slice
7. Emit CloseEvent → positions/manager.go → analytics update
8. Update strategy tracker: wins/losses, dailyPnL, consecutiveLosses
```

### PnL Calculation Formulas

```
LONG unrealized: (currentPrice - entryPrice) / entryPrice × notional × leverage - fees
SHORT unrealized: (entryPrice - currentPrice) / entryPrice × notional × leverage - fees
Entry fee: notional × 0.0005 (0.05% taker)
Exit fee: notional × 0.0005
Slippage per leg: price × 0.0005 (5 bps)
Round-trip cost: notional × 0.001 + 2 × price × 0.0005 × contracts
```

**Note: Funding rate is NOT applied to paper OMS positions.** The funding_collector.go in alpha/ polls Binance for funding rates but this data is not fed into paper_oms.go PnL calculations. This is a known gap causing reported PnL to be inflated for long-duration positions.

### Reconciliation Flow

```
reconciliation/service.go: Check(ctx) every interval
  │
  ├─ provider.GetSnapshot() → {OMSOrders, ExchangeOrders, OMSPositions, ExchangePositions}
  ├─ orders.Detect(oms, exchange) → []Alert (stale if > 30s since update)
  ├─ positions.Detect(oms, exchange) → []Alert (drift > 1e-8 BTC)
  └─ balances.Detect(oms, exchange) → []Alert (drift > $1)
  └─ For each alert: ledger.Append(ReconciliationMismatchEvent)
  └─ sync.Correct(alerts) — attempt auto-correction
```

---

## SECTION 8 — OMS ANALYSIS

### OMS Versions

**Legacy Paper OMS (Active — Confirmed):** `engine/internal/execution/paper_oms.go`
- Slice-based position storage, sync.Mutex for determinism
- 7 exit types, breakeven + trailing mechanics
- All paper trading flows through this

**OMS v3 (Implemented — Integration Status Uncertain):** `engine/internal/omsv3/`
- Event-sourced aggregate with state machine
- 9 states, 10 valid transitions
- Integration with main trading loop not confirmed

### OMS v3 State Machine

```
NEW
 ├─→ VALIDATED
 │    ├─→ RISK_APPROVED
 │    │    ├─→ SUBMITTED
 │    │    │    ├─→ ACKNOWLEDGED
 │    │    │    │    ├─→ PARTIALLY_FILLED
 │    │    │    │    │    └─→ FILLED
 │    │    │    │    └─→ FILLED
 │    │    │    └─→ CANCELLED
 │    │    └─→ CANCELLED
 │    └─→ REJECTED
 └─→ CANCELLED
     └─→ REJECTED
```

### Order Lifecycle (Paper OMS — Simplified)

```
Signal Approved → OpenPositionRequest created
  → Paper OMS: append to openPositions[]
  → Status: OPEN (implicit)
  → Per-tick: evalPosition() checks all exit conditions
  → Close triggered: closePosition() called
  → Status: CLOSED
  → Appended to closedTrades[]
  → Strategy tracker updated
  → MongoDB write via Next.js API
```

### Persistence

Paper OMS state is persisted via two paths:
1. `persistence/saver.go` — Snapshots full engine state to SQLite/Postgres every 15–30 seconds (serializes positions and trades to JSON blobs in engine_state table)
2. MongoDB — Each closed trade written to `paper_trades` collection via Next.js API route on the client side

**Recovery on reboot:**
1. `persistence/store.go LoadState()` reads engine_state from DB
2. Deserializes positions JSON → `oms.RestorePositions()`
3. Deserializes trades JSON → `oms.RestoreTrades()`
4. Restored balance overrides initial $1M if different

### Missing Institutional OMS Features
- No order book depth — all fills at mid + slippage
- No partial fill modeling
- No IOC / POST_ONLY differentiation in paper mode
- No maker/taker classification per fill
- No order amendment / cancel-replace
- No TWAPs or algorithmic order types
- Funding rate not applied to open positions

---

## SECTION 9 — RISK ENGINE ANALYSIS

### Risk Architecture (4 Layers)

**Layer 1 — Go Risk Engine (`risk/engine.go`):**
```go
type RiskProfile struct {
    MaxPositionBTC  float64  // Default: 2.0 BTC
    MaxCapitalUSD   float64  // Default: $1,000,000
    MaxDailyLossPct float64  // Default: 5% = $50,000 circuit breaker
}
```
Gates: Symbol whitelist → size cap → capital cap → daily loss → confidence gate

**Layer 2 — Strategy Tracker (`risk/strategy_tracker.go`):**
- Per-strategy sizing multiplier: 0.35x–1.60x based on win rate + consecutive losses
- Auto-disable after 6 trades with < 35% win rate
- Cooldown on loss streaks: 0.15x deduction per consecutive loss
- Category weights: Trend Elite = 1.45x, Mean Rev = 1.3x, Momentum = 1.1x

**Layer 3 — Pre-Trade Risk Gate (`risk/gate/pipeline.go`):**
- Composable pipeline: context → kill switch → engine availability → input validation → risk decision
- Returns `Decision{Status: APPROVED | BLOCKED, Reason}`

**Layer 4 — AI Constitution (`ai/constitution.go`):**
Inviolable rules enforced at Risk Arbitrator veto level:
- Max 2% account risk per trade
- Daily halt at 5% total loss
- Max 5 simultaneous positions, max 3 same direction
- 4+ consecutive losses = trading halt
- No trades in UNKNOWN regime
- 50% size reduction in VOLATILE regime
- Min R:R: 2.0
- Min confidence: 0.70

**Layer 5 — Mock Engine Client (`mockTradingEngine.ts`):**
Separate controls for paper simulator:
- Daily loss: 3% of equity
- Weekly loss: 6%
- Max drawdown: 10%
- Max 5 open trades (3 long, 3 short)
- Min R:R: 1.5

### Kill Switch (`killswitch/service.go`)

8 trigger types:
```
DAILY_LOSS_BREACH, EXCHANGE_OUTAGE, DATA_FEED_OUTAGE, OMS_DESYNC,
RISK_SERVICE_FAILURE, LARGE_POSITION_DRIFT, FUNDING_SHOCK,
LIQUIDATION_EVENT_SPIKE, MANUAL_OPERATOR_TRIGGER
```

4 actions:
```
CANCEL_OPEN_ORDERS, BLOCK_NEW_ORDERS, FLATTEN_POSITIONS, SEND_ALERTS
```

### Missing Institutional Risk Controls
| Control | Why It Matters | Priority |
|---|---|---|
| Kelly Criterion per strategy | Optimal sizing = edge/odds — fixed TargetSize ignores edge quality | P0 |
| VaR (95%/99% daily) | Required for risk reporting | P0 |
| Position correlation matrix | 5 long BTC strategies = 5× concentration, not diversification | P0 |
| Portfolio heat gauge | Total risk-adjusted exposure % of equity | P1 |
| CVaR (expected shortfall) | Average loss when VaR is breached | P1 |
| Drawdown-adjusted sizing | Reduce size as drawdown deepens | P1 |
| Funding rate risk tracking | Longs pay funding; not currently subtracted from PnL | P0 |

---

## SECTION 10 — STRATEGY ENGINE

### Strategy Interface

```go
// engine/internal/strategy/interface.go
type Strategy interface {
    Name() string
    OnTick(tick marketdata.Tick) []Signal
    OnCandle(candle marketdata.Tick) []Signal
}

type Signal struct {
    Symbol, Action string
    TargetSize, Confidence float64
    StopLossPct, TakeProfitPct float64
    AIDecisionID, AIReasoning string
}
```

### Complete Strategy Registry

**Original 40 (engine/internal/strategy/scalpers.go):**

| # | Name | Category | TF | SL% | TP% | Warmup | R:R |
|---|---|---|---|---|---|---|---|
| 1 | TestScalper | Dev | tick | — | — | 0 | — |
| 2 | EMA Cross (8,21) | Trend | 1m | 0.18 | 0.42 | 21 | 2.3 |
| 3 | Triple EMA (5,13,34) | Trend | 1m | 0.20 | 0.42 | 34 | 2.1 |
| 4 | Hull MA (16) | Trend | 1m | 0.18 | 0.38 | 32 | 2.1 |
| 5 | ADX Trend | Trend | 1m | 0.20 | 0.44 | 28 | 2.2 |
| 6 | Ichimoku | Trend | 1m | 0.22 | 0.46 | 52 | 2.1 |
| 7 | Parabolic SAR | Trend | 1m | 0.16 | 0.34 | 10 | 2.1 |
| 8 | RSI Reversal (14) | Mean Rev | 1m | 0.18 | 0.38 | 14 | 2.1 |
| 9 | Bollinger (20,2.0) | Mean Rev | 1m | 0.20 | 0.42 | 20 | 2.1 |
| 10 | VWAP (50, 0.15%) | Mean Rev | 1m | 0.15 | 0.30 | 50 | 2.0 |
| 11 | Mean Reversion (30,2.0) | Mean Rev | 1m | 0.20 | 0.42 | 30 | 2.1 |
| 12 | Stoch RSI | Mean Rev | 1m | 0.18 | 0.38 | 28 | 2.1 |
| 13 | Williams %R (14) | Mean Rev | 1m | 0.18 | 0.38 | 14 | 2.1 |
| 14 | CCI (20) | Mean Rev | 1m | 0.18 | 0.40 | 20 | 2.2 |
| 15 | Momentum (10, 0.3%) | Momentum | 1m | 0.20 | 0.42 | 10 | 2.1 |
| 16 | Donchian (20) | Breakout | 5m | 0.25 | 0.50 | 20 | 2.0 |
| 17 | Keltner | Breakout | 5m | 0.22 | 0.46 | 20 | 2.1 |
| 18 | Pivot (60) | Breakout | 1h | 0.28 | 0.56 | 60 | 2.0 |
| 19 | MACD | Momentum | 1m | 0.18 | 0.40 | 35 | 2.2 |
| 20 | ROC (12, 0.5%) | Momentum | 1m | 0.18 | 0.38 | 12 | 2.1 |
| 21 | Order Flow (100, 0.65) | Order Flow | tick | 0.15 | 0.32 | 100t | 2.1 |
| 22 | Tick Velocity (20, 5.0) | Tick | tick | 0.12 | 0.25 | 20t | 2.1 |
| 23 | Volume Spike (50) | Volume | 1m | 0.18 | 0.38 | 50 | 2.1 |
| 24 | Gap Fill (0.2%) | Price Action | 1m | 0.20 | 0.40 | 1 | 2.0 |
| 25 | Fibonacci (50) | Price Action | 1m | 0.22 | 0.46 | 50 | 2.1 |
| 26 | LinReg (30, 2.0) | Statistical | 1m | 0.20 | 0.42 | 30 | 2.1 |
| 27 | EMA Spread (8,21,50) | Trend | 1m | 0.18 | 0.40 | 50 | 2.2 |
| 28 | Consensus | Composite | 1m | 0.20 | 0.44 | 52 | 2.2 |
| 29 | Volatility Squeeze | Volatility | 1m | 0.20 | 0.44 | 20 | 2.2 |
| 30 | Range Compression (20, 0.3%) | Volatility | 5m | 0.22 | 0.46 | 20 | 2.1 |
| 31 | VWAP RSI2 | Mean Rev | 1m | 0.15 | 0.32 | 50 | 2.1 |
| 32 | Bollinger RSI Fade | Mean Rev | 1m | 0.18 | 0.40 | 20 | 2.2 |
| 33 | MACD VWAP Flip | Momentum | 1m | 0.18 | 0.40 | 35 | 2.2 |
| 34 | Stochastic Range | Mean Rev | 1m | 0.16 | 0.34 | 14 | 2.1 |
| 35 | ATR Volume Impulse | Breakout | 1m | 0.18 | 0.40 | 14 | 2.2 |
| 36 | Opening Range Breakout (15m) | Breakout | 5m | 0.22 | 0.48 | 15min | 2.2 |
| 37 | OBV (20) | Smart Money | 1m | 0.18 | 0.38 | 20 | 2.1 |
| 38 | Chaikin MF (20) | Smart Money | 1m | 0.18 | 0.38 | 20 | 2.1 |
| 39 | AD Line (20) | Smart Money | 1m | 0.18 | 0.38 | 20 | 2.1 |
| 40 | Aroon (14) | Smart Money | 1m | 0.18 | 0.38 | 14 | 2.1 |
| 41 | Engulfing | Price Action | 1m | 0.16 | 0.34 | 2 | 2.1 |
| 42 | Heikin Ashi | Price Action | 1m | 0.18 | 0.38 | 5 | 2.1 |
| 43 | ZigZag (0.5%) | Price Action | 1m | 0.20 | 0.42 | 20 | 2.1 |
| 44 | Micro Pullback | Price Action | 1m | 0.15 | 0.32 | 20 | 2.1 |
| 45 | Supertrend (10,3.0) | Adaptive | 1m | 0.20 | 0.44 | 10 | 2.2 |
| 46 | KAMA (10) | Adaptive | 1m | 0.18 | 0.40 | 10 | 2.2 |
| 47 | MTF RSI | Adaptive | Multi | 0.20 | 0.44 | 52+ | 2.2 |
| 48 | DEMA Cross (8,21) | Trend | 1m | 0.18 | 0.40 | 21 | 2.2 |
| 49 | Vortex (14) | Trend | 1m | 0.18 | 0.38 | 14 | 2.1 |
| 50 | DMI (14) | Trend | 1m | 0.18 | 0.40 | 14 | 2.2 |

**Elite 50 (strategies 51–100):** DEMA, Vortex, DMI, MA Ribbon, TII (Trend Elite); RSI Divergence, ZScore, RSI+BB, Anchored VWAP, Double Pattern (Mean Rev Elite); ATR Breakout, Squeeze Momentum, Price Channel, Fractal, VCP (Breakout Elite); TRIX, Coppock, KST, PPO, ROC Accel, Momentum Divergence, Momentum RSI (Momentum Elite); Fisher Transform, Connors RSI, Chande MO (Oscillator Elite); and more.

**Alpha Strategies (Phase 11 — integration status NOT CONFIRMED with registry):**

| Module | Signal Logic | SL% | TP% |
|---|---|---|---|
| CVD Divergence | CVD series vs price divergence | 0.30 | 0.75 |
| Funding Rate | funding > 0.1% → SHORT; funding < -0.05% → LONG | custom | custom |
| Liquidity Sweep | price sweeps level then reverses | custom | custom |
| FVG | 3-candle imbalance fill | custom | custom |
| MSS | BOS / CHoCH break of structure | custom | custom |
| Order Block | last candle before strong move | custom | custom |
| Volume Profile | price at VPOC/HVN/LVN | custom | custom |
| Delta Divergence | candlestick delta vs direction | custom | custom |
| Liquidation Cascade | large liquidation event → dislocation | custom | custom |

### Strategy Registry Design

Strategies are organized into groups by timeframe in `BuildAllScalpers()`. The Orchestrator in `trading/loop.go` dispatches:
- Tick-level strategies: called on every `Tick` event
- 1m strategies: called on `CandleClose(1m)` event
- 5m strategies: called on `CandleClose(5m)` event
- 1h strategies: called on `CandleClose(1h)` event

`CandleAggregator` in `marketdata/` builds OHLCV from the raw tick stream and emits `CandleClose` events.

**Strategies to disable (dev artifacts):**
- `TestScalper` — always returns BUY, pollutes research metrics
- `GapFill` — BTC perpetuals trade 24/7, no gaps
- Tick Velocity — sensitive to WebSocket connection quality, not market structure

**Overfitting risk:**
- Elite V2 + V3 contribute ~200 near-identical parameter variations. Any promoted variant is an overfitted parameter selection from 200 random draws. OOS validation is missing from the research tournament.

---

## SECTION 11 — PHASE IMPLEMENTATION STATUS

| Phase | Goal | Status | Key Modules | Remaining |
|---|---|---|---|---|
| Phase 1–5 | Core paper OMS, strategy registry, basic risk | ✅ 100% | execution/, strategy/, risk/engine.go | — |
| Phase 6–7 | Research tournament, verdict logic | ✅ 100% | researchEdgeScore.ts, cron/rank-strategies | — |
| Phase 8 | Multi-agent AI council | ✅ 100% | ai/agents.go, ai/constitution.go | Latency still ~300ms–2000ms |
| Phase 9 | Dual persistence (MongoDB + SQLite) | ✅ 100% | persistence/store.go | No compound indexes in Mongo |
| Phase 10 | Options scalper (BTC + NIFTY) | ✅ 90% | options/, options_selling/, niftystocks/ | NIFTY options selling incomplete |
| Phase 11 | Microstructure alpha engine | ✅ 90% | alpha/cvd/, funding/, liquidity/, fvg/, mss/, orderblock/, volumeprofile/, delta/, liquidations/ | Integration with main registry NOT CONFIRMED |
| Phase 12 | Institutional infrastructure scaffold | ✅ 80% | events/bus.go, ledger/, killswitch/, omsv3/ | Kill switch wiring to trading loop not confirmed; OMS v3 vs legacy paper_oms dual-track |
| Phase 13 | Risk gate pipeline, AI constitution | ✅ 100% | risk/gate/, ai/constitution.go | Kill switch integration in gate not confirmed |
| Phase 14 | Production integration | ✅ 70% | reconciliation/, performance/, telemetry/, TimescaleDB schema, Redis schema | Schema not applied to DB; ML classifier untrained; Redis AUTH missing; security unresolved |

---

## SECTION 12 — AI SYSTEM

### AI Council Architecture

5 specialized agents in `engine/internal/ai/agents.go`:

**Bull Agent**
- Model: `gpt-4o-mini`
- Role: Find LONG opportunities
- Entry conditions checked: EMA9 > EMA21, RSI 40–68, ADX ≥ 18, price ≥ VWAP
- Output schema: `{should_trade, confidence, thesis, size_btc, stop_loss_pct, take_profit_pct}`
- SL geometry: 0.12–0.20%, TP: 0.20–0.32%

**Bear Agent**
- Model: `gpt-4o-mini`
- Role: Find SHORT opportunities
- Entry conditions: EMA9 < EMA21, RSI 32–60, ADX ≥ 18, price ≤ VWAP
- Same output schema as Bull

**Macro Analyst**
- Model: `Gemini Flash 2.0` (optional, can be disabled)
- Role: Top-down regime — BULLISH / BEARISH / NEUTRAL
- Checks: ADX ≥ 20, EMA separation, VWAP alignment, ATR level
- Output: `{should_trade, bias, confidence}`

**Risk Arbitrator**
- Model: `gpt-4o` (most capable, used for final veto)
- Role: Final yes/no on every trade
- Blocks if: Bull/Bear disagree, ADX < 15, SL > 0.22%, TP > 0.35%, RSI at extremes, price > 0.4% from VWAP, > 3 open positions, daily loss > 2%
- Output: `{approved, approved_action, veto_reason, adjusted_size}`

**Signal Auditor**
- Model: `Groq Llama-3-70b` (fallback for batch validation)
- Role: Validate trend confirmation for batches of signals
- Checks: EMA direction matches signal, VWAP alignment, RSI not extreme, ADX ≥ 18
- Output: `{approved, confidence, reason}`

### Provider Fallback Chain
```
Primary attempt: OpenAI (GPT-4o / GPT-4o-mini)
  ↓ timeout / error
Gemini Flash 2.0
  ↓
Mistral
  ↓
HuggingFace
  ↓
Cloudflare Workers AI
  ↓
OpenRouter (multi-model router)
  ↓
Groq Llama-3-70b (Groq — free tier)
  ↓ FREE TIER: time.Sleep(4200ms) between calls
```

### Critical Performance Issue
**Free-tier throttle:** `time.Sleep(4200 * time.Millisecond)` in `agents.go` line ~345.
- Maximum throughput: **14 signals per minute**
- Strategy registry can produce hundreds of signals per minute
- Signal queue fills; system executes on stale prices
- At 0.12% SL, 2.2 seconds of BTC price movement can consume 83% of stop budget before order is placed

### ML Classifier (Replacement Path)
- Location: `engine/internal/ml/ml_classifier.go` and `performance/ml_classifier.go`
- Training data pipeline: `alpha/ai_training_data/dataset.go`
- Purpose: Replace LLM API calls with sub-5ms local inference
- Status: **Module implemented. Training status and deployment integration NOT CONFIRMED.**

### Caching
Redis cache layer in `performance/redis_cache.go` is designed to cache AI decisions. Integration with the AI agent call path is **NOT CONFIRMED**.

### Trading Dependency on AI
**Yes — AI council is a mandatory gate.** All signals with confidence < 0.82 are parked in `pendingSignals` for manual review and are not auto-executed. If all LLM providers fail simultaneously and ML classifier is not wired in, **no new positions will open.**

---

## SECTION 13 — MARKET DATA SYSTEM

### Data Providers

| Provider | Markets | Protocol | Auth | Files |
|---|---|---|---|---|
| Coinbase WebSocket | BTC-USD | WSS | None (public) | marketdata/client.go (CoinbaseClient) |
| Binance REST | BTC klines | HTTPS | Optional (public klines) | Fetched via Next.js /api/btc/futures-klines |
| Delta Exchange WebSocket | BTC spot ticker | WSS | None (public) | delta/ticker_client.go (DeltaTickerClient) |
| Delta Exchange REST | BTC options chain, live orders | HTTPS | API key + secret (HMAC) | delta/live_bridge.go |
| Angel One SmartAPI | NIFTY 50, stocks | HTTPS REST | JWT + TOTP | marketdata/angelone.go |
| NSE Index | NIFTY 50 index | HTTPS REST | None | marketdata/ nse client |

### Tick Ingestion Flow

```
Exchange WebSocket
  → marketdata.Client.GetTickChannel()  (buffered Go channel)
  → trading/loop.go: Orchestrator processes Tick
    → CandleAggregator.AddTick(tick)    (builds 1m, 5m OHLCV)
    → Performance layer market_data_bus (if wired)
    → Strategy evaluation (OnTick for tick-level strategies)
    → Every N ticks: dispatch 1m candle close → OnCandle strategies
```

### Candle Aggregation
`marketdata/candle_aggregator.go` builds OHLCV candles from raw ticks in-memory. **No warm-up pre-fetch is confirmed for the alpha strategies.** The legacy `warmup.go` pre-fetches klines for original strategies, but alpha module warmup requirements are not confirmed enforced.

Cold-start gap: After engine restart, Ichimoku (52 bars), Pivot (60 bars), MTF RSI (52+ bars), and all alpha strategies producing garbage signals until sufficient history accumulates. This takes 52–60 minutes of live ticks.

### Funding Rate Data
`alpha/funding/funding_collector.go` polls Binance `/fapi/v1/fundingRate` endpoint. Data feeds into `funding_engine.go` for signal generation. **This data is NOT fed into paper OMS PnL calculations.** The gap between what is collected and what is modeled is the primary PnL inflation source.

### Liquidation Data
`alpha/liquidations/liquidations_engine.go` is implemented. External liquidation feed (Coinglass API) integration status is **NOT VERIFIED**.

### Order Book
**NOT IMPLEMENTED.** No order book / Level 2 data ingested. All fills modeled at mid-price + slippage flat bps. No volume-at-price, no bid/ask spread model.

### Open Interest
**NOT IMPLEMENTED.** No open interest data collected or modeled.

---

## SECTION 14 — PERFORMANCE ANALYSIS

### Bottlenecks (Ranked by Severity)

**CRITICAL: Free-tier AI throttle**
- `agents.go`: `time.Sleep(4200ms)` between free-tier LLM calls
- Max throughput: 14 signals/minute regardless of strategy count
- Signals queue and execute on stale prices
- **Fix:** Wire ML classifier or upgrade to paid API tier

**HIGH: AI council serial calls (4 LLM calls per signal)**
- Even with paid API, Bull → Bear → Macro → Risk = 3–4 sequential or parallel HTTPS calls
- End-to-end latency: 300ms–2000ms per signal
- **Fix:** Parallelize Bull+Bear+Macro (goroutine fan-out), then Risk Arbitrator (reduces to 1×parallel + 1×serial)

**HIGH: MongoDB without compound indexes**
- `paper_trades` collection scanned fully for analytics queries
- At 100k documents, query time degrades from ms to seconds
- **Fix:** 4 index creations (see Section 5)

**MEDIUM: `computeAnalytics()` O(n) every render**
- Client-side recalculation over full trade array on every React render cycle
- Degrades at ~1,000 trades
- **Fix:** `useMemo` keyed on trade array length + last trade ID; or `analytics.go` incremental update

**MEDIUM: Redis cache layer not confirmed wired**
- `performance/redis_cache.go` exists but integration with strategy rankings API route is not confirmed
- Every page load may re-query MongoDB
- **Fix:** Verify /api/strategy-rankings uses Redis cache

**MEDIUM: strategy_scheduler.go exists but integration not confirmed**
- Without it, all 100 strategies run on every WebSocket tick (sub-ms in Go but wasted computation)
- With it, 1m strategies only evaluate on candle close — 60x reduction in evaluations

**LOW: Python FastAPI serial evaluation**
- Without asyncio parallelism, 120 strategies evaluated sequentially
- Estimated ~10 strategies/second maximum throughput

**LOW: No Go pprof endpoint**
- `net/http/pprof` not confirmed registered
- No way to profile goroutine count, heap, or CPU in production without restart

### Concurrency Notes
- Paper OMS uses `sync.Mutex` for all position operations — deterministic and correct but single-threaded for OMS operations
- Event bus is non-blocking with buffered subscriber channels — publishers never wait, dropped events counted
- Orchestrator's `aiCandleCh` channel feeds candle history to AI agent independently of tick processing
- `pendingSignals map` protected by `sync.RWMutex`

### Memory
- `priceWindow`, `volumeWindow` in Orchestrator: bounded in-memory circular buffers
- `candleHistory`: 20×5m candles (fixed size)
- `RingLogger`: fixed 100 lines
- `closedTrades []PaperClosedTrade`: unbounded growth in memory — large backtests accumulate here

---

## SECTION 15 — SECURITY AUDIT

### CRITICAL (immediate action required)

| Finding | Location | Detail |
|---|---|---|
| Live API credentials in source | .env | GROQ_API_KEY=gsk_qP78f..., DELTA_API_KEY=VSPFbT1n..., DELTA_API_SECRET=xH7WfPzZ... — live credentials. If .env was ever committed to git, they are already leaked. **Revoke immediately.** |
| password123 database credential | .env | DATABASE_URL=postgres://antigravity:password123@... — trivially cracked default credential. Rotate immediately. |
| No authentication on Go engine | All endpoints | /paper/open, /paper/close, /paper/reset, /api/admin/killswitch, /api/delta-live/enable — zero auth middleware. Anyone with port 8080 access can open live trades, reset paper account, or trigger emergency halt. |
| ANGELONE_TOTP_SECRET in env | .env | Live Indian equities account TOTP seed. Leaked = full account takeover. |

### HIGH

| Finding | Location | Detail |
|---|---|---|
| Session cookie security | auth/signin/route.ts | sameSite: "lax" (should be strict), secure: false in dev (session tokens leak on shared HTTP network), maxAge: 30 days (should be 8 hours for a trading platform) |
| No rate limiting on auth | auth/signin/route.ts | Unlimited login attempts — credential stuffing trivial |
| Docker runs as root | docker-compose.prod.yml | No `user:` directive — container escape = host root |
| Redis no AUTH | REDIS_URL in .env | `redis://localhost:6379` — no password. Any process on the network can read all cached positions. |
| No CORS on Go engine | engine HTTP server | Port 8080 is open to any origin |
| No input validation on /paper/open | execution handlers | OpenPositionRequest deserialized without confirmed schema validation |

### MEDIUM

| Finding | Detail |
|---|---|
| NEXT_PUBLIC_ env vars leaking config | Signal thresholds, feature flags embedded in client JS — visible to anyone loading the page |
| 97+ API routes with no global auth middleware | Each route implements (or omits) session check individually — inconsistent coverage |
| No API versioning | /api/paper-trades — any schema change breaks all clients |
| AI reasoning strings logged to SQLite | LLM responses stored in ai_audit_logs — LLMs can be prompted to include sensitive content |

### Security Score: 2/10 — Unchanged from Phase 1. No security work done in Phases 11–14.

---

## SECTION 16 — INFRASTRUCTURE

### Docker Compose (`infrastructure/docker-compose.yml`)

5 services:
1. **timescaledb:latest-pg15** — PostgreSQL 15 + TimescaleDB. Port 5432. DB: `antigravity`. Volume: `pgdata`.
2. **redis:7-alpine** — Redis 7. Port 6379. No AUTH configured.
3. **prometheus:latest** — Metrics scraper. Port 9090. Scrapes Go engine /metrics.
4. **grafana:latest** — Metrics dashboards. Port 3001 (host) → 3000 (container). Admin password: `admin`.
5. **Go engine** (implied) — Built from `engine/` Dockerfile.

### Deployment Topology

```
User Browser
  ↕ HTTPS
Vercel (Next.js client + API routes)
  ↕ HTTP (INTERNAL_API_URL env var)
Render.com (Go engine, port 8080)
  ↕
MongoDB Atlas (paper_trades, analytics, logs)
  ↕
PostgreSQL (AWS RDS or local Docker)
Redis (local Docker or managed)
```

### CI/CD
**NOT IMPLEMENTED.** No GitHub Actions, no CI pipeline, no test runner on push. Manual deployments via Vercel (git push → auto-deploy) and Render.com (manual trigger or git push).

### Monitoring
- Prometheus `/metrics` endpoint on Go engine — confirmed
- Grafana container in Docker Compose — no dashboards provisioned
- `infrastructure/observability/PHASE14_PROMETHEUS_METRICS.md` — metric catalogue exists but dashboards are empty
- No alerting rules confirmed active
- Ring logger `/api/logs` — last 100 log lines in memory

### Backups
- MongoDB Atlas: managed backups (Atlas tier-dependent)
- SQLite: Docker volume `pgdata` — lost if volume not mounted
- LocalFS JSON: `persistence/file_snapshot.go` writes atomic JSON for options state to `/data/` directory
- No automated backup scripts confirmed for SQLite or Postgres data

### Disaster Recovery
`PHASE14_RECOVERY_RUNBOOK.md` exists at repo root. Content: NOT READ (recommend reading before any production deployment).

**Warm restart sequence (from Redis schema doc):**
1. Replay event ledger into Postgres projections
2. Load active positions, open orders, risk state from projections
3. Populate Redis hot cache
4. Reconcile exchange state
5. Keep kill switch active until reconciliation passes

---

## SECTION 17 — EVENT FLOW MAPPING

### Complete BTC Trade Event Flow (with exact files)

```
1. BTC tick arrives
   File: engine/internal/marketdata/client.go
   Struct: Tick{Symbol:"BTC-USD", Price:67430.00, Qty:0.05, Side:"buy", TimeMs:...}
   Channel: GetTickChannel() → buffered Go channel

2. Orchestrator receives tick
   File: engine/internal/trading/loop.go
   Method: Orchestrator.processTick(tick)
   Action: feed to CandleAggregator + performance layer market_data_bus

3. Candle aggregation
   File: engine/internal/marketdata/candle_aggregator.go
   Action: builds 1m OHLCV; on close emits CandleClose event

4. Strategy evaluation
   File: engine/internal/strategy/registry.go, scalpers.go, elite_v2.go, etc.
   Method: strategy.OnTick(tick) or strategy.OnCandle(candle)
   Returns: []Signal{Action:"BUY", Confidence:0.73, SL:0.18%, TP:0.42%, ...}

5. Signal aggregation
   File: engine/internal/trading/aggregator.go
   Checks: cooldown (15 min), dedup, regime gate
   Output: approved []Signal

6. AI council evaluation
   File: engine/internal/ai/agents.go
   Calls: Bull Agent → Bear Agent → Macro Analyst → Risk Arbitrator (sequential or parallel)
   Output: VoteResult{approved:true, confidence:0.85, action:"BUY", sl:0.18%, tp:0.42%}
   If confidence < 0.82: stored in pendingSignals, NOT executed

7. Pre-trade risk gate
   File: engine/internal/risk/gate/pipeline.go
   Checks: context → kill switch → engine → inputs → risk decision
   Output: Decision{Status:APPROVED}

8. Risk engine validation
   File: engine/internal/risk/engine.go
   Checks: symbol whitelist → size → capital → daily loss → confidence gate
   Output: approved bool

9. Execution routing
   File: engine/internal/execution/routing.go
   Method: RouteModeForCategory("Trend", "TRENDING") → "POST_ONLY"
   Output: OrderMode

10. Paper OMS open
    File: engine/internal/execution/paper_oms.go
    Method: oms.OpenPosition(OpenPositionRequest{...})
    Action: apply slippage, set SL/TP/liq, deduct entry fee, append to openPositions
    Output: PaperPosition{ID:"abc123", Side:"LONG", EntryPrice:67433, ...}

11. Ledger event written
    File: engine/internal/ledger/store.go
    Method: ledger.Append(PositionOpenedEvent{...})
    Payload hash: SHA-256
    Storage: Postgres event_store or SQLite

12. Event bus publish
    File: engine/internal/events/bus.go
    Event: EventPositionOpened
    Subscribers: telemetry (metrics), analytics (stats update), reconciliation (snapshot update)

13. Per-tick position evaluation (subsequent ticks)
    File: engine/internal/execution/paper_oms.go
    Method: oms.Tick(price) called on every tick
    Checks: SL hit → TP hit → TIME → LIQ → Breakeven → Trailing

14. Position close event
    File: engine/internal/execution/paper_oms.go
    Method: closePosition(pos, exitReason, exitPrice)
    Action: apply exit slippage, deduct exit fee, compute net PnL, remove from openPositions

15. CloseEvent emitted
    File: engine/internal/positions/manager.go
    Event: CloseEvent{StrategyID, PnL, ExitReason, ...}
    Action: strategy tracker updated (wins/losses/consecutiveLosses)

16. Event bus publish
    File: engine/internal/events/bus.go
    Event: EventPositionClosed
    Subscribers: telemetry, analytics, reconciliation

17. State persistence
    File: engine/internal/persistence/saver.go
    Interval: every 15–30 seconds
    Action: serialize full engine state → SQLite/Postgres engine_state table

18. MongoDB write (via client)
    Route: POST /api/paper-trades
    File: client/src/app/api/paper-trades/route.ts
    Action: insert closed trade document to paper_trades collection
    Indexes: strategyId, timestamp (simple — compound missing)

19. Analytics update
    File: client/src/lib/researchEdgeScore.ts
    Method: computeVerdict(strategyTrades)
    Action: recalculate win rate, expectancy, fee%, profit factor → verdict

20. Dashboard display
    File: client/src/app/page.tsx → TerminalDashboard
    Polling: /api/paper-trades, /api/strategy-rankings, /api/engine/stats
    Render: equity curve, leaderboard, KPI tiles updated
```

---

## SECTION 18 — CODE QUALITY REVIEW

### Dead / Dev Code

| Item | Location | Issue |
|---|---|---|
| TestScalper | strategy/registry.go | Always returns BUY — development artifact. Must be excluded from production registry and research scoring. |
| GapFill Scalper | strategy/scalpers.go | BTC perpetuals trade 24/7 — no opening gaps. Signal source is near-zero. |
| Tick Velocity Scalper | strategy/scalpers.go | Measures WebSocket connection quality, not market structure. |
| DPO Scalper | (if present in elite files) | Detrended Price Oscillator removes the trend that SL/TP geometry depends on. |

### Duplicate Code

| Item | Locations | Risk |
|---|---|---|
| Fee model (0.05% taker, 5 bps slippage) | Defined independently in paper_oms.go AND mockTradingEngine.ts | Will drift — already 1 divergence: paper_oms uses 0.05% taker, mockTradingEngine uses "Delta India futures" rate. |
| Signal struct | strategy/interface.go (Go) AND alpha/types.go (Go) AND mockTradingEngine.ts (TS) | Three separate signal type definitions. |
| Risk limit constants | DEFAULT_MOCK_TRADING_CONFIG in mockTradingEngine.ts AND risk/engine.go AND ai/constitution.go | Three separate limit sets that can drift independently. |

### Legacy / Experimental Code

| Item | Status | Notes |
|---|---|---|
| bridge/ | Prototype | Node.js ChatGPT/Claude handoff scripts — unclear production role |
| research_ai/ | Experimental | Research-grade signal generation — integration status unclear |
| brain/ | Archived | AI training data (archived) |
| paper_oms.go vs omsv3/ | Dual-track | Both exist. Legacy is confirmed active. v3 integration not confirmed. |
| elite_v2.go + elite_v3.go | Experimental | ~200 near-identical parameter variations. Overfitting risk if any are promoted. |

### Technical Debt (Ranked)

| Rank | Item | Severity | File |
|---|---|---|---|
| 1 | mockTradingEngine.ts — 1,821-line God object | HIGH | client/src/lib/mockTradingEngine.ts |
| 2 | Dual OMS (paper_oms + omsv3) — unclear which is primary | HIGH | execution/paper_oms.go, omsv3/ |
| 3 | Alpha strategies not confirmed integrated with registry | HIGH | alpha/, strategy/registry.go |
| 4 | ML classifier not confirmed trained or wired | HIGH | ml/ml_classifier.go |
| 5 | No compound MongoDB indexes applied | HIGH | — (admin task) |
| 6 | TimescaleDB schema not confirmed applied | HIGH | phase14_timescale_schema.sql |
| 7 | Phase 14 infrastructure modules not confirmed wired to trading loop | MEDIUM | performance/, events/ |
| 8 | AI prompts hardcoded in agents.go — no versioning | MEDIUM | ai/agents.go |
| 9 | No API versioning on Next.js routes | MEDIUM | client/src/app/api/ |
| 10 | No annualized Sharpe (uses √n not √252) | MEDIUM | researchEdgeScore.ts |

### Refactor Priorities

1. **Decompose mockTradingEngine.ts** → separate Risk, Analytics, Persistence, SignalProcessor, PositionManager modules
2. **Deprecate paper_oms.go** — confirm OMS v3 is the execution path; migrate all tests
3. **Unify Signal types** — single canonical Signal type shared across Go strategy, alpha, and TypeScript layers
4. **Extract risk limit constants** to single configuration source (env vars or shared config struct)

---

## SECTION 19 — INSTITUTIONAL READINESS AUDIT

| Area | Current | Target | Gap |
|---|---|---|---|
| Architecture | 8/10 | 9/10 | Clarify OMS v3 vs legacy; confirm alpha integration |
| Execution | 6/10 | 9/10 | Wire OMS v3; add funding rate to PnL; partial fills; market impact |
| Risk | 7/10 | 9/10 | Kelly Criterion; VaR/CVaR; position correlation; portfolio heat |
| Security | 2/10 | 9/10 | Rotate credentials; add auth middleware; fix session; Docker non-root |
| Database | 6/10 | 9/10 | Apply TimescaleDB schema; add compound Mongo indexes; Redis AUTH |
| Performance | 6/10 | 9/10 | Wire ML classifier; confirm Redis cache; add pprof |
| Scalability | 5/10 | 8/10 | Multiple symbols; multi-user; horizontal engine |
| Backtesting | 4/10 | 8/10 | Add funding, spread, market impact; OOS split |
| AI | 6/10 | 8/10 | Wire ML classifier; cache decisions; parallelize agents |
| Observability | 5/10 | 9/10 | Provision Grafana dashboards; configure alerting rules |
| Operations | 5/10 | 9/10 | CI/CD pipeline; automated backups; runbook testing |
| **Overall** | **61/100** | **92/100** | **31 points gap** |

### Gaps to reach 90/100
1. Security hardening (5+ points)
2. ML classifier wired + AI latency resolved (5 points)
3. Production database schema applied + indexed (4 points)
4. Alpha strategies confirmed integrated (3 points)
5. Kill switch + reconciliation fully wired (3 points)
6. Funding rate in PnL model (2 points)
7. OOS split in research tournament (2 points)
8. VaR / Kelly / correlation matrix (2 points)
9. CI/CD + alerting (2 points)
10. OMS v3 as primary path, legacy deprecated (2 points)

---

## SECTION 20 — FINAL RECOMMENDATIONS

### Top 20 Blockers (in order — do not skip)

| # | Blocker | Time | Impact |
|---|---|---|---|
| B1 | Rotate GROQ_API_KEY, DELTA_API_KEY, DELTA_API_SECRET, DATABASE_URL password | 2 hrs | **Live account compromise is active risk** |
| B2 | Add authentication middleware to all Go engine endpoints (bearer token at minimum) | 4 hrs | Anyone on network can reset account or enable live trading |
| B3 | Move all secrets to environment variable manager (Vercel env, Render env) — never in repo | 4 hrs | Prevents re-occurrence |
| B4 | Purge .env from git history (BFG Repo Cleaner) | 2 hrs | Leaked credentials in history are still leaked |
| B5 | Verify kill switch wires to trading/loop.go | 2 hrs | Emergency halt is a stub if not connected |
| B6 | Clarify OMS v3 vs paper_oms — pick one as primary; disable the other | 4 hrs | Dual OMS creates state divergence risk |
| B7 | Apply phase14_timescale_schema.sql to running Postgres DB | 4 hrs | Phase 14 persistence layer is schema-only until applied |
| B8 | Confirm alpha strategies are registered in strategy/registry.go | 1 day | 9 new alpha strategies block at NO_APPROVED_STRATEGY with zero trades |
| B9 | Add Redis AUTH password to REDIS_URL | 1 hr | All cached positions readable by any process |
| B10 | Confirm ML classifier is trained on ai_training_data and wired to agent call path | 1 day | 4.2s throttle kills throughput until this is done |
| B11 | Add compound MongoDB indexes (strategyId, timestamp, status) | 2 hrs | Analytics queries become full table scans at 100k trades |
| B12 | Add TTL index to logs collection (30 days) | 1 hr | MongoDB Atlas cost grows unbounded |
| B13 | Enforce candle warmup for alpha strategies before accepting signals | 1 day | Cold-start garbage signals from MSS, Ichimoku, MTF RSI (need 52+ bars) |
| B14 | Remove TestScalper from production strategy registry | 1 hr | Inflates win rate metrics; pollutes research tournament |
| B15 | Fix session cookie: sameSite strict, secure true always, maxAge 8h | 2 hrs | 30-day session on financial account is unacceptable |
| B16 | Add rate limiting on /api/auth/signin (5 attempts/minute/IP) | 2 hrs | Credential stuffing vector |
| B17 | Run Docker container as non-root (user: "1000:1000") | 1 hr | Container escape = host root |
| B18 | Verify reconciliation service has exchange connector wired | 1 day | Reconciliation is a dead code path if no exchange provider injected |
| B19 | Add funding rate to paper OMS PnL calculation | 2 days | Reported expectancy is inflated for all long-duration positions |
| B20 | Verify performance/ modules (market_data_bus, strategy_scheduler) are wired to trading/loop.go | 1 day | Phase 14 performance improvements are dead code until wired |

---

### Top 20 Quick Wins (< 4 hours each)

| # | Quick Win | Time | Impact |
|---|---|---|---|
| Q1 | Add MongoDB compound index (strategyId, timestamp, status) on paper_trades | 30 min | 10–100x analytics query speedup at scale |
| Q2 | Add TTL index on logs collection (30 days) | 15 min | Prevents Atlas storage bloat |
| Q3 | Remove TestScalper from BuildAllScalpers() | 10 min | Removes metric pollution |
| Q4 | Fix Docker user: "1000:1000" in docker-compose | 15 min | Security hardening |
| Q5 | Add Redis AUTH=password to docker-compose Redis service | 15 min | Security hardening |
| Q6 | Add Go pprof endpoint to main.go | 30 min | Enables production profiling |
| Q7 | Set sameSite: "strict", secure: true in auth cookies | 30 min | Session security |
| Q8 | Add X-Engine-Token header check on Go engine endpoints | 2 hrs | Basic auth (pre-JWT) |
| Q9 | Annotate alpha strategy SL/TP geometry constants in types.go | 30 min | Makes SL 0.30/TP 0.75 CVD values discoverable |
| Q10 | Verify timescale hypertable for market.ticks with \d+ in psql | 15 min | Confirms or refutes Phase 14 DB status |
| Q11 | Add benchmark comparison (buy-and-hold BTC) to equity curve chart | 2 hrs | Context for whether system generates alpha |
| Q12 | Add annualized Sharpe (√252 not √n) to researchEdgeScore.ts | 1 hr | Industry-comparable metric |
| Q13 | Parallelize Bull + Bear + Macro agents (goroutine fan-out) | 2 hrs | Reduces agent latency from 1.5s to ~600ms |
| Q14 | Add drawdown duration tracking (days in drawdown) | 2 hrs | Key institutional metric missing from dashboard |
| Q15 | Add rolling Sharpe (30d/90d) to strategy leaderboard | 2 hrs | Detect regime-dependent strategy decay |
| Q16 | Add useMemo to computeAnalytics() in mock trading dashboard | 1 hr | Prevents O(n) recalculation on every React render |
| Q17 | Add funding rate display widget on dashboard | 2 hrs | Shows current BTC funding rate + direction |
| Q18 | Provision Grafana dashboard with Go engine /metrics | 3 hrs | Operational visibility from day one |
| Q19 | Add .env to .gitignore (verify it is already there) | 5 min | Prevent future credential commits |
| Q20 | Reduce auth session maxAge from 30 days to 8 hours | 15 min | Session security improvement |

---

### Top 20 Production Risks

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | Delta Exchange API credentials in .env compromised | CRITICAL | Rotate immediately. Enable Delta IP whitelist. |
| R2 | Unauthenticated Go engine endpoints | CRITICAL | /api/delta-live/enable allows enabling live trading — attackers can open real positions |
| R3 | 4.2s AI throttle causes stale signal execution | CRITICAL | At 0.12% SL, stale price consumes entire stop budget |
| R4 | Paper OMS state lost on Docker container restart without volume mount | HIGH | All open positions and trade history lost |
| R5 | Kill switch not wired to trading loop | HIGH | Emergency halt button does nothing in production |
| R6 | position correlation — 5 long BTC strategies = 5× concentration | HIGH | Single adverse candle wipes all 5 positions, triggers daily loss circuit breaker |
| R7 | MongoDB logs collection unbounded growth | HIGH | Atlas costs grow linearly with uptime |
| R8 | Alpha strategies blocked by NO_APPROVED_STRATEGY at cold start | HIGH | 9 new strategies contribute zero fills until research tournament runs 12 trade cycles |
| R9 | OMS v3 vs paper_oms dual-track — state divergence | HIGH | Two OMSes tracking state independently leads to reconciliation failures |
| R10 | AI council single point of failure | HIGH | If OpenAI + fallback chain all fail, no new positions open |
| R11 | ANGELONE_TOTP_SECRET leaked = live equities account takeover | HIGH | Rotate immediately |
| R12 | TimescaleDB hypertable not configured — tick queries O(n) | MEDIUM | Time-series performance degrades past 1M rows without hypertable |
| R13 | Cold-start candle gap — garbage signals for 52 minutes | MEDIUM | Strategy promotes from garbage signals at cold start contaminate research |
| R14 | Funding rate not modeled — expectancy inflated | MEDIUM | System appears profitable; live trading underperforms paper by 0.03-0.05%/day |
| R15 | No OOS split in research engine — overfitting | MEDIUM | Strategies promoted from in-sample data may fail in live trading |
| R16 | Elite V2/V3 ~200 parameter variations — overfit promotion | MEDIUM | Any promoted Elite V2/V3 variant is likely overfit to paper period |
| R17 | No CI/CD — breaking changes deployed without test validation | MEDIUM | Manual deploys skip test gate |
| R18 | Browser localStorage stores position data — XSS risk | LOW | Any XSS in dashboard exposes trading state |
| R19 | Docker Grafana admin:admin credentials | LOW | Monitoring exposed on port 3001 with default password |
| R20 | No automated DB backup — SQLite volume loss = total data loss | LOW | Regular volume snapshot or pg_dump cron needed |

---

### Top 20 Upgrades (in recommended order)

| # | Upgrade | Impact | Time |
|---|---|---|---|
| U1 | Wire ML classifier as primary AI decision path (bypass LLM for rule-based decisions) | CRITICAL — eliminates 4.2s throttle | 2 days |
| U2 | Add Kelly Criterion per-strategy position sizing | HIGH — optimal sizing increases expectancy 20–40% | 3 days |
| U3 | Implement OOS gate in research tournament (holdout last 30% of trades) | HIGH — prevents overfit promotion | 3 days |
| U4 | Add VaR/CVaR daily risk dashboard widget | HIGH — institutional reporting | 3 days |
| U5 | Add position correlation matrix — prevent 5 correlated longs | HIGH — eliminates concentration risk | 2 days |
| U6 | Add funding rate to paper OMS PnL calculation | HIGH — corrects inflated expectancy | 2 days |
| U7 | Decompose mockTradingEngine.ts into modules | MEDIUM — enables testing, prevents regression | 3 days |
| U8 | Set up CI/CD (GitHub Actions: go test + next build on PR) | MEDIUM — prevents broken deploys | 1 day |
| U9 | Provision Grafana dashboards for all Prometheus metrics | MEDIUM — operational visibility | 1 day |
| U10 | Add drawdown-adjusted position sizing (50% at 5%, 25% at 7%) | MEDIUM — standard institutional risk management | 2 days |
| U11 | Replace Elite V2/V3 bulk with 20 walk-forward optimized parameter sets | MEDIUM — reduces overfit risk | 2 weeks |
| U12 | Implement anchored VWAP (from swing points, not just session) | MEDIUM — higher-quality VWAP reference | 3 days |
| U13 | Add regime-conditional strategy routing (confirm RegimeStrategyRouter wired) | MEDIUM — activates breakout on session open, mean-rev in ranging | 2 days |
| U14 | Add session-aware strategy activation (verify alpha/session/ wired) | MEDIUM — Asia/London/NY session edge | 1 day |
| U15 | Add T-statistic on strategy edge (is win rate statistically significant?) | MEDIUM — prevents false positive promotions | 2 days |
| U16 | Add order book depth visualization on dashboard | MEDIUM — missing from all institutional platforms | 1 week |
| U17 | Implement WebSocket reconnect with exponential backoff (Binance/Coinbase) | MEDIUM — prevents silent data feed gaps | 1 day |
| U18 | Add circuit breaker on AI provider failures (don't block trading if OpenAI is down) | MEDIUM — resilience | 1 day |
| U19 | Add live vs paper divergence tracker | HIGH for production readiness decision | 3 days |
| U20 | Add multi-tenancy to paper OMS (user ID on every position) | LOW now, CRITICAL for scaling | 1 week |

---

### Exact Implementation Order for a New Principal Engineer

**Week 1 — Security (non-negotiable before any live trading)**
1. B1: Rotate all credentials
2. B3/B4: Move secrets to env management, purge git history
3. B9: Redis AUTH
4. Q8: Add engine auth header check
5. B15/B16/B17: Session cookie + rate limiting + Docker non-root

**Week 2 — Integration verification**
6. B5: Wire kill switch to trading loop
7. B6: Decide OMS v3 vs legacy paper_oms — pick one, deprecate other
8. B7: Apply TimescaleDB schema, verify hypertable
9. Q10: Verify timescale hypertable
10. B11/B12: MongoDB compound indexes + TTL

**Week 3 — Alpha integration + performance**
11. B8: Register alpha strategies in strategy/registry.go
12. B13: Candle warmup enforcement for alpha strategies
13. B10: Confirm ML classifier trained, wire to agent call path
14. Q13: Parallelize Bull+Bear+Macro agents
15. B20: Wire performance/ market_data_bus + strategy_scheduler

**Week 4 — Risk + analytics gaps**
16. B19: Funding rate in paper OMS PnL
17. U2: Kelly Criterion sizing
18. U5: Position correlation matrix
19. U3: OOS gate in research tournament
20. U4: VaR dashboard widget

**Week 5 — Operations**
21. U8: CI/CD (GitHub Actions)
22. U9: Grafana dashboards
23. B18: Reconciliation exchange connector
24. U17: WebSocket reconnect backoff
25. U18: AI provider circuit breaker

---

*This document was generated from direct inspection of 60+ source files across the Go engine, Next.js client, Python service, infrastructure schemas, and environment configuration. All findings reference actual file paths, function signatures, and struct names. Items explicitly marked "NOT CONFIRMED" or "NOT IMPLEMENTED" require codebase verification before being treated as production-ready capabilities.*

*Engine: RAIG Autonomous Trading Engine v6.0 (Phase 14) | Inspected: 2026-06-01*
