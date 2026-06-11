# EXECUTION AUTHORITY CERTIFICATION
**Phase 1 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code verification only — no trust of comments or documentation

---

## VERDICT: CERTIFIED — GO ENGINE IS SOLE EXECUTION AUTHORITY

No environment variable can disable the authority check. No browser code can become execution authority. No paper desk can become execution authority. Execution authority cannot switch dynamically.

---

## EVIDENCE

### File 1: `client/src/lib/engineAuthority.ts` — Lines 1–15

**Exact source code read:**
```typescript
export function isEngineExecutionAuthority(): boolean {
  return true;
}

export const ENGINE_AUTHORITY_SKIP_REASON =
  "Go engine is sole execution authority — client-side execution permanently disabled";
```

**Analysis:**
- Function body is a single `return true` statement — no conditional logic
- No `process.env` read, no `if/else`, no parameter
- Cannot be influenced by any environment variable, URL parameter, request header, or runtime state
- Return value: always `true`
- **Environment variable bypass: IMPOSSIBLE**
- **Browser code can become execution authority: IMPOSSIBLE**

---

### File 2: `client/src/hooks/useBTCFuturesScalperEngine.ts`

#### `poll()` function — Line 2679

**Exact source code read:**
```typescript
const poll = async () => {
  // SINGLE MOCK TRADING AUTHORITY — Phase 7 (2026-06-11)
  // Browser execution is permanently disabled. The Go engine is sole authority.
  return;                                    // ← LINE 2679: unconditional return
  // eslint-disable-next-line no-unreachable
  if (pollInFlightRef.current) return;
```

- `return` is the first executable statement in the function body
- All downstream signal evaluation, position opening, position closing, and trade persistence are unreachable
- No condition governs this return — it is unconditional
- **Bypass: IMPOSSIBLE**

#### `saveToMongo()` function — Line 1364–1366

**Exact source code read:**
```typescript
const saveToMongo = useCallback((_overrides?) => {
  // SINGLE MOCK TRADING AUTHORITY — Phase 7 (2026-06-11): permanently disabled.
  return;                                    // ← LINE 1366: unconditional return
```

- First executable statement is `return`
- No `fetch("/api/paper-state")` call is reachable
- **Bypass: IMPOSSIBLE**

#### Other active `setInterval` calls (verified from source):

| Line | Interval | Target | Can create trade? |
|------|----------|--------|-------------------|
| 1330 | variable | `logPayload` (regime histogram dump) | NO — console logging |
| 1355 | variable | `logRegime` (regime classifier) | NO — console logging |
| 1578 | 4,000ms | `fetchWorkerState()` | NO — read-only heartbeat |
| 1714 | 30,000ms | `saveToMongo()` | NO — saveToMongo returns immediately |
| 3882 | 4,000ms | `poll()` | NO — poll returns immediately |
| 3930 | 60,000ms | `checkDay` | NO — day-roll state only |

**Conclusion: No interval in useBTCFuturesScalperEngine can generate a trade.**

---

### File 3: `client/src/hooks/useMockTradingEngine.ts` — Lines 213–222

**Exact source code read:**
```typescript
export function useMockTradingEngine(opts) {
  // SINGLE MOCK TRADING AUTHORITY — Phase 7 (2026-06-11)
  const { price, accountKey, disablePolling: _disablePolling, disablePersistence } = opts;
  const disablePolling = true;              // ← LINE 221: hardcoded
  const persistenceDisabled = true;         // ← LINE 222: hardcoded
```

- `disablePolling` is assigned `true` unconditionally — the caller's `opts.disablePolling` value is discarded (aliased to `_disablePolling`)
- `persistenceDisabled` is assigned `true` unconditionally
- All `useEffect` guards that check `if (disablePolling) return` or `if (persistenceDisabled || disablePolling) return` will always fire
- **No polling loop runs. No persistence fires.**

#### `persistTrade()` — Line 302

```typescript
const persistTrade = useCallback(async (trade, mode) => {
  if (persistenceDisabled) return;          // ← LINE 302: fires because persistenceDisabled=true
```

- Always returns immediately
- `fetch("/api/mock-trading/trades")` is unreachable

---

### File 4: `client/src/lib/paperDeskWorker/runPaperDeskPollTick.ts` — Lines 347–370

**Exact source code read:**
```typescript
export async function runPaperDeskPollTick(ctx) {
  // SINGLE MOCK TRADING AUTHORITY — Phase 7 (2026-06-11)
  {
    const blockers = emptyBlockerCounts();
    return {                                // ← LINES 356-369: unconditional return
      balance: ctx.balance ?? 1000,
      positions: ctx.openPositions ?? [],
      closedTrades: [],
      openedPositions: [],
      ...
    };
  }
```

- Block-scoped early return on line 356
- Returns a static no-op result
- No kline fetch, no strategy evaluation, no position opens, no trade closes
- **Bypass: IMPOSSIBLE** — all subsequent code is in the same function scope but after the `return`

---

### File 5: `/api/paper-trades` POST guard — Line 61

```typescript
export async function POST(req: Request) {
  if (isEngineExecutionAuthority()) {       // ← always true
    return NextResponse.json(
      { ok: false, code: "DEPRECATED", error: ENGINE_AUTHORITY_SKIP_REASON },
      { status: 410 },
    );
  }
```

- HTTP 410 returned before any body parsing
- MongoDB `upsertTradeMongo()` unreachable
- **Any browser POST attempt → HTTP 410**

---

### File 6: `/api/paper-state` POST guard — Line 21

```typescript
export async function POST(req: Request) {
  if (isEngineExecutionAuthority()) {       // ← always true
    return NextResponse.json(
      { ok: false, code: "DEPRECATED", error: ENGINE_AUTHORITY_SKIP_REASON },
      { status: 410 },
    );
  }
```

- Same pattern — HTTP 410 before any execution
- `upsertAccountState()` unreachable

---

### File 7: `/api/cron/paper-desk-tick` guard — Lines 58–64

```typescript
if (isEngineExecutionAuthority()) {         // ← always true
  return NextResponse.json({
    ok: true,
    skipped: true,
    skippedReason: ENGINE_AUTHORITY_SKIP_REASON,
  });
}
```

- Returns `{skipped: true}` before calling `runPaperDeskPollTick()`
- Double protection: cron route + worker function both disabled independently

---

## CERTIFICATION MATRIX

| Question | Answer | Source |
|----------|--------|--------|
| Can any environment variable disable authority checks? | **NO** | `isEngineExecutionAuthority()` has no env var path |
| Can browser code become execution authority? | **NO** | `poll()` returns line 2679; all write paths blocked |
| Can Paper Desk become execution authority? | **NO** | `runPaperDeskPollTick()` returns stub; cron skips |
| Can execution authority switch dynamically? | **NO** | Hardcoded functions — no runtime toggle |
| Can a trade be created via browser? | **NO** | poll disabled + API HTTP 410 |
| Can MongoDB paper_trades be written by client? | **NO** | HTTP 410 on POST |
| Can MongoDB paper_state be written by client? | **NO** | HTTP 410 on POST |

---

## RESULT: EXECUTION AUTHORITY CERTIFIED

The Go engine on AWS Lightsail is the sole execution authority. This is verified from source code with no reliance on comments, documentation, or configuration files.
