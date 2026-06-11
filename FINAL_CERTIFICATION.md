# FINAL CERTIFICATION — Phase 20
Generated: 2026-06-11 | Forensic Code Audit

All answers are based exclusively on source code evidence with file:line citations.

---

## CERTIFICATION QUESTIONS

---

### Q1: Is UI aligned with backend?
**PARTIAL**

**Evidence supporting alignment:**
- The Paper Desk UI (`/paper-desk`) reads from the exact MongoDB collections written by the Go engine: `paper_state`, `paper_trades`, `paper_positions`, `paper_orders`, `equity_curve`, `daily_pnl_history`, `strategy_scores`, `strategy_health`. Collection name constants match exactly between `engine/internal/paperpersist/mongo.go:60-73` and `client/src/lib/paperDeskClient.ts:18-27`.
- TypeScript type `PaperStateDoc` (`paperDeskClient.ts:30-51`) matches Go struct `AccountSnapshot` (`state_snapshotter.go:25-53`) field by field including `balance`, `equity`, `unrealized_pnl`, `realized_pnl`, `peak_equity`, etc.
- Authentication is JWT-validated on every Paper Desk API route.

**Evidence against alignment:**
- The entire `/terminal/*` UI section (`/terminal/execution`, `/terminal/risk`, `/terminal/analytics`, `/terminal/research`, `/terminal/journal`) is **permanently disconnected** from the backend because `NEXT_PUBLIC_TERMINAL_WS_URL` is not set. These 5 pages show `initialTerminalSnapshot` hardcoded data: `terminalStore.tsx:21-22`.
- The `/btc-future-trading` and `/mock-trading` pages run a client-side simulation engine — not the Go backend.
- BTC mark price used for unrealized PnL differs between engine (`GetLastPrice()` from engine's market data client) and UI (`/api/btc/price` from Binance REST → Coinbase fallback). These can diverge.

---

### Q2: Is UI aligned with OMS?
**PARTIAL**

**Evidence supporting alignment:**
- OMS v3 transitions are written to MongoDB `paper_orders` collection by `paperpersist/order_writer.go`.
- The UI reads `paper_orders` via `/api/paper-desk/orders/route.ts` which feeds the OMS tab in Paper Desk.
- The OMS tab in `PaperDeskDashboard.tsx` displays order transitions (state machine events) from the real engine OMS.

**Evidence against alignment:**
- OMS v3 event ledger is `ledger.NewMemoryStore()` (`trading/loop.go:235`). This means the full OMS event log is in-process memory only. The MongoDB `paper_orders` collection captures snapshots of OMS transitions written by `order_writer.go` but is not the event sourcing store.
- The terminal `/terminal/execution` Quick Trade Panel is not connected to the OMS. Any "order" placed there has no path to execution.
- The blocked `/api/angelone/order` route means the AngelOne OMS path for NSE equity is retired with no replacement in the Next.js layer. Engine handles it internally.

---

### Q3: Is UI aligned with portfolio?
**PARTIAL**

**Evidence supporting alignment:**
- `/api/paper-desk/portfolio/route.ts` calls `getPortfolioAccountingSnapshot()` which computes: equity, realized PnL, unrealized PnL, exposure, drawdown from MongoDB — all from engine-written data.
- Portfolio accounting service independently recalculates rather than trusting cached `paper_state` values: `portfolioAccountingService.ts:14-84`.
- Portfolio consistency validation (`/api/paper-desk/validation`) cross-checks `paper_state` vs `SUM(paper_trades.net_pnl)`.

**Evidence against alignment:**
- The `buildPortfolioAccountingSnapshot` path at `snapshot/route.ts:43-53` does not receive a live BTC mark price. Unrealized PnL is computed from position data without a real-time mark. The engine's `paper_state.unrealized_pnl` (from `GetLastPrice()`) and the UI's portfolio accounting unrealized PnL will diverge in fast markets.
- Portfolio summary on terminal pages uses hardcoded risk data (`netExposureUsd: 7_210`, `grossExposureUsd: 30_720`) from `terminalSnapshot.ts:73-76`.

---

### Q4: Is UI aligned with execution?
**PARTIAL**

**Evidence supporting alignment:**
- Paper Desk positions (`/api/paper-desk/positions`) and trades (`/api/paper-desk/trades`) reflect real engine execution output. The Go engine's execution path (`trading/loop.go`) writes fills to MongoDB via paperpersist hooks.
- The fill record includes `fill_quality`, `slippage_bps`, `latency_ms`, `exec_confidence` — engine execution intelligence is captured and exposed.

**Evidence against alignment:**
- Terminal execution page (`/terminal/execution`) shows 2 hardcoded positions. Real engine positions are not shown here.
- The `CommandCenter` component (`CommandCenter.tsx`) calls engine bridge APIs (`/api/ai/pending`, `/api/ai/submit`) with empty `catch {}` blocks. Failed bridge calls are invisible. The AI execution bridge status shown may be incorrect.
- `AngelOne` direct execution is blocked. Real NSE execution goes through the engine, not the Next.js API layer, which means the Vercel UI cannot independently verify an AngelOne fill occurred.

---

### Q5: Is UI aligned with reconciliation?
**NO**

**Evidence:**
- The reconciliation engine (`reconciliationv2`) is fully implemented and starts via `WireProduction` in `main.go`: `reconciliationv2/wiring.go:29-90`.
- Reconciliation runs cycles and detects drift events. When drift is detected above threshold, the kill switch is triggered via `CriticalDriftKillSwitchHook`.
- **No reconciliation data is written to MongoDB**. Audit entries go to `AuditLog` which writes to `ledger.Store` = `ledger.NewMemoryStore()`. This is in-process only.
- **No UI route reads reconciliation results**. There is no `/api/paper-desk/reconciliation` route, no reconciliation panel in Paper Desk, and no display of drift events in any dashboard.
- Users are completely blind to reconciliation events. They cannot see if drift was detected, repaired, or if the kill switch was triggered by reconciliation.

---

### Q6: Can users trust dashboard data?
**PARTIAL (with important caveats)**

**Paper Desk dashboard (`/paper-desk`):** Data flows from Go engine → MongoDB Atlas → Next.js API → UI. The data pipeline is real. However:
- Up to 15s latency for state updates (10s engine snapshot + 5s UI poll)
- BTC mark price source mismatch can cause unrealized PnL discrepancy
- `$50` PnL tolerance allows undetected drift

**Terminal dashboard (`/terminal/execution`, `/terminal/risk`, etc.):** **Cannot be trusted**. All data is hardcoded in `terminalSnapshot.ts`. The "LIVE" badge and real-looking numbers are fabricated. Evidence: `client/src/lib/terminal/terminalStore.tsx:21-22`, `client/.env.local` (no `NEXT_PUBLIC_TERMINAL_WS_URL`).

**Root dashboard (`/`):** BTC price, equity, PnL, open positions — partially trusted when connected. Win rate defaults to 0, profit factor and Sharpe show "—" when strategy scores are absent.

---

### Q7: Can users trust strategy data?
**PARTIAL**

**Paper Desk strategy health**: Real data from `strategy_health` and `strategy_scores` MongoDB collections, written by engine every 15 minutes. Delays of up to 15 minutes. Strategies with fewer than 20 trades always show `INSUFFICIENT_DATA`. Evidence: `engine/internal/paperpersist/strategy_health.go:37`.

**Terminal research page**: 5 hardcoded strategies from `terminalSnapshot.ts:81-87`. The 600+ real strategies running in the engine are not shown anywhere in the terminal.

**Strategy enable/disable**: Cannot be done from UI. The `BuildCuratedScalpers()` function is hardcoded; no runtime toggle. Evidence: `engine/internal/strategy/curated_registry.go:6`.

---

### Q8: Can users trust execution data?
**PARTIAL**

**Paper Desk fills/trades tab**: Real data from `paper_trades` MongoDB collection. Includes slippage, latency, fill quality. Trusted.

**Paper Desk OMS tab**: Shows real OMS order transitions from `paper_orders`. Trusted within the scope of what paperpersist captures.

**Terminal execution page**: Completely fabricated. 2 hardcoded positions, hardcoded order book. Untrusted.

**Silent failures**: `BTCFuturesScalper.tsx:268` has `.catch(() => {})` on persistence writes. A trade that the UI shows as executed may not have been written to MongoDB. Evidence: `client/src/components/BTCFuturesScalper.tsx:268`.

---

### Q9: Can users trust PnL data?
**PARTIAL**

**Realized PnL**: Accumulated in-memory `PortfolioLedger`, written to `paper_state.realized_pnl` every 10s. Read from MongoDB by UI. The consistency validation at `portfolioConsistencyValidation.ts:58-78` cross-checks this against `SUM(paper_trades.net_pnl)` but with $50 tolerance. Partially trusted.

**Unrealized PnL**: Computed independently at 3 different points with 3 different BTC price sources:
1. Engine: `paperpersist_hooks.go:139-140` using engine's WS mark price
2. UI per-position: `paperDeskPositionMath.ts:16-24` using `/api/btc/price` (Binance REST)
3. Portfolio accounting: `portfolioAccountingService.ts` from MongoDB positions

These three values will frequently diverge. The displayed total PnL on the dashboard combines realized PnL (from point 1) with unrealized PnL (from point 3 in the snapshot, or point 2 in per-position display). This creates inconsistent totals depending on which tab the user is on.

**Terminal PnL**: Hardcoded `unrealizedPnl: 166.05` and `40.42`. Untrusted.

---

### Q10: Can users trust risk data?
**PARTIAL**

**Paper Desk drawdown**: Real — computed from MongoDB equity curve + peak equity in `portfolioAccountingService.ts:29-80`. Trusted within the 10s snapshot interval.

**Paper Desk exposure**: Real — computed from open positions. Trusted.

**Terminal VaR/CVaR/heat**: Hardcoded static values. Completely untrusted.

**Kill switch status**: Not shown in any UI. Users cannot see if the kill switch is active. Not exposed via any dashboard API route.

**Reconciliation drift status**: Not shown in any UI. Not trusted because invisible.

---

### Q11: Can backend operate without UI awareness?
**YES**

**Evidence:**
- The Go engine runs as an independent process on AWS Lightsail.
- It connects directly to MongoDB Atlas, Binance WS, Coinbase WS, Delta Exchange REST, and Yahoo Finance — all without the UI.
- Kill switch, reconciliation, risk gate, OMS v3, execution loop — all run headlessly.
- The engine self-pings `/health` to prevent hosting sleep (`main.go:92-115` area).
- Paper trades and state are written to MongoDB regardless of UI connectivity.
- Evidence confirmed: `engine/cmd/antigravity/main.go` imports and initializes all subsystems without any Next.js dependency.

---

### Q12: Can UI operate without backend awareness?
**PARTIALLY (degrades gracefully for Paper Desk; fully operational for Terminal)**

**Evidence:**
- If MongoDB is unavailable, Paper Desk routes return `503 MONGO_NOT_CONFIGURED` or `MONGO_UNAVAILABLE`. The hook sets `connection = "error"`.
- If the Go engine is down, Paper Desk shows stale data (last MongoDB snapshot).
- Root dashboard: BTC price uses Binance → Coinbase fallback — operational without engine.
- **Terminal pages: fully operational** without backend — they always show hardcoded data regardless. This is not a feature; it is the bug described throughout this audit.
- Mock trading (`/mock-trading`, `/btc-future-trading`): fully client-side, MongoDB persistence optional. Operates without the Go engine.
- The UI does NOT write to MongoDB directly for Paper Desk — only reads. All Paper Desk writes come from the Go engine. So the UI genuinely cannot "operate" Paper Desk without the backend.

---

## FINAL VERDICT

### VERDICT 3 — MATERIAL MISALIGNMENTS

**Justification:** The application has a clear split between the Go engine paper trading path (which is well-wired and mostly functional) and the terminal UI (which is completely disconnected from the backend). The terminal sections — which include execution, risk, analytics, research, and trade journal — are showing hardcoded demo data as if it were live data. This is a material misalignment because it causes users to read fabricated risk metrics, fabricated positions, and fabricated PnL as real data. Additionally, the reconciliation system, kill switch persistence, and OMS event log are all invisible to the UI.

---

## SCORES (0-10)

| Dimension | Score | Justification |
|---|---|---|
| **Architecture Score** | 6/10 | Go engine architecture is sound (event-driven, paperpersist pipeline, reconciliation v2, kill switch, risk gate). Deducted for: in-memory ledger store, no reconciliation UI surface, dual trading engine confusion, no mark price consensus. |
| **Frontend Quality Score** | 5/10 | Paper Desk pages are well-built with real data flows. Deducted heavily for: all 5 terminal pages showing hardcoded data, silent failures in critical paths (CommandCenter, BTCFuturesScalper), no error propagation for persistence failures. |
| **Backend Quality Score** | 7/10 | Go engine has institutional-grade components: reconciliationv2, kill switch with triggers, OMS v3 event sourcing, risk gate pipeline, strategy health scoring. Deducted for: in-memory ledger (loses events on restart), kill switch not persisted to MongoDB, strategy management requires redeploy. |
| **Synchronization Score** | 5/10 | Paper Desk is synchronized via 10s engine snapshot + 5s UI poll. But: unrealized PnL has 3 independent calculation paths with different price sources, no mark price consensus, no write fencing between `paper_state` and `paper_positions`, reconciliation events never reach UI. |
| **Operational Trust Score** | 4/10 | Paper Desk positions/fills/trades: trusted. Terminal risk/PnL/execution: completely untrustworthy (hardcoded). Kill switch status: invisible. Reconciliation drift: invisible. PnL tolerance $50 too high. Silent failures in persistence layer. |
| **Production Readiness Score** | 5/10 | Engine runs continuously with kill switch, reconciliation, risk gates, and persistent MongoDB store. NOT production-ready: in-memory OMS ledger (events lost on restart), kill switch state not persisted (lost on restart), terminal UI is demo-mode, no visibility into reconciliation or kill switch from the dashboard. |

---

## EVIDENCE SUMMARY TABLE

| Claim | Evidence File:Line |
|---|---|
| Terminal uses hardcoded data | `client/src/lib/terminal/terminalStore.tsx:21-22` |
| WS URL not configured | `client/.env.local` (no `NEXT_PUBLIC_TERMINAL_WS_URL` key) |
| Hardcoded snapshot data | `client/src/lib/terminal/terminalSnapshot.ts:17-121` |
| Paper Desk polls real MongoDB | `client/src/hooks/usePaperDesk.ts:84` |
| Engine writes to MongoDB every 10s | `engine/internal/paperpersist/state_snapshotter.go:88-90` |
| OMS ledger is in-memory | `engine/internal/trading/loop.go:235` |
| Kill switch not persisted | `engine/internal/killswitch/service.go:52-59` |
| Reconciliation never reaches UI | `engine/internal/reconciliationv2/wiring.go:43-90` (no MongoDB write) |
| Silent persistence failure | `client/src/components/BTCFuturesScalper.tsx:268` |
| Empty catch blocks in CommandCenter | `client/src/components/CommandCenter.tsx:62,79` |
| Hardcoded RiskModule families | `client/src/components/terminal/institutional/RiskModule.tsx:7-12` |
| Hardcoded correlation heatmap formula | `client/src/components/terminal/institutional/RiskModule.tsx:38-42` |
| AngelOne order route blocked/retired | `client/src/app/api/angelone/order/route.ts:6-9` |
| $50 PnL tolerance | `client/src/lib/portfolioConsistencyValidation.ts:33` |
| Paper Desk collection name match | `engine/internal/paperpersist/mongo.go:60-73`, `client/src/lib/paperDeskClient.ts:18-27` |
| Engine operates independently | `engine/cmd/antigravity/main.go:1-46` (all imports, no Next.js dep) |
| 3 separate unrealized PnL calculations | `paperpersist_hooks.go:139`, `paperDeskPositionMath.ts:16`, `portfolioAccountingService.ts` |
