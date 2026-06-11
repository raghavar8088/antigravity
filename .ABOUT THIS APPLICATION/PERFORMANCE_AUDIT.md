# PERFORMANCE AUDIT
**Phase 12 — Single Mock Trading Authority Forensic Certification Program**
**Date:** 2026-06-11
**Method:** Source code analysis of hooks, intervals, and effect dependencies

---

## VERDICT: FUNCTIONAL BUT INEFFICIENT — UNNECESSARY POLLING AND DEAD INTERVALS

The application has no memory leaks from the disabled execution hooks. The dominant performance issues are unnecessary polling intervals, dead setIntervals that can't be cleaned up cleanly, and a lack of WebSocket/SSE for high-frequency data.

---

## POLLING INVENTORY

### Active Polling (contributes to network traffic)

| Hook | Interval | Endpoint | Requests/min |
|------|----------|----------|-------------|
| `usePaperDesk.ts` | 5s | `/api/paper-desk/snapshot` | 12 |
| `useEngineState.ts` | 10s | `/api/health` | 6 |
| `usePositions.ts` | 2s | `/api/positions` | 30 |
| `useTrades.ts` (Terminal) | 5s | `/api/engine/trades` | 12 |
| `useStrategies.ts` | 30s | `/api/engine/strategies` | 2 |
| `useRiskMetrics.ts` (Terminal) | 5s | `/api/engine/risk` | 12 |
| `usePortfolio.ts` | 10s | `/api/paper-desk/portfolio` | 6 |
| `useLeaderboard.ts` | 30s | `/api/paper-trades/leaderboard` | 2 |
| `useEquityCurve.ts` | 60s | `/api/paper-desk/equity` | 1 |
| `useStrategyHealth.ts` | 60s | `/api/paper-desk/strategy-health` | 1 |
| **Total** | | | **~84 requests/min** |

**Assessment:** 84 HTTP requests/minute from a single browser tab is high for a dashboard. If both PaperDeskDashboard and Terminal are open simultaneously, this doubles. No request deduplication or caching layer exists.

---

### Dead Polling (consumes CPU cycles, no data)

| Hook | Interval | Function | Effect |
|------|----------|----------|--------|
| `useBTCFuturesScalperEngine.ts` | 4s | `poll()` | Returns immediately; 15 calls/min wasted |
| `useBTCFuturesScalperEngine.ts` | 5s | `saveToMongo()` | Returns immediately; 12 calls/min wasted |
| `useBTCFuturesScalperEngine.ts` | ~60s | `fetchWorkerState()` | Read-only heartbeat — minimal cost |
| `useBTCFuturesScalperEngine.ts` | ~10s | regime histogram dump | Console log only — minimal cost |
| `useMockTradingEngine.ts` | N/A | No setInterval (guarded by effect) | Zero overhead when hook is disabled |

**Net:** The scalper engine fires ~27 pointless intervals per minute when the BTCFuturesScalper screen is mounted. These are pure CPU waste since each call returns in <1ms, but they create unnecessary React scheduling overhead.

---

## RE-RENDER ANALYSIS

### `useBTCFuturesScalperEngine.ts`

**Issue:** The hook has 3,967 lines and maintains large React state (positions array, trades array, multiple configuration objects). Even with the poll disabled:
- State initialization runs on mount (expensive for large config objects)
- `useCallback` hooks with large dependency arrays run on every render of consuming component
- The hook is still mounted when BTCFuturesScalper renders — even though it provides no data

**Recommendation:** When poll is permanently disabled, the hook should short-circuit at the top of the hook body and return empty stable refs. This would prevent all the expensive initialization.

### `usePaperDesk.ts`

**Issue:** The hook uses `setInterval` for polling. React's strict mode double-invokes effects in development. In production this is fine, but the hook does not use `useRef` to prevent double-subscriptions.

**Assessment:** Minor issue — not causing correctness problems.

---

## MEMORY LEAK ANALYSIS

### Cleared properly

| Interval | Cleanup in useEffect return? |
|----------|------------------------------|
| `usePaperDesk.ts` poll | YES — `clearInterval(id)` in cleanup |
| `usePositions.ts` poll | YES |
| `useEngineState.ts` poll | YES |
| `useBTCFuturesScalperEngine.ts` regime histogram | YES — cleanup at line ~1340 |
| `useBTCFuturesScalperEngine.ts` `poll()` interval | YES — cleanup at line ~3890 |
| `useBTCFuturesScalperEngine.ts` `saveToMongo()` interval | YES — cleanup at line ~1720 |

**No memory leaks identified.** All intervals have corresponding cleanup functions.

---

## DUPLICATE REQUESTS

**Identified:** `usePaperDesk.ts` and `usePositions.ts` can be mounted simultaneously by different components on the same page. If both PaperDeskDashboard and Terminal Suite are rendered at the same time, both hooks are active, polling overlapping data:

- `/api/paper-desk/snapshot`: polled by PaperDeskDashboard
- `/api/positions`: polled by Terminal Suite
- These return overlapping data (positions from different sources)
- No deduplication

**Impact:** In practice, these two screens are in different routes, so both are unlikely to be mounted simultaneously. But if they are (in a multi-panel layout), request count doubles.

---

## WEBSOCKET / SSE ASSESSMENT

**Current state:** REST polling only. No WebSocket or Server-Sent Events anywhere in the client stack.

**Impact:**
- Positions dashboard has 2s polling lag minimum
- Position close events don't appear until next poll
- High-frequency price updates (subsecond) are impossible via REST

**What would help:**
- SSE stream from `/api/engine/events` for position open/close events (push on event, not pull every 2s)
- WebSocket for live price tick display
- At minimum, a 1s position poll during active trading hours

---

## VERCEL FUNCTION COLD STARTS

**Risk:** `/api/paper-desk/snapshot` is a Vercel serverless function that runs a MongoDB aggregation query. If this function cold-starts on each request, the 5s polling interval may experience periodic 2–3s latency spikes.

**Mitigation available:** Vercel function warming is limited on hobby plan. Connection pooling via `lib/mongodb.ts` (cached MongoDB client) reduces cold-start impact for DB queries.

**Assessment:** Acceptable for the current usage pattern. Would become an issue under high traffic.

---

## RECOMMENDATIONS

| Priority | Issue | Fix |
|----------|-------|-----|
| P0 | Dead `poll()` interval in scalper | Return empty stable refs at top of hook when authority mode is active |
| P1 | 84 requests/min polling | Consider React Query or SWR with deduplication |
| P1 | No SSE for position events | Add SSE endpoint for position open/close push events |
| P2 | Duplicate polling when both screens mounted | Shared state context or SWR global cache |
| P3 | No WebSocket for price ticks | Add WS price feed for sub-second display if needed |
