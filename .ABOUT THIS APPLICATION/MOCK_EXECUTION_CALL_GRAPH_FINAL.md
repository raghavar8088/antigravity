# MOCK EXECUTION CALL GRAPH — FINAL
**Phase 11 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**CERTIFIED — Single execution call graph with no alternative paths.**

---

## CANONICAL EXECUTION CALL GRAPH

```
Market Data Sources
├── Coinbase WebSocket (BTC-USD price feed)
├── Binance REST (BTC spot fallback)
├── Delta Exchange REST (BTC options ticks)
├── NSE REST (NIFTY index quotes)
└── AngelOne (NSE/BSE equity + NIFTY)
          │
          ▼
engine/internal/trading/loop.go (Run function)
  ├── processTickPipeline (goroutine)
  ├── process1mCandles (goroutine)
  ├── process5mCandles (goroutine)
  └── processAIDecisions (goroutine — advisory only)
          │
          ▼
Strategy Registry: engine/internal/strategy/curated_registry.go
  └── 600+ strategies: OnTick() / OnCandle()
  └── Signals: []strategy.Signal{Action, Confidence, StopLossPct, TakeProfitPct}
          │
          ▼
[GATE 1] Portfolio Management System (loop.go:463)
  ├── Heat % check
  ├── VaR 95% check
  ├── CVaR 95% check
  ├── Max drawdown check
  └── Daily loss limit check
  → REJECT if any limit exceeded → OMSRejected recorded to MongoDB
          │
          ▼
[GATE 2] Pre-Trade Risk Pipeline (engine/internal/risk/gate/pipeline.go:46)
  ├── Kill switch IsActive() check → BLOCK if active
  ├── Signal validation (symbol, entry, SL, size)
  ├── Kelly fraction sizing
  ├── Confidence threshold (≥ 0.68)
  ├── Risk/reward ratio (≥ 2.4)
  ├── Family concentration limits
  └── Drawdown scaling
  → Decision{Approved/Blocked, RiskDecision}
          │
          ▼
[GATE 3] Kill Switch (engine/internal/killswitch/service.go)
  └── IsActive() — blocks all orders if triggered
  └── Survives restarts via PostgreSQL ledger replay
          │
          ▼
OMS v3 Ledger (engine/internal/omsv3/authority.go)
  ├── RecordOrderCreated() → EventOrderCreated emitted
  ├── RecordOrderValidated() → EventOrderValidated emitted
  └── In-memory projections updated (orderProjs, positionProjs)
          │
          ▼
executeThroughInstitutionalPath (loop.go:327) ← SOLE EXECUTION ENTRY POINT
          │
          ▼
Execution Client (implements Engine interface)
  ├── Paper: engine/internal/execution/paper.go:137
  │     └── ExecuteSignal(signal, entry)
  │           ├── Apply slippage (OrderMode: MARKET/POST_ONLY/IOC)
  │           ├── Deduct Binance taker fee (0.05%)
  │           ├── Update signed BTC position balance
  │           └── Record fill in trade journal
  └── Binance Live: engine/internal/execution/binance_live.go:31
        └── ExecuteSignal(signal, entry)
              ├── HMAC-SHA256 sign request
              └── POST /api/v3/order to Binance REST
          │
          ▼
Position Manager (engine/internal/positions/manager.go:125)
  └── OpenPosition(signal, entryPrice, stratName)
        ├── Create Position{ID, symbol, side, entry, SL, TP, strategy}
        ├── Store in positions map
        └── Emit to paperpersist hooks
          │
          ▼
PnL Calculation (engine/internal/trading/paperpersist_hooks.go)
  ├── On every tick: computeUnrealizedPnL(openPositions, markPrice)
  ├── On close: calculatePnL(pos, exitPrice)
  └── Every 10s: GetAccountSnapshot() → MongoDB paper_state upsert
          │
          ▼
OMS v3 Ledger — Position Events
  ├── RecordPositionOpened() → EventPositionOpened
  └── RecordPositionClosed() → EventPositionClosed
  └── Immutable in PostgreSQL + replicated to MongoDB
          │
          ▼
MongoDB Persistence
  ├── paper_trades (closed trades)
  ├── paper_state (account snapshot)
  ├── paper_positions (open position cache)
  ├── equity_curve (1-min snapshots)
  ├── portfolio_metrics (30-min snapshots)
  └── strategy_health (15-min snapshots)
          │
          ▼
Next.js API Routes (read-only proxies)
  ├── /api/engine/[...path] → forwards to Go engine REST
  ├── /api/paper-desk/snapshot → MongoDB aggregation read
  ├── /api/paper-desk/trades → MongoDB paper_trades read
  ├── /api/paper-desk/positions → MongoDB paper_positions read
  └── /api/paper-desk/equity → MongoDB equity_curve read
          │
          ▼
Dashboard UI (read-only consumer)
  ├── Positions panel → usePositions.ts
  ├── Trade history → useTrades.ts
  ├── PnL metrics → usePaperDesk.ts
  ├── Strategy status → useStrategies.ts
  ├── Equity curve → usePaperDesk.ts
  └── Kill switch → useEngineState.ts
```

---

## PROHIBITED PATHS (Confirmed Absent)

```
❌ Browser → executeTrade() — DOES NOT EXIST (poll disabled)
❌ Browser → MongoDB paper_trades POST — BLOCKED (HTTP 410)
❌ Browser → MongoDB paper_state POST — BLOCKED (HTTP 410)
❌ Vercel cron → runPaperDeskPollTick() — SKIPPED (isEngineExecutionAuthority)
❌ Mock engine → persistTrade() — BLOCKED (persistenceDisabled=true)
❌ Signal → execution without PMS gate — IMPOSSIBLE (gate is in the only path)
❌ Signal → execution without kill switch check — IMPOSSIBLE (kill switch is in gate)
❌ Position → MongoDB without Go engine — IMPOSSIBLE (only Go writes paper_positions)
```

---

## CERTIFICATION

The canonical execution call graph has exactly ONE entry point for trade creation:
`executeThroughInstitutionalPath()` in `engine/internal/trading/loop.go:327`.

No alternative paths exist. This is certified as of 2026-06-11.
