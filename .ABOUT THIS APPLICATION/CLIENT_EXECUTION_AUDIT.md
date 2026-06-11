# CLIENT EXECUTION AUDIT
**Phase 3 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification only

---

## VERDICT: NO ACTIVE CLIENT-SIDE EXECUTION REMAINS

All execution-capable code patterns are dead code or are blocked at the API layer.

---

## PATTERN SEARCH RESULTS

### `ExecuteSignal`
- **Result:** NOT FOUND in `client/src/`
- **Verdict:** PASS

### `PlaceOrder`
- **Result:** Found in `client/src/hooks/useAngelOneOrders.ts:128` — type definition only, marked `@deprecated`
- **Reachable?** NO — deprecated type, not a function call
- **Verdict:** PASS

### `CreateOrder` / `FillOrder`
- **Result:** NOT FOUND in `client/src/`
- **Verdict:** PASS

### `openPosition`
- **Result:** `client/src/hooks/useBTCFuturesScalperEngine.ts:2555`
- **Context:**
  ```typescript
  const openPosition = useCallback((stratId, sym, side, sig, mark) => {
    // ... position creation logic ...
    persistShadowTradeIntent(...)  // only if shadow logging enabled
  }, [...]);
  ```
- **Reachable?** The callback exists but is only invoked from within `poll()` execution path. Since `poll()` returns immediately on line 2679, `openPosition()` is **NEVER CALLED**
- **Can reach fetch/DB write?** `persistShadowTradeIntent()` would POST to `/api/shadow-trade-intents` — but this call is inside `openPosition()` which is unreachable
- **Verdict:** DEAD CODE — unreachable from disabled poll

### `closePosition`
- **Result:** `client/src/hooks/useBTCFuturesScalperEngine.ts:2575`
- **Context (critical section):**
  ```typescript
  const closePosition = useCallback((posId, exitPrice, reason) => {
    // ... close logic ...
    persistTradeToServer(trade, cloudAccountKey, moduleKey);  // line ~2660
  }, [...]);
  ```
- **Reachable?** Only called from within `poll()` execution path — **NEVER CALLED** (poll returns line 2679)
- **Can reach DB write?** `persistTradeToServer()` would POST to `/api/paper-trades` → API returns HTTP 410 regardless
- **Verdict:** DEAD CODE — double protection (unreachable + API blocked)

### `executeTrade` / `submitOrder`
- **Result:** NOT FOUND in `client/src/`
- **Verdict:** PASS

### `persistTradeToServer`
- **Result:** `client/src/lib/paperTradesSync.ts:95`
- **Definition:**
  ```typescript
  export function persistTradeToServer(trade, accountKey, moduleKey) {
    // POSTs to /api/paper-trades
  }
  ```
- **Call sites:**
  - `client/src/hooks/useBTCFuturesScalperEngine.ts` — inside `closePosition()` callback (dead code, unreachable)
- **Can reach DB write?** Would POST to `/api/paper-trades` — HTTP 410 (hardcoded)
- **Verdict:** DEAD CODE at call site; API is blocked regardless

### `upsertTradeMongo`
- **Result:** `client/src/lib/mongoTradesClient.ts:140`
- **Definition:** Exported function that writes to MongoDB `paper_trades`
- **Call sites:**
  - `/api/paper-trades` route.ts — only reached if `isEngineExecutionAuthority()` is false (which it never is)
  - `/api/cron/paper-desk-tick` — only reached if `isEngineExecutionAuthority()` is false (which it never is)
  - `/api/paper-desk-smoke-test` — **RISK: conditionally active if `NEXT_PUBLIC_DESK_SMOKE_TEST=1`** (see below)
- **Verdict:** BLOCKED in all normal paths; smoke-test route is a gap (see Note 1)

### `ingestTraceRows`
- **Result:** `client/src/hooks/useMockTradingEngine.ts:477`
- **Guard:** `disablePolling=true` prevents the polling `useEffect` from calling `ingestTraceRows()`
- **In-memory creation:** If called externally, creates mock trades in React state — but `persistTrade()` always returns (persistenceDisabled=true)
- **Can reach DB write?** NO — `persistenceDisabled=true` blocks all paths
- **Verdict:** DEAD CODE for normal use; in-memory only if called manually

### `buildMockTradeFromTrace`
- **Result:** `client/src/lib/mockTradingEngine.ts:671`
- **Call sites:** Only called from `ingestTraceRows()` which is dead code
- **Verdict:** DEAD CODE

### `persistTrade`
- **Result:** `client/src/hooks/useMockTradingEngine.ts:297`
- **Guard (line 302):** `if (persistenceDisabled) return;` — fires always (persistenceDisabled=true)
- **Verdict:** DEAD CODE

---

## NOTE 1: `/api/paper-desk-smoke-test` — MEDIUM RISK

**File:** `client/src/app/api/paper-desk-smoke-test/route.ts`

**Behavior:**
- POST handler creates a synthetic test trade in MongoDB via `upsertTradeMongo()`
- Guard: Feature-gated by `NEXT_PUBLIC_DESK_SMOKE_TEST === "1"`
- Requires `accountKey` parameter in request body

**Risk Assessment:**
- If `NEXT_PUBLIC_DESK_SMOKE_TEST=1` is set in production env, this route can inject synthetic trades into `paper_trades` collection
- The injected trade is labeled but could corrupt PnL aggregations
- **Current status:** This env var is NOT set in `.env.local` → route is inactive
- **Recommendation:** Add `isEngineExecutionAuthority()` guard to this route

---

## NOTE 2: `/api/admin/migrate-owner` — MEDIUM RISK

**File:** `client/src/app/api/admin/migrate-owner/route.ts`

**Behavior:**
- POST handler performs bulk MongoDB document migration
- Renames account keys across 17 collections including `paper_trades`, `paper_state`, `mock_trades`
- Requires authenticated session (JWT cookie)
- Has dry-run mode

**Risk Assessment:**
- Does NOT create trades or positions — only renames existing documents
- Requires valid session authentication
- Not a trade execution path
- **Verdict:** SAFE for execution authority — is a data migration tool, not an execution path

---

## REACHABILITY ANALYSIS SUMMARY

| Pattern | Found | Reachable | Can Write DB | Verdict |
|---------|-------|-----------|-------------|---------|
| ExecuteSignal | NO | — | — | PASS |
| PlaceOrder | YES (type def) | NO | NO | PASS |
| openPosition | YES (callback) | NO (poll disabled) | NO (API 410) | DEAD CODE |
| closePosition | YES (callback) | NO (poll disabled) | NO (API 410) | DEAD CODE |
| persistTradeToServer | YES (function) | NO (callers dead) | NO (API 410) | DEAD CODE |
| upsertTradeMongo | YES (function) | GUARDED | BLOCKED normally | NOTE 1 |
| ingestTraceRows | YES (callback) | NO (disablePolling) | NO (persistenceDisabled) | DEAD CODE |
| buildMockTradeFromTrace | YES (function) | NO (callers dead) | NO | DEAD CODE |
| persistTrade | YES (callback) | NO (persistenceDisabled) | NO | DEAD CODE |

---

## CONCLUSION

No client-side execution remains active. All execution-capable patterns are either:
1. Not present in client code
2. Dead code (unreachable from disabled poll functions)
3. Blocked at the API layer (HTTP 410)
4. Feature-gated and inactive (smoke test)

**One gap identified:** `/api/paper-desk-smoke-test` can write a synthetic test trade if `NEXT_PUBLIC_DESK_SMOKE_TEST=1` is set. This env var is currently unset. Recommend adding `isEngineExecutionAuthority()` guard as belt-and-suspenders.
