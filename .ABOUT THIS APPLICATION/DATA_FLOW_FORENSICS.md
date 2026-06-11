# DATA FLOW FORENSICS
**Phase 7 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification only

---

## COMPLETE CALL GRAPH: MARKET DATA → UI

```
LAYER 0: MARKET DATA INGESTION
══════════════════════════════════════════════════════════════
  Coinbase WS (BTC-USD price feed, public)
      └─→ engine/internal/feed/coinbase_ws.go
              └─→ broadcast to registered strategy callbacks
  
  Binance REST (BTC spot fallback)
      └─→ engine/internal/feed/binance_rest.go
              └─→ broadcast (only if Coinbase WS fails)
  
  Delta Exchange REST (BTC options ticks)
      └─→ engine/internal/feed/delta_klines.go
              └─→ kline bars to paper trading engine
  
  AngelOne (NSE/BSE equity + NIFTY)
      └─→ engine/internal/feed/angelone.go
              └─→ broadcast to NSE strategy callbacks
  
  Yahoo Finance (NIFTY warmup bars)
      └─→ engine/internal/feed/yahoo.go (startup only)

LAYER 1: STRATEGY EVALUATION
══════════════════════════════════════════════════════════════
  engine/internal/strategy/curated_registry.go
      ├─→ 600+ Strategy.OnTick(price) / Strategy.OnCandle(bar)
      │       Each strategy returns: Signal{Side, Confidence, StrategyID, RegimeTag}
      │
      └─→ engine/internal/strategy/regime_classifier.go
              └─→ market regime annotation appended to signal

LAYER 2: RISK GATE
══════════════════════════════════════════════════════════════
  engine/internal/risk/gate/pipeline.go
      ├─→ Stage 1: Kelly sizing (position size = Kelly fraction × balance × leverage)
      ├─→ Stage 2: Confidence threshold gate (reject if confidence < threshold)
      ├─→ Stage 3: Strategy family limits (max concurrent positions per family)
      ├─→ Stage 4: Portfolio-level PMS heat / VaR / drawdown gate
      └─→ Stage 5: Kill switch check (engine/internal/killswitch/service.go)
              └─→ [signal passes all gates] → approved, sized signal

LAYER 3: OMS V3
══════════════════════════════════════════════════════════════
  engine/internal/omsv3/authority.go
      ├─→ RecordOrderCreated(signal) → EventOrderCreated in PostgreSQL ledger
      ├─→ RecordOrderValidated()     → EventOrderValidated
      └─→ RecordPositionOpened()     → EventPositionOpened
              │
              └─→ In-memory projections updated:
                      - orderProjs map[OrderID]*OrderProjection
                      - positionProjs map[PositionID]*PositionProjection

LAYER 4: EXECUTION (PAPER BROKER)
══════════════════════════════════════════════════════════════
  engine/internal/trading/loop.go
      └─→ paperBroker.PlaceOrder(sized_signal)
              ├─→ Slippage model applied (basis points)
              ├─→ Fee model applied (May 2026 formula)
              ├─→ Fill price calculated
              └─→ positions/manager.go:OpenPosition(fill)
                      ├─→ In-memory position map updated (authoritative)
                      └─→ OMS v3: RecordPositionOpened()

LAYER 5: POSITION LIFECYCLE
══════════════════════════════════════════════════════════════
  engine/internal/positions/manager.go
      ├─→ Per-tick mark-to-market (unrealized PnL updated in memory)
      ├─→ SL/TP/TIME/TRAIL/BREAKEVEN check on every price tick
      └─→ On exit trigger:
              ├─→ calculatePnL(entry, exit, side, size) → realized PnL
              ├─→ OMS v3: RecordPositionClosed()
              └─→ paperpersist_hooks.go:OnTradeClosed(closedTrade)

LAYER 6: PERSISTENCE PIPELINE
══════════════════════════════════════════════════════════════
  engine/internal/paperpersist/paperpersist_hooks.go
      ├─→ OnTradeClosed(t):
      │       ├─→ MongoDB paper_trades.insert(t)        [immediate]
      │       ├─→ MongoDB equity_curve.insert(snapshot) [every 1 min]
      │       └─→ daily_pnl_history update              [midnight UTC]
      │
      ├─→ state_snapshotter.go (every 10s):
      │       └─→ MongoDB paper_state.upsert({
      │               balance, unrealized_pnl, realized_pnl,
      │               open_positions[], equity, open_count
      │           })
      │
      ├─→ portfolio_metrics_writer.go (every 30 min):
      │       └─→ MongoDB portfolio_metrics.insert(aggregated_stats)
      │
      └─→ order_writer.go:
              └─→ MongoDB paper_orders.insert(order_event)

  PostgreSQL (OMS v3 event ledger — always written):
      └─→ events table: EventOrderCreated, EventOrderValidated,
                        EventPositionOpened, EventPositionClosed,
                        EventKillSwitchTriggered

LAYER 7: RECONCILIATION
══════════════════════════════════════════════════════════════
  engine/internal/reconciliation/ (every 60s):
      ├─→ Compare in-memory positions vs MongoDB paper_state
      ├─→ If drift > threshold → trigger kill switch
      └─→ Kill switch: RecordKillSwitchEvent(PostgreSQL) + halt all new orders

LAYER 8: API EXPOSURE (Go Engine HTTP Server)
══════════════════════════════════════════════════════════════
  engine/cmd/antigravity/main.go HTTP endpoints:
      ├─→ GET /api/stats           → journals.GetAggregateStats() + paperExecute.GetEquityUSD() + riskEngine.GetDailyPnL() [IN-MEMORY]
      ├─→ GET /api/positions       → posMgr.GetOpenPositions() [IN-MEMORY]
      ├─→ GET /api/trades          → journal.GetRecentTrades() [IN-MEMORY + DB]
      ├─→ GET /api/health          → {ok: true, ...} [IMMEDIATE]
      ├─→ GET /api/strategies      → registry.GetAll() [IN-MEMORY]
      ├─→ GET /api/risk            → riskEngine.GetMetrics() [IN-MEMORY]
      └─→ POST /api/kill-switch    → killswitch.Trigger() [DB write]

LAYER 9: NEXT.JS API PROXY
══════════════════════════════════════════════════════════════
  client/src/app/api/engine/[...path]/route.ts
      └─→ Forward to Go engine at INTERNAL_API_URL (no transformation)
              └─→ Returns raw engine JSON to browser

  client/src/app/api/paper-desk/snapshot/route.ts
      └─→ Aggregation query on MongoDB paper_state [STALE: 10s lag]

  client/src/app/api/paper-desk/trades/route.ts
      └─→ MongoDB paper_trades.find() [NEAR-REAL-TIME]

  client/src/app/api/positions/route.ts
      └─→ Forward to Go /api/positions [LIVE]

LAYER 10: REACT HOOKS (Browser)
══════════════════════════════════════════════════════════════
  usePaperDesk.ts  (5s poll interval)
      └─→ GET /api/paper-desk/snapshot → React state [STALE 15s max]

  usePositions.ts  (2s poll interval)
      └─→ GET /api/positions → React state [LIVE, 2s max]

  useEngineState.ts (10s poll interval)
      └─→ GET /api/health → engineOnline: boolean [LIVE]
      └─→ balance: HARDCODED 1000000.0 [FAKE — not from engine]

  useBTCFuturesScalperEngine.ts
      └─→ poll() → RETURNS IMMEDIATELY → no data [DISABLED]

  useMockTradingEngine.ts
      └─→ polling useEffect → RETURNS IMMEDIATELY → no data [DISABLED]

LAYER 11: DASHBOARD RENDERING
══════════════════════════════════════════════════════════════
  PaperDeskDashboard.tsx
      └─→ usePaperDesk.ts state → STALE (15s max)

  BTCFuturesScalper.tsx
      └─→ useBTCFuturesScalperEngine.ts state → EMPTY (poll disabled)

  Terminal Suite (ExecutionCenter, AnalyticsCenter, etc.)
      └─→ usePositions.ts / direct engine proxy → LIVE (2s max)
```

---

## LATENCY BUDGET (Market Event → UI Update)

| Path | Breakdown | Total |
|------|-----------|-------|
| Price tick → strategy signal | In-memory, ≈0ms | ~0ms |
| Signal → risk gate | In-memory pipeline, ≈0ms | ~0ms |
| Risk gate → OMS | In-memory, ≈0ms | ~0ms |
| OMS → position opened | In-memory, ≈0ms | ~0ms |
| Position → MongoDB paper_state | Async 10s cycle | ≤10s |
| MongoDB → Next.js API snapshot | Query on poll | +0–5s |
| Next.js API → Browser | HTTP response | +0–1s |
| Browser → React render | usePaperDesk 5s poll | +0–5s |
| **Total (PaperDeskDashboard)** | | **≤16s** |
| **Total (Terminal via /api/positions)** | | **≤2s** |
| **Total (BTCFuturesScalper)** | | **∞ (poll disabled)** |

---

## SINGLE POINTS OF FAILURE

| Component | Failure Mode | Recovery |
|-----------|-------------|---------|
| MongoDB paper_state write | Engine crash between ticks | RestorePositions() from last snapshot (may be 10s stale) |
| PostgreSQL OMS ledger | DB connection loss | Kill switch triggers; engine halts new orders |
| In-memory posMgr | Engine restart | Restored from MongoDB snapshot via RestorePositions() |
| PnL aggregation (3 sources) | Any of 3 in-memory objects diverges | No reconciliation; values may mismatch until restart |

---

## WRITE PATH PURITY

Every write to MongoDB that carries trade/position/PnL data originates from:
- `engine/internal/paperpersist/` hooks
- Called from `engine/internal/trading/loop.go` or `positions/manager.go`
- No Next.js route can write to these collections (except smoke-test gap noted in Phase 6)

**Single writer constraint: VERIFIED for all 8 MongoDB collections.**
