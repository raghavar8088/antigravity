# UI STATE CONSISTENCY REPORT — Forensic Audit Phase 6

**Date:** 2026-06-11  
**Scope:** client/src/app/api/paper-* routes and consuming components  
**Method:** Source code reading only. No assumptions.

---

## Data Sources and Polling Intervals

### Primary Poll: `/api/paper-desk/snapshot` — every 5 seconds
**File:** `client/src/hooks/usePaperDesk.ts:34`  
`const POLL_MS = 5000`

The hook (`usePaperDesk`) polls a single aggregated snapshot every 5s. Polling pauses when tab is hidden (`document.visibilityState` check, line 118). On tab-return, an immediate poll fires.

The snapshot returns:
- `state` → from `paper_state` MongoDB collection (engine writes every 10s)
- `open_positions` → from `paper_positions` collection (engine writes on event)
- `recent_trades` → last 20 rows from `paper_trades` collection
- `health_summary` → from `strategy_health` collection

---

## Per-Metric Analysis

### POSITIONS

**Source:** `paper_positions` MongoDB collection  
**Write path:** Go engine goroutines in `paperpersist_hooks.go:230-267`  
**Write trigger:** Every position open/close event, async goroutine  
**Read path:** `listOpenPositions(accountKey)` → `paper_positions.find({status:"OPEN"})` 
**Update frequency:** Engine writes on event; UI polls every 5s  
**Staleness:** Up to 5s after MongoDB write + async write delay  

**Can it show stale data?** YES  
- The engine writes are fire-and-forget goroutines. A position can be open in engine RAM for seconds before MongoDB is updated.
- Poll interval is 5s on top of that.
- **Evidence:** `engine/internal/trading/paperpersist_hooks.go:249` — `go func() { b.orderWriter.PersistOpenPosition(ctx, p) }()`

**Can it show incorrect data?** YES (in a failure case)  
- If the goroutine fails and retry is exhausted, the position exists in engine RAM but never reaches MongoDB. The UI will never see it.
- Unrealized PnL for positions is computed client-side in `portfolioAccountingService.ts` using the current BTC price from the snapshot. The mark price is the snapshot's price at the moment of fetch, not the real-time price. For volatile markets this diverges.

---

### PnL (REALIZED)

**Source:** `paper_trades` MongoDB collection + `paper_state`  
**Write path:** `TradeWriter.Write()` with retry queue — `paperpersist_hooks.go:422`  
**Write trigger:** Every position close event  
**Staleness:** Variable — retry queue is in-memory, written async

**Can it show stale data?** YES  
- On engine restart, the retry queue is lost. A trade that failed to write to MongoDB is permanently lost from the UI's perspective. The engine's in-memory `portfolioLedger` has it but that is not surfaced to the UI.

**Can it show incorrect data?** YES  
- The `buildPortfolioAccountingSnapshot` function in `portfolioAccountingService.ts` (line 93+) recomputes equity as `startingBalance + closedStats.realized_pnl + unrealized_pnl`. If the `paper_state` document has a different equity than what's computed from `paper_trades`, they will disagree. The code takes `paper_state` as primary but falls back to a reconstructed value.

---

### BALANCE (CASH)

**Source:** `paper_state.balance` field  
**Write path:** `StateSnapshotter` goroutine, every 10 seconds  
**Read path:** `getPaperState(accountKey)` → `paper_state.find_one()`  
**Staleness:** Up to 10s for engine writes + 5s poll = **up to 15 seconds stale**

**Can it show incorrect data?** YES  
- `paper_state.balance` is written from `GetAccountSnapshot()` in `paperpersist_hooks.go:128` which calls `o.exec.GetBalanceUSD()` — this is the RAM-only PaperClient balance. It includes fees but not unrealized exposure.
- The UI's `buildPortfolioAccountingSnapshot` also independently calculates balance as equity minus unrealized. These two values may differ.

---

### EQUITY

**Source:** `paper_state.equity` + client-side computation  
**Write path:** `StateSnapshotter` goroutine, every 10 seconds  
**Staleness:** Up to 15 seconds

**Can it show stale data?** YES  
- The equity curve is written only when `GetEquityPoint()` is called, which is every **5 minutes** (`engine/internal/paperpersist/equity_recorder.go` — inferred from comment in `paperpersist_hooks.go:306`).
- The equity sparkline on the UI is based on the equity curve collection, so it is at minimum 5 minutes stale.

---

### ORDERS

**Source:** Two separate systems — this is a critical finding  

**System A — Go Engine OMS v3:**  
Go engine uses `omsv3/aggregate.go` states: NEW → VALIDATED → RISK_APPROVED → SUBMITTED → ACKNOWLEDGED → FILLED  
These are stored in `ledger.MemoryStore` (in RAM) or PostgreSQL if `DATABASE_URL` is set.  
**There is no code path that writes Go engine OMS v3 state to any MongoDB collection.**

**System B — Next.js TypeScript paper_oms_orders:**  
`client/src/lib/paperOmsMongo.ts` writes to `paper_oms_orders` collection.  
States: NEW, RISK_CHECKED, REJECTED, ACCEPTED, SIMULATED_FILL, POSITION_OPENED, POSITION_CLOSED, CANCELLED  
`/api/paper-oms/orders` reads this collection.  
`PaperOmsPanel` component displays this.

**Evidence of disconnect:** No grep for `insertPaperOmsOrder` or `updatePaperOmsOrderStatus` calls from Go code was found. The TypeScript OMS and Go engine OMS v3 are completely separate state machines.

**Verdict:** The Orders panel in the UI shows TypeScript-side OMS state, NOT Go engine institutional OMS v3 state. If the TypeScript OMS is not called (e.g., all trades go through the Go engine directly), the UI order panel will be empty.

---

### FILLS / TRADES

**Source:** `paper_trades` MongoDB collection  
**Write path:** `TradeWriter.Write()` async with retry queue  
**Read path:** `listPaperTrades({limit:20})` from snapshot; full page via `fetchTradesPage`  
**Staleness:** Async write + 5s poll = variable

**Can it show stale data?** YES  
**Can it show incorrect data?** NO (if the trade reaches MongoDB, the data is accurate — it mirrors exactly what the engine computed)

---

### SIGNALS (Pending)

**Source:** Go engine in-memory `pendingSignals` map (loop.go:122)  
**Exposure path:** HTTP endpoint exposed at `/api/paper-bridge/pending` (inferred from CommandCenter component)  
**Update mechanism:** CommandCenter polls the engine proxy endpoint

**Can it show stale data?** YES — pending signals expire after 5 minutes (loop.go:1921) but the UI may show them for up to the poll interval after expiry.

---

## Polling Intervals Summary

| Component | Interval | Source |
|-----------|----------|--------|
| Paper Desk snapshot | 5 seconds | `usePaperDesk.ts:34` |
| Terminal Risk module | WebSocket (no interval) | `terminalStore.tsx:22` |
| Paper OMS Panel | Manual refresh only (no auto-poll) | `PaperOmsPanel.tsx:150-153` |
| Command Center bridge | ~10 seconds (inferred from engine autoFallbackMonitor) | loop.go:1864 |
| Equity curve | 5-minute engine write | `paperpersist_hooks.go:306` |
| State snapshot (engine→MongoDB) | 10 seconds | paperpersist StateSnapshotter |

---

## LocalStorage/SessionStorage Usage

**Files found using localStorage:**  
`client/src/components/BTCFutureTradingScalper.tsx:137-157`  
- Reads `${STORAGE_NS}_btc_ft_winners` and `${STORAGE_NS}_btc_ft_retired` as legacy migration data
- Immediately removes the keys after migration (`localStorage.removeItem`)
- This is a one-time migration path only. After migration, data is in MongoDB.
- **No current trading state is stored in localStorage** — only legacy migration remnants.

---

## Client-Side State That Could Diverge From Server

1. **`usePaperDesk` hook state** (`useState` in React):  
   On poll failure, the hook keeps last-good data and marks connection "stale" (line 105). The UI shows stale data with a "stale" indicator but does NOT clear the data. A user who was on the page during an engine outage will see the last-known state.

2. **Terminal Snapshot** (`terminalStore.tsx`):  
   Uses WebSocket connection to `NEXT_PUBLIC_TERMINAL_WS_URL`. On disconnect, keeps last snapshot with `connected: false`. The **initial snapshot is hardcoded mock data** (`terminalSnapshot.ts:17-80`) — if the WebSocket URL is not configured (env var not set), the terminal always shows **hardcoded fake data** (price $105,842.5, positions with fake IDs, fake risk metrics).

3. **Risk Module (Terminal)**:  
   `RiskModule.tsx:7-12` — the family exposure data is hardcoded:
   ```ts
   const families = [
     { name: "Funding", exposure: 24, heat: 1.1 },
     { name: "Order Flow", exposure: 19, heat: 0.8 },
   ```
   These values NEVER update regardless of actual portfolio state. The correlation heatmap (lines 38-47) computes values from a static formula, not from real strategy correlations.

---

## Verdict: Can the UI Lie?

**YES — confirmed for the following:**

1. **Terminal Risk Module shows hardcoded mock data** if `NEXT_PUBLIC_TERMINAL_WS_URL` is not set (which it is not in Vercel production based on CLAUDE.md describing a static deploy). The risk metrics (VaR, CVaR, heat, correlations, family exposures) shown in the terminal are FAKE.  
   Evidence: `client/src/lib/terminal/terminalSnapshot.ts:17` — `initialTerminalSnapshot` with hardcoded prices and risk figures.

2. **Orders panel shows TypeScript OMS state, not Go engine OMS v3 state.** If the TypeScript OMS is not being populated (all execution flows through the Go engine), the orders panel is empty or shows stale/irrelevant orders.

3. **Unrealized PnL is computed client-side** using a snapshot price that is at most 5 seconds fresh. In fast markets, the displayed unrealized PnL may differ significantly from engine-computed values.

4. **Paper state equity can disagree with reconstructed equity** in the UI snapshot route, where both `state.equity` from MongoDB and a recomputed value are available and could be inconsistent.

5. **If MongoDB is unavailable**, the UI returns `mongoUnavailable` error responses. The client hook sets `connection: "error"` and **keeps the last-good data displayed** — so the UI shows real data that is now frozen and aging.
