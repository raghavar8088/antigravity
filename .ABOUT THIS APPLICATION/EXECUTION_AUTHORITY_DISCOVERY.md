# EXECUTION AUTHORITY DISCOVERY REPORT
**Phase 1 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**27 distinct execution paths identified.**
**Multiple trading authorities confirmed — remediation required.**

---

## EXECUTION PATHS INVENTORY

### TIER 1 — GO INSTITUTIONAL ENGINE (Approved Authority)

| ID | File | Function | Type | Status |
|----|------|----------|------|--------|
| E01 | `engine/internal/trading/loop.go:327` | `executeThroughInstitutionalPath` | Paper trade execution gate | ACTIVE |
| E02 | `engine/internal/execution/paper.go:137` | `ExecuteSignal` | Paper broker fill | ACTIVE |
| E03 | `engine/internal/execution/binance_live.go:31` | `ExecuteSignal` | Live Binance execution | ACTIVE (live key required) |
| E04 | `engine/internal/positions/manager.go:125` | `OpenPosition` | Position lifecycle | ACTIVE |
| E05 | `engine/internal/omsv3/authority.go:159` | `RecordOrderCreated` | OMS event sourcing | ACTIVE |
| E06 | `engine/internal/delta/live_bridge.go:85` | `Bridge.OnOpen/OnClose` | Delta options execution | ACTIVE |
| E07 | `engine/internal/executiongateway/handler.go:28` | `ServeHTTP` | HTTP execution gateway | ACTIVE |
| E08 | `engine/internal/admin/killswitch.go:61` | `HandleTrigger` | Emergency flatten | ACTIVE |

### TIER 2 — GO ENGINE ANCILLARY (Controlled by E01 gate)

| ID | File | Function | Type | Status |
|----|------|----------|------|--------|
| E09 | `engine/internal/risk/gate/pipeline.go:46` | `Check` | Pre-trade risk block | ACTIVE |
| E10 | `engine/internal/trading/loop.go:463` | PMS gate | Portfolio-level veto | ACTIVE |
| E11 | `engine/internal/killswitch/service.go:137` | `Trigger` | Circuit breaker | ACTIVE |
| E12 | `engine/internal/trading/loop.go:340` | `ExecuteEmergencyFlatten` | Force-close all | ACTIVE |
| E13 | `engine/internal/reconciliationv2/` | Wiring | Desync kill-switch trigger | ACTIVE |
| E14 | `engine/cmd/antigravity/main.go:954` | Options engines x4 | BTC/NIFTY options accounts | ACTIVE |

### TIER 3 — GO ENGINE SIMULATION (Backtest only)

| ID | File | Function | Type | Status |
|----|------|----------|------|--------|
| E15 | `engine/cmd/backtest/main.go` | `main` | Backtest runner | SIMULATION ONLY |
| E16 | `engine/internal/backtest/v3/engine.go:47` | `Run` | V3 backtest engine | SIMULATION ONLY |

### TIER 4 — CLIENT-SIDE BROWSER EXECUTION (UNAUTHORIZED — MUST REMOVE)

| ID | File | Function | Type | Status |
|----|------|----------|------|--------|
| **E17** | `client/src/hooks/useBTCFuturesScalperEngine.ts` | Full execution loop | Browser paper trade generator | **ACTIVE — VIOLATION** |
| **E18** | `client/src/hooks/useMockTradingEngine.ts` | `ingestTraceRows` | Browser mock trade generator | **ACTIVE — VIOLATION** |
| **E19** | `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` | `runPaperDeskPollTick` | Server/cron paper desk engine | **ACTIVE — VIOLATION** |
| **E20** | `client/src/app/api/cron/paper-desk-tick/route.ts` | Cron handler | Vercel cron paper desk | **ACTIVE — VIOLATION** |
| **E21** | `client/src/lib/mongoTradesClient.ts` | `upsertTradeMongo` | Browser MongoDB trade writer | **ACTIVE — VIOLATION** |
| **E22** | `client/src/lib/paperTradesSync.ts` | `persistTradeToServer` | Client trade persistence | **ACTIVE — VIOLATION** |
| **E23** | `client/src/lib/paperOms.ts` | Order state machine | Client-side OMS | **ACTIVE — VIOLATION** |
| **E24** | `client/src/lib/paperOmsMongo.ts` | MongoDB OMS writes | Client OMS persistence | **ACTIVE — VIOLATION** |
| **E25** | `client/src/hooks/useBTCSpotScalperEngine.ts` | Spot execution loop | Browser spot paper engine | **REQUIRES AUDIT** |
| **E26** | `client/src/hooks/useMCXEngine.ts` | MCX execution | Browser MCX paper engine | **REQUIRES AUDIT** |
| **E27** | `client/src/hooks/useCryptoEquityEngine.ts` | Crypto equity engine | Browser crypto paper engine | **REQUIRES AUDIT** |

---

## AUTHORIZED EXECUTION PATH

```
Market Data (Coinbase WS / Binance REST / NSE)
  ↓
Strategy Registry (engine/internal/strategy/) — 600+ strategies
  ↓
Orchestrator Trading Loop (engine/internal/trading/loop.go)
  ↓
Portfolio Management System Gate (loop.go:463) — heat/VaR/drawdown limits
  ↓
Pre-Trade Risk Pipeline (engine/internal/risk/gate/pipeline.go) — Kelly/confidence/loss limits
  ↓
Kill Switch Check (engine/internal/killswitch/service.go) — IsActive() blocks all
  ↓
OMS v3 Ledger (engine/internal/omsv3/authority.go) — event-sourced authority
  ↓
executeThroughInstitutionalPath (loop.go:327)
  ↓
Paper Execution Client (engine/internal/execution/paper.go) OR Binance Live
  ↓
Position Manager (engine/internal/positions/manager.go)
  ↓
PnL Calculation (paperpersist_hooks.go) → Ledger → MongoDB
  ↓
Next.js API proxy → Dashboard (read-only)
```

---

## FINDINGS SUMMARY

- **Authorized execution paths:** 14 (E01–E14)
- **Unauthorized client-side execution paths:** 11 (E17–E27)
- **Simulation-only paths:** 2 (E15–E16)
- **Critical violations:** E17, E18, E19, E20 — all actively generating trades

---

## NEXT ACTION

Remove E17–E27. Convert client to read-only UI consumer.
