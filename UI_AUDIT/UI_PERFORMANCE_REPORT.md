# PHASE 14 — UI PERFORMANCE REPORT
## Forensic Audit | Trading Platform | 2026-06-11

---

## AUDIT QUESTION: Can the UI sustain 72-hour autonomous monitoring without degrading?

---

## POLLING LOAD ANALYSIS

### Active Polling Inventory

| Hook / Component | Endpoint | Interval | Payload Est. |
|-----------------|----------|----------|-------------|
| `usePaperDesk` | `/api/paper-desk/snapshot` | 5s | ~5–20KB (state + positions + recent trades + health) |
| `useBTCFuturesScalperEngine` | `/api/mock-trading/signals` | 5s | ~2–5KB |
| `useLiveBTCPrice` | `/api/btc/price` | 1–5s | ~100B |
| `useNiftyMarket` | `/api/nifty/state` | 5s | ~1KB |
| `useOptionChain` | `/api/nifty/option-chain` | 5–10s | ~20–100KB |
| `useMockTradingEngine` (persist) | `/api/mock-trading/{account,trades}` | 15s | ~5KB |
| Market data hooks (if active) | Various | 5–15s | variable |

**If multiple dashboards are open simultaneously** (paper-desk + btc-future-trading + mock-trading tabs in browser):
- Estimated total polling: 8–12 requests per 5s interval
- Estimated data transfer: ~50–150KB per 5s
- **Over 72 hours**: ~125 million polling requests from a single browser session (theoretical upper bound)

**Vercel serverless cost**: Each poll hits a Next.js API route, spinning up a serverless function. 12 req/5s = 8,640 req/hour. Over 72h = ~622,000 serverless invocations per browser session.

---

## RENDER PERFORMANCE CONCERNS

### Large Component Files

| Component | File Size | Risk |
|-----------|-----------|------|
| `TradeJournalPro.tsx` | 256KB | High — large component likely has expensive renders |
| `useBTCFuturesScalperEngine.ts` | 171KB | High — large hook re-runs on every poll |
| `useBTCSpotScalperEngine.ts` | 109KB | High |
| `MockTradingDashboard.tsx` | 81KB | Medium-High |
| `useMCXEngine.ts` | 72KB | Medium |
| `useNiftyOptionsEngine.ts` | 74KB | Medium |

Evidence basis: file sizes strongly suggest these components manage large state objects and compute expensive aggregations on every render cycle.

### `useBTCFuturesScalperEngine` (171KB) — Performance Risk
This hook:
- Contains 600+ strategy definitions (`FUTURES_STRAT_DEFS` catalog)
- Runs multi-timeframe confluence scoring on every poll tick
- Manages Kelly sizing calculation
- Manages family conflict detection across all strategies
- Persists marks every 15s

On each 5s poll, the entire signal evaluation pipeline runs against 600+ strategies in JavaScript, in the browser's main thread. This is a CPU-intensive operation that will cause frame drops on lower-end hardware and may degrade over long sessions as state accumulates.

### Store Complexity

Evidence:
- No Zustand or Redux stores found — all state is component-local via `useState`
- Terminal snapshot: single context spread across all terminal pages
- BTC Futures engine: single hook with 170KB of logic managing all state
- **Risk**: Component-local state means React reconciliation runs on every hook update across potentially hundreds of state items

---

## MEMORY CONSUMPTION CONCERNS

### Trade History Growth
Evidence:
- `useMockTradingEngine` (42KB) — "Maintains up to 500 trade cache"
- BTC Futures engine caches trades for signal trace, attribution, equity curve
- Over 72 hours with active trading: trade arrays grow large
- No evidence of memory-bounded caching with LRU eviction in visible hook code
- Paper desk lazy-fetches trade pages on demand — this is safer, bounded server-side

### Strategy State Accumulation
Evidence:
- 600+ strategies each maintain individual state (cooldown, last signal, performance metrics)
- In-browser state machine grows over time
- No cleanup/garbage collection mechanism observed

---

## REALTIME UPDATE LOAD

### WebSocket (Terminal)
- Currently NOT CONNECTED — no WS load
- If wired: single persistent WebSocket, efficient for real-time

### HTTP Polling
- 5s polling is the primary mechanism
- Each poll is a stateless HTTP request — no connection overhead but no push capability
- Vercel cold starts: if serverless function isn't warm, first poll after idle could have 200–500ms latency spike

---

## LONG-SESSION STABILITY RISKS

| Risk | Mechanism | Severity |
|------|-----------|----------|
| Memory leak in 600-strategy state machine | State grows unbounded | HIGH |
| Render degradation over 72h | State accumulation → expensive diffs | MEDIUM |
| Polling accumulation | Multiple open tabs multiply load | HIGH |
| No visibility into client performance | No `useDeskPerformanceMonitor` output visible | MEDIUM |
| Browser tab crash | No recovery UI | HIGH — trader would lose monitoring context |

---

## PERFORMANCE SCORECARD

| Dimension | Score |
|-----------|-------|
| Polling efficiency | 5/10 — pattern is correct but load unoptimized |
| Render performance | 3/10 — large components, no memoization audit |
| Memory management | 3/10 — unbounded state growth risk |
| Long-session stability | 3/10 — no tested 72h run evidence |
| Real-time update efficiency | 7/10 — WS design is correct (when wired) |
| Serverless cost efficiency | 4/10 — polling costs compound on Vercel Hobby |

**Overall Score: 4/10**

The most serious risk for 72-hour autonomous operation is the in-browser strategy engine (171KB) running CPU-intensive signal evaluation every 5s. This is not a scalable architecture for continuous unattended monitoring — it requires the browser to remain open and active.
