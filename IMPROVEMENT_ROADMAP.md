# IMPROVEMENT ROADMAP — Phase 18
Generated: 2026-06-11 | Forensic Code Audit

Each item: Impact (HIGH/MED/LOW) | Risk (HIGH/MED/LOW) | Complexity (HIGH/MED/LOW)

---

## QUICK WINS (< 1 day each)

### QW-1: Set `NEXT_PUBLIC_TERMINAL_WS_URL` or remove the terminal as a feature
- **File**: `client/.env.local` + `client/src/lib/terminal/terminalStore.tsx:21`
- **Action**: Either configure the env var to point to a real WebSocket endpoint on the Go engine, or add a visible "DEMO DATA" banner on all terminal pages so users know the data is not live.
- **Impact**: HIGH — currently users view fabricated risk/PnL/position data as real
- **Risk**: LOW — banner is purely additive; connecting WS requires engine endpoint to exist
- **Complexity**: LOW

### QW-2: Add "DEMO DATA" overlay to all terminal pages
- **File**: `client/src/components/terminal/institutional/TerminalShell.tsx`
- **Action**: Check if `wsUrl` is falsy and render a persistent top banner: "Terminal is displaying demo data — connect engine WebSocket to view live data"
- **Impact**: HIGH — prevents user trust violation
- **Risk**: LOW
- **Complexity**: LOW

### QW-3: Surface stale-data timestamp in Paper Desk header
- **File**: `client/src/components/PaperDeskDashboard.tsx`, `client/src/hooks/usePaperDesk.ts:74`
- **Action**: Show "Last updated: Xs ago" using `lastUpdated` state field. When connection = "stale", add red border to header.
- **Impact**: MED — users know how fresh the data is
- **Risk**: LOW
- **Complexity**: LOW

### QW-4: Remove `catch {}` in CommandCenter
- **File**: `client/src/components/CommandCenter.tsx:62,79`
- **Action**: Replace both empty `catch {}` blocks with error state updates so users see "Bridge unreachable" when the engine is down.
- **Impact**: HIGH — bridge failure is silent; AI signal review becomes non-functional without notice
- **Risk**: LOW
- **Complexity**: LOW

### QW-5: Add persistence error notification in `useBTCFuturesScalperEngine`
- **File**: `client/src/hooks/useBTCFuturesScalperEngine.ts:1413`
- **Action**: Replace `.catch(() => {})` with `.catch((err) => markPersistenceError(err))` so users see the persistence error badge when trades fail to save.
- **Impact**: HIGH — trades may silently not be saved
- **Risk**: LOW (function already exists)
- **Complexity**: LOW

### QW-6: Reduce PnL consistency tolerance from $50 to $5
- **File**: `client/src/lib/portfolioConsistencyValidation.ts:33`
- **Action**: Change `PNL_TOLERANCE_USD = 50` to `PNL_TOLERANCE_USD = 5`
- **Impact**: MED — catch smaller PnL drift before it compounds
- **Risk**: LOW
- **Complexity**: LOW

### QW-7: Block hardcoded families in RiskModule
- **File**: `client/src/components/terminal/institutional/RiskModule.tsx:7-12`
- **Action**: Replace static `families` array with a prop derived from `snapshot.strategies` grouped by family.
- **Impact**: MED — exposure by family will reflect actual strategy groupings
- **Risk**: LOW
- **Complexity**: LOW

### QW-8: Add explicit deprecation header to AngelOne order route
- **File**: `client/src/app/api/angelone/order/route.ts`
- **Action**: The route already returns `blockedDirectExecutionRoute` but callers may be confused. Add a logged warning when this route is hit to surface any code paths still calling it.
- **Impact**: LOW
- **Risk**: LOW
- **Complexity**: LOW

---

## MEDIUM FIXES (1-3 days each)

### MF-1: Wire Terminal pages to real Go engine data
- **Files**: `client/src/lib/terminal/terminalStore.tsx`, `engine/cmd/antigravity/main.go`
- **Description**: Implement a WebSocket endpoint in the Go engine that emits `TerminalDelta` JSON matching `TerminalSnapshot` shape. Register it at `/api/ws/terminal`. Set `NEXT_PUBLIC_TERMINAL_WS_URL` to the engine endpoint. The store already handles WebSocket connection with reconnect logic.
- **Impact**: HIGH — all terminal pages become live
- **Risk**: MED — requires engine-side WebSocket handler and schema alignment
- **Complexity**: MED

### MF-2: Replace hardcoded correlation heatmap with real strategy correlations
- **File**: `client/src/components/terminal/institutional/RiskModule.tsx:36-47`
- **Description**: Fetch strategy PnL time series from MongoDB `strategy_scores` collection. Compute Pearson correlations server-side in a new API route `/api/paper-desk/strategy-correlations`. Replace formula with real data.
- **Impact**: MED — risk view becomes meaningful for strategy diversification decisions
- **Risk**: LOW
- **Complexity**: MED

### MF-3: Replace hardcoded research tournament stats
- **File**: `client/src/components/terminal/institutional/ResearchCenter.tsx:47-62`
- **Description**: Create API route `/api/paper-desk/research-summary` that queries MongoDB `strategy_health` collection and counts: HEALTHY (eligible), WARNING, CRITICAL/DISABLED (retired), total. Replace 4 hardcoded metric values and 4 hardcoded walk-forward percentages.
- **Impact**: MED — research center becomes informational
- **Risk**: LOW
- **Complexity**: MED

### MF-4: Add SSE/WebSocket for Paper Desk real-time updates
- **Files**: `client/src/hooks/usePaperDesk.ts`, engine
- **Description**: The hook comment (`usePaperDesk.ts:3-20`) says Vercel serverless rules out SSE. However, the engine (AWS Lightsail) could push updates via WebSocket and the Vercel frontend could connect directly to the engine WebSocket (not via Vercel). Implement this as an optional upgrade to eliminate 5s polling latency for trade opens/closes.
- **Impact**: MED — positions update in <1s instead of ~5s
- **Risk**: MED — CORS, engine WebSocket registration needed
- **Complexity**: MED

### MF-5: Unify BTC mark price source between engine and UI
- **Files**: `engine/internal/trading/paperpersist_hooks.go:139`, `client/src/app/api/btc/price/route.ts`
- **Description**: The engine uses `o.exec.GetLastPrice()` from its own market data client (Delta/Binance WS). The UI uses a different REST polling path (Binance REST API → Coinbase fallback). Expose the engine's current BTC mark price via `/api/paper-desk/state` response and have the UI use this as its mark price for unrealized PnL calculation.
- **Impact**: HIGH — eliminates unrealized PnL discrepancy between engine and UI
- **Risk**: LOW — additive field in existing response
- **Complexity**: LOW-MED

### MF-6: Add stale-data TTL indicator to all positions
- **File**: `client/src/components/PaperDeskDashboard.tsx`
- **Description**: The `opened_at` field exists on `PaperPositionDoc`. Add a visual indicator showing position age and a warning when positions haven't been updated in >60s (would indicate engine stopped writing).
- **Impact**: MED — users notice if engine stops running
- **Risk**: LOW
- **Complexity**: LOW-MED

### MF-7: Consolidate duplicate paper state models
- **Files**: `client/src/app/api/paper-desk/state/route.ts:42-60`, `client/src/app/api/paper-desk/snapshot/route.ts`
- **Description**: `/api/paper-desk/state` and `/api/paper-desk/snapshot` both construct synthetic paper state when `paper_state` MongoDB doc is null. The fallback construction at `state/route.ts:43-60` creates a synthetic doc with hardcoded `balance: 1_000_000` that duplicates `portfolioAccountingService.ts`. Consolidate into one function.
- **Impact**: MED — prevents divergence between state and snapshot responses
- **Risk**: LOW-MED (test both endpoints before/after)
- **Complexity**: MED

### MF-8: Surface MongoDB connection status in Paper Desk header
- **Files**: `client/src/hooks/usePaperDesk.ts`, `client/src/components/PaperDeskDashboard.tsx`
- **Description**: Currently if MongoDB is unconfigured, the user sees a 503 error with `{ code: "MONGO_NOT_CONFIGURED" }` but there is no UI handling for this specific error code — it falls into the generic `setConnection("error")`. Add specific handling to show "Database not configured — contact administrator".
- **Impact**: MED — better operational awareness
- **Risk**: LOW
- **Complexity**: LOW

---

## MAJOR UPGRADES (1+ week each)

### MU-1: Wire Terminal to Live Engine Data (Complete)
- **Files**: All `client/src/components/terminal/institutional/*.tsx`, `client/src/lib/terminal/terminalStore.tsx`, engine
- **Description**: Complete re-architecture of the terminal data layer. Currently all 5 terminal sub-pages (execution, risk, analytics, research, journal) consume `initialTerminalSnapshot` permanently. Required work: (1) Define engine-side WebSocket handler that emits live `TerminalSnapshot` deltas, (2) Map `paper_state`, `paper_positions`, `paper_orders`, `strategy_scores`, `equity_curve` to `TerminalSnapshot` shape, (3) Remove all hardcoded values from `terminalSnapshot.ts`, (4) Connect `terminalStore` WebSocket.
- **Impact**: HIGH — terminal becomes an operational dashboard instead of a demo
- **Risk**: MED — risk of data shape mismatch; need careful testing
- **Complexity**: HIGH

### MU-2: Engine-UI Strategy Enable/Disable Synchronization
- **Files**: `client/src/app/api/paper-desk/strategy-health/route.ts`, `engine/internal/paperpersist/strategy_health.go`, `engine/internal/strategy/curated_registry.go`
- **Description**: Users can see strategy health status in Paper Desk but cannot enable/disable strategies from the UI. The Go engine's strategy set is hardcoded in `BuildCuratedScalpers()`. Required: (1) Admin API to toggle strategy enabled state in the engine, (2) Engine persists enabled/disabled state to MongoDB, (3) UI shows toggle per strategy, (4) Engine reads state on startup.
- **Impact**: HIGH — operators cannot respond to losing strategies without redeploying
- **Risk**: HIGH — modifying live strategy registry risks accidental trade stoppage
- **Complexity**: HIGH

### MU-3: Real-Time Risk Metrics Pipeline
- **Files**: `engine/internal/risk/gate/pipeline.go`, `engine/internal/reconciliationv2/`, `client/src/components/terminal/institutional/RiskModule.tsx`
- **Description**: The engine has a `PreTradeRiskPipeline` that computes per-trade risk decisions but does not emit portfolio-level VaR/CVaR/heat to MongoDB or WebSocket. Required: (1) Compute portfolio VaR/CVaR using position-level mark-to-market at regular intervals, (2) Write to a new `portfolio_risk` MongoDB collection, (3) UI reads and displays via new API route.
- **Impact**: HIGH — enables real risk monitoring
- **Risk**: MED — VaR computation needs historical price data; could be expensive
- **Complexity**: HIGH

### MU-4: Trade Journal Wired to Real Paper Trades
- **Files**: `client/src/components/terminal/institutional/TradeJournalPro.tsx`, `client/src/app/api/paper-desk/trades/route.ts`
- **Description**: `TradeJournalPro` shows 3 hardcoded journal entries. Required: (1) Map `PaperTradeDoc` fields to `TerminalSnapshot.journal` shape (add `setupTag`, `rMultiple`, `holdingMinutes` fields to paper trade writes), (2) Fetch real trade history in the journal component, (3) Implement real CSV export from MongoDB data.
- **Impact**: MED — journal becomes a real audit tool
- **Risk**: LOW — additive fields on existing MongoDB documents
- **Complexity**: MED-HIGH

### MU-5: Reconciliation v2 Exposed in Dashboard
- **Files**: `engine/internal/reconciliationv2/`, `client/src/app/api/paper-desk/`
- **Description**: The reconciliation engine (`reconciliationv2`) runs internal checks comparing OMS v3 ledger vs position manager vs Delta exchange. Results go to the ledger store (in-memory `ledger.NewMemoryStore()`). This data never surfaces in the UI. Required: (1) Write reconciliation audit entries to MongoDB, (2) Expose `/api/paper-desk/reconciliation` route, (3) Show reconciliation status in Paper Desk with drift alerts.
- **Impact**: HIGH — operators blind to position drift events
- **Risk**: MED — requires schema for reconciliation events
- **Complexity**: HIGH

### MU-6: Dead Code Removal — Legacy Mock Trading Infrastructure
- **Files**: `client/src/hooks/useMockTradingEngine.ts`, `client/src/lib/mockTradingEngine.ts`, `client/src/lib/mockResearchIndicators.ts`, `client/src/lib/mockCandleBuilder.ts`, `client/src/app/api/mock-trading/*`
- **Description**: The client-side mock trading engine is a parallel trading simulation that runs entirely in the browser. It uses real MongoDB for persistence but bypasses the Go engine. This creates a confusing dual-system where `/mock-trading` and `/btc-future-trading` run client-side strategies while `/paper-desk` shows Go engine results. Required: (1) Decide which path is canonical, (2) Migrate client-side users to Go engine path, (3) Archive or remove mock trading infrastructure.
- **Impact**: MED — reduces complexity and confusion
- **Risk**: MED — breaking change for existing mock trading users
- **Complexity**: HIGH
