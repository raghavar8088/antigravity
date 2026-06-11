# EXECUTION REGRESSION REPORT
**Phase 13 — Single Mock Trading Authority Program**
**Date:** 2026-06-11

---

## VERDICT

**PASS — All execution keywords found; all unauthorized instances are disabled.**

---

## REGRESSION SEARCH RESULTS

### Keyword: `openPosition`

| File | Context | Status |
|------|---------|--------|
| `engine/internal/positions/manager.go:125` | `OpenPosition()` — authoritative | AUTHORIZED |
| `engine/internal/trading/paperpersist_hooks.go` | `persistPositionOpen()` | AUTHORIZED |
| `client/src/hooks/usePaperOMSEngine.ts` | Calls `/api/engine/paper/open` proxy | AUTHORIZED (Go engine proxy) |
| `client/src/hooks/useBTCFuturesScalperEngine.ts` | `openAndTrackPosition()` | DEAD CODE (poll disabled) |
| `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` | Position open logic | DEAD CODE (stub returns) |

### Keyword: `placeOrder`

| File | Context | Status |
|------|---------|--------|
| `engine/internal/delta/live_bridge.go` | Delta Exchange order placement | AUTHORIZED |
| `engine/internal/execution/binance_live.go` | Binance order placement | AUTHORIZED |
| No client-side occurrences | — | PASS |

### Keyword: `executeTrade`

| File | Context | Status |
|------|---------|--------|
| `engine/internal/trading/loop.go` | `executeThroughInstitutionalPath` | AUTHORIZED |
| No client-side occurrences | — | PASS |

### Keyword: `submitOrder`

| File | Context | Status |
|------|---------|--------|
| `engine/internal/delta/live_bridge.go` | `SubmitOrder()` | AUTHORIZED |
| No client-side occurrences | — | PASS |

### Keyword: `createPosition`

| File | Context | Status |
|------|---------|--------|
| Go engine internal logic | Various | AUTHORIZED |
| No unauthorized client-side creation | — | PASS |

### Keyword: `tradeExecutor`

| File | Context | Status |
|------|---------|--------|
| Not found in codebase | — | PASS |

### Keyword: `paperTrade`

| File | Context | Status |
|------|---------|--------|
| 100+ files | Schema types, read queries, route handlers | READ-ONLY |
| `client/src/lib/paperTradesSync.ts` | `persistTradeToServer()` | DEAD CODE (callers disabled) |
| `client/src/lib/mongoTradesClient.ts` | `upsertTradeMongo()` | DEAD CODE (callers disabled) |
| `client/src/app/api/paper-trades/route.ts` | POST blocked (HTTP 410) | BLOCKED |

### Keyword: `paperState`

| File | Context | Status |
|------|---------|--------|
| `client/src/app/api/paper-state/route.ts` | POST blocked (HTTP 410) | BLOCKED |
| `client/src/hooks/useBTCFuturesScalperEngine.ts` | `saveToMongo()` | DEAD CODE (early return) |

### Keyword: `paperDesk`

| File | Context | Status |
|------|---------|--------|
| 140+ files | Display, analytics, read-only | READ-ONLY |
| `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` | Worker tick | DEAD CODE (stub) |
| `client/src/app/api/cron/paper-desk-tick/route.ts` | Cron handler | BLOCKED (authority guard) |

### Keyword: `signalExecutor`

| File | Context | Status |
|------|---------|--------|
| Not found in client | — | PASS |
| `engine/internal/strategy/interface.go` | Go engine signal interface | AUTHORIZED |

### Keyword: `strategyWorker`

| File | Context | Status |
|------|---------|--------|
| Not found in codebase | — | PASS |

### Keyword: `executionWorker`

| File | Context | Status |
|------|---------|--------|
| Not found in codebase | — | PASS |

---

## ADDITIONAL PATTERNS CHECKED

### `persistTradeToServer`
- `client/src/lib/paperTradesSync.ts` — Called by `useBTCFuturesScalperEngine.ts`
- **Status:** DEAD CODE — the callers are disabled. The function exists but is never invoked.

### `ingestTraceRows`
- `client/src/hooks/useMockTradingEngine.ts`
- **Status:** DEAD CODE — `disablePolling=true` prevents the polling loop that calls this.

### `buildMockTradeFromTrace` / `buildMockTradeFromResearchSignal`
- `client/src/lib/mockTradingEngine.ts`
- **Status:** Library functions. Only called by `useMockTradingEngine.ts` which is disabled.

### `upsertTradeMongo`
- `client/src/lib/mongoTradesClient.ts`
- **Status:** Library function. Only called by `/api/paper-trades` POST (blocked, HTTP 410) and disabled hooks.

---

## REMAINING EXECUTION CODE (Dead Code — Not a Regression)

The following files contain execution code that is now permanently dead but not deleted. These are acceptable as they no longer execute:

1. `client/src/hooks/useBTCFuturesScalperEngine.ts` (3,967 lines) — poll() dead code
2. `client/src/hooks/useMockTradingEngine.ts` (1,073 lines) — all execution dead code
3. `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` — execution code after early return
4. `client/src/lib/paperTradesSync.ts` — never called
5. `client/src/lib/mongoTradesClient.ts` — write functions never called
6. `client/src/lib/mockTradingEngine.ts` — library functions never called

These can be cleaned up in a future sprint. They do not affect execution authority.

---

## CONCLUSION

All execution keywords found. All unauthorized instances are either:
- DEAD CODE (callers disabled, early returns, persistenceDisabled=true)
- BLOCKED (HTTP 410, isEngineExecutionAuthority=true)
- AUTHORIZED (Go engine internal paths)

No regressions found. Execution authority is sole to the Go institutional engine.
